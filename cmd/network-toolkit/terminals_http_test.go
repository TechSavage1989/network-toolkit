package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"networktoolkit/internal/terminal"
)

func TestTerminalAPIGuardsAndBoundedRequests(t *testing.T) {
	a := testApplication(t)
	for _, path := range []string{"/api/terminals", "/api/terminals/events?wait=0", "/api/terminals/serial-ports", "/api/terminals/known-hosts", "/api/terminals/example/input"} {
		if r := requestApp(a, "GET", path, "", false); r.Code != http.StatusUnauthorized {
			t.Fatalf("Unauthenticated terminal route %s: %d", path, r.Code)
		}
	}
	for _, query := range []string{"after=-1&wait=0", "after=not-a-cursor&wait=0", "wait=-1", "wait=20001", "wait=forever"} {
		if r := requestApp(a, "GET", "/api/terminals/events?"+query, "", true); r.Code != http.StatusBadRequest {
			t.Fatalf("Invalid terminal query %s: %d", query, r.Code)
		}
	}
	for _, body := range []string{`{"type":"shell","host":"localhost"}`, `{"type":"telnet","host":"127.0.0.1","unexpected":"field"}`, `{"type":"telnet","host":"127.0.0.1"} {}`, `{"type":"ssh","host":"127.0.0.1\ncommand"}`, `{"type":"ssh","privateKey":"` + strings.Repeat("a", 512<<10) + `"}`} {
		r := requestApp(a, "POST", "/api/terminals", body, true)
		if r.Code != http.StatusBadRequest {
			t.Fatalf("Invalid terminal connection accepted (%d): %.200s", r.Code, r.Body.String())
		}
	}
	if r := requestApp(a, "GET", "/api/terminals/events?wait=0", "", true); r.Code != http.StatusOK {
		t.Fatalf("Empty bounded event poll: %d %s", r.Code, r.Body.String())
	}
	r := httptest.NewRequest("POST", "http://"+a.host+"/api/terminals", strings.NewReader(`{"type":"telnet","host":"127.0.0.1"}`))
	r.Header.Set("X-Session-Token", a.token)
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("Origin", "https://other.example")
	w := httptest.NewRecorder()
	a.ServeHTTP(w, r)
	if w.Code != http.StatusForbidden {
		t.Fatalf("Cross-origin terminal connection accepted: %d", w.Code)
	}
}

func TestEmbeddedTerminalAssetsAreOfflineAndAllowlisted(t *testing.T) {
	a := testApplication(t)
	page := requestApp(a, "GET", "/", "", false)
	for _, asset := range []string{"subnet-planner.js", "report-export.js"} {
		if !strings.Contains(page.Body.String(), "src=\"/assets/"+asset+"\"") {
			t.Fatalf("Main interface cannot resolve embedded asset %s", asset)
		}
	}
	for _, file := range []string{"xterm.js", "addon-fit.js", "addon-search.js", "terminal-highlight.js", "subnet-planner.js", "report-export.js", "xterm.css"} {
		r := requestApp(a, "GET", "/assets/"+file, "", false)
		if r.Code != http.StatusOK || r.Body.Len() < 1000 {
			t.Fatalf("Embedded asset %s is missing: %d, %d bytes", file, r.Code, r.Body.Len())
		}
		if !strings.Contains(r.Header().Get("Content-Type"), map[bool]string{true: "text/css", false: "text/javascript"}[strings.HasSuffix(file, ".css")]) {
			t.Fatalf("Wrong content type for %s", file)
		}
	}
	for _, path := range []string{"/assets/", "/assets/SOURCE.json", "/assets/../main.go", "/assets/xterm.js.map"} {
		if r := requestApp(a, "GET", path, "", false); r.Code != http.StatusNotFound {
			t.Fatalf("Non-public asset path exposed: %s (%d)", path, r.Code)
		}
	}
	if r := requestApp(a, "POST", "/assets/xterm.js", "", false); r.Code != http.StatusMethodNotAllowed {
		t.Fatalf("Asset mutation accepted: %d", r.Code)
	}
}

func TestTerminalProfilesCannotPersistAuthenticationOrOutput(t *testing.T) {
	a := testApplication(t)
	body := `{"theme":"light","reportNotes":"Retain unrelated notes","terminalConnection":{"type":"ssh","host":"192.0.2.10","port":22,"username":"operator","password":"secret-password-marker","privateKey":"secret-key-marker","passphrase":"secret-passphrase-marker","answers":["secret-answer-marker"],"output":"secret-output-marker"},"terminalProfiles":[{"id":"p1","name":"Lab switch","type":"ssh","host":"192.0.2.10","port":22,"username":"operator","authMethod":"password","password":"secret-password-marker","privateKey":"secret-key-marker","passphrase":"secret-passphrase-marker","answers":["secret-answer-marker"]},{"name":"Serial","type":"serial","serialPort":"COM4","baudRate":115200,"dataBits":8,"stopBits":"1","parity":"none","flowControl":"none","localEcho":false,"lineEnding":"crlf"}]}`
	r := requestApp(a, "PUT", "/api/state", body, true)
	if r.Code != http.StatusOK {
		t.Fatalf("Safe settings write failed: %d %s", r.Code, r.Body.String())
	}
	disk, err := os.ReadFile(a.dataPath)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(disk, []byte("secret-")) {
		t.Fatal("Authentication or output reached the portable settings file")
	}
	for _, wanted := range []string{"operator", "192.0.2.10", "Lab switch", "COM4", "115200", "Retain unrelated notes"} {
		if !bytes.Contains(disk, []byte(wanted)) {
			t.Fatalf("Safe profile/settings field lost: %s", wanted)
		}
	}
	var result struct {
		Sessions []terminal.SessionInfo `json:"sessions"`
	}
	r = requestApp(a, "GET", "/api/terminals", "", true)
	if err = json.Unmarshal(r.Body.Bytes(), &result); err != nil || len(result.Sessions) != 0 {
		t.Fatal("Saving or reading a profile must not open a connection")
	}
}

// This checks the actual public API, manager and Telnet transport together,
// using a loopback-only server. It never contacts a discovered or remote host.
func TestTerminalHTTPLoopbackSessionAndGlobalEvents(t *testing.T) {
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	received := make(chan string, 1)
	serverDone := make(chan struct{})
	go func() {
		defer close(serverDone)
		conn, acceptErr := listener.Accept()
		if acceptErr != nil {
			return
		}
		defer conn.Close()
		_ = conn.SetDeadline(time.Now().Add(8 * time.Second))
		_, _ = io.WriteString(conn, "\x1b[32mLocal Telnet fixture\x1b[0m\r\nrouter> ")
		var input []byte
		buf := make([]byte, 4096)
		for {
			n, readErr := conn.Read(buf)
			input = append(input, buf[:n]...)
			if bytes.Contains(input, []byte("show fixture")) {
				received <- string(input)
				_, _ = io.WriteString(conn, "\r\nfixture response\r\nrouter> ")
				_, _ = io.Copy(io.Discard, conn)
				return
			}
			if readErr != nil {
				return
			}
		}
	}()
	a := testApplication(t)
	port := listener.Addr().(*net.TCPAddr).Port
	r := requestApp(a, "POST", "/api/terminals", `{"type":"telnet","host":"127.0.0.1","port":`+strconv.Itoa(port)+`,"cols":90,"rows":28}`, true)
	if r.Code != http.StatusCreated {
		t.Fatalf("Connect %d: %s", r.Code, r.Body.String())
	}
	var connected struct {
		Session terminal.SessionInfo `json:"session"`
	}
	if err = json.Unmarshal(r.Body.Bytes(), &connected); err != nil || connected.Session.ID == "" {
		t.Fatalf("Invalid connect response: %s", r.Body.String())
	}
	id := connected.Session.ID
	after := uint64(0)
	output := []byte{}
	deadline := time.Now().Add(5 * time.Second)
	for !bytes.Contains(output, []byte("Local Telnet fixture")) && time.Now().Before(deadline) {
		r = requestApp(a, "GET", "/api/terminals/events?after="+strconv.FormatUint(after, 10)+"&wait=100", "", true)
		if r.Code != http.StatusOK {
			t.Fatalf("Event poll: %d %s", r.Code, r.Body.String())
		}
		var batch terminal.Batch
		if err = json.Unmarshal(r.Body.Bytes(), &batch); err != nil {
			t.Fatal(err)
		}
		after = batch.Next
		for _, event := range batch.Events {
			if event.SessionID == id && event.Type == "output" {
				output = append(output, event.Data...)
			}
		}
	}
	if !bytes.Contains(output, []byte("\x1b[32mLocal Telnet fixture\x1b[0m")) {
		t.Fatalf("Terminal output was missing or ANSI bytes changed: %q", output)
	}
	r = requestApp(a, "POST", "/api/terminals/"+id+"/input", `{"data":"`+base64.StdEncoding.EncodeToString([]byte("show fixture\r"))+`"}`, true)
	if r.Code != http.StatusOK {
		t.Fatalf("Input %d: %s", r.Code, r.Body.String())
	}
	select {
	case <-received:
	case <-time.After(3 * time.Second):
		t.Fatal("Queued terminal input did not reach loopback server")
	}
	r = requestApp(a, "POST", "/api/terminals/"+id+"/resize", `{"cols":110,"rows":32}`, true)
	if r.Code != http.StatusOK {
		t.Fatalf("Resize %d: %s", r.Code, r.Body.String())
	}
	r = requestApp(a, "POST", "/api/terminals/"+id+"/disconnect", `{}`, true)
	if r.Code != http.StatusOK {
		t.Fatalf("Disconnect %d: %s", r.Code, r.Body.String())
	}
	select {
	case <-serverDone:
	case <-time.After(3 * time.Second):
		t.Fatal("Disconnect did not release the connection")
	}
	r = requestApp(a, "POST", "/api/terminals/"+id+"/input", `{"data":"YQ=="}`, true)
	if r.Code != http.StatusConflict {
		t.Fatalf("Input after disconnect should fail clearly: %d %s", r.Code, r.Body.String())
	}
	r = requestApp(a, "DELETE", "/api/terminals/"+id, "", true)
	if r.Code != http.StatusOK {
		t.Fatalf("Remove disconnected session %d: %s", r.Code, r.Body.String())
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err = a.terminals.Events(ctx, after, time.Second); err != nil && !strings.Contains(err.Error(), "canceled") {
		t.Fatalf("Unexpected canceled-poll error: %v", err)
	}
}
