package editorbuffer

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"networktoolkit/internal/texttools"
)

// tool runs a text tool over [from, to) of a buffer and commits it.
func tool(t *testing.T, b *Buffer, id string, opts texttools.Options, from, to int64) (Metadata, *Replacement, error) {
	t.Helper()
	s, err := b.Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	info, _ := texttools.Lookup(id)
	staged, err := s.Rewrite(context.Background(), from, to, info.Rewrite, func(src *RangeSource, out Edits) (bool, error) {
		result, err := texttools.Run(context.Background(), id, opts, src, out, texttools.Env{Dir: b.ScratchDir()})
		return result.Changed, err
	})
	s.Close()
	if err != nil {
		return Metadata{}, nil, err
	}
	defer staged.Abort()
	meta, err := staged.Commit()
	return meta, staged, err
}

func TestRewriteRunsAToolAsOneUndoStep(t *testing.T) {
	v := importBytes(t, []byte("keep\nb  \na  \nkeep\n"))
	b := v.Buffer
	before := b.Metadata()
	// Lines 2 and 3 (units 5 to 13), sorted.
	meta, staged, err := tool(t, b, "sort-asc", texttools.Options{}, 5, 13)
	if err != nil || staged.Count != 1 || staged.Units != 8 {
		t.Fatal(err, staged)
	}
	if got := textOf(t, b); got != "keep\na  \nb  \nkeep\n" {
		t.Fatalf("%q", got)
	}
	if meta.Revision != before.Revision+1 || !meta.Dirty || !meta.CanUndo {
		t.Fatalf("%+v", meta)
	}
	if _, err = b.History(meta.Revision, false); err != nil || textOf(t, b) != "keep\nb  \na  \nkeep\n" {
		t.Fatal("one undo step does not take the sort back: ", err, textOf(t, b))
	}
	// A tool with nothing to do changes nothing and stores nothing.
	end := b.end
	if _, staged, err = tool(t, b, "lower", texttools.Options{}, 0, 4); err != nil || staged.Count != 0 {
		t.Fatal(err)
	}
	if _, staged, err = tool(t, b, "trim-trailing", texttools.Options{}, 0, 4); err != nil || staged.Count != 0 || b.end != end {
		t.Fatal("a tool that changed nothing stored text or changed the document: ", err, b.end, end)
	}
	if b.Metadata().Revision != meta.Revision+1 {
		t.Fatal("the revision moved")
	}
	// A failing tool changes nothing and gives its text back.
	s, _ := b.Snapshot()
	_, err = s.Rewrite(context.Background(), 0, 4, true, func(src *RangeSource, out Edits) (bool, error) {
		out.Drop(src.Size())
		out.Insert([]byte(strings.Repeat("x", 3*ChunkBytes)))
		return false, errors.New("the tool failed")
	})
	s.Close()
	if err == nil || b.end != end {
		t.Fatal("a failed tool left its text in the scratch file: ", err)
	}
}

// Lines a tool did not change keep their own endings; a sort gives each
// place the ending it had (as an edit of the range would).
func TestRewriteKeepsMixedLineEndings(t *testing.T) {
	const raw = "one\r\ntwo  \nthree\rfour  \r\nfive\r\n"
	cases := []struct {
		id   string
		opts texttools.Options
		want string
	}{
		{"trim-trailing", texttools.Options{}, "one\r\ntwo\nthree\rfour\r\nfive\r\n"},
		{"upper", texttools.Options{}, "ONE\r\nTWO  \nTHREE\rFOUR  \r\nFIVE\r\n"},
		{"sort-desc", texttools.Options{}, "two  \r\nthree\none\rfour  \r\nfive\r\n"},
		{"remove-empty", texttools.Options{}, raw},
		{"split", texttools.Options{Separator: "t"}, "one\r\n\r\nwo  \n\r\nhree\rfour  \r\nfive\r\n"},
		{"join", texttools.Options{Separator: "+"}, "one+two  +three+four  +five\r\n"},
	}
	for _, c := range cases {
		t.Run(c.id, func(t *testing.T) {
			v := importBytes(t, []byte(raw))
			units := v.Buffer.Metadata().Units
			if _, _, err := tool(t, v.Buffer, c.id, c.opts, 0, units); err != nil {
				t.Fatal(err)
			}
			if got, _ := encoded(t, v.Buffer, v.Format); string(got) != c.want {
				t.Fatalf("saved %q, want %q", got, c.want)
			}
		})
	}
	// Removing empty lines keeps the endings of the lines that stay.
	v := importBytes(t, []byte("a\r\n\nb\r\r\nc\n"))
	if _, _, err := tool(t, v.Buffer, "remove-empty", texttools.Options{}, 0, v.Buffer.Metadata().Units); err != nil {
		t.Fatal(err)
	}
	if got, _ := encoded(t, v.Buffer, v.Format); string(got) != "a\r\nb\rc\n" {
		t.Fatalf("%q", got)
	}
}

// Large tools store their text in chunks; kept stretches longer than the
// inline limit keep their pieces.
func TestRewriteOfALargeDocument(t *testing.T) {
	line := strings.Repeat("abc ", 2000) + "\n"
	raw := strings.Repeat(line, 300)
	v := importBytes(t, []byte(raw))
	b := v.Buffer
	if _, _, err := tool(t, b, "upper", texttools.Options{}, 0, b.Metadata().Units); err != nil {
		t.Fatal(err)
	}
	if textOf(t, b) != strings.ToUpper(raw) {
		t.Fatal("UPPERCASE of a large document")
	}
	encodedText := base64.StdEncoding.EncodeToString([]byte(raw))
	w := importBytes(t, []byte(encodedText))
	if _, _, err := tool(t, w.Buffer, "base64-decode", texttools.Options{}, 0, w.Buffer.Metadata().Units); err != nil {
		t.Fatal(err)
	}
	if textOf(t, w.Buffer) != raw {
		t.Fatal("decoding a large single line")
	}
	// Removing the trailing spaces of every line keeps the long stretches
	// as pieces of the text that was read.
	spaced := strings.Repeat(strings.Repeat("y", replaceInlineBytes*2)+"  \n", 20)
	x := importBytes(t, []byte(spaced))
	if _, _, err := tool(t, x.Buffer, "trim-trailing", texttools.Options{}, 0, x.Buffer.Metadata().Units); err != nil {
		t.Fatal(err)
	}
	if textOf(t, x.Buffer) != strings.ReplaceAll(spaced, "  \n", "\n") {
		t.Fatal("trim of long lines")
	}
}

func TestLineRange(t *testing.T) {
	v := importBytes(t, []byte("first\nsecond\nthird"))
	s, _ := v.Buffer.Snapshot()
	defer s.Close()
	for _, c := range [][4]int64{{2, 3, 0, 6}, {2, 8, 0, 13}, {6, 13, 6, 13}, {14, 15, 13, 18}, {0, 18, 0, 18}} {
		from, to, err := s.LineRange(c[0], c[1])
		if err != nil || from != c[2] || to != c[3] {
			t.Errorf("%v: %d %d %v", c, from, to, err)
		}
	}
}

type templateCase struct {
	Pattern     string `json:"pattern"`
	Text        string `json:"text"`
	Replacement string `json:"replacement"`
	Want        string `json:"want"`
	Error       bool   `json:"error"`
}

func templateCases(t *testing.T) []templateCase {
	raw, err := os.ReadFile(filepath.Join("testdata", "replace-template.json"))
	if err != nil {
		t.Fatal(err)
	}
	var file struct{ Cases []templateCase }
	if err = json.Unmarshal(raw, &file); err != nil {
		t.Fatal(err)
	}
	return file.Cases
}

// The Replace text of a regular expression, through the template alone and
// through Replace All and the single Replace of the engine.
func TestReplaceTemplate(t *testing.T) {
	for _, c := range templateCases(t) {
		re := regexp.MustCompile(c.Pattern)
		parts, err := parseTemplate(c.Replacement, re.SubexpNames())
		if c.Error {
			if err == nil {
				t.Errorf("%q: no error", c.Replacement)
			}
			continue
		}
		if err != nil {
			t.Fatalf("%q: %v", c.Replacement, err)
		}
		match := re.FindStringSubmatchIndex(c.Text)
		var groups []string
		for k := 0; k < len(match); k += 2 {
			if match[k] < 0 {
				groups = append(groups, "")
			} else {
				groups = append(groups, c.Text[match[k]:match[k+1]])
			}
		}
		if got := expandTemplate(parts, groups); got != c.Want {
			t.Errorf("%q with %q: %q, want %q", c.Pattern, c.Replacement, got, c.Want)
		}
		// Replace All over a buffer.
		v := importBytes(t, []byte(c.Text))
		s, _ := v.Buffer.Snapshot()
		staged, err := s.ReplaceAll(context.Background(), c.Pattern, c.Replacement, true, true, false)
		if err == nil {
			_, err = staged.Commit()
		}
		want := c.Text[:match[0]] + c.Want + c.Text[match[1]:]
		if got := textOf(t, v.Buffer); err != nil || got != want {
			t.Errorf("Replace All %q with %q: %q (%v), want %q", c.Pattern, c.Replacement, got, err, want)
		}
		// Replace All over text a page holds (the Find window's page-held
		// documents): the same template, the same result.
		if got, _, err := ReplaceInText(context.Background(), c.Text, Search{Query: c.Pattern, Regex: true, MatchCase: true}, c.Replacement, 0, -1, 1<<20); err != nil || got != want {
			t.Errorf("Replace All of page text %q with %q: %q (%v), want %q", c.Pattern, c.Replacement, got, err, want)
		}
		// The single Replace of the selected match.
		w := importBytes(t, []byte(c.Text))
		s2, _ := w.Buffer.Snapshot()
		from, to := int64(len([]rune(c.Text[:match[0]]))), int64(len([]rune(c.Text[:match[1]])))
		text, found, err := s2.ReplaceText(context.Background(), c.Pattern, c.Replacement, from, to, true, true, false)
		s2.Close()
		s.Close()
		if err != nil || !found || text != c.Want {
			t.Errorf("Replace %q with %q: %q %t %v", c.Pattern, c.Replacement, text, found, err)
		}
	}
	// A group longer than one chunk is copied in pieces.
	long := "<" + strings.Repeat("é", ChunkBytes) + ">"
	v := importBytes(t, []byte(long+long))
	s, _ := v.Buffer.Snapshot()
	staged, err := s.ReplaceAll(context.Background(), "<(.*?)>", "[$1]", true, true, false)
	s.Close()
	if err == nil {
		_, err = staged.Commit()
	}
	if want := strings.Repeat("["+long[1:len(long)-1]+"]", 2); err != nil || textOf(t, v.Buffer) != want {
		t.Fatal("a long group: ", err)
	}
	// Outside a match the selection is not replaced.
	w := importBytes(t, []byte("abc"))
	s, _ = w.Buffer.Snapshot()
	defer s.Close()
	if _, found, err := s.ReplaceText(context.Background(), "b", "x", 0, 1, true, true, false); found || err != nil {
		t.Fatal(found, err)
	}
}
