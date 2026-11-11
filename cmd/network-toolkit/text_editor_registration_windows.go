//go:build windows

package main

import (
	"encoding/binary"
	"errors"
	"path/filepath"
	"syscall"
	"unicode/utf16"
	"unsafe"
)

var (
	editorRegAdvapi        = syscall.NewLazyDLL(filepath.Join(localSystemDirectory, "advapi32.dll"))
	editorRegCreateKey     = editorRegAdvapi.NewProc("RegCreateKeyExW")
	editorRegSetValue      = editorRegAdvapi.NewProc("RegSetValueExW")
	editorRegDeleteValueW  = editorRegAdvapi.NewProc("RegDeleteValueW")
	editorRegDeleteKeyW    = editorRegAdvapi.NewProc("RegDeleteKeyW")
	editorRegEnumValueW    = editorRegAdvapi.NewProc("RegEnumValueW")
	editorRegShell         = syscall.NewLazyDLL(filepath.Join(localSystemDirectory, "shell32.dll"))
	editorRegChangeNotify  = editorRegShell.NewProc("SHChangeNotify")
	editorRegKernel        = syscall.NewLazyDLL(filepath.Join(localSystemDirectory, "kernel32.dll"))
	editorRegCreateMutex   = editorRegKernel.NewProc("CreateMutexW")
	editorRegErrorNotFound = syscall.ERROR_FILE_NOT_FOUND
)

const (
	editorRegNoneType = 0 // REG_NONE
	editorRegKeyWrite = 0x20006
	editorRegKeyRead  = syscall.KEY_READ
)

// The per-user registry, through advapi32 only. Every key is opened in the
// 64-bit view, which is the only view of HKCU\Software\Classes for the current
// user (the program is 64-bit).
type windowsEditorRegistry struct{}

func editorRegUTF16(s string) *uint16 {
	p, err := syscall.UTF16PtrFromString(s)
	if err != nil {
		return nil
	}
	return p
}

func editorRegOpen(path string, access uint32) (syscall.Handle, bool, error) {
	p := editorRegUTF16(path)
	if p == nil {
		return 0, false, errors.New("invalid registry path")
	}
	var key syscall.Handle
	err := syscall.RegOpenKeyEx(syscall.HKEY_CURRENT_USER, p, 0, access, &key)
	if errors.Is(err, editorRegErrorNotFound) || errors.Is(err, syscall.ERROR_PATH_NOT_FOUND) {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, err
	}
	return key, true, nil
}

func (windowsEditorRegistry) KeyInfo(path string) (bool, int, int, error) {
	key, exists, err := editorRegOpen(path, syscall.KEY_QUERY_VALUE|syscall.KEY_ENUMERATE_SUB_KEYS)
	if err != nil || !exists {
		return false, 0, 0, err
	}
	defer syscall.RegCloseKey(key)
	var subkeys, values uint32
	if err := syscall.RegQueryInfoKey(key, nil, nil, nil, &subkeys, nil, nil, &values, nil, nil, nil, nil); err != nil {
		return true, 0, 0, err
	}
	return true, int(subkeys), int(values), nil
}

func editorRegQuery(path, name string) (kind uint32, data []byte, found bool, err error) {
	key, exists, err := editorRegOpen(path, syscall.KEY_QUERY_VALUE)
	if err != nil || !exists {
		return 0, nil, false, err
	}
	defer syscall.RegCloseKey(key)
	n := editorRegUTF16(name)
	var size uint32
	err = syscall.RegQueryValueEx(key, n, nil, &kind, nil, &size)
	if errors.Is(err, editorRegErrorNotFound) {
		return 0, nil, false, nil
	}
	if err != nil {
		return 0, nil, false, err
	}
	if size > 64<<10 {
		return kind, nil, true, errors.New("registry value too large")
	}
	data = make([]byte, size)
	if size > 0 {
		if err = syscall.RegQueryValueEx(key, n, nil, &kind, &data[0], &size); err != nil {
			return 0, nil, false, err
		}
	}
	return kind, data[:size], true, nil
}

func (windowsEditorRegistry) String(path, name string) (string, bool, bool, error) {
	kind, data, found, err := editorRegQuery(path, name)
	if err != nil || !found {
		return "", found, false, err
	}
	if kind != syscall.REG_SZ && kind != syscall.REG_EXPAND_SZ || len(data)%2 != 0 {
		return "", true, false, nil
	}
	units := make([]uint16, len(data)/2)
	for i := range units {
		units[i] = binary.LittleEndian.Uint16(data[i*2:])
	}
	for len(units) > 0 && units[len(units)-1] == 0 {
		units = units[:len(units)-1]
	}
	return string(utf16.Decode(units)), true, true, nil
}

func (windowsEditorRegistry) DWORD(path, name string) (uint32, bool, error) {
	kind, data, found, err := editorRegQuery(path, name)
	if err != nil || !found {
		return 0, found, err
	}
	if kind != syscall.REG_DWORD || len(data) != 4 {
		return 0, true, nil
	}
	return binary.LittleEndian.Uint32(data), true, nil
}

func (windowsEditorRegistry) ValueNames(path string) ([]string, error) {
	key, exists, err := editorRegOpen(path, syscall.KEY_QUERY_VALUE)
	if err != nil || !exists {
		return nil, err
	}
	defer syscall.RegCloseKey(key)
	var names []string
	for index := uint32(0); ; index++ {
		buffer := make([]uint16, 16384)
		size := uint32(len(buffer))
		r, _, _ := editorRegEnumValueW.Call(uintptr(key), uintptr(index), uintptr(unsafe.Pointer(&buffer[0])), uintptr(unsafe.Pointer(&size)), 0, 0, 0, 0)
		if r == 259 { // ERROR_NO_MORE_ITEMS
			return names, nil
		}
		if r != 0 {
			return nil, syscall.Errno(r)
		}
		names = append(names, syscall.UTF16ToString(buffer[:size]))
	}
}

func editorRegCreate(path string) (syscall.Handle, error) {
	p := editorRegUTF16(path)
	if p == nil {
		return 0, errors.New("invalid registry path")
	}
	var key syscall.Handle
	r, _, _ := editorRegCreateKey.Call(uintptr(syscall.HKEY_CURRENT_USER), uintptr(unsafe.Pointer(p)), 0, 0, 0, editorRegKeyWrite, 0, uintptr(unsafe.Pointer(&key)), 0)
	if r != 0 {
		return 0, syscall.Errno(r)
	}
	return key, nil
}

func editorRegSet(path, name string, kind uint32, data []byte) error {
	key, err := editorRegCreate(path)
	if err != nil {
		return err
	}
	defer syscall.RegCloseKey(key)
	var pointer uintptr
	if len(data) > 0 {
		pointer = uintptr(unsafe.Pointer(&data[0]))
	}
	r, _, _ := editorRegSetValue.Call(uintptr(key), uintptr(unsafe.Pointer(editorRegUTF16(name))), 0, uintptr(kind), pointer, uintptr(len(data)))
	if r != 0 {
		return syscall.Errno(r)
	}
	return nil
}

func (windowsEditorRegistry) SetString(path, name, value string) error {
	units := utf16.Encode([]rune(value + "\x00"))
	data := make([]byte, len(units)*2)
	for i, unit := range units {
		binary.LittleEndian.PutUint16(data[i*2:], unit)
	}
	return editorRegSet(path, name, syscall.REG_SZ, data)
}

func (windowsEditorRegistry) SetDWORD(path, name string, value uint32) error {
	data := make([]byte, 4)
	binary.LittleEndian.PutUint32(data, value)
	return editorRegSet(path, name, syscall.REG_DWORD, data)
}

func (windowsEditorRegistry) SetNone(path, name string) error {
	return editorRegSet(path, name, editorRegNoneType, nil)
}

func (windowsEditorRegistry) DeleteValue(path, name string) error {
	key, exists, err := editorRegOpen(path, syscall.KEY_SET_VALUE)
	if err != nil || !exists {
		return err
	}
	defer syscall.RegCloseKey(key)
	r, _, _ := editorRegDeleteValueW.Call(uintptr(key), uintptr(unsafe.Pointer(editorRegUTF16(name))))
	if r != 0 && syscall.Errno(r) != editorRegErrorNotFound {
		return syscall.Errno(r)
	}
	return nil
}

// RegDeleteKeyW removes a key without subkeys only; it never deletes a tree.
func (windowsEditorRegistry) DeleteKey(path string) error {
	r, _, _ := editorRegDeleteKeyW.Call(uintptr(syscall.HKEY_CURRENT_USER), uintptr(unsafe.Pointer(editorRegUTF16(path))))
	if r != 0 && syscall.Errno(r) != editorRegErrorNotFound {
		return syscall.Errno(r)
	}
	return nil
}

// The mutex Setup and Uninstall hold for their whole run (AcquireSetupMutex in
// windows.nsi). Holding it here keeps a registration and Setup from
// interleaving; either one refuses while the other runs.
func editorRegSetupLock() (func(), error) {
	handle, _, callErr := editorRegCreateMutex.Call(0, 0, uintptr(unsafe.Pointer(editorRegUTF16(`Local\NetworkToolkitSetup`))))
	if handle == 0 {
		return nil, errors.New("Windows could not start the registration. Try again.")
	}
	if errors.Is(callErr, syscall.ERROR_ALREADY_EXISTS) {
		syscall.CloseHandle(syscall.Handle(handle))
		return nil, errors.New("Network Toolkit Setup or Uninstall is running. Wait until it has finished, then try again.")
	}
	return func() { syscall.CloseHandle(syscall.Handle(handle)) }, nil
}

func editorRegNotifyShell() {
	// SHCNE_ASSOCCHANGED, SHCNF_IDLIST: Explorer reloads associations and icons.
	editorRegChangeNotify.Call(0x08000000, 0, 0, 0)
}

func editorRegDefaultPlatform() (editorRegPlatform, bool) {
	return editorRegPlatform{registry: windowsEditorRegistry{}, lock: editorRegSetupLock, notify: editorRegNotifyShell, missing: editorRegFileMissing}, true
}
