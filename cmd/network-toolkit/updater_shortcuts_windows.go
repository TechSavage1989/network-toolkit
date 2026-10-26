//go:build windows

package main

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"unsafe"
)

var updateShortcutOLE = syscall.NewLazyDLL("ole32.dll")

type updateShortcutCOM struct{ table *[21]uintptr }

// Pointer-valued COM arguments cross this Go wrapper before reaching SyscallN.
// Keep their storage off the movable Go stack and alive through the call.
//
//go:uintptrescapes
func (p *updateShortcutCOM) call(method int, args ...uintptr) uintptr {
	params := append([]uintptr{uintptr(unsafe.Pointer(p))}, args...)
	r, _, _ := syscall.SyscallN(p.table[method], params...)
	runtime.KeepAlive(p)
	return r
}

func updateShortcutHRESULT(operation string, result uintptr) error {
	if result != 0 { // Require S_OK, not an incomplete S_FALSE serialization.
		return fmt.Errorf("%s failed (Windows HRESULT 0x%08x)", operation, uint32(result))
	}
	return nil
}

func updateMigrateShortcuts(oldPaths []string, newPath, settingsRoot string) error {
	if len(oldPaths) == 0 {
		return nil
	}
	if err := updateShortcutValidatePaths(oldPaths, newPath, settingsRoot); err != nil {
		return err
	}
	// IShellLink::GetPath only guarantees MAX_PATH. Do not mistake a
	// truncated path for an exact match, or retire an old target we cannot
	// reliably recognize. Keep such installations pending for manual repair.
	for _, path := range append(append([]string(nil), oldPaths...), newPath) {
		chars, err := syscall.UTF16FromString(path)
		if err != nil || len(chars) >= 260 {
			return errors.New("Shortcut target exceeds the safe Windows Shell path length; existing shortcuts were left unchanged")
		}
	}
	// COM apartment ownership is thread-local. Nothing is launched and Resolve
	// is never called, so Shell tracking cannot search for alternate targets.
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	r, _, _ := updateShortcutOLE.NewProc("CoInitializeEx").Call(0, 2|4)
	if int32(r) < 0 {
		return updateShortcutHRESULT("Initialize shortcut migration", r)
	}
	defer updateShortcutOLE.NewProc("CoUninitialize").Call()
	var roots []updateShortcutRoot
	var failures []error
	for _, folder := range []struct {
		id    syscall.GUID
		name  string
		depth int
	}{
		{syscall.GUID{Data1: 0xb4bfcc3a, Data2: 0xdb2c, Data3: 0x424c, Data4: [8]byte{0xb0, 0x29, 0x7f, 0xe9, 0x9a, 0x87, 0xc6, 0x41}}, "Personal Desktop", 0},
		{syscall.GUID{Data1: 0x625b53c3, Data2: 0xab48, Data3: 0x4ec1, Data4: [8]byte{0xba, 0x1f, 0xa1, 0xef, 0x41, 0x46, 0xfc, 0x19}}, "Personal Start Menu", updateShortcutMaxDepth},
	} {
		path, err := updateShortcutKnownFolder(folder.id)
		if err != nil {
			failures = append(failures, fmt.Errorf("%s: %w", folder.name, err))
			continue
		}
		roots = append(roots, updateShortcutRoot{path, folder.depth})
	}
	if len(roots) > 0 {
		failures = append(failures, updateMigrateShortcutsInRoots(oldPaths, newPath, settingsRoot, roots, updateShortcutOps{updateShortcutStageNative, updateShortcutCommitNative}))
	}
	return errors.Join(failures...)
}

func updateShortcutKnownFolder(id syscall.GUID) (string, error) {
	var ptr *uint16
	// Token NULL means this user, never the Default User or Public folder.
	r, _, _ := syscall.NewLazyDLL("shell32.dll").NewProc("SHGetKnownFolderPath").Call(uintptr(unsafe.Pointer(&id)), 0, 0, uintptr(unsafe.Pointer(&ptr)))
	if ptr != nil {
		defer updateShortcutOLE.NewProc("CoTaskMemFree").Call(uintptr(unsafe.Pointer(ptr)))
	}
	if err := updateShortcutHRESULT("Find per-user shortcut folder", r); err != nil {
		return "", err
	}
	if ptr == nil {
		return "", errors.New("Windows did not return a per-user shortcut folder")
	}
	// SHGetKnownFolderPath owns a terminated UTF-16 string; bound the read to
	// the maximum normal Windows path rather than inventing a fallback folder.
	var chars []uint16
	for i := 0; i < 32768; i++ {
		c := *(*uint16)(unsafe.Add(unsafe.Pointer(ptr), i*2))
		if c == 0 {
			return syscall.UTF16ToString(chars), nil
		}
		chars = append(chars, c)
	}
	return "", errors.New("Windows returned an overlong shortcut folder")
}

func updateShortcutCreate() (*updateShortcutCOM, *updateShortcutCOM, error) {
	class := syscall.GUID{Data1: 0x00021401, Data4: [8]byte{0xc0, 0, 0, 0, 0, 0, 0, 0x46}}
	linkIID := syscall.GUID{Data1: 0x000214f9, Data4: [8]byte{0xc0, 0, 0, 0, 0, 0, 0, 0x46}}
	persistIID := syscall.GUID{Data1: 0x0000010b, Data4: [8]byte{0xc0, 0, 0, 0, 0, 0, 0, 0x46}}
	var link, persist *updateShortcutCOM
	r, _, _ := updateShortcutOLE.NewProc("CoCreateInstance").Call(uintptr(unsafe.Pointer(&class)), 0, 1, uintptr(unsafe.Pointer(&linkIID)), uintptr(unsafe.Pointer(&link)))
	if err := updateShortcutHRESULT("Create Shell shortcut reader", r); err != nil {
		return nil, nil, err
	}
	if err := updateShortcutHRESULT("Open shortcut persistence interface", link.call(0, uintptr(unsafe.Pointer(&persistIID)), uintptr(unsafe.Pointer(&persist)))); err != nil {
		link.call(2)
		return nil, nil, err
	}
	return link, persist, nil
}

func updateShortcutLoad(path string) (*updateShortcutCOM, *updateShortcutCOM, error) {
	link, persist, err := updateShortcutCreate()
	if err != nil {
		return nil, nil, err
	}
	filename, err := syscall.UTF16PtrFromString(path)
	if err == nil {
		// IPersistFile::Load, STGM_READ. Never ShellExecute or Resolve.
		err = updateShortcutHRESULT("Read shortcut", persist.call(5, uintptr(unsafe.Pointer(filename)), 0))
		runtime.KeepAlive(filename)
	}
	if err != nil {
		persist.call(2)
		link.call(2)
		return nil, nil, err
	}
	return link, persist, nil
}

func updateShortcutRawTarget(link *updateShortcutCOM) (string, error) {
	var buffer [32768]uint16
	r := link.call(3, uintptr(unsafe.Pointer(&buffer[0])), uintptr(len(buffer)), 0, 4) // SLGP_RAWPATH
	if r == 1 {                                                                        // S_FALSE: no filesystem path, so it cannot be our old EXE.
		return "", nil
	}
	if err := updateShortcutHRESULT("Read exact shortcut target", r); err != nil {
		return "", err
	}
	if buffer[len(buffer)-1] != 0 {
		return "", errors.New("Shortcut target is too long")
	}
	for i, c := range buffer {
		if c == 0 {
			if i >= 259 {
				return "", errors.New("Shortcut target may exceed Windows Shell path limits; left unchanged")
			}
			break
		}
	}
	return syscall.UTF16ToString(buffer[:]), nil
}

func updateShortcutStageNative(source, staged, newPath string, oldPaths []string) (bool, error) {
	link, persist, err := updateShortcutLoad(source)
	if err != nil {
		return false, err
	}
	defer link.call(2)
	defer persist.call(2)
	target, err := updateShortcutRawTarget(link)
	if err != nil {
		return false, err
	}
	matched := false
	if filepath.IsAbs(target) && filepath.Clean(target) == target && validateStoragePlatformPath(target) == nil {
		for _, old := range oldPaths {
			if strings.EqualFold(target, old) {
				matched = true
				break
			}
		}
	}
	if !matched {
		return false, nil
	}
	original, err := updateNamingReadFile(source, updateShortcutLimit)
	if err != nil {
		return false, err
	}
	settings, err := updateShortcutParseSettings(original)
	if err != nil {
		return false, err
	}
	// Read the persisted icon, not a Shell-computed default. If no explicit
	// icon exists, leave it implicit so it naturally uses the new target.
	iconPath := updateShortcutString(settings.strings[4], settings.flags&0x80 != 0)
	newTarget, err := syscall.UTF16PtrFromString(newPath)
	if err != nil {
		return false, err
	}
	if err = updateShortcutHRESULT("Retarget shortcut copy", link.call(20, uintptr(unsafe.Pointer(newTarget)))); err != nil {
		return false, err
	}
	for _, old := range oldPaths {
		if strings.EqualFold(iconPath, old) {
			var icon [32768]uint16
			var iconIndex int32
			if err = updateShortcutHRESULT("Read shortcut icon index", link.call(16, uintptr(unsafe.Pointer(&icon[0])), uintptr(len(icon)), uintptr(unsafe.Pointer(&iconIndex)))); err != nil {
				return false, err
			}
			if err = updateShortcutHRESULT("Retarget application icon", link.call(17, uintptr(unsafe.Pointer(newTarget)), uintptr(iconIndex))); err != nil {
				return false, err
			}
			break
		}
	}
	runtime.KeepAlive(newTarget)
	file, err := syscall.UTF16PtrFromString(staged)
	if err != nil {
		return false, err
	}
	// FALSE: Save a Copy As. This never writes the original .lnk, and retains
	// all Shell fields instead of rebuilding a minimal WScript shortcut.
	if err = updateShortcutHRESULT("Save shortcut copy", persist.call(6, uintptr(unsafe.Pointer(file)), 0)); err != nil {
		return false, err
	}
	runtime.KeepAlive(file)
	check, checkPersist, err := updateShortcutLoad(staged)
	if err != nil {
		return false, err
	}
	defer check.call(2)
	defer checkPersist.call(2)
	actual, err := updateShortcutRawTarget(check)
	if err != nil || !strings.EqualFold(actual, newPath) {
		return false, errors.New("Saved shortcut did not retain the exact stable target; original was left unchanged")
	}
	return true, nil
}

func updateShortcutCommitNative(path string, identity os.FileInfo, before, after []byte) error {
	if err := updateNamingCheckDirectory(filepath.Dir(path)); err != nil {
		return err
	}
	p, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return err
	}
	// OPEN_REPARSE_POINT avoids following a swapped .lnk symlink. Denying
	// write/delete sharing pins the verified file against other writers while
	// updating it, preserves its file identity/ACL/streams, and needs no admin.
	h, err := syscall.CreateFile(p, syscall.GENERIC_READ|syscall.GENERIC_WRITE, syscall.FILE_SHARE_READ, nil, syscall.OPEN_EXISTING, syscall.FILE_FLAG_OPEN_REPARSE_POINT, 0)
	if err != nil {
		return err
	}
	f := os.NewFile(uintptr(h), path)
	defer f.Close()
	var attributes syscall.ByHandleFileInformation
	if err = syscall.GetFileInformationByHandle(h, &attributes); err != nil {
		return err
	}
	if attributes.FileAttributes&(syscall.FILE_ATTRIBUTE_REPARSE_POINT|syscall.FILE_ATTRIBUTE_DIRECTORY) != 0 || attributes.NumberOfLinks != 1 {
		return errors.New("Shortcut is a reparse point, directory or hard-linked file; left unchanged")
	}
	opened, err := f.Stat()
	if err != nil || !os.SameFile(identity, opened) {
		return errors.New("Shortcut identity changed before writing; left unchanged")
	}
	current, err := io.ReadAll(io.LimitReader(f, updateShortcutLimit+1))
	if err != nil || !bytes.Equal(current, before) {
		return errors.New("Shortcut contents changed before writing; left unchanged")
	}
	currentIdentity, err := os.Lstat(path)
	if err != nil || storageIsLink(currentIdentity) || !os.SameFile(opened, currentIdentity) {
		return errors.New("Shortcut path changed before writing; left unchanged")
	}
	write := func(data []byte) error {
		if _, err := f.Seek(0, io.SeekStart); err != nil {
			return err
		}
		if n, err := f.Write(data); err != nil {
			return err
		} else if n != len(data) {
			return io.ErrShortWrite
		}
		if err := f.Truncate(int64(len(data))); err != nil {
			return err
		}
		if err := f.Sync(); err != nil {
			return err
		}
		if _, err := f.Seek(0, io.SeekStart); err != nil {
			return err
		}
		readback, err := io.ReadAll(io.LimitReader(f, updateShortcutLimit+1))
		if err != nil {
			return err
		}
		if !bytes.Equal(readback, data) {
			return errors.New("Shortcut write could not be verified")
		}
		return nil
	}
	if err := write(after); err != nil {
		if restoreErr := write(before); restoreErr != nil {
			return fmt.Errorf("Shortcut update failed (%v) and restoration failed (%v); original backup remains in settings", err, restoreErr)
		}
		return fmt.Errorf("Shortcut update failed; original restored: %w", err)
	}
	return nil
}
