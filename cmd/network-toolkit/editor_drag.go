package main

// Drag data is an opaque, short-lived gesture ticket, never a document or a
// window capability. Claiming one asks the ORIGINAL renderer to use its existing
// offer/accept/ACK path; it grants no file access and changes no document lease.
import (
	"errors"
	"net/http"
	"sort"
	"time"
)

const editorDragLifetime = 30 * time.Second
const editorDragRetention = 2 * time.Minute
const editorDragLimit = 64

var errEditorDrag = errors.New("The editor drag is no longer available. Start a new drag from the original tab.")
var errEditorDragMissing = errors.New("editor drag not registered")
var errEditorDragGone = errors.New("editor drag cancelled or expired")

type editorDrag struct {
	ticket, source, uid, destination, state string
	lease                                   uint64
	created                                 time.Time
}

type editorDragRequest struct {
	Ticket      string `json:"ticket"`
	UID         string `json:"uid"`
	Lease       uint64 `json:"lease"`
	Destination string `json:"destination"`
}

func (d *editorDrag) info() map[string]any {
	// No UID, title, path, contents, source capability or transport metadata is
	// exposed to a prospective receiver, including on rejected claims.
	return map[string]any{"state": d.state, "expiresAt": d.created.Add(editorDragLifetime).UTC().Format(time.RFC3339Nano)}
}

func (m *editorViewRegistry) dragSourceValidLocked(d *editorDrag) bool {
	g := m.documents[d.uid]
	return m.callerLocked(d.source) && g != nil && g.owner == d.source && g.lease == d.lease && d.lease > 0 && g.pending == "" && g.busy == 0 && !g.orphan
}

func (m *editorViewRegistry) expireDragsLocked() {
	now := m.now()
	for key, d := range m.drags {
		// Tombstones are retained beyond the gesture lifetime, including cancel
		// arriving before registration. Pruning cannot accept a claim: absent
		// tickets are always rejected and only the owning authenticated renderer
		// can explicitly register a fresh gesture for its current document lease.
		if now.Sub(d.created) >= editorDragRetention {
			delete(m.drags, key)
			continue
		}
		if d.state != "issued" && d.state != "claimed" {
			continue
		}
		if !now.Before(d.created.Add(editorDragLifetime)) {
			d.state = "expired"
		} else if !m.dragSourceValidLocked(d) || (d.state == "claimed" && d.destination != "new" && !m.callerLocked(d.destination)) {
			d.state = "invalid"
		}
	}
}

func (m *editorViewRegistry) registerDrag(caller, ticket, uid string, lease uint64) (map[string]any, error) {
	if !editorHexID.MatchString(ticket) || !editorUID.MatchString(uid) || lease == 0 {
		return nil, errEditorDrag
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.expireDragsLocked()
	if old := m.drags[ticket]; old != nil {
		if old.source != caller || old.uid != uid || old.lease != lease || (old.state != "issued" && old.state != "claimed") {
			return nil, errEditorDrag
		}
		return old.info(), nil // A retry never extends the original deadline.
	}
	d := &editorDrag{ticket: ticket, source: caller, uid: uid, lease: lease, state: "issued", created: m.now()}
	if m.app.launchEditorView == nil || !m.dragSourceValidLocked(d) || m.transfers[ticket] != nil || len(m.drags) >= editorDragLimit {
		return nil, errEditorDrag
	}
	for _, old := range m.drags {
		if old.source == caller && old.state == "claimed" {
			return nil, errEditorDrag // Do not replace a drop awaiting source ACK.
		}
	}
	for _, old := range m.drags {
		if old.source == caller && old.state == "issued" {
			old.state = "cancelled"
		}
	}
	if m.drags == nil {
		m.drags = make(map[string]*editorDrag)
	}
	m.drags[ticket] = d
	return d.info(), nil
}

func (m *editorViewRegistry) claimDrag(caller, ticket string, detach bool) (map[string]any, error) {
	if !editorHexID.MatchString(ticket) {
		return nil, errEditorDrag
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.expireDragsLocked()
	if !m.callerLocked(caller) {
		return nil, errEditorDrag
	}
	d := m.drags[ticket]
	if d == nil {
		// The receiver may have completed a fast drop before the source's
		// authenticated registration arrived. Only this status permits a short,
		// same-ticket client retry; it grants no ownership or file capability.
		return nil, errEditorDragMissing
	}
	destination := caller
	if detach {
		if d.source != caller {
			return nil, errEditorDrag
		}
		destination = "new"
	} else if d.source == caller {
		return nil, errEditorDrag // Same-window tab reordering remains local.
	}
	if d.state == "cancelled" || d.state == "expired" {
		return nil, errEditorDragGone
	}
	if d.state == "claimed" || d.state == "consumed" {
		if d.destination != destination {
			return nil, errEditorDrag
		}
		return d.info(), nil // Lost response: one claimant, no second transfer.
	}
	if d.state != "issued" || !m.dragSourceValidLocked(d) {
		return nil, errEditorDrag
	}
	d.destination, d.state = destination, "claimed"
	return d.info(), nil
}

func (m *editorViewRegistry) cancelDrag(caller, ticket string) (map[string]any, error) {
	if !editorHexID.MatchString(ticket) {
		return nil, errEditorDrag
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.expireDragsLocked()
	d := m.drags[ticket]
	if d == nil {
		if !m.callerLocked(caller) || len(m.drags) >= editorDragLimit {
			return nil, errEditorDrag
		}
		if m.drags == nil {
			m.drags = make(map[string]*editorDrag)
		}
		d = &editorDrag{ticket: ticket, source: caller, state: "cancelled", created: m.now()}
		m.drags[ticket] = d // Reject a delayed registration after cancellation.
	} else if d.source != caller {
		return nil, errEditorDrag
	} else if d.state == "issued" {
		d.state = "cancelled"
	}
	// An accepted drop wins a racing dragend/Escape. Never revoke its claim,
	// unfreeze a source or undo an offered/accepted transfer here.
	return d.info(), nil
}

func (m *editorViewRegistry) dragRequestsLocked(caller string) []editorDragRequest {
	out := []editorDragRequest{}
	for _, d := range m.drags {
		if d.source == caller && d.state == "claimed" {
			destination := d.destination
			if destination == "" {
				destination = "main"
			}
			out = append(out, editorDragRequest{Ticket: d.ticket, UID: d.uid, Lease: d.lease, Destination: destination})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Ticket < out[j].Ticket })
	return out
}

func (m *editorViewRegistry) editorDragOfferLocked(caller string, req editorTransferRequest) (*editorDrag, error) {
	if req.DragTicket == "" {
		if m.drags[req.ID] != nil {
			return nil, errEditorDrag // A reserved drag nonce cannot bypass claim.
		}
		return nil, nil
	}
	d := m.drags[req.DragTicket]
	if d == nil || d.state != "claimed" || d.source != caller || d.destination != req.Destination || len(req.Batch.Documents) != 1 || req.Batch.Documents[0].UID != d.uid || req.Leases[d.uid] != d.lease || !m.dragSourceValidLocked(d) {
		return nil, errEditorDrag
	}
	return d, nil
}

func (m *editorViewRegistry) handleDrag(w http.ResponseWriter, r *http.Request, caller, path string) {
	var info map[string]any
	var err error
	if path == "/drags" {
		var request struct {
			Ticket string `json:"ticket"`
			UID    string `json:"uid"`
			Lease  uint64 `json:"lease"`
		}
		if err = decodeEditorBody(w, r, 1024, &request); err == nil {
			info, err = m.registerDrag(caller, request.Ticket, request.UID, request.Lease)
		} else {
			jsonError(w, http.StatusBadRequest, err.Error())
			return
		}
	} else if path == "/drags/claim" || path == "/drags/detach" || path == "/drags/cancel" {
		var request struct {
			Ticket string `json:"ticket"`
		}
		if err = decodeEditorBody(w, r, 1024, &request); err != nil {
			jsonError(w, http.StatusBadRequest, err.Error())
			return
		}
		if path == "/drags/cancel" {
			info, err = m.cancelDrag(caller, request.Ticket)
		} else {
			info, err = m.claimDrag(caller, request.Ticket, path == "/drags/detach")
		}
	} else {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		status := http.StatusConflict
		if errors.Is(err, errEditorDragMissing) {
			status = http.StatusNotFound
		} else if errors.Is(err, errEditorDragGone) {
			status = http.StatusGone
		}
		jsonError(w, status, errEditorDrag.Error())
		return
	}
	jsonResponse(w, http.StatusOK, info)
}
