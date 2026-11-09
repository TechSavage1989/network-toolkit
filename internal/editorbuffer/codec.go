package editorbuffer

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"io"
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

// Import streams validated text into a private normalized scratch file. It has
// no file-size or line-count limit and never materializes the input as a string.
// progress receives bytes read; callers run this outside their UI request path.
func Import(ctx context.Context, source io.Reader, dir string, progress func(int64)) (*Imported, error) {
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
	var order binary.ByteOrder = binary.LittleEndian
	if format.Encoding == "UTF-16BE" {
		order = binary.BigEndian
	}
	unit := func() (uint16, error) {
		var data [2]byte
		_, e := io.ReadFull(in, data[:])
		if e != nil {
			return 0, e
		}
		return order.Uint16(data[:]), nil
	}
	next := func() (rune, error) {
		if format.Encoding == "UTF-8" {
			r, n, e := in.ReadRune()
			if e == nil && r == utf8.RuneError && n == 1 {
				return 0, errors.New("invalid UTF-8 text")
			}
			return r, e
		}
		a, e := unit()
		if e != nil {
			return 0, e
		}
		if a >= 0xdc00 && a <= 0xdfff {
			return 0, errors.New("invalid UTF-16 surrogate pair")
		}
		if a >= 0xd800 && a <= 0xdbff {
			z, e := unit()
			if e != nil || z < 0xdc00 || z > 0xdfff {
				return 0, errors.New("invalid UTF-16 surrogate pair")
			}
			return utf16.DecodeRune(rune(a), rune(z)), nil
		}
		return rune(a), nil
	}
	chunk := make([]byte, 0, ChunkBytes+4)
	var crlf, lf, cr int64
	pendingCR := false
	flush := func() error {
		if len(chunk) == 0 {
			return nil
		}
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
			return nil, errors.New("file contains NUL characters and cannot be edited as text")
		}
		if pendingCR {
			pendingCR = false
			if e = emit('\n'); e != nil {
				return nil, e
			}
			if r == '\n' {
				crlf++
				continue
			}
			cr++
		}
		if r == '\r' {
			pendingCR = true
			continue
		}
		if r == '\n' {
			lf++
		}
		if e = emit(r); e != nil {
			return nil, e
		}
	}
	if pendingCR {
		cr++
		if e = emit('\n'); e != nil {
			return nil, e
		}
	}
	if e = flush(); e != nil {
		return nil, e
	}
	if e = b.file.Sync(); e != nil {
		return nil, e
	}
	if crlf > 0 && crlf >= lf && crlf >= cr {
		format.EOL = "CRLF"
	} else if cr > 0 && cr > lf {
		format.EOL = "CR"
	}
	b.saved = b.root
	ok = true
	return &Imported{b, format, hex.EncodeToString(hash.Sum(nil)), counter.count}, nil
}

func validFormat(f Format) bool {
	return (f.Encoding == "UTF-8" || f.Encoding == "UTF-16LE" || f.Encoding == "UTF-16BE") && (f.EOL == "LF" || f.EOL == "CRLF" || f.EOL == "CR")
}

// Encode writes one stable revision in the selected encoding and line endings.
// Its caller commits a staged file only after checking conflicts and consent.
func (s *Snapshot) Encode(ctx context.Context, dst io.Writer, f Format) error {
	return s.encode(ctx, dst, f, s.root)
}

// PrepareEncoding validates a snapshot range and returns its exact encoded
// byte length and a streaming encoder. The snapshot must stay open until the
// encoder finishes. A nil range selects the complete document.
func (s *Snapshot) PrepareEncoding(f Format, selection *[2]int64) (int64, func(context.Context, io.Writer) error, error) {
	if !validFormat(f) {
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
	return count, func(ctx context.Context, dst io.Writer) error { return s.encode(ctx, dst, f, root) }, nil
}

func (s *Snapshot) encode(ctx context.Context, dst io.Writer, f Format, root *node) error {
	if !validFormat(f) {
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
			for len(data) > 0 {
				r, k := utf8.DecodeRune(data)
				data = data[k:]
				if r == '\n' && f.EOL != "LF" {
					if e = writeRune('\r'); e != nil {
						return e
					}
					if f.EOL == "CR" {
						continue
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
