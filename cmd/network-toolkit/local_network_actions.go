package main

import (
	"context"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf16"
)

// Deliberately no program, arguments, adapter alias, script, path, or remote host
// can be supplied by the renderer. One reviewed action uses one helper lifetime.
type localNetworkAction string

const (
	localDHCPRelease    localNetworkAction = "dhcp-release"
	localDHCPRenew      localNetworkAction = "dhcp-renew"
	localAdapterRestart localNetworkAction = "adapter-restart"
	localDNSFlush       localNetworkAction = "dns-flush"
	localDNSRegister    localNetworkAction = "dns-register"
	localWinsockReset   localNetworkAction = "winsock-reset"
)

type localNetworkRequest struct {
	Action    localNetworkAction `json:"action"`
	AdapterID string             `json:"adapterId"`
	Index     uint32             `json:"index"`
	Revision  string             `json:"revision"`
	Scope     string             `json:"scope"`
	Confirmed bool               `json:"confirmed"`
}

func localNetworkScoped(action localNetworkAction) bool {
	return action == localDHCPRelease || action == localDHCPRenew || action == localAdapterRestart
}
func validateLocalNetworkRequest(r localNetworkRequest) error {
	switch r.Action {
	case localDHCPRelease, localDHCPRenew, localAdapterRestart, localDNSFlush, localDNSRegister, localWinsockReset:
	default:
		return errors.New("Unsupported network maintenance action.")
	}
	if !r.Confirmed {
		return errors.New("Review and confirm this network action first.")
	}
	if localNetworkScoped(r.Action) {
		if r.Scope != "adapter" || !validAdapterID(r.AdapterID) || r.Index == 0 || !validLocalRevision(r.Revision) {
			return errors.New("Reload and review the selected adapter before continuing.")
		}
	} else if r.Scope != "computer" || r.AdapterID != "" || r.Index != 0 || r.Revision != "" {
		return errors.New("This action affects this computer, not a selected adapter. Review its computer-wide scope.")
	}
	return nil
}
func planLocalNetworkAction(snapshot localIPSnapshot, r localNetworkRequest) (localIPAdapter, error) {
	if err := validateLocalNetworkRequest(r); err != nil {
		return localIPAdapter{}, err
	}
	if !localNetworkScoped(r.Action) {
		return localIPAdapter{}, nil
	}
	a, found := localFindAdapter(snapshot, r.AdapterID, r.Index)
	if !found || a.Revision != r.Revision {
		return localIPAdapter{}, errLocalConfigConflict
	}
	if !a.Editable {
		return localIPAdapter{}, errors.New("The selected interface does not support this action.")
	}
	if r.Action == localDHCPRelease || r.Action == localDHCPRenew {
		if !a.DHCP {
			return localIPAdapter{}, errors.New("DHCP IPv4 actions are unavailable for a static adapter. Its configuration has not been changed.")
		}
		if !a.AddressEditable {
			return localIPAdapter{}, errors.New("DHCP maintenance is unavailable for this complex or managed IPv4 configuration; its additional addresses and routes are preserved.")
		}
	}
	return a, nil
}

type localNetworkActionOps struct {
	read func(context.Context) (localIPSnapshot, error)
	run  func(context.Context, localNetworkRequest, localIPAdapter) (string, bool, error)
}

func executeLocalNetworkAction(ctx context.Context, r localNetworkRequest, ops localNetworkActionOps) (result localApplyResult) {
	fail := func(err error) { result.Error, result.ErrorCode = err.Error(), localErrorCode(err) }
	if err := validateLocalNetworkRequest(r); err != nil {
		result.Error, result.ErrorCode = err.Error(), "validation"
		return
	}
	result.NetworkAction, result.NetworkScope = string(r.Action), r.Scope
	result.NetworkTarget = "This computer — all adapters/applications"
	if localNetworkScoped(r.Action) {
		result.NetworkTarget = r.AdapterID
	}
	if err := ctx.Err(); err != nil {
		fail(err)
		return
	}
	// This read occurs in the elevated helper too, after any UAC wait. GUID,
	// interface index, revision and DHCP eligibility must still match the review.
	snapshot, err := ops.read(ctx)
	if err != nil {
		fail(err)
		return
	}
	result.Result.IP = &snapshot
	a, err := planLocalNetworkAction(snapshot, r)
	if err != nil {
		fail(err)
		return
	}
	if err = ctx.Err(); err != nil {
		fail(err)
		return
	}
	result.NetworkAction, result.NetworkScope = string(r.Action), r.Scope
	if localNetworkScoped(r.Action) {
		result.NetworkTarget = a.Name + " (" + a.ID + ")"
	} else {
		result.NetworkTarget = "This computer — all adapters/applications"
	}
	output, reboot, err := ops.run(ctx, r, a)
	result.Output, result.RebootRequired = output, reboot
	if err != nil {
		fail(err)
		result.Partial = true
		result.Message = "The action was attempted; its final effect is not confirmed. Do not retry automatically."
	} else {
		result.Message = localNetworkSuccess(r.Action)
	}
	if ctx.Err() != nil {
		fail(ctx.Err())
		result.Partial = true
		result.Message = "Confirmation was interrupted after dispatch. Windows may still have applied the action; review the current state before retrying."
	}
	refreshCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	fresh, refreshErr := ops.read(refreshCtx)
	if refreshErr == nil {
		result.Result.IP = &fresh
	} else {
		result.Message += " Adapter refresh failed; use Reload to inspect the current state."
	}
	return
}
func localNetworkSuccess(action localNetworkAction) string {
	switch action {
	case localDHCPRelease:
		return "The selected adapter's DHCP IPv4 lease was released. Its IPv4 connections are interrupted until a lease is obtained. Select Renew DHCP IPv4 when ready."
	case localDHCPRenew:
		return "Windows completed DHCP IPv4 renewal for the selected adapter. Review its current address; this does not verify Internet connectivity."
	case localAdapterRestart:
		return "Windows completed the selected adapter restart (disable and re-enable). Stored IP/DNS settings were not edited. Connectivity may take time to return."
	case localDNSFlush:
		return "The computer-wide Windows DNS resolver cache was flushed. Application/browser caches are separate."
	case localDNSRegister:
		return "Windows accepted computer-wide dynamic DNS registration. DNS server acceptance is asynchronous and has not been verified."
	case localWinsockReset:
		return "The computer-wide Winsock catalog was reset. Restart Windows manually when ready; no restart has been scheduled."
	}
	return ""
}

// Entire script is constant: JSON stdin is data, never executable source. Module
// lookup is pinned by the runner to Windows' own PowerShell module directory.
const localRestartAdapterScript = `$ErrorActionPreference = 'Stop'
try {
 $request = Microsoft.PowerShell.Utility\ConvertFrom-Json -InputObject ([Console]::In.ReadToEnd())
 $id = [guid]$request.adapterId
 $index = [uint32]$request.index
 if ($index -eq 0) { throw 'Invalid adapter index.' }
 Import-Module (Join-Path $PSHOME 'Modules\NetAdapter\NetAdapter.psd1') -ErrorAction Stop
 $matches = @(NetAdapter\Get-NetAdapter -IncludeHidden -ErrorAction Stop | Where-Object { [guid]$_.InterfaceGuid -eq $id -and [uint32]$_.ifIndex -eq $index })
 if ($matches.Count -ne 1) { throw 'Adapter identity changed. Reload before retrying.' }
 NetAdapter\Restart-NetAdapter -InputObject $matches[0] -Confirm:$false -ErrorAction Stop
 [Console]::Out.WriteLine('Windows completed Restart-NetAdapter for the reviewed interface.')
} catch {
 [Console]::Error.WriteLine($_.Exception.Message)
 exit 1
}`

type localNetworkCommand struct {
	Program   string
	Arguments []string
	Input     string
}

func localNetworkCommandPlan(r localNetworkRequest) (localNetworkCommand, error) {
	if err := validateLocalNetworkRequest(r); err != nil {
		return localNetworkCommand{}, err
	}
	switch r.Action {
	case localDNSFlush:
		return localNetworkCommand{Program: "ipconfig.exe", Arguments: []string{"/flushdns"}}, nil
	case localDNSRegister:
		return localNetworkCommand{Program: "ipconfig.exe", Arguments: []string{"/registerdns"}}, nil
	case localWinsockReset:
		return localNetworkCommand{Program: "netsh.exe", Arguments: []string{"winsock", "reset"}}, nil
	case localAdapterRestart:
		units := utf16.Encode([]rune(localRestartAdapterScript))
		data := make([]byte, len(units)*2)
		for i, u := range units {
			binary.LittleEndian.PutUint16(data[i*2:], u)
		}
		input, err := json.Marshal(struct {
			AdapterID string `json:"adapterId"`
			Index     uint32 `json:"index"`
		}{r.AdapterID, r.Index})
		return localNetworkCommand{Program: "WindowsPowerShell/v1.0/powershell.exe", Arguments: []string{"-NoLogo", "-NoProfile", "-NonInteractive", "-EncodedCommand", base64.StdEncoding.EncodeToString(data)}, Input: string(input)}, err
	default:
		return localNetworkCommand{}, fmt.Errorf("%s has no executable command plan", r.Action)
	}
}

// GetInterfaceInfo returns a device name, not a friendly alias. Both the exact
// GUID-bearing name and index must match: IpRelease/RenewAddress use only Name.
func localDHCPIdentityMatches(name string, index uint32, a localIPAdapter) bool {
	return index != 0 && index == a.Index && validAdapterID(a.ID) && (strings.EqualFold(name, a.ID) || strings.EqualFold(name, `\DEVICE\TCPIP_`+a.ID))
}

type localDHCPAdapterMap struct {
	Index uint32
	Name  [128]uint16
}

func localDecodeDHCPMap(buf []byte, a localIPAdapter) (localDHCPAdapterMap, error) {
	if len(buf) < 4 {
		return localDHCPAdapterMap{}, errors.New("Windows returned an invalid interface table.")
	}
	count := binary.LittleEndian.Uint32(buf[:4])
	// NumAdapters=0 has an undefined dummy entry; never inspect it.
	if count == 0 || uint64(count) > uint64(len(buf)-4)/260 {
		return localDHCPAdapterMap{}, errors.New("No safe matching DHCP interface is available. Reload the adapter list.")
	}
	var selected localDHCPAdapterMap
	matches := 0
	for i := uint32(0); i < count; i++ {
		offset := 4 + int(i)*260
		candidate := localDHCPAdapterMap{Index: binary.LittleEndian.Uint32(buf[offset : offset+4])}
		for j := range candidate.Name {
			candidate.Name[j] = binary.LittleEndian.Uint16(buf[offset+4+j*2 : offset+6+j*2])
		}
		end := 0
		for end < len(candidate.Name) && candidate.Name[end] != 0 {
			end++
		}
		if end == len(candidate.Name) {
			continue
		}
		if localDHCPIdentityMatches(string(utf16.Decode(candidate.Name[:end])), candidate.Index, a) {
			selected = candidate
			matches++
		}
	}
	if matches != 1 {
		return localDHCPAdapterMap{}, errLocalConfigConflict
	}
	return selected, nil
}

// A dispatched native DHCP call has no cancellation API. Keep its manager busy
// until the one-shot helper actually exits, rather than permit overlapping work.
func localNetworkMustDrainHelper(r localToolsRequest, dispatched bool) bool {
	return dispatched && r.Kind == "network" && r.Network != nil && (r.Network.Action == localDHCPRelease || r.Network.Action == localDHCPRenew)
}

func localNetworkActionLabel(action localNetworkAction) string {
	switch action {
	case localDHCPRelease:
		return "Release DHCP IPv4"
	case localDHCPRenew:
		return "Renew DHCP IPv4"
	case localAdapterRestart:
		return "Restart adapter"
	case localDNSFlush:
		return "Flush DNS (computer-wide)"
	case localDNSRegister:
		return "Register DNS (computer-wide)"
	case localWinsockReset:
		return "Reset Winsock (computer-wide)"
	}
	return "network action"
}
