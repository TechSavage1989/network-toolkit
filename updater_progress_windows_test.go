package main

import (
	"os"
	"os/exec"
	"testing"
	"time"
)

func TestUpdateProgressWindowLifecycle(t *testing.T) {
	progress := startUpdateProgressWindow("1.17.99")
	native, ok := progress.(*nativeUpdateProgress)
	if !ok {
		t.Fatal("native installation progress window could not be created")
	}
	visible, _, _ := updateProgressUser.NewProc("IsWindowVisible").Call(native.hwnd)
	if visible == 0 {
		t.Fatal("installation progress window is hidden")
	}
	progress.Set("Verifying update files...")
	time.Sleep(100 * time.Millisecond)
	done := make(chan struct{})
	go func() { progress.Close(); close(done) }()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("installation progress window did not close")
	}
	progress.Set("Late status must be ignored")
	progress.Close()
	if os.Getenv("NTK_TEST_PROGRESS_HIDDEN") == "" {
		executable, err := os.Executable()
		if err != nil {
			t.Fatal(err)
		}
		command := exec.Command(executable, "-test.run=^TestUpdateProgressWindowLifecycle$", "-test.v")
		command.Env = append(os.Environ(), "NTK_TEST_PROGRESS_HIDDEN=1")
		configureUpdateHelperProcess(command)
		if output, err := command.CombinedOutput(); err != nil {
			t.Fatalf("hidden-start helper progress: %v\n%s", err, output)
		}
	}
}
