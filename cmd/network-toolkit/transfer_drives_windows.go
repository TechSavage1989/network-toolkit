//go:build windows

package main

import (
	"runtime"
	"sort"
	"strings"
	"syscall"
	"time"
	"unsafe"
)

var driveKernel = syscall.NewLazyDLL("kernel32.dll")
var driveLogical = driveKernel.NewProc("GetLogicalDriveStringsW")
var driveType = driveKernel.NewProc("GetDriveTypeW")
var driveFirstVolume = driveKernel.NewProc("FindFirstVolumeW")
var driveNextVolume = driveKernel.NewProc("FindNextVolumeW")
var driveCloseVolume = driveKernel.NewProc("FindVolumeClose")
var driveVolumeInformation = driveKernel.NewProc("GetVolumeInformationW")
var driveGetThreadErrorMode = driveKernel.NewProc("GetThreadErrorMode")
var driveSetThreadErrorMode = driveKernel.NewProc("SetThreadErrorMode")
var driveLabels = newDriveLabelCache(readDriveLabel)

func readDriveLabel(path string) string {
	p, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return ""
	}
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	var previous uint32
	current, _, _ := driveGetThreadErrorMode.Call()
	if ok, _, _ := driveSetThreadErrorMode.Call(current|1, uintptr(unsafe.Pointer(&previous))); ok == 0 {
		return ""
	}
	defer driveSetThreadErrorMode.Call(uintptr(previous), 0)
	label := make([]uint16, 261)
	if ok, _, _ := driveVolumeInformation.Call(uintptr(unsafe.Pointer(p)), uintptr(unsafe.Pointer(&label[0])), uintptr(len(label)), 0, 0, 0, 0, 0); ok != 0 {
		return syscall.UTF16ToString(label)
	}
	return ""
}

var driveVolumePaths = driveKernel.NewProc("GetVolumePathNamesForVolumeNameW")

func splitDriveMultiString(b []uint16) []string {
	var out []string
	for len(b) > 0 && b[0] != 0 {
		n := 0
		for n < len(b) && b[n] != 0 {
			n++
		}
		out = append(out, syscall.UTF16ToString(b[:n]))
		if n == len(b) {
			break
		}
		b = b[n+1:]
	}
	return out
}
func localTransferDrives() []localTransferDrive {
	// GetLogicalDriveStrings includes current-user mapped network drives. Volume
	// mount-point enumeration also includes mounted folders without drive letters.
	paths := map[string]string{}
	add := func(p string) {
		if len(p) > 0 && len(p) <= 32767 {
			paths[strings.ToLower(p)] = p
		}
	}
	b := make([]uint16, 32768)
	if n, _, _ := driveLogical.Call(uintptr(len(b)), uintptr(unsafe.Pointer(&b[0]))); n > 0 && n < uintptr(len(b)) {
		for _, p := range splitDriveMultiString(b[:n+1]) {
			add(p)
		}
	}
	volume := make([]uint16, 1024)
	handle, _, _ := driveFirstVolume.Call(uintptr(unsafe.Pointer(&volume[0])), uintptr(len(volume)))
	if handle != ^uintptr(0) {
		defer driveCloseVolume.Call(handle)
		for count := 0; count < 1024; count++ {
			var needed uint32
			if ok, _, _ := driveVolumePaths.Call(uintptr(unsafe.Pointer(&volume[0])), uintptr(unsafe.Pointer(&b[0])), uintptr(len(b)), uintptr(unsafe.Pointer(&needed))); ok != 0 {
				for _, p := range splitDriveMultiString(b) {
					add(p)
				}
			}
			if ok, _, _ := driveNextVolume.Call(handle, uintptr(unsafe.Pointer(&volume[0])), uintptr(len(volume))); ok == 0 {
				break
			}
		}
	}
	out := make([]localTransferDrive, 0, len(paths))
	for _, p := range paths {
		v, e := syscall.UTF16PtrFromString(p)
		if e != nil {
			continue
		}
		kind, _, _ := driveType.Call(uintptr(unsafe.Pointer(v)))
		name, label := "other", "Mounted volume"
		switch kind {
		case 2:
			name, label = "removable", "Removable drive"
		case 3:
			name, label = "local", "Local drive"
		case 4:
			name, label = "network", "Network drive"
		case 5:
			name, label = "optical", "Optical drive"
		case 6:
			name, label = "memory", "Memory drive"
		}
		out = append(out, localTransferDrive{p, label, name})
	}
	driveLabels.enrich(out, 75*time.Millisecond)
	sort.Slice(out, func(i, j int) bool { return strings.ToLower(out[i].Path) < strings.ToLower(out[j].Path) })
	return out
}
