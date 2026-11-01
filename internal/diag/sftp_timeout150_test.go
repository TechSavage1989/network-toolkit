package diag

import (
    "bytes"
    "encoding/binary"
    "sync/atomic"
    "testing"
    "time"
)

// A finished exchange must not close its scoped SFTP channel after the
// response has returned, even when its timeout window subsequently elapses.
func TestSharedSFTPSuccessDisarmsResponseTimeout(t *testing.T) {
    for cycle := 0; cycle < 20; cycle++ {
        frame := make([]byte, 9)
        binary.BigEndian.PutUint32(frame[:4], 5)
        frame[4] = fxVersion
        binary.BigEndian.PutUint32(frame[5:], 3)
        var aborted atomic.Int32
        client := &wireSFTP{
            in: bytes.NewReader(frame),
            out: &bytes.Buffer{},
            timeout: 10 * time.Millisecond,
            abort: func() { aborted.Add(1) },
        }
        typ, payload, err := client.exchange(func() error { return nil })
        if err != nil || typ != fxVersion || len(payload) != 4 {
            t.Fatalf("cycle %d: successful response failed: type=%d payload=%d err=%v", cycle, typ, len(payload), err)
        }
        time.Sleep(12 * time.Millisecond)
        if got := aborted.Load(); got != 0 {
            t.Fatalf("cycle %d: completed response closed its SFTP channel %d times", cycle, got)
        }
    }
}
