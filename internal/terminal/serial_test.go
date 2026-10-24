package terminal

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net"
	"runtime"
	"sync"
	"testing"
	"time"
	"unsafe"
)

func TestSerialPortValidationAndDefaults(t *testing.T) {
	for _, test := range []struct{ input, goos, want string }{
		{"com3", "windows", "COM3"}, {"COM0001", "windows", "COM1"}, {"\\\\.\\COM42", "windows", "COM42"},
		{"COM65535", "windows", "COM65535"}, {"/dev/ttyUSB0", "linux", "/dev/ttyUSB0"},
		{"/dev/pts/23", "linux", "/dev/pts/23"}, {"/dev/serial/by-id/usb-example", "linux", "/dev/serial/by-id/usb-example"},
	} {
		got, err := normalizeSerialPort(test.input, test.goos)
		if err != nil || got != test.want {
			t.Errorf("%q: %q %v", test.input, got, err)
		}
	}
	for _, value := range []string{"COM0", "COM65536", "COM+3", "COM-1", "COM3:", "COM3\\foo", "C:\\file", "\\\\server\\share", "NUL", "COM３"} {
		if _, err := normalizeSerialPort(value, "windows"); err == nil {
			t.Errorf("unsafe port accepted: %q", value)
		}
	}
	for _, value := range []string{"/dev/null", "/etc/passwd", "/dev/pts/../ttyUSB0", "/dev/pts/0/file", "/dev/ttyUSB*", "/dev/serial/by-id/../x"} {
		if _, err := normalizeSerialPort(value, "linux"); err == nil {
			t.Errorf("unsafe path accepted: %q", value)
		}
	}
	port := "/dev/ttyUSB0"
	if runtime.GOOS == "windows" {
		port = "COM3"
	}
	base, err := normalizeSerialRequest(ConnectRequest{SerialPort: port})
	if err != nil || base.BaudRate != 9600 || base.DataBits != 8 || base.StopBits != "1" || base.Parity != "none" || base.FlowControl != "none" {
		t.Fatalf("serial defaults: %+v %v", base, err)
	}
	for _, change := range []func(*ConnectRequest){
		func(r *ConnectRequest) { r.BaudRate = 49 }, func(r *ConnectRequest) { r.BaudRate = 4000001 },
		func(r *ConnectRequest) { r.DataBits = 9 }, func(r *ConnectRequest) { r.StopBits = "1.5" },
		func(r *ConnectRequest) { r.DataBits = 5; r.StopBits = "2" }, func(r *ConnectRequest) { r.Parity = "invalid" },
		func(r *ConnectRequest) { r.FlowControl = "invalid" }, func(r *ConnectRequest) { v := false; r.FlowControl = "rtscts"; r.SerialRTS = &v },
	} {
		req := base
		change(&req)
		if _, err := normalizeSerialRequest(req); err == nil {
			t.Errorf("invalid settings accepted: %+v", req)
		}
	}
}

func TestWindowsSerialDCBLayoutAndFlowFlags(t *testing.T) {
	dcb := serialDCB{}
	if unsafe.Sizeof(dcb) != 28 || unsafe.Offsetof(dcb.Flags) != 8 || unsafe.Offsetof(dcb.ByteSize) != 18 || unsafe.Offsetof(dcb.Parity) != 19 || unsafe.Offsetof(dcb.StopBits) != 20 || unsafe.Offsetof(dcb.Reserved1) != 26 {
		t.Fatalf("DCB does not match the Win32 layout: %+v size=%d", dcb, unsafe.Sizeof(dcb))
	}
	base := ConnectRequest{BaudRate: 115200, DataBits: 8, StopBits: "1", Parity: "none", FlowControl: "none"}
	dcb = makeSerialDCB(base)
	if dcb.Length != 28 || dcb.BaudRate != 115200 || dcb.ByteSize != 8 || dcb.Flags&1 == 0 || dcb.Flags&(3<<4) != 1<<4 || dcb.Flags&(3<<12) != 1<<12 || dcb.Flags&(1<<8|1<<9|1<<2) != 0 || dcb.XonChar != 17 || dcb.XoffChar != 19 {
		t.Fatalf("default DCB: %+v", dcb)
	}
	base.FlowControl = "rtscts"
	dcb = makeSerialDCB(base)
	if dcb.Flags&(1<<2) == 0 || dcb.Flags&(3<<12) != 2<<12 || dcb.Flags&(1<<8|1<<9) != 0 {
		t.Fatalf("RTS/CTS flags: %#x", dcb.Flags)
	}
	base.FlowControl = "xonxoff"
	base.Parity = "even"
	base.StopBits = "2"
	dcb = makeSerialDCB(base)
	if dcb.Flags&(1<<8|1<<9) != (1<<8|1<<9) || dcb.Flags&2 == 0 || dcb.Parity != 2 || dcb.StopBits != 2 {
		t.Fatalf("software flow/parity: %+v", dcb)
	}
	clear := false
	base.SerialDTR = &clear
	base.SerialRTS = &clear
	dcb = makeSerialDCB(base)
	if dcb.Flags&(3<<4|3<<12) != 0 {
		t.Fatal("manual low modem lines ignored")
	}
}

type fixtureSerialDevice struct {
	conn      net.Conn
	mu        sync.Mutex
	breaks    chan bool
	closed    bool
	failClear bool
}

func (d *fixtureSerialDevice) Read(p []byte) (int, error)  { return d.conn.Read(p) }
func (d *fixtureSerialDevice) Write(p []byte) (int, error) { return d.conn.Write(p) }
func (d *fixtureSerialDevice) Close() error {
	d.mu.Lock()
	d.closed = true
	d.mu.Unlock()
	return d.conn.Close()
}
func (d *fixtureSerialDevice) SetSignals(*bool, *bool) error { return nil }
func (d *fixtureSerialDevice) SetBreak(v bool) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.breaks <- v
	if !v && d.failClear {
		return errors.New("fixture clear failed")
	}
	return nil
}

func serialPipeFixture(t *testing.T) (*serialTransport, *fixtureSerialDevice, net.Conn, chan []byte, context.CancelFunc) {
	t.Helper()
	client, server := net.Pipe()
	ctx, cancel := context.WithCancel(context.Background())
	device := &fixtureSerialDevice{conn: client, breaks: make(chan bool, 16)}
	output := make(chan []byte, 16)
	tr := startSerial(ctx, device, transportCallbacks{Output: func(p []byte) { output <- bytes.Clone(p) }})
	t.Cleanup(func() { cancel(); _ = server.Close(); _ = tr.Close() })
	return tr, device, server, output, cancel
}

func TestSerialRawBytesAndCancellation(t *testing.T) {
	tr, _, server, output, cancel := serialPipeFixture(t)
	data := []byte{'\r', '\n', 0, 255, 27, '[', 'm'}
	if _, err := server.Write(data); err != nil {
		t.Fatal(err)
	}
	if got := collectTransportOutput(t, output, len(data)); !bytes.Equal(got, data) {
		t.Fatalf("serial output modified: %v", got)
	}
	written := make(chan error, 1)
	go func() { _, err := tr.Write(data); written <- err }()
	received := make([]byte, len(data))
	if _, err := io.ReadFull(server, received); err != nil {
		t.Fatal(err)
	}
	if err := <-written; err != nil || !bytes.Equal(received, data) {
		t.Fatalf("serial input modified: %v %v", received, err)
	}
	go func() { _, err := tr.Write(bytes.Repeat([]byte{'x'}, 1024)); written <- err }()
	cancel()
	select {
	case <-written:
	case <-time.After(time.Second):
		t.Fatal("cancel did not unblock serial write")
	}
	select {
	case err, ok := <-tr.Done():
		if !ok || err != nil {
			t.Fatalf("serial completion: %v %v", ok, err)
		}
	case <-time.After(time.Second):
		t.Fatal("cancel did not unblock serial read")
	}
	if _, ok := <-tr.Done(); ok {
		t.Fatal("serial Done was not closed after one result")
	}
}

func TestSerialBreakFailsafeAndCloseReset(t *testing.T) {
	tr, device, _, _, _ := serialPipeFixture(t)
	if err := tr.SetBreak(true); err != nil {
		t.Fatal(err)
	}
	if !<-device.breaks {
		t.Fatal("break was not asserted")
	}
	select {
	case enabled := <-device.breaks:
		if enabled {
			t.Fatal("break not cleared")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("failsafe did not clear break")
	}
	if err := tr.SetBreak(true); err != nil {
		t.Fatal(err)
	}
	if !<-device.breaks {
		t.Fatal("second break was not asserted")
	}
	if err := tr.Close(); err != nil {
		t.Fatal(err)
	}
	select {
	case enabled := <-device.breaks:
		if enabled {
			t.Fatal("close left break active")
		}
	case <-time.After(time.Second):
		t.Fatal("close did not reset break")
	}
	select {
	case <-tr.Done():
	case <-time.After(time.Second):
		t.Fatal("serial close did not finish")
	}
}

func TestSerialBreakResetFailureClosesSession(t *testing.T) {
	tr, device, _, _, _ := serialPipeFixture(t)
	device.mu.Lock()
	device.failClear = true
	device.mu.Unlock()
	if err := tr.SetBreak(true); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-tr.Done():
		if err == nil {
			t.Fatal("failed break reset was not reported")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("break reset failure left session running")
	}
	device.mu.Lock()
	closed := device.closed
	device.mu.Unlock()
	if !closed {
		t.Fatal("device remained open after failed break reset")
	}
}
