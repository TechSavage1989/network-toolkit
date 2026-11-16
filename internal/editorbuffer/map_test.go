package editorbuffer

import (
	"strings"
	"testing"
	"unicode/utf16"
)

func TestMapSamplesCoverTheWholeDocument(t *testing.T) {
	var text strings.Builder
	for i := 0; i < 1000; i++ {
		text.WriteString("set name \"row ")
		text.WriteString(strings.Repeat("x", i%7))
		text.WriteString("\" 192.0.2.1\n")
	}
	text.WriteString("😀 last " + strings.Repeat("long ", 400))
	b := openTest(t, text.String())
	meta := b.Metadata()
	lines := strings.Split(text.String(), "\n")
	got, err := b.MapSamples(200, 40, false)
	if err != nil || len(got.Rows) != 200 || got.Lines != meta.Lines {
		t.Fatal(err, len(got.Rows))
	}
	for k, row := range got.Rows {
		line := int64(k) * meta.Lines / 200
		want := strings.TrimSuffix(lines[line], "\r")
		if row.Line != line || row.Column != 0 || !strings.HasPrefix(want, row.Text) || (len(want) > 0 && row.Text == "") {
			t.Fatalf("row %d: %+v, want the start of line %d %q", k, row, line, want)
		}
	}
	// By offset: wrapped rows of 40 units, the first, a middle and the last
	// part of the document, never across a line break.
	got, err = b.MapSamples(333, 40, true)
	if err != nil || len(got.Rows) != 333 {
		t.Fatal(err)
	}
	units := utf16.Encode([]rune(text.String()))
	for k, row := range got.Rows {
		if row.Column%40 != 0 || strings.ContainsAny(row.Text, "\r\n") || row.Start > meta.Units {
			t.Fatalf("row %d: %+v", k, row)
		}
		at := int64(k) * meta.Units / 333
		if row.Start > at || at-row.Start > 40 {
			t.Fatalf("row %d starts at %d for position %d", k, row.Start, at)
		}
		if want := string(utf16.Decode(units[row.Start:min(int64(len(units)), row.Start+int64(len(utf16.Encode([]rune(row.Text)))))])); want != row.Text {
			t.Fatalf("row %d: %q is not the text at %d (%q)", k, row.Text, row.Start, want)
		}
	}
	if last := got.Rows[len(got.Rows)-1]; last.Line != meta.Lines-1 || !strings.HasPrefix(last.Text, "long") && !strings.Contains(last.Text, "long") {
		t.Fatalf("the last row is not in the last line: %+v", last)
	}
	for _, bad := range [][2]int{{0, 40}, {MaxMapRows + 1, 40}, {10, 0}, {10, MaxMapColumns + 1}} {
		if _, err := b.MapSamples(bad[0], bad[1], false); err != ErrBoundary {
			t.Fatalf("%v: %v", bad, err)
		}
	}
	// A row that would start inside a surrogate pair is read without it.
	pairs := openTest(t, strings.Repeat("😀", 50))
	if got, err = pairs.MapSamples(7, 3, true); err != nil || len(got.Rows) != 7 {
		t.Fatal(err)
	}
}
