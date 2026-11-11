package editorbuffer

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"sort"
	"unicode/utf16"
	"unicode/utf8"
)

type Format struct {
	Encoding string `json:"encoding"`
	BOM      bool   `json:"bom"`
	EOL      string `json:"eol"`
}
type Imported struct {
	Buffer *Buffer
	Format Format
	Hash   string
	Bytes  int64
	// Binary is set for a file that is not text. Its buffer holds a rendering
	// for display only; the file must never be written from it.
	Binary bool
}
type countingReader struct {
	io.Reader
	count int64
}

func (r *countingReader) Read(p []byte) (int, error) {
	n, e := r.Reader.Read(p)
	r.count += int64(n)
	return n, e
}

// Import streams a file into a private normalized scratch file. It has no
// file-size or line-count limit and never materializes the input as a string.
// progress receives bytes read; callers run this outside their UI request path.
//
// What comes back always writes the source bytes again exactly when it is
// encoded unedited in the returned Format:
//   - UTF-8 (with or without a byte order mark) and BOM-marked UTF-16 text.
//   - Text that is not valid UTF-8 is read as Windows-1252, in which every
//     byte is one character, so no byte is lost or replaced.
//   - Text whose lines do not all end the same way gets the "Mixed" line
//     ending: the buffer remembers how each imported line ended.
//   - Anything with NUL characters, or BOM-marked UTF-16 that is not valid,
//     is not text. It comes back with Binary set and a display-only rendering
//     that Encode refuses to write.
//
// A second kind of read needs the source again from its start, so a source
// that is not an io.Seeker keeps the strict behaviour and reports the error.
func Import(ctx context.Context, source io.Reader, dir string, progress func(int64)) (*Imported, error) {
	seeker, _ := source.(io.Seeker)
	var start int64
	if seeker != nil {
		at, e := seeker.Seek(0, io.SeekCurrent)
		if e != nil {
			seeker = nil
		}
		start = at
	}
	mode := importText
	for {
		result, e := importAs(ctx, source, dir, progress, mode)
		if e == nil {
			return result, nil
		}
		next := mode
		switch {
		case errors.Is(e, errNotUTF8) && mode == importText:
			next = importLegacy
		case (errors.Is(e, errNUL) || errors.Is(e, errUTF16)) && mode != importBinary:
			next = importBinary
		}
		if next == mode || seeker == nil {
			return nil, e
		}
		if _, seekErr := seeker.Seek(start, io.SeekStart); seekErr != nil {
			return nil, e
		}
		mode = next
	}
}

const (
	importText   = iota // UTF-8, or UTF-16 when a byte order mark says so
	importLegacy        // Windows-1252: every byte is one character
	importBinary        // display only
)

var (
	errNotUTF8 = errors.New("invalid UTF-8 text")
	errNUL     = errors.New("file contains NUL characters and cannot be edited as text")
	errUTF16   = errors.New("invalid UTF-16 surrogate pair")
)

func importAs(ctx context.Context, source io.Reader, dir string, progress func(int64), mode int) (*Imported, error) {
	b, e := empty(dir)
	if e != nil {
		return nil, e
	}
	ok := false
	defer func() {
		if !ok {
			b.Close()
		}
	}()
	hash := sha256.New()
	counter := &countingReader{Reader: io.TeeReader(source, hash)}
	in := bufio.NewReaderSize(counter, ChunkBytes)
	format := Format{Encoding: "UTF-8", EOL: "LF"}
	if mode == importText {
		prefix, _ := in.Peek(3)
		if bytes.HasPrefix(prefix, []byte{0xef, 0xbb, 0xbf}) {
			format.BOM = true
			in.Discard(3)
		} else if len(prefix) >= 2 && prefix[0] == 0xff && prefix[1] == 0xfe {
			format.Encoding = "UTF-16LE"
			format.BOM = true
			in.Discard(2)
		} else if len(prefix) >= 2 && prefix[0] == 0xfe && prefix[1] == 0xff {
			format.Encoding = "UTF-16BE"
			format.BOM = true
			in.Discard(2)
		}
	} else if mode == importLegacy {
		format.Encoding = Windows1252
	} else {
		format.Encoding = BinaryEncoding
	}
	var order binary.ByteOrder = binary.LittleEndian
	if format.Encoding == "UTF-16BE" {
		order = binary.BigEndian
	}
	unit := func() (uint16, error) {
		var data [2]byte
		n, e := io.ReadFull(in, data[:])
		if e != nil {
			if n == 1 {
				// Half a code unit at the end of the file.
				return 0, errUTF16
			}
			return 0, e
		}
		return order.Uint16(data[:]), nil
	}
	next := func() (rune, error) {
		switch format.Encoding {
		case "UTF-8":
			r, n, e := in.ReadRune()
			if e == nil && r == utf8.RuneError && n == 1 {
				return 0, errNotUTF8
			}
			return r, e
		case Windows1252:
			c, e := in.ReadByte()
			if e != nil {
				return 0, e
			}
			return legacyRune(c), nil
		case BinaryEncoding:
			c, e := in.ReadByte()
			if e != nil {
				return 0, e
			}
			return binaryRune(c), nil
		}
		a, e := unit()
		if e != nil {
			return 0, e
		}
		if a >= 0xdc00 && a <= 0xdfff {
			return 0, errUTF16
		}
		if a >= 0xd800 && a <= 0xdbff {
			z, e := unit()
			if e != nil || z < 0xdc00 || z > 0xdfff {
				return 0, errUTF16
			}
			return utf16.DecodeRune(rune(a), rune(z)), nil
		}
		return rune(a), nil
	}
	chunk := make([]byte, 0, ChunkBytes+4)
	var crlf, lf, cr int64
	endings := &eolMap{}
	var flushedLines int64
	pendingCR := false
	flush := func() error {
		if len(chunk) == 0 {
			return nil
		}
		endings.starts = append(endings.starts, b.end)
		endings.ranks = append(endings.ranks, flushedLines)
		flushedLines += int64(bytes.Count(chunk, []byte{'\n'}))
		tree, e := b.append(chunk)
		if e != nil {
			return e
		}
		b.root = join(b.root, tree)
		chunk = chunk[:0]
		if progress != nil {
			progress(counter.count)
		}
		return ctx.Err()
	}
	emit := func(r rune) error {
		chunk = utf8.AppendRune(chunk, r)
		if len(chunk) >= ChunkBytes {
			return flush()
		}
		return nil
	}
	// Every line ending is stored as one '\n'; how it was written is recorded
	// before the '\n' is stored, so the record and the scratch file agree.
	newline := func(kind byte) error {
		endings.add(kind)
		return emit('\n')
	}
	for {
		if e = ctx.Err(); e != nil {
			return nil, e
		}
		r, readErr := next()
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			return nil, readErr
		}
		if r == 0 {
			return nil, errNUL
		}
		if pendingCR {
			pendingCR = false
			if r == '\n' {
				crlf++
				if e = newline(eolCRLF); e != nil {
					return nil, e
				}
				continue
			}
			cr++
			if e = newline(eolCR); e != nil {
				return nil, e
			}
		}
		if r == '\r' {
			pendingCR = true
			continue
		}
		if r == '\n' {
			lf++
			if e = newline(eolLF); e != nil {
				return nil, e
			}
			continue
		}
		if e = emit(r); e != nil {
			return nil, e
		}
	}
	if pendingCR {
		cr++
		if e = newline(eolCR); e != nil {
			return nil, e
		}
	}
	if e = flush(); e != nil {
		return nil, e
	}
	if e = b.file.Sync(); e != nil {
		return nil, e
	}
	endings.base = eolLF
	if crlf > 0 && crlf >= lf && crlf >= cr {
		format.EOL = "CRLF"
		endings.base = eolCRLF
	} else if cr > 0 && cr > lf {
		format.EOL = "CR"
		endings.base = eolCR
	}
	if endings.mixed && mode != importBinary {
		// The line endings differ: keep the record, so an unedited line is
		// written back the way it was read. NewLineEOL stays the most common
		// ending, which text added later uses.
		endings.end = b.end
		b.eol = endings
		format.EOL = MixedEOL
	}
	b.saved = b.root
	b.binary = mode == importBinary
	ok = true
	return &Imported{Buffer: b, Format: format, Hash: hex.EncodeToString(hash.Sum(nil)), Bytes: counter.count, Binary: mode == importBinary}, nil
}

// Windows1252 names the single-byte encoding used for text that is not valid
// UTF-8. MixedEOL names the line ending of a document whose lines do not all
// end the same way. BinaryEncoding marks content that is not text: it can be
// shown, never encoded.
const (
	Windows1252    = "Windows-1252"
	MixedEOL       = "Mixed"
	BinaryEncoding = "Binary"
)

// The characters of bytes 0x80 to 0x9F in Windows-1252 as browsers decode it
// (WHATWG Encoding Standard): the five bytes Windows leaves undefined map to
// the control characters with the same number, so all 256 bytes are distinct
// characters and decoding followed by encoding returns the same bytes.
var legacyHigh = [32]rune{
	0x20ac, 0x81, 0x201a, 0x192, 0x201e, 0x2026, 0x2020, 0x2021, 0x2c6, 0x2030, 0x160, 0x2039, 0x152, 0x8d, 0x17d, 0x8f,
	0x90, 0x2018, 0x2019, 0x201c, 0x201d, 0x2022, 0x2013, 0x2014, 0x2dc, 0x2122, 0x161, 0x203a, 0x153, 0x9d, 0x17e, 0x178,
}

func legacyRune(c byte) rune {
	if c >= 0x80 && c <= 0x9f {
		return legacyHigh[c-0x80]
	}
	return rune(c)
}

func legacyByte(r rune) (byte, bool) {
	if r < 0x80 || r >= 0xa0 && r <= 0xff {
		return byte(r), true
	}
	for i, known := range legacyHigh {
		if known == r {
			return byte(0x80 + i), true
		}
	}
	return 0, false
}

// binaryRune is the display character of one byte of a file that is not
// text. Control bytes become their Unicode control pictures so that every
// byte shows as something; this rendering is never written to a file.
func binaryRune(c byte) rune {
	switch {
	case c == '\t' || c == '\n' || c == '\r':
		return rune(c)
	case c < 0x20:
		return 0x2400 + rune(c)
	case c == 0x7f:
		return 0x2421
	}
	return legacyRune(c)
}

const (
	eolLF byte = iota
	eolCRLF
	eolCR
)

var eolNames = [...]string{"LF", "CRLF", "CR"}

// eolMap remembers how each line of an imported document ended. It is kept
// only for a document whose line endings differ. Pieces refer to the scratch
// file by offset and imported text is never rewritten there, so a '\n' that is
// still at an imported offset is a line the user did not retype. Text added
// later lies beyond end and uses base, the most common ending of the file.
// The record costs two bits for each imported line.
type eolMap struct {
	base   byte
	end    int64
	mixed  bool
	first  byte
	count  int64
	kinds  []byte  // two bits per imported '\n', in scratch order
	starts []int64 // scratch offset of each imported chunk
	ranks  []int64 // number of '\n' before that offset
}

func (m *eolMap) set(index int64, kind byte) {
	for int64(len(m.kinds)) <= index>>2 {
		m.kinds = append(m.kinds, 0)
	}
	m.kinds[index>>2] |= kind << ((index & 3) * 2)
}

func (m *eolMap) add(kind byte) {
	if !m.mixed {
		if m.count == 0 || kind == m.first {
			m.first = kind
			m.count++
			return
		}
		// The first different ending: write down all the earlier ones.
		m.mixed = true
		whole := m.count >> 2
		m.kinds = bytes.Repeat([]byte{m.first * 0x55}, int(whole))
		for i := whole << 2; i < m.count; i++ {
			m.set(i, m.first)
		}
	}
	m.set(m.count, kind)
	m.count++
}

func (m *eolMap) kind(index int64) byte {
	if index < 0 || index >= m.count {
		return m.base
	}
	return m.kinds[index>>2] >> ((index & 3) * 2) & 3
}

// eolCursor finds, for one encoding pass, which imported '\n' a piece starts
// at. It keeps the imported chunk it last looked at, so a document cut into
// many pieces by edits reads each chunk once instead of once for every piece.
type eolCursor struct {
	m     *eolMap
	file  io.ReaderAt
	chunk int
	data  []byte
	at    int   // bytes of data counted so far
	lines int64 // '\n' in data[:at]
}

// rank returns the index of the first imported '\n' at or after a scratch
// offset, or -1 for text added after the import.
func (c *eolCursor) rank(offset int64) (int64, error) {
	m := c.m
	if offset >= m.end {
		return -1, nil
	}
	i := sort.Search(len(m.starts), func(i int) bool { return m.starts[i] > offset }) - 1
	if i < 0 {
		return 0, nil
	}
	if c.data == nil || i != c.chunk {
		end := m.end
		if i+1 < len(m.starts) {
			end = m.starts[i+1]
		}
		data := make([]byte, end-m.starts[i])
		if _, e := c.file.ReadAt(data, m.starts[i]); e != nil {
			return 0, e
		}
		c.chunk, c.data, c.at, c.lines = i, data, 0, 0
	}
	to := int(offset - m.starts[i])
	if to < c.at {
		c.at, c.lines = 0, 0
	}
	c.lines += int64(bytes.Count(c.data[c.at:to], []byte{'\n'}))
	c.at = to
	return m.ranks[i] + c.lines, nil
}

// NewLineEOL is the line ending of text added to a document whose imported
// line endings differ, or "" for any other document.
func (b *Buffer) NewLineEOL() string {
	if b.eol == nil {
		return ""
	}
	return eolNames[b.eol.base]
}

func (s *Snapshot) validFormat(f Format) bool {
	switch f.Encoding {
	case "UTF-8", "UTF-16LE", "UTF-16BE":
	case Windows1252:
		if f.BOM {
			return false
		}
	default:
		return false
	}
	// Only a document that was imported with differing line endings can
	// write them back; nothing else has a record of them.
	return f.EOL == "LF" || f.EOL == "CRLF" || f.EOL == "CR" || f.EOL == MixedEOL && s.b.eol != nil
}

// ErrUnencodable reports a character that the chosen encoding cannot hold.
// Nothing has been written to the destination file when a save returns it.
type ErrUnencodable struct {
	Rune     rune
	Encoding string
}

func (e *ErrUnencodable) Error() string {
	return fmt.Sprintf("the character %q cannot be saved as %s text; choose UTF-8 under Encoding to keep every character", e.Rune, e.Encoding)
}

type countingWriter struct{ count int64 }

func (w *countingWriter) Write(p []byte) (int, error) { w.count += int64(len(p)); return len(p), nil }

// Encode writes one stable revision in the selected encoding and line endings.
// Its caller commits a staged file only after checking conflicts and consent.
func (s *Snapshot) Encode(ctx context.Context, dst io.Writer, f Format) error {
	return s.encode(ctx, dst, f, s.root)
}

// PrepareEncoding validates a snapshot range and returns its exact encoded
// byte length and a streaming encoder. The snapshot must stay open until the
// encoder finishes. A nil range selects the complete document.
func (s *Snapshot) PrepareEncoding(f Format, selection *[2]int64) (int64, func(context.Context, io.Writer) error, error) {
	if s.b.binary {
		return 0, nil, ErrBinary
	}
	if !s.validFormat(f) {
		return 0, nil, errors.New("unsupported text encoding or line ending")
	}
	root := s.root
	if selection != nil {
		from, to := selection[0], selection[1]
		if from < 0 || to < from || to > size(root).units {
			return 0, nil, ErrBoundary
		}
		_, tail, err := s.b.split(root, from)
		if err != nil {
			return 0, nil, err
		}
		root, _, err = s.b.split(tail, to-from)
		if err != nil {
			return 0, nil, err
		}
	}
	content := size(root)
	count, scale, bom := content.bytes, int64(1), int64(3)
	if f.Encoding != "UTF-8" {
		count, scale, bom = content.units*2, 2, 2
	}
	if f.EOL == "CRLF" {
		count += scale * content.lines
	}
	if f.BOM {
		count += bom
	}
	if f.Encoding == Windows1252 || f.EOL == MixedEOL {
		// The length depends on each line's own ending, and a character may
		// not exist in a single-byte encoding: find out before anything is
		// promised to the receiver.
		var counted countingWriter
		if e := s.encode(context.Background(), &counted, f, root); e != nil {
			return 0, nil, e
		}
		count = counted.count
	}
	return count, func(ctx context.Context, dst io.Writer) error { return s.encode(ctx, dst, f, root) }, nil
}

func (s *Snapshot) encode(ctx context.Context, dst io.Writer, f Format, root *node) error {
	if s.b.binary {
		return ErrBinary
	}
	if !s.validFormat(f) {
		return errors.New("unsupported text encoding or line ending")
	}
	w := bufio.NewWriterSize(dst, ChunkBytes)
	if f.BOM {
		var bom []byte
		switch f.Encoding {
		case "UTF-8":
			bom = []byte{0xef, 0xbb, 0xbf}
		case "UTF-16LE":
			bom = []byte{0xff, 0xfe}
		case "UTF-16BE":
			bom = []byte{0xfe, 0xff}
		}
		if _, e := w.Write(bom); e != nil {
			return e
		}
	}
	var order binary.ByteOrder = binary.LittleEndian
	if f.Encoding == "UTF-16BE" {
		order = binary.BigEndian
	}
	writeRune := func(r rune) error {
		if f.Encoding == "UTF-8" {
			_, e := w.WriteRune(r)
			return e
		}
		if f.Encoding == Windows1252 {
			c, known := legacyByte(r)
			if !known {
				return &ErrUnencodable{Rune: r, Encoding: f.Encoding}
			}
			return w.WriteByte(c)
		}
		var p [4]byte
		n := 2
		if r > 0xffff {
			a, z := utf16.EncodeRune(r)
			order.PutUint16(p[:2], uint16(a))
			order.PutUint16(p[2:], uint16(z))
			n = 4
		} else {
			order.PutUint16(p[:2], uint16(r))
		}
		_, e := w.Write(p[:n])
		return e
	}
	var endings *eolCursor
	if f.EOL == MixedEOL {
		endings = &eolCursor{m: s.b.eol, file: s.b.file}
	}
	var visit func(*node) error
	visit = func(n *node) error {
		if n == nil {
			return nil
		}
		if e := ctx.Err(); e != nil {
			return e
		}
		if e := visit(n.left); e != nil {
			return e
		}
		data, e := s.b.bytes(n.part)
		if e != nil {
			return e
		}
		// Most files are UTF-8/LF. Copy whole pieces on this path.
		if f.Encoding == "UTF-8" && f.EOL == "LF" {
			if _, e = w.Write(data); e != nil {
				return e
			}
		} else {
			// With differing line endings, each imported '\n' is written the
			// way it was read; rank finds this piece's first one.
			ending, line := eolLF, int64(-1)
			switch f.EOL {
			case "CRLF":
				ending = eolCRLF
			case "CR":
				ending = eolCR
			case MixedEOL:
				if line, e = endings.rank(n.part.offset); e != nil {
					return e
				}
			}
			for len(data) > 0 {
				r, k := utf8.DecodeRune(data)
				data = data[k:]
				if r == '\n' {
					if f.EOL == MixedEOL {
						ending = s.b.eol.kind(line)
						if line >= 0 {
							line++
						}
					}
					if ending != eolLF {
						if e = writeRune('\r'); e != nil {
							return e
						}
						if ending == eolCR {
							continue
						}
					}
				}
				if e = writeRune(r); e != nil {
					return e
				}
			}
		}
		return visit(n.right)
	}
	if e := visit(root); e != nil {
		return e
	}
	if e := ctx.Err(); e != nil {
		return e
	}
	return w.Flush()
}
