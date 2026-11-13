package texttools

import (
	"encoding/base64"
	"errors"
	"io"
	"strconv"
	"unicode/utf8"
)

func init() {
	register(&Tool{ID: "base64-decode", Rewrite: true, run: base64Decode})
	register(&Tool{ID: "base64-encode", Rewrite: true, run: base64Encode})
	register(&Tool{ID: "hex-decode", Rewrite: true, run: hexDecode})
}

func base64Value(c byte) (byte, bool) {
	switch {
	case c >= 'A' && c <= 'Z':
		return c - 'A', true
	case c >= 'a' && c <= 'z':
		return c - 'a' + 26, true
	case c >= '0' && c <= '9':
		return c - '0' + 52, true
	case c == '+' || c == '-':
		return 62, true
	case c == '/' || c == '_':
		return 63, true
	}
	return 0, false
}

func isSpace(c byte) bool {
	return c == ' ' || c == '\t' || c == '\n' || c == '\r' || c == '\f' || c == '\v'
}

// notDecoded is the error of a decoder that met c, a character it does not
// decode, at offset at: nothing changes, and the message names the character
// (the caller adds its line and column). before tells whether the text before
// it decodes on its own; the page then offers "Decode the part before it",
// the same tool over just that part. A decoder never skips such a character.
func (j *job) notDecoded(at int64, c byte, what string, before bool) error {
	return &PositionError{Offset: at, Message: quoted(c, j.in.peek(utf8.UTFMax-1)) + " is not " + what + ".", Before: before && at > 0}
}

// quoted shows the character that starts with the byte c (rest are the bytes
// after it) as a message names it: '&', 'é', '\u00a0'.
func quoted(c byte, rest []byte) string {
	if c < utf8.RuneSelf {
		return strconv.QuoteRune(rune(c))
	}
	r, _ := utf8.DecodeRune(append([]byte{c}, rest...))
	return strconv.QuoteRune(r)
}

// base64Stream decodes Base64 a symbol at a time: the standard and the
// URL-safe alphabet, with or without padding, with any white space between
// symbols. emit receives each decoded byte and the input position (UTF-16
// units) of the group it came from.
type base64Stream struct {
	quad    [4]byte
	n       int
	start   int64
	padded  bool
	symbols int64
	emit    func(b byte, at int64) error
}

// feed takes the next character. It returns false, and takes nothing, for a
// character that is not Base64.
func (s *base64Stream) feed(c byte, at int64) (bool, error) {
	if isSpace(c) {
		return true, nil
	}
	if c == '=' {
		if s.n < 2 && !s.padded {
			return true, &PositionError{Offset: at, Message: "The Base64 padding (=) comes too early."}
		}
		if !s.padded {
			s.padded = true
			return true, s.flush()
		}
		return true, nil
	}
	v, ok := base64Value(c)
	if !ok {
		return false, nil
	}
	if s.padded {
		return true, &PositionError{Offset: at, Message: "The Base64 text continues after its padding (=)."}
	}
	if s.n == 0 {
		s.start = at
	}
	s.quad[s.n] = v
	s.n++
	s.symbols++
	if s.n == 4 {
		return true, s.flush()
	}
	return true, nil
}

// flush decodes the symbols collected so far: four make three bytes, three
// make two and two make one.
func (s *base64Stream) flush() error {
	q, n := s.quad, s.n
	s.n = 0
	if n == 0 {
		return nil
	}
	if n == 1 {
		return &PositionError{Offset: s.start, Message: "The Base64 text ends with an incomplete group."}
	}
	out := []byte{q[0]<<2 | q[1]>>4, q[1]<<4 | q[2]>>2, q[2]<<6 | q[3]}
	for _, b := range out[:n-1] {
		if err := s.emit(b, s.start); err != nil {
			return err
		}
	}
	return nil
}

// Base64 decode: the decoded bytes must be UTF-8 text. A character that is
// not Base64 (such as a "&&" after the Base64) stops it; the text before that
// character is offered on its own when its Base64 is complete.
func base64Decode(j *job) error {
	d := &decoded{j: j}
	stream := &base64Stream{emit: d.add}
	for {
		at := j.in.units
		c, err := j.in.byte()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		ok, err := stream.feed(c, at)
		if err != nil {
			return err
		}
		if !ok {
			// What a run over the text before c would do at its end.
			before := stream.symbols > 0 && stream.flush() == nil && d.done() == nil
			return j.notDecoded(at, c, "a Base64 character", before)
		}
		if len(d.out) >= 32<<10 {
			if err = j.out.Insert(d.take()); err != nil {
				return err
			}
		}
	}
	if err := stream.flush(); err != nil {
		return err
	}
	if stream.symbols == 0 {
		return errors.New("There is no Base64 text to decode.")
	}
	if err := d.done(); err != nil {
		return err
	}
	return j.out.Insert(d.take())
}

// insertWriter passes what is written to it on as new text.
type insertWriter struct{ j *job }

func (w insertWriter) Write(p []byte) (int, error) {
	if err := w.j.out.Insert(p); err != nil {
		return 0, err
	}
	return len(p), nil
}

// Base64 encode: the UTF-8 bytes of the text (line breaks as LF), standard
// alphabet with padding, on one line.
func base64Encode(j *job) error {
	enc := base64.NewEncoder(base64.StdEncoding, insertWriter{j})
	buf := make([]byte, 48<<10)
	for {
		n, err := io.ReadFull(j.in.r, buf)
		j.in.off += int64(n)
		if n > 0 {
			if _, e := enc.Write(buf[:n]); e != nil {
				return e
			}
		}
		if err == io.EOF || err == io.ErrUnexpectedEOF {
			break
		}
		if err != nil {
			return err
		}
		if e := j.in.tick(); e != nil {
			return e
		}
	}
	return enc.Close()
}

// Hex to text: pairs of hexadecimal digits, each pair one byte; white
// space, colons, dashes and commas between them and a 0x before them are
// ignored. The bytes must be UTF-8 text. Another character stops it, as in
// Base64 decode.
func hexDecode(j *job) error {
	d := &decoded{j: j}
	var high byte
	half := false
	var digits int64
	var highAt int64
	for {
		at := j.in.units
		c, err := j.in.byte()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		if isSpace(c) || c == ':' || c == '-' || c == ',' {
			if half {
				return &PositionError{Offset: highAt, Message: "A hexadecimal digit has no partner."}
			}
			continue
		}
		if !half && c == '0' {
			if p := j.in.peek(1); len(p) == 1 && (p[0] == 'x' || p[0] == 'X') {
				j.in.byte()
				continue
			}
		}
		v, ok := hexValue(c)
		if !ok {
			// What a run over the text before c would do at its end.
			return j.notDecoded(at, c, "a hexadecimal digit", !half && digits > 0 && d.done() == nil)
		}
		digits++
		if !half {
			high, highAt, half = v, at, true
			continue
		}
		half = false
		if err = d.add(high<<4|v, highAt); err != nil {
			return err
		}
		if len(d.out) >= 32<<10 {
			if err = j.out.Insert(d.take()); err != nil {
				return err
			}
		}
	}
	if half {
		return &PositionError{Offset: highAt, Message: "The hexadecimal text has an odd number of digits."}
	}
	if digits == 0 {
		return errors.New("There is no hexadecimal text to decode.")
	}
	if err := d.done(); err != nil {
		return err
	}
	return j.out.Insert(d.take())
}
