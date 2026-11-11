package editorbuffer

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"math/rand"
	"os"
	"strings"
	"testing"
)

func importBytes(t *testing.T, raw []byte) *Imported {
	t.Helper()
	dir := t.TempDir()
	v, e := Import(context.Background(), bytes.NewReader(raw), dir, nil)
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { v.Buffer.Close() })
	sum := sha256.Sum256(raw)
	if v.Hash != hex.EncodeToString(sum[:]) || v.Bytes != int64(len(raw)) {
		t.Fatalf("the digest or length is not that of the source: %s %d", v.Hash, v.Bytes)
	}
	if files, _ := os.ReadDir(dir); len(files) != 1 {
		t.Fatalf("an abandoned reading left its scratch file: %d files", len(files))
	}
	return v
}

func encoded(t *testing.T, b *Buffer, f Format) ([]byte, error) {
	t.Helper()
	s, e := b.Snapshot()
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	var out bytes.Buffer
	if e = s.Encode(context.Background(), &out, f); e != nil {
		return nil, e
	}
	count, _, e := s.PrepareEncoding(context.Background(), f, nil)
	if e != nil || count != int64(out.Len()) {
		t.Fatalf("announced length %d, wrote %d (%v) for %+v", count, out.Len(), e, f)
	}
	return out.Bytes(), nil
}

func utf16Bytes(text string, big, bom bool) []byte {
	var out []byte
	put := func(u uint16) {
		if big {
			out = append(out, byte(u>>8), byte(u))
		} else {
			out = append(out, byte(u), byte(u>>8))
		}
	}
	if bom {
		put(0xfeff)
	}
	for _, r := range text {
		if r > 0xffff {
			r -= 0x10000
			put(uint16(0xd800 + r>>10))
			put(uint16(0xdc00 + r&0x3ff))
		} else {
			put(uint16(r))
		}
	}
	return out
}

// Every kind of text file comes back from an untouched save as the bytes
// that were read, whatever its encoding, byte order mark and line endings.
func TestImportOpensAnyFile(t *testing.T) {
	allBytes := make([]byte, 0, 255)
	for c := 1; c < 256; c++ {
		allBytes = append(allBytes, byte(c))
	}
	long := strings.Repeat("interface Ethernet1/1\r\n description uplink\n", 40000)
	cases := []struct {
		name   string
		raw    []byte
		format Format
	}{
		{"empty", nil, Format{"UTF-8", false, "LF"}},
		{"only a UTF-8 mark", []byte{0xef, 0xbb, 0xbf}, Format{"UTF-8", true, "LF"}},
		{"plain ASCII, no final line ending", []byte("hostname core-1"), Format{"UTF-8", false, "LF"}},
		{"UTF-8", []byte("naïve λ 😀\nsecond\n"), Format{"UTF-8", false, "LF"}},
		{"UTF-8 with a mark and CRLF", append([]byte{0xef, 0xbb, 0xbf}, "naïve λ 😀\r\nsecond\r\n"...), Format{"UTF-8", true, "CRLF"}},
		{"UTF-8 with CR", []byte("one\rtwo\rthree"), Format{"UTF-8", false, "CR"}},
		{"UTF-16LE", utf16Bytes("naïve λ 😀\r\nsecond\r\n", false, true), Format{"UTF-16LE", true, "CRLF"}},
		{"UTF-16BE", utf16Bytes("naïve λ 😀\nsecond", true, true), Format{"UTF-16BE", true, "LF"}},
		{"UTF-16LE with mixed line endings", utf16Bytes("a\r\nb\nc\rd", false, true), Format{"UTF-16LE", true, "Mixed"}},
		{"Windows-1252", []byte("caf\xe9 \x80 100\r\n\x93quoted\x94\r\n"), Format{"Windows-1252", false, "CRLF"}},
		{"Latin-1 with LF", []byte("gr\xfc\xdfe\nstra\xdfe\n"), Format{"Windows-1252", false, "LF"}},
		{"every byte but NUL", allBytes, Format{"Windows-1252", false, "Mixed"}},
		{"valid UTF-8 followed by an invalid sequence", []byte("ok λ\nbad \xc3\x28 \xff\n"), Format{"Windows-1252", false, "LF"}},
		{"a UTF-8 mark before invalid UTF-8", []byte("\xef\xbb\xbfbad \xc0\xaf\r\n"), Format{"Windows-1252", false, "CRLF"}},
		{"a truncated UTF-8 sequence at the end", []byte("caf\xc3"), Format{"Windows-1252", false, "LF"}},
		{"mixed: mostly LF", []byte("a\nb\r\nc\nd\n"), Format{"UTF-8", false, "Mixed"}},
		{"mixed: mostly CRLF", []byte("a\r\nb\nc\r\nd\r\n"), Format{"UTF-8", false, "Mixed"}},
		{"mixed: all three", []byte("a\rb\nc\r\nd\r\r\n\n\re"), Format{"UTF-8", false, "Mixed"}},
		{"mixed: ends with a lone CR", []byte("a\nb\r"), Format{"UTF-8", false, "Mixed"}},
		{"mixed: with a mark", []byte("\xef\xbb\xbfa\r\nb\n"), Format{"UTF-8", true, "Mixed"}},
		{"mixed: across many chunks", []byte(long), Format{"UTF-8", false, "Mixed"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			v := importBytes(t, c.raw)
			if v.Format != c.format || v.Binary {
				t.Fatalf("read as %+v binary=%v, want %+v", v.Format, v.Binary, c.format)
			}
			if strings.ContainsAny(textOf(t, v.Buffer), "\x00\r") {
				t.Fatal("the editing text holds a NUL or a carriage return")
			}
			out, e := encoded(t, v.Buffer, v.Format)
			if e != nil || !bytes.Equal(out, c.raw) {
				t.Fatalf("an untouched save changed the file (%v):\n got %q\nwant %q", e, clip(out), clip(c.raw))
			}
		})
	}
}

func clip(b []byte) []byte {
	if len(b) > 120 {
		return b[:120]
	}
	return b
}

// An edit changes only what was edited: every other line keeps the ending it
// was read with, also after undo, and text that is added uses the file's most
// common ending.
func TestMixedLineEndingsSurviveEdits(t *testing.T) {
	v := importBytes(t, []byte("one\r\ntwo\nthree\rfour\r\nfive\r\n"))
	b := v.Buffer
	if b.NewLineEOL() != "CRLF" {
		t.Fatal("new lines should use the most common ending, got", b.NewLineEOL())
	}
	// "one\ntwo\nthree\nfour\nfive\n": change "two" to "2", add a line after it.
	m, e := b.Edit(1, 4, 7, "2\nadded")
	if e != nil {
		t.Fatal(e)
	}
	out, e := encoded(t, b, v.Format)
	if want := "one\r\n2\r\nadded\nthree\rfour\r\nfive\r\n"; e != nil || string(out) != want {
		t.Fatalf("%v %q, want %q", e, out, want)
	}
	// Join "three" and "four": the lone CR between them is the only loss.
	if m, e = b.Edit(m.Revision, 17, 18, " "); e != nil {
		t.Fatal(e)
	}
	out, _ = encoded(t, b, v.Format)
	if want := "one\r\n2\r\nadded\nthree four\r\nfive\r\n"; string(out) != want {
		t.Fatalf("%q, want %q", out, want)
	}
	for i := 0; i < 2; i++ {
		if m, e = b.History(m.Revision, false); e != nil {
			t.Fatal(e)
		}
	}
	out, _ = encoded(t, b, v.Format)
	if string(out) != "one\r\ntwo\nthree\rfour\r\nfive\r\n" || m.Dirty {
		t.Fatalf("undo did not return the file as it was read: %q dirty=%v", out, m.Dirty)
	}
	// Choosing one ending is the user's explicit conversion of every line.
	for name, want := range map[string]string{"LF": "one\ntwo\nthree\nfour\nfive\n", "CRLF": "one\r\ntwo\r\nthree\r\nfour\r\nfive\r\n", "CR": "one\rtwo\rthree\rfour\rfive\r"} {
		if out, e = encoded(t, b, Format{"UTF-8", false, name}); e != nil || string(out) != want {
			t.Fatalf("%s: %v %q", name, e, out)
		}
	}
	// A selection is written with the endings of its own lines.
	s, _ := b.Snapshot()
	defer s.Close()
	var part bytes.Buffer
	count, write, e := s.PrepareEncoding(context.Background(), v.Format, &[2]int64{4, 14})
	if e == nil {
		e = write(context.Background(), &part)
	}
	if e != nil || part.String() != "two\nthree\r" || count != int64(part.Len()) {
		t.Fatalf("selection: %v %q (%d)", e, part.String(), count)
	}
	// A document that never had differing endings has no record of them.
	plain := importBytes(t, []byte("a\nb\n"))
	if _, e = encoded(t, plain.Buffer, Format{"UTF-8", false, "Mixed"}); e == nil {
		t.Fatal("wrote per-line endings for a document that has none recorded")
	}
}

// A model of the file as tokens checks random edits over thousands of lines
// and several import chunks: the result is always the unedited tokens as they
// were read plus the typed text with the file's most common ending. The
// model is mixedModel (mixed_reopen_test.go), which also knows the rules for
// a range replaced by as many lines and for a bare LF after a lone CR.
func TestMixedLineEndingsRandomEdits(t *testing.T) {
	r := rand.New(rand.NewSource(7))
	endings := []string{"\r\n", "\n", "\r\n", "\r", "\r\n"}
	model := &mixedModel{base: "\r\n"}
	for size := 0; size < ChunkBytes*3; {
		for _, c := range "line" + strings.Repeat("x", r.Intn(30)) {
			model.tokens = append(model.tokens, string(c))
			size++
		}
		ending := endings[r.Intn(len(endings))]
		if ending == "\n" && model.tokens[len(model.tokens)-1] == "\r" {
			ending = "\r\n"
		}
		model.tokens = append(model.tokens, ending)
		size += len(ending)
	}
	v := importBytes(t, []byte(strings.Join(model.tokens, "")))
	if v.Format.EOL != "Mixed" || v.Buffer.NewLineEOL() != "CRLF" {
		t.Fatal(v.Format, v.Buffer.NewLineEOL())
	}
	revision := v.Buffer.Metadata().Revision
	for step := 0; step < 300; step++ {
		from := r.Intn(len(model.tokens) + 1)
		to := min(len(model.tokens), from+r.Intn(40))
		var text strings.Builder
		for i := r.Intn(12); i > 0; i-- {
			if r.Intn(4) == 0 {
				text.WriteByte('\n')
			} else {
				text.WriteByte('y')
			}
		}
		m, e := v.Buffer.Edit(revision, int64(from), int64(to), text.String())
		if e != nil {
			t.Fatal(e)
		}
		if m.Revision != revision {
			model.edit(from, to, text.String())
		}
		revision = m.Revision
	}
	out, e := encoded(t, v.Buffer, v.Format)
	if want := model.bytes(); e != nil || !bytes.Equal(out, want) {
		t.Fatalf("edited save differs from the model (%v): %d bytes, want %d", e, len(out), len(want))
	}
	if again := importBytes(t, out); textOf(t, again.Buffer) != textOf(t, v.Buffer) {
		t.Fatal("the saved file reopens as different text")
	}
}

// Windows-1252 text keeps its bytes through edits; a character the encoding
// cannot hold stops the save instead of being replaced.
func TestLegacyEncodingEditsAndRefusesOtherCharacters(t *testing.T) {
	v := importBytes(t, []byte("caf\xe9\r\nprix: 5 \x80\r\n"))
	if textOf(t, v.Buffer) != "café\nprix: 5 €\n" {
		t.Fatalf("%q", textOf(t, v.Buffer))
	}
	m, e := v.Buffer.Edit(1, 0, 0, "é€ ")
	if e != nil {
		t.Fatal(e)
	}
	out, e := encoded(t, v.Buffer, v.Format)
	if e != nil || string(out) != "\xe9\x80 caf\xe9\r\nprix: 5 \x80\r\n" {
		t.Fatalf("%v %q", e, out)
	}
	if _, e = v.Buffer.Edit(m.Revision, 0, 0, "→"); e != nil {
		t.Fatal(e)
	}
	s, _ := v.Buffer.Snapshot()
	defer s.Close()
	var sink bytes.Buffer
	var refused *ErrUnencodable
	if e = s.Encode(context.Background(), &sink, v.Format); !errors.As(e, &refused) || refused.Rune != '→' {
		t.Fatal("a character outside Windows-1252 was written", e, sink.Bytes())
	}
	if _, _, e = s.PrepareEncoding(context.Background(), v.Format, nil); !errors.As(e, &refused) {
		t.Fatal("an export was promised for text the encoding cannot hold", e)
	}
	if e = s.Encode(context.Background(), &sink, Format{"Windows-1252", true, "CRLF"}); e == nil {
		t.Fatal("Windows-1252 has no byte order mark")
	}
	sink.Reset()
	if e = s.Encode(context.Background(), &sink, Format{"UTF-8", false, "CRLF"}); e != nil || sink.String() != "→é€ café\r\nprix: 5 €\r\n" {
		t.Fatal("explicit conversion to UTF-8 failed", e, sink.String())
	}
}

// A file that is not text can be looked at. Nothing can change it and no
// format writes it, so it can never be damaged from the editor.
func TestBinaryFilesAreDisplayOnly(t *testing.T) {
	png := []byte("\x89PNG\r\n\x1a\n\x00\x00\x00\rIHDR\x00\x00\x00\x01")
	exe := append([]byte("MZ\x90\x00\x03\x00\x00\x00\x04\x00\x00\x00\xff\xff\x00\x00"), bytes.Repeat([]byte{0}, ChunkBytes*2)...)
	zip := []byte("PK\x03\x04\x14\x00\x00\x00\x08\x00name.txt\xff\xfe\x00")
	cases := map[string][]byte{
		"image":                        png,
		"program":                      exe,
		"archive":                      zip,
		"only NUL bytes":               bytes.Repeat([]byte{0}, 4096),
		"text with one NUL":            []byte("valid text\x00more\n"),
		"NUL after invalid UTF-8":      []byte("caf\xe9 then \x00"),
		"UTF-16 without a mark":        utf16Bytes("plain", false, false),
		"UTF-16 with a NUL":            utf16Bytes("a\x00b", false, true),
		"UTF-16 with a lone surrogate": {0xff, 0xfe, 0x00, 0xd8, 0x41, 0x00},
		"UTF-16 with half a unit":      {0xfe, 0xff, 0x00, 0x41, 0x00},
	}
	for name, raw := range cases {
		t.Run(name, func(t *testing.T) {
			v := importBytes(t, raw)
			if !v.Binary || v.Format.Encoding != BinaryEncoding {
				t.Fatalf("read as text: %+v", v.Format)
			}
			text := textOf(t, v.Buffer)
			if strings.ContainsAny(text, "\x00\r") || text == "" {
				t.Fatalf("unusable display text %q", clip([]byte(text)))
			}
			meta := v.Buffer.Metadata()
			if _, e := v.Buffer.Edit(meta.Revision, 0, 0, "x"); !errors.Is(e, ErrBinary) {
				t.Fatal("edited a file that is not text", e)
			}
			if _, e := v.Buffer.BeginEdit(meta.Revision, 0, 1); !errors.Is(e, ErrBinary) {
				t.Fatal("began a bulk edit of a file that is not text", e)
			}
			s, _ := v.Buffer.Snapshot()
			defer s.Close()
			for _, f := range []Format{v.Format, {"UTF-8", false, "LF"}, {"Windows-1252", false, "CRLF"}, {"UTF-16LE", true, "LF"}, {"UTF-8", false, "Mixed"}} {
				var out bytes.Buffer
				if e := s.Encode(context.Background(), &out, f); e == nil || out.Len() != 0 {
					t.Fatalf("wrote %d bytes of a file that is not text as %+v", out.Len(), f)
				}
				if _, _, e := s.PrepareEncoding(context.Background(), f, nil); e == nil {
					t.Fatalf("prepared an export of a file that is not text as %+v", f)
				}
			}
			if after := v.Buffer.Metadata(); after.Dirty || after.Revision != meta.Revision || after.CanUndo {
				t.Fatalf("the refused edits left a trace: %+v", after)
			}
		})
	}
	if text := textOf(t, importBytes(t, []byte("A\x00\x01\x7f\x80\xff\tB\r\nC")).Buffer); text != "A␀␁␡€ÿ\tB\nC" {
		t.Fatalf("display text %q", text)
	}
}

// The kept copy of unsaved work holds each line's own ending, so the session
// that reopens it saves the same bytes the first session would have saved.
func TestKeptCopyRestoresMixedLineEndings(t *testing.T) {
	v := importBytes(t, []byte("one\r\ntwo\nthree\r\nfour\r\n"))
	if _, e := v.Buffer.Edit(1, 0, 0, "new\n"); e != nil {
		t.Fatal(e)
	}
	want, _ := encoded(t, v.Buffer, v.Format)
	s, _ := v.Buffer.Snapshot()
	var kept bytes.Buffer
	e := s.WriteKept(context.Background(), &kept)
	s.Close()
	if e != nil || kept.String() != "new\r\none\r\ntwo\nthree\r\nfour\r\n" {
		t.Fatalf("%v %q", e, kept.String())
	}
	restored := importBytes(t, kept.Bytes())
	got, e := encoded(t, restored.Buffer, v.Format)
	if e != nil || !bytes.Equal(got, want) {
		t.Fatalf("restored copy saves %q, want %q (%v)", got, want, e)
	}
	// Any other document is kept as plain normalized text, as before.
	plain := importBytes(t, []byte("a\r\nb\r\n"))
	s, _ = plain.Buffer.Snapshot()
	defer s.Close()
	kept.Reset()
	if e = s.WriteKept(context.Background(), &kept); e != nil || kept.String() != "a\nb\n" {
		t.Fatalf("%v %q", e, kept.String())
	}
}
