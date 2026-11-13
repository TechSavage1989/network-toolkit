package texttools

import (
	"bytes"
	"errors"
	"io"
	"unicode/utf8"
)

// A SonicWall settings export (.exp) is one line of Base64. Decoded it is
// ASCII "key=value&key=value&...&", tens of thousands of settings with
// percent-encoded values ("%20" a space, "%3a" a colon); a literal '+' is a
// plus sign, not a space. The readable copy has one "key=value" per line in
// the export's order, values percent-decoded. A character that would break
// the line or cannot be shown is written as a visible escape: \n, \r and
// \xNN (a tab stays a tab), so one setting is always one line. A literal
// backslash stays as it is, so the copy is for reading, never for importing
// back into a firewall.

func init() {
	register(&Tool{ID: "sonicwall-readable", Rewrite: true, run: sonicwallReadable})
}

// readableValue writes decoded bytes for reading.
type readableValue struct {
	w       *writer
	pending []byte
}

func (v *readableValue) hex(b byte) error {
	const digits = "0123456789abcdef"
	return v.w.text([]byte{'\\', 'x', digits[b>>4], digits[b&15]})
}

func (v *readableValue) flushPending() error {
	for _, b := range v.pending {
		if err := v.hex(b); err != nil {
			return err
		}
	}
	v.pending = v.pending[:0]
	return nil
}

func (v *readableValue) char(r rune) error {
	switch {
	case r == '\t':
		return v.w.byte('\t')
	case r == '\n':
		return v.w.text([]byte(`\n`))
	case r == '\r':
		return v.w.text([]byte(`\r`))
	case r < 0x20 || r == 0x7f:
		return v.hex(byte(r))
	case r >= 0x80 && r < 0xa0:
		// C1 control characters are not shown by the editor either.
		for _, b := range []byte(string(r)) {
			if err := v.hex(b); err != nil {
				return err
			}
		}
		return nil
	}
	return v.w.text([]byte(string(r)))
}

func (v *readableValue) add(b byte) error {
	if len(v.pending) > 0 {
		if b&0xc0 == 0x80 {
			v.pending = append(v.pending, b)
			if !utf8.FullRune(v.pending) {
				return nil
			}
			r, size := utf8.DecodeRune(v.pending)
			if r == utf8.RuneError && size <= 1 {
				return v.flushPending()
			}
			v.pending = v.pending[:0]
			return v.char(r)
		}
		if err := v.flushPending(); err != nil {
			return err
		}
	}
	switch {
	case b < 0x80:
		return v.char(rune(b))
	case b >= 0xc2 && b <= 0xf4:
		v.pending = append(v.pending, b)
		return nil
	}
	return v.hex(b)
}

// sonicwallPairs turns the decoded export into lines.
type sonicwallPairs struct {
	value     *readableValue
	w         *writer
	pair      int64 // bytes in the current setting
	inValue   bool
	pct       int // 0, or 1 or 2 hex digits of a %XX read so far
	pctDigits [2]byte
	first     bool // the first setting is still being read
	count     int64
}

func (p *sonicwallPairs) literal(b byte) error { return p.value.add(b) }

func (p *sonicwallPairs) endPct() error {
	if p.pct == 0 {
		return nil
	}
	if err := p.literal('%'); err != nil {
		return err
	}
	if p.pct == 2 {
		if err := p.literal(p.pctDigits[0]); err != nil {
			return err
		}
	}
	p.pct = 0
	return nil
}

func notExport() error {
	return &PositionError{Offset: 0, Message: "This does not look like a SonicWall settings export (Base64 of key=value& settings)."}
}

func (p *sonicwallPairs) add(b byte, _ int64) error {
	if p.first && !p.inValue && b != '=' && !(b >= 'A' && b <= 'Z' || b >= 'a' && b <= 'z' || b >= '0' && b <= '9' || b == '_' || b == '.' || b == '-') {
		return notExport()
	}
	if p.pct > 0 {
		if v, ok := hexValue(b); ok {
			if p.pct == 1 {
				p.pctDigits[0] = b
				p.pct = 2
				return nil
			}
			hi, _ := hexValue(p.pctDigits[0])
			p.pct = 0
			p.pair++
			return p.value.add(hi<<4 | v)
		}
		if err := p.endPct(); err != nil {
			return err
		}
	}
	switch {
	case b == '&':
		return p.endPair()
	case b == '=' && !p.inValue:
		p.inValue = true
		p.first = false
		if err := p.value.flushPending(); err != nil {
			return err
		}
		p.pair++
		return p.w.byte('=')
	case b == '%':
		p.pct = 1
		return nil
	}
	p.pair++
	return p.literal(b)
}

func (p *sonicwallPairs) endPair() error {
	if err := p.endPct(); err != nil {
		return err
	}
	if err := p.value.flushPending(); err != nil {
		return err
	}
	if p.first && p.pair > 0 && !p.inValue {
		return notExport()
	}
	if p.pair > 0 {
		p.count++
		if err := p.w.byte('\n'); err != nil {
			return err
		}
	}
	p.pair, p.inValue, p.first = 0, false, false
	return nil
}

func sonicwallReadable(j *job) error {
	w := &writer{j: j}
	pairs := &sonicwallPairs{value: &readableValue{w: w}, w: w, first: true}
	stream := &base64Stream{emit: pairs.add}
	// A real export ends its Base64 with "&&" (for example "...AA==&&"):
	// after the Base64, only '&' and white space may follow.
	trailer := false
	for {
		at := j.in.units
		c, err := j.in.byte()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		if c == '&' && stream.symbols > 0 {
			trailer = true
			continue
		}
		if trailer && !isSpace(c) {
			return &PositionError{Offset: at, Message: "The export continues after its closing &."}
		}
		if err = stream.feed(c, at); err != nil {
			return err
		}
	}
	if err := stream.flush(); err != nil {
		return err
	}
	if stream.symbols == 0 {
		return errors.New("There is no SonicWall settings export here to make readable.")
	}
	if err := pairs.endPair(); err != nil {
		return err
	}
	if pairs.count == 0 {
		return notExport()
	}
	j.count = pairs.count
	return w.flush()
}

// LooksLikeSonicWall reports whether the start of a document looks like a
// SonicWall settings export: Base64 (and nothing else) whose decoded start
// is key=value& settings, among them buildNum= or shortProdName=. Only the
// prefix it is given is read; the caller passes the first few kilobytes.
func LooksLikeSonicWall(prefix []byte) bool {
	var symbols []byte
	for _, c := range prefix {
		if isSpace(c) {
			continue
		}
		if c == '=' {
			break
		}
		if _, ok := base64Value(c); !ok {
			return false
		}
		symbols = append(symbols, c)
	}
	symbols = symbols[:len(symbols)/4*4]
	if len(symbols) < 16 {
		return false
	}
	var decoded []byte
	stream := &base64Stream{emit: func(b byte, _ int64) error { decoded = append(decoded, b); return nil }}
	for _, c := range symbols {
		if stream.feed(c, 0) != nil {
			return false
		}
	}
	first := bytes.IndexByte(decoded, '&')
	equal := bytes.IndexByte(decoded, '=')
	if first < 0 || equal <= 0 || equal > first {
		return false
	}
	for _, c := range decoded[:equal] {
		if !(c >= 'A' && c <= 'Z' || c >= 'a' && c <= 'z' || c >= '0' && c <= '9' || c == '_') {
			return false
		}
	}
	text := append([]byte{'&'}, decoded...)
	return bytes.Contains(text, []byte("&buildNum=")) || bytes.Contains(text, []byte("&shortProdName="))
}
