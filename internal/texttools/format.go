package texttools

import (
	"bytes"
	"errors"
	"fmt"
	"io"
)

// Pretty-print and minify JSON and XML. Both read their input once, check
// that it is well formed and copy every string, number, tag, comment and
// text exactly as written: only the white space between them changes.
//
// Pretty-printing writes one indent for every level on every line, so a
// small input that nests deeply would give an output that grows with the
// square of its depth (8 KB of brackets, 64 MB of indents). Two bounds keep
// it in proportion: nesting deeper than maxDepth is refused, and a result
// more than maxGrowth times its input (plus growthAllowance) stops the tool.
// Both change nothing. The writer asks the context whether to stop for
// every block it writes, so Cancel and closing a tab stop it promptly
// however little input is left.

// maxDepth is the deepest nesting the formatters take.
const maxDepth = 512

// maxGrowth and growthAllowance bound a pretty-printed result: at most
// maxGrowth times its input plus growthAllowance bytes.
var maxGrowth, growthAllowance int64 = 64, 64 << 20

func init() {
	register(&Tool{ID: "json-pretty", Rewrite: true, run: func(j *job) error { return formatJSON(j, true) }})
	register(&Tool{ID: "json-minify", Rewrite: true, run: func(j *job) error { return formatJSON(j, false) }})
	register(&Tool{ID: "xml-pretty", Rewrite: true, run: func(j *job) error { return formatXML(j, true) }})
	register(&Tool{ID: "xml-minify", Rewrite: true, run: func(j *job) error { return formatXML(j, false) }})
}

// writer collects output and passes it on in blocks.
type writer struct {
	j   *job
	buf []byte
	// written counts what was passed on; limit, when set, bounds it.
	written, limit int64
}

func (w *writer) byte(c byte) error {
	w.buf = append(w.buf, c)
	if len(w.buf) >= 32<<10 {
		return w.flush()
	}
	return nil
}

func (w *writer) text(s []byte) error {
	w.buf = append(w.buf, s...)
	if len(w.buf) >= 32<<10 {
		return w.flush()
	}
	return nil
}

func (w *writer) flush() error {
	if len(w.buf) == 0 {
		return nil
	}
	if err := w.j.ctx.Err(); err != nil {
		return err
	}
	w.written += int64(len(w.buf))
	if w.limit > 0 && w.written > w.limit {
		return fmt.Errorf("Pretty-printing would make this text more than %d times larger, so it was not formatted.", maxGrowth)
	}
	err := w.j.out.Insert(w.buf)
	w.buf = w.buf[:0]
	return err
}

// grows bounds a pretty-printed result (see maxGrowth).
func (w *writer) grows() *writer {
	w.limit = maxGrowth*w.j.size + growthAllowance
	return w
}

func (j *job) indentUnit() []byte {
	if j.opts.Indent <= 0 {
		return []byte{'\t'}
	}
	return bytes.Repeat([]byte{' '}, j.opts.Indent)
}

func (w *writer) newline(unit []byte, depth int) error {
	if err := w.byte('\n'); err != nil {
		return err
	}
	for i := 0; i < depth; i++ {
		if err := w.text(unit); err != nil {
			return err
		}
	}
	return nil
}

// ---- JSON ----

type jsonScan struct {
	j    *job
	w    *writer
	last byte
}

func (s *jsonScan) read() (byte, int64, error) {
	at := s.j.in.units
	c, err := s.j.in.byte()
	if err == nil {
		s.last = c
	}
	return c, at, err
}

func (s *jsonScan) peek() (byte, bool) {
	p := s.j.in.peek(1)
	if len(p) == 0 {
		return 0, false
	}
	return p[0], true
}

func jsonSpace(c byte) bool { return c == ' ' || c == '\t' || c == '\n' || c == '\r' }

func (s *jsonScan) skip() {
	for {
		c, ok := s.peek()
		if !ok || !jsonSpace(c) {
			return
		}
		s.read()
	}
}

func (s *jsonScan) fail(at int64, format string, args ...any) error {
	return &PositionError{Offset: at, Message: fmt.Sprintf(format, args...)}
}

func (s *jsonScan) eof() error {
	return &PositionError{Offset: s.j.in.units, Message: "The JSON ends before it is complete."}
}

// str copies a string, from its opening quote (already read) to its closing one.
func (s *jsonScan) str() error {
	if err := s.w.byte('"'); err != nil {
		return err
	}
	for {
		c, at, err := s.read()
		if err == io.EOF {
			return s.eof()
		}
		if err != nil {
			return err
		}
		if c < 0x20 {
			return s.fail(at, "A control character in a JSON string must be written as an escape such as \\n or \\t.")
		}
		if err = s.w.byte(c); err != nil {
			return err
		}
		switch c {
		case '"':
			return nil
		case '\\':
			e, at, err := s.read()
			if err == io.EOF {
				return s.eof()
			}
			if err != nil {
				return err
			}
			if err = s.w.byte(e); err != nil {
				return err
			}
			switch e {
			case '"', '\\', '/', 'b', 'f', 'n', 'r', 't':
			case 'u':
				for i := 0; i < 4; i++ {
					h, hat, err := s.read()
					if err == io.EOF {
						return s.eof()
					}
					if err != nil {
						return err
					}
					if _, ok := hexValue(h); !ok {
						return s.fail(hat, "\\u must be followed by four hexadecimal digits.")
					}
					if err = s.w.byte(h); err != nil {
						return err
					}
				}
			default:
				return s.fail(at-1, "\\%c is not a JSON escape.", e)
			}
		}
	}
}

// number copies a number whose first character was already read.
func (s *jsonScan) number(first byte, at int64) error {
	if err := s.w.byte(first); err != nil {
		return err
	}
	digits := func(min int) (int, error) {
		n := 0
		for {
			c, ok := s.peek()
			if !ok || c < '0' || c > '9' {
				if n < min {
					return n, s.fail(s.j.in.units, "A JSON number is incomplete.")
				}
				return n, nil
			}
			s.read()
			n++
			if err := s.w.byte(c); err != nil {
				return n, err
			}
		}
	}
	c := first
	if c == '-' {
		next, nat, err := s.read()
		if err == io.EOF {
			return s.eof()
		}
		if err != nil {
			return err
		}
		if next < '0' || next > '9' {
			return s.fail(nat, "A JSON number is incomplete.")
		}
		if err = s.w.byte(next); err != nil {
			return err
		}
		c = next
	}
	if c != '0' {
		if _, err := digits(0); err != nil {
			return err
		}
	} else if p, ok := s.peek(); ok && p >= '0' && p <= '9' {
		return s.fail(at, "A JSON number cannot start with 0 followed by more digits.")
	}
	if p, ok := s.peek(); ok && p == '.' {
		s.read()
		if err := s.w.byte('.'); err != nil {
			return err
		}
		if _, err := digits(1); err != nil {
			return err
		}
	}
	if p, ok := s.peek(); ok && (p == 'e' || p == 'E') {
		s.read()
		if err := s.w.byte(p); err != nil {
			return err
		}
		if q, ok := s.peek(); ok && (q == '+' || q == '-') {
			s.read()
			if err := s.w.byte(q); err != nil {
				return err
			}
		}
		if _, err := digits(1); err != nil {
			return err
		}
	}
	return nil
}

func formatJSON(j *job, pretty bool) error {
	s := &jsonScan{j: j, w: &writer{j: j}}
	if pretty {
		s.w.grows()
	}
	unit := j.indentUnit()
	var stack []byte
	const (
		value = iota
		key
		after
	)
	mode := value
	s.skip()
	if _, ok := s.peek(); !ok {
		return errors.New("There is no JSON here to format.")
	}
	newline := func() error {
		if !pretty {
			return nil
		}
		return s.w.newline(unit, len(stack))
	}
	for {
		switch mode {
		case value:
			s.skip()
			c, at, err := s.read()
			if err == io.EOF {
				return s.eof()
			}
			if err != nil {
				return err
			}
			switch {
			case c == '{' || c == '[':
				closer := byte('}')
				if c == '[' {
					closer = ']'
				}
				s.skip()
				if p, ok := s.peek(); ok && p == closer {
					s.read()
					if err = s.w.text([]byte{c, closer}); err != nil {
						return err
					}
					mode = after
					continue
				}
				if len(stack) >= maxDepth {
					return s.fail(at, "The JSON nests deeper than %d levels, more than the formatter takes.", maxDepth)
				}
				if err = s.w.byte(c); err != nil {
					return err
				}
				stack = append(stack, c)
				if err = newline(); err != nil {
					return err
				}
				mode = value
				if c == '{' {
					mode = key
				}
			case c == '"':
				if err = s.str(); err != nil {
					return err
				}
				mode = after
			case c == '-' || c >= '0' && c <= '9':
				if err = s.number(c, at); err != nil {
					return err
				}
				mode = after
			case c == 't' || c == 'f' || c == 'n':
				word := map[byte]string{'t': "true", 'f': "false", 'n': "null"}[c]
				for i := 1; i < len(word); i++ {
					d, _, err := s.read()
					if err == io.EOF {
						return s.eof()
					}
					if err != nil {
						return err
					}
					if d != word[i] {
						return s.fail(at, "Expected %s.", word)
					}
				}
				if err = s.w.text([]byte(word)); err != nil {
					return err
				}
				mode = after
			default:
				return s.fail(at, "Expected a JSON value (an object, array, string, number, true, false or null).")
			}
		case key:
			s.skip()
			c, at, err := s.read()
			if err == io.EOF {
				return s.eof()
			}
			if err != nil {
				return err
			}
			if c != '"' {
				return s.fail(at, "Expected a name in double quotes.")
			}
			if err = s.str(); err != nil {
				return err
			}
			s.skip()
			c, at, err = s.read()
			if err == io.EOF {
				return s.eof()
			}
			if err != nil {
				return err
			}
			if c != ':' {
				return s.fail(at, "Expected a colon after the name.")
			}
			sep := []byte(":")
			if pretty {
				sep = []byte(": ")
			}
			if err = s.w.text(sep); err != nil {
				return err
			}
			mode = value
		case after:
			if len(stack) == 0 {
				s.skip()
				if _, ok := s.peek(); ok {
					return s.fail(s.j.in.units, "There is more text after the end of the JSON value.")
				}
				if s.last == '\n' {
					if err := s.w.byte('\n'); err != nil {
						return err
					}
				}
				return s.w.flush()
			}
			s.skip()
			c, at, err := s.read()
			if err == io.EOF {
				return s.eof()
			}
			if err != nil {
				return err
			}
			top := stack[len(stack)-1]
			closer := byte('}')
			if top == '[' {
				closer = ']'
			}
			switch c {
			case ',':
				if err = s.w.byte(','); err != nil {
					return err
				}
				if err = newline(); err != nil {
					return err
				}
				mode = value
				if top == '{' {
					mode = key
				}
			case closer:
				stack = stack[:len(stack)-1]
				if err = newline(); err != nil {
					return err
				}
				if err = s.w.byte(closer); err != nil {
					return err
				}
			default:
				return s.fail(at, "Expected a comma or %c.", closer)
			}
		}
	}
}

// ---- XML ----

type xmlOpen struct {
	name     string
	at       int64
	hasChild bool
}

type xmlFormat struct {
	j      *job
	w      *writer
	pretty bool
	unit   []byte
	stack  []xmlOpen
	// last is what was written last: 0 nothing, 'o' a start tag, 't' text,
	// 'c' anything that ends a line (an end tag, a comment, an empty element).
	last     byte
	elements int
	lastByte byte
}

func (x *xmlFormat) read() (byte, error) {
	c, err := x.j.in.byte()
	if err == nil {
		x.lastByte = c
	}
	return c, err
}

func (x *xmlFormat) fail(at int64, format string, args ...any) error {
	return &PositionError{Offset: at, Message: fmt.Sprintf(format, args...)}
}

// place starts a line for a block (anything but text) in pretty-print.
func (x *xmlFormat) place() error {
	if len(x.stack) > 0 {
		x.stack[len(x.stack)-1].hasChild = true
	}
	if !x.pretty || x.last == 0 {
		return nil
	}
	return x.w.newline(x.unit, len(x.stack))
}

// copyUntil copies bytes up to and including the terminator.
func (x *xmlFormat) copyUntil(at int64, end string, what string) error {
	var tail [3]byte
	n := 0
	for {
		c, err := x.read()
		if err == io.EOF {
			return x.fail(at, "This %s is not closed.", what)
		}
		if err != nil {
			return err
		}
		if err = x.w.byte(c); err != nil {
			return err
		}
		copy(tail[:], tail[1:])
		tail[2] = c
		if n++; n >= len(end) && string(tail[3-len(end):]) == end {
			return nil
		}
	}
}

// tag copies a start, empty or end tag up to its '>'; quotes may hold '>'.
func (x *xmlFormat) tag(at int64) (name string, empty bool, err error) {
	var nameBuf []byte
	inName, quote := true, byte(0)
	var prev byte
	for {
		c, err := x.read()
		if err == io.EOF {
			return "", false, x.fail(at, "This tag is not closed with >.")
		}
		if err != nil {
			return "", false, err
		}
		if err = x.w.byte(c); err != nil {
			return "", false, err
		}
		switch {
		case quote != 0:
			if c == quote {
				quote = 0
			}
		case c == '"' || c == '\'':
			quote = c
			inName = false
		case c == '>':
			return string(nameBuf), prev == '/', nil
		case c == '<':
			return "", false, x.fail(at, "A tag contains a <.")
		case isSpace(c) || c == '/':
			inName = false
		default:
			if inName {
				nameBuf = append(nameBuf, c)
			}
		}
		prev = c
	}
}

func formatXML(j *job, pretty bool) error {
	x := &xmlFormat{j: j, w: &writer{j: j}, pretty: pretty, unit: j.indentUnit()}
	if pretty {
		x.w.grows()
	}
	var ws []byte
	inText := false
	text := func(c byte, at int64) error {
		if !inText {
			if isSpace(c) {
				ws = append(ws, c)
				return nil
			}
			if len(x.stack) == 0 {
				return x.fail(at, "There is text outside the XML elements.")
			}
			inText = true
			if x.pretty && x.last != 'o' && x.last != 't' {
				// Text after a child element starts its own line; the white
				// space before it was formatting.
				if err := x.w.newline(x.unit, len(x.stack)); err != nil {
					return err
				}
			} else if err := x.w.text(ws); err != nil {
				return err
			}
			ws = ws[:0]
			x.last = 't'
		}
		return x.w.byte(c)
	}
	for {
		at := j.in.units
		c, err := x.read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		if c != '<' {
			if err = text(c, at); err != nil {
				return err
			}
			continue
		}
		// White space only between two tags is formatting: it goes.
		inText = false
		ws = ws[:0]
		p := j.in.peek(9)
		switch {
		case bytes.HasPrefix(p, []byte("!--")):
			if err = x.place(); err != nil {
				return err
			}
			if err = x.w.byte('<'); err != nil {
				return err
			}
			if err = x.copyUntil(at, "-->", "comment"); err != nil {
				return err
			}
			x.last = 'c'
		case bytes.HasPrefix(p, []byte("![CDATA[")):
			if len(x.stack) == 0 {
				return x.fail(at, "CDATA must be inside an element.")
			}
			if x.pretty && x.last != 'o' && x.last != 't' {
				if err = x.w.newline(x.unit, len(x.stack)); err != nil {
					return err
				}
			}
			if err = x.w.byte('<'); err != nil {
				return err
			}
			if err = x.copyUntil(at, "]]>", "CDATA section"); err != nil {
				return err
			}
			x.last = 't'
		case len(p) > 0 && p[0] == '?':
			if err = x.place(); err != nil {
				return err
			}
			if err = x.w.byte('<'); err != nil {
				return err
			}
			if err = x.copyUntil(at, "?>", "processing instruction"); err != nil {
				return err
			}
			x.last = 'c'
		case len(p) > 0 && p[0] == '!':
			if err = x.place(); err != nil {
				return err
			}
			if err = x.w.byte('<'); err != nil {
				return err
			}
			if err = x.declaration(at); err != nil {
				return err
			}
			x.last = 'c'
		case len(p) > 0 && p[0] == '/':
			if len(x.stack) == 0 {
				return x.fail(at, "This end tag closes no element.")
			}
			open := x.stack[len(x.stack)-1]
			x.stack = x.stack[:len(x.stack)-1]
			if x.pretty && open.hasChild {
				if err = x.w.newline(x.unit, len(x.stack)); err != nil {
					return err
				}
			}
			x.read()
			if err = x.w.text([]byte("</")); err != nil {
				return err
			}
			name, _, err := x.tag(at)
			if err != nil {
				return err
			}
			if name != open.name {
				return x.fail(at, "</%s> does not close <%s>.", name, open.name)
			}
			x.last = 'c'
		default:
			if err = x.place(); err != nil {
				return err
			}
			if err = x.w.byte('<'); err != nil {
				return err
			}
			name, empty, err := x.tag(at)
			if err != nil {
				return err
			}
			if name == "" {
				return x.fail(at, "A tag has no name.")
			}
			x.elements++
			if empty {
				x.last = 'c'
			} else {
				if len(x.stack) >= maxDepth {
					return x.fail(at, "The XML nests deeper than %d levels, more than the formatter takes.", maxDepth)
				}
				x.stack = append(x.stack, xmlOpen{name: name, at: at})
				x.last = 'o'
			}
		}
	}
	if len(x.stack) > 0 {
		open := x.stack[len(x.stack)-1]
		return x.fail(open.at, "<%s> is not closed.", open.name)
	}
	if x.elements == 0 {
		return errors.New("There is no XML element here to format.")
	}
	if x.lastByte == '\n' {
		if err := x.w.byte('\n'); err != nil {
			return err
		}
	}
	return x.w.flush()
}

// declaration copies <!DOCTYPE ...> and other declarations, whose internal
// subset in [ ] may hold '>'.
func (x *xmlFormat) declaration(at int64) error {
	depth, quote := 0, byte(0)
	for {
		c, err := x.read()
		if err == io.EOF {
			return x.fail(at, "This declaration is not closed with >.")
		}
		if err != nil {
			return err
		}
		if err = x.w.byte(c); err != nil {
			return err
		}
		switch {
		case quote != 0:
			if c == quote {
				quote = 0
			}
		case c == '"' || c == '\'':
			quote = c
		case c == '[':
			depth++
		case c == ']':
			depth--
		case c == '>' && depth <= 0:
			return nil
		}
	}
}
