package main

import (
	"context"
	"net/http"
	"path/filepath"

	"networktoolkit/internal/editorbuffer"
)

// Promote the bounded legacy state before a growing document reaches that
// renderer's limit. Both undo stacks and the saved root survive the transition.
func (m *editorViewRegistry) promoteDiskDocument(w http.ResponseWriter, r *http.Request, caller string) {
	var req struct {
		Document editorDocumentSnapshot `json:"document"`
		Lease    uint64                 `json:"lease"`
	}
	if err := decodeEditorBody(w, r, editorTransferLimit+16384, &req); err != nil {
		jsonError(w, 400, err.Error())
		return
	}
	s := req.Document
	if s.Disk {
		jsonError(w, 400, "The document is already a disk-backed editor.")
		return
	}
	if err := validateEditorBatch(editorBatch{Documents: []editorDocumentSnapshot{s}, Active: s.UID}); err != nil {
		jsonError(w, 400, err.Error())
		return
	}
	m.mu.Lock()
	previous := m.documents[s.UID]
	conflict := !m.callerLocked(caller) || m.explorerConflictLocked(s.Binding, s.Path)
	if previous != nil {
		if previous.owner != caller || previous.lease != req.Lease || previous.pending != "" || previous.busy != 0 || !editorSameBinding(previous.snapshot.Binding, s.Binding) {
			conflict = true
		} else if previous.buffer != nil && previous.snapshot.ID == s.ID {
			// The client can reconcile a lost successful promotion response.
			lease := previous.lease
			m.mu.Unlock()
			jsonResponse(w, 200, map[string]any{"lease": lease, "disk": true})
			return
		} else if previous.snapshot.Path != s.Path || previous.snapshot.EditRevision > s.EditRevision {
			conflict = true
		}
	} else if req.Lease != 0 {
		conflict = true
	}
	if !conflict && previous != nil {
		previous.busy++
	}
	m.mu.Unlock()
	if conflict {
		jsonError(w, 409, "The document changed ownership during preparation.")
		return
	}
	if previous != nil {
		defer func() {
			m.mu.Lock()
			previous.busy--
			if previous.orphan && previous.busy == 0 {
				previous.owner = ""
				previous.lease++
				previous.orphan = false
				m.recovery[s.UID] = true
			}
			m.mu.Unlock()
		}()
	}
	g := previous
	if g == nil {
		var err error
		g, err = m.makeGrant(caller, s)
		if err != nil {
			jsonError(w, 403, err.Error())
			return
		}
	}
	if s.Binding.Kind == "local" || s.Binding.Kind == "localDraft" && filepath.IsAbs(s.Path) {
		var err error
		s.Path, err = absoluteExplorerPath(s.Path)
		if err != nil {
			jsonError(w, 400, err.Error())
			return
		}
	}
	b, err := editorbuffer.FromHistory(r.Context(), "", s.EditRevision, s.Content, s.Saved, s.Undo, s.Redo)
	if err != nil {
		jsonError(w, 400, err.Error())
		return
	}
	parent := m.app.ctx
	if parent == nil {
		parent = context.Background()
	}
	ctx, cancel := context.WithCancel(parent)
	disk := &editorDiskDocument{ctx: ctx, cancel: cancel, imported: &editorbuffer.Imported{
		Buffer: b, Hash: s.Hash, Format: editorbuffer.Format{Encoding: s.Encoding, BOM: s.BOM, EOL: s.EOL},
	}}
	if s.Binding.Kind == "localDraft" && !filepath.IsAbs(s.Path) && (s.Source == "Desktop" || s.Source == "Desktop import") {
		disk.mode = "upload"
	}
	s.Content, s.Saved, s.Original = "", "", ""
	s.Undo, s.Redo = nil, nil
	s.Disk = true
	s.EditRevision = b.Metadata().Revision
	m.mu.Lock()
	owned := 0
	for _, doc := range m.documents {
		if doc.owner == caller {
			owned++
		}
	}
	if r.Context().Err() != nil || ctx.Err() != nil || !m.callerLocked(caller) || m.documents[s.UID] != previous ||
		(previous != nil && (previous.orphan || previous.owner != caller || previous.lease != req.Lease || previous.pending != "" || previous.busy != 1)) ||
		(previous == nil && (len(m.documents) >= editorDocumentCountLimit || owned >= 12)) ||
		m.explorerConflictLocked(s.Binding, s.Path) {
		m.mu.Unlock()
		disk.close()
		jsonError(w, 409, "The document changed ownership during preparation.")
		return
	}
	g.buffer = disk
	g.snapshot = s
	g.bytes = editorSnapshotBytes(s)
	m.documents[s.UID] = g
	m.registerLoadedLocked(s.UID)
	lease := g.lease
	m.mu.Unlock()
	jsonResponse(w, 200, map[string]any{"lease": lease, "disk": true})
}
