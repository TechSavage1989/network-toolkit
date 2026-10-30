package main

import (
	"context"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
	"time"
	"unicode/utf16"
)

func localNetworkTestRequest(action localNetworkAction, a localIPAdapter) localNetworkRequest {
	r := localNetworkRequest{Action: action, Scope: "computer", Confirmed: true}
	if localNetworkScoped(action) {
		r.Scope = "adapter"
		r.AdapterID = a.ID
		r.Index = a.Index
		r.Revision = a.Revision
	}
	return r
}
func TestLocalNetworkStrictAllowlistAndScope(t *testing.T) {
	a := localTestAdapter()
	for _, action := range []localNetworkAction{localDHCPRelease, localDHCPRenew, localAdapterRestart, localDNSFlush, localDNSRegister, localWinsockReset} {
		r := localNetworkTestRequest(action, a)
		if err := validateLocalNetworkRequest(r); err != nil {
			t.Fatal(action, err)
		}
		if err := validateLocalToolsRequest(localToolsRequest{Kind: "network", Network: &r}); err != nil {
			t.Fatal(err)
		}
		if err := validateLocalHelperRequest(localHelperRequest{Version: 1, Request: localToolsRequest{Kind: "network", Network: &r}, WiFi: &localWiFiHelperRequest{}}); err == nil {
			t.Fatal("mixed helper request accepted")
		}
		r.Confirmed = false
		if validateLocalNetworkRequest(r) == nil {
			t.Fatal("missing review accepted")
		}
	}
	for _, action := range []localNetworkAction{"", "reset", "tcpip-reset", "restart-computer", "dns-flush & whoami"} {
		r := localNetworkTestRequest(action, a)
		if validateLocalNetworkRequest(r) == nil {
			t.Fatal("unknown action accepted", action)
		}
	}
	for _, mutate := range []func(*localNetworkRequest){func(r *localNetworkRequest) { r.AdapterID = "*" }, func(r *localNetworkRequest) { r.Index = 0 }, func(r *localNetworkRequest) { r.Revision = "" }, func(r *localNetworkRequest) { r.Scope = "computer" }, func(r *localNetworkRequest) { r.AdapterID = "{01234567-89ab-cdef-0123-456789abcdef};whoami" }} {
		r := localNetworkTestRequest(localAdapterRestart, a)
		mutate(&r)
		if validateLocalNetworkRequest(r) == nil {
			t.Fatal("invalid target accepted", r)
		}
	}
	r := localNetworkTestRequest(localDNSFlush, a)
	r.AdapterID = a.ID
	if validateLocalNetworkRequest(r) == nil {
		t.Fatal("global action accepted misleading adapter")
	}
}
func TestLocalNetworkEligibilityAndRevalidation(t *testing.T) {
	a := localTestAdapter()
	r := localNetworkTestRequest(localDHCPRelease, a)
	if _, err := planLocalNetworkAction(localIPSnapshot{Adapters: []localIPAdapter{a}}, r); err == nil {
		t.Fatal("static adapter accepted")
	}
	a.DHCP = true
	a.Revision = localAdapterRevision(a)
	r = localNetworkTestRequest(localDHCPRenew, a)
	if _, err := planLocalNetworkAction(localIPSnapshot{Adapters: []localIPAdapter{a}}, r); err != nil {
		t.Fatal(err)
	}
	a.AddressEditable = false
	if _, err := planLocalNetworkAction(localIPSnapshot{Adapters: []localIPAdapter{a}}, r); err == nil {
		t.Fatal("complex config accepted")
	}
	a.AddressEditable = true
	a.Index++
	if _, err := planLocalNetworkAction(localIPSnapshot{Adapters: []localIPAdapter{a}}, r); !errors.Is(err, errLocalConfigConflict) {
		t.Fatal("reused interface accepted", err)
	}
}
func TestLocalNetworkCommandConstruction(t *testing.T) {
	a := localTestAdapter()
	for action, want := range map[localNetworkAction][]string{localDNSFlush: {"/flushdns"}, localDNSRegister: {"/registerdns"}, localWinsockReset: {"winsock", "reset"}} {
		plan, err := localNetworkCommandPlan(localNetworkTestRequest(action, a))
		if err != nil || !reflect.DeepEqual(plan.Arguments, want) || plan.Input != "" {
			t.Fatal(plan, err)
		}
		if strings.Contains(strings.Join(plan.Arguments, " "), "int ip") {
			t.Fatal("destructive TCP/IP reset")
		}
	}
	r := localNetworkTestRequest(localAdapterRestart, a)
	plan, err := localNetworkCommandPlan(r)
	if err != nil {
		t.Fatal(err)
	}
	if plan.Program != "WindowsPowerShell/v1.0/powershell.exe" || !reflect.DeepEqual(plan.Arguments[:4], []string{"-NoLogo", "-NoProfile", "-NonInteractive", "-EncodedCommand"}) {
		t.Fatal(plan)
	}
	raw, err := base64.StdEncoding.DecodeString(plan.Arguments[4])
	if err != nil {
		t.Fatal(err)
	}
	units := make([]uint16, len(raw)/2)
	for i := range units {
		units[i] = binary.LittleEndian.Uint16(raw[i*2:])
	}
	script := string(utf16.Decode(units))
	if script != localRestartAdapterScript || strings.Contains(script, a.ID) || strings.Contains(script, "-Name ") || strings.Contains(script, "-CimSession") || strings.Contains(script, "Invoke-Expression") {
		t.Fatal("unsafe dynamic script", script)
	}
	for _, required := range []string{"[guid]$_.InterfaceGuid -eq $id -and [uint32]$_.ifIndex -eq $index", "$matches.Count -ne 1", "Restart-NetAdapter -InputObject $matches[0]", "Modules\\NetAdapter\\NetAdapter.psd1"} {
		if !strings.Contains(script, required) {
			t.Fatal("missing guard", required)
		}
	}
	var input map[string]any
	if json.Unmarshal([]byte(plan.Input), &input) != nil || len(input) != 2 || input["adapterId"] != a.ID {
		t.Fatal(plan.Input)
	}
	if _, err := localNetworkCommandPlan(localNetworkTestRequest(localDHCPRelease, a)); err == nil {
		t.Fatal("DHCP acquired executable fallback")
	}
}
func localDHCPTestBuffer(a localIPAdapter, name string, count int) []byte {
	b := make([]byte, 4+count*260)
	binary.LittleEndian.PutUint32(b, uint32(count))
	for i := 0; i < count; i++ {
		offset := 4 + i*260
		binary.LittleEndian.PutUint32(b[offset:], a.Index)
		for j, u := range utf16.Encode([]rune(name)) {
			if j >= 128 {
				break
			}
			binary.LittleEndian.PutUint16(b[offset+4+j*2:], u)
		}
	}
	return b
}
func TestLocalNetworkNativeDHCPIdentityTable(t *testing.T) {
	a := localTestAdapter()
	name := `\DEVICE\TCPIP_` + a.ID
	bare, err := localDecodeDHCPMap(localDHCPTestBuffer(a, a.ID, 1), a)
	if err != nil || bare.Index != a.Index {
		t.Fatal("bare GUID rejected", err)
	}
	mapping, err := localDecodeDHCPMap(localDHCPTestBuffer(a, name, 1), a)
	if err != nil || mapping.Index != a.Index {
		t.Fatal(mapping, err)
	}
	for _, name := range []string{"Ethernet", "*", a.ID + "*", `\DEVICE\TCPIP_{ffffffff-ffff-ffff-ffff-ffffffffffff}`, `\DEVICE\TCPIP_` + a.ID + "*"} {
		if _, err := localDecodeDHCPMap(localDHCPTestBuffer(a, name, 1), a); err == nil {
			t.Fatal("wrong name accepted", name)
		}
	}
	for _, b := range [][]byte{nil, make([]byte, 264), localDHCPTestBuffer(a, name, 2), {255, 255, 255, 255}, localDHCPTestBuffer(a, strings.Repeat("x", 128), 1)} {
		if _, err := localDecodeDHCPMap(b, a); err == nil {
			t.Fatal("malformed/ambiguous interface table accepted")
		}
	}
	b := localDHCPTestBuffer(a, name, 1)
	a.Index++
	if _, err := localDecodeDHCPMap(b, a); err == nil {
		t.Fatal("wrong index accepted")
	}
}
func TestLocalNetworkExecutionOnlyMocks(t *testing.T) {
	a := localTestAdapter()
	snapshot := localIPSnapshot{Available: true, Adapters: []localIPAdapter{a}}
	r := localNetworkTestRequest(localAdapterRestart, a)
	for _, scenario := range []string{"success", "failure", "cancel-before", "cancel-after", "stale", "refresh-failure"} {
		t.Run(scenario, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			calls, reads := 0, 0
			if scenario == "cancel-before" {
				cancel()
			}
			result := executeLocalNetworkAction(ctx, r, localNetworkActionOps{read: func(context.Context) (localIPSnapshot, error) {
				reads++
				s := snapshot
				if scenario == "stale" {
					s.Adapters = append([]localIPAdapter{}, snapshot.Adapters...)
					s.Adapters[0].Revision = strings.Repeat("f", 64)
				}
				if scenario == "refresh-failure" && reads > 1 {
					return localIPSnapshot{}, errors.New("fixture refresh failure")
				}
				return s, nil
			}, run: func(context.Context, localNetworkRequest, localIPAdapter) (string, bool, error) {
				calls++
				if scenario == "cancel-after" {
					cancel()
				}
				if scenario == "failure" {
					return "fixture native failure", false, errors.New("fixture error")
				}
				return "fixture completed", false, nil
			}})
			if scenario == "stale" || scenario == "cancel-before" {
				if calls != 0 || result.Error == "" || result.Partial {
					t.Fatal(result, calls)
				}
				return
			}
			if calls != 1 {
				t.Fatal(calls)
			}
			if scenario == "failure" || scenario == "cancel-after" {
				if result.Error == "" || !result.Partial {
					t.Fatal(result)
				}
			} else if result.Error != "" {
				t.Fatal(result)
			}
			if scenario == "refresh-failure" && !strings.Contains(result.Message, "refresh failed") {
				t.Fatal(result)
			}
			if !reflect.DeepEqual(snapshot.Adapters[0], a) {
				t.Fatal("static config changed")
			}
		})
	}
	r = localNetworkTestRequest(localWinsockReset, a)
	result := executeLocalNetworkAction(context.Background(), r, localNetworkActionOps{read: func(context.Context) (localIPSnapshot, error) { return snapshot, nil }, run: func(context.Context, localNetworkRequest, localIPAdapter) (string, bool, error) {
		return "fixture only", true, nil
	}})
	if !result.RebootRequired || !strings.Contains(result.Message, "manually") {
		t.Fatal(result)
	}
}
func TestLocalNetworkAPIStrictMockDispatch(t *testing.T) {
	a := testApplication(t)
	adapter := localTestAdapter()
	var calls atomic.Int32
	a.localTools = newLocalToolsManagerWithBackend(a.ctx, localToolsBackend{available: true, readIP: func(context.Context) (localIPSnapshot, error) {
		return localIPSnapshot{Available: true, Adapters: []localIPAdapter{adapter}}, nil
	}, apply: func(context.Context, localToolsRequest) localApplyResult {
		calls.Add(1)
		return localApplyResult{Message: "mock only"}
	}})
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		_ = a.localTools.Close(ctx)
	}()
	request := localNetworkTestRequest(localDNSFlush, adapter)
	body, _ := json.Marshal(request)
	if response := requestApp(a, "POST", "/api/local-tools/network", string(body), false); response.Code != 401 {
		t.Fatal(response.Code)
	}
	for _, body := range []string{`{"action":"dns-flush","scope":"computer","confirmed":true,"command":"whoami"}`, `{"action":"dns-flush","scope":"computer","confirmed":true,"Action":"dns-flush"}`, `{"action":"dns-flush","scope":"computer","confirmed":false}`, `{"action":"tcpip-reset","scope":"computer","confirmed":true}`} {
		if response := requestApp(a, "POST", "/api/local-tools/network", body, true); response.Code != 400 {
			t.Fatal(response.Code, response.Body.String())
		}
	}
	if calls.Load() != 0 {
		t.Fatal("invalid request dispatched")
	}
	response := requestApp(a, "POST", "/api/local-tools/network", string(body), true)
	if response.Code != 202 {
		t.Fatal(response.Code, response.Body.String())
	}
}

func TestLocalNetworkCancelledNativeKeepsBusyUntilDrained(t *testing.T) {
	a := localTestAdapter()
	r := localNetworkTestRequest(localDHCPRenew, a)
	request := localToolsRequest{Kind: "network", Network: &r}
	if !localNetworkMustDrainHelper(request, true) || localNetworkMustDrainHelper(request, false) || localNetworkMustDrainHelper(localToolsRequest{Kind: "ip"}, true) {
		t.Fatal("incorrect drain fence")
	}
	started, drain := make(chan struct{}), make(chan struct{})
	m := newLocalToolsManagerWithBackend(context.Background(), localToolsBackend{available: true, apply: func(ctx context.Context, _ localToolsRequest) localApplyResult {
		close(started)
		<-ctx.Done()
		<-drain
		return localApplyResult{Error: "cancelled after native dispatch", Partial: true}
	}})
	op, err := m.start(request)
	if err != nil {
		t.Fatal(err)
	}
	<-started
	closeCtx, cancel := context.WithTimeout(context.Background(), time.Millisecond)
	defer cancel()
	if err := m.Close(closeCtx); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatal(err)
	}
	if _, err := m.start(request); !errors.Is(err, errLocalConfigBusy) {
		t.Fatal("new mutation bypassed pending native call", err)
	}
	current, _ := m.operation(op.ID)
	if current.Status != "running" {
		t.Fatal(current)
	}
	close(drain)
	doneCtx, stop := context.WithTimeout(context.Background(), time.Second)
	defer stop()
	if err := m.Close(doneCtx); err != nil {
		t.Fatal(err)
	}
	current, _ = m.operation(op.ID)
	if current.Status != "failed" || !current.Partial {
		t.Fatal(current)
	}
}
