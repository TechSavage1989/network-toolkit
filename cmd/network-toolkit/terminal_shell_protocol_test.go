package main

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"net/http/httptest"
	"net/url"
	"networktoolkit/internal/terminal"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestTerminalShellProtocolBoundsAndCommands(t *testing.T) {
	good := terminalShellFrame{Version: 1, Kind: "start", Options: &terminal.LocalShellOptions{Type: "cmd", Cols: 80, Rows: 24}}
	if err := validateTerminalShellStart(good); err != nil {
		t.Fatal(err)
	}
	for _, mutate := range []func(*terminalShellFrame){func(f *terminalShellFrame) { f.Version = 2 }, func(f *terminalShellFrame) { f.Kind = "exec" }, func(f *terminalShellFrame) { f.Data = []byte("echo unsafe") }, func(f *terminalShellFrame) {
		f.Options = &terminal.LocalShellOptions{Type: "C:\\evil.exe", Cols: 80, Rows: 24}
	}, func(f *terminalShellFrame) { f.State = &terminal.LocalShellState{Elevated: true} }} {
		f := good
		mutate(&f)
		if validateTerminalShellStart(f) == nil {
			t.Fatal("invalid startup accepted", f)
		}
	}
	for _, f := range []terminalShellFrame{{Version: 1, Kind: "input", Data: []byte("test")}, {Version: 1, Kind: "resize", Cols: 100, Rows: 25}, {Version: 1, Kind: "close"}} {
		if err := validateTerminalShellControl(f); err != nil {
			t.Fatal(err)
		}
	}
	for _, f := range []terminalShellFrame{{Version: 1, Kind: "input", Data: make([]byte, (16<<10)+1)}, {Version: 1, Kind: "input"}, {Version: 1, Kind: "resize", Cols: 0, Rows: 25}, {Version: 1, Kind: "exec"}, {Version: 1, Kind: "close", Options: good.Options}} {
		if validateTerminalShellControl(f) == nil {
			t.Fatal("invalid control accepted", f.Kind)
		}
	}
	var b bytes.Buffer
	if err := writeTerminalShellFrame(&b, good); err != nil {
		t.Fatal(err)
	}
	decoded, err := readTerminalShellFrame(&b)
	if err != nil || decoded.Options.Type != "cmd" {
		t.Fatal(decoded, err)
	}
	for _, body := range []string{`null`, `{"version":1,"kind":"close","command":"bad"}`, `{"version":1} {}`, `{`} {
		b.Reset()
		binary.Write(&b, binary.LittleEndian, uint32(len(body)))
		b.WriteString(body)
		f, e := readTerminalShellFrame(&b)
		if e == nil && validateTerminalShellControl(f) == nil {
			t.Fatal("invalid frame accepted", body)
		}
	}
	b.Reset()
	binary.Write(&b, binary.LittleEndian, uint32(terminalShellFrameLimit+1))
	if _, err = readTerminalShellFrame(&b); err == nil {
		t.Fatal("oversize frame accepted")
	}
	if err = writeTerminalShellFrame(&b, terminalShellFrame{Version: 1, Kind: "output", Data: []byte(strings.Repeat("x", terminalShellFrameLimit))}); err == nil {
		t.Fatal("oversize write accepted")
	}
}
func TestTerminalLocalDirectoryChooserEmptyAndNoFileContents(t *testing.T) {
	app := testApplication(t)
	dir := t.TempDir()
	request := func(query string) *httptest.ResponseRecorder {
		r := httptest.NewRequest("GET", "/api/terminals/local-directories?"+query, nil)
		w := httptest.NewRecorder()
		app.handleTerminalLocalDirectories(w, r)
		return w
	}
	w := request("path=" + url.QueryEscape(dir))
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	os.Mkdir(filepath.Join(dir, "subfolder"), 0700)
	os.WriteFile(filepath.Join(dir, "secret.txt"), []byte("SYNTHETIC_PRIVATE_CONTENT"), 0600)
	w = request("path=" + url.QueryEscape(dir))
	if w.Code != 200 || strings.Contains(w.Body.String(), "secret.txt") || strings.Contains(w.Body.String(), "SYNTHETIC_PRIVATE_CONTENT") {
		t.Fatal(w.Code, w.Body.String())
	}
	var body struct{ Entries []struct{ Name string } }
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil || len(body.Entries) != 1 || body.Entries[0].Name != "subfolder" {
		t.Fatal(w.Body.String(), err)
	}
	if got := requestApp(app, "GET", "/api/terminals/local-directories?path="+url.QueryEscape(dir), "", false); got.Code != 401 {
		t.Fatal("unauthenticated directory listing", got.Code)
	}
	child := httptest.NewRequest("GET", "/api/terminals/local-directories?path="+url.QueryEscape(dir), nil)
	child = child.WithContext(context.WithValue(child.Context(), terminalViewContextKey{}, "single-child"))
	app.views.mu.Lock()
	app.views.views["single-child"] = &terminalView{info: terminalViewInfo{ID: "single-child", State: "detached", Workspace: false}}
	app.views.mu.Unlock()
	denied := httptest.NewRecorder()
	app.serveDetachedAPI(denied, child, "single-child")
	if denied.Code != 403 {
		t.Fatal("single-session child can browse", denied.Code)
	}
	app.views.mu.Lock()
	app.views.views["single-child"].info.Workspace = true
	app.views.mu.Unlock()
	allowed := httptest.NewRecorder()
	app.serveDetachedAPI(allowed, child, "single-child")
	if allowed.Code != 200 {
		t.Fatal("ready workspace cannot choose directory", allowed.Code)
	}
	for _, phase := range []string{"starting", "return_requested", "returned", "closing"} {
		app.views.mu.Lock()
		app.views.views["single-child"].info.State = phase
		app.views.mu.Unlock()
		denied = httptest.NewRecorder()
		app.serveDetachedAPI(denied, child, "single-child")
		if denied.Code != 403 || strings.Contains(denied.Body.String(), "subfolder") {
			t.Fatal("nonready workspace can browse", phase, denied.Code)
		}
	}
	if w = request("preview=" + dir); w.Code != 400 {
		t.Fatal("preview admitted", w.Code)
	}
	r := httptest.NewRequest("POST", "/api/terminals/local-directories", nil)
	w = httptest.NewRecorder()
	app.handleTerminalLocalDirectories(w, r)
	if w.Code != 405 {
		t.Fatal(w.Code)
	}
}

func TestTerminalShellPeerAndReadyFailClosed(t *testing.T) {
	if terminalShellPeerMatches(0, 0) || terminalShellPeerMatches(7, 8) || !terminalShellPeerMatches(7, 7) {
		t.Fatal("peer matching failed")
	}
	good := terminalShellFrame{Version: 1, Kind: "ready", State: &terminal.LocalShellState{WorkingDirectory: "exact-folder", Elevated: true}}
	if _, err := validateTerminalShellReady(good, "exact-folder"); err != nil {
		t.Fatal(err)
	}
	for _, change := range []func(*terminalShellFrame){func(f *terminalShellFrame) {
		f.State = &terminal.LocalShellState{WorkingDirectory: "different", Elevated: true}
	}, func(f *terminalShellFrame) {
		f.State = &terminal.LocalShellState{WorkingDirectory: "exact-folder", Elevated: false}
	}, func(f *terminalShellFrame) { f.State = nil }, func(f *terminalShellFrame) { f.Kind = "output" }, func(f *terminalShellFrame) { f.Options = &terminal.LocalShellOptions{} }, func(f *terminalShellFrame) { f.Version = 2 }} {
		f := good
		change(&f)
		if _, err := validateTerminalShellReady(f, "exact-folder"); err == nil {
			t.Fatal("bad ready accepted")
		}
	}
}

func TestLocalShellWindowPresetRetainsSettingsWithoutLaunching(t *testing.T) {
	for _, kind := range []string{"cmd", "powershell"} {
		p := terminalWindowPreset{Type: kind, WorkingDirectory: `C:\Synthetic & not yet available`, RunAsAdministrator: true}
		if err := p.normalize(); err != nil {
			t.Fatal(err)
		}
		if p.WorkingDirectory != `C:\Synthetic & not yet available` || !p.RunAsAdministrator {
			t.Fatal("handoff lost local settings")
		}
	}
	p := terminalWindowPreset{Type: "ssh", WorkingDirectory: `C:\Synthetic`, RunAsAdministrator: true}
	if p.normalize() == nil {
		t.Fatal("remote preset accepted local-only settings")
	}
}
