package diag

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/binary"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"golang.org/x/crypto/ssh"
	"networktoolkit/internal/model"
)

func fakeSFTP(channel ssh.Channel, files map[string][]byte, mu *sync.Mutex) {
	defer channel.Close()
	c := wireSFTP{in: channel, out: channel}
	op, _, e := c.recv()
	if e != nil || op != fxInit {
		return
	}
	_ = c.send(fxVersion, sfU32(3))
	var opened string
	reading := false
	for {
		op, p, e := c.recv()
		if e != nil {
			return
		}
		id, e := sfGetU32(&p)
		if e != nil {
			return
		}
		respond := func(kind byte, body ...[]byte) {
			out := sfU32(id)
			for _, x := range body {
				out = append(out, x...)
			}
			_ = c.send(kind, out)
		}
		status := func(code uint32) { respond(fxStatus, sfU32(code), sfString("remote status"), sfString("")) }
		switch op {
		case fxOpendir:
			name, _ := sfGetString(&p)
			if name != "/" {
				status(2)
				continue
			}
			respond(fxHandle, sfString("dir"))
			reading = false
		case fxReaddir:
			if reading {
				status(1)
				continue
			}
			reading = true
			mu.Lock()
			entry := make([][]byte, 0, len(files))
			for name, content := range files {
				attrs := append(append(sfU32(5), sfU64(uint64(len(content)))...), sfU32(0100644)...)
				line := append(append(sfString(strings.TrimPrefix(name, "/")), sfString(name)...), attrs...)
				entry = append(entry, line)
			}
			mu.Unlock()
			data := sfU32(uint32(len(entry)))
			for _, b := range entry {
				data = append(data, b...)
			}
			respond(fxName, data)
		case fxLstat:
			name, _ := sfGetString(&p)
			mu.Lock()
			content, ok := files[name]
			mu.Unlock()
			if !ok {
				status(2)
				continue
			}
			attrs := append(append(sfU32(5), sfU64(uint64(len(content)))...), sfU32(0100644)...)
			respond(fxAttrs, attrs)
		case fxOpen:
			opened, _ = sfGetString(&p)
			flags, _ := sfGetU32(&p)
			mu.Lock()
			_, exists := files[opened]
			if flags&sftpExcl != 0 && exists {
				mu.Unlock()
				status(4)
				continue
			}
			if !exists && flags&sftpCreate != 0 {
				files[opened] = []byte{}
			}
			if flags&sftpTrunc != 0 {
				files[opened] = []byte{}
			}
			_, exists = files[opened]
			mu.Unlock()
			if !exists {
				status(2)
				continue
			}
			respond(fxHandle, sfString("file"))
		case fxRead:
			_, _ = sfGetString(&p)
			raw, _ := sfTake(&p, 8)
			length, _ := sfGetU32(&p)
			offset := binary.BigEndian.Uint64(raw)
			mu.Lock()
			data := append([]byte(nil), files[opened]...)
			mu.Unlock()
			if offset >= uint64(len(data)) {
				status(1)
				continue
			}
			end := offset + uint64(length)
			if end > uint64(len(data)) {
				end = uint64(len(data))
			}
			respond(fxData, sfBytes(data[offset:end]))
		case fxWrite:
			_, _ = sfGetString(&p)
			raw, _ := sfTake(&p, 8)
			contents, _ := sfGetData(&p)
			offset := int(binary.BigEndian.Uint64(raw))
			mu.Lock()
			data := files[opened]
			if offset > len(data) {
				data = append(data, bytes.Repeat([]byte{0}, offset-len(data))...)
			}
			if offset+len(contents) > len(data) {
				data = append(data, make([]byte, offset+len(contents)-len(data))...)
			}
			copy(data[offset:], contents)
			files[opened] = data
			mu.Unlock()
			status(0)
		case fxClose:
			status(0)
		case fxMkdir:
			status(0)
		default:
			status(8)
		}
	}
}

func TestSFTPProtocolOperations(t *testing.T) {
	_, key, e := ed25519.GenerateKey(rand.Reader)
	if e != nil {
		t.Fatal(e)
	}
	signer, e := ssh.NewSignerFromKey(key)
	if e != nil {
		t.Fatal(e)
	}
	listener, e := net.Listen("tcp", "127.0.0.1:0")
	if e != nil {
		t.Fatal(e)
	}
	defer listener.Close()
	config := &ssh.ServerConfig{PasswordCallback: func(meta ssh.ConnMetadata, password []byte) (*ssh.Permissions, error) {
		if meta.User() == "operator" && string(password) == "secret" {
			return nil, nil
		}
		return nil, io.EOF
	}}
	config.AddHostKey(signer)
	files := map[string][]byte{"/readme.txt": []byte("Readme test content")}
	var mu sync.Mutex
	go func() {
		for {
			conn, e := listener.Accept()
			if e != nil {
				return
			}
			go func() {
				defer conn.Close()
				server, channels, reqs, e := ssh.NewServerConn(conn, config)
				if e != nil {
					return
				}
				defer server.Close()
				go ssh.DiscardRequests(reqs)
				for ch := range channels {
					if ch.ChannelType() != "session" {
						_ = ch.Reject(ssh.UnknownChannelType, "session only")
						continue
					}
					channel, requests, e := ch.Accept()
					if e != nil {
						continue
					}
					go func() {
						for req := range requests {
							if req.Type != "subsystem" || len(req.Payload) < 4 || string(req.Payload[4:]) != "sftp" {
								_ = req.Reply(false, nil)
								continue
							}
							_ = req.Reply(true, nil)
							fakeSFTP(channel, files, &mu)
							return
						}
					}()
				}
			}()
		}
	}()
	address := listener.Addr().(*net.TCPAddr)
	base := map[string]any{"port": address.Port, "username": "operator", "password": "secret", "timeoutMs": 3000, "authMode": "password", "remotePath": "/"}
	mk := func(op string) model.Request {
		options := make(map[string]any)
		for k, v := range base {
			options[k] = v
		}
		options["operation"] = op
		return model.Request{Tool: "sftp", Target: "127.0.0.1", Options: options}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	inspect := mk("fingerprint")
	inspect.Options["username"] = ""
	inspect.Options["password"] = ""
	result, e := sftpTest(ctx, inspect, nil)
	if e != nil {
		t.Fatalf("inspect: %v", e)
	}
	fingerprint := result.Data.(map[string]any)["hostKeyFingerprint"].(string)
	if !strings.HasPrefix(fingerprint, "SHA256:") {
		t.Fatal("missing fingerprint")
	}
	base["fingerprint"] = fingerprint
	listing, e := sftpTest(ctx, mk("list"), nil)
	if e != nil {
		t.Fatalf("list: %v", e)
	}
	if listing.Data.(map[string]any)["entryCount"].(int) != 1 {
		t.Fatalf("incorrect listing %v", listing.Data)
	}
	download := mk("download")
	download.Options["remotePath"] = "/readme.txt"
	download.Options["localPath"] = filepath.Join(t.TempDir(), "copy.txt")
	_, e = sftpTest(ctx, download, nil)
	if e != nil {
		t.Fatalf("download: %v", e)
	}
	got, e := os.ReadFile(download.Options["localPath"].(string))
	if e != nil || string(got) != "Readme test content" {
		t.Fatalf("download mismatch %q %v", got, e)
	}
	uploadPath := filepath.Join(t.TempDir(), "upload.bin")
	_ = os.WriteFile(uploadPath, []byte("Uploaded content"), 0600)
	upload := mk("upload")
	upload.Options["remotePath"] = "/upload.bin"
	upload.Options["localPath"] = uploadPath
	_, e = sftpTest(ctx, upload, nil)
	if e != nil {
		t.Fatalf("upload: %v", e)
	}
	mu.Lock()
	got = append([]byte(nil), files["/upload.bin"]...)
	mu.Unlock()
	if string(got) != "Uploaded content" {
		t.Fatalf("upload mismatch %q", got)
	}
	denied := mk("list")
	denied.Options["fingerprint"] = "SHA256:wrong"
	_, e = sftpTest(ctx, denied, nil)
	if e == nil || !strings.Contains(e.Error(), "mismatch") {
		t.Fatalf("fingerprint not enforced: %v", e)
	}
}
