package main

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"
)

// Administrative Up/Down is independent of cable/media operational status.
// https://learn.microsoft.com/en-us/previous-versions/windows/desktop/legacy/hh968170(v=vs.85)
type localAdapterAdminRecord struct {
	ID          string `json:"id"`
	Index       uint32 `json:"index"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Admin       uint32 `json:"admin"`
	Oper        uint32 `json:"oper"`
	Type        uint32 `json:"type"`
	Filter      bool   `json:"filter"`
	Endpoint    bool   `json:"endpoint"`
	Locked      bool   `json:"locked"`
}

func localDecodeAdapterAdmin(data []byte) ([]localAdapterAdminRecord, error) {
	var rows []localAdapterAdminRecord
	if len(data) == 0 || len(data) > 64<<10 {
		return nil, errors.New("Adapter administrative inventory is unavailable or too large.")
	}
	if err := json.Unmarshal(data, &rows); err != nil {
		return nil, err
	}
	if len(rows) > 4096 {
		return nil, errors.New("Adapter administrative inventory exceeds its limit.")
	}
	return rows, nil
}
func localAdminStatus(value uint32) string {
	if value == 1 {
		return "enabled"
	}
	if value == 2 {
		return "disabled"
	}
	return "unknown"
}
func localMergeAdapterAdmin(adapters []localIPAdapter, records []localAdapterAdminRecord) []localIPAdapter {
	out := append([]localIPAdapter(nil), adapters...)
	nativeIDs, nativeIndexes := map[string]int{}, map[uint32]int{}
	recordIDs, recordIndexes := map[string]int{}, map[uint32]int{}
	for i := range out {
		out[i].AdminStatus = "unknown"
		out[i].AdminEditable = false
		nativeIDs[strings.ToLower(out[i].ID)]++
		nativeIndexes[out[i].Index]++
	}
	for _, r := range records {
		recordIDs[strings.ToLower(r.ID)]++
		recordIndexes[r.Index]++
	}
	for _, r := range records {
		id := strings.ToLower(r.ID)
		if !validAdapterID(r.ID) || id == "{00000000-0000-0000-0000-000000000000}" || r.Index == 0 || recordIDs[id] != 1 || recordIndexes[r.Index] != 1 || nativeIDs[id] > 1 || nativeIndexes[r.Index] > 1 {
			continue
		}
		slot := -1
		conflict := false
		for i, a := range out {
			if strings.EqualFold(a.ID, r.ID) {
				if a.Index != r.Index {
					conflict = true
				}
				slot = i
			}
			if a.Index == r.Index && !strings.EqualFold(a.ID, r.ID) {
				conflict = true
			}
		}
		if conflict {
			continue
		}
		state := localAdminStatus(r.Admin)
		manageable := state != "unknown" && !r.Filter && !r.Endpoint && !r.Locked && r.Type != 0 && r.Type != 24 && r.Oper != 6
		if slot < 0 {
			if !manageable {
				continue
			}
			a := localIPAdapter{ID: r.ID, Index: r.Index, Name: r.Name, Description: r.Description, Addresses: []localIPAddress{}, Gateways: []string{}, DNSServers: []string{}, Note: "IPv4 configuration is unavailable for this adapter. Administrative Enable/Disable does not edit stored IP or DNS settings."}
			a.Type = map[uint32]string{6: "Ethernet", 71: "Wi-Fi", 23: "PPP", 131: "Tunnel"}[r.Type]
			if a.Type == "" {
				a.Type = "Other"
			}
			a.Status = map[uint32]string{1: "Up", 2: "Down", 3: "Testing", 4: "Unknown", 5: "Dormant", 6: "Not present", 7: "Lower layer down"}[r.Oper]
			if a.Status == "" {
				a.Status = "Unknown"
			}
			out = append(out, a)
			slot = len(out) - 1
		}
		out[slot].AdminStatus, out[slot].AdminEditable = state, manageable
		if state == "disabled" {
			out[slot].Editable = false
			out[slot].AddressEditable = false
			out[slot].Note = "Enable this adapter and reload before editing IPv4 settings. Stored IP and DNS settings are not changed by Enable/Disable."
		}
	}
	return finalizeLocalAdapters(out)
}

// Get-NetAdapter includes disabled/hidden interfaces that may be absent from
// GetAdaptersAddresses. This fixed read-only script never requests elevation.
const localReadAdapterAdminScript = `$ErrorActionPreference = 'Stop'
try {
 [Console]::OutputEncoding = [System.Text.UTF8Encoding]::new($false)
 Import-Module (Join-Path $PSHOME 'Modules\NetAdapter\NetAdapter.psd1') -ErrorAction Stop
 $rows = @(NetAdapter\Get-NetAdapter -IncludeHidden -ErrorAction Stop | ForEach-Object {
  [pscustomobject]@{id=([guid]$_.InterfaceGuid).ToString('B');index=[uint32]$_.InterfaceIndex;name=[string]$_.Name;description=[string]$_.InterfaceDescription;admin=[uint32]$_.InterfaceAdminStatus;oper=[uint32]$_.InterfaceOperationalStatus;type=[uint32]$_.InterfaceType;filter=[bool]$_.IMFilter;endpoint=[bool]$_.EndPointInterface;locked=[bool]$_.AdminLocked}
 })
 [Console]::Out.Write((Microsoft.PowerShell.Utility\ConvertTo-Json -InputObject $rows -Compress -Depth 3))
} catch { [Console]::Error.WriteLine($_.Exception.Message); exit 1 }
`

// Only these two fixed action names can reach their corresponding cmdlet.
const localToggleAdapterScript = `$ErrorActionPreference = 'Stop'
try {
 $request = Microsoft.PowerShell.Utility\ConvertFrom-Json -InputObject ([Console]::In.ReadToEnd())
 $id = [guid]$request.adapterId
 $index = [uint32]$request.index
 if ($index -eq 0) { throw 'Invalid adapter index.' }
 Import-Module (Join-Path $PSHOME 'Modules\NetAdapter\NetAdapter.psd1') -ErrorAction Stop
 $all = @(NetAdapter\Get-NetAdapter -IncludeHidden -ErrorAction Stop)
 $matches = @($all | Where-Object { [guid]$_.InterfaceGuid -eq $id })
 $indexes = @($all | Where-Object { [uint32]$_.InterfaceIndex -eq $index })
 if ($matches.Count -ne 1 -or $indexes.Count -ne 1 -or [uint32]$matches[0].InterfaceIndex -ne $index) { throw 'Adapter identity changed. Reload before retrying.' }
 $adapter = $matches[0]
 if ($adapter.IMFilter -or $adapter.EndPointInterface -or $adapter.AdminLocked -or [uint32]$adapter.InterfaceType -eq 0 -or [uint32]$adapter.InterfaceType -eq 24 -or [uint32]$adapter.InterfaceOperationalStatus -eq 6) { throw 'Adapter cannot be managed safely.' }
 switch -Exact ($request.action) {
  'adapter-enable' {
   if ([uint32]$adapter.InterfaceAdminStatus -ne 2) { throw 'Adapter administrative state changed. Reload before enabling.' }
   NetAdapter\Enable-NetAdapter -InputObject $adapter -Confirm:$false -ErrorAction Stop
  }
  'adapter-disable' {
   if ([uint32]$adapter.InterfaceAdminStatus -ne 1) { throw 'Adapter administrative state changed. Reload before disabling.' }
   NetAdapter\Disable-NetAdapter -InputObject $adapter -Confirm:$false -ErrorAction Stop
  }
  default { throw 'Unsupported adapter state action.' }
 }
 [Console]::Out.WriteLine('Windows completed the reviewed adapter administrative action.')
} catch { [Console]::Error.WriteLine($_.Exception.Message); exit 1 }
`

// Injected readers make empty-native inventory and read-failure behavior testable
// without launching PowerShell or touching a real adapter.
func localReadIPWithAdmin(ctx context.Context, readIP func(context.Context) (localIPSnapshot, error), readAdmin func(context.Context) ([]localAdapterAdminRecord, error)) (localIPSnapshot, error) {
	snapshot, err := readIP(ctx)
	if err != nil {
		return snapshot, err
	}
	// Always supplement, including GetAdaptersAddresses' NO_DATA/empty result.
	records, adminErr := readAdmin(ctx)
	if adminErr != nil {
		records = nil
		snapshot.Note = "Enable/Disable availability could not be read. Existing IP configuration remains available: " + adminErr.Error()
	}
	snapshot.Adapters = localMergeAdapterAdmin(snapshot.Adapters, records)
	stampLocalIPSnapshot(&snapshot, time.Now())
	return snapshot, ctx.Err()
}
