package main

// Terminal file access is a read-only use of the SSH transport already owned by
// this window. It does not borrow editor approvals, create connections or accept
// credentials/local destinations from the child renderer.
import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"networktoolkit/internal/diag"
	"networktoolkit/internal/model"
)

const terminalExplorerOwnerPrefix = "terminal:"

var errTerminalSFTPLease = errors.New("The terminal owner or SSH connection changed. Refresh its files before continuing.")

type terminalSFTPContextKey struct{}
type terminalSFTPOperation func(context.Context, remoteBrowserRequest) (model.Result, error)

func explorerRequestOwner(r *http.Request) string {
	if id := terminalRequestView(r); id != "" {
		return terminalExplorerOwnerPrefix + id
	}
	return editorRequestView(r)
}

func (a *application) terminalSFTPOwnerLocked(view, session string, revision uint64) bool {
	owner := a.views.ownerLocked(session)
	if owner.ViewID != view || owner.Revision != revision || !owner.Ready {
		return false
	}
	if view == "" {
		return a.views.homePresent && !a.views.homeClosing
	}
	v := a.views.views[view]
	return v != nil && v.info.Workspace && v.includesSession(session) && (v.info.State == "detached" || v.info.State == "return_requested")
}

// The returned grant is private to this read/export operation; it is never
// registered as an editor grant or made available for delegation. Check both
// ownership and transport identity before, during and after every channel use.
func (a *application) terminalSFTPGrant(view string, b editorBinding) (*editorDocumentGrant, error) {
	if a.views == nil || a.terminals == nil || b.Kind != "terminal" || b.SessionID == "" || len(b.SessionID) > 128 || b.ConnectionID != "" {
		return nil, errors.New("Choose an SSH terminal owned by this window.")
	}
	a.views.mu.Lock()
	defer a.views.mu.Unlock()
	if !a.terminalSFTPOwnerLocked(view, b.SessionID, b.Revision) {
		return nil, errTerminalSFTPLease
	}
	client, lifetime, err := a.terminals.SFTPClient(b.SessionID)
	if err != nil {
		return nil, errors.New("The original SSH terminal is unavailable.")
	}
	grant := &editorDocumentGrant{snapshot: editorDocumentSnapshot{Binding: b}}
	grant.valid = func() bool {
		a.views.mu.Lock()
		defer a.views.mu.Unlock()
		if !a.terminalSFTPOwnerLocked(view, b.SessionID, b.Revision) || lifetime.Err() != nil {
			return false
		}
		current, currentLifetime, err := a.terminals.SFTPClient(b.SessionID)
		return err == nil && current == client && currentLifetime == lifetime
	}
	grant.run = func(ctx context.Context, req remoteBrowserRequest) (model.Result, error) {
		switch req.Action {
		case "probe", "list", "stat", "read-text":
		case "download":
			if !req.SafeExport || req.ExportSize < 0 || req.ExportSize > explorerExportMaxBytes {
				return model.Result{}, errors.New("A bounded private export is required.")
			}
		default:
			return model.Result{}, errors.New("This terminal file access is read-only.")
		}
		if !grant.valid() {
			return model.Result{}, errTerminalSFTPLease
		}
		operation, cancel := context.WithCancel(ctx)
		defer cancel()
		stop := context.AfterFunc(lifetime, cancel)
		defer stop()
		done := make(chan struct{})
		defer close(done)
		go func() {
			tick := time.NewTicker(50 * time.Millisecond)
			defer tick.Stop()
			for {
				select {
				case <-done:
					return
				case <-operation.Done():
					return
				case <-tick.C:
					if !grant.valid() {
						cancel()
						return
					}
				}
			}
		}()
		result, err := diag.RunSFTPOnClient(operation, client, req.request(), nil)
		if !grant.valid() {
			return model.Result{}, errTerminalSFTPLease
		}
		return result, err
	}
	return grant, nil
}

func (a *application) handleTerminalSFTPRead(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodError(w)
		return
	}
	var req struct {
		Protocol  string `json:"protocol"`
		SessionID string `json:"sessionId"`
		Action    string `json:"action"`
		Path      string `json:"path,omitempty"`
	}
	if decodeEditorBody(w, r, 16<<10, &req) != nil || (req.Protocol != "ssh" && req.Protocol != "sftp") || len(req.Path) > 4096 || strings.ContainsAny(req.Path, "\x00\r\n") {
		jsonError(w, 400, "Invalid terminal file request.")
		return
	}
	switch req.Action {
	case "probe", "list", "stat", "preview", "read-text":
	default:
		jsonError(w, 403, "This terminal file access is read-only.")
		return
	}
	revision := uint64(0)
	if value := r.Header.Get("X-Terminal-Revision"); value != "" {
		var err error
		revision, err = strconv.ParseUint(value, 10, 64)
		if err != nil {
			jsonError(w, 400, "Invalid terminal ownership revision.")
			return
		}
	}
	grant, err := a.terminalSFTPGrant(terminalRequestView(r), editorBinding{Kind: "terminal", SessionID: req.SessionID, Revision: revision})
	if err != nil {
		jsonError(w, 409, "The SSH terminal is not available to this window.")
		return
	}
	// Reuse the existing response/preview/text formatting without its credential
	// resolver or session lookup. Even a reconnect during the request cannot move
	// this operation to a replacement transport.
	previewSize := int64(-1)
	run := terminalSFTPOperation(func(ctx context.Context, operation remoteBrowserRequest) (model.Result, error) {
		if operation.Action == "download" {
			if req.Action != "preview" || previewSize < 0 || previewSize > 131072 {
				return model.Result{}, errors.New("Preview is limited to 128 KiB.")
			}
			operation.SafeExport, operation.ExportSize = true, previewSize
		}
		finishActivity := a.activity.begin(ctx, "sftp")
		defer finishActivity()
		result, err := grant.run(ctx, operation)
		if err == nil && req.Action == "preview" && operation.Action == "stat" {
			data := remoteExportData(result)
			raw, _ := json.Marshal(data["sizeBytes"])
			if data["kind"] != "file" || json.Unmarshal(raw, &previewSize) != nil {
				previewSize = -1
			}
		}
		return result, err
	})
	raw, _ := json.Marshal(req)
	r = r.WithContext(context.WithValue(r.Context(), terminalSFTPContextKey{}, run))
	r.Body = io.NopCloser(bytes.NewReader(raw))
	a.handleRemoteBrowser(w, r)
}
