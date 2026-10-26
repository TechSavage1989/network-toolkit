package terminal

// ConPTY provides the same UTF-8/VT stream as remote sessions. Only Windows'
// built-in shells are launched, with the current user's token and policies.
import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"syscall"
	"unsafe"
)

var (
	localKernel           = syscall.NewLazyDLL("kernel32.dll")
	localCreatePTY        = localKernel.NewProc("CreatePseudoConsole")
	localResizePTY        = localKernel.NewProc("ResizePseudoConsole")
	localClosePTY         = localKernel.NewProc("ClosePseudoConsole")
	localInitAttributes   = localKernel.NewProc("InitializeProcThreadAttributeList")
	localUpdateAttribute  = localKernel.NewProc("UpdateProcThreadAttribute")
	localDeleteAttributes = localKernel.NewProc("DeleteProcThreadAttributeList")
	localSystemDirectory  = localKernel.NewProc("GetSystemDirectoryW")
	localCreateJob        = localKernel.NewProc("CreateJobObjectW")
	localSetJob           = localKernel.NewProc("SetInformationJobObject")
	localAssignJob        = localKernel.NewProc("AssignProcessToJobObject")
	localResumeThread     = localKernel.NewProc("ResumeThread")
)

func LocalShellAvailable() bool {
	return localCreatePTY.Find() == nil && localResizePTY.Find() == nil && localClosePTY.Find() == nil
}

type localStartupInfo struct {
	syscall.StartupInfo
	attributes *uintptr
}
type localJobLimits struct {
	processTime, jobTime                 int64
	flags                                uint32
	minimumWorkingSet, maximumWorkingSet uintptr
	activeProcesses                      uint32
	affinity                             uintptr
	priority, scheduling                 uint32
}
type localExtendedJobLimits struct {
	basic                                                      localJobLimits
	ioCounters                                                 [6]uint64
	processMemory, jobMemory, peakProcessMemory, peakJobMemory uintptr
}
type localShellTransport struct {
	mu            sync.Mutex // Serializes resize with the start of PTY shutdown.
	pty           uintptr
	input, output *os.File
	job, process  syscall.Handle
	closeOnce     sync.Once
	closed        chan struct{}
	done          chan error
}

func localShellExecutable(kind string) (string, string, error) {
	buffer := make([]uint16, 32768)
	n, _, callErr := localSystemDirectory.Call(uintptr(unsafe.Pointer(&buffer[0])), uintptr(len(buffer)))
	if n == 0 || n >= uintptr(len(buffer)) {
		return "", "", fmt.Errorf("locate the Windows system directory: %w", callErr)
	}
	system := syscall.UTF16ToString(buffer[:n])
	switch kind {
	case "cmd":
		return filepath.Join(system, "cmd.exe"), " /d", nil
	case "powershell":
		return filepath.Join(system, "WindowsPowerShell", "v1.0", "powershell.exe"), " -NoLogo", nil
	default:
		return "", "", errors.New("unsupported local shell")
	}
}

func openLocalShell(ctx context.Context, req ConnectRequest, cb transportCallbacks) (_ transport, err error) {
	if !LocalShellAvailable() {
		return nil, errors.New("local shells require Windows 10 version 1809 or later, or Windows Server 2019 or later")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	exe, args, err := localShellExecutable(req.Type)
	if err != nil {
		return nil, err
	}
	var inputRead, inputWrite, outputRead, outputWrite syscall.Handle
	var pty uintptr
	var job syscall.Handle
	var process syscall.ProcessInformation
	success := false
	defer func() {
		if success {
			return
		}
		// Closing the output side before ClosePseudoConsole also prevents shutdown
		// deadlocks on Windows versions before 11 24H2 when startup fails.
		for _, h := range []syscall.Handle{inputRead, inputWrite, outputRead, outputWrite, job} {
			if h != 0 {
				_ = syscall.CloseHandle(h)
			}
		}
		if process.Process != 0 {
			_ = syscall.TerminateProcess(process.Process, 1)
			_ = syscall.CloseHandle(process.Process)
		}
		if process.Thread != 0 {
			_ = syscall.CloseHandle(process.Thread)
		}
		if pty != 0 {
			localClosePTY.Call(pty)
		}
	}()
	if err = syscall.CreatePipe(&inputRead, &inputWrite, nil, 0); err != nil {
		return nil, err
	}
	if err = syscall.CreatePipe(&outputRead, &outputWrite, nil, 0); err != nil {
		return nil, err
	}
	size := uintptr(uint32(uint16(req.Cols)) | uint32(uint16(req.Rows))<<16)
	hr, _, _ := localCreatePTY.Call(size, uintptr(inputRead), uintptr(outputWrite), 0, uintptr(unsafe.Pointer(&pty)))
	if int32(hr) < 0 {
		return nil, fmt.Errorf("create local terminal: HRESULT 0x%08x", uint32(hr))
	}
	var bytes uintptr
	localInitAttributes.Call(0, 1, 0, uintptr(unsafe.Pointer(&bytes)))
	if bytes == 0 || bytes > 1024*1024 {
		return nil, errors.New("invalid Windows process attribute size")
	}
	attributes := make([]uintptr, (bytes+unsafe.Sizeof(uintptr(0))-1)/unsafe.Sizeof(uintptr(0)))
	attributePointer := uintptr(unsafe.Pointer(&attributes[0]))
	ok, _, callErr := localInitAttributes.Call(attributePointer, 1, 0, uintptr(unsafe.Pointer(&bytes)))
	if ok == 0 {
		return nil, fmt.Errorf("initialize local terminal attributes: %w", callErr)
	}
	defer func() { localDeleteAttributes.Call(attributePointer); runtime.KeepAlive(attributes) }()
	ok, _, callErr = localUpdateAttribute.Call(attributePointer, 0, 0x00020016, pty, unsafe.Sizeof(pty), 0, 0)
	if ok == 0 {
		return nil, fmt.Errorf("attach local terminal: %w", callErr)
	}
	jobValue, _, callErr := localCreateJob.Call(0, 0)
	if jobValue == 0 {
		return nil, fmt.Errorf("create local session job: %w", callErr)
	}
	job = syscall.Handle(jobValue)
	limits := localExtendedJobLimits{}
	limits.basic.flags = 0x00002000 // JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE
	ok, _, callErr = localSetJob.Call(uintptr(job), 9, uintptr(unsafe.Pointer(&limits)), unsafe.Sizeof(limits))
	if ok == 0 {
		return nil, fmt.Errorf("configure local session cleanup: %w", callErr)
	}
	startup := localStartupInfo{attributes: &attributes[0]}
	startup.Cb = uint32(unsafe.Sizeof(startup))
	application, err := syscall.UTF16PtrFromString(exe)
	if err != nil {
		return nil, err
	}
	command, err := syscall.UTF16PtrFromString(syscall.EscapeArg(exe) + args)
	if err != nil {
		return nil, err
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("locate your home directory: %w", err)
	}
	directory, err := syscall.UTF16PtrFromString(home)
	if err != nil {
		return nil, err
	}
	// Suspend until assigned to our own job so descendants cannot escape cleanup.
	err = syscall.CreateProcess(application, command, nil, nil, false, 0x00080000|0x00000004, nil, directory, &startup.StartupInfo, &process)
	runtime.KeepAlive(attributes)
	if err != nil {
		return nil, fmt.Errorf("start %s: %w", req.Type, err)
	}
	ok, _, callErr = localAssignJob.Call(uintptr(job), uintptr(process.Process))
	if ok == 0 {
		return nil, fmt.Errorf("own local session process: %w", callErr)
	}
	_ = syscall.CloseHandle(inputRead)
	inputRead = 0
	_ = syscall.CloseHandle(outputWrite)
	outputWrite = 0
	resumed, _, callErr := localResumeThread.Call(uintptr(process.Thread))
	if uint32(resumed) == 0xffffffff {
		return nil, fmt.Errorf("resume local shell: %w", callErr)
	}
	_ = syscall.CloseHandle(process.Thread)
	process.Thread = 0
	t := &localShellTransport{pty: pty, input: os.NewFile(uintptr(inputWrite), "local-terminal-input"), output: os.NewFile(uintptr(outputRead), "local-terminal-output"), job: job, process: process.Process, closed: make(chan struct{}), done: make(chan error, 1)}
	success = true
	readDone := make(chan struct{})
	go func() {
		defer close(readDone)
		buffer := make([]byte, maxOutputChunk)
		for {
			n, readErr := t.output.Read(buffer)
			if n > 0 && cb.Output != nil {
				cb.Output(buffer[:n])
			}
			if readErr != nil {
				_ = t.Close()
				return
			}
		}
	}()
	go func() {
		_, waitErr := syscall.WaitForSingleObject(t.process, syscall.INFINITE)
		var code uint32
		codeErr := syscall.GetExitCodeProcess(t.process, &code)
		_ = t.Close()
		<-readDone
		<-t.closed
		_ = syscall.CloseHandle(t.process)
		if waitErr != nil {
			t.done <- waitErr
		} else if codeErr != nil {
			t.done <- codeErr
		} else if code != 0 && ctx.Err() == nil {
			t.done <- fmt.Errorf("%s exited with code %d", req.Type, code)
		} else {
			t.done <- nil
		}
		close(t.done)
	}()
	go func() {
		select {
		case <-ctx.Done():
			_ = t.Close()
		case <-t.closed:
		}
	}()
	return t, nil
}
func (t *localShellTransport) Write(data []byte) (int, error) { return t.input.Write(data) }
func (t *localShellTransport) Resize(cols, rows int) error {
	if err := validDimensions(cols, rows); err != nil {
		return err
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.pty == 0 {
		return io.ErrClosedPipe
	}
	hr, _, _ := localResizePTY.Call(t.pty, uintptr(uint32(uint16(cols))|uint32(uint16(rows))<<16))
	if int32(hr) < 0 {
		return fmt.Errorf("resize local terminal: HRESULT 0x%08x", uint32(hr))
	}
	return nil
}
func (t *localShellTransport) Done() <-chan error { return t.done }
func (t *localShellTransport) Close() error {
	t.closeOnce.Do(func() {
		t.mu.Lock()
		pty := t.pty
		t.pty = 0
		t.mu.Unlock()
		// The output reader keeps draining on another goroutine until shutdown ends.
		// Closing the owned job first also releases blocked writes and child processes.
		_ = syscall.CloseHandle(t.job)
		go func() { _ = t.input.Close(); localClosePTY.Call(pty); _ = t.output.Close(); close(t.closed) }()
	})
	return nil
}
