package diag

import (
	"bytes"
	"context"
	"encoding/binary"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"networktoolkit/internal/model"
)

func TestTFTPDownloadAndUploadWithNegotiation(t *testing.T) {
	blob := bytes.Repeat([]byte("NetworkToolkit configuration data!"), 100)
	for _, op := range []string{"download", "upload"} {
		t.Run(op, func(t *testing.T) {
			server, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 0})
			if err != nil {
				t.Fatal(err)
			}
			defer server.Close()
			dataFile := filepath.Join(t.TempDir(), "transferred.bin")
			if op == "upload" {
				if err = os.WriteFile(dataFile, blob, 0600); err != nil {
					t.Fatal(err)
				}
			}
			done := make(chan error, 1)
			go func() {
				buf := make([]byte, 3000)
				_ = server.SetDeadline(time.Now().Add(6 * time.Second))
				n, peer, e := server.ReadFromUDP(buf)
				if e != nil {
					done <- e
					return
				}
				code := binary.BigEndian.Uint16(buf[:2])
				if op == "download" && code != tftpReadRequest || op == "upload" && code != tftpWriteRequest {
					done <- errUnexpectedPacket{}
					return
				}
				reply := tfPacket(tftpOACK, []byte("blksize\x0064\x00tsize\x00"))
				if op == "download" {
					reply = append(reply, []byte("3100\x00")...)
				} else {
					reply = append(reply, []byte("0\x00")...)
				}
				if _, e = server.WriteToUDP(reply, peer); e != nil {
					done <- e
					return
				}
				if op == "download" {
					n, peer, e = server.ReadFromUDP(buf)
					if e != nil {
						done <- e
						return
					}
					if n != 4 || binary.BigEndian.Uint16(buf[:2]) != tftpAck {
						done <- errUnexpectedPacket{}
						return
					}
					for seq, pos := uint16(1), 0; ; seq++ {
						end := pos + 64
						if end > len(blob) {
							end = len(blob)
						}
						_, e = server.WriteToUDP(tfPacket(tftpData, tf16(seq), blob[pos:end]), peer)
						if e != nil {
							done <- e
							return
						}
						n, peer, e = server.ReadFromUDP(buf)
						if e != nil {
							done <- e
							return
						}
						if n != 4 || binary.BigEndian.Uint16(buf[2:4]) != seq {
							done <- errUnexpectedPacket{}
							return
						}
						pos = end
						if pos == len(blob) {
							if pos%64 == 0 {
								seq++
								_, e = server.WriteToUDP(tfPacket(tftpData, tf16(seq)), peer)
								if e == nil {
									_, _, e = server.ReadFromUDP(buf)
								}
							}
							done <- e
							return
						}
					}
				}
				var received []byte
				for seq := uint16(1); ; seq++ {
					n, peer, e = server.ReadFromUDP(buf)
					if e != nil {
						done <- e
						return
					}
					if binary.BigEndian.Uint16(buf[:2]) != tftpData || binary.BigEndian.Uint16(buf[2:4]) != seq {
						done <- errUnexpectedPacket{}
						return
					}
					received = append(received, buf[4:n]...)
					_, e = server.WriteToUDP(tfPacket(tftpAck, tf16(seq)), peer)
					if e != nil {
						done <- e
						return
					}
					if n-4 < 64 {
						if !bytes.Equal(received, blob) {
							done <- errUnexpectedPacket{}
							return
						}
						done <- nil
						return
					}
				}
			}()
			req := model.Request{Tool: "tftp", Target: "127.0.0.1", Options: map[string]any{"port": server.LocalAddr().(*net.UDPAddr).Port, "operation": op, "remotePath": "config.bin", "localPath": dataFile, "blockSize": 64, "timeoutSec": 1, "retries": 2}}
			result, e := tftpTest(context.Background(), req, nil)
			if e != nil {
				t.Fatalf("%s failed: %v", op, e)
			}
			if result.Summary == "" {
				t.Fatal("no summary")
			}
			if op == "download" {
				content, e := os.ReadFile(dataFile)
				if e != nil || !bytes.Equal(content, blob) {
					t.Fatalf("wrong download: %v %d", e, len(content))
				}
			}
			if e = <-done; e != nil {
				t.Fatal(e)
			}
		})
	}
}

type errUnexpectedPacket struct{}

func (errUnexpectedPacket) Error() string { return "unexpected TFTP protocol packet" }

func TestTFTPRefusesLocalOverwrite(t *testing.T) {
	file := filepath.Join(t.TempDir(), "existing")
	_ = os.WriteFile(file, []byte("keep"), 0600)
	req := model.Request{Target: "127.0.0.1", Options: map[string]any{"operation": "download", "remotePath": "file", "localPath": file}}
	if _, e := tftpTest(context.Background(), req, nil); e == nil {
		t.Fatal("expected overwrite refusal")
	}
}
