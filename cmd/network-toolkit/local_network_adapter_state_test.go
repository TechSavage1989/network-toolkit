package main

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"reflect"
	"strings"
	"testing"
	"time"
)

func localAdminFixture(state uint32) localAdapterAdminRecord {
	a := localTestAdapter()
	return localAdapterAdminRecord{ID: a.ID, Index: a.Index, Name: a.Name, Admin: state, Oper: 2, Type: 6}
}
func TestLocalNetworkAdminInventoryOnlyMocks(t *testing.T) {
	a := localTestAdapter()
	disabled := localAdminFixture(2)
	reads := 0
	snapshot, err := localReadIPWithAdmin(context.Background(), func(context.Context) (localIPSnapshot, error) {
		return localIPSnapshot{Available: true, Adapters: []localIPAdapter{}}, nil
	}, func(context.Context) ([]localAdapterAdminRecord, error) {
		reads++
		return []localAdapterAdminRecord{disabled}, nil
	})
	if err != nil || reads != 1 || len(snapshot.Adapters) != 1 {
		t.Fatal(snapshot, err, reads)
	}
	got := snapshot.Adapters[0]
	if got.AdminStatus != "disabled" || !got.AdminEditable || got.Editable || got.AddressEditable || got.ID != a.ID || !validLocalRevision(got.Revision) {
		t.Fatal(got)
	}
	if _, err := planLocalNetworkAction(snapshot, localNetworkTestRequest(localAdapterEnable, got)); err != nil {
		t.Fatal(err)
	}
	if _, err := planLocalNetworkAction(snapshot, localNetworkTestRequest(localAdapterRestart, got)); err == nil {
		t.Fatal("incomplete IP adapter became restart/edit eligible")
	}
	snapshot, err = localReadIPWithAdmin(context.Background(), func(context.Context) (localIPSnapshot, error) {
		return localIPSnapshot{Available: true, Adapters: []localIPAdapter{a}}, nil
	}, func(context.Context) ([]localAdapterAdminRecord, error) {
		return []localAdapterAdminRecord{disabled}, errors.New("mock inventory read failure")
	})
	got = snapshot.Adapters[0]
	if err != nil || got.AdminStatus != "unknown" || got.AdminEditable || !got.Editable || !got.AddressEditable || !reflect.DeepEqual(got.Addresses, a.Addresses) || !strings.Contains(snapshot.Note, "could not be read") {
		t.Fatal(snapshot, err)
	}
	enabled := localMergeAdapterAdmin([]localIPAdapter{a}, []localAdapterAdminRecord{localAdminFixture(1)})[0]
	enabled.Status = "Down"
	if enabled.AdminStatus != "enabled" || !enabled.AdminEditable {
		t.Fatal(enabled)
	}
	if _, err := planLocalNetworkAction(localIPSnapshot{Adapters: []localIPAdapter{enabled}}, localNetworkTestRequest(localAdapterDisable, enabled)); err != nil {
		t.Fatal("unplugged enabled adapter cannot be disabled", err)
	}
	if !reflect.DeepEqual(enabled.Addresses, a.Addresses) || !reflect.DeepEqual(enabled.DNSServers, a.DNSServers) {
		t.Fatal("inventory rewrote static settings")
	}
}
func TestLocalNetworkAdminInventoryFailsClosed(t *testing.T) {
	a := localTestAdapter()
	row := localAdminFixture(1)
	for _, tc := range []string{"same-guid", "same-index", "same-pair", "native-guid-conflict", "native-index-conflict", "unknown", "filter", "endpoint", "locked", "loopback", "absent", "zero-guid", "zero-index"} {
		t.Run(tc, func(t *testing.T) {
			records := []localAdapterAdminRecord{row}
			native := []localIPAdapter{a}
			other := row
			switch tc {
			case "same-guid":
				other.Index++
				records = append(records, other)
			case "same-index":
				other.ID = "{11111111-1111-1111-1111-111111111111}"
				records = append(records, other)
			case "same-pair":
				records = append(records, other)
			case "native-guid-conflict":
				native[0].Index++
			case "native-index-conflict":
				native[0].ID = "{11111111-1111-1111-1111-111111111111}"
			case "unknown":
				records[0].Admin = 3
			case "filter":
				records[0].Filter = true
			case "endpoint":
				records[0].Endpoint = true
			case "locked":
				records[0].Locked = true
			case "loopback":
				records[0].Type = 24
			case "absent":
				records[0].Oper = 6
			case "zero-guid":
				records[0].ID = "{00000000-0000-0000-0000-000000000000}"
			case "zero-index":
				records[0].Index = 0
			}
			for _, got := range localMergeAdapterAdmin(native, records) {
				if got.AdminEditable {
					t.Fatal("unsafe inventory became manageable", got)
				}
			}
		})
	}
	for _, data := range []string{"", "{}", "[{", strings.Repeat(" ", 65537)} {
		if _, err := localDecodeAdapterAdmin([]byte(data)); err == nil {
			t.Fatal("invalid inventory decoded", len(data))
		}
	}
	data, _ := json.Marshal([]localAdapterAdminRecord{row})
	rows, err := localDecodeAdapterAdmin(data)
	if err != nil || !reflect.DeepEqual(rows, []localAdapterAdminRecord{row}) {
		t.Fatal(rows, err)
	}
}
func TestLocalNetworkAdminRevisionAndToggleValidation(t *testing.T) {
	a := localMergeAdapterAdmin([]localIPAdapter{localTestAdapter()}, []localAdapterAdminRecord{localAdminFixture(1)})[0]
	r := localNetworkTestRequest(localAdapterDisable, a)
	if err := validateLocalToolsRequest(localToolsRequest{Kind: "network", Network: &r}); err != nil {
		t.Fatal(err)
	}
	b := a
	b.Status = "Down"
	if localAdapterRevision(b) != a.Revision {
		t.Fatal("link status invalidated review")
	}
	b.AdminStatus = "disabled"
	if localAdapterRevision(b) == a.Revision {
		t.Fatal("admin state missing from revision")
	}
	b.Revision = localAdapterRevision(b)
	if _, err := planLocalNetworkAction(localIPSnapshot{Adapters: []localIPAdapter{b}}, r); !errors.Is(err, errLocalConfigConflict) {
		t.Fatal("stale admin state accepted", err)
	}
	for _, tc := range []string{"same-guid", "same-index", "unknown", "not-manageable", "wrong-state"} {
		rows := []localIPAdapter{a}
		other := a
		switch tc {
		case "same-guid":
			other.Index++
			rows = append(rows, other)
		case "same-index":
			other.ID = "{11111111-1111-1111-1111-111111111111}"
			rows = append(rows, other)
		case "unknown":
			rows[0].AdminStatus = "unknown"
		case "not-manageable":
			rows[0].AdminEditable = false
		case "wrong-state":
			rows[0].AdminStatus = "disabled"
		}
		if _, err := planLocalNetworkAction(localIPSnapshot{Adapters: rows}, r); err == nil {
			t.Fatal("unsafe toggle accepted", tc)
		}
	}
	for _, action := range []localNetworkAction{localAdapterEnable, localAdapterDisable} {
		request := localNetworkTestRequest(action, a)
		if validateLocalNetworkRequest(request) != nil {
			t.Fatal(request)
		}
		request.Scope = "computer"
		if validateLocalNetworkRequest(request) == nil {
			t.Fatal("global toggle accepted")
		}
	}
}
func TestLocalNetworkToggleFixedCommandConstruction(t *testing.T) {
	a := localTestAdapter()
	for _, action := range []localNetworkAction{localAdapterEnable, localAdapterDisable} {
		request := localNetworkTestRequest(action, a)
		plan, err := localNetworkCommandPlan(request)
		expected := localPowerShellPlan(localToggleAdapterScript, plan.Input)
		if err != nil || !reflect.DeepEqual(plan, expected) {
			t.Fatal(plan, err)
		}
		var input map[string]any
		if json.Unmarshal([]byte(plan.Input), &input) != nil || len(input) != 3 || input["action"] != string(action) || input["adapterId"] != a.ID || input["index"] != float64(a.Index) {
			t.Fatal(plan.Input)
		}
		if strings.Contains(localToggleAdapterScript, a.ID) {
			t.Fatal("target interpolated into script")
		}
	}
	for _, script := range []string{localReadAdapterAdminScript, localToggleAdapterScript} {
		for _, unsafe := range []string{"-Name ", "-CimSession", "Invoke-Expression", "Restart-Computer", "Set-NetIP", "Set-Dns"} {
			if strings.Contains(script, unsafe) {
				t.Fatal(unsafe)
			}
		}
		if !strings.Contains(script, "Modules\\NetAdapter\\NetAdapter.psd1") || !strings.Contains(script, "Get-NetAdapter -IncludeHidden") {
			t.Fatal("untrusted module/incomplete inventory")
		}
	}
	for _, guard := range []string{"$matches.Count -ne 1", "$indexes.Count -ne 1", "[uint32]$matches[0].InterfaceIndex -ne $index", "InterfaceAdminStatus -ne 2", "InterfaceAdminStatus -ne 1", "switch -Exact", "Enable-NetAdapter -InputObject $adapter", "Disable-NetAdapter -InputObject $adapter"} {
		if !strings.Contains(localToggleAdapterScript, guard) {
			t.Fatal("missing command-time guard", guard)
		}
	}
}
func TestLocalNetworkToggleExecutionOnlyMocks(t *testing.T) {
	for _, action := range []localNetworkAction{localAdapterEnable, localAdapterDisable} {
		for _, scenario := range []string{"success", "new-index", "stale", "wrong-state", "unknown", "unconfirmed", "refresh-failure", "dispatch-failure", "cancel-after", "duplicate-guid"} {
			t.Run(string(action)+"/"+scenario, func(t *testing.T) {
				from, to := "disabled", "enabled"
				if action == localAdapterDisable {
					from, to = to, from
				}
				a := localTestAdapter()
				a.AdminStatus = from
				a.AdminEditable = true
				a.Revision = localAdapterRevision(a)
				request := localNetworkTestRequest(action, a)
				calls, reads := 0, 0
				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()
				result := executeLocalNetworkAction(ctx, request, localNetworkActionOps{
					read: func(context.Context) (localIPSnapshot, error) {
						reads++
						current := a
						if reads == 1 {
							if scenario == "stale" {
								current.Revision = strings.Repeat("f", 64)
							}
							if scenario == "wrong-state" {
								current.AdminStatus = to
							}
							if scenario == "unknown" {
								current.AdminStatus = "unknown"
								current.AdminEditable = false
							}
						} else {
							current.AdminStatus = to
							if scenario == "unconfirmed" {
								current.AdminStatus = from
							}
							if scenario == "new-index" {
								current.Index++
							}
							if scenario == "refresh-failure" {
								return localIPSnapshot{}, errors.New("mock refresh failure")
							}
						}
						rows := []localIPAdapter{current}
						if scenario == "duplicate-guid" && reads > 1 {
							rows = append(rows, current)
						}
						return localIPSnapshot{Available: true, Adapters: rows}, nil
					},
					run: func(context.Context, localNetworkRequest, localIPAdapter) (string, bool, error) {
						calls++
						if scenario == "cancel-after" {
							cancel()
						}
						if scenario == "dispatch-failure" {
							return "mock only", false, errors.New("mock denied")
						}
						return "mock only", false, nil
					},
				})
				if scenario == "stale" || scenario == "wrong-state" || scenario == "unknown" {
					if calls != 0 || result.Error == "" || result.Partial {
						t.Fatal(result, calls)
					}
				} else if scenario == "success" || scenario == "new-index" {
					if calls != 1 || result.Error != "" || result.Partial || result.RebootRequired || !strings.Contains(result.Message, "Stored IP/DNS settings were not edited") {
						t.Fatal(result, calls)
					}
				} else if calls != 1 || result.Error == "" || !result.Partial || strings.Contains(result.Message, "is administratively") {
					t.Fatal(result, calls)
				}
				if !reflect.DeepEqual(a.Addresses, localTestAdapter().Addresses) || !reflect.DeepEqual(a.DNSServers, localTestAdapter().DNSServers) {
					t.Fatal("static settings mutated")
				}
			})
		}
	}
}

func TestLocalNetworkAdminReadStreamsAndBounds(t *testing.T) {
	data, _ := json.Marshal([]localAdapterAdminRecord{localAdminFixture(1)})
	progress := []byte("#< CLIXML\n<Objs><Obj>Preparing modules for first use.</Obj></Objs>")
	if _, err := localDecodeAdapterAdmin(append(append([]byte{}, progress...), data...)); err == nil {
		t.Fatal("negative control accepted mixed stream JSON")
	}
	rows, err := localCollectAdapterAdmin(func(stdout, stderr io.Writer) error {
		if stdout == stderr {
			t.Fatal("stdout/stderr share a JSON buffer")
		}
		_, _ = stderr.Write(progress)
		_, _ = stdout.Write(data)
		_, _ = stderr.Write([]byte("other nonfatal diagnostics"))
		return nil
	})
	if err != nil || len(rows) != 1 || rows[0].Admin != 1 {
		t.Fatal(rows, err)
	}
	for _, failure := range []error{context.Canceled, context.DeadlineExceeded, errors.New("mock command failure")} {
		rows, err := localCollectAdapterAdmin(func(stdout, stderr io.Writer) error { _, _ = stdout.Write(data); return failure })
		if !errors.Is(err, failure) || len(rows) != 0 {
			t.Fatal("partial output bypassed command failure", rows, err)
		}
	}
	if localAdapterAdminReadTimeout != 20*time.Second || localIPInventoryTimeout != 25*time.Second || localAdapterAdminReadTimeout >= localIPInventoryTimeout || localIPInventoryTimeout >= 30*time.Second {
		t.Fatal("inventory deadlines no longer fit the existing HTTP write bound")
	}
	if !strings.Contains(localReadAdapterAdminScript, "$ProgressPreference = 'SilentlyContinue'") {
		t.Fatal("read-only discovery progress suppression missing")
	}
	for _, kind := range []string{"ip", "network", "hosts"} {
		want := 25 * time.Second
		if kind == "hosts" {
			want = 8 * time.Second
		}
		if localToolsReadTimeout(kind) != want {
			t.Fatal(kind, localToolsReadTimeout(kind))
		}
	}
}
func TestLocalNetworkAdminHTTPReadDeadlineOnlyMocks(t *testing.T) {
	a := testApplication(t)
	adapter := localTestAdapter()
	adapter.AdminStatus = "enabled"
	adapter.AdminEditable = true
	adapter.Revision = localAdapterRevision(adapter)
	reads := 0
	a.localTools = newLocalToolsManagerWithBackend(a.ctx, localToolsBackend{available: true, readIP: func(ctx context.Context) (localIPSnapshot, error) {
		reads++
		deadline, ok := ctx.Deadline()
		remaining := time.Until(deadline)
		if !ok || remaining < 24*time.Second || remaining > 25*time.Second {
			t.Errorf("IP inventory context has %s remaining", remaining)
		}
		return localIPSnapshot{Available: true, Adapters: []localIPAdapter{adapter}}, nil
	}, apply: func(context.Context, localToolsRequest) localApplyResult {
		return localApplyResult{Message: "mock only"}
	}})
	if response := requestApp(a, "GET", "/api/local-tools/ip", "", true); response.Code != 200 {
		t.Fatal(response.Code)
	}
	request := localNetworkTestRequest(localAdapterDisable, adapter)
	body, _ := json.Marshal(request)
	if response := requestApp(a, "POST", "/api/local-tools/network", string(body), true); response.Code != 202 {
		t.Fatal(response.Code)
	}
	if reads != 2 {
		t.Fatal(reads)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := a.localTools.Close(ctx); err != nil {
		t.Fatal(err)
	}
}
func TestLocalNetworkToggleRefreshDeadlineOnlyMocks(t *testing.T) {
	for _, cancelled := range []bool{false, true} {
		ctx, cancel := context.WithCancel(context.Background())
		adapter := localTestAdapter()
		adapter.AdminStatus = "enabled"
		adapter.AdminEditable = true
		adapter.Revision = localAdapterRevision(adapter)
		reads := 0
		result := executeLocalNetworkAction(ctx, localNetworkTestRequest(localAdapterDisable, adapter), localNetworkActionOps{
			read: func(ctx context.Context) (localIPSnapshot, error) {
				reads++
				if reads > 1 {
					deadline, ok := ctx.Deadline()
					remaining := time.Until(deadline)
					want := 25 * time.Second
					if cancelled {
						want = 3 * time.Second
					}
					if !ok || remaining < want-time.Second || remaining > want {
						t.Errorf("refresh context has %s remaining, want %s", remaining, want)
					}
					adapter.AdminStatus = "disabled"
				}
				return localIPSnapshot{Adapters: []localIPAdapter{adapter}}, nil
			},
			run: func(context.Context, localNetworkRequest, localIPAdapter) (string, bool, error) {
				if cancelled {
					cancel()
				}
				return "mock only", false, nil
			},
		})
		cancel()
		if reads != 2 || (!cancelled && result.Error != "") || (cancelled && !result.Partial) {
			t.Fatal(result, reads)
		}
	}
}
