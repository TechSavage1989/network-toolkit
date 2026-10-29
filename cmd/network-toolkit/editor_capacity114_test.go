package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestEditorCapacity114EncodedBoundaryAndUnicodeRoundTrip(t *testing.T) {
	if textEditorLimit != 32<<20 || editorDocumentLimit != 64<<20 || editorTransferLimit != 256<<20 {
		t.Fatal("capacity contract changed")
	}
	for _, encoding := range []string{"UTF-8", "UTF-16LE", "UTF-16BE"} {
		t.Run(encoding, func(t *testing.T) {
			text := strings.Repeat("SYNTHETIC 日本語 😀\r\n", 600000)
			encoded, err := encodeEditableText(text, encoding, true)
			if err != nil {
				t.Fatal(err)
			}
			if len(encoded) <= 8<<20 {
				t.Fatal("fixture must exceed former limit")
			}
			got, kind, bom, err := decodeEditableText(encoded)
			if err != nil || got != text || kind != encoding || !bom {
				t.Fatal("Unicode/BOM/EOL roundtrip changed", err)
			}
		})
	}
	exact := bytes.Repeat([]byte{'x'}, textEditorLimit)
	if _, err := readTextBytes(exact); err != nil {
		t.Fatal(err)
	}
	if _, err := readTextBytes(append(exact, 'x')); err == nil {
		t.Fatal("accepted oversized encoded file")
	}
}
func TestEditorCapacity114LocalHTTPLargeExactSave(t *testing.T) {
	path := filepath.Join(t.TempDir(), "synthetic-large.txt")
	text := strings.Repeat("SYNTHETIC capacity114\r\n", 800000)
	body, _ := json.Marshal(map[string]any{"action": "write-text", "path": path, "content": text, "encoding": "UTF-8", "bom": true})
	req := httptest.NewRequest(http.MethodPost, "/api/file-editor/local", bytes.NewReader(body))
	w := httptest.NewRecorder()
	(&application{}).handleLocalTextEditor(w, req)
	if w.Code != 200 {
		t.Fatalf("large save rejected: %d %s", w.Code, w.Body.String())
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	want := append([]byte{0xef, 0xbb, 0xbf}, []byte(text)...)
	if !bytes.Equal(raw, want) {
		t.Fatal("saved content truncated or changed")
	}
	got, err := readEditableFile(path)
	if err != nil || got["content"] != text || got["hash"] != editorDigest(want) {
		t.Fatal("large read does not roundtrip", err)
	}
}
func TestEditorCapacity114CloneIsolation(t *testing.T) {
	b := editorFixtureBatch()
	b.Documents[0].Content = strings.Repeat("synthetic ", 2<<20)
	b.Documents[0].View = &editorSelection{Start: 1, End: 2, Direction: "forward"}
	b.Documents[0].CompareView = &editorSelection{Start: 2, End: 3, Direction: "backward"}
	b.Layout = &editorLayout{Groups: []editorTabGroup{{UID: editorFixtureUID, Side: "left"}}}
	b.Preferences = &editorPreferences{Colors: map[string]string{"bg": "#111111"}}
	b.Compare = &editorCompare{Left: "before"}
	cloned := editorCloneBatch(b)
	cloned.Documents[0].Content = "changed"
	cloned.Documents[0].Undo[0] = "changed"
	cloned.Documents[0].View.Start = 9
	cloned.Documents[0].CompareView.Start = 9
	cloned.Layout.Groups[0].Side = "right"
	cloned.Preferences.Colors["bg"] = "#222222"
	cloned.Compare.Left = "changed"
	if b.Documents[0].Content == "changed" || b.Documents[0].Undo[0] == "changed" || b.Documents[0].View.Start != 1 || b.Documents[0].CompareView.Start != 2 || b.Layout.Groups[0].Side != "left" || b.Preferences.Colors["bg"] != "#111111" || b.Compare.Left != "before" {
		t.Fatal("mutable snapshot alias escaped")
	}
}
func BenchmarkEditorCapacity114Clone32MiB(b *testing.B) {
	batch := editorFixtureBatch()
	batch.Documents[0].Content = strings.Repeat("x", 32<<20)
	batch.Documents[0].Original = batch.Documents[0].Content
	batch.Documents[0].Saved = batch.Documents[0].Content
	batch.Documents[0].Undo = []string{batch.Documents[0].Content}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = editorCloneBatch(batch)
	}
}

func TestEditorCapacity114EscapingBudget(t *testing.T) {
	if textEditorRequestLimit != textEditorLimit*6+16384 {
		t.Fatal("request cannot carry worst-case JSON escaping")
	}
	content := strings.Repeat("\x01", (2<<20)+1) // >12 MiB JSON; former request guard rejected this.
	path := filepath.Join(t.TempDir(), "synthetic-escaped.txt")
	raw, _ := json.Marshal(map[string]any{"action": "write-text", "path": path, "content": content, "encoding": "UTF-8"})
	if len(raw) <= 12<<20 {
		t.Fatal("fixture did not cross old request limit")
	}
	w := httptest.NewRecorder()
	(&application{}).handleLocalTextEditor(w, httptest.NewRequest(http.MethodPost, "/api/file-editor/local", bytes.NewReader(raw)))
	if w.Code != 200 {
		t.Fatalf("escaped text rejected: %d %s", w.Code, w.Body.String())
	}
	got, err := os.ReadFile(path)
	if err != nil || string(got) != content {
		t.Fatal("escaped text not exact", err)
	}
}

func TestEditorCapacity114WorstCaseRequestMemory(t *testing.T) {
	if os.Getenv("NETWORK_TOOLKIT_CAPACITY_STRESS114") != "1" {
		t.Skip("explicit isolated memory measurement")
	}
	var peak atomic.Uint64
	done := make(chan struct{})
	go func() {
		tick := time.NewTicker(10 * time.Millisecond)
		defer tick.Stop()
		for {
			select {
			case <-done:
				return
			case <-tick.C:
				var m runtime.MemStats
				runtime.ReadMemStats(&m)
				for old := peak.Load(); m.HeapAlloc > old && !peak.CompareAndSwap(old, m.HeapAlloc); old = peak.Load() {
				}
			}
		}
	}()
	defer close(done)
	char := "\x01"
	if os.Getenv("NETWORK_TOOLKIT_CAPACITY_STRESS_CONTENT") == "ascii" {
		char = "x"
	}
	content := strings.Repeat(char, textEditorLimit)
	p := filepath.Join(t.TempDir(), "synthetic-worstcase.txt")
	started := time.Now()
	raw, err := json.Marshal(map[string]any{"action": "write-text", "path": p, "content": content, "encoding": "UTF-8"})
	if err != nil {
		t.Fatal(err)
	}
	if len(raw) > textEditorRequestLimit {
		t.Fatal("guard too small")
	}
	w := httptest.NewRecorder()
	(&application{}).handleLocalTextEditor(w, httptest.NewRequest(http.MethodPost, "/api/file-editor/local", bytes.NewReader(raw)))
	if w.Code != 200 {
		t.Fatalf("%d %s", w.Code, w.Body.String())
	}
	got, err := os.ReadFile(p)
	if err != nil || string(got) != content {
		t.Fatal("worst-case exact save failed", err)
	}
	t.Logf("encodedBytes=%d requestBytes=%d peakObservedHeapBytes=%d elapsedMs=%d", len(content), len(raw), peak.Load(), time.Since(started).Milliseconds())
}
func BenchmarkEditorCapacity114LegacyClone4MiB(b *testing.B) {
	batch := editorFixtureBatch()
	batch.Documents[0].Content = strings.Repeat("x", 4<<20)
	batch.Documents[0].Original = batch.Documents[0].Content
	batch.Documents[0].Saved = batch.Documents[0].Content
	batch.Documents[0].Undo = []string{batch.Documents[0].Content}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		raw, _ := json.Marshal(batch)
		var copy editorBatch
		_ = json.Unmarshal(raw, &copy)
	}
}
func BenchmarkEditorCapacity114Clone4MiB(b *testing.B) {
	batch := editorFixtureBatch()
	batch.Documents[0].Content = strings.Repeat("x", 4<<20)
	batch.Documents[0].Original = batch.Documents[0].Content
	batch.Documents[0].Saved = batch.Documents[0].Content
	batch.Documents[0].Undo = []string{batch.Documents[0].Content}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = editorCloneBatch(batch)
	}
}

func TestEditorCapacity114OversizeWriteDoesNotCreate(t *testing.T) {
	p := filepath.Join(t.TempDir(), "must-not-exist.txt")
	if _, err := saveEditableLocal(p, bytes.Repeat([]byte{'x'}, textEditorLimit+1), ""); err == nil {
		t.Fatal("accepted one byte over encoded limit")
	}
	if _, err := os.Stat(p); !os.IsNotExist(err) {
		t.Fatal("oversize write changed destination", err)
	}
}

func TestEditorCapacity114ClonePreservesEmptyArrays(t *testing.T) {
	for _, history := range [][]string{nil, {}, {"synthetic"}} {
		b := editorFixtureBatch()
		b.Documents[0].Undo = history
		b.Documents[0].Redo = history
		b.Layout = &editorLayout{Groups: []editorTabGroup{}}
		before, _ := json.Marshal(b)
		after, _ := json.Marshal(editorCloneBatch(b))
		if !bytes.Equal(before, after) {
			t.Fatalf("snapshot wire shape changed: before=%s after=%s", before, after)
		}
	}
}
