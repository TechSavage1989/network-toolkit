//go:build windows

package desktop

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"runtime"
	"strings"
	"sync/atomic"
	"syscall"
	"unicode/utf16"
	"unsafe"
)

var (
	wndCommonDialog = syscall.NewLazyDLL("comdlg32.dll")
	wndGetSaveName  = wndCommonDialog.NewProc("GetSaveFileNameW")
	wndDialogError  = wndCommonDialog.NewProc("CommDlgExtendedError")
)

// openFilenameW follows the Windows (not the historical Macintosh) structure.
// Microsoft documents lpEditInfo and lpstrPrompt only for conditional _MAC
// builds; those fields must not be inserted into the Win64 ABI.
type openFilenameW struct {
	Size            uint32
	Owner           uintptr
	Instance        uintptr
	Filter          *uint16
	CustomFilter    *uint16
	MaxCustomFilter uint32
	FilterIndex     uint32
	File            *uint16
	MaxFile         uint32
	FileTitle       *uint16
	MaxFileTitle    uint32
	InitialDir      *uint16
	Title           *uint16
	Flags           uint32
	FileOffset      uint16
	FileExtension   uint16
	DefaultExt      *uint16
	CustomData      uintptr
	Hook            uintptr
	TemplateName    *uint16
	Reserved        uintptr
	ReservedWord    uint32
	FlagsEx         uint32
}

// saveDownloadPath must run on the STA window thread, after the WebView event
// callback has returned through a DownloadStarting deferral.
func saveDownloadPath(owner uintptr, suggested string) (filename string, cancelled bool, err error) {
	name := safeDownloadName(suggested)
	ext := strings.ToLower(filepath.Ext(name))
	label := exportExtensions[ext] + " (*" + ext + ")"
	filter := utf16.Encode([]rune(label + "\x00*" + ext + "\x00\x00"))
	buffer := make([]uint16, 32768)
	initial, err := syscall.UTF16FromString(name)
	if err != nil {
		return "", false, err
	}
	copy(buffer, initial)
	dialog := openFilenameW{
		Size: uint32(unsafe.Sizeof(openFilenameW{})), Owner: owner,
		Filter: &filter[0], FilterIndex: 1, File: &buffer[0], MaxFile: uint32(len(buffer)),
		Title: utf16String("Save Network Toolkit export"), DefaultExt: utf16String(strings.TrimPrefix(ext, ".")),
		Flags: 0x00000002 | 0x00000004 | 0x00000008 | 0x00000800 | 0x00008000 | 0x00080000 | 0x02000000,
	}
	for {
		ok, _, _ := wndGetSaveName.Call(uintptr(unsafe.Pointer(&dialog)))
		runtime.KeepAlive(filter)
		runtime.KeepAlive(buffer)
		if ok == 0 {
			code, _, _ := wndDialogError.Call()
			if code == 0 {
				return "", true, nil
			}
			return "", false, fmt.Errorf("Windows Save As dialog failed (code 0x%04X)", uint32(code))
		}
		filename = syscall.UTF16ToString(buffer)
		if filename == "" || !filepath.IsAbs(filename) {
			return "", false, errors.New("Windows did not return an absolute save path")
		}
		// The common dialog performs normal Win32 path validation. Also reject
		// device namespaces and alternate data streams at this handoff boundary.
		volume := filepath.VolumeName(filename)
		if strings.HasPrefix(filename, `\\.\`) || strings.Contains(filename[len(volume):], ":") {
			return "", false, errors.New("choose a regular file path for the exported report")
		}
		if validationErr := validateDownloadExtension(filename, ext); validationErr != nil {
			message := utf16String(validationErr.Error() + ".")
			title := utf16String("Choose a report filename")
			wndMessageBox.Call(owner, uintptr(unsafe.Pointer(message)), uintptr(unsafe.Pointer(title)), 0x00000030)
			continue
		}
		return filename, false, nil
	}
}

// ErrSaveDialogUnavailable is returned by other platforms; on Windows the
// dialog is always available, but the name keeps call sites uniform.
var ErrSaveDialogUnavailable = errors.New("The Windows Save As dialog is available only in the Windows application")

// ErrSaveDialogBusy reports a second Save As request while one dialog is open.
var ErrSaveDialogBusy = errors.New("A Save As dialog is already open. Finish or cancel it first.")

var saveDialogOpen atomic.Bool

// PromptSaveFile shows the standard Windows Save As dialog for the Text
// Editor. Like the vault dialogs it runs on a locked OS thread with an
// invisible owner window, because the HTTP backend has no HWND of its own.
// The dialog's own overwrite prompt is the user's consent to replace a file;
// the caller passes that consent on as an explicit overwrite flag.
func PromptSaveFile(ctx context.Context, request SaveFileRequest) (SaveFileResponse, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if !saveDialogOpen.CompareAndSwap(false, true) {
		return SaveFileResponse{}, ErrSaveDialogBusy
	}
	defer saveDialogOpen.Store(false)
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	instance, _, callErr := wndGetModuleHandle.Call(0)
	if instance == 0 {
		return SaveFileResponse{}, nativeError("get the Save As dialog module", callErr)
	}
	class, title := utf16String("STATIC"), utf16String("Network Toolkit Save As owner")
	owner, _, callErr := wndCreateWindow.Call(0x00000080, uintptr(unsafe.Pointer(class)), uintptr(unsafe.Pointer(title)), 0x80000000,
		0, 0, 0, 0, 0, 0, instance, 0)
	runtime.KeepAlive(class)
	runtime.KeepAlive(title)
	if owner == 0 {
		return SaveFileResponse{}, nativeError("create the Save As dialog owner", callErr)
	}
	defer wndDestroyWindow.Call(owner)
	filter := utf16.Encode([]rune("All files (*.*)\x00*.*\x00Text files (*.txt)\x00*.txt\x00\x00"))
	buffer := make([]uint16, 32768)
	suggested := strings.Map(func(r rune) rune {
		if r < 0x20 || strings.ContainsRune("\\/:*?\"<>|", r) {
			return -1
		}
		return r
	}, request.Suggested)
	if initial, err := syscall.UTF16FromString(suggested); err == nil && len(initial) < len(buffer) {
		copy(buffer, initial)
	}
	dialogTitle := request.Title
	if strings.TrimSpace(dialogTitle) == "" {
		dialogTitle = "Save As"
	}
	dialog := openFilenameW{
		Size: uint32(unsafe.Sizeof(openFilenameW{})), Owner: owner,
		Filter: &filter[0], FilterIndex: 1, File: &buffer[0], MaxFile: uint32(len(buffer)),
		Title: utf16String(dialogTitle),
		// OFN_OVERWRITEPROMPT | OFN_HIDEREADONLY | OFN_NOCHANGEDIR | OFN_PATHMUSTEXIST |
		// OFN_NOREADONLYRETURN | OFN_EXPLORER | OFN_DONTADDTORECENT
		Flags: 0x00000002 | 0x00000004 | 0x00000008 | 0x00000800 | 0x00008000 | 0x00080000 | 0x02000000,
	}
	var initialDir *uint16
	if dir := strings.TrimSpace(request.Directory); dir != "" && filepath.IsAbs(dir) {
		if value, err := syscall.UTF16PtrFromString(dir); err == nil {
			initialDir = value
			dialog.InitialDir = initialDir
		}
	}
	ok, _, _ := wndGetSaveName.Call(uintptr(unsafe.Pointer(&dialog)))
	runtime.KeepAlive(filter)
	runtime.KeepAlive(buffer)
	runtime.KeepAlive(initialDir)
	if err := ctx.Err(); err != nil {
		return SaveFileResponse{}, err
	}
	if ok == 0 {
		code, _, _ := wndDialogError.Call()
		if code == 0 {
			return SaveFileResponse{Cancelled: true}, nil
		}
		return SaveFileResponse{}, fmt.Errorf("Windows Save As dialog failed (code 0x%04X)", uint32(code))
	}
	filename := syscall.UTF16ToString(buffer)
	if filename == "" || !filepath.IsAbs(filename) {
		return SaveFileResponse{}, errors.New("Windows did not return an absolute save path")
	}
	volume := filepath.VolumeName(filename)
	if strings.HasPrefix(filename, `\\.\`) || strings.Contains(filename[len(volume):], ":") {
		return SaveFileResponse{}, errors.New("Choose a regular file path")
	}
	return SaveFileResponse{Path: filename}, nil
}
