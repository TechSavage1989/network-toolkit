//go:build linux

package terminal

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"syscall"
	"unsafe"
)

type linuxSerialDevice struct {
	file      *os.File
	flow      string
	controlMu sync.Mutex
}

var linuxSerialBaud = map[int]uint32{
	50: syscall.B50, 75: syscall.B75, 110: syscall.B110, 134: syscall.B134,
	150: syscall.B150, 200: syscall.B200, 300: syscall.B300, 600: syscall.B600,
	1200: syscall.B1200, 1800: syscall.B1800, 2400: syscall.B2400, 4800: syscall.B4800,
	9600: syscall.B9600, 19200: syscall.B19200, 38400: syscall.B38400, 57600: syscall.B57600,
	115200: syscall.B115200, 230400: syscall.B230400, 460800: syscall.B460800, 500000: syscall.B500000,
	576000: syscall.B576000, 921600: syscall.B921600, 1000000: syscall.B1000000,
	1152000: syscall.B1152000, 1500000: syscall.B1500000, 2000000: syscall.B2000000,
	2500000: syscall.B2500000, 3000000: syscall.B3000000, 3500000: syscall.B3500000, 4000000: syscall.B4000000,
}

func linuxSerialIoctl(fd uintptr, request uintptr, value unsafe.Pointer) error {
	_, _, errno := syscall.Syscall(syscall.SYS_IOCTL, fd, request, uintptr(value))
	if errno != 0 {
		return errno
	}
	return nil
}

func openSerialDevice(ctx context.Context, req ConnectRequest) (serialDevice, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	baud, supported := linuxSerialBaud[req.BaudRate]
	if !supported {
		return nil, fmt.Errorf("baud rate %d is not supported by this Linux serial build; use a standard baud rate", req.BaudRate)
	}
	fd, err := syscall.Open(req.SerialPort, syscall.O_RDWR|syscall.O_NOCTTY|syscall.O_NONBLOCK|syscall.O_CLOEXEC, 0)
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", req.SerialPort, err)
	}
	owned := true
	exclusive := false
	defer func() {
		if owned {
			if exclusive {
				_ = linuxSerialIoctl(uintptr(fd), syscall.TIOCNXCL, nil)
			}
			_ = syscall.Close(fd)
		}
	}()
	var stat syscall.Stat_t
	if err := syscall.Fstat(fd, &stat); err != nil {
		return nil, err
	}
	if stat.Mode&syscall.S_IFMT != syscall.S_IFCHR {
		return nil, errors.New("the selected serial path is not a character device")
	}
	var settings syscall.Termios
	if err := linuxSerialIoctl(uintptr(fd), syscall.TCGETS, unsafe.Pointer(&settings)); err != nil {
		return nil, fmt.Errorf("the selected device is not a usable terminal: %w", err)
	}
	settings.Iflag = 0
	settings.Oflag = 0
	settings.Lflag = 0
	settings.Cflag = syscall.CLOCAL | syscall.CREAD | syscall.HUPCL | baud
	settings.Ispeed, settings.Ospeed = baud, baud
	settings.Cc[syscall.VMIN], settings.Cc[syscall.VTIME] = 1, 0
	settings.Cc[syscall.VSTART], settings.Cc[syscall.VSTOP] = 0x11, 0x13
	settings.Cflag |= map[int]uint32{5: syscall.CS5, 6: syscall.CS6, 7: syscall.CS7, 8: syscall.CS8}[req.DataBits]
	if req.StopBits != "1" {
		settings.Cflag |= syscall.CSTOPB
	}
	if req.Parity != "none" {
		settings.Cflag |= syscall.PARENB
		settings.Iflag |= syscall.INPCK
		if req.Parity == "odd" || req.Parity == "mark" {
			settings.Cflag |= syscall.PARODD
		}
		if req.Parity == "mark" || req.Parity == "space" {
			settings.Cflag |= 0x40000000
		} // CMSPAR
	}
	if req.FlowControl == "rtscts" {
		settings.Cflag |= 0x80000000
	} // CRTSCTS
	if req.FlowControl == "xonxoff" {
		settings.Iflag |= syscall.IXON | syscall.IXOFF
	}
	if err := linuxSerialIoctl(uintptr(fd), syscall.TIOCEXCL, nil); err != nil {
		return nil, fmt.Errorf("reserve serial device: %w", err)
	}
	exclusive = true
	if err := linuxSerialIoctl(uintptr(fd), syscall.TCSETS, unsafe.Pointer(&settings)); err != nil {
		return nil, fmt.Errorf("configure serial device: %w", err)
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	file := os.NewFile(uintptr(fd), req.SerialPort)
	if file == nil {
		return nil, errors.New("could not create the serial file handle")
	}
	owned = false
	device := &linuxSerialDevice{file: file, flow: req.FlowControl}
	dtr, rts := true, true
	if req.SerialDTR != nil {
		dtr = *req.SerialDTR
	}
	if req.SerialRTS != nil {
		rts = *req.SerialRTS
	}
	var rtsSetting *bool
	if req.FlowControl != "rtscts" {
		rtsSetting = &rts
	}
	if err := device.SetSignals(&dtr, rtsSetting); err != nil {
		// PTYs have no modem lines. Their normal default configuration remains
		// usable for local terminal integration tests, but explicit line control
		// is never reported as successful when the device cannot perform it.
		if !(errors.Is(err, syscall.ENOTTY) && req.SerialDTR == nil && req.SerialRTS == nil) {
			_ = device.Close()
			return nil, fmt.Errorf("set serial modem lines: %w", err)
		}
	}
	return device, nil
}

func (d *linuxSerialDevice) Read(data []byte) (int, error)  { return d.file.Read(data) }
func (d *linuxSerialDevice) Write(data []byte) (int, error) { return d.file.Write(data) }
func (d *linuxSerialDevice) Close() error {
	_ = d.control(func(fd uintptr) error { return linuxSerialIoctl(fd, syscall.TIOCNXCL, nil) })
	return d.file.Close()
}

func (d *linuxSerialDevice) control(fn func(uintptr) error) error {
	d.controlMu.Lock()
	defer d.controlMu.Unlock()
	raw, err := d.file.SyscallConn()
	if err != nil {
		return err
	}
	var result error
	err = raw.Control(func(fd uintptr) { result = fn(fd) })
	if err != nil {
		return err
	}
	return result
}

func (d *linuxSerialDevice) SetSignals(dtr, rts *bool) error {
	if rts != nil && d.flow == "rtscts" {
		return errors.New("RTS is managed by hardware flow control")
	}
	return d.control(func(fd uintptr) error {
		var set, clear int32
		for _, signal := range []struct {
			value *bool
			bit   int32
		}{{dtr, syscall.TIOCM_DTR}, {rts, syscall.TIOCM_RTS}} {
			if signal.value == nil {
				continue
			}
			if *signal.value {
				set |= signal.bit
			} else {
				clear |= signal.bit
			}
		}
		if set != 0 {
			if err := linuxSerialIoctl(fd, syscall.TIOCMBIS, unsafe.Pointer(&set)); err != nil {
				return err
			}
		}
		if clear != 0 {
			return linuxSerialIoctl(fd, syscall.TIOCMBIC, unsafe.Pointer(&clear))
		}
		return nil
	})
}

func (d *linuxSerialDevice) SetBreak(enabled bool) error {
	return d.control(func(fd uintptr) error {
		request := uintptr(syscall.TIOCCBRK)
		if enabled {
			request = syscall.TIOCSBRK
		}
		return linuxSerialIoctl(fd, request, nil)
	})
}

func ListSerialPorts(ctx context.Context) ([]SerialPort, error) {
	ports := make([]SerialPort, 0)
	seen := make(map[string]bool)
	for _, pattern := range []string{"/dev/serial/by-id/*", "/dev/ttyUSB*", "/dev/ttyACM*", "/dev/ttyS*", "/dev/ttyAMA*", "/dev/ttyTHS*", "/dev/rfcomm*"} {
		if err := ctx.Err(); err != nil {
			return ports, err
		}
		paths, _ := filepath.Glob(pattern)
		for _, path := range paths {
			if err := ctx.Err(); err != nil {
				return ports, err
			}
			if _, err := normalizeSerialPort(path, "linux"); err != nil {
				continue
			}
			info, err := os.Stat(path)
			if err != nil || info.Mode()&os.ModeCharDevice == 0 {
				continue
			}
			actual, err := filepath.EvalSymlinks(path)
			if err != nil || seen[actual] {
				continue
			}
			seen[actual] = true
			ports = append(ports, SerialPort{Name: path, Description: filepath.Base(path)})
			if len(ports) >= 1024 {
				return ports, errors.New("serial device enumeration exceeded 1024 entries")
			}
		}
	}
	sort.Slice(ports, func(i, j int) bool { return ports[i].Name < ports[j].Name })
	return ports, nil
}
