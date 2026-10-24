//go:build windows

package terminal

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"path/filepath"
	"sort"
	"sync"
	"syscall"
	"unsafe"
)

var (
	serialSetupOnce          sync.Once
	serialSetupDLL           *syscall.DLL
	serialSetupErr           error
	serialGetSystemDirectory = serialKernel32.NewProc("GetSystemDirectoryW")
	serialLoadLibraryEx      = serialKernel32.NewProc("LoadLibraryExW")
	serialQueryDosDevice     = serialKernel32.NewProc("QueryDosDeviceW")
)

func loadSerialSetupAPI() (*syscall.DLL, error) {
	serialSetupOnce.Do(func() {
		buffer := make([]uint16, 32768)
		n, _, err := serialGetSystemDirectory.Call(uintptr(unsafe.Pointer(&buffer[0])), uintptr(len(buffer)))
		if n == 0 || n >= uintptr(len(buffer)) {
			serialSetupErr = serialWindowsError("find the Windows system directory", err)
			return
		}
		path := filepath.Join(syscall.UTF16ToString(buffer[:n]), "setupapi.dll")
		name, err := syscall.UTF16PtrFromString(path)
		if err != nil {
			serialSetupErr = err
			return
		}
		handle, _, err := serialLoadLibraryEx.Call(uintptr(unsafe.Pointer(name)), 0, 0x100|0x800) // DLL directory and System32 only.
		if handle == 0 {
			serialSetupErr = serialWindowsError("load Windows serial enumeration API", err)
			return
		}
		serialSetupDLL = &syscall.DLL{Name: path, Handle: syscall.Handle(handle)}
	})
	return serialSetupDLL, serialSetupErr
}

type serialDeviceInfo struct {
	Size      uint32
	ClassGUID syscall.GUID
	DevInst   uint32
	Reserved  uintptr
}

func serialUTF16Bytes(data []byte) string {
	if len(data)%2 != 0 {
		return ""
	}
	characters := make([]uint16, len(data)/2)
	for i := range characters {
		characters[i] = binary.LittleEndian.Uint16(data[i*2:])
	}
	return syscall.UTF16ToString(characters)
}

func serialDeviceProperty(proc *syscall.Proc, handle uintptr, info *serialDeviceInfo, property uint32) string {
	buffer := make([]byte, 512)
	for attempt := 0; attempt < 3; attempt++ {
		var kind, needed uint32
		ok, _, err := proc.Call(handle, uintptr(unsafe.Pointer(info)), uintptr(property), uintptr(unsafe.Pointer(&kind)), uintptr(unsafe.Pointer(&buffer[0])), uintptr(len(buffer)), uintptr(unsafe.Pointer(&needed)))
		if ok != 0 {
			if needed > uint32(len(buffer)) || kind != syscall.REG_SZ && kind != syscall.REG_MULTI_SZ {
				return ""
			}
			return serialUTF16Bytes(buffer[:needed])
		}
		if !errors.Is(err, syscall.ERROR_INSUFFICIENT_BUFFER) || needed == 0 || needed > 64*1024 {
			return ""
		}
		buffer = make([]byte, needed)
	}
	return ""
}

func serialRegistryPort(key syscall.Handle) string {
	name, _ := syscall.UTF16PtrFromString("PortName")
	buffer := make([]byte, 256)
	needed, kind := uint32(len(buffer)), uint32(0)
	if err := syscall.RegQueryValueEx(key, name, nil, &kind, &buffer[0], &needed); err != nil || kind != syscall.REG_SZ || needed > uint32(len(buffer)) {
		return ""
	}
	return serialUTF16Bytes(buffer[:needed])
}

func listSetupSerialPorts(ctx context.Context) ([]SerialPort, error) {
	ports := make([]SerialPort, 0)
	dll, err := loadSerialSetupAPI()
	if err != nil {
		return ports, err
	}
	functions := make(map[string]*syscall.Proc)
	for _, name := range []string{"SetupDiGetClassDevsW", "SetupDiEnumDeviceInfo", "SetupDiGetDeviceRegistryPropertyW", "SetupDiOpenDevRegKey", "SetupDiDestroyDeviceInfoList"} {
		proc, err := dll.FindProc(name)
		if err != nil {
			return ports, err
		}
		functions[name] = proc
	}
	// GUID_DEVINTERFACE_COMPORT, documented in Ntddser.h and Microsoft Learn.
	class := syscall.GUID{Data1: 0x86e0d1e0, Data2: 0x8089, Data3: 0x11d0, Data4: [8]byte{0x9c, 0xe4, 0x08, 0x00, 0x3e, 0x30, 0x1f, 0x73}}
	handle, _, err := functions["SetupDiGetClassDevsW"].Call(uintptr(unsafe.Pointer(&class)), 0, 0, 0x2|0x10)
	if handle == uintptr(syscall.InvalidHandle) {
		return ports, serialWindowsError("enumerate present COM interfaces", err)
	}
	defer functions["SetupDiDestroyDeviceInfoList"].Call(handle)
	for index := uint32(0); index < 4096; index++ {
		if err := ctx.Err(); err != nil {
			return ports, err
		}
		info := serialDeviceInfo{Size: uint32(unsafe.Sizeof(serialDeviceInfo{}))}
		ok, _, err := functions["SetupDiEnumDeviceInfo"].Call(handle, uintptr(index), uintptr(unsafe.Pointer(&info)))
		if ok == 0 {
			if errors.Is(err, syscall.Errno(259)) { // ERROR_NO_MORE_ITEMS
				return ports, nil
			}
			return ports, serialWindowsError("enumerate COM device details", err)
		}
		key, _, _ := functions["SetupDiOpenDevRegKey"].Call(handle, uintptr(unsafe.Pointer(&info)), 1, 0, 1, syscall.KEY_QUERY_VALUE)
		if key == uintptr(syscall.InvalidHandle) {
			continue
		}
		name := serialRegistryPort(syscall.Handle(key))
		_ = syscall.RegCloseKey(syscall.Handle(key))
		name, err = normalizeSerialPort(name, "windows")
		if err != nil {
			continue
		}
		description := serialDeviceProperty(functions["SetupDiGetDeviceRegistryPropertyW"], handle, &info, 12) // SPDRP_FRIENDLYNAME
		if description == "" {
			description = serialDeviceProperty(functions["SetupDiGetDeviceRegistryPropertyW"], handle, &info, 0)
		}
		hardwareID := serialDeviceProperty(functions["SetupDiGetDeviceRegistryPropertyW"], handle, &info, 1)
		ports = append(ports, SerialPort{Name: name, Description: description, HardwareID: hardwareID})
	}
	return ports, errors.New("COM device enumeration exceeded 4096 entries")
}

func listDOSSerialPorts(ctx context.Context) ([]SerialPort, error) {
	for size := 4096; size <= 262144; size *= 2 {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		buffer := make([]uint16, size)
		n, _, err := serialQueryDosDevice.Call(0, uintptr(unsafe.Pointer(&buffer[0])), uintptr(len(buffer)))
		if n == 0 {
			if errors.Is(err, syscall.ERROR_INSUFFICIENT_BUFFER) {
				continue
			}
			return nil, serialWindowsError("enumerate COM names", err)
		}
		if n > uintptr(len(buffer)) {
			return nil, errors.New("invalid COM-name enumeration size")
		}
		ports := make([]SerialPort, 0)
		start := 0
		for i, c := range buffer[:n] {
			if c != 0 {
				continue
			}
			if i == start {
				break
			}
			name := syscall.UTF16ToString(buffer[start:i])
			start = i + 1
			if name, err := normalizeSerialPort(name, "windows"); err == nil {
				ports = append(ports, SerialPort{Name: name})
			}
		}
		return ports, nil
	}
	return nil, errors.New("COM-name enumeration exceeded its buffer limit")
}

func ListSerialPorts(ctx context.Context) ([]SerialPort, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	ports, setupErr := listSetupSerialPorts(ctx)
	if err := ctx.Err(); err != nil {
		return ports, err
	}
	fallback, fallbackErr := listDOSSerialPorts(ctx)
	byName := make(map[string]SerialPort)
	for _, port := range fallback {
		byName[port.Name] = port
	}
	for _, port := range ports {
		byName[port.Name] = port
	}
	ports = make([]SerialPort, 0, len(byName))
	for _, port := range byName {
		ports = append(ports, port)
	}
	sort.Slice(ports, func(i, j int) bool { return serialPortSortKey(ports[i].Name) < serialPortSortKey(ports[j].Name) })
	if err := ctx.Err(); err != nil {
		return ports, err
	}
	if setupErr != nil && fallbackErr != nil {
		return ports, fmt.Errorf("could not enumerate serial ports: %w", errors.Join(setupErr, fallbackErr))
	}
	return ports, nil
}

var _ [unsafe.Sizeof(serialDeviceInfo{}) - (24 + unsafe.Sizeof(uintptr(0)))]byte
var _ [(24 + unsafe.Sizeof(uintptr(0))) - unsafe.Sizeof(serialDeviceInfo{})]byte
