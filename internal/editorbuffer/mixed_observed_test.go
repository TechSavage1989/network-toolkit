package editorbuffer

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"math/rand"
	"strings"
	"testing"
	"unicode/utf8"
)

// observed is what a person can see of a file with line endings of several
// kinds, and nothing else: the text, and for each line break whether an edit
// request covered it. It does not know how the engine decides which ending a
// line gets (mixedModel does, and shares that rule with the engine).
type observed struct {
	text []rune
	// kept[i] is the ending the break text[i] was read with, while no edit
	// request has covered that position; "" otherwise.
	kept []string
}

func (o observed) clone() observed {
	return observed{append([]rune{}, o.text...), append([]string{}, o.kept...)}
}

func readObserved(raw string) observed {
	var o observed
	for i := 0; i < len(raw); {
		switch {
		case strings.HasPrefix(raw[i:], "\r\n"):
			o.text, o.kept, i = append(o.text, '\n'), append(o.kept, "\r\n"), i+2
		case raw[i] == '\r' || raw[i] == '\n':
			o.text, o.kept, i = append(o.text, '\n'), append(o.kept, raw[i:i+1]), i+1
		default:
			r, width := utf8.DecodeRuneInString(raw[i:])
			o.text, o.kept, i = append(o.text, r), append(o.kept, ""), i+width
		}
	}
	return o
}

// replace is the request "put text where [from, to) was": everything inside
// the range is covered, whatever the engine makes of it.
func (o *observed) replace(from, to int, text string) {
	added := []rune(text)
	o.text = append(append(append([]rune{}, o.text[:from]...), added...), o.text[to:]...)
	o.kept = append(append(append([]string{}, o.kept[:from]...), make([]string, len(added))...), o.kept[to:]...)
}

func (o observed) units(index int) int64 { return units(string(o.text[:index])) }

// check compares a written file with what was observed:
//   - it reads back as the text, with as many lines;
//   - a line break that no edit request covered has the ending it was read
//     with. The only thing allowed instead is CRLF for an LF that directly
//     follows a line break written as a lone CR, because "\r\n" there would
//     read back as one line break.
func (o observed) check(written []byte, lines int64) error {
	again := readObserved(string(written))
	if string(again.text) != string(o.text) {
		return fmt.Errorf("it reads back as %.200q, the editor shows %.200q", string(again.text), string(o.text))
	}
	if count := int64(strings.Count(string(o.text), "\n")) + 1; lines != count {
		return fmt.Errorf("the editor counts %d lines, the text has %d", lines, count)
	}
	for i, ending := range o.kept {
		if ending == "" || again.kept[i] == ending {
			continue
		}
		if ending == "\n" && again.kept[i] == "\r\n" && i > 0 && again.kept[i-1] == "\r" {
			continue
		}
		return fmt.Errorf("the line break at character %d, which no edit covered, was read as %q and is written as %q", i, ending, again.kept[i])
	}
	return nil
}

// Random files with empty lines, lone CRs next to each other, characters of
// two, three and four bytes, and line endings at the edge of an import chunk
// go through edits, bulk edits (committed and given up), Replace All
// (committed, given up and refused), undo and redo. After every step the
// save, the export and the kept copy are checked against what was observed;
// at the end, undoing everything gives the bytes of the file as it was read.
func TestMixedLineEndingsObservedFacts(t *testing.T) {
	pieces := []string{"\r", "\n", "\r\n", "a", "b", "é", "€", "😀", "\r", "\n", "ab", " "}
	typed := []string{"", "a", "\n", "\n\n", "a\nb", "é", "😀\n", "x\ny\nz", "b", "\na", "a\n", "€"}
	dir := t.TempDir()
	open := func(raw []byte) *Imported {
		v, e := Import(context.Background(), bytes.NewReader(raw), dir, nil)
		if e != nil {
			t.Fatal(e)
		}
		return v
	}
	steps, mixed := 0, 0
	for seed := int64(1); seed <= 400; seed++ {
		r := rand.New(rand.NewSource(seed))
		var source strings.Builder
		edits := 16
		if seed%50 == 0 {
			// Line endings on both sides of the edge of an import chunk.
			source.WriteString(strings.Repeat("x", ChunkBytes-3-r.Intn(4)))
			source.WriteString(strings.Repeat("\r", r.Intn(4)))
			edits = 6
		}
		for i := 2 + r.Intn(60); i > 0; i-- {
			source.WriteString(pieces[r.Intn(len(pieces))])
		}
		raw := source.String()
		v := open([]byte(raw))
		b := v.Buffer
		if v.Format.EOL == MixedEOL {
			mixed++
		}
		now := readObserved(raw)
		var undo, redo []observed
		changed := func(next observed) {
			undo, redo = append(undo, now), nil
			now = next
		}
		verify := func(step string) {
			t.Helper()
			if shown := textOf(t, b); shown != string(now.text) {
				t.Fatalf("seed %d %s: the editor shows %.200q, expected %.200q", seed, step, shown, string(now.text))
			}
			s, e := b.Snapshot()
			if e != nil {
				t.Fatal(e)
			}
			var saved, exported, kept bytes.Buffer
			e = s.Encode(context.Background(), &saved, v.Format)
			if e == nil {
				_, export, prepareErr := s.PrepareEncoding(context.Background(), v.Format, nil)
				if e = prepareErr; e == nil {
					e = export(context.Background(), &exported)
				}
			}
			if e == nil {
				e = s.WriteKept(context.Background(), &kept)
			}
			s.Close()
			if e != nil {
				t.Fatalf("seed %d %s: %v", seed, step, e)
			}
			lines := b.Metadata().Lines
			for name, out := range map[string][]byte{"save": saved.Bytes(), "export": exported.Bytes(), "kept copy": kept.Bytes()} {
				if v.Format.EOL != MixedEOL && name == "kept copy" {
					// A file with one kind of ending is kept as plain text.
					if string(readObserved(string(out)).text) != string(now.text) {
						t.Fatalf("seed %d %s: the kept copy reads back as different text", seed, step)
					}
					continue
				}
				if e := now.check(out, lines); e != nil {
					t.Fatalf("seed %d %s, file %.120q: the %s %.200q: %v", seed, step, raw[max(0, len(raw)-120):], name, out, e)
				}
				// The engine itself reads it back as the same text and line count.
				again := open(out)
				text, count := textOf(t, again.Buffer), again.Buffer.Metadata().Lines
				again.Buffer.Close()
				if text != string(now.text) || count != lines {
					t.Fatalf("seed %d %s: the %s %.200q reopens as %.200q (%d lines), the editor shows %.200q (%d lines)", seed, step, name, out, text, count, string(now.text), lines)
				}
			}
		}
		verify("as read")
		if out, e := encoded(t, b, v.Format); e != nil || string(out) != raw {
			t.Fatalf("seed %d: the unedited file is not written back byte for byte (%v)", seed, e)
		}
		for step := 0; step < edits; step++ {
			steps++
			revision := b.Metadata().Revision
			size := len(now.text)
			from := r.Intn(size + 1)
			name := ""
			switch op := r.Intn(10); {
			case op < 4:
				to := min(size, from+r.Intn(5))
				text := typed[r.Intn(len(typed))]
				if r.Intn(4) == 0 && to > from {
					// A command that hands over the range again with a change in it.
					old := now.text[from:to]
					cut := r.Intn(len(old) + 1)
					text = string(old[:cut]) + text + string(old[max(cut, len(old)-r.Intn(3)):])
				}
				name = fmt.Sprintf("step %d: edit %d..%d %q", step, from, to, text)
				m, e := b.Edit(revision, now.units(from), now.units(to), text)
				if e != nil {
					t.Fatalf("seed %d %s: %v", seed, name, e)
				}
				if m.Revision != revision {
					next := now.clone()
					next.replace(from, to, text)
					changed(next)
				} else if from != to || text != "" {
					t.Fatalf("seed %d %s: the edit was not applied", seed, name)
				}
			case op < 6:
				to := min(size, from+r.Intn(8))
				stage, e := b.BeginEdit(revision, now.units(from), now.units(to))
				if e != nil {
					t.Fatal(e)
				}
				text := ""
				for parts := r.Intn(4); parts > 0; parts-- {
					part := typed[r.Intn(len(typed))]
					if e = stage.Append(part); e != nil {
						t.Fatal(e)
					}
					text += part
				}
				name = fmt.Sprintf("step %d: bulk edit %d..%d %q", step, from, to, text)
				if r.Intn(4) == 0 {
					stage.Abort()
					name += " (given up)"
				} else if _, e = stage.Commit(); e != nil {
					t.Fatal(e)
				} else if from != to || text != "" {
					next := now.clone()
					next.replace(from, to, text)
					changed(next)
				}
			case op < 8:
				query := []string{"a", "b", "\n", "a\n", "\nb", "é", "\n\n", " ", "😀"}[r.Intn(9)]
				replacement := typed[r.Intn(len(typed))]
				name = fmt.Sprintf("step %d: replace all %q by %q", step, query, replacement)
				s, _ := b.Snapshot()
				staged, e := s.ReplaceAll(context.Background(), query, replacement, false, true, false)
				s.Close()
				if e != nil {
					t.Fatal(e)
				}
				switch r.Intn(5) {
				case 0:
					staged.Abort()
					name += " (given up)"
				case 1:
					// The document changes first: the prepared result is refused.
					if _, e = b.Edit(revision, 0, 0, "b"); e != nil {
						t.Fatal(e)
					}
					next := now.clone()
					next.replace(0, 0, "b")
					changed(next)
					if _, e = staged.Commit(); !errors.Is(e, ErrRevision) {
						t.Fatalf("seed %d %s: a stale Replace All: %v", seed, name, e)
					}
					staged.Abort()
					name += " (refused)"
				default:
					if _, e = staged.Commit(); e != nil {
						t.Fatal(e)
					}
					// Each match is one covered range, replaced from the last to the first.
					next, found, width := now.clone(), 0, len([]rune(query))
					text := string(now.text)
					var starts []int
					for at := 0; ; {
						i := strings.Index(text[at:], query)
						if i < 0 {
							break
						}
						starts = append(starts, len([]rune(text[:at+i])))
						at += i + len(query)
						found++
					}
					for i := len(starts) - 1; i >= 0; i-- {
						next.replace(starts[i], starts[i]+width, replacement)
					}
					if staged.Count != int64(found) {
						t.Fatalf("seed %d %s: %d replaced, %d found", seed, name, staged.Count, found)
					}
					if found > 0 {
						changed(next)
					}
				}
			default:
				forward := op == 9
				from, to := &undo, &redo
				if forward {
					from, to = to, from
				}
				name = fmt.Sprintf("step %d: redo=%v", step, forward)
				m, e := b.History(revision, forward)
				if e != nil {
					t.Fatal(e)
				}
				if (m.Revision != revision) != (len(*from) > 0) {
					t.Fatalf("seed %d %s: the engine and the test disagree about the history", seed, name)
				}
				if len(*from) > 0 {
					*to = append(*to, now)
					now = (*from)[len(*from)-1]
					*from = (*from)[:len(*from)-1]
				}
			}
			verify(name)
		}
		// Undoing everything gives the file as it was read, byte for byte.
		m := b.Metadata()
		for m.CanUndo {
			var e error
			if m, e = b.History(m.Revision, false); e != nil {
				t.Fatal(e)
			}
		}
		if out, e := encoded(t, b, v.Format); e != nil || string(out) != raw || m.Dirty {
			t.Fatalf("seed %d: after undoing every step the file is %.200q, it was read as %.200q (%v, dirty=%v)", seed, out, raw, e, m.Dirty)
		}
		b.Close()
	}
	if mixed < 200 {
		t.Fatalf("only %d of the files mixed their line endings", mixed)
	}
	t.Logf("%d steps on 400 files, %d of them with mixed line endings", steps, mixed)
}
