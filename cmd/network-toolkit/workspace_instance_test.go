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

// A synthetic protected-record provider, so the Windows rules are exercised on
// every platform. It is reversible, binary, and never looks like JSON.
func syntheticInstanceProtection(t *testing.T) *int {
	t.Helper()
	header := []byte{1, 0, 0, 0, 0xd0, 0x8c}
	protects := 0
	previous := workspaceInstanceGuard
	workspaceInstanceGuard = workspaceInstanceProtection{
		required: true,
		protect: func(plain []byte) ([]byte, error) {
			protects++
			out := append([]byte(nil), header...)
			for _, b := range plain {
				out = append(out, b^0x5a)
			}
			return out, nil
		},
		unprotect: func(stored []byte) ([]byte, error) {
			if !bytes.HasPrefix(stored, header) {
				return nil, errors.New("synthetic unprotect failure")
			}
			out := make([]byte, 0, len(stored))
			for _, b := range stored[len(header):] {
				out = append(out, b^0x5a)
			}
			return out, nil
		},
	}
	t.Cleanup(func() { workspaceInstanceGuard = previous })
	return &protects
}

func TestTerminalInstanceRecordIsProtectedAndOlderPlainRecordStillActivates(t *testing.T) {
	protects := syntheticInstanceProtection(t)
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
	stored, err := os.ReadFile(path)
	if err != nil || *protects != 1 {
		t.Fatalf("record not written through protection: %v (%d)", err, *protects)
	}
	if plainWorkspaceInstance(stored) || bytes.Contains(stored, []byte(a.windowActivationToken)) || bytes.Contains(stored, []byte("127.0.0.1")) || bytes.Contains(stored, []byte(a.windowInstanceID)) {
		t.Fatalf("the Terminal record is stored in plain text: %q", stored)
	}
	if strings.Contains(string(stored), a.token) {
		t.Fatal("the record holds the API token")
	}
	// Activation, peer send and unregistration all read the protected record.
	record, err := readTerminalWindowInstance(dir)
	if err != nil || record.Token != a.windowActivationToken || record.InstanceID != a.windowInstanceID {
		t.Fatalf("protected record unreadable for activation: %+v %v", record, err)
	}
	if record, err = readPeerInstance(dir); err != nil || record.Host != a.host {
		t.Fatalf("protected record unreadable for peer send: %+v %v", record, err)
	}
	if err = activateTerminalWindow(context.Background(), dir, nil); err != nil {
		t.Fatalf("activation through the protected record: %v", err)
	}
	// An older Terminal wrote plain JSON. It still receives launches, but its
	// record is never trusted with text.
	plain, _ := json.Marshal(terminalWindowInstance{Version: 2, Host: a.host, Token: a.windowActivationToken, InstanceID: a.windowInstanceID, PID: os.Getpid()})
	if err = writeAtomic(path, append(plain, '\n')); err != nil {
		t.Fatal(err)
	}
	preset := &terminalWindowPreset{Type: "ssh", Host: "192.0.2.4", Username: "operator"}
	if err = preset.normalize(); err != nil {
		t.Fatal(err)
	}
	if err = activateTerminalWindow(context.Background(), dir, preset); err != nil {
		t.Fatalf("an older plain Terminal record no longer activates: %v", err)
	}
	if batch := a.windowInbox.batch(0); len(batch.Requests) != 1 || batch.Requests[0].Config.Host != "192.0.2.4" {
		t.Fatalf("launch through the plain record lost its preset: %+v", batch)
	}
	if _, err = readPeerInstance(dir); !errors.Is(err, errWorkspaceInstanceUnprotected) {
		t.Fatalf("a plain record was trusted for peer send: %v", err)
	}
	if _, err = readWorkspaceWindowInstance(dir, true); !errors.Is(err, errWorkspaceInstanceUnprotected) {
		t.Fatalf("strict read accepted a plain record: %v", err)
	}
	// Unregistering removes this instance's record, plain or protected.
	unregister()
	if _, err = os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("own instance registration was not removed")
	}
	// A damaged protected record is refused, never read as plain.
	if err = writeAtomic(path, []byte{1, 2, 3, 4}); err != nil {
		t.Fatal(err)
	}
	if _, err = readTerminalWindowInstance(dir); err == nil {
		t.Fatal("a damaged protected record was accepted")
	}
}

func TestInstanceRecordIsNeverWrittenWithoutProtection(t *testing.T) {
	syntheticInstanceProtection(t)
	for name, protect := range map[string]func([]byte) ([]byte, error){
		"failure":       func([]byte) ([]byte, error) { return nil, errors.New("synthetic protect failure") },
		"empty":         func([]byte) ([]byte, error) { return nil, nil },
		"returns plain": func(plain []byte) ([]byte, error) { return append([]byte(nil), plain...), nil },
	} {
		workspaceInstanceGuard.protect = protect
		for _, mode := range []string{"terminal", standaloneEditorMode, "main"} {
			a := testApplication(t)
			a.windowMode = mode
			dir := filepath.Dir(a.dataPath)
			if _, err := a.registerWorkspaceWindow(dir); err == nil {
				t.Fatalf("%s/%s: an unprotected record was registered", name, mode)
			}
			if _, err := os.Stat(filepath.Join(dir, terminalInstanceFilename)); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("%s/%s: a record was written without protection", name, mode)
			}
		}
	}
	// The main and Text Editor records were always protected: a plain record
	// is refused for them, as before.
	a := testApplication(t)
	a.windowMode = standaloneEditorMode
	dir := filepath.Dir(a.dataPath)
	plain, _ := json.Marshal(terminalWindowInstance{Version: 2, Host: "127.0.0.1:38191", Token: a.windowActivationToken, InstanceID: a.windowInstanceID, PID: os.Getpid()})
	if err := writeAtomic(filepath.Join(dir, terminalInstanceFilename), plain); err != nil {
		t.Fatal(err)
	}
	if _, err := readWorkspaceWindowInstance(dir, true); err == nil {
		t.Fatal("a plain Text Editor record was accepted")
	}
}

func TestInstanceProtectionCodec(t *testing.T) {
	plain := []byte(`{"version":2}` + "\n")
	off := workspaceInstanceProtection{}
	stored, err := off.seal(append([]byte(nil), plain...))
	if err != nil || !bytes.Equal(stored, plain) {
		t.Fatalf("platforms without protection store the private file as is: %q %v", stored, err)
	}
	if opened, protected, err := off.open(stored, false); err != nil || protected || !bytes.Equal(opened, plain) {
		t.Fatalf("%q %t %v", opened, protected, err)
	}
	syntheticInstanceProtection(t)
	on := workspaceInstanceGuard
	source := append([]byte(nil), plain...)
	stored, err = on.seal(source)
	if err != nil || plainWorkspaceInstance(stored) {
		t.Fatalf("%q %v", stored, err)
	}
	if !bytes.Equal(source, make([]byte, len(source))) {
		t.Fatal("the plain record was not cleared after protection")
	}
	if opened, protected, err := on.open(stored, false); err != nil || !protected || !bytes.Equal(opened, plain) {
		t.Fatalf("%q %t %v", opened, protected, err)
	}
	for _, legacy := range []string{`{"version":2}`, "\r\n \t{\"version\":2}"} {
		if _, _, err := on.open([]byte(legacy), false); !errors.Is(err, errWorkspaceInstanceUnprotected) {
			t.Fatalf("strict open accepted plain JSON: %v", err)
		}
		if opened, protected, err := on.open([]byte(legacy), true); err != nil || protected || string(opened) != legacy {
			t.Fatalf("legacy plain record: %q %t %v", opened, protected, err)
		}
	}
	for _, damaged := range [][]byte{nil, {}, []byte("not json"), {0xff, 0xfe}} {
		if _, _, err := on.open(damaged, true); err == nil {
			t.Fatalf("damaged record accepted: %q", damaged)
		}
	}
	if _, err := (workspaceInstanceProtection{required: true}).seal([]byte("{}")); err == nil {
		t.Fatal("sealed without a provider")
	}
	if _, _, err := (workspaceInstanceProtection{required: true}).open([]byte{1, 0}, true); err == nil {
		t.Fatal("opened without a provider")
	}
}
