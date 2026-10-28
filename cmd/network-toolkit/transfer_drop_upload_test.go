package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func explorerCommit() (func(), error) { return func() {}, nil }
func TestExplorerDropNames(t *testing.T) {
	for _, name := range []string{"a.cfg", "Firmware 界.bin", "empty.txt"} {
		if !validExplorerName(name) {
			t.Fatal(name)
		}
	}
	for _, name := range []string{"", ".", "..", "../escape", "C:\\escape", "CON", "aux.txt", "COM¹.txt", "NUL.cfg", "end.", "end ", "line\nfile", "control\x01", "a:b", strings.Repeat("界", 86)} {
		if validExplorerName(name) {
			t.Fatalf("unsafe name accepted: %q", name)
		}
	}
	for _, name := range []string{"a.lnk", "a.URL"} {
		if validateExplorerDropName(name) == nil {
			t.Fatalf("shortcut accepted: %s", name)
		}
	}
}
func TestExplorerDropAtomicCollisionCancelAndPermissions(t *testing.T) {
	root := t.TempDir()
	old := filepath.Join(root, "same.cfg")
	os.WriteFile(old, []byte("old"), 0600)
	if _, e := writeExplorerDrop(context.Background(), root, "", "same.cfg", false, strings.NewReader("new"), explorerCommit); e == nil {
		t.Fatal("implicit overwrite")
	}
	ctx, cancel := context.WithCancel(context.Background())
	reader := &explorerCancelReader{cancel: cancel}
	if _, e := writeExplorerDrop(ctx, root, "", "same.cfg", true, reader, explorerCommit); e == nil {
		t.Fatal("cancelled copy committed")
	}
	b, _ := os.ReadFile(old)
	if string(b) != "old" {
		t.Fatal("cancel changed existing destination")
	}
	if _, e := writeExplorerDrop(context.Background(), root, "", "same.cfg", true, strings.NewReader("new"), explorerCommit); e != nil {
		t.Fatal(e)
	}
	if _, e := writeExplorerDrop(context.Background(), root, "", "unique.cfg", false, strings.NewReader("copy"), explorerCommit); e != nil {
		t.Fatal(e)
	}
	if _, e := writeExplorerDrop(context.Background(), root, "", "late.cfg", false, strings.NewReader("mine"), func() (func(), error) {
		os.WriteFile(filepath.Join(root, "late.cfg"), []byte("other"), 0600)
		return explorerCommit()
	}); e == nil {
		t.Fatal("late collision overwritten")
	}
	if _, e := writeExplorerDrop(context.Background(), root, "", "root-change.cfg", false, strings.NewReader("mine"), func() (func(), error) { return nil, errors.New("root changed") }); e == nil {
		t.Fatal("root change allowed")
	}
	os.Mkdir(filepath.Join(root, "folder"), 0700)
	if _, e := writeExplorerDrop(context.Background(), root, "", "folder", true, strings.NewReader("mine"), explorerCommit); e == nil {
		t.Fatal("directory replaced")
	}
	entries, _ := os.ReadDir(root)
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".ntk-drop-") {
			t.Fatal("staging leaked after cancellation/failure")
		}
	}
	if runtime.GOOS != "windows" && os.Geteuid() != 0 {
		dir := filepath.Join(root, "denied")
		os.Mkdir(dir, 0500)
		defer os.Chmod(dir, 0700)
		if _, e := writeExplorerDrop(context.Background(), root, "denied", "a.cfg", false, strings.NewReader("no"), explorerCommit); e == nil {
			t.Fatal("permission failure ignored")
		}
	}
}

type explorerCancelReader struct {
	cancel context.CancelFunc
	sent   bool
}

func (r *explorerCancelReader) Read(b []byte) (int, error) {
	if r.sent {
		return 0, io.EOF
	}
	r.sent = true
	n := copy(b, "partial")
	r.cancel()
	return n, nil
}
func TestExplorerDropTraversalAndLinks(t *testing.T) {
	root := t.TempDir()
	for _, sub := range []string{"../outside", "sub/../../outside", "/absolute", "C:\\outside"} {
		if _, e := writeExplorerDrop(context.Background(), root, sub, "a.cfg", false, strings.NewReader("no"), explorerCommit); e == nil {
			t.Fatalf("unsafe path %s", sub)
		}
	}
	outside := t.TempDir()
	if e := os.Symlink(outside, filepath.Join(root, "link")); e == nil {
		if _, e := writeExplorerDrop(context.Background(), root, "link", "a.cfg", false, strings.NewReader("no"), explorerCommit); e == nil {
			t.Fatal("followed folder symlink")
		}
		if _, e := os.Stat(filepath.Join(outside, "a.cfg")); !os.IsNotExist(e) {
			t.Fatal("wrote outside root")
		}
	}
}
func explorerMultipart(t *testing.T, d explorerDropDestination, extra bool) (*bytes.Buffer, string) {
	t.Helper()
	var b bytes.Buffer
	writer := multipart.NewWriter(&b)
	details, _ := json.Marshal(d)
	writer.WriteField("destination", string(details))
	f, _ := writer.CreateFormFile("file", "synthetic.cfg")
	io.WriteString(f, "synthetic bytes")
	if extra {
		f, _ = writer.CreateFormFile("file", "second.cfg")
		io.WriteString(f, "extra")
	}
	writer.Close()
	return &b, writer.FormDataContentType()
}
func TestExplorerDropHTTPBoundary(t *testing.T) {
	a := testApplication(t)
	root := t.TempDir()
	if _, e := a.transfers.setRoot("tftp", root); e != nil {
		t.Fatal(e)
	}
	// The API returns its canonical configured root (Windows TEMP may use an
	// 8.3 alias). A real browser snapshots that exact listing value, not TempDir.
	root = a.transfers.root("tftp")
	for _, tc := range []struct {
		name, origin string
		auth, extra  bool
		dest         explorerDropDestination
		want         int
	}{{"auth", "", false, false, explorerDropDestination{Protocol: "tftp", Root: root, Name: "a.cfg"}, 401}, {"origin", "https://evil.invalid", true, false, explorerDropDestination{Protocol: "tftp", Root: root, Name: "a.cfg"}, 403}, {"root", "", true, false, explorerDropDestination{Protocol: "tftp", Root: root + "-other", Name: "a.cfg"}, 409}, {"multi", "", true, true, explorerDropDestination{Protocol: "tftp", Root: root, Name: "a.cfg"}, 400}, {"copy", "", true, false, explorerDropDestination{Protocol: "tftp", Root: root, Name: "a.cfg"}, 200}} {
		t.Run(tc.name, func(t *testing.T) {
			body, ct := explorerMultipart(t, tc.dest, tc.extra)
			req := httptest.NewRequest(http.MethodPost, "http://"+a.host+"/api/transfer/drop-upload", body)
			req.Header.Set("Content-Type", ct)
			if tc.auth {
				req.Header.Set("X-Session-Token", a.token)
			}
			if tc.origin != "" {
				req.Header.Set("Origin", tc.origin)
			}
			rec := httptest.NewRecorder()
			a.ServeHTTP(rec, req)
			if rec.Code != tc.want {
				t.Fatalf("got %d want %d: %s", rec.Code, tc.want, rec.Body.String())
			}
			if tc.want != 200 {
				if _, e := os.Stat(filepath.Join(root, "a.cfg")); !os.IsNotExist(e) {
					t.Fatal("failed request published a file")
				}
			}
		})
	}
}

func TestExplorerDropChangedFolderBeforePublication(t *testing.T) {
	root := t.TempDir()
	os.Mkdir(filepath.Join(root, "target"), 0700)
	_, e := writeExplorerDrop(context.Background(), root, "target", "a.cfg", false, strings.NewReader("synthetic"), func() (func(), error) {
		if e := os.Rename(filepath.Join(root, "target"), filepath.Join(root, "moved")); e != nil {
			return nil, e
		}
		if e := os.Mkdir(filepath.Join(root, "target"), 0700); e != nil {
			return nil, e
		}
		return explorerCommit()
	})
	if e == nil {
		t.Fatal("published into changed folder")
	}
	for _, dir := range []string{"target", "moved"} {
		if _, e := os.Stat(filepath.Join(root, dir, "a.cfg")); !os.IsNotExist(e) {
			t.Fatal("wrong destination published")
		}
	}
}
