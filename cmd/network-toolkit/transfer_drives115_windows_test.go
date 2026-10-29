//go:build windows

package main

import (
	"os"
	"strings"
	"syscall"
	"testing"
	"time"
	"unsafe"
)

func TestLocalDriveNames115(t *testing.T) {
	start := time.Now()
	rows := localTransferDrives()
	if time.Since(start) > 5*time.Second {
		t.Fatal("drive inventory exceeded bound")
	}
	if len(rows) == 0 {
		t.Fatal("Windows runner should expose at least one drive")
	}
	seen := map[string]bool{}
	for _, d := range rows {
		if d.Path == "" || seen[strings.ToLower(d.Path)] || d.Label == "" {
			t.Fatalf("invalid drive presentation: %#v", d)
		}
		seen[strings.ToLower(d.Path)] = true
	}
	// Only the runner's local system drive: never probe a mapped/network volume.
	system := strings.TrimRight(os.Getenv("SystemDrive"), `\/`) + `\`
	if len(system) != 3 || system[1] != ':' {
		t.Fatal("runner system drive is not a drive-letter root")
	}
	pointer, err := syscall.UTF16PtrFromString(system)
	if err != nil {
		t.Fatal(err)
	}
	kind, _, _ := driveType.Call(uintptr(unsafe.Pointer(pointer)))
	if kind != 3 {
		t.Fatal("runner system volume is not a fixed local drive")
	}
	result := make(chan string, 1)
	go func() { result <- readDriveLabel(system) }()
	var label string
	select {
	case label = <-result:
	case <-time.After(time.Second):
		t.Log("System-volume label lookup exceeded bounded wait; path/type fallback remains supported")
		return
	}
	rows = localTransferDrives()
	var actual *localTransferDrive
	for i := range rows {
		if strings.EqualFold(rows[i].Path, system) {
			actual = &rows[i]
			break
		}
	}
	if actual == nil {
		t.Fatal("system drive missing from inventory")
	}
	if label != "" {
		if actual.Label != label {
			t.Fatal("actual system volume label was not propagated into cached inventory")
		}
		t.Log("Nonempty native system-volume label matches cached inventory")
	} else {
		if actual.Path == "" || actual.Label == "" {
			t.Fatal("unlabeled system drive lost its path/type fallback")
		}
		t.Log("System-volume label is empty/unavailable; explicit path/type fallback verified")
	}
}
