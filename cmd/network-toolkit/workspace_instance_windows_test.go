//go:build windows

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The Terminal's instance record is protected with the real Windows DPAPI
// provider, as the main and Text Editor records already were.
func TestWindowsTerminalInstanceRecordUsesDPAPI(t *testing.T) {
	a := testApplication(t)
	a.windowMode = "terminal"
	server := httptest.NewServer(a)
	defer server.Close()
	a.host = strings.TrimPrefix(server.URL, "http://")
	dir := filepath.Dir(a.dataPath)
	path := filepath.Join(dir, terminalInstanceFilename)
	unregister, err := a.registerTerminalWindow(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer unregister()
	stored, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if plainWorkspaceInstance(stored) || bytes.Contains(stored, []byte(a.windowActivationToken)) || bytes.Contains(stored, []byte("127.0.0.1")) || bytes.Contains(stored, []byte(a.windowInstanceID)) {
		t.Fatal("the Terminal instance record is not DPAPI protected")
	}
	plain, err := unprotectCredentialBlob(stored)
	if err != nil {
		t.Fatalf("the record is not a DPAPI blob for this user: %v", err)
	}
	var decoded terminalWindowInstance
	if err = json.Unmarshal(plain, &decoded); err != nil || decoded.Token != a.windowActivationToken || decoded.Host != a.host {
		t.Fatalf("unexpected protected record: %v", err)
	}
	if record, err := readPeerInstance(dir); err != nil || record.InstanceID != a.windowInstanceID {
		t.Fatalf("peer send cannot read the protected record: %v", err)
	}
	if err = activateTerminalWindow(context.Background(), dir, nil); err != nil {
		t.Fatalf("activation through the protected record: %v", err)
	}
	// A tampered blob is refused.
	tampered := append([]byte(nil), stored...)
	tampered[len(tampered)-1] ^= 1
	if err = writeAtomic(path, tampered); err != nil {
		t.Fatal(err)
	}
	if _, err = readTerminalWindowInstance(dir); err == nil {
		t.Fatal("a tampered protected record was accepted")
	}
	// A plain record from an older Terminal still activates, but is never
	// trusted for peer send.
	legacy, _ := json.Marshal(decoded)
	if err = writeAtomic(path, append(legacy, '\n')); err != nil {
		t.Fatal(err)
	}
	if err = activateTerminalWindow(context.Background(), dir, nil); err != nil {
		t.Fatalf("an older plain Terminal record no longer activates: %v", err)
	}
	if _, err = readPeerInstance(dir); !errors.Is(err, errWorkspaceInstanceUnprotected) {
		t.Fatalf("a plain record was trusted for peer send: %v", err)
	}
}
