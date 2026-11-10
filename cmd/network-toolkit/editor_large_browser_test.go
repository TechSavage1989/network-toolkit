package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"networktoolkit/internal/model"
)

// Exercise the shipped page and authenticated document handlers against real
// generated files. Unrelated engine routes use synthetic replies, so startup
// never inspects the machine's network, shell sessions or stored settings.
func TestEditorLargeFileBrowser(t *testing.T) {
	if os.Getenv("NTK_EDITOR_LARGE_BROWSER") != "1" {
		t.Skip("requires the isolated Playwright browser")
	}
	const head, tail = "HEAD-VERIFIED", "TAIL-VERIFIED"
	const inserted = "    set alias synthetic-added\n    set status enable\n"
	const blockSize = 1 << 20
	size := int64(2 << 30)
	if value := os.Getenv("NTK_EDITOR_FIXTURE_BYTES"); value != "" {
		var err error
		size, err = strconv.ParseInt(value, 10, 64)
		if err != nil || size < blockSize || size%blockSize != 0 {
			t.Fatal("fixture size must be a positive multiple of 1 MiB")
		}
	}
	dir := t.TempDir()
	left, right := filepath.Join(dir, "before.txt"), filepath.Join(dir, "edited.txt")
	file, err := os.Create(left)
	if err != nil {
		t.Fatal(err)
	}
	block := bytes.Repeat([]byte(strings.Repeat("x", 1023)+"\n"), blockSize/1024)
	firstBlock := bytes.Clone(block)
	copy(firstBlock, "#config-version=FGTTEST-7.4.0 synthetic ")
	copy(firstBlock[1024:], "config system global ")
	want := sha256.New()
	_, _ = io.WriteString(want, head)
	for remaining := size; remaining > 0; remaining -= blockSize {
		data := block
		if remaining == size {
			data = firstBlock
			_, _ = want.Write(data[:1024])
			_, _ = io.WriteString(want, inserted)
			_, _ = want.Write(data[1024:])
		} else {
			_, _ = want.Write(data)
		}
		if _, err := file.Write(data); err != nil {
			file.Close()
			t.Fatal(err)
		}
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	if err := os.Link(left, right); err != nil {
		t.Fatal(err)
	}
	_, _ = io.WriteString(want, tail)

	a, registry, _ := editorFixture(t)
	a.windowMode = "main"
	shellFile := filepath.Join(dir, "résumé & notes %[v2].txt")
	const shellText = "Opened from the synthetic Explorer launch.\n"
	if err := os.WriteFile(shellFile, []byte(shellText), 0600); err != nil {
		t.Fatal(err)
	}
	shellFile, err = editorLaunchPath(shellFile)
	if err != nil {
		t.Fatal(err)
	}
	a.editorLaunchAvailable = true
	a.editorLaunchInbox.add(editorLaunchRequest{Path: shellFile})
	var saveReturned atomic.Bool
	var editsDuringSave atomic.Int64
	var releaseOnce sync.Once
	releaseSave := make(chan struct{})
	releaseReply := func() { releaseOnce.Do(func() { close(releaseSave) }) }
	savingSnapshot := func() bool {
		registry.mu.Lock()
		defer registry.mu.Unlock()
		for _, grant := range registry.documents {
			// filePath is reserved only after taking the immutable save snapshot.
			if grant.filePath == right {
				return true
			}
		}
		return false
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api/status":
			jsonResponse(w, 200, map[string]any{"version": model.Version, "os": runtime.GOOS, "arch": runtime.GOARCH, "editorWindowAvailable": true, "editorDiskAvailable": true, "editorLaunchAvailable": true, "uiHost": "browser", "system": map[string]any{}})
		case r.URL.Path == "/api/state":
			jsonResponse(w, 200, map[string]any{"activePage": "overview", "appUpdatePreferences": map[string]any{"enabled": false}})
		case r.URL.Path == "/api/jobs":
			jsonResponse(w, 200, []any{})
		case r.URL.Path == "/api/editor-fixture/state":
			if r.Header.Get("X-Session-Token") != a.token {
				jsonError(w, http.StatusForbidden, "Fixture session required.")
				return
			}
			registry.mu.Lock()
			grants, comparisons := len(registry.documents), len(registry.diskComparisons)
			registry.mu.Unlock()
			jsonResponse(w, http.StatusOK, map[string]any{
				"snapshotWriting": savingSnapshot(), "saveReturned": saveReturned.Load(),
				"editsDuringSave": editsDuringSave.Load(), "grants": grants, "comparisons": comparisons,
			})
		case r.URL.Path == "/api/editor-fixture/release-save":
			if r.Header.Get("X-Session-Token") != a.token {
				jsonError(w, http.StatusForbidden, "Fixture session required.")
				return
			}
			releaseReply()
			jsonResponse(w, http.StatusOK, map[string]bool{"ok": true})
		case strings.HasPrefix(r.URL.Path, "/api/editor-views/documents/") && strings.HasSuffix(r.URL.Path, "/buffer") && r.Method == http.MethodPost:
			body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 8<<20))
			if err != nil {
				jsonError(w, http.StatusBadRequest, "Cannot read fixture request.")
				return
			}
			r.Body.Close()
			r.Body = io.NopCloser(bytes.NewReader(body))
			var req struct{ Action string }
			_ = json.Unmarshal(body, &req)
			if req.Action == "edit" && savingSnapshot() {
				editsDuringSave.Add(1)
			}
			if req.Action != "save" {
				a.ServeHTTP(w, r)
				return
			}
			// Hold only the acknowledgement, never the save itself. Even the
			// smallest smoke fixture can test an edit before a save reply.
			response := httptest.NewRecorder()
			a.ServeHTTP(response, r)
			saveReturned.Store(true)
			select {
			case <-releaseSave:
			case <-r.Context().Done():
				return
			}
			for key, values := range response.Header() {
				w.Header()[key] = values
			}
			w.WriteHeader(response.Code)
			w.Write(response.Body.Bytes())
		case strings.HasPrefix(r.URL.Path, "/api/editor-views"), r.URL.Path == "/api/file-editor/local", r.URL.Path == editorLaunchEndpoint, !strings.HasPrefix(r.URL.Path, "/api/"):
			a.ServeHTTP(w, r)
		default:
			jsonResponse(w, 200, map[string]any{"sessions": []any{}, "entries": []any{}})
		}
	}))
	defer func() {
		releaseReply()
		server.Close()
		// Also clean up deterministically when browser assertions fail before
		// the normal Close All action reaches the registry.
		registry.mu.Lock()
		for _, job := range registry.diskComparisons {
			job.discard()
		}
		clear(registry.diskComparisons)
		var buffers []*editorDiskDocument
		for _, grant := range registry.documents {
			if grant.buffer != nil {
				buffers = append(buffers, grant.buffer)
			}
		}
		clear(registry.documents)
		registry.mu.Unlock()
		for _, buffer := range buffers {
			buffer.close()
		}
	}()
	a.host = strings.TrimPrefix(server.URL, "http://")

	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Minute)
	defer cancel()
	command := exec.CommandContext(ctx, "node", "scripts/editor-large-file.browser.cjs")
	command.Dir = filepath.Join("..", "..")
	command.Env = append(os.Environ(), "NTK_EDITOR_FIXTURE_URL="+server.URL, "NTK_EDITOR_FIXTURE_TOKEN="+a.token,
		"NTK_EDITOR_FIXTURE_LEFT="+left, "NTK_EDITOR_FIXTURE_RIGHT="+right, "NTK_EDITOR_FIXTURE_SIZE="+strconv.FormatInt(size, 10),
		"NTK_EDITOR_FIXTURE_HEAD="+head, "NTK_EDITOR_FIXTURE_TAIL="+tail, "NTK_EDITOR_FIXTURE_SHELL="+shellFile, "NTK_EDITOR_FIXTURE_SHELL_TEXT="+shellText)
	output, err := command.CombinedOutput()
	t.Log(string(output))
	if err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(right)
	if err != nil || info.Size() != size+int64(len(head)+len(tail)+len(inserted)) {
		t.Fatalf("saved file size: %v, %v", info, err)
	}
	file, err = os.Open(right)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	got := sha256.New()
	if _, err := io.Copy(got, file); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got.Sum(nil), want.Sum(nil)) {
		t.Fatalf("saved file bytes differ: got %x, want %x", got.Sum(nil), want.Sum(nil))
	}
	before, err := os.Stat(left)
	if err != nil || before.Size() != size {
		t.Fatalf("comparison source changed: %v, %v", before, err)
	}
	registry.mu.Lock()
	grants, comparisons := len(registry.documents), len(registry.diskComparisons)
	registry.mu.Unlock()
	if grants != 0 || comparisons != 0 {
		t.Fatalf("fixture left %d document grants and %d comparison jobs", grants, comparisons)
	}
	if size >= 2<<30 && editsDuringSave.Load() == 0 {
		t.Fatal("no edit request overlapped the multi-gigabyte save")
	}
	t.Logf("verified exact SHA-256 %x over %d saved bytes", got.Sum(nil), info.Size())
}
