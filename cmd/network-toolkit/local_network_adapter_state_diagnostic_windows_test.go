//go:build windows

package main

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"
)

// Explicitly opt-in: reads only. Never invokes localRunNetworkAction, a helper,
// elevation, or any adapter-changing cmdlet. Logs omit identities, names, paths,
// addresses, environment contents and raw system error messages.
func TestLocalNetworkAdminReadOnlyWindowsDiagnostic(t *testing.T) {
	if os.Getenv("NTK_NETWORK_ADMIN_READONLY_DIAGNOSTIC") != "1" {
		t.Skip("opt-in hosted Windows read-only administrative inventory diagnostic")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	start := time.Now()
	rows, productionErr := localReadAdapterAdmin(ctx)
	t.Logf("production_cold budget_ms=%d elapsed_ms=%d error=%s rows=%d", localAdapterAdminReadTimeout.Milliseconds(), time.Since(start).Milliseconds(), localAdminDiagnosticError(productionErr), len(rows))
	native, nativeErr := readLocalIPConfiguration(ctx)
	t.Logf("native_read_only error=%s adapter_count=%d", localAdminDiagnosticError(nativeErr), len(native.Adapters))
	localAdminDiagnosticRows(t, "production_cold", native, rows)
	if productionErr != nil || nativeErr != nil {
		t.Fatalf("actual read-only production inventory failed: admin=%s native=%s", localAdminDiagnosticError(productionErr), localAdminDiagnosticError(nativeErr))
	}
	if len(native.Adapters) == 0 || len(rows) == 0 {
		t.Fatal("hosted acceptance requires an actual existing adapter; none was observed")
	}
	// Exercise the authenticated production GET handler, including its own
	// deadline, with the real Windows read backend. No HTTP POST is sent.
	a := testApplication(t)
	a.localTools = newLocalToolsManagerWithBackend(a.ctx, localToolsBackend{available: true, readIP: readLocalIP, apply: func(context.Context, localToolsRequest) localApplyResult {
		panic("read-only diagnostic must never dispatch a mutation")
	}})
	start = time.Now()
	response := requestApp(a, "GET", "/api/local-tools/ip", "", true)
	var snapshot localIPSnapshot
	decodeErr := json.Unmarshal(response.Body.Bytes(), &snapshot)
	t.Logf("production_http_repeat status=%d elapsed_ms=%d decode_error=%s adapter_count=%d", response.Code, time.Since(start).Milliseconds(), localAdminDiagnosticError(decodeErr), len(snapshot.Adapters))
	if response.Code != 200 || decodeErr != nil {
		t.Fatal("actual read-only HTTP inventory failed (response redacted)")
	}
	var selected localIPAdapter
	managed := 0
	for _, adapter := range snapshot.Adapters {
		if adapter.AdminEditable && (adapter.AdminStatus == "enabled" || adapter.AdminStatus == "disabled") {
			managed++
			selected = adapter
		}
	}
	t.Logf("production_http_repeat managed_count=%d", managed)
	if managed == 0 {
		t.Fatal("actual HTTP inventory has no verified manageable administrative state")
	}
	// prepare performs only a fresh inventory read and typed validation. Do not
	// call start, apply, any action runner or any elevated helper.
	action := localAdapterDisable
	if selected.AdminStatus == "disabled" {
		action = localAdapterEnable
	}
	request := localNetworkTestRequest(action, selected)
	prepareCtx, stop := context.WithTimeout(ctx, localToolsReadTimeout("network"))
	defer stop()
	start = time.Now()
	prepareErr := a.localTools.prepare(prepareCtx, localToolsRequest{Kind: "network", Network: &request})
	t.Logf("production_prepare_read_only elapsed_ms=%d error=%s", time.Since(start).Milliseconds(), localAdminDiagnosticError(prepareErr))
	if prepareErr != nil {
		t.Fatalf("actual read-only network preflight failed: %s", localAdminDiagnosticError(prepareErr))
	}
}

func localAdminDiagnosticRows(t *testing.T, label string, native localIPSnapshot, rows []localAdapterAdminRecord) {
	t.Helper()
	counts := map[string]int{}
	exact := 0
	for _, r := range rows {
		counts[localAdminStatus(r.Admin)]++
		if validAdapterID(r.ID) {
			counts["validGuid"]++
		}
		for _, a := range native.Adapters {
			if strings.EqualFold(a.ID, r.ID) && a.Index == r.Index {
				exact++
			}
		}
	}
	merged := localMergeAdapterAdmin(native.Adapters, rows)
	managed := 0
	for _, a := range merged {
		if a.AdminEditable {
			managed++
		}
	}
	data, _ := json.Marshal(counts)
	t.Logf("%s inventory_states=%s exact_identity_pairs=%d merged_count=%d managed_count=%d", label, data, exact, len(merged), managed)
}
func localAdminDiagnosticError(err error) string {
	if err == nil {
		return "none"
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return "deadline-exceeded"
	}
	if errors.Is(err, context.Canceled) {
		return "cancelled"
	}
	var syntax *json.SyntaxError
	if errors.As(err, &syntax) {
		return "json-syntax"
	}
	var typed *json.UnmarshalTypeError
	if errors.As(err, &typed) {
		return "json-type"
	}
	var exit *exec.ExitError
	if errors.As(err, &exit) {
		return "process-exit"
	}
	return "other-error(redacted)"
}
