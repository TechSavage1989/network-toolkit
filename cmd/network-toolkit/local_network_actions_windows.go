//go:build windows

package main

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"
	"unsafe"

	"networktoolkit/internal/platform"
)

func localCurrentDHCPMap(a localIPAdapter) (localDHCPAdapterMap, error) {
	var size uint32
	proc := localIPHelper.NewProc("GetInterfaceInfo")
	code, _, _ := proc.Call(0, uintptr(unsafe.Pointer(&size)))
	if code != 122 || size < 4 || size > 4<<20 {
		return localDHCPAdapterMap{}, fmt.Errorf("Read current DHCP interface identities: Windows error %d", code)
	}
	for attempt := 0; attempt < 3; attempt++ {
		buf := make([]byte, size)
		code, _, _ = proc.Call(uintptr(unsafe.Pointer(&buf[0])), uintptr(unsafe.Pointer(&size)))
		if code == 122 && size >= 4 && size <= 4<<20 {
			continue
		}
		if code != 0 {
			return localDHCPAdapterMap{}, syscall.Errno(code)
		}
		if size > uint32(len(buf)) || size < 4 {
			return localDHCPAdapterMap{}, errors.New("Windows returned an invalid interface table.")
		}
		return localDecodeDHCPMap(buf[:size], a)
	}
	return localDHCPAdapterMap{}, errLocalConfigConflict
}
func localRunNetworkAction(ctx context.Context, r localNetworkRequest, a localIPAdapter) (string, bool, error) {
	if err := validateLocalNetworkRequest(r); err != nil {
		return "", false, err
	}
	if err := ctx.Err(); err != nil {
		return "", false, err
	}
	if r.Action == localDHCPRelease || r.Action == localDHCPRenew {
		mapping, err := localCurrentDHCPMap(a)
		if err != nil {
			return "", false, err
		}
		if err = ctx.Err(); err != nil {
			return "", false, err
		}
		procedure := "IpReleaseAddress"
		if r.Action == localDHCPRenew {
			procedure = "IpRenewAddress"
		}
		// Windows' native DHCP call is synchronous. An already-dispatched call can
		// finish after cancellation; drain it and report unknown/partial, never retry.
		code, _, _ := localIPHelper.NewProc(procedure).Call(uintptr(unsafe.Pointer(&mapping)))
		runtime.KeepAlive(mapping)
		if code != 0 {
			return "", false, fmt.Errorf("%s: %w", procedure, syscall.Errno(code))
		}
		return procedure + " completed for the reviewed adapter GUID.", false, nil
	}
	plan, err := localNetworkCommandPlan(r)
	if err != nil {
		return "", false, err
	}
	if localSystemDirectoryError != nil {
		return "", false, localSystemDirectoryError
	}
	commandCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	cmd := exec.CommandContext(commandCtx, filepath.Join(localSystemDirectory, filepath.FromSlash(plan.Program)), plan.Arguments...)
	platform.ConfigureCommand(cmd)
	cmd.Dir = localSystemDirectory
	cmd.Stdin = strings.NewReader(plan.Input)
	// Do not load user-writable modules under the helper's elevated token.
	cmd.Env = localTrustedPowerShellEnvironment()
	var output localOutputBuffer
	cmd.Stdout, cmd.Stderr = &output, &output
	err = cmd.Run()
	text := string(output.data)
	if decoded, _, decodeErr := localDecodeLegacy(output.data); decodeErr == nil {
		text = decoded
	}
	if commandCtx.Err() != nil {
		err = commandCtx.Err()
	}
	reboot := r.Action == localWinsockReset && cmd.Process != nil
	return strings.TrimSpace(text), reboot, err
}

// ABI assertions only; cross-compilation does not call any Windows API.
var (
	_ [260 - int(unsafe.Sizeof(localDHCPAdapterMap{}))]byte
	_ [int(unsafe.Sizeof(localDHCPAdapterMap{})) - 260]byte
	_ [4 - int(unsafe.Offsetof(localDHCPAdapterMap{}.Name))]byte
	_ [int(unsafe.Offsetof(localDHCPAdapterMap{}.Name)) - 4]byte
)
