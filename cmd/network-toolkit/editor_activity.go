package main

import (
	"encoding/json"
	"errors"
	"net/http"
)

// Renderer-only documents (such as browser file handles) have no file grant.
// Their bounded activity metadata cannot create a grant or carry document text.
type editorLoadedDocument struct {
	UID    string `json:"uid"`
	Lease  uint64 `json:"lease"`
	Loaded bool   `json:"loaded"`
}

func (a *application) editorAskSaveBeforeClose() bool {
	a.stateMu.Lock()
	data := append([]byte(nil), a.state...)
	a.stateMu.Unlock()
	var state struct {
		EditorPreferences struct {
			AskSaveBeforeClose *bool `json:"askSaveBeforeClose"`
		} `json:"editorPreferences"`
	}
	if json.Unmarshal(data, &state) != nil || state.EditorPreferences.AskSaveBeforeClose == nil {
		return true
	}
	return *state.EditorPreferences.AskSaveBeforeClose
}

func (m *editorViewRegistry) updateLoaded(caller string, rows []editorLoadedDocument) error {
	if len(rows) > editorDocumentCountLimit {
		return errors.New("Too many editor activity entries.")
	}
	next := make(map[string]editorLoadedDocument, len(rows))
	for _, row := range rows {
		if !editorUID.MatchString(row.UID) {
			return errors.New("Invalid editor document identity.")
		}
		if _, found := next[row.UID]; found {
			return errors.New("Duplicate editor activity identity.")
		}
		next[row.UID] = row
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.callerLocked(caller) {
		return errEditorLease
	}
	for _, row := range rows {
		grant := m.documents[row.UID]
		if grant != nil {
			if grant.owner != caller || grant.lease != row.Lease {
				return errEditorLease
			}
		} else if row.Lease != 0 {
			return errEditorLease
		}
	}
	if m.loaded == nil {
		m.loaded = make(map[string]map[string]editorLoadedDocument)
	}
	m.loaded[caller] = next
	return nil
}

// On first registration, retire lease-zero renderer metadata everywhere. It
// must never resurrect as a phantom document when the real grant is released.
func (m *editorViewRegistry) registerLoadedLocked(uid string) {
	for _, rows := range m.loaded {
		delete(rows, uid)
	}
}

func editorSnapshotLoaded(d editorDocumentSnapshot) bool {
	return !d.Untitled || d.Content != "" || d.Dirty || d.EditRevision > 1 || len(d.Undo) > 0 || len(d.Redo) > 0
}

func (m *editorViewRegistry) loadedCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	uids := make(map[string]bool)
	for uid, grant := range m.documents {
		loaded := editorSnapshotLoaded(grant.snapshot)
		if row, ok := m.loaded[grant.owner][uid]; ok && row.Lease == grant.lease {
			loaded = row.Loaded
		}
		if loaded {
			uids[uid] = true
		}
	}
	for owner, rows := range m.loaded {
		if owner == "" && !m.homePresent {
			continue
		}
		if owner != "" {
			v := m.views[owner]
			if v == nil || v.info.State == "closed" || v.info.State == "returned" {
				continue
			}
		}
		for uid, row := range rows {
			if row.Loaded && row.Lease == 0 && m.documents[uid] == nil {
				uids[uid] = true
			}
		}
	}
	return len(uids)
}

func (m *editorViewRegistry) handleLoaded(w http.ResponseWriter, r *http.Request, caller string) {
	var body struct {
		Documents []editorLoadedDocument `json:"documents"`
	}
	if err := decodeEditorBody(w, r, 16384, &body); err != nil {
		jsonError(w, 400, err.Error())
		return
	}
	if err := m.updateLoaded(caller, body.Documents); err != nil {
		jsonError(w, 409, err.Error())
		return
	}
	jsonResponse(w, 200, map[string]any{"ok": true})
}
