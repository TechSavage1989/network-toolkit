//go:build linux

package terminal

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"syscall"
	"testing"
	"time"
	"unsafe"
)

func openSerialPTYFixture(t *testing.T) (*os.File, string) {
	t.Helper()
	fd, err := syscall.Open("/dev/ptmx", syscall.O_RDWR|syscall.O_NOCTTY|syscall.O_NONBLOCK|syscall.O_CLOEXEC, 0)
	if err != nil {
		t.Fatalf("open local pseudo-terminal: %v", err)
	}
	var unlock int32
	if err := linuxSerialIoctl(uintptr(fd), syscall.TIOCSPTLCK, unsafe.Pointer(&unlock)); err != nil {
		_ = syscall.Close(fd)
		t.Fatal(err)
	}
	var number uint32
	if err := linuxSerialIoctl(uintptr(fd), syscall.TIOCGPTN, unsafe.Pointer(&number)); err != nil {
		_ = syscall.Close(fd)
		t.Fatal(err)
	}
	master := os.NewFile(uintptr(fd), "/dev/ptmx")
	t.Cleanup(func() { _ = master.Close() })
	return master, fmt.Sprintf("/dev/pts/%d", number)
}

func TestLinuxSerialPTYRoundTripCancelAndReopen(t *testing.T) {
	master, path := openSerialPTYFixture(t)
	for round := 0; round < 2; round++ {
		ctx, cancel := context.WithCancel(context.Background())
		output := make(chan []byte, 16)
		tr, err := openSerial(ctx, ConnectRequest{SerialPort: path}, transportCallbacks{Output: func(p []byte) { output <- bytes.Clone(p) }})
		if err != nil {
			cancel()
			t.Fatal(err)
		}
		data := []byte("\x1b[32mrouter>\x1b[0m\r\n\x00\xff")
		if _, err := master.Write(data); err != nil {
			cancel()
			t.Fatal(err)
		}
		if got := collectTransportOutput(t, output, len(data)); !bytes.Equal(got, data) {
			cancel()
			t.Fatalf("PTY output changed: %v", got)
		}
		input := []byte("show version\r\x00\xff")
		if n, err := tr.Write(input); err != nil || n != len(input) {
			cancel()
			t.Fatalf("PTY write: %d %v", n, err)
		}
		_ = master.SetReadDeadline(time.Now().Add(time.Second))
		received := make([]byte, len(input))
		if _, err := io.ReadFull(master, received); err != nil || !bytes.Equal(input, received) {
			cancel()
			t.Fatalf("PTY input changed: %v %v", received, err)
		}
		cancel()
		select {
		case err := <-tr.Done():
			if err != nil {
				t.Fatal(err)
			}
		case <-time.After(time.Second):
			t.Fatal("PTY cancellation did not finish")
		}
	}
}
