package main

import (
	"context"
	"fmt"
	"strings"
	"time"
)

type localIPApplyOps struct {
	read func(context.Context) (localIPSnapshot, error)
	run  func(context.Context, localNetshStep) (string, error)
}

// executeLocalIP is separately injectable so tests can exercise cancellation,
// adapter replacement and partial failure without changing a real interface.
func executeLocalIP(ctx context.Context, request localIPRequest, ops localIPApplyOps) (result localApplyResult) {
	if err := ctx.Err(); err != nil {
		return localApplyResult{Error: err.Error(), ErrorCode: localErrorCode(err)}
	}
	if err := validateLocalIPRequest(request); err != nil {
		return localApplyResult{Error: err.Error(), ErrorCode: "validation"}
	}
	snapshot, err := ops.read(ctx)
	if err != nil {
		return localApplyResult{Error: err.Error(), ErrorCode: localErrorCode(err)}
	}
	result.Result.IP = &snapshot
	adapter, steps, err := planLocalIP(snapshot, request)
	if err != nil {
		result.Error, result.ErrorCode = err.Error(), localErrorCode(err)
		return
	}
	if len(steps) == 0 {
		result.Message = "No changes were needed."
		return
	}
	// Refresh even after cancellation or a failed Windows command. A command
	// can return an error after making part of a change, so never claim that the
	// previous configuration was retained merely because its exit code failed.
	attempted := false
	defer func() {
		refreshCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		fresh, refreshErr := ops.read(refreshCtx)
		if refreshErr == nil {
			result.Result.IP = &fresh
		} else if result.Error != "" {
			result.Error += " The current configuration could not be refreshed: " + refreshErr.Error()
		} else {
			result.Error = "Windows completed the command, but the current configuration could not be refreshed: " + refreshErr.Error()
			result.ErrorCode = "failed"
		}
		if result.Error != "" {
			result.Partial = attempted
			return
		}
		if !localIPMatchesRequest(fresh, adapter.Index, request) {
			result.Error = "Windows completed the commands, but the refreshed settings do not match every requested change. Review the current configuration before retrying."
			result.ErrorCode, result.Partial = "failed", attempted
		}
	}()
	expectedRevision := request.Revision
	for i, step := range steps {
		if err = ctx.Err(); err != nil {
			result.Error, result.ErrorCode = err.Error(), localErrorCode(err)
			return
		}
		// Interface indexes can be reused after a device is removed. Check the
		// permanent adapter GUID and index before every individual mutation.
		fresh, readErr := ops.read(ctx)
		if readErr != nil {
			result.Error, result.ErrorCode = readErr.Error(), localErrorCode(readErr)
			return
		}
		current, found := localFindAdapter(fresh, adapter.ID, adapter.Index)
		if !found || current.Revision != expectedRevision {
			result.Error, result.ErrorCode = errLocalConfigConflict.Error(), "conflict"
			return
		}
		if err = ctx.Err(); err != nil {
			result.Error, result.ErrorCode = err.Error(), localErrorCode(err)
			return
		}
		attempted = true
		output, runErr := ops.run(ctx, step)
		if runErr != nil {
			result.Error = fmt.Sprintf("%s could not be completed: %s", step.Label, runErr)
			if text := strings.TrimSpace(output); text != "" {
				result.Error += ". " + text
			}
			result.ErrorCode = localErrorCode(runErr)
			if ctx.Err() != nil {
				result.ErrorCode = "cancelled"
			}
			return
		}
		if i+1 < len(steps) {
			// Record the state produced by this command. If another editor
			// changes it before the next command, stop rather than continuing
			// a multi-step apply against a configuration we have not reviewed.
			after, readErr := ops.read(ctx)
			if readErr != nil {
				result.Error, result.ErrorCode = readErr.Error(), localErrorCode(readErr)
				return
			}
			updated, sameAdapter := localFindAdapter(after, adapter.ID, adapter.Index)
			if !sameAdapter {
				result.Error, result.ErrorCode = errLocalConfigConflict.Error(), "conflict"
				return
			}
			expectedRevision = updated.Revision
		}
	}
	result.Message = "The IPv4 configuration was applied and refreshed."
	if request.Mode == "dhcp" {
		result.Message += " A DHCP lease can take a few moments to appear."
	}
	return
}

func localFindAdapter(snapshot localIPSnapshot, id string, index uint32) (localIPAdapter, bool) {
	for _, a := range snapshot.Adapters {
		if strings.EqualFold(a.ID, id) && a.Index == index {
			return a, true
		}
	}
	return localIPAdapter{}, false
}

func localIPMatchesRequest(snapshot localIPSnapshot, index uint32, r localIPRequest) bool {
	a, found := localFindAdapter(snapshot, r.AdapterID, index)
	if !found || a.DHCP != (r.Mode == "dhcp") {
		return false
	}
	address, prefix, gateway := primaryLocalIP(a)
	if r.Mode == "static" && (r.Address != address || r.PrefixLength != prefix || r.Gateway != gateway) {
		return false
	}
	if r.DNSMode == "automatic" {
		return a.DNSAutomatic
	}
	if len(r.DNSServers) == 0 && len(a.effectiveDNS) != 0 {
		return false
	}
	return (!a.DNSAutomatic || len(r.DNSServers) == 0) && equalLocalStrings(a.DNSServers, r.DNSServers)
}

// The runner still drains all process output after this buffer fills, avoiding
// either unbounded allocation or a child blocked on a full stdout pipe.
type localOutputBuffer struct{ data []byte }

func (b *localOutputBuffer) Write(p []byte) (int, error) {
	const limit = 64 << 10
	n := len(p)
	if len(b.data) < limit {
		keep := limit - len(b.data)
		if keep > n {
			keep = n
		}
		b.data = append(b.data, p[:keep]...)
	}
	return n, nil
}
