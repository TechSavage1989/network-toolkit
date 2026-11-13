package texttools

import (
	"html"
	"io"
	"strconv"
	"unicode"
	"unicode/utf8"
)

// Tools that change characters where they stand. Each character either stays
// (Keep) or is replaced (Drop and Insert); line breaks they do not touch
// stay the line breaks that were read.

func init() {
	register(&Tool{ID: "upper", run: mapRunes(unicode.ToUpper)})
	register(&Tool{ID: "lower", run: mapRunes(unicode.ToLower)})
	register(&Tool{ID: "title", run: titleCase})
	register(&Tool{ID: "url-decode", run: urlDecode(false)})
	register(&Tool{ID: "url-decode-form", run: urlDecode(true)})
	register(&Tool{ID: "url-encode", run: urlEncode})
	register(&Tool{ID: "html-decode", run: htmlDecode})
	register(&Tool{ID: "html-encode", run: htmlEncode})
	register(&Tool{ID: "unescape", run: unescape})
	register(&Tool{ID: "escape", run: escape})
	register(&Tool{ID: "hex-encode", run: hexEncode})
}

// each calls fn for every character of the input.
func (j *job) each(fn func(c rune, size int) error) error {
	for {
		c, size, err := j.in.next()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		if err = fn(c, size); err != nil {
			return err
		}
	}
}

// replace drops the size bytes just read and inserts text in their place.
func (j *job) replace(size int, text string) error {
	if err := j.out.Drop(int64(size)); err != nil {
		return err
	}
	return j.out.InsertString(text)
}

func mapRunes(fn func(rune) rune) func(*job) error {
	return func(j *job) error {
		return j.each(func(c rune, size int) error {
			if m := fn(c); m != c {
				j.count++
				return j.replace(size, string(m))
			}
			return j.out.Keep(int64(size))
		})
	}
}

// Title Case: the first letter of every word in title case, the rest in
// lower case. A word is a run of letters, digits, marks and apostrophes, so
// "don't" stays one word and "e-mail" becomes "E-Mail".
func titleCase(j *job) error {
	inWord := false
	return j.each(func(c rune, size int) error {
		word := unicode.IsLetter(c) || unicode.IsDigit(c) || unicode.IsMark(c) || (inWord && (c == '\'' || c == '’'))
		m := c
		if word && !inWord {
			m = unicode.ToTitle(c)
		} else if word {
			m = unicode.ToLower(c)
		}
		inWord = word
		if m != c {
			j.count++
			return j.replace(size, string(m))
		}
		return j.out.Keep(int64(size))
	})
}

func hexValue(c byte) (byte, bool) {
	switch {
	case c >= '0' && c <= '9':
		return c - '0', true
	case c >= 'a' && c <= 'f':
		return c - 'a' + 10, true
	case c >= 'A' && c <= 'F':
		return c - 'A' + 10, true
	}
	return 0, false
}

// decoded collects the bytes a decoder produces and turns them into
// characters the editor can hold: they must be UTF-8, a carriage return
// becomes a line break ("\r\n" one break), and a NUL character is refused.
type decoded struct {
	j       *job
	pending []byte // bytes of a character not complete yet
	start   int64  // UTF-16 offset in the input where pending began
	cr      bool   // the last character was a carriage return
	out     []byte
}

func (d *decoded) add(c byte, at int64) error {
	if len(d.pending) == 0 {
		d.start = at
	}
	d.pending = append(d.pending, c)
	if !utf8.FullRune(d.pending) {
		return nil
	}
	r, size := utf8.DecodeRune(d.pending)
	if r == utf8.RuneError && size <= 1 {
		return &PositionError{Offset: d.start, Message: "The decoded bytes are not UTF-8 text."}
	}
	d.pending = d.pending[:0]
	return d.char(r, at)
}

func (d *decoded) char(r rune, at int64) error {
	switch {
	case r == 0:
		return &PositionError{Offset: at, Message: "The decoded text contains a NUL character, which the Text Editor cannot hold."}
	case r == '\n' && d.cr:
		d.cr = false
		return nil
	case r == '\r':
		d.cr = true
		d.out = append(d.out, '\n')
		return nil
	}
	d.cr = false
	d.out = utf8.AppendRune(d.out, r)
	return nil
}

// done reports an unfinished character.
func (d *decoded) done() error {
	if len(d.pending) > 0 {
		return &PositionError{Offset: d.start, Message: "The decoded bytes are not UTF-8 text."}
	}
	return nil
}

// take returns the characters decoded so far.
func (d *decoded) take() []byte {
	p := d.out
	d.out = d.out[:0]
	return p
}

// URL decode: %XX becomes the byte it names. With form data a '+' is a space;
// otherwise it stays a '+' (a SonicWall export keeps literal plus signs).
func urlDecode(form bool) func(*job) error {
	return func(j *job) error {
		d := &decoded{j: j}
		var span int64 // input bytes of the escapes being decoded
		flush := func() error {
			if span == 0 {
				return nil
			}
			if err := d.done(); err != nil {
				return err
			}
			if err := j.out.Drop(span); err != nil {
				return err
			}
			span = 0
			return j.out.Insert(d.take())
		}
		for {
			start := j.in.units
			c, err := j.in.byte()
			if err == io.EOF {
				return flush()
			}
			if err != nil {
				return err
			}
			switch {
			case c == '%':
				p := j.in.peek(2)
				hi, ok1 := byte(0), false
				lo, ok2 := byte(0), false
				if len(p) == 2 {
					hi, ok1 = hexValue(p[0])
					lo, ok2 = hexValue(p[1])
				}
				if !ok1 || !ok2 {
					return &PositionError{Offset: start, Message: "A % is not followed by two hexadecimal digits."}
				}
				j.in.byte()
				j.in.byte()
				span += 3
				j.count++
				if err = d.add(hi<<4|lo, start); err != nil {
					return err
				}
			case form && c == '+':
				if len(d.pending) > 0 {
					return d.done()
				}
				span++
				j.count++
				if err = d.char(' ', start); err != nil {
					return err
				}
			case c == '\n' && d.cr && len(d.pending) == 0:
				// "%0D" followed by a line break is one line break.
				span++
				d.cr = false
			default:
				if err = flush(); err != nil {
					return err
				}
				d.cr = false
				if err = j.out.Keep(1); err != nil {
					return err
				}
			}
		}
	}
}

// URL encode: every byte but the unreserved characters of RFC 3986
// (A-Z a-z 0-9 - _ . ~) becomes %XX.
func urlEncode(j *job) error {
	const digits = "0123456789ABCDEF"
	return j.each(func(c rune, size int) error {
		if c < 0x80 && (c >= 'A' && c <= 'Z' || c >= 'a' && c <= 'z' || c >= '0' && c <= '9' || c == '-' || c == '_' || c == '.' || c == '~') {
			return j.out.Keep(int64(size))
		}
		var buf [utf8.UTFMax * 3]byte
		out := buf[:0]
		for _, b := range []byte(string(c)) {
			out = append(out, '%', digits[b>>4], digits[b&15])
		}
		j.count++
		return j.replace(size, string(out))
	})
}

// HTML entities decode: named entities (&amp;, &nbsp;, &eacute; ...) and
// numeric ones (&#38;, &#x26;) that end with a semicolon. Anything else
// stays as written.
func htmlDecode(j *job) error {
	d := &decoded{j: j}
	for {
		c, err := j.in.byte()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		if c == '\n' && d.cr {
			// "&#13;" followed by a line break is one line break.
			d.cr = false
			if err = j.out.Drop(1); err != nil {
				return err
			}
			continue
		}
		d.cr = false
		if c != '&' {
			if err = j.out.Keep(1); err != nil {
				return err
			}
			continue
		}
		p := j.in.peek(40)
		end := -1
		for i, b := range p {
			if b == ';' {
				end = i
				break
			}
			if !(b == '#' && i == 0 || b >= '0' && b <= '9' || b >= 'a' && b <= 'z' || b >= 'A' && b <= 'Z') {
				break
			}
		}
		if end <= 0 {
			if err = j.out.Keep(1); err != nil {
				return err
			}
			continue
		}
		entity := "&" + string(p[:end+1])
		text := html.UnescapeString(entity)
		if text == entity {
			if err = j.out.Keep(1); err != nil {
				return err
			}
			continue
		}
		for i := 0; i <= end; i++ {
			j.in.byte()
		}
		for _, r := range text {
			if err = d.char(r, j.in.units); err != nil {
				return err
			}
		}
		j.count++
		if err = j.out.Drop(int64(len(entity))); err != nil {
			return err
		}
		if err = j.out.Insert(d.take()); err != nil {
			return err
		}
	}
}

func htmlEncode(j *job) error {
	return j.each(func(c rune, size int) error {
		var e string
		switch c {
		case '&':
			e = "&amp;"
		case '<':
			e = "&lt;"
		case '>':
			e = "&gt;"
		case '"':
			e = "&quot;"
		case '\'':
			e = "&#39;"
		default:
			return j.out.Keep(int64(size))
		}
		j.count++
		return j.replace(size, e)
	})
}

// Unescape: \n \t \r \" \' \\ \/ \b \f \v \a, \xNN (bytes that make UTF-8
// text) and \uNNNN (with surrogate pairs). Another character after a
// backslash stays as written. \0 is refused: the editor cannot hold NUL.
func unescape(j *job) error {
	d := &decoded{j: j}
	var span int64
	flush := func() error {
		if span == 0 {
			return nil
		}
		if err := d.done(); err != nil {
			return err
		}
		if err := j.out.Drop(span); err != nil {
			return err
		}
		span = 0
		return j.out.Insert(d.take())
	}
	for {
		start := j.in.units
		c, err := j.in.byte()
		if err == io.EOF {
			return flush()
		}
		if err != nil {
			return err
		}
		if c != '\\' {
			if len(d.pending) > 0 {
				return d.done()
			}
			if c == '\n' && d.cr {
				span++
				d.cr = false
				continue
			}
			if err = flush(); err != nil {
				return err
			}
			d.cr = false
			if err = j.out.Keep(1); err != nil {
				return err
			}
			continue
		}
		p := j.in.peek(1)
		if len(p) == 0 {
			if err = flush(); err != nil {
				return err
			}
			return j.out.Keep(1)
		}
		if r, ok := simpleEscapes[p[0]]; ok {
			if len(d.pending) > 0 {
				return d.done()
			}
			j.in.byte()
			span += 2
			j.count++
			if err = d.char(r, start); err != nil {
				return err
			}
			continue
		}
		switch p[0] {
		case '0':
			return &PositionError{Offset: start, Message: "\\0 would add a NUL character, which the Text Editor cannot hold."}
		case 'x':
			q := j.in.peek(3)
			hi, ok1 := byte(0), false
			lo, ok2 := byte(0), false
			if len(q) == 3 {
				hi, ok1 = hexValue(q[1])
				lo, ok2 = hexValue(q[2])
			}
			if ok1 && ok2 {
				j.in.byte()
				j.in.byte()
				j.in.byte()
				span += 4
				j.count++
				if err = d.add(hi<<4|lo, start); err != nil {
					return err
				}
				continue
			}
		case 'u':
			if r, n, ok := unicodeEscape(j.in.peek(13)); ok {
				if len(d.pending) > 0 {
					return d.done()
				}
				for i := 0; i < n; i++ {
					j.in.byte()
				}
				span += int64(n) + 1
				j.count++
				if err = d.char(r, start); err != nil {
					return err
				}
				continue
			}
		}
		// Not an escape this tool knows: the backslash stays.
		if len(d.pending) > 0 {
			return d.done()
		}
		if err = flush(); err != nil {
			return err
		}
		d.cr = false
		if err = j.out.Keep(1); err != nil {
			return err
		}
	}
}

// unicodeEscape reads "uNNNN" (and a following "\uNNNN" low surrogate) from
// p, which starts at the 'u'. It returns the character and the bytes used.
func unicodeEscape(p []byte) (rune, int, bool) {
	hex4 := func(q []byte) (rune, bool) {
		if len(q) < 4 {
			return 0, false
		}
		v, err := strconv.ParseUint(string(q[:4]), 16, 32)
		return rune(v), err == nil
	}
	if len(p) < 5 {
		return 0, 0, false
	}
	r, ok := hex4(p[1:])
	if !ok {
		return 0, 0, false
	}
	if r >= 0xd800 && r < 0xdc00 && len(p) >= 11 && p[5] == '\\' && p[6] == 'u' {
		if low, ok := hex4(p[7:]); ok && low >= 0xdc00 && low < 0xe000 {
			return 0x10000 + (r-0xd800)<<10 + (low - 0xdc00), 11, true
		}
	}
	if r >= 0xd800 && r < 0xe000 {
		return 0, 0, false
	}
	return r, 5, true
}

// Escape: a backslash, a line break, a tab and a double quote become \\ \n
// \t \"; other control characters become \xNN.
func escape(j *job) error {
	const digits = "0123456789abcdef"
	return j.each(func(c rune, size int) error {
		var e string
		switch {
		case c == '\\':
			e = `\\`
		case c == '\n':
			e = `\n`
		case c == '\t':
			e = `\t`
		case c == '"':
			e = `\"`
		case c < 0x20 || c == 0x7f:
			e = `\x` + string([]byte{digits[c>>4], digits[c&15]})
		default:
			return j.out.Keep(int64(size))
		}
		j.count++
		return j.replace(size, e)
	})
}

// Text to hex: the UTF-8 bytes of the text as pairs of lower-case digits.
func hexEncode(j *job) error {
	const digits = "0123456789abcdef"
	return j.each(func(c rune, size int) error {
		var buf [8]byte
		out := buf[:0]
		for _, b := range []byte(string(c)) {
			out = append(out, digits[b>>4], digits[b&15])
		}
		j.count++
		return j.replace(size, string(out))
	})
}

var simpleEscapes = map[byte]rune{'n': '\n', 't': '\t', 'r': '\r', '"': '"', '\'': '\'', '\\': '\\', '/': '/', 'b': '\b', 'f': '\f', 'v': '\v', 'a': '\a'}
