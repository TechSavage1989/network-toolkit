package editorbuffer

import (
	"bytes"
	"context"
	"math/rand"
	"strings"
	"testing"
)

// reopened is the editing text of the file a save produced.
func reopened(t *testing.T, saved []byte) string {
	t.Helper()
	return textOf(t, importBytes(t, saved).Buffer)
}

func keptCopy(t *testing.T, b *Buffer) []byte {
	t.Helper()
	s, e := b.Snapshot()
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	var kept bytes.Buffer
	if e = s.WriteKept(context.Background(), &kept); e != nil {
		t.Fatal(e)
	}
	return kept.Bytes()
}

// A line that ended in a lone CR, directly followed by a break written as LF,
// used to be saved as "\r\n": one line break on disk where the editor showed
// two. Enter at the start of the line after the CR is the shortest way there.
func TestMixedCRThenTypedBreakReopens(t *testing.T) {
	v := importBytes(t, []byte("a\rb\nc\nd\n"))
	if v.Format.EOL != MixedEOL || v.Buffer.NewLineEOL() != "LF" {
		t.Fatal(v.Format, v.Buffer.NewLineEOL())
	}
	// "a\nb\nc\nd\n": Enter at the start of line b.
	if _, e := v.Buffer.Edit(1, 2, 2, "\n"); e != nil {
		t.Fatal(e)
	}
	shown := textOf(t, v.Buffer)
	saved, e := encoded(t, v.Buffer, v.Format)
	if e != nil {
		t.Fatal(e)
	}
	if again := reopened(t, saved); again != shown {
		t.Errorf("saved %q: the editor showed %q, the file reopens as %q", saved, shown, again)
	}
	if kept := keptCopy(t, v.Buffer); reopened(t, kept) != shown {
		t.Errorf("kept copy %q restores %q, the editor showed %q", kept, reopened(t, kept), shown)
	}
	// The untouched endings keep their bytes; the typed break after the lone
	// CR is the one that cannot be a bare LF.
	if string(saved) != "a\r\r\nb\nc\nd\n" {
		t.Errorf("saved %q", saved)
	}
}

// Deleting the text between a CR-ended line and an LF-ended one leaves the
// two endings next to each other.
func TestMixedCRThenLFAfterDeleteReopens(t *testing.T) {
	v := importBytes(t, []byte("a\rX\nb\r\nc\r\n"))
	if _, e := v.Buffer.Edit(1, 2, 3, ""); e != nil {
		t.Fatal(e)
	}
	shown := textOf(t, v.Buffer)
	saved, _ := encoded(t, v.Buffer, v.Format)
	if again := reopened(t, saved); again != shown || strings.Count(again, "\n") != 4 {
		t.Errorf("saved %q: the editor showed %q, the file reopens as %q", saved, shown, again)
	}
	if kept := keptCopy(t, v.Buffer); reopened(t, kept) != shown {
		t.Errorf("kept copy %q restores %q, the editor showed %q", kept, reopened(t, kept), shown)
	}
}

// mixedModel is the file as the engine promises to keep it: one token for
// each character and each line break, a break holding the bytes it is written
// with. It applies an edit by the two rules of Buffer.Edit for a file whose
// lines end in different ways:
//   - text at the start and the end of the replaced range that the new text
//     repeats is not part of the edit;
//   - if what is left replaces as many line breaks as it removes, each new
//     break is written like the one it stands for; otherwise new breaks get
//     the file's most common ending.
type mixedModel struct {
	tokens     []string
	base       string
	undo, redo [][]string
}

func isBreak(token string) bool { return token == "\n" || token == "\r\n" || token == "\r" }

func (m *mixedModel) text(tokens []string) string {
	var out strings.Builder
	for _, token := range tokens {
		if isBreak(token) {
			out.WriteByte('\n')
		} else {
			out.WriteString(token)
		}
	}
	return out.String()
}

func (m *mixedModel) edit(from, to int, text string) {
	m.undo = append(m.undo, m.tokens)
	m.redo = nil
	old := m.tokens[from:to]
	oldText := m.text(old)
	if oldText == text {
		return
	}
	prefix := 0
	for prefix < len(old) && prefix < len(text) && oldText[prefix] == text[prefix] {
		prefix++
	}
	suffix := 0
	for suffix < len(old)-prefix && suffix < len(text)-prefix && oldText[len(old)-1-suffix] == text[len(text)-1-suffix] {
		suffix++
	}
	replaced, typed := old[prefix:len(old)-suffix], text[prefix:len(text)-suffix]
	var endings []string
	for _, token := range replaced {
		if isBreak(token) {
			endings = append(endings, token)
		}
	}
	keep := len(endings) > 0 && len(endings) == strings.Count(typed, "\n")
	var added []string
	for _, c := range typed {
		switch {
		case c != '\n':
			added = append(added, string(c))
		case keep:
			added = append(added, endings[0])
			endings = endings[1:]
		default:
			added = append(added, m.base)
		}
	}
	next := append([]string{}, m.tokens[:from+prefix]...)
	next = append(next, added...)
	m.tokens = append(next, m.tokens[to-suffix:]...)
}

// bytes is the file a save must write. Every break is written with its own
// bytes, with ONE exception: a bare LF directly after a lone CR would read
// back as a single CRLF, one line break where the editor shows two. That LF
// is written as CRLF ("\r" then "\r\n"), so the text stays right although an
// ending the user did not retype changed its bytes. The two can only come to
// stand together through an edit: in a file as it was read, CR directly
// before LF is one CRLF.
func (m *mixedModel) bytes() []byte {
	var out []byte
	afterCR := false
	for _, token := range m.tokens {
		if token == "\n" && afterCR {
			token = "\r\n"
		}
		out = append(out, token...)
		afterCR = token == "\r"
	}
	return out
}

// Random files that mix LF, CRLF and lone CR (also at the edge of an import
// chunk and at the end of the file) go through random edits: typing, deleting
// across line breaks, Enter beside every kind of ending, replacing lines by
// as many lines, undo and redo. After every few steps:
//
//	(a) what Encode, an export, and the kept copy write reopens as exactly
//	    the text the editor shows, with the same number of lines;
//	(b) it is byte for byte the model's file: every line the edits did not
//	    touch has the ending it was read with, but for the one exception
//	    described at mixedModel.bytes.
func TestMixedLineEndingsReopenAsShown(t *testing.T) {
	r := rand.New(rand.NewSource(11))
	kinds := []string{"\n", "\r\n", "\r"}
	files, exceptions := 0, 0
	for round := 0; files < 220; round++ {
		model := &mixedModel{}
		if round%20 == 0 {
			// A lone CR or a CRLF across the edge of an import chunk.
			for i := ChunkBytes - 3 + r.Intn(6); i > 0; i-- {
				model.tokens = append(model.tokens, "p")
			}
		}
		weights := []int{1 + r.Intn(4), 1 + r.Intn(4), 1 + r.Intn(4)}
		for lines := 2 + r.Intn(24); lines > 0; lines-- {
			for i := r.Intn(4); i > 0; i-- {
				model.tokens = append(model.tokens, string(rune('a'+r.Intn(3))))
			}
			pick := r.Intn(weights[0] + weights[1] + weights[2])
			kind := 0
			if pick >= weights[0] {
				kind = 1
			}
			if pick >= weights[0]+weights[1] {
				kind = 2
			}
			// As a file is read, CR directly before LF is one CRLF.
			if kinds[kind] == "\n" && len(model.tokens) > 0 && model.tokens[len(model.tokens)-1] == "\r" {
				kind = 1
			}
			model.tokens = append(model.tokens, kinds[kind])
		}
		if r.Intn(2) == 0 {
			model.tokens = append(model.tokens, "t", "a", "i", "l")
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
		files++
		model.base = map[string]string{"LF": "\n", "CRLF": "\r\n", "CR": "\r"}[v.Buffer.NewLineEOL()]
		revision := v.Buffer.Metadata().Revision
		var log []string
		check := func() {
			t.Helper()
			shown := textOf(t, v.Buffer)
			if shown != model.text(model.tokens) {
				t.Fatalf("file %q after %q: the editor shows %q, the model %q", clip(raw), log, clip([]byte(shown)), clip([]byte(model.text(model.tokens))))
			}
			want := model.bytes()
			if !bytes.Equal(want, []byte(strings.Join(model.tokens, ""))) {
				exceptions++
			}
			saved, e := encoded(t, v.Buffer, v.Format)
			if e != nil {
				t.Fatal(e)
			}
			s, _ := v.Buffer.Snapshot()
			var exported, kept bytes.Buffer
			_, write, e := s.PrepareEncoding(context.Background(), v.Format, nil)
			if e == nil {
				e = write(context.Background(), &exported)
			}
			if e == nil {
				e = s.WriteKept(context.Background(), &kept)
			}
			s.Close()
			if e != nil {
				t.Fatal(e)
			}
			for name, out := range map[string][]byte{"save": saved, "export": exported.Bytes(), "kept copy": kept.Bytes()} {
				again, e := Import(context.Background(), bytes.NewReader(out), t.TempDir(), nil)
				if e != nil {
					t.Fatal(e)
				}
				text, lines := textOf(t, again.Buffer), again.Buffer.Metadata().Lines
				rewritten, e := encoded(t, again.Buffer, again.Format)
				again.Buffer.Close()
				if text != shown || lines != v.Buffer.Metadata().Lines {
					t.Fatalf("file %q (new lines %q) after %q: the %s %q reopens as %q, the editor shows %q", clip(raw), model.base, log, name, clip(out), clip([]byte(text)), clip([]byte(shown)))
				}
				if !bytes.Equal(out, want) {
					t.Fatalf("file %q (new lines %q) after %q: the %s is %q, want %q", clip(raw), model.base, log, name, clip(out), clip(want))
				}
				if e != nil || !bytes.Equal(rewritten, out) {
					t.Fatalf("the %s %q does not save back unchanged after reopening: %q (%v)", name, clip(out), clip(rewritten), e)
				}
			}
		}
		check()
		for step := 4 + r.Intn(10); step > 0; step-- {
			size := len(model.tokens)
			var breaks []int
			for i, token := range model.tokens {
				if isBreak(token) {
					breaks = append(breaks, i)
				}
			}
			from, to, text := r.Intn(size+1), 0, ""
			to = from
			switch op := r.Intn(10); {
			case op < 2: // type
				text = []string{"y", "yz", "y\n", "\ny", "\n\n", "y\nz\ny"}[r.Intn(6)]
			case op < 4: // delete, often across line breaks
				to = min(size, from+r.Intn(6))
			case op < 6 && len(breaks) > 0: // Enter directly before or after a line break
				from = breaks[r.Intn(len(breaks))] + r.Intn(2)
				to, text = from, "\n"
			case op < 8 && len(breaks) > 0: // delete what stands between two line breaks
				i := r.Intn(len(breaks))
				from = breaks[i] + 1
				to = size
				if i+1 < len(breaks) {
					to = breaks[i+1]
				}
			case op < 9: // replace a range by the same text in capitals: as many lines as before
				to = min(size, from+r.Intn(12))
				text = strings.ToUpper(model.text(model.tokens[from:to]))
			default:
				redo := r.Intn(3) == 0
				stack, other := &model.undo, &model.redo
				if redo {
					stack, other = other, stack
				}
				m, e := v.Buffer.History(revision, redo)
				if e != nil {
					t.Fatal(e)
				}
				if m.Revision != revision {
					*other = append(*other, model.tokens)
					model.tokens = (*stack)[len(*stack)-1]
					*stack = (*stack)[:len(*stack)-1]
					revision = m.Revision
					log = append(log, map[bool]string{false: "undo", true: "redo"}[redo])
				} else if len(*stack) != 0 {
					t.Fatalf("history after %q: the engine had nothing to %v, the model has", log, redo)
				}
				continue
			}
			m, e := v.Buffer.Edit(revision, int64(from), int64(to), text)
			if e != nil {
				t.Fatal(e)
			}
			if m.Revision != revision {
				model.edit(from, to, text)
				revision = m.Revision
				log = append(log, strings.Join([]string{"edit", itoa(from), itoa(to), text}, " "))
			}
			if step%3 == 0 {
				check()
			}
		}
		check()
		v.Buffer.Close()
	}
	if exceptions == 0 {
		t.Fatal("no case put a bare LF directly after a lone CR; the test does not cover the exception")
	}
	t.Logf("%d mixed files; %d checks needed the CR-then-LF exception", files, exceptions)
}

func itoa(n int) string {
	if n >= ChunkBytes-8 {
		return "chunk+" + itoa(n-ChunkBytes)
	}
	if n < 0 {
		return "-" + itoa(-n)
	}
	if n < 10 {
		return string(rune('0' + n))
	}
	return itoa(n/10) + string(rune('0'+n%10))
}
