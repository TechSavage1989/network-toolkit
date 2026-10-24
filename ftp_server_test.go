package main

import (
	"bytes"
	"context"
	"net"
	"networktoolkit/internal/diag"
	"networktoolkit/internal/model"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestFTPServerPlainAndFTPSClientOperations(t *testing.T) {
	for _, security := range []string{"plain", "explicit"} {
		t.Run(security, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			base := t.TempDir()
			root := filepath.Join(base, "files")
			_ = os.Mkdir(root, 0700)
			l, e := net.Listen("tcp", "127.0.0.1:0")
			if e != nil {
				t.Fatal(e)
			}
			port := l.Addr().(*net.TCPAddr).Port
			l.Close()
			manager := newTransferHostManager(ctx, base)
			info, e := manager.start(transferServerConfig{Protocol: "ftp", Root: root, Bind: "127.0.0.1", Port: port, Access: "readwrite", Username: "testuser", Password: "testpassword", Security: security})
			if e != nil {
				t.Fatal(e)
			}
			defer manager.stop("ftp")
			fingerprint, _ := info["fingerprint"].(string)
			if security == "explicit" && fingerprint == "" {
				t.Fatal("expected certificate fingerprint")
			}
			opts := map[string]any{"port": port, "username": "testuser", "password": "testpassword", "security": security, "fingerprint": fingerprint, "timeoutMs": 5000}
			action := func(op, path, local, newPath string) (model.Result, error) {
				options := map[string]any{}
				for k, v := range opts {
					options[k] = v
				}
				options["operation"] = op
				options["remotePath"] = path
				options["localPath"] = local
				options["newPath"] = newPath
				options["confirmDelete"] = true
				return diag.Run(ctx, model.Request{Tool: "ftp", Target: "127.0.0.1", Options: options}, nil)
			}
			if _, e = action("mkdir", "configs", "", ""); e != nil {
				t.Fatal("mkdir", e)
			}
			file := filepath.Join(base, "upload.txt")
			original := bytes.Repeat([]byte("Hello from FTP\n"), 120)
			if e = os.WriteFile(file, original, 0600); e != nil {
				t.Fatal(e)
			}
			if _, e = action("upload", "configs/backup.txt", file, ""); e != nil {
				t.Fatal("upload", e)
			}
			result, e := action("list", "configs", "", "")
			if e != nil {
				t.Fatal("list", e)
			}
			if result.Data.(map[string]any)["entryCount"] != 1 {
				t.Fatalf("missing upload: %+v", result.Data)
			}
			if _, e = action("rename", "configs/backup.txt", "", "configs/new.txt"); e != nil {
				t.Fatal("rename", e)
			}
			download := filepath.Join(base, "download.txt")
			if _, e = action("download", "configs/new.txt", download, ""); e != nil {
				t.Fatal("download", e)
			}
			b, e := os.ReadFile(download)
			if e != nil || !bytes.Equal(b, original) {
				t.Fatalf("download mismatch: %v", e)
			}
			if _, e = action("remove", "configs/new.txt", "", ""); e != nil {
				t.Fatal("delete", e)
			}
			if _, e = action("rmdir", "configs", "", ""); e != nil {
				t.Fatal("rmdir", e)
			}
			_, e = action("list", "..", "", "")
			if e != nil {
				t.Log("parent clamped safely:", e)
			}
			if records := manager.status("ftp")["transfers"].([]transferLog); len(records) < 5 {
				t.Fatalf("missing server activity log: %d entries", len(records))
			}
		})
	}
}
func TestFTPRequireTLSPreventsPlaintextLogin(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	l, e := net.Listen("tcp", "127.0.0.1:0")
	if e != nil {
		t.Fatal(e)
	}
	port := l.Addr().(*net.TCPAddr).Port
	l.Close()
	m := newTransferHostManager(ctx, t.TempDir())
	_, e = m.start(transferServerConfig{Protocol: "ftp", Root: t.TempDir(), Bind: "127.0.0.1", Port: port, Access: "read", Username: "test", Password: "secret", Security: "explicit"})
	if e != nil {
		t.Fatal(e)
	}
	defer m.stop("ftp")
	c, e := net.DialTimeout("tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(port)), time.Second)
	if e != nil {
		t.Fatal(e)
	}
	defer c.Close()
	b := make([]byte, 300)
	n, _ := c.Read(b)
	if !strings.Contains(string(b[:n]), "220") {
		t.Fatal("no FTP greeting")
	}
	_, _ = c.Write([]byte("USER test\r\n"))
	n, _ = c.Read(b)
	if !strings.Contains(string(b[:n]), "534") {
		t.Fatalf("TLS requirement bypassed: %q", b[:n])
	}
}
