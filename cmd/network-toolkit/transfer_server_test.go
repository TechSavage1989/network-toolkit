package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"networktoolkit/internal/diag"
	"networktoolkit/internal/model"
)

func TestTransferServerRootBoundaries(t *testing.T) {
	root := t.TempDir()
	outside := filepath.Join(root, "..", "escape.txt")
	for _, bad := range []string{"../escape.txt", "/etc/passwd", "C:/secret.txt", "sub/../../escape.txt"} {
		if _, err := transferPath(root, bad, true); err == nil {
			t.Errorf("accepted traversal %q", bad)
		}
	}
	if _, e := transferPath(root, "folder/file", false); e != nil {
		t.Fatal(e)
	}
	link := filepath.Join(root, "link")
	if e := os.Symlink(filepath.Dir(outside), link); e == nil {
		if _, e = transferPath(root, "link/escape.txt", false); e == nil {
			t.Fatal("followed symlink")
		}
	}
}
func TestSFTPServerEndToEnd(t *testing.T) {
	base := t.TempDir()
	root := filepath.Join(base, "share")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	m := newTransferHostManager(ctx, base)
	got, e := m.start(transferServerConfig{Protocol: "sftp", Bind: "127.0.0.1", Port: 0, Root: root, Access: "readwrite", Username: "test", Password: "local secret"})
	// Port 0 defaults to 2222. To avoid shared CI port conflicts, direct a new ephemeral port selection below.
	if e != nil {
		t.Skipf("TCP listener unavailable in test environment: %v", e)
	}
	defer m.Close()
	fp := got["fingerprint"].(string)
	req := func(operation, remote, local string, extras map[string]any) (model.Result, error) {
		opts := map[string]any{"port": 2222, "operation": operation, "remotePath": remote, "localPath": local, "username": "test", "password": "local secret", "fingerprint": fp, "overwrite": false, "timeoutMs": 2500}
		for k, v := range extras {
			opts[k] = v
		}
		return diag.Run(context.Background(), model.Request{Tool: "sftp", Target: "127.0.0.1", Options: opts}, nil)
	}
	if _, e = req("mkdir", "config", "", nil); e != nil {
		t.Fatal("mkdir", e)
	}
	source := filepath.Join(base, "original.txt")
	contents := bytes.Repeat([]byte("network toolkit test\n"), 1000)
	if e = os.WriteFile(source, contents, 0600); e != nil {
		t.Fatal(e)
	}
	if _, e = req("upload", "config/backup.txt", source, nil); e != nil {
		t.Fatal("upload", e)
	}
	if _, e = req("list", "config", "", nil); e != nil {
		t.Fatal("list", e)
	}
	dest := filepath.Join(base, "download.txt")
	if _, e = req("download", "config/backup.txt", dest, nil); e != nil {
		t.Fatal("download", e)
	}
	saved, e := os.ReadFile(dest)
	if e != nil || !bytes.Equal(saved, contents) {
		t.Fatal("download mismatch", e)
	}
	if _, e = req("rename", "config/backup.txt", "", map[string]any{"newPath": "config/renamed.txt"}); e != nil {
		t.Fatal("rename", e)
	}
	if _, e = req("remove", "config/renamed.txt", "", map[string]any{"confirmDelete": true}); e != nil {
		t.Fatal("remove", e)
	}
	if _, e = req("rmdir", "config", "", map[string]any{"confirmDelete": true}); e != nil {
		t.Fatal("rmdir", e)
	}
	if _, e = req("stat", "../escape.txt", "", nil); e == nil {
		t.Fatal("root traversal unexpectedly accessible")
	}
	if status := m.status("sftp"); status["running"] != true {
		t.Fatal("server not running")
	}
	if e = m.stop("sftp"); e != nil {
		t.Fatal(e)
	}
	if m.status("sftp")["running"] != false {
		t.Fatal("server did not stop")
	}
}
func TestTFTPServerEndToEnd(t *testing.T) {
	base := t.TempDir()
	root := filepath.Join(base, "share")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	m := newTransferHostManager(ctx, base)
	got, e := m.start(transferServerConfig{Protocol: "tftp", Bind: "127.0.0.1", Port: 10691, Root: root, Access: "readwrite"})
	if e != nil {
		t.Skipf("UDP listener unavailable in test environment: %v", e)
	}
	defer m.Close()
	if got["running"] != true {
		t.Fatal("not running")
	}
	data := bytes.Repeat([]byte("TFTP firmware block\n"), 600)
	source := filepath.Join(base, "firmware.img")
	if e = os.WriteFile(source, data, 0600); e != nil {
		t.Fatal(e)
	}
	run := func(op, remote, local string) (model.Result, error) {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		return diag.Run(ctx, model.Request{Tool: "tftp", Target: "127.0.0.1", Options: map[string]any{"port": 10691, "operation": op, "remotePath": remote, "localPath": local, "blockSize": 1024, "timeoutSec": 1, "retries": 2}}, nil)
	}
	if _, e = run("upload", "switch.cfg", source); e != nil {
		t.Fatal("upload", e)
	}
	target := filepath.Join(root, "switch.cfg")
	time.Sleep(100 * time.Millisecond)
	saved, e := os.ReadFile(target)
	if e != nil || !bytes.Equal(saved, data) {
		t.Fatalf("server upload mismatch: %v", e)
	}
	dest := filepath.Join(base, "restored.cfg")
	if _, e = run("download", "switch.cfg", dest); e != nil {
		t.Fatal("download", e)
	}
	saved, e = os.ReadFile(dest)
	if e != nil || !bytes.Equal(saved, data) {
		t.Fatalf("client download mismatch: %v", e)
	}
	if _, e = run("upload", "../escape", source); e == nil {
		t.Fatal("accepted traversal")
	}
	if e = m.stop("tftp"); e != nil {
		t.Fatal(e)
	}
}
func TestTransferFileManager(t *testing.T) {
	root := t.TempDir()
	if _, e := transferFileAction(root, "mkdir", "configs", "", "", false, false); e != nil {
		t.Fatal(e)
	}
	source := filepath.Join(t.TempDir(), "switch.cfg")
	_ = os.WriteFile(source, []byte("config"), 0600)
	if _, e := transferFileAction(root, "upload", "configs/switch.cfg", "", source, false, false); e != nil {
		t.Fatal(e)
	}
	data, e := transferList(root, "configs")
	if e != nil {
		t.Fatal(e)
	}
	items := data["entries"].([]transferFileRecord)
	if len(items) != 1 {
		t.Fatalf("entries: %+v", items)
	}
	if _, e = transferFileAction(root, "rename", "configs/switch.cfg", "configs/renamed.cfg", "", false, false); e != nil {
		t.Fatal(e)
	}
	if _, e = transferFileAction(root, "delete", "configs/renamed.cfg", "", "", false, false); e == nil {
		t.Fatal("delete without confirmation")
	}
	if _, e = transferFileAction(root, "delete", "configs/renamed.cfg", "", "", false, true); e != nil {
		t.Fatal(e)
	}
	if _, e = transferFileAction(root, "mkdir", "../outside", "", "", false, false); e == nil || !strings.Contains(e.Error(), "traversal") {
		t.Fatal("missing traversal denial", e)
	}
}
