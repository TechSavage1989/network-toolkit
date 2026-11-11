//go:build windows

package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"unsafe"
)

// Who can change a file or folder, and folders and files only this user can
// change (security review 11): the Text Editor registration refuses a copy
// that others can change, and "Install the full version" keeps the downloaded
// Setup in an owner-only folder and starts it from a handle that forbids
// writing and deleting it.

var (
	privateGetAce             = credentialSecurityDLL.NewProc("GetAce")
	privateCreateDirectory    = syscall.NewLazyDLL("kernel32.dll").NewProc("CreateDirectoryW")
	privateGetFinalPathByFile = syscall.NewLazyDLL("kernel32.dll").NewProc("GetFinalPathNameByHandleW")
)

// SIDs that may always change the program: SYSTEM, Administrators and
// TrustedInstaller (the current user is added at run time).
var privateTrustedSIDs = map[string]bool{
	"S-1-5-18":     true,
	"S-1-5-32-544": true,
	"S-1-5-80-956008885-3418522649-1831038044-1853292631-2271478464": true,
}

// Rights that let someone change a file or what a folder holds: write or
// append data (add a file or folder), delete, delete a child, change the
// permissions or the owner, and the generic write and all rights.
const privateWriteRights = 0x2 | 0x4 | 0x40 | 0x10000 | 0x40000 | 0x80000 | 0x10000000 | 0x40000000

func privateAccountName(sid *syscall.SID) string {
	if account, domain, _, err := sid.LookupAccount(""); err == nil && account != "" {
		if domain != "" {
			return domain + `\` + account
		}
		return account
	}
	text, _ := sid.String()
	return text
}

// editorRegWritableByOthers names who, besides this user, SYSTEM,
// Administrators and TrustedInstaller, can change path: through an allow
// entry with a write right, or as its owner (an owner can always rewrite the
// permissions). A missing access list means everyone.
func editorRegWritableByOthers(path string) (string, error) {
	user, err := credentialCurrentUserSID()
	if err != nil {
		return "", err
	}
	name, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return "", err
	}
	var owner, dacl, descriptor unsafe.Pointer
	// SE_FILE_OBJECT; OWNER_SECURITY_INFORMATION | DACL_SECURITY_INFORMATION.
	code, _, _ := credentialGetNamedSecurity.Call(uintptr(unsafe.Pointer(name)), 1, 1|4, uintptr(unsafe.Pointer(&owner)), 0, uintptr(unsafe.Pointer(&dacl)), 0, uintptr(unsafe.Pointer(&descriptor)))
	runtime.KeepAlive(name)
	if code != 0 {
		return "", syscall.Errno(code)
	}
	defer syscall.LocalFree(syscall.Handle(uintptr(descriptor)))
	trusted := func(sid string) bool { return sid == user || privateTrustedSIDs[sid] }
	var others []string
	add := func(who string) {
		for _, seen := range others {
			if seen == who {
				return
			}
		}
		others = append(others, who)
	}
	ownerTrusted := true
	if owner != nil {
		sid := (*syscall.SID)(owner)
		text, err := sid.String()
		if err != nil {
			return "", err
		}
		if !trusted(text) {
			ownerTrusted = false
			add(privateAccountName(sid) + " (the owner)")
		}
	}
	if dacl == nil {
		return "Everyone (the item has no access list)", nil
	}
	count := *(*uint16)(unsafe.Add(dacl, 4)) // ACL.AceCount
	for i := uint16(0); i < count; i++ {
		var ace unsafe.Pointer
		if ok, _, callErr := privateGetAce.Call(uintptr(dacl), uintptr(i), uintptr(unsafe.Pointer(&ace))); ok == 0 {
			return "", callErr
		}
		kind, flags := *(*byte)(ace), *(*byte)(unsafe.Add(ace, 1))
		if flags&0x08 != 0 { // INHERIT_ONLY_ACE: does not apply to this item
			continue
		}
		switch kind {
		case 1, 6, 10, 12: // deny entries never grant
			continue
		case 0, 9: // ACCESS_ALLOWED_ACE, ACCESS_ALLOWED_CALLBACK_ACE
		default:
			// An allow entry of another layout: count it unless it grants nothing.
			if *(*uint32)(unsafe.Add(ace, 4))&privateWriteRights != 0 {
				add("an access entry of type " + fmt.Sprint(kind))
			}
			continue
		}
		if *(*uint32)(unsafe.Add(ace, 4))&privateWriteRights == 0 {
			continue
		}
		sid := (*syscall.SID)(unsafe.Add(ace, 8))
		text, err := sid.String()
		if err != nil {
			return "", err
		}
		switch {
		case trusted(text), text == "S-1-3-0": // CREATOR OWNER applies only to new children
		case text == "S-1-3-4": // OWNER RIGHTS: as trusted as the owner
			if !ownerTrusted {
				add("the owner")
			}
		default:
			add(privateAccountName(sid))
		}
	}
	return strings.Join(others, ", "), nil
}

// fullInstallMakePrivateDir creates one folder whose protected access list
// lets only the current user in, set at creation (never afterwards by path).
// An existing folder is reported as os.ErrExist and left alone.
func fullInstallMakePrivateDir(path string) error {
	user, err := credentialCurrentUserSID()
	if err != nil {
		return err
	}
	text, err := syscall.UTF16PtrFromString("D:P(A;OICI;FA;;;" + user + ")")
	if err != nil {
		return err
	}
	var descriptor uintptr
	ok, _, callErr := credentialConvertDescriptor.Call(uintptr(unsafe.Pointer(text)), 1, uintptr(unsafe.Pointer(&descriptor)), 0)
	runtime.KeepAlive(text)
	if ok == 0 {
		return callErr
	}
	defer syscall.LocalFree(syscall.Handle(descriptor))
	name, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return err
	}
	attributes := syscall.SecurityAttributes{SecurityDescriptor: descriptor}
	attributes.Length = uint32(unsafe.Sizeof(attributes))
	ok, _, callErr = privateCreateDirectory.Call(uintptr(unsafe.Pointer(name)), uintptr(unsafe.Pointer(&attributes)))
	runtime.KeepAlive(name)
	runtime.KeepAlive(attributes)
	if ok == 0 {
		if errors.Is(callErr, syscall.ERROR_ALREADY_EXISTS) || errors.Is(callErr, syscall.ERROR_FILE_EXISTS) {
			return os.ErrExist
		}
		return &os.PathError{Op: "mkdir", Path: path, Err: callErr}
	}
	return nil
}

// fullInstallCheckPrivateDir accepts only a real folder (no junction or link)
// that nobody but this user, SYSTEM and Administrators can change.
func fullInstallCheckPrivateDir(path string) error {
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if !info.IsDir() || storageIsLink(info) {
		return errors.New(path + " is not a plain folder")
	}
	others, err := editorRegWritableByOthers(path)
	if err != nil {
		return err
	}
	if others != "" {
		return errors.New(others + " can change " + path)
	}
	return nil
}

// fullInstallOpenHeld opens the downloaded Setup once, for reading, sharing
// only reading: while the handle is open nobody can write, rename or delete
// the file. The handle must name exactly path (no link or junction on the
// way) and be a plain file.
func fullInstallOpenHeld(path string) (*os.File, error) {
	name, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return nil, err
	}
	const openReparsePoint = 0x00200000 // FILE_FLAG_OPEN_REPARSE_POINT
	handle, err := syscall.CreateFile(name, syscall.GENERIC_READ, syscall.FILE_SHARE_READ, nil, syscall.OPEN_EXISTING, syscall.FILE_ATTRIBUTE_NORMAL|openReparsePoint, 0)
	runtime.KeepAlive(name)
	if err != nil {
		return nil, &os.PathError{Op: "open", Path: path, Err: err}
	}
	file := os.NewFile(uintptr(handle), path)
	var info syscall.ByHandleFileInformation
	if err := syscall.GetFileInformationByHandle(handle, &info); err != nil {
		file.Close()
		return nil, err
	}
	if info.FileAttributes&(syscall.FILE_ATTRIBUTE_DIRECTORY|syscall.FILE_ATTRIBUTE_REPARSE_POINT) != 0 {
		file.Close()
		return nil, errors.New("The downloaded Setup is not a plain file.")
	}
	final, err := privateFinalPath(handle)
	if err != nil {
		file.Close()
		return nil, err
	}
	expected, err := filepath.Abs(path)
	if err != nil || !strings.EqualFold(filepath.Clean(final), filepath.Clean(expected)) {
		file.Close()
		return nil, fmt.Errorf("The downloaded Setup resolves to %s instead of %s.", final, path)
	}
	return file, nil
}

func privateFinalPath(handle syscall.Handle) (string, error) {
	buffer := make([]uint16, 32768)
	n, _, callErr := privateGetFinalPathByFile.Call(uintptr(handle), uintptr(unsafe.Pointer(&buffer[0])), uintptr(len(buffer)), 0)
	if n == 0 || int(n) >= len(buffer) {
		return "", callErr
	}
	path := syscall.UTF16ToString(buffer[:n])
	switch {
	case strings.HasPrefix(path, `\\?\UNC\`):
		path = `\\` + path[len(`\\?\UNC\`):]
	case strings.HasPrefix(path, `\\?\`):
		path = path[len(`\\?\`):]
	}
	return path, nil
}

// fullInstallCreateProcess starts the program at path with no arguments, as a
// new process of this user (Setup asks for no elevation), and returns its
// process handle. CreateProcess maps the image while the caller still holds
// the file open without write or delete sharing.
func fullInstallCreateProcess(path string) (syscall.Handle, error) {
	application, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return 0, err
	}
	commandLine, err := syscall.UTF16PtrFromString(syscall.EscapeArg(path))
	if err != nil {
		return 0, err
	}
	directory, err := syscall.UTF16PtrFromString(filepath.Dir(path))
	if err != nil {
		return 0, err
	}
	var startup syscall.StartupInfo
	startup.Cb = uint32(unsafe.Sizeof(startup))
	var process syscall.ProcessInformation
	const createNewProcessGroup = 0x00000200
	if err := syscall.CreateProcess(application, commandLine, nil, nil, false, createNewProcessGroup, nil, directory, &startup, &process); err != nil {
		if errors.Is(err, syscall.Errno(740)) { // ERROR_ELEVATION_REQUIRED
			return 0, errors.New("Windows asked for administrator approval to start Setup, which it should not need. Setup was not started.")
		}
		return 0, err
	}
	syscall.CloseHandle(process.Thread)
	return process.Process, nil
}

func fullInstallStartSetup(ctx context.Context, path string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	process, err := fullInstallCreateProcess(path)
	if err != nil {
		return err
	}
	return syscall.CloseHandle(process)
}
