package texttools

import (
	"bufio"
	"bytes"
	"errors"
	"io"
	"unicode"
	"unicode/utf8"
)

// Line tools. A line is the text up to a '\n'. The last line of the input
// has no '\n' after it unless the input ends with one: that final line break
// belongs to the last line, so a sorted, reversed or filtered result ends
// with a line break exactly when its input did.

func init() {
	register(&Tool{ID: "trim-trailing", Lines: true, run: trimSpaces(false)})
	register(&Tool{ID: "trim", Lines: true, run: trimSpaces(true)})
	register(&Tool{ID: "remove-empty", Lines: true, run: removeEmpty})
	register(&Tool{ID: "split", run: split})
	register(&Tool{ID: "join", Lines: true, run: join})
	register(&Tool{ID: "tabs-to-spaces", Lines: true, run: tabsToSpaces})
	register(&Tool{ID: "spaces-to-tabs", Lines: true, run: spacesToTabs})
	register(&Tool{ID: "sort-asc", Lines: true, Rewrite: true, run: sortLines(false)})
	register(&Tool{ID: "sort-desc", Lines: true, Rewrite: true, run: sortLines(true)})
	register(&Tool{ID: "reverse", Lines: true, Rewrite: true, run: reverseLines})
	register(&Tool{ID: "dedupe", Lines: true, run: dedupe})
}

// eachByte calls fn for every byte of the input. Line tools look only at
// ASCII bytes (spaces, tabs, line breaks); every other byte is kept or
// dropped together with its character.
func (j *job) eachByte(fn func(c byte) error) error {
	for {
		c, err := j.in.byte()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		if err = fn(c); err != nil {
			return err
		}
	}
}

// line reads the next line without its '\n' and says whether a '\n' ended
// it. It returns io.EOF when no line is left.
func (r *reader) line() ([]byte, bool, error) {
	var out []byte
	started := false
	for {
		chunk, err := r.r.ReadSlice('\n')
		r.off += int64(len(chunk))
		for _, c := range chunk {
			if c < 0x80 || c >= 0xc0 {
				r.units++
				if c >= 0xf0 {
					r.units++
				}
			}
		}
		if e := r.tick(); e != nil {
			return nil, false, e
		}
		switch {
		case err == bufio.ErrBufferFull:
			out = append(out, chunk...)
			started = true
		case err == io.EOF:
			if !started && len(chunk) == 0 {
				return nil, false, io.EOF
			}
			return append(out, chunk...), false, nil
		case err != nil:
			return nil, false, err
		default:
			return append(out, chunk[:len(chunk)-1]...), true, nil
		}
	}
}

// Trim trailing spaces (and with leading, leading spaces too): spaces and
// tabs at the end (and the start) of each line go.
func trimSpaces(leading bool) func(*job) error {
	return func(j *job) error {
		var ws int64
		atStart, trimmed := true, false
		endLine := func() error {
			if ws > 0 {
				trimmed = true
				if err := j.out.Drop(ws); err != nil {
					return err
				}
				ws = 0
			}
			if trimmed {
				j.count++
			}
			atStart, trimmed = true, false
			return nil
		}
		err := j.eachByte(func(c byte) error {
			switch c {
			case ' ', '\t':
				if leading && atStart {
					trimmed = true
					return j.out.Drop(1)
				}
				ws++
				return nil
			case '\n':
				if err := endLine(); err != nil {
					return err
				}
				return j.out.Keep(1)
			}
			atStart = false
			if err := j.out.Keep(ws); err != nil {
				return err
			}
			ws = 0
			return j.out.Keep(1)
		})
		if err != nil {
			return err
		}
		return endLine()
	}
}

// Remove empty lines: lines that are empty or hold only spaces and tabs.
// The '\n' that ends the last kept line is held back until the next kept
// line appears: when the input ends in removed lines without a final line
// break, that '\n' goes too, so no empty line is left where the input ended.
func removeEmpty(j *job) error {
	var lineWS, heldDrop int64
	blank, held := true, false
	err := j.eachByte(func(c byte) error {
		if !blank {
			if c == '\n' {
				held, heldDrop, blank = true, 0, true
				return nil
			}
			return j.out.Keep(1)
		}
		switch c {
		case ' ', '\t':
			lineWS++
			return nil
		case '\n':
			j.count++
			if held {
				heldDrop += lineWS + 1
			} else if err := j.out.Drop(lineWS + 1); err != nil {
				return err
			}
			lineWS = 0
			return nil
		}
		if held {
			if err := j.out.Keep(1); err != nil {
				return err
			}
			if err := j.out.Drop(heldDrop); err != nil {
				return err
			}
			held, heldDrop = false, 0
		}
		if err := j.out.Keep(lineWS + 1); err != nil {
			return err
		}
		lineWS, blank = 0, false
		return nil
	})
	if err != nil {
		return err
	}
	switch {
	case blank && lineWS > 0:
		j.count++
		if held {
			return j.out.Drop(1 + heldDrop + lineWS)
		}
		return j.out.Drop(lineWS)
	case held:
		if err := j.out.Keep(1); err != nil {
			return err
		}
		return j.out.Drop(heldDrop)
	}
	return nil
}

// Split at text: every occurrence of the separator ends a line. The
// separator goes, or stays at the end of the line with KeepSeparator.
// Matches are found with Knuth-Morris-Pratt over the bytes, so the input is
// read once and only a part of one separator is ever held.
func split(j *job) error {
	sep := []byte(j.opts.Separator)
	if len(sep) == 0 {
		return errors.New("Enter the text to split at.")
	}
	if !utf8.Valid(sep) || bytes.IndexByte(sep, 0) >= 0 || bytes.IndexByte(sep, '\r') >= 0 {
		return errors.New("The text to split at cannot hold NUL or carriage return characters.")
	}
	fail := make([]int, len(sep))
	for i, k := 1, 0; i < len(sep); i++ {
		for k > 0 && sep[i] != sep[k] {
			k = fail[k-1]
		}
		if sep[i] == sep[k] {
			k++
		}
		fail[i] = k
	}
	q := 0
	err := j.eachByte(func(c byte) error {
		for q > 0 && sep[q] != c {
			next := fail[q-1]
			if err := j.out.Keep(int64(q - next)); err != nil {
				return err
			}
			q = next
		}
		if sep[q] != c {
			return j.out.Keep(1)
		}
		q++
		if q < len(sep) {
			return nil
		}
		q = 0
		j.count++
		var err error
		if j.opts.KeepSeparator {
			err = j.out.Keep(int64(len(sep)))
		} else {
			err = j.out.Drop(int64(len(sep)))
		}
		if err != nil {
			return err
		}
		return j.out.InsertString("\n")
	})
	if err != nil {
		return err
	}
	return j.out.Keep(int64(q))
}

// Join lines: every line break but a final one becomes the separator.
func join(j *job) error {
	if bytes.ContainsAny([]byte(j.opts.Separator), "\x00\r") || !utf8.ValidString(j.opts.Separator) {
		return errors.New("The separator cannot hold NUL or carriage return characters.")
	}
	held := false
	err := j.eachByte(func(c byte) error {
		if held {
			held = false
			j.count++
			if err := j.out.Drop(1); err != nil {
				return err
			}
			if err := j.out.InsertString(j.opts.Separator); err != nil {
				return err
			}
		}
		if c == '\n' {
			held = true
			return nil
		}
		return j.out.Keep(1)
	})
	if err != nil {
		return err
	}
	if held {
		return j.out.Keep(1)
	}
	return nil
}

// Tabs to spaces: each tab becomes the spaces up to the next tab stop.
// Columns count characters.
func tabsToSpaces(j *job) error {
	width := j.indent()
	col := 0
	return j.each(func(c rune, size int) error {
		switch c {
		case '\t':
			n := width - col%width
			col += n
			j.count++
			return j.replace(size, spaces(n))
		case '\n':
			col = 0
		default:
			col++
		}
		return j.out.Keep(int64(size))
	})
}

func spaces(n int) string { return string(bytes.Repeat([]byte{' '}, n)) }

// Leading spaces to tabs: the spaces and tabs that start a line become as
// many tabs as fit, followed by the spaces left over. A line of only spaces
// and tabs stays as it is.
func spacesToTabs(j *job) error {
	width := j.indent()
	var lead []byte
	col := 0
	atStart := true
	err := j.eachByte(func(c byte) error {
		if !atStart {
			if c == '\n' {
				atStart = true
			}
			return j.out.Keep(1)
		}
		switch c {
		case ' ':
			lead = append(lead, c)
			col++
			return nil
		case '\t':
			lead = append(lead, c)
			col += width - col%width
			return nil
		case '\n':
			if err := j.out.Keep(int64(len(lead)) + 1); err != nil {
				return err
			}
			lead, col = lead[:0], 0
			return nil
		}
		want := string(bytes.Repeat([]byte{'\t'}, col/width)) + spaces(col%width)
		if want == string(lead) {
			if err := j.out.Keep(int64(len(lead))); err != nil {
				return err
			}
		} else {
			j.count++
			if err := j.out.Drop(int64(len(lead))); err != nil {
				return err
			}
			if err := j.out.InsertString(want); err != nil {
				return err
			}
		}
		lead, col, atStart = lead[:0], 0, false
		return j.out.Keep(1)
	})
	if err != nil {
		return err
	}
	return j.out.Keep(int64(len(lead)))
}

// naturalCompare orders lines the way the editor's Sort lines always has:
// letters without regard to case, runs of digits by their value ("item2"
// before "item10"), other characters by their Unicode code point.
func naturalCompare(a, b []byte) int {
	i, k := 0, 0
	for i < len(a) && k < len(b) {
		if isDigit(a[i]) && isDigit(b[k]) {
			ea, eb := i, k
			for ea < len(a) && isDigit(a[ea]) {
				ea++
			}
			for eb < len(b) && isDigit(b[eb]) {
				eb++
			}
			na, nb := bytes.TrimLeft(a[i:ea], "0"), bytes.TrimLeft(b[k:eb], "0")
			if len(na) != len(nb) {
				return compareInt(len(na), len(nb))
			}
			if c := bytes.Compare(na, nb); c != 0 {
				return c
			}
			i, k = ea, eb
			continue
		}
		ra, sa := utf8.DecodeRune(a[i:])
		rb, sb := utf8.DecodeRune(b[k:])
		if fa, fb := unicode.ToLower(ra), unicode.ToLower(rb); fa != fb {
			return compareInt(int(fa), int(fb))
		}
		i, k = i+sa, k+sb
	}
	return compareInt(len(a)-i, len(b)-k)
}

func isDigit(c byte) bool { return c >= '0' && c <= '9' }

func compareInt(a, b int) int {
	switch {
	case a < b:
		return -1
	case a > b:
		return 1
	}
	return 0
}

// Sort A→Z or Z→A. Lines that compare equal keep their order. An input
// that is already in order is left as it is.
func sortLines(desc bool) func(*job) error {
	return func(j *job) error {
		cmp := naturalCompare
		if desc {
			cmp = func(a, b []byte) int { return naturalCompare(b, a) }
		}
		j.passes = 2
		s := newSorter(j, cmp)
		defer s.close()
		var prev []byte
		ordered, trailing := true, false
		for {
			line, term, err := j.in.line()
			if err == io.EOF {
				break
			}
			if err != nil {
				return err
			}
			if s.count > 0 && ordered && cmp(prev, line) > 0 {
				ordered = false
			}
			prev, trailing = line, term
			if err = s.add(line); err != nil {
				return err
			}
		}
		if ordered {
			j.same = true
			return nil
		}
		j.count = s.count
		return writeLines(j, s.count, trailing, s.each)
	}
}

// writeLines inserts lines one after the other, each with a line break but
// the last, which has one only when the input ended with one.
func writeLines(j *job, total int64, trailing bool, each func(func([]byte) error) error) error {
	var written int64
	return each(func(line []byte) error {
		written++
		if err := j.out.Insert(line); err != nil {
			return err
		}
		if written < total || trailing {
			if err := j.out.InsertString("\n"); err != nil {
				return err
			}
		}
		if written%4096 == 0 {
			if j.env.Progress != nil {
				j.env.Progress((1 + float64(written)/float64(total)) / 2)
			}
			return j.ctx.Err()
		}
		return nil
	})
}

// Reverse line order. The lines are read in runs that fit the memory budget;
// each run is written to a scratch file in reverse, and the runs are then
// copied out last run first.
func reverseLines(j *job) error {
	j.passes = 2
	s := newSorter(j, nil)
	defer s.close()
	trailing := false
	for {
		line, term, err := j.in.line()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		trailing = term
		if err = s.add(line); err != nil {
			return err
		}
	}
	if s.count < 2 {
		j.same = true
		return nil
	}
	j.count = s.count
	return writeLines(j, s.count, trailing, s.reversed)
}

// Remove duplicate lines: of lines with exactly the same text the first
// stays. The first pass sorts every line together with its number (an
// external sort, so memory stays within the budget); lines equal to the one
// before them in that order are duplicates, and their numbers are sorted
// again. The second pass reads the input once more and drops those lines.
func dedupe(j *job) error {
	j.passes = 2
	byText := newSorter(j, func(a, b []byte) int {
		if c := bytes.Compare(a[:len(a)-8], b[:len(b)-8]); c != 0 {
			return c
		}
		return bytes.Compare(a[len(a)-8:], b[len(b)-8:])
	})
	defer byText.close()
	var index uint64
	for {
		line, _, err := j.in.line()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		if err = byText.add(appendIndex(line, index)); err != nil {
			return err
		}
		index++
	}
	drops := newSorter(j, bytes.Compare)
	defer drops.close()
	var prev []byte
	err := byText.each(func(record []byte) error {
		text := record[:len(record)-8]
		if prev != nil && bytes.Equal(prev, text) {
			return drops.add(append([]byte(nil), record[len(record)-8:]...))
		}
		prev = append(prev[:0], text...)
		if prev == nil {
			prev = []byte{}
		}
		return nil
	})
	if err != nil {
		return err
	}
	if drops.count == 0 {
		j.same = true
		return nil
	}
	j.count = drops.count
	next, stop, err := drops.stream()
	if err != nil {
		return err
	}
	defer stop()
	pending, more, err := next()
	if err != nil {
		return err
	}
	if err = j.reopen(); err != nil {
		return err
	}
	return filterLines(j, func(i uint64) (bool, error) {
		if !more || readIndex(pending) != i {
			return false, nil
		}
		pending, more, err = next()
		return true, err
	})
}

func appendIndex(line []byte, i uint64) []byte {
	out := make([]byte, len(line)+8)
	copy(out, line)
	for k := 0; k < 8; k++ {
		out[len(line)+k] = byte(i >> (56 - 8*k))
	}
	return out
}

func readIndex(p []byte) uint64 {
	var v uint64
	for _, c := range p[len(p)-8:] {
		v = v<<8 | uint64(c)
	}
	return v
}

// filterLines keeps or drops whole lines, in input order. As in Remove empty
// lines, the line break of the last kept line waits for the next kept line,
// so that removing the last lines of an input without a final line break
// leaves no line break behind.
func filterLines(j *job, drop func(uint64) (bool, error)) error {
	held, lastTerm := false, false
	var heldDrop int64
	for i := uint64(0); ; i++ {
		line, term, err := j.in.line()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		lastTerm = term
		n := int64(len(line))
		if term {
			n++
		}
		gone, err := drop(i)
		if err != nil {
			return err
		}
		if gone {
			if held {
				heldDrop += n
			} else if err = j.out.Drop(n); err != nil {
				return err
			}
			continue
		}
		if held {
			if err = j.out.Keep(1); err != nil {
				return err
			}
			if err = j.out.Drop(heldDrop); err != nil {
				return err
			}
			held, heldDrop = false, 0
		}
		if err = j.out.Keep(int64(len(line))); err != nil {
			return err
		}
		held = term
	}
	if !held {
		return nil
	}
	if lastTerm {
		if err := j.out.Keep(1); err != nil {
			return err
		}
		return j.out.Drop(heldDrop)
	}
	return j.out.Drop(1 + heldDrop)
}
