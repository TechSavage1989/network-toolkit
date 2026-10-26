//go:build windows && amd64

package main

import (
	"testing"
	"unsafe"
)

func TestShellExecuteInfoLayout(t *testing.T) {
	var info externalShellExecuteInfo
	if unsafe.Sizeof(info) != 112 || unsafe.Offsetof(info.File) != 24 || unsafe.Offsetof(info.Parameters) != 32 || unsafe.Offsetof(info.Show) != 48 || unsafe.Offsetof(info.Process) != 104 {
		t.Fatal("SHELLEXECUTEINFOW x64 layout does not match the Windows ABI")
	}
}
