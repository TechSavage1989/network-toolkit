//go:build windows

package main

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"networktoolkit/internal/platform"
)

// Explicitly opt-in: reads only. Never invokes localRunNetworkAction, a helper,
// elevation, or any adapter-changing cmdlet. Logs omit identities, names, paths,
// addresses, environment contents and raw system error messages.
func TestLocalNetworkAdminReadOnlyWindowsDiagnostic(t *testing.T) {
	if os.Getenv("NTK_NETWORK_ADMIN_READONLY_DIAGNOSTIC") != "1" {
		t.Skip("opt-in hosted Windows read-only administrative inventory diagnostic")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	start := time.Now()
	rows, productionErr := localReadAdapterAdmin(ctx)
	t.Logf("production_exact_5s elapsed_ms=%d error=%s rows=%d", time.Since(start).Milliseconds(), localAdminDiagnosticError(productionErr), len(rows))
	native, nativeErr := readLocalIPConfiguration(ctx)
	t.Logf("native_read_only error=%s adapter_count=%d", localAdminDiagnosticError(nativeErr), len(native.Adapters))
	localAdminDiagnosticRows(t, "production", native, rows)
	if localSystemDirectoryError != nil {
		t.Fatal("system-directory resolution failed (path omitted)")
	}
	// Repeat the identical read-only production script with separate streams and
	// an explicit diagnostic budget, to distinguish JSON contamination from a
	// cold-start timeout. This is diagnostic evidence, not a production fallback.
	diagnosticCtx, stop := context.WithTimeout(ctx, 30*time.Second)
	defer stop()
	plan := localPowerShellPlan(localReadAdapterAdminScript, "")
	cmd := exec.CommandContext(diagnosticCtx, filepath.Join(localSystemDirectory, filepath.FromSlash(plan.Program)), plan.Arguments...)
	platform.ConfigureCommand(cmd)
	cmd.Dir = localSystemDirectory
	cmd.Env = localTrustedPowerShellEnvironment()
	var stdout, stderr localOutputBuffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	start = time.Now()
	runErr := cmd.Run()
	parsed, decodeErr := localDecodeAdapterAdmin(stdout.data)
	stderrText := string(stderr.data)
	firstByte := -1
	if len(stdout.data) > 0 {
		firstByte = int(stdout.data[0])
	}
	t.Logf("identical_script_split_streams elapsed_ms=%d run_error=%s context_error=%s stdout_bytes=%d stdout_first_byte=%d stderr_bytes=%d stderr_clixml=%t stderr_module_progress=%t decode_error=%s", time.Since(start).Milliseconds(), localAdminDiagnosticError(runErr), localAdminDiagnosticError(diagnosticCtx.Err()), len(stdout.data), firstByte, len(stderr.data), strings.Contains(stderrText, "CLIXML"), strings.Contains(stderrText, "Preparing modules for first use"), localAdminDiagnosticError(decodeErr))
	var raw []map[string]json.RawMessage
	if json.Unmarshal(stdout.data, &raw) == nil {
		for i, r := range raw {
			if i >= 12 {
				break
			}
			fields := map[string]string{}
			for _, key := range []string{"id", "index", "admin", "oper", "type", "filter", "endpoint", "locked"} {
				var value any
				if json.Unmarshal(r[key], &value) != nil {
					fields[key] = "missing/invalid"
					continue
				}
				switch v := value.(type) {
				case string:
					fields[key] = "string(redacted)"
				case float64:
					fields[key] = string(r[key])
				case bool:
					if v {
						fields[key] = "true"
					} else {
						fields[key] = "false"
					}
				default:
					fields[key] = "other"
				}
			}
			encoded, _ := json.Marshal(fields)
			t.Logf("serialized_row_%d %s", i, encoded)
		}
	}
	localAdminDiagnosticRows(t, "split", native, parsed)
	if productionErr != nil {
		t.Fatalf("exact production administrative inventory failed: %s", localAdminDiagnosticError(productionErr))
	}
	if nativeErr != nil {
		t.Fatalf("native read-only inventory failed: %s", localAdminDiagnosticError(nativeErr))
	}
	if len(native.Adapters) > 0 && len(rows) == 0 {
		t.Fatal("native adapters exist but production administrative inventory is empty")
	}
	merged := localMergeAdapterAdmin(native.Adapters, rows)
	managed := 0
	for _, a := range merged {
		if a.AdminEditable {
			managed++
		}
	}
	if len(native.Adapters) > 0 && managed == 0 {
		t.Fatal("no native or supplemental adapter has verified administrative state")
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
