package diag

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func stagedReply(kind byte, id uint32, body []byte) []byte {
	payload := append(sfU32(id), body...)
	return append(append(sfU32(uint32(len(payload)+1)), kind), payload...)
}
func stagedStatus(id, code uint32) []byte {
	return stagedReply(fxStatus, id, append(append(sfU32(code), sfString("fixture")...), sfString("")...))
}
func stagedWire(reply []byte) (*wireSFTP, *bytes.Buffer) {
	output := &bytes.Buffer{}
	return &wireSFTP{in: bytes.NewReader(reply), out: output}, output
}
func stagedFile(t *testing.T) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "synthetic.cfg")
	if e := os.WriteFile(p, []byte("synthetic bytes"), 0600); e != nil {
		t.Fatal(e)
	}
	return p
}
func TestSFTPStagedUploadNoReplaceAndCollision(t *testing.T) {
	for _, collision := range []bool{false, true} {
		t.Run(map[bool]string{false: "success", true: "collision"}[collision], func(t *testing.T) {
			response := stagedReply(fxHandle, 1, sfString("file"))
			response = append(response, stagedStatus(2, 0)...)
			response = append(response, stagedStatus(3, 0)...)
			code := uint32(0)
			if collision {
				code = 4
			}
			response = append(response, stagedStatus(4, code)...)
			if collision {
				response = append(response, stagedStatus(5, 0)...)
			}
			wire, out := stagedWire(response)
			_, err := wire.uploadStaged(context.Background(), stagedFile(t), "/dest/target.cfg", false, func(uint64, uint64) {})
			if (err != nil) != collision {
				t.Fatalf("collision=%t: %v", collision, err)
			}
			reader := bytes.NewReader(out.Bytes())
			var kinds []byte
			for reader.Len() > 0 {
				var length uint32
				binary.Read(reader, binary.BigEndian, &length)
				b := make([]byte, length)
				io.ReadFull(reader, b)
				kinds = append(kinds, b[0])
				if b[0] == fxOpen {
					payload := b[5:]
					name, _ := sfGetString(&payload)
					flags, _ := sfGetU32(&payload)
					if !strings.HasPrefix(name, "/dest/.ntk-upload-") || flags&sftpExcl == 0 || flags&sftpTrunc != 0 {
						t.Fatal("unsafe staging open")
					}
				}
			}
			if kinds[len(kinds)-1] != map[bool]byte{false: fxRename, true: fxRemove}[collision] {
				t.Fatalf("wrong operations: %v", kinds)
			}
		})
	}
}
func TestSFTPStagedOverwriteRequiresAdvertisedCapability(t *testing.T) {
	wire, out := stagedWire(nil)
	if _, e := wire.uploadStaged(context.Background(), stagedFile(t), "/target", true, func(uint64, uint64) {}); e == nil || !strings.Contains(e.Error(), "advertise") {
		t.Fatal(e)
	}
	if out.Len() != 0 {
		t.Fatal("sent bytes before capability check")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, e := wire.uploadStaged(ctx, stagedFile(t), "/target", false, func(uint64, uint64) {}); !errors.Is(e, context.Canceled) {
		t.Fatal(e)
	}
}
func TestSFTPStagedCancellationCleansOnlyStage(t *testing.T) {
	response := stagedReply(fxHandle, 1, sfString("file"))
	for i := uint32(2); i <= 4; i++ {
		response = append(response, stagedStatus(i, 0)...)
	}
	wire, out := stagedWire(response)
	ctx, cancel := context.WithCancel(context.Background())
	_, e := wire.uploadStaged(ctx, stagedFile(t), "/target", false, func(uint64, uint64) { cancel() })
	if !errors.Is(e, context.Canceled) {
		t.Fatal(e)
	}
	if bytes.Contains(out.Bytes(), sfString("/target")) {
		t.Fatal("cancel touched final destination")
	}
}
func TestSFTPStagedUnconfirmedCleanupIsDisclosed(t *testing.T) {
	wire, _ := stagedWire(stagedReply(fxHandle, 1, sfString("file")))
	_, e := wire.uploadStaged(context.Background(), stagedFile(t), "/target", false, func(uint64, uint64) {})
	if e == nil || !strings.Contains(e.Error(), "cleanup not confirmed") {
		t.Fatal(e)
	}
}

func TestSFTPStagedOverwriteUsesNegotiatedPOSIXExtension(t *testing.T) {
	version := append(sfU32(3), sfString("posix-rename@openssh.com")...)
	version = append(version, sfString("1")...)
	replies := append(append(sfU32(uint32(len(version)+1)), fxVersion), version...)
	replies = append(replies, stagedReply(fxHandle, 1, sfString("file"))...)
	for i := uint32(2); i <= 4; i++ {
		replies = append(replies, stagedStatus(i, 0)...)
	}
	wire, out := stagedWire(replies)
	if e := wire.init(); e != nil {
		t.Fatal(e)
	}
	if !wire.posixRename {
		t.Fatal("capability not negotiated")
	}
	if _, e := wire.uploadStaged(context.Background(), stagedFile(t), "/existing", true, func(uint64, uint64) {}); e != nil {
		t.Fatal(e)
	}
	if !bytes.Contains(out.Bytes(), sfString("posix-rename@openssh.com")) {
		t.Fatal("did not use negotiated atomic replacement")
	}
}

func TestSFTPStagedCloseFailureNeverPublishes(t *testing.T) {
	replies := stagedReply(fxHandle, 1, sfString("file"))
	replies = append(replies, stagedStatus(2, 0)...)
	replies = append(replies, stagedStatus(3, 4)...)
	replies = append(replies, stagedStatus(4, 0)...)
	wire, out := stagedWire(replies)
	_, e := wire.uploadStaged(context.Background(), stagedFile(t), "/original", false, func(uint64, uint64) {})
	if e == nil || !strings.Contains(e.Error(), "close/flush failed") {
		t.Fatal(e)
	}
	if bytes.Contains(out.Bytes(), sfString("/original")) {
		t.Fatal("published after failed remote flush")
	}
}

func TestSFTPStagedFailedExclusiveOpenNeverRemovesUnownedPath(t *testing.T) {
	wire, out := stagedWire(stagedStatus(1, 4))
	_, e := wire.uploadStaged(context.Background(), stagedFile(t), "/target", false, func(uint64, uint64) {})
	if e == nil {
		t.Fatal("failed exclusive creation accepted")
	}
	b := out.Bytes()
	if len(b) < 5 || b[4] != fxOpen {
		t.Fatal("missing exclusive open")
	}
	first := int(binary.BigEndian.Uint32(b[:4])) + 4
	if first != len(b) {
		t.Fatalf("sent cleanup for unowned stage: %d extra bytes", len(b)-first)
	}
}
