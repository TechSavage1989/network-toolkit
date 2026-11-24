//go:build windows

package main

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"
	"unsafe"
)

func adminRestartSupported() bool { return true }

// adminRestartExecutable is the program the restart starts (tests replace it).
var adminRestartExecutable = updateCurrentExecutable

// adminRestartLaunch starts the program through Windows' administrator
// approval, visibly (SW_SHOWNORMAL), with the given switches. Tests replace it.
var adminRestartLaunch = func(executable string, args []string, directory string) (syscall.Handle, error) {
	if localSystemDirectoryError != nil {
		return 0, localSystemDirectoryError
	}
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	ole := syscall.NewLazyDLL(filepath.Join(localSystemDirectory, "ole32.dll"))
	status, _, _ := ole.NewProc("CoInitializeEx").Call(0, 2|4)
	if int32(status) < 0 {
		return 0, errors.New("Windows could not initialize administrator approval.")
	}
	defer ole.NewProc("CoUninitialize").Call()
	quoted := make([]string, len(args))
	for i, arg := range args {
		quoted[i] = syscall.EscapeArg(arg)
	}
	verb, _ := syscall.UTF16PtrFromString("runas")
	file, err := syscall.UTF16PtrFromString(executable)
	if err != nil {
		return 0, err
	}
	parameters, err := syscall.UTF16PtrFromString(strings.Join(quoted, " "))
	if err != nil {
		return 0, err
	}
	dir, err := syscall.UTF16PtrFromString(directory)
	if err != nil {
		return 0, err
	}
	// SEE_MASK_NOCLOSEPROCESS | SEE_MASK_NOASYNC | SEE_MASK_FLAG_NO_UI
	info := externalShellExecuteInfo{Mask: 0x40 | 0x100 | 0x400, Verb: verb, File: file, Parameters: parameters, Directory: dir, Show: 1}
	info.Size = uint32(unsafe.Sizeof(info))
	ok, _, callErr := syscall.NewLazyDLL(filepath.Join(localSystemDirectory, "shell32.dll")).NewProc("ShellExecuteExW").Call(uintptr(unsafe.Pointer(&info)))
	runtime.KeepAlive(info)
	if ok == 0 || info.Process == 0 {
		if info.Process != 0 {
			syscall.CloseHandle(info.Process)
		}
		if callErr == syscall.Errno(1223) {
			return 0, errAdminRestartDeclined
		}
		if callErr == syscall.Errno(0) {
			return 0, errors.New("Windows did not start Network Toolkit as administrator.")
		}
		return 0, callErr
	}
	return info.Process, nil
}

// launchAdminRestart starts the waiting administrator copy and returns its
// folder once that copy holds this process open and is waiting for it.
func (a *application) launchAdminRestart() (string, error) {
	executable, err := adminRestartExecutable()
	if err != nil {
		return "", err
	}
	if !filepath.IsAbs(executable) {
		return "", errors.New("The application executable path is not absolute.")
	}
	root, err := updateStorageRoot(filepath.Dir(a.dataPath), filepath.Dir(executable))
	if err != nil {
		return "", err
	}
	adminRestartCleanup(root)
	dir, err := os.MkdirTemp(root, adminReopenPrefix)
	if err != nil {
		return "", err
	}
	cwd, err := os.Getwd()
	if err != nil {
		cwd = filepath.Dir(executable)
	}
	process, err := adminRestartLaunch(executable, adminRestartArgs(os.Getpid(), dir), cwd)
	if err != nil {
		_ = os.Remove(dir)
		return "", err
	}
	defer syscall.CloseHandle(process)
	ready := filepath.Join(dir, "ready")
	deadline := time.Now().Add(adminReadyWait)
	for time.Now().Before(deadline) {
		if b, e := updateNamingReadFile(ready, 1024); e == nil && string(b) == "ready" {
			return dir, nil
		}
		if result, _ := syscall.WaitForSingleObject(process, 50); result == 0 {
			_ = os.Remove(dir)
			return "", errors.New("Network Toolkit could not start as administrator.")
		}
	}
	adminRestartCancelDir(dir)
	return "", errors.New("Network Toolkit did not start as administrator in time. Nothing was changed.")
}

// adminRestartCleanup removes folders of earlier restarts that never started.
func adminRestartCleanup(root string) {
	entries, err := os.ReadDir(root)
	if err != nil {
		return
	}
	for _, entry := range entries {
		if !entry.IsDir() || !strings.HasPrefix(entry.Name(), adminReopenPrefix) {
			continue
		}
		info, err := entry.Info()
		if err != nil || time.Since(info.ModTime()) < 2*adminReopenWait {
			continue
		}
		dir := filepath.Join(root, entry.Name())
		for _, name := range []string{"ready", "cancel"} {
			_ = os.Remove(filepath.Join(dir, name))
		}
		_ = os.Remove(dir)
	}
}

// waitAdminParent runs in the new administrator copy before anything else:
// it holds the old process open, says it is ready, and waits for it to exit.
// A "cancel" from the old process, or the time limit, ends this copy quietly.
func waitAdminParent(pid int, timeout time.Duration, dir string) error {
	h, err := syscall.OpenProcess(0x00100000, false, uint32(pid))
	if err != nil {
		return errAdminReopenCancelled // the old copy has gone already: nothing to wait for or reopen
	}
	defer syscall.CloseHandle(h)
	if err = updateWriteExclusive(filepath.Join(dir, "ready"), []byte("ready"), 0600); err != nil {
		return err
	}
	cancel := filepath.Join(dir, "cancel")
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		result, e := syscall.WaitForSingleObject(h, 200)
		if e != nil {
			return e
		}
		if result == 0 {
			return nil
		}
		if _, e := os.Lstat(cancel); e == nil {
			return errAdminReopenCancelled
		}
	}
	return errAdminReopenCancelled
}
