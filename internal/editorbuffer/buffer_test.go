package editorbuffer

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"math/rand"
	"os"
	"runtime"
	"strings"
	"testing"
	"time"
	"unicode/utf16"
)

func openTest(t *testing.T, text string) *Buffer {
	t.Helper()
	v, e := Import(context.Background(), strings.NewReader(text), t.TempDir(), nil)
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { v.Buffer.Close() })
	return v.Buffer
}
func textOf(t *testing.T, b *Buffer) string {
	t.Helper()
	s, e := b.Snapshot()
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	var out bytes.Buffer
	if e = s.WriteTo(context.Background(), &out); e != nil {
		t.Fatal(e)
	}
	return out.String()
}
func units(text string) int64 { return int64(len(utf16.Encode([]rune(text)))) }
func TestRandomEditsHistoryAndExactSave(t *testing.T) {
	text := strings.Repeat("interface α😀\n", 10000)
	b := openTest(t, text)
	original := text
	rng := rand.New(rand.NewSource(7))
	history := []string{text}
	for range 400 {
		r := []rune(text)
		a := rng.Intn(len(r) + 1)
		z := a + rng.Intn(min(40, len(r)-a)+1)
		insert := []string{"X", "\n", "λ😀\n", ""}[rng.Intn(4)]
		m, e := b.Edit(b.Metadata().Revision, units(string(r[:a])), units(string(r[:z])), insert)
		if e != nil {
			t.Fatal(e)
		}
		next := string(r[:a]) + insert + string(r[z:])
		if next != text || a != z || insert != "" {
			history = append(history, next)
		}
		text = next
		if m.Units != units(text) || m.Lines != int64(strings.Count(text, "\n"))+1 {
			t.Fatalf("wrong metadata: %+v", m)
		}
	}
	if got := textOf(t, b); got != text {
		t.Fatal("edited bytes changed")
	}
	for i := len(history) - 2; i >= 0; i-- {
		if _, e := b.History(b.Metadata().Revision, false); e != nil {
			t.Fatal(e)
		}
		if got := textOf(t, b); got != history[i] {
			t.Fatalf("undo %d differs", i)
		}
	}
	if textOf(t, b) != original || b.Metadata().Dirty {
		t.Fatal("undo did not restore saved revision")
	}
	for i := 1; i < len(history); i++ {
		b.History(b.Metadata().Revision, true)
	}
	if textOf(t, b) != text {
		t.Fatal("redo changed bytes")
	}
}
func TestBoundariesAndStaleRevisionDoNotMutate(t *testing.T) {
	b := openTest(t, "A😀\nB")
	before := b.Metadata()
	for _, r := range []struct {
		revision uint64
		from, to int64
		text     string
	}{{before.Revision, 2, 2, "x"}, {before.Revision, 0, 2, "x"}, {0, 0, 0, "x"}, {before.Revision, 0, 0, "\x00"}, {before.Revision, 0, 0, strings.Repeat("x", MaxEditBytes+1)}} {
		if _, e := b.Edit(r.revision, r.from, r.to, r.text); e == nil {
			t.Fatal("invalid edit accepted")
		}
		if b.Metadata() != before || textOf(t, b) != "A😀\nB" {
			t.Fatal("rejected edit mutated document")
		}
	}
	w, e := b.Window(0, 2)
	if e != nil || w.Text != "A" {
		t.Fatalf("window split Unicode: %+v %v", w, e)
	}
}
func TestPagedLinesAndVeryLongLine(t *testing.T) {
	text := strings.Repeat("x", 4*ChunkBytes) + "😀\n" + strings.Repeat("界\n", 100000)
	b := openTest(t, text)
	for _, line := range []int64{0, 1, 50000, 100000, 100001} {
		w, e := b.WindowAtLine(line, 100)
		if e != nil {
			t.Fatal(e)
		}
		if w.Line != line || units(w.Text) > 100 {
			t.Fatalf("bad line window: %+v", w)
		}
		if line == 0 && !strings.HasPrefix(w.Text, "xxx") {
			t.Fatal("long line missing")
		}
		if line > 0 && line < 100001 && !strings.HasPrefix(w.Text, "界\n") {
			t.Fatal("line index mismatch")
		}
	}
}
func TestSnapshotSaveDoesNotBlockOrLoseNewerEdits(t *testing.T) {
	b := openTest(t, "before\n")
	s, e := b.Snapshot()
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	m, e := b.Edit(b.Metadata().Revision, 0, 6, "after")
	if e != nil {
		t.Fatal(e)
	}
	var out bytes.Buffer
	if e = s.Encode(context.Background(), &out, Format{"UTF-8", false, "CRLF"}); e != nil {
		t.Fatal(e)
	}
	if out.String() != "before\r\n" {
		t.Fatal("save changed its revision")
	}
	saved := s.MarkSaved()
	if !saved.Dirty || saved.Revision != m.Revision || textOf(t, b) != "after\n" {
		t.Fatal("save erased newer edits")
	}
}
func TestEncodingRoundTripsAcrossChunks(t *testing.T) {
	source := strings.Repeat("abλ😀\n", 20000) + "tail"
	b := openTest(t, source)
	for _, encoding := range []string{"UTF-8", "UTF-16LE", "UTF-16BE"} {
		for _, eol := range []string{"LF", "CRLF", "CR"} {
			for _, bom := range []bool{false, true} {
				if encoding != "UTF-8" && !bom {
					continue
				}
				f := Format{encoding, bom, eol}
				s, _ := b.Snapshot()
				var out bytes.Buffer
				e := s.Encode(context.Background(), &out, f)
				s.Close()
				if e != nil {
					t.Fatal(e)
				}
				expected := sha256.Sum256(out.Bytes())
				v, e := Import(context.Background(), bytes.NewReader(out.Bytes()), t.TempDir(), nil)
				if e != nil {
					t.Fatal(e)
				}
				if v.Format != f || v.Hash != hex.EncodeToString(expected[:]) || textOf(t, v.Buffer) != source {
					t.Fatalf("roundtrip failed: %+v", f)
				}
				v.Buffer.Close()
			}
		}
	}
}
func TestInvalidEncodingAndCancellationCleanScratch(t *testing.T) {
	for _, raw := range [][]byte{{0xff}, {0}, {0xff, 0xfe, 0}, {0xff, 0xfe, 0, 0xd8}, {0xfe, 0xff, 0xdc, 0}, {0xef, 0xbb, 0xbf, 0xc0, 0xaf}} {
		dir := t.TempDir()
		if _, e := Import(context.Background(), bytes.NewReader(raw), dir, nil); e == nil {
			t.Fatal("invalid Unicode accepted")
		}
		files, _ := os.ReadDir(dir)
		if len(files) != 0 {
			t.Fatal("failed import leaked scratch")
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	dir := t.TempDir()
	_, e := Import(ctx, strings.NewReader(strings.Repeat("x", ChunkBytes*4)), dir, func(int64) { cancel() })
	if !errors.Is(e, context.Canceled) {
		t.Fatal(e)
	}
	files, _ := os.ReadDir(dir)
	if len(files) != 0 {
		t.Fatal("cancelled import leaked scratch")
	}
}

type repeatedReader struct {
	remaining int64
	pattern   []byte
	at        int
}

func (r *repeatedReader) Read(p []byte) (int, error) {
	if r.remaining == 0 {
		return 0, io.EOF
	}
	n := min(int64(len(p)), r.remaining)
	for i := range int(n) {
		p[i] = r.pattern[r.at]
		r.at = (r.at + 1) % len(r.pattern)
	}
	r.remaining -= n
	return int(n), nil
}
func TestFileBeyondFormerSizeLimit(t *testing.T) {
	if testing.Short() {
		t.Skip("large disk fixture")
	}
	const length = 64 * 1024 * 1024
	v, e := Import(context.Background(), &repeatedReader{remaining: length, pattern: []byte("fixture\n")}, t.TempDir(), nil)
	if e != nil {
		t.Fatal(e)
	}
	defer v.Buffer.Close()
	b := v.Buffer
	m := b.Metadata()
	if m.Units != length || m.Lines != length/8+1 {
		t.Fatalf("bad large metadata: %+v", m)
	}
	m, e = b.Edit(m.Revision, length-8, length, "changed\n")
	if e != nil {
		t.Fatal(e)
	}
	w, e := b.WindowAtLine(m.Lines-2, 100)
	if e != nil || w.Text != "changed\n" {
		t.Fatalf("bad final page: %+v %v", w, e)
	}
	s, _ := b.Snapshot()
	defer s.Close()
	hash := sha256.New()
	if e = s.WriteTo(context.Background(), hash); e != nil {
		t.Fatal(e)
	}
	expected := sha256.New()
	io.CopyN(expected, &repeatedReader{remaining: length - 8, pattern: []byte("fixture\n")}, length-8)
	expected.Write([]byte("changed\n"))
	if !bytes.Equal(hash.Sum(nil), expected.Sum(nil)) {
		t.Fatal("large save changed unrelated bytes")
	}
}

func TestRepeatedEditsKeepAccessBalanced(t *testing.T) {
	b := openTest(t, strings.Repeat("x", ChunkBytes))
	text := strings.Repeat("x", ChunkBytes)
	for i := 0; i < 4000; i++ {
		at := int64(i * 3)
		if _, e := b.Edit(b.Metadata().Revision, at, at, "y"); e != nil {
			t.Fatal(e)
		}
		text = text[:at] + "y" + text[at:]
	}
	if textOf(t, b) != text {
		t.Fatal("repeated inserts changed source")
	}
	var verify func(*node) int
	verify = func(n *node) int {
		if n == nil {
			return 0
		}
		a, z := verify(n.left), verify(n.right)
		if a-z > 1 || z-a > 1 {
			t.Fatalf("unbalanced editor tree: %d %d", a, z)
		}
		return 1 + max(a, z)
	}
	if verify(b.root) > 24 {
		t.Fatal("editing degraded into a linear scan")
	}
}

func BenchmarkLargeFileInteraction(b *testing.B) {
	for _, length := range []int64{256 << 20, 2 << 30} {
		name := "256MiB"
		if length > 1<<30 {
			name = "2GiB"
		}
		b.Run(name, func(b *testing.B) {
			for range b.N {
				b.StopTimer()
				runtime.GC()
				var before, after runtime.MemStats
				runtime.ReadMemStats(&before)
				start := time.Now()
				v, e := Import(context.Background(), &repeatedReader{remaining: length, pattern: []byte("fixture\n")}, b.TempDir(), nil)
				if e != nil {
					b.Fatal(e)
				}
				load := time.Since(start)
				runtime.GC()
				runtime.ReadMemStats(&after)
				b.ReportMetric(float64(load.Milliseconds()), "import-ms")
				if after.HeapAlloc >= before.HeapAlloc {
					b.ReportMetric(float64(after.HeapAlloc-before.HeapAlloc)/(1024*1024), "retained-MiB")
				}
				b.StartTimer()
				for i := 0; i < 100; i++ {
					at := int64(i) * length / 100
					m := v.Buffer.Metadata()
					if _, e = v.Buffer.Edit(m.Revision, at, at, "edit\n"); e != nil {
						b.Fatal(e)
					}
					if _, e = v.Buffer.WindowAtLine(int64(i)*(length/8)/100, 4096); e != nil {
						b.Fatal(e)
					}
				}
				b.StopTimer()
				v.Buffer.Close()
			}
		})
	}
}
