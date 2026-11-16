package main

import (
	"bytes"
	"compress/gzip"
	"testing"

	"networktoolkit/internal/editorbuffer"
)

func gzipBytes(t *testing.T, data []byte) []byte {
	t.Helper()
	var out bytes.Buffer
	w := gzip.NewWriter(&out)
	if _, err := w.Write(data); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return out.Bytes()
}

func TestGunzipEditorChunkIsBounded(t *testing.T) {
	text := bytes.Repeat([]byte("set name \"synthetic\" 192.0.2.1\n"), 2000)
	got, err := gunzipEditorChunk(gzipBytes(t, text))
	if err != nil || !bytes.Equal(got, text) {
		t.Fatalf("a compressed chunk does not come back as it was: %v", err)
	}
	full := bytes.Repeat([]byte{'a'}, editorbuffer.MaxEditBytes)
	if got, err = gunzipEditorChunk(gzipBytes(t, full)); err != nil || len(got) != len(full) {
		t.Fatalf("a chunk of exactly the limit is accepted: %v", err)
	}
	// One byte more than a chunk may hold: refused, however small it travels.
	bomb := gzipBytes(t, bytes.Repeat([]byte{'a'}, editorbuffer.MaxEditBytes+1))
	if len(bomb) > 4096 {
		t.Fatalf("the fixture should be tiny when compressed: %d", len(bomb))
	}
	for name, data := range map[string][]byte{
		"too large":     bomb,
		"far too large": gzipBytes(t, bytes.Repeat([]byte{0}, 64*1024*1024)),
		"not gzip":      []byte("plain text"),
		"empty":         nil,
		"empty member":  gzipBytes(t, nil),
		"truncated":     gzipBytes(t, text)[:40],
	} {
		if got, err := gunzipEditorChunk(data); err == nil {
			t.Errorf("%s: accepted %d bytes", name, len(got))
		}
	}
}

// A browser file sent as compressed chunks is the same document as one sent
// plain, and a compressed chunk that is not what it claims changes nothing.
func TestDiskEditorImportsCompressedChunks(t *testing.T) {
	a, m, _ := editorFixture(t)
	r := editorHTTP(a, a.token, "/api/editor-views/buffers/open", map[string]any{"document": emptyDiskFixture(), "mode": "upload"})
	if r.Code != 202 {
		t.Fatal(r.Code, r.Body)
	}
	defer m.releaseDocument("", editorFixtureUID, 1)
	call := func(action string, fields map[string]any) int {
		t.Helper()
		fields["action"] = action
		return diskCall(a, a.token, 1, fields).Code
	}
	raw := bytes.Repeat([]byte("interface ge-0/0/1\r\n description synthetic λ 192.0.2.1\r\n"), 4000)
	half := len(raw) / 2
	if code := call("import-chunk", map[string]any{"offset": 0, "data": []byte("not gzip"), "gzip": true}); code != 400 {
		t.Fatalf("a chunk marked compressed that is not gzip: %d", code)
	}
	if code := call("import-chunk", map[string]any{"offset": 0, "data": gzipBytes(t, raw[:half]), "gzip": true}); code != 200 {
		t.Fatalf("first compressed chunk: %d", code)
	}
	// A retry of the same chunk, compressed again, is accepted as the same bytes.
	if code := call("import-chunk", map[string]any{"offset": 0, "data": gzipBytes(t, raw[:half]), "gzip": true}); code != 200 {
		t.Fatalf("retried compressed chunk: %d", code)
	}
	// The rest plain: both kinds may be mixed in one upload.
	if code := call("import-chunk", map[string]any{"offset": half, "data": raw[half:]}); code != 200 {
		t.Fatalf("plain chunk after a compressed one: %d", code)
	}
	if code := call("import-finish", map[string]any{"size": len(raw)}); code != 202 {
		t.Fatalf("finish: %d", code)
	}
	if r = waitDiskReady(t, a, 1); !bytes.Contains(r.Body.Bytes(), []byte(editorDigest(raw))) {
		t.Fatal("the imported document is not the file that was sent", r.Body)
	}
}
