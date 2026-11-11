package editorbuffer

import (
	"bytes"
	"context"
	"errors"
	"math/rand"
	"regexp"
	"strings"
	"testing"
)

func replaceAll(t *testing.T, b *Buffer, query, replacement string, regex bool) (Metadata, int64) {
	t.Helper()
	s, e := b.Snapshot()
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	staged, e := s.ReplaceAll(context.Background(), query, replacement, regex, true, false)
	if e != nil {
		t.Fatal(e)
	}
	before, was := textOf(t, b), b.Metadata()
	m, e := staged.Commit()
	if e != nil {
		t.Fatal(e)
	}
	if staged.Count == 0 && (textOf(t, b) != before || m != was) {
		t.Fatal("a Replace All without a match changed the document")
	}
	return m, staged.Count
}

// Replace All gives the text a plain replacement gives, across piece
// boundaries, after earlier edits, for matches that touch each other, that
// are empty or that span lines; one undo takes all of it back.
func TestReplaceAllMatchesPlainReplacement(t *testing.T) {
	r := rand.New(rand.NewSource(5))
	words := []string{"interface", "vlan", "é", "😀", "aa", "a", "\n", " ", "end\n"}
	for round := 0; round < 60; round++ {
		var source strings.Builder
		size := 200 + r.Intn(3000)
		if round%15 == 0 {
			size = ChunkBytes*2 + r.Intn(ChunkBytes)
		}
		for source.Len() < size {
			source.WriteString(words[r.Intn(len(words))])
		}
		b := openTest(t, source.String())
		revision := b.Metadata().Revision
		for i := r.Intn(6); i > 0; i-- {
			at := int64(r.Intn(100))
			m, e := b.Edit(revision, at, at, "aa vlan")
			if e != nil {
				// The position split a character; any other one will do.
				continue
			}
			revision = m.Revision
		}
		text := textOf(t, b)
		for _, c := range []struct {
			query, replacement string
			regex              bool
		}{{"a", "aa", false}, {"aa", "", false}, {"vlan", "VLAN 10", false}, {"é", "e", false}, {"\n", "\n\n", false}, {"end\n", "", false}, {"(?m)^", "> ", true}, {"[a-z]+", "w", true}, {"nothing here", "x", false}} {
			want, count := strings.ReplaceAll(text, c.query, c.replacement), int64(strings.Count(text, c.query))
			if c.regex {
				// The replacement is literal text, as in the editor.
				pattern := regexp.MustCompile(c.query)
				want, count = pattern.ReplaceAllLiteralString(text, c.replacement), int64(len(pattern.FindAllStringIndex(text, -1)))
			}
			m, replaced := replaceAll(t, b, c.query, c.replacement, c.regex)
			if got := textOf(t, b); got != want || replaced != count {
				t.Fatalf("round %d %q -> %q: %d replaced, want %d; text %q, want %q", round, c.query, c.replacement, replaced, count, clip([]byte(got)), clip([]byte(want)))
			}
			if meta := openTest(t, want).Metadata(); m.Units != meta.Units || m.Lines != meta.Lines || m.Bytes != meta.Bytes {
				t.Fatalf("round %d %q: the document counts %+v, its text %+v", round, c.query, m, meta)
			}
			if replaced > 0 {
				if m, e := b.History(m.Revision, false); e != nil || textOf(t, b) != text || m.Revision == 0 {
					t.Fatalf("one undo did not take the Replace All back: %v", e)
				}
			}
		}
		b.Close()
	}
}

// A stale or closed document is not replaced, and a cancelled search stops.
func TestReplaceAllRefusesStaleAndCancelled(t *testing.T) {
	b := openTest(t, strings.Repeat("vlan 10\n", 50000))
	s, _ := b.Snapshot()
	staged, e := s.ReplaceAll(context.Background(), "vlan", "VLAN", false, true, false)
	s.Close()
	if e != nil || staged.Count != 50000 {
		t.Fatal(e, staged)
	}
	if _, e = b.Edit(1, 0, 0, "x"); e != nil {
		t.Fatal(e)
	}
	if _, e = staged.Commit(); !errors.Is(e, ErrRevision) || !strings.HasPrefix(textOf(t, b), "xvlan 10\n") {
		t.Fatal("a Replace All prepared for an earlier revision was applied: ", e)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	s, _ = b.Snapshot()
	defer s.Close()
	if _, e = s.ReplaceAll(ctx, "vlan", "VLAN", false, true, false); !errors.Is(e, context.Canceled) {
		t.Fatal(e)
	}
	if _, e = s.ReplaceAll(context.Background(), "vlan", "a\rb", false, true, false); e == nil {
		t.Fatal("a replacement with a carriage return was accepted")
	}
}

// A replaced range keeps the endings of its lines when the new text has as
// many lines, also when it arrives in several parts and with characters of
// several bytes at the ends of what the old and the new text share.
func TestMixedReplacedRangeKeepsItsLineEndings(t *testing.T) {
	const raw = "één\r\ntwee\ndrie\rvier\r\nvijf\r\n"
	// Editing text "één\ntwee\ndrie\nvier\nvijf\n".
	v := importBytes(t, []byte(raw))
	stage, e := v.Buffer.BeginEdit(1, 0, 18)
	if e != nil {
		t.Fatal(e)
	}
	for _, part := range []string{"ÉÉN\nTW", "EE\n", "DRIE\nVIER"} {
		if e = stage.Append(part); e != nil {
			t.Fatal(e)
		}
	}
	m, e := stage.Commit()
	if e != nil {
		t.Fatal(e)
	}
	if out, _ := encoded(t, v.Buffer, v.Format); string(out) != "ÉÉN\r\nTWEE\nDRIE\rVIER\r\nvijf\r\n" {
		t.Fatalf("%q", out)
	}
	// "é" and "è" share their first byte: what the texts have in common ends
	// before the character, not inside it.
	if m, e = v.Buffer.Edit(m.Revision, 19, 23, "vèjf"); e != nil {
		t.Fatal(e)
	}
	if m, e = v.Buffer.Edit(m.Revision, 0, 3, "ÉÈN"); e != nil {
		t.Fatal(e)
	}
	out, _ := encoded(t, v.Buffer, v.Format)
	if string(out) != "ÉÈN\r\nTWEE\nDRIE\rVIER\r\nvèjf\r\n" || textOf(t, importBytes(t, out).Buffer) != textOf(t, v.Buffer) {
		t.Fatalf("%q", out)
	}
	for i := 0; i < 3; i++ {
		if m, e = v.Buffer.History(m.Revision, false); e != nil {
			t.Fatal(e)
		}
	}
	if out, _ = encoded(t, v.Buffer, v.Format); string(out) != raw || m.Dirty {
		t.Fatalf("undo: %q", out)
	}
	// More lines than were replaced: nothing can be matched up, the new lines
	// get the file's most common ending.
	stage, _ = v.Buffer.BeginEdit(m.Revision, 4, 13)
	stage.Append("a\nb\nc")
	if _, e = stage.Commit(); e != nil {
		t.Fatal(e)
	}
	if out, _ = encoded(t, v.Buffer, v.Format); string(out) != "één\r\na\r\nb\r\nc\rvier\r\nvijf\r\n" {
		t.Fatalf("%q", out)
	}
}

// Every method of a buffer and of its snapshot that changes text or writes it
// out refuses a file that is not text. WriteTo is not one of them: it is how
// the display rendering is read.
func TestBinaryBufferRefusesEveryWrite(t *testing.T) {
	raw := []byte("PK\x03\x04\x00\x00name\r\nmore\x00\n")
	v := importBytes(t, raw)
	b := v.Buffer
	before, shown := b.Metadata(), textOf(t, b)
	s, _ := b.Snapshot()
	defer s.Close()
	var sink bytes.Buffer
	text := Format{Encoding: "UTF-8", EOL: "LF"}
	refusals := map[string]error{}
	_, refusals["Edit"] = b.Edit(before.Revision, 0, 1, "x")
	_, refusals["Edit that changes nothing"] = b.Edit(before.Revision, 0, 0, "")
	_, refusals["BeginEdit"] = b.BeginEdit(before.Revision, 0, 0)
	_, refusals["History (undo)"] = b.History(before.Revision, false)
	_, refusals["History (redo)"] = b.History(before.Revision, true)
	refusals["Encode"] = s.Encode(context.Background(), &sink, text)
	refusals["Encode in its own format"] = s.Encode(context.Background(), &sink, v.Format)
	_, _, refusals["PrepareEncoding"] = s.PrepareEncoding(context.Background(), text, nil)
	_, _, refusals["PrepareEncoding of a range"] = s.PrepareEncoding(context.Background(), text, &[2]int64{0, 2})
	refusals["WriteKept"] = s.WriteKept(context.Background(), &sink)
	_, refusals["ReplaceAll"] = s.ReplaceAll(context.Background(), "name", "x", false, true, false)
	_, refusals["ReplaceAll without a match"] = s.ReplaceAll(context.Background(), "absent", "x", false, true, false)
	_, refusals["Replacement.Commit"] = (&Replacement{b: b, revision: before.Revision, Count: 1}).Commit()
	for name, e := range refusals {
		if !errors.Is(e, ErrBinary) {
			t.Errorf("%s of a file that is not text: %v", name, e)
		}
	}
	if sink.Len() != 0 {
		t.Errorf("%d bytes of a file that is not text were written", sink.Len())
	}
	// Nothing can mark it as unsaved work either.
	if m := b.MarkUnsaved(); m.Dirty {
		t.Error("a file that is not text was marked as unsaved work")
	}
	if after := b.Metadata(); after != before || textOf(t, b) != shown {
		t.Errorf("the refusals left a trace: %+v, was %+v", after, before)
	}
}

// Replace All in files whose lines end in different ways: every line break
// outside a match is written as it was read (whether its stretch of text was
// kept in place or copied beside the replacements), line breaks inside a
// replacement get the file's most common ending, and the file reopens as the
// text the editor shows.
func TestReplaceAllKeepsMixedLineEndings(t *testing.T) {
	r := rand.New(rand.NewSource(9))
	kinds := []string{"\n", "\r\n", "\r"}
	for round := 0; round < 80; round++ {
		model := &mixedModel{}
		for lines := 3 + r.Intn(40); lines > 0; lines-- {
			word := []string{"vlan", "host", "ab", "x"}[r.Intn(4)]
			for _, c := range word + strings.Repeat("p", r.Intn(4)) {
				model.tokens = append(model.tokens, string(c))
			}
			if r.Intn(6) == 0 {
				// A long stretch without a match keeps its pieces.
				for i := replaceInlineBytes + r.Intn(ChunkBytes); i > 0; i-- {
					model.tokens = append(model.tokens, "q")
				}
			}
			kind := kinds[r.Intn(3)]
			if kind == "\n" && model.tokens[len(model.tokens)-1] == "\r" {
				kind = "\r\n"
			}
			model.tokens = append(model.tokens, kind)
		}
		raw := []byte(strings.Join(model.tokens, ""))
		v, e := Import(context.Background(), bytes.NewReader(raw), t.TempDir(), nil)
		if e != nil {
			t.Fatal(e)
		}
		if v.Format.EOL != MixedEOL {
			v.Buffer.Close()
			continue
		}
		model.base = map[string]string{"LF": "\n", "CRLF": "\r\n", "CR": "\r"}[v.Buffer.NewLineEOL()]
		revision := v.Buffer.Metadata().Revision
		for step := 0; step < 4; step++ {
			c := [][2]string{{"vlan", "VLAN 10"}, {"host", ""}, {"ab", "a\nb"}, {"p\n", "\n"}, {"x", "xx"}, {"\n", "\n\n"}, {"pp", "\n"}}[r.Intn(7)]
			text := model.text(model.tokens)
			var next []string
			count, at := int64(0), 0
			for {
				i := strings.Index(text[at:], c[0])
				if i < 0 {
					break
				}
				next = append(next, model.tokens[at:at+i]...)
				for _, ch := range c[1] {
					if ch == '\n' {
						next = append(next, model.base)
					} else {
						next = append(next, string(ch))
					}
				}
				at += i + len(c[0])
				count++
			}
			next = append(next, model.tokens[at:]...)
			before := model.tokens
			m, replaced := replaceAll(t, v.Buffer, c[0], c[1], false)
			if replaced != count {
				t.Fatalf("%q: %d replaced, want %d", c[0], replaced, count)
			}
			model.tokens, revision = next, m.Revision
			out, e := encoded(t, v.Buffer, v.Format)
			if want := model.bytes(); e != nil || !bytes.Equal(out, want) {
				t.Fatalf("round %d, %q -> %q in %q (new lines %q): saved %q, want %q (%v)", round, c[0], c[1], clip(raw), model.base, clip(out), clip(want), e)
			}
			if again := importBytes(t, out); textOf(t, again.Buffer) != textOf(t, v.Buffer) {
				t.Fatalf("round %d, %q -> %q: the saved file reopens as different text", round, c[0], c[1])
			}
			if kept := keptCopy(t, v.Buffer); !bytes.Equal(kept, out) {
				t.Fatalf("round %d: the kept copy differs from the save", round)
			}
			if count > 0 && r.Intn(3) == 0 {
				// Undo gives the document, and its endings, as before.
				if m, e = v.Buffer.History(revision, false); e != nil {
					t.Fatal(e)
				}
				model.tokens, revision = before, m.Revision
				if out, _ = encoded(t, v.Buffer, v.Format); !bytes.Equal(out, model.bytes()) {
					t.Fatalf("round %d: undo of %q -> %q saved %q", round, c[0], c[1], clip(out))
				}
			}
		}
		v.Buffer.Close()
	}
}
