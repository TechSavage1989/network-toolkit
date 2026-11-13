package editorbuffer

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"os"
	"regexp"
	"strings"
	"testing"
	"unicode/utf16"
	"unicode/utf8"
)

// positions gives the UTF-16 position and the line of every byte of text.
func positions(text string) (units, lines []int64) {
	units, lines = make([]int64, len(text)+1), make([]int64, len(text)+1)
	for i, c := range text {
		w := int64(1)
		if c > 0xffff {
			w = 2
		}
		n := utf8.RuneLen(c)
		for k := 1; k <= n; k++ {
			units[i+k], lines[i+k] = units[i]+w, lines[i]
		}
		if c == '\n' {
			lines[i+n]++
		}
	}
	return units, lines
}

// both runs a check over a disk-backed buffer and over the same text held
// by a page (TextSnapshot): the Find window searches both with one code.
func both(t *testing.T, text string, check func(name string, s *Snapshot)) {
	t.Helper()
	b := openTest(t, text)
	s, err := b.Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	// Closed even when a check fails: the buffer's cleanup waits for it.
	func() {
		defer s.Close()
		check("disk", s)
	}()
	page, err := TextSnapshot(text)
	if err != nil {
		t.Fatal(err)
	}
	defer page.Close()
	check("page", page)
}

type refHit struct{ from, to, line int64 }

// reference lists the matches of a pattern that matches no empty text, the
// way Go's regexp finds them in the whole text.
func reference(text, pattern string) []refHit {
	var out []refHit
	units, lines := positions(text)
	for _, m := range regexp.MustCompile(pattern).FindAllStringIndex(text, -1) {
		out = append(out, refHit{units[m[0]], units[m[1]], lines[m[0]]})
	}
	return out
}

func allRows(t *testing.T, s *Snapshot, q Search, from, to int64) ([]Row, int64) {
	t.Helper()
	var rows []Row
	hits, err := s.FindAll(context.Background(), q, from, to, func(r Row) error { rows = append(rows, r); return nil }, nil)
	if err != nil {
		t.Fatal(err)
	}
	return rows, hits
}

// Count, Find All and Find Next agree with Go's own search of the whole
// text, on a document of many pieces with characters outside the BMP, for
// text in a disk-backed buffer and text held by a page.
func TestFindWindowSearchesAgreeWithTheWholeText(t *testing.T) {
	r := rand.New(rand.NewSource(55))
	words := []string{"vlan", "VLAN", "host", "192.0.2.1", "λ", "😀", "interface", "x", " ", "\t", "\n", "\n", "ab"}
	var sb strings.Builder
	for sb.Len() < 3*ChunkBytes {
		sb.WriteString(words[r.Intn(len(words))])
	}
	text := sb.String()
	cases := []struct {
		q       Search
		pattern string
	}{
		{Search{Query: "vlan", MatchCase: true}, `vlan`},
		{Search{Query: "vlan"}, `(?i)vlan`},
		{Search{Query: "host", WholeWord: true, MatchCase: true}, `\bhost\b`},
		{Search{Query: "😀λ", MatchCase: true}, `😀λ`},
		{Search{Query: `\d+\.\d+`, Regex: true, MatchCase: true}, `\d+\.\d+`},
		{Search{Query: `^vlan`, Regex: true, MatchCase: true}, `(?m)^vlan`},
		{Search{Query: `x$`, Regex: true, MatchCase: true}, `(?m)x$`},
		{Search{Query: `b\nh`, Regex: true, MatchCase: true}, `b\nh`},
		{Search{Query: `ab.v`, Regex: true, MatchCase: true}, `ab.v`},
		{Search{Query: `ab.v`, Regex: true, MatchCase: true, DotAll: true}, `(?s)ab.v`},
	}
	both(t, text, func(name string, s *Snapshot) {
		for _, c := range cases {
			want := reference(text, c.pattern)
			if len(want) == 0 {
				t.Fatalf("%s: the case %q has no match to check", name, c.pattern)
			}
			count, err := s.Count(context.Background(), c.q, 0, -1, nil)
			if err != nil || count != int64(len(want)) {
				t.Fatalf("%s Count %q = %d %v, want %d", name, c.pattern, count, err, len(want))
			}
			rows, hits := allRows(t, s, c.q, 0, -1)
			if hits != int64(len(want)) {
				t.Fatalf("%s Find All %q found %d, want %d", name, c.pattern, hits, len(want))
			}
			k := 0
			for _, row := range rows {
				for _, hit := range row.Hits {
					w := want[k]
					if hit[0] != w.from || hit[1] != w.to || row.Line != w.line {
						t.Fatalf("%s Find All %q hit %d = %v line %d, want %+v", name, c.pattern, k, hit, row.Line, w)
					}
					k++
				}
			}
			// Find Next from the end of each match finds the next one, and
			// wraps to the first at the end.
			at := int64(0)
			// (The first matches one after the other, then the last, which
			// wraps to the first.)
			for k := 0; k <= len(want); k++ {
				if k == 30 && len(want) > 31 {
					k = len(want) - 1
					at = want[k-1].to
				}
				m, err := s.FindNext(context.Background(), c.q, at, 0, -1, false, true)
				w := want[k%len(want)]
				if err != nil || !m.Found || m.From != w.from || m.To != w.to || m.Line != w.line || m.Wrapped != (k == len(want)) {
					t.Fatalf("%s Find Next %q #%d from %d = %+v %v, want %+v", name, c.pattern, k, at, m, err, w)
				}
				at = m.To
			}
			// Backwards from the end finds the last; from the first match it
			// wraps to the last.
			units := size(s.root).units
			if m, err := s.FindNext(context.Background(), c.q, units, 0, -1, true, true); err != nil || m.From != want[len(want)-1].from || m.Wrapped {
				t.Fatalf("%s backwards %q: %+v %v", name, c.pattern, m, err)
			}
			if m, err := s.FindNext(context.Background(), c.q, want[0].from, 0, -1, true, true); err != nil || m.From != want[len(want)-1].from || !m.Wrapped {
				t.Fatalf("%s backwards wrap %q: %+v %v", name, c.pattern, m, err)
			}
			if m, err := s.FindNext(context.Background(), c.q, want[0].from, 0, -1, true, false); err != nil || m.Found {
				t.Fatalf("%s backwards without wrap %q found %+v %v", name, c.pattern, m, err)
			}
		}
	})
}

// "In selection": only matches inside the range count, are listed, are
// found and are replaced; word boundaries and anchors still see the text
// around the range.
func TestFindWindowInSelection(t *testing.T) {
	text := "vlan 10\nvlan 20\nvlan 30\nvlan 40\n"
	// Lines two and three: [8, 24).
	both(t, text, func(name string, s *Snapshot) {
		q := Search{Query: "vlan", MatchCase: true}
		if n, err := s.Count(context.Background(), q, 8, 24, nil); err != nil || n != 2 {
			t.Fatalf("%s: %d %v", name, n, err)
		}
		rows, hits := allRows(t, s, q, 8, 24)
		if hits != 2 || rows[0].Line != 1 || rows[1].Line != 2 || rows[0].Text != "vlan 20" {
			t.Fatalf("%s: %d %+v", name, hits, rows)
		}
		// Find Next at the end of the range wraps to its start, not to the
		// document's start; without wrap it finds nothing.
		m, _ := s.FindNext(context.Background(), q, 20, 8, 24, false, true)
		if !m.Found || m.From != 8 || !m.Wrapped {
			t.Fatalf("%s: %+v", name, m)
		}
		if m, _ = s.FindNext(context.Background(), q, 20, 8, 24, false, false); m.Found {
			t.Fatalf("%s: found %+v past the range without wrap", name, m)
		}
		// A match that would run past the range's end is not one.
		if n, _ := s.Count(context.Background(), Search{Query: "vlan 30", MatchCase: true}, 8, 20, nil); n != 0 {
			t.Fatalf("%s: a match past the range counted", name)
		}
		// The range starts inside a word: that is not the start of a word.
		if n, _ := s.Count(context.Background(), Search{Query: "lan", MatchCase: true, WholeWord: true}, 9, 24, nil); n != 0 {
			t.Fatalf("%s: a word boundary at the range start", name)
		}
		if n, _ := s.Count(context.Background(), Search{Query: `^v`, Regex: true}, 9, 24, nil); n != 1 {
			t.Fatalf("%s: ^ in the range: %d", name, n)
		}
	})
	got, n, err := ReplaceInText(context.Background(), text, Search{Query: `vlan (\d)`, Regex: true}, "VLAN-$1", 8, 24, 1<<20)
	if err != nil || n != 2 || got != "vlan 10\nVLAN-20\nVLAN-30\nvlan 40\n" {
		t.Fatalf("%d %v %q", n, err, got)
	}
	b := openTest(t, text)
	s, _ := b.Snapshot()
	r, err := s.ReplaceAllIn(context.Background(), Search{Query: `vlan (\d)`, Regex: true}, "VLAN-$1", 8, 24, nil)
	s.Close()
	if err != nil || r.Count != 2 {
		t.Fatal(r, err)
	}
	if _, err = r.Commit(); err != nil || textOf(t, b) != got {
		t.Fatalf("%v %q", err, textOf(t, b))
	}
}

// ". matches newline" and lines: ^ and $ are at every line.
func TestFindWindowDotMatchesNewline(t *testing.T) {
	text := "begin\nmiddle\nend\n"
	both(t, text, func(name string, s *Snapshot) {
		q := Search{Query: `begin.*end`, Regex: true}
		if n, _ := s.Count(context.Background(), q, 0, -1, nil); n != 0 {
			t.Fatalf("%s: . matched a line break without DotAll", name)
		}
		q.DotAll = true
		m, err := s.FindNext(context.Background(), q, 0, 0, -1, false, false)
		if err != nil || !m.Found || m.From != 0 || m.To != 16 {
			t.Fatalf("%s: %+v %v", name, m, err)
		}
		// The row shows the first line of a match that spans lines.
		rows, _ := allRows(t, s, q, 0, -1)
		if len(rows) != 1 || rows[0].Text != "begin" || rows[0].Hits[0] != [2]int64{0, 16} {
			t.Fatalf("%s: %+v", name, rows)
		}
		if n, _ := s.Count(context.Background(), Search{Query: `^\w+$`, Regex: true}, 0, -1, nil); n != 3 {
			t.Fatalf("%s: ^ and $ at each line: %d", name, n)
		}
	})
}

// Rows of a very long line show the matches with a little of the line
// around them, cut down, and the text of each row is the document's text at
// its position.
func TestFindAllRowsOfALongLine(t *testing.T) {
	var sb strings.Builder
	for i := 0; i < 60000; i++ {
		fmt.Fprintf(&sb, "k%d=%d&", i, i%10)
	}
	sb.WriteString("\nshort λ k1=1 line\n")
	text := sb.String()
	utf := utf16.Encode([]rune(text))
	both(t, text, func(name string, s *Snapshot) {
		rows, hits := allRows(t, s, Search{Query: "=7&", MatchCase: true}, 0, -1)
		if hits != 6000 {
			t.Fatalf("%s: %d hits", name, hits)
		}
		for _, row := range rows {
			end := row.Start + int64(len(utf16.Encode([]rune(row.Text))))
			if string(utf16.Decode(utf[row.Start:end])) != row.Text {
				t.Fatalf("%s: row text %q is not the document at %d", name, row.Text, row.Start)
			}
			lineEnd := int64(strings.IndexByte(text, '\n'))
			if row.Line != 0 || row.CutEnd != (end < lineEnd) || len(row.Text) > rowText*4 || utf8.RuneCountInString(row.Text) > rowText {
				t.Fatalf("%s: %+v", name, row)
			}
			for _, hit := range row.Hits {
				if hit[0] < row.Start || hit[1] > end || string(utf16.Decode(utf[hit[0]:hit[1]])) != "=7&" {
					t.Fatalf("%s: hit %v outside its row %+v", name, hit, row)
				}
			}
		}
		if rows[0].CutStart || !rows[1].CutStart || rows[1].Hits[0][0]-rows[1].Start != rowBefore {
			t.Fatalf("%s: %+v", name, rows[0])
		}
		// Several matches on a short line share one row.
		rows, hits = allRows(t, s, Search{Query: "1"}, int64(len(utf))-12, -1)
		if hits != 2 || len(rows) != 1 || rows[0].Line != 1 || rows[0].Text != "short λ k1=1 line" || rows[0].CutStart || rows[0].CutEnd {
			t.Fatalf("%s: %d %+v", name, hits, rows)
		}
	})
}

// The shown part of a document is searched with a bounded read: matches
// that start in the range, at most max of them.
func TestFindWindowMatchesOfAPage(t *testing.T) {
	text := strings.Repeat("ab\n", 50000)
	both(t, text, func(name string, s *Snapshot) {
		q := Search{Query: "b", MatchCase: true}
		hits, more, err := s.Matches(context.Background(), q, 300, 330, -1, 100)
		if err != nil || more || len(hits) != 10 || hits[0] != (Hit{From: 301, To: 302, Line: 100}) {
			t.Fatalf("%s: %v %v %+v", name, err, more, hits)
		}
		hits, more, _ = s.Matches(context.Background(), q, 0, 150000, -1, 50)
		if !more || len(hits) != 50 {
			t.Fatalf("%s: %d %v", name, len(hits), more)
		}
		// Marks of a range end where the range ends.
		if hits, _, _ = s.Matches(context.Background(), q, 300, 330, 310, 100); len(hits) != 3 {
			t.Fatalf("%s: %+v", name, hits)
		}
	})
}

// Mark All counts the matches and lists the lines they start on, bounded.
func TestFindWindowMarkLines(t *testing.T) {
	text := "a x x\nb\nc x\nd\n" + strings.Repeat("x\n", 30)
	both(t, text, func(name string, s *Snapshot) {
		count, lines, more, err := s.Mark(context.Background(), Search{Query: "x"}, 0, -1, 4, nil)
		if err != nil || count != 33 || !more || fmt.Sprint(lines) != "[0 2 4 5]" {
			t.Fatalf("%s: %d %v %v %v", name, count, lines, more, err)
		}
		text, n, err := s.MatchedText(context.Background(), Search{Query: `[a-d] ?x?`, Regex: true}, 0, 14, "\n", 1<<20)
		if err != nil || n != 4 || text != "a x\nb\nc x\nd" {
			t.Fatalf("%s: %q %d %v", name, text, n, err)
		}
		if _, _, err = s.MatchedText(context.Background(), Search{Query: "x"}, 0, -1, "\n", 10); !errors.Is(err, ErrTooMuchText) {
			t.Fatalf("%s: %v", name, err)
		}
	})
}

// A long search stops when it is cancelled.
func TestFindWindowCancel(t *testing.T) {
	b := openTest(t, strings.Repeat("abcdefgh\n", 100000))
	s, _ := b.Snapshot()
	defer s.Close()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	for name, run := range map[string]func() error{
		"Count": func() error { _, err := s.Count(ctx, Search{Query: "zz"}, 0, -1, nil); return err },
		"Find All": func() error {
			_, err := s.FindAll(ctx, Search{Query: "a"}, 0, -1, func(Row) error { return nil }, nil)
			return err
		},
		"Mark":    func() error { _, _, _, err := s.Mark(ctx, Search{Query: "a"}, 0, -1, 10, nil); return err },
		"Replace": func() error { _, err := s.ReplaceAllIn(ctx, Search{Query: "a"}, "b", 0, -1, nil); return err },
	} {
		if err := run(); !errors.Is(err, context.Canceled) {
			t.Errorf("%s: %v", name, err)
		}
	}
	// Progress is reported as the search reads.
	var last float64
	if _, err := s.Count(context.Background(), Search{Query: "zz"}, 0, -1, func(p float64) { last = p }); err != nil || last < 0.9 {
		t.Fatalf("%v %v", last, err)
	}
}

// A file that is not text is searched, counted, listed and marked, and
// nothing replaces anything in it.
func TestFindWindowReadsAFileThatIsNotText(t *testing.T) {
	v := importBytes(t, []byte("PK\x03\x04\x00\x00name\r\nmore name\x00\n"))
	s, _ := v.Buffer.Snapshot()
	defer s.Close()
	q := Search{Query: "name", MatchCase: true}
	if n, err := s.Count(context.Background(), q, 0, -1, nil); err != nil || n != 2 {
		t.Fatal(n, err)
	}
	if _, hits := allRows(t, s, q, 0, -1); hits != 2 {
		t.Fatal(hits)
	}
	if _, err := s.ReplaceAllIn(context.Background(), q, "x", 0, -1, nil); !errors.Is(err, ErrBinary) {
		t.Fatal(err)
	}
}

// The tracker that turns bytes into positions and lines agrees with the
// tree at every character, in a document edited into many pieces.
func TestFindPositionTrackerAgreesWithTheTree(t *testing.T) {
	r := rand.New(rand.NewSource(7))
	text := strings.Repeat("ab\nλ😀c\n\n", 9000)
	b := openTest(t, text)
	for i := 0; i < 300; i++ {
		meta := b.Metadata()
		at := r.Int63n(meta.Units)
		if _, err := b.Window(at, 1); err != nil {
			continue
		}
		if _, err := b.Edit(meta.Revision, at, at, []string{"\n", "x", "😀\n", "yy"}[r.Intn(4)]); err != nil && !errors.Is(err, ErrBoundary) {
			t.Fatal(err)
		}
	}
	s, _ := b.Snapshot()
	defer s.Close()
	all := textOf(t, b)
	unitAt, lineAt := positions(all)
	for _, start := range []int64{0, 5, int64(len(all) / 3)} {
		for !utf8.RuneStart(all[start]) {
			start++
		}
		track := s.tracker(start)
		for at := start; at <= int64(len(all)); at += 1 + r.Int63n(300) {
			for at < int64(len(all)) && !utf8.RuneStart(all[at]) {
				at++
			}
			if err := track.advance(at); err != nil {
				t.Fatal(err)
			}
			wantUnits, wantLine := unitAt[at], lineAt[at]
			lineByte := int64(strings.LastIndexByte(all[:at], '\n') + 1)
			gotByte, gotUnits, err := track.lineStart()
			if err != nil || track.units != wantUnits || track.lines != wantLine || gotByte != lineByte || gotUnits != unitAt[lineByte] {
				t.Fatalf("at %d: %d %d %d %d, want %d %d %d", at, track.units, track.lines, gotByte, gotUnits, wantUnits, wantLine, lineByte)
			}
		}
	}
}

// Results keep every row in order, also past their memory budget, and
// Close removes their file.
func TestFindResultsPagesAndSpills(t *testing.T) {
	old := resultMemory
	resultMemory = 64 << 10
	defer func() { resultMemory = old }()
	results := NewResults(t.TempDir())
	const rows = 200000
	for i := int64(0); i < rows; i++ {
		row := Row{Line: i, Start: i * 10, Text: fmt.Sprintf("line %d λ", i), CutEnd: i%3 == 0, Hits: [][2]int64{{i*10 + 1, i*10 + 3}, {i*10 + 5, i*10 + 6}}}
		if err := results.Add(row); err != nil {
			t.Fatal(err)
		}
	}
	if n, hits := results.Size(); n != rows || hits != 2*rows {
		t.Fatal(n, hits)
	}
	if results.file == nil {
		t.Fatal("rows past the memory budget were not written to the scratch file")
	}
	for _, start := range []int64{0, 255, 256, 99999, rows - 3} {
		page, err := results.Page(start, 100)
		if err != nil {
			t.Fatal(err)
		}
		if want := min(100, rows-start); int64(len(page)) != want {
			t.Fatalf("page at %d: %d rows", start, len(page))
		}
		for k, row := range page {
			i := start + int64(k)
			if row.Line != i || row.Text != fmt.Sprintf("line %d λ", i) || row.CutEnd != (i%3 == 0) || row.Hits[1] != [2]int64{i*10 + 5, i*10 + 6} {
				t.Fatalf("row %d: %+v", i, row)
			}
		}
	}
	name := results.file.Name()
	results.Close()
	if _, err := results.Page(0, 1); !errors.Is(err, ErrResultsClosed) {
		t.Fatal(err)
	}
	if _, err := os.Stat(name); err == nil {
		t.Fatal("the results file is still there")
	}
}
