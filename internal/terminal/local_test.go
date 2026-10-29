package terminal

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestLocalShellRequestDiscardsRemoteFields(t *testing.T) {
	for _, kind := range []string{"cmd", "powershell"} {
		req, err := normalizeRequest(ConnectRequest{Type: kind, ProfileID: "local-profile", Host: "remote.invalid", Port: 22, Username: "someone", Password: "hidden", PrivateKey: "hidden", Passphrase: "hidden", AuthMethod: "key", SerialPort: "COM1", SerialDTR: new(bool)})
		if err != nil {
			t.Fatal(err)
		}
		if req.Host != "" || req.Port != 0 || req.Username != "" || req.Password != "" || req.PrivateKey != "" || req.Passphrase != "" || req.AuthMethod != "" || req.SerialPort != "" || req.SerialDTR != nil {
			t.Fatalf("remote fields retained: %#v", req)
		}
		if req.ProfileID != "local-profile" || req.Cols != 80 || req.Rows != 24 || req.LineEnding != "cr" {
			t.Fatalf("local defaults: %#v", req)
		}
		session := newLiveSession(context.Background(), "test", req)
		defer session.cancel()
		if session.info.Host != "" || session.info.Port != 0 || session.info.Label == "" {
			t.Fatalf("local endpoint: %#v", session.info)
		}
	}
}

func TestLocalShellWorkingDirectoryAndElevationDefaults(t *testing.T) {
	dir := t.TempDir()
	req, err := normalizeRequest(ConnectRequest{Type: "cmd", WorkingDirectory: dir, RunAsAdministrator: true})
	if err != nil {
		t.Fatal(err)
	}
	original, originalErr := os.Stat(dir)
	actual, actualErr := os.Stat(req.WorkingDirectory)
	if originalErr != nil || actualErr != nil || !os.SameFile(original, actual) || !req.RunAsAdministrator {
		t.Fatal("lost local settings", req)
	}
	s := newLiveSession(context.Background(), "requested-only", req)
	defer s.cancel()
	if s.info.Elevated {
		t.Fatal("request checkbox presented as actual elevation")
	}
	for _, path := range []string{"relative", "bad\x00path", dir + "/missing"} {
		if _, err = normalizeRequest(ConnectRequest{Type: "cmd", WorkingDirectory: path}); err == nil {
			t.Fatal("accepted invalid directory", path)
		}
	}
	file := filepath.Join(dir, "file")
	if err = os.WriteFile(file, []byte("synthetic"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err = ValidateLocalWorkingDirectory(file); err == nil {
		t.Fatal("accepted file as directory")
	}
	for _, kind := range []string{"cmd", "powershell"} {
		req, err = normalizeRequest(ConnectRequest{Type: kind})
		if err != nil || req.RunAsAdministrator || req.WorkingDirectory != "" {
			t.Fatal("bad safe default", req, err)
		}
	}
}

func TestLocalShellOptionsRejectedForRemoteTypes(t *testing.T) {
	for _, kind := range []string{"ssh", "telnet", "serial"} {
		for _, req := range []ConnectRequest{{Type: kind, WorkingDirectory: t.TempDir()}, {Type: kind, RunAsAdministrator: true}} {
			if _, err := normalizeRequest(req); err == nil {
				t.Fatal("remote protocol accepted local-only options", kind)
			}
		}
	}
}
