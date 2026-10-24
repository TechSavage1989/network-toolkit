package terminal

import (
	"errors"
	"fmt"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
)

func normalizeSerialRequest(req ConnectRequest) (ConnectRequest, error) {
	var err error
	req.SerialPort, err = normalizeSerialPort(req.SerialPort, runtime.GOOS)
	if err != nil {
		return req, err
	}
	if req.BaudRate == 0 {
		req.BaudRate = 9600
	}
	if req.DataBits == 0 {
		req.DataBits = 8
	}
	if req.StopBits == "" {
		req.StopBits = "1"
	}
	if req.Parity == "" {
		req.Parity = "none"
	}
	if req.FlowControl == "" {
		req.FlowControl = "none"
	}
	req.Parity = strings.ToLower(strings.TrimSpace(req.Parity))
	req.FlowControl = strings.ToLower(strings.TrimSpace(req.FlowControl))
	req.StopBits = strings.TrimSpace(req.StopBits)
	if req.BaudRate < 50 || req.BaudRate > 4000000 {
		return req, errors.New("serial baud rate must be between 50 and 4000000")
	}
	if req.DataBits < 5 || req.DataBits > 8 {
		return req, errors.New("serial data bits must be between 5 and 8")
	}
	if req.StopBits != "1" && req.StopBits != "1.5" && req.StopBits != "2" {
		return req, errors.New("serial stop bits must be 1, 1.5 or 2")
	}
	if req.StopBits == "1.5" && req.DataBits != 5 {
		return req, errors.New("1.5 stop bits requires 5 data bits")
	}
	if req.StopBits == "2" && req.DataBits == 5 {
		return req, errors.New("2 stop bits requires 6, 7 or 8 data bits")
	}
	if req.Parity != "none" && req.Parity != "odd" && req.Parity != "even" && req.Parity != "mark" && req.Parity != "space" {
		return req, errors.New("serial parity must be none, odd, even, mark or space")
	}
	if req.FlowControl != "none" && req.FlowControl != "rtscts" && req.FlowControl != "xonxoff" {
		return req, errors.New("serial flow control must be none, rtscts or xonxoff")
	}
	if req.FlowControl == "rtscts" && req.SerialRTS != nil {
		return req, errors.New("RTS is managed by RTS/CTS flow control; remove the manual RTS setting")
	}
	return req, nil
}

func normalizeSerialPort(value, goos string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" || len(value) > 512 || strings.ContainsRune(value, 0) {
		return "", errors.New("select a serial port")
	}
	if goos == "windows" {
		value = strings.ToUpper(value)
		value = strings.TrimPrefix(value, "\\\\.\\")
		if !strings.HasPrefix(value, "COM") {
			return "", errors.New("use a COM port name such as COM3")
		}
		n, err := strconv.Atoi(value[3:])
		if err != nil || n < 1 || n > 65535 {
			return "", errors.New("COM port number must be between 1 and 65535")
		}
		for _, c := range value[3:] {
			if c < '0' || c > '9' {
				return "", errors.New("invalid COM port name")
			}
		}
		return "COM" + strconv.Itoa(n), nil
	}
	if value != filepath.Clean(value) {
		return "", errors.New("use a complete serial device path without traversal")
	}
	for _, prefix := range []string{"/dev/ttyS", "/dev/ttyUSB", "/dev/ttyACM", "/dev/ttyAMA", "/dev/ttyTHS", "/dev/rfcomm", "/dev/pts/"} {
		if strings.HasPrefix(value, prefix) {
			suffix := strings.TrimPrefix(value, prefix)
			if suffix == "" {
				continue
			}
			valid := true
			for _, c := range suffix {
				if c < '0' || c > '9' {
					valid = false
					break
				}
			}
			if valid {
				return value, nil
			}
		}
	}
	if strings.HasPrefix(value, "/dev/serial/by-id/") && filepath.Base(value) != "by-id" && !strings.Contains(strings.TrimPrefix(value, "/dev/serial/by-id/"), "/") {
		return value, nil
	}
	return "", errors.New("use an explicit serial device under /dev, such as /dev/ttyUSB0")
}

// Fixed-width fields match the 28-byte Win32 DCB. Keeping the layout and flag
// construction platform-independent lets normal tests verify the Windows ABI.
type serialDCB struct {
	Length    uint32
	BaudRate  uint32
	Flags     uint32
	Reserved  uint16
	XonLimit  uint16
	XoffLimit uint16
	ByteSize  uint8
	Parity    uint8
	StopBits  uint8
	XonChar   uint8
	XoffChar  uint8
	ErrorChar uint8
	EOFChar   uint8
	EventChar uint8
	Reserved1 uint16
}

func makeSerialDCB(req ConnectRequest) serialDCB {
	dcb := serialDCB{Length: 28, BaudRate: uint32(req.BaudRate), Flags: 1 | 1<<4 | 1<<7 | 1<<12,
		ByteSize: uint8(req.DataBits), XonLimit: 1024, XoffLimit: 1024, XonChar: 0x11, XoffChar: 0x13}
	dcb.Parity = map[string]uint8{"none": 0, "odd": 1, "even": 2, "mark": 3, "space": 4}[req.Parity]
	dcb.StopBits = map[string]uint8{"1": 0, "1.5": 1, "2": 2}[req.StopBits]
	if req.Parity != "none" {
		dcb.Flags |= 1 << 1
	}
	if req.SerialDTR != nil && !*req.SerialDTR {
		dcb.Flags &^= 3 << 4
	}
	if req.SerialRTS != nil && !*req.SerialRTS {
		dcb.Flags &^= 3 << 12
	}
	if req.FlowControl == "rtscts" {
		dcb.Flags = dcb.Flags&^(3<<12) | 2<<12 | 1<<2
	}
	if req.FlowControl == "xonxoff" {
		dcb.Flags |= 1<<8 | 1<<9
	}
	return dcb
}

func serialPortSortKey(name string) string {
	if normalized, err := normalizeSerialPort(name, "windows"); err == nil {
		n, _ := strconv.Atoi(normalized[3:])
		return fmt.Sprintf("COM%05d", n)
	}
	return name
}
