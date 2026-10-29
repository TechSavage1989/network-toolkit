package main

// Detached editors exchange bounded document snapshots, never application
// settings or authentication. A backend lease—not a renderer timeout—owns every
// transferred document. Accepted transfers cannot be rolled back by their source.
import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"networktoolkit/internal/diag"
	"networktoolkit/internal/model"
)

const editorTransferLimit = 256 << 20
const editorDocumentLimit = 64 << 20
const editorViewLimit = 8
const editorDocumentCountLimit = 64

var editorHexID = regexp.MustCompile(`^[0-9a-f]{32}$`)
var editorUID = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)
var editorHash = regexp.MustCompile(`^[0-9a-f]{64}$`)
var errEditorLease = errors.New("The document owner or revision changed. Refresh the editor window before continuing.")

type editorBinding struct {
	Kind         string `json:"kind"`
	SessionID    string `json:"sessionId,omitempty"`
	Revision     uint64 `json:"revision,omitempty"`
	ConnectionID string `json:"connectionId,omitempty"`
}
type editorSelection struct {
	Start     int     `json:"start"`
	End       int     `json:"end"`
	Direction string  `json:"direction"`
	Top       float64 `json:"top"`
	Left      float64 `json:"left"`
}
type editorDocumentSnapshot struct {
	ID            string           `json:"id"`
	UID           string           `json:"uid"`
	Path          string           `json:"path"`
	Source        string           `json:"source"`
	Binding       editorBinding    `json:"binding"`
	Hash          string           `json:"hash"`
	Encoding      string           `json:"encoding"`
	BOM           bool             `json:"bom"`
	Original      string           `json:"original"`
	Content       string           `json:"content"`
	Saved         string           `json:"saved"`
	SavedEOL      string           `json:"savedEol"`
	SavedBOM      bool             `json:"savedBOM"`
	SavedEncoding string           `json:"savedEncoding"`
	EOL           string           `json:"eol"`
	Dirty         bool             `json:"dirty"`
	SaveFailed    bool             `json:"saveFailed,omitempty"`
	Undo          []string         `json:"undo"`
	Redo          []string         `json:"redo"`
	Language      string           `json:"language"`
	Untitled      bool             `json:"untitled"`
	EditRevision  uint64           `json:"editRevision"`
	View          *editorSelection `json:"view,omitempty"`
	CompareView   *editorSelection `json:"compareView,omitempty"`
}
type editorCompare struct {
	Left  string  `json:"left"`
	Right string  `json:"right"`
	Scale float64 `json:"scale"`
	Sync  bool    `json:"sync"`
}

// View membership contains document identities only, never another file grant.
type editorLayout struct {
	Split      bool             `json:"split"`
	ActiveSide string           `json:"activeSide"`
	Left       string           `json:"left"`
	Right      string           `json:"right"`
	Groups     []editorTabGroup `json:"groups"`
}
type editorTabGroup struct {
	UID  string `json:"uid"`
	Side string `json:"side"`
}
type editorPreferences struct {
	WordWrap bool              `json:"wordWrap"`
	Syntax   bool              `json:"syntax"`
	Indent   int               `json:"indent"`
	FontSize int               `json:"fontSize"`
	Palette  string            `json:"palette"`
	AppTheme string            `json:"appTheme,omitempty"`
	Colors   map[string]string `json:"colors"`
}
type editorBatch struct {
	Documents   []editorDocumentSnapshot `json:"documents"`
	Active      string                   `json:"active"`
	Compare     *editorCompare           `json:"compare"`
	Layout      *editorLayout            `json:"layout,omitempty"`
	Preferences *editorPreferences       `json:"preferences,omitempty"`
}
type editorTransferRequest struct {
	ID          string            `json:"id"`
	Destination string            `json:"destination"`
	Batch       editorBatch       `json:"batch"`
	Leases      map[string]uint64 `json:"leases"`
	DragTicket  string            `json:"dragTicket,omitempty"`
}
type editorViewInfo struct {
	ID    string `json:"id"`
	State string `json:"state"`
	Name  string `json:"name"`
}
type editorWindow struct {
	savedConnection *editorSavedConnectionRequest
	explorer        *editorExplorerAccess
	info            editorViewInfo
	token           string
	lastPulse       time.Time
}
type editorDocumentGrant struct {
	filePath string
	snapshot editorDocumentSnapshot
	owner    string
	lease    uint64
	pending  string
	busy     int
	orphan   bool
	bytes    int
	run      func(context.Context, remoteBrowserRequest) (model.Result, error)
	valid    func() bool
}
type editorTransfer struct {
	id, source, destination, state, failure, digest string
	batch                                           *editorBatch
	leases                                          map[string]uint64
	created                                         time.Time
	acked                                           bool
}
type editorViewRegistry struct {
	loaded                   map[string]map[string]editorLoadedDocument
	explorerOperations       map[string][]editorExplorerTarget
	mu                       sync.Mutex
	app                      *application
	views                    map[string]*editorWindow
	documents                map[string]*editorDocumentGrant
	transfers                map[string]*editorTransfer
	drags                    map[string]*editorDrag
	order                    []string
	recovery                 map[string]bool
	homePresent, homeClosing bool
	homeCloseApproved        bool
	nextView                 uint64
	now                      func() time.Time
}
type editorViewContextKey struct{}

func editorRequestView(r *http.Request) string {
	value, _ := r.Context().Value(editorViewContextKey{}).(string)
	return value
}
func withEditorRequestView(r *http.Request, id string) *http.Request {
	return r.WithContext(context.WithValue(r.Context(), editorViewContextKey{}, id))
}

func newEditorViewRegistry(a *application) *editorViewRegistry {
	m := &editorViewRegistry{app: a, views: map[string]*editorWindow{}, documents: map[string]*editorDocumentGrant{}, transfers: map[string]*editorTransfer{}, recovery: map[string]bool{}, homePresent: true, now: time.Now}
	if a.ctx != nil {
		go func() {
			ticker := time.NewTicker(time.Second)
			defer ticker.Stop()
			for {
				select {
				case <-a.ctx.Done():
					return
				case <-ticker.C:
					m.mu.Lock()
					m.expireLocked()
					m.mu.Unlock()
				}
			}
		}()
	}
	return m
}
func (m *editorViewRegistry) authenticate(token string) (string, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(token) != 64 {
		return "", false
	}
	for id, v := range m.views {
		if v.info.State != "closed" && subtle.ConstantTimeCompare([]byte(v.token), []byte(token)) == 1 {
			return id, true
		}
	}
	return "", false
}
func (m *editorViewRegistry) token(id string) (string, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	v := m.views[id]
	if v == nil || v.info.State == "closed" || v.info.State == "returned" {
		return "", false
	}
	return v.token, true
}
func (m *editorViewRegistry) info(id string) (editorViewInfo, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	v := m.views[id]
	if v == nil || v.info.State == "closed" || v.info.State == "returned" {
		return editorViewInfo{}, false
	}
	return v.info, true
}
func (m *editorViewRegistry) callerLocked(caller string) bool {
	if m.app.shutdownRequested.Load() || (m.app.ctx != nil && m.app.ctx.Err() != nil) {
		return false
	}
	if caller == "" {
		return m.homePresent && !m.homeClosing
	}
	v := m.views[caller]
	return v != nil && v.info.State != "closed" && v.info.State != "returned"
}

func editorSameBinding(previous, next editorBinding) bool {
	if previous == next {
		return true
	}
	return previous.Kind == "localDraft" && next.Kind == "local" && previous.SessionID == "" && next.SessionID == "" && previous.ConnectionID == "" && next.ConnectionID == "" && previous.Revision == 0 && next.Revision == 0
}
func (m *editorViewRegistry) hasLiveViews() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.recovery) > 0 {
		return true
	}
	for _, d := range m.documents {
		if d.orphan || (d.owner == "" && !m.homeCloseApproved) {
			return true
		}
	}
	for _, v := range m.views {
		if v.info.State != "closed" {
			return true
		}
	}
	return false
}
func (m *editorViewRegistry) retainedTransports() (map[string]bool, map[string]bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	terminals, browsers := map[string]bool{}, map[string]bool{}
	for uid, d := range m.documents {
		if d.owner == "" && d.pending == "" && !m.recovery[uid] {
			continue
		}
		switch d.snapshot.Binding.Kind {
		case "terminal":
			terminals[d.snapshot.Binding.SessionID] = true
		case "sshConnection":
			browsers[d.snapshot.Binding.ConnectionID] = true
		}
	}
	return terminals, browsers
}
func (m *editorViewRegistry) setHomePresent(present bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if !present {
		delete(m.loaded, "")
	}
	m.homePresent = present
	m.homeClosing = false
	if present {
		m.homeCloseApproved = false
		return
	}
	// This is a confirmed host exit, unlike a missed renderer heartbeat. A
	// prepared dialog is not discard consent; only accepted /shutdown is.
	for _, t := range m.transfers {
		if t.state == "offered" && (t.source == "" || t.destination == "") {
			m.failLocked(t, "failed", "The main editor host closed before accepting the transfer.")
		}
		if t.source == "" && t.state != "offered" {
			t.acked = true
		}
	}
	for uid, d := range m.documents {
		if d.owner != "" {
			continue
		}
		if m.homeCloseApproved && d.pending == "" && d.busy == 0 && !m.recovery[uid] {
			delete(m.documents, uid)
			continue
		}
		if d.busy > 0 {
			d.orphan = true
			continue
		}
		d.lease++
		m.recovery[uid] = true
	}
}

func (m *editorViewRegistry) hasRecovery() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.recovery) > 0
}
func (m *editorViewRegistry) approveHomeClose() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.homeClosing {
		return errors.New("Prepare the editor close before approving shutdown.")
	}
	m.homeCloseApproved = true
	return nil
}

// Called by the authenticated shutdown path before closing the primary window.
// This atomically prevents a new source offer from racing the close decision.
func (m *editorViewRegistry) beginHomeClose() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.recovery) > 0 {
		return errors.New("Restore recovered editor documents before closing this workspace.")
	}
	for _, d := range m.documents {
		if d.owner == "" && (d.pending != "" || d.busy != 0) {
			return errors.New("Finish the editor transfer or save before closing this workspace.")
		}
	}
	for _, t := range m.transfers {
		if t.state == "offered" && t.destination == "" {
			return errors.New("Finish receiving the editor document before closing this workspace.")
		}
	}
	m.homeClosing = true
	return nil
}

func (m *editorViewRegistry) cancelHomeClose() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.homeCloseApproved {
		return errors.New("Shutdown has already been approved.")
	}
	if m.homePresent {
		m.homeClosing = false
	}
	return nil
}

func validEditorText(s string, limit int) bool {
	return len(s) <= limit && utf8.ValidString(s) && !strings.ContainsRune(s, 0)
}
func validateEditorBatch(batch editorBatch) error {
	if p := batch.Preferences; p != nil {
		if p.AppTheme != "" && p.AppTheme != "light" && p.AppTheme != "dark" {
			return errors.New("Invalid editor application theme.")
		}
		if (p.Indent != 0 && p.Indent != 2 && p.Indent != 4 && p.Indent != 8) || p.FontSize < 10 || p.FontSize > 24 {
			return errors.New("Invalid editor window preferences.")
		}
		switch p.Palette {
		case "auto", "dark", "light", "pastel", "forest", "highContrast", "custom":
		default:
			return errors.New("Invalid editor palette.")
		}
		allowed := map[string]bool{"bg": true, "fg": true, "comment": true, "string": true, "keyword": true, "number": true, "property": true}
		for k, v := range p.Colors {
			if !allowed[k] || !regexp.MustCompile(`^#[0-9a-fA-F]{6}$`).MatchString(v) {
				return errors.New("Invalid editor custom color.")
			}
		}
	}
	if len(batch.Documents) == 0 || len(batch.Documents) > 12 {
		return errors.New("Transfer between one and twelve editor documents.")
	}
	uids, ids := map[string]bool{}, map[string]bool{}
	for _, d := range batch.Documents {
		if !editorUID.MatchString(d.UID) || uids[d.UID] || d.ID == "" || ids[d.ID] || !validEditorText(d.ID, 1024) || !validEditorText(d.Path, 4096) || d.Path == "" || !validEditorText(d.Source, 512) {
			return errors.New("Invalid or duplicate editor document identity.")
		}
		uids[d.UID] = true
		ids[d.ID] = true
		if d.Hash != "" && !editorHash.MatchString(d.Hash) {
			return errors.New("Invalid editor file revision.")
		}
		if d.EditRevision > 9007199254740991 || !validEditorText(d.Language, 128) {
			return errors.New("Invalid editor document metadata.")
		}
		for _, s := range []string{d.Content, d.Original, d.Saved} {
			if !validEditorText(s, editorDocumentLimit) {
				return errors.New("Editor contents exceed the 64 MiB decoded document limit or contain invalid text.")
			}
		}
		for _, s := range []string{d.EOL, d.SavedEOL} {
			if s != "LF" && s != "CRLF" && s != "CR" {
				return errors.New("Unsupported editor line ending.")
			}
		}
		for _, s := range []string{d.Encoding, d.SavedEncoding} {
			if s != "UTF-8" && s != "UTF-16LE" && s != "UTF-16BE" {
				return errors.New("Unsupported editor encoding.")
			}
		}
		if len(d.Undo) > 80 || len(d.Redo) > 80 {
			return errors.New("Editor history exceeds the transfer limit.")
		}
		for _, history := range [][]string{d.Undo, d.Redo} {
			for _, s := range history {
				if !validEditorText(s, editorDocumentLimit) {
					return errors.New("Invalid or oversized editor history.")
				}
			}
		}
		for _, v := range []*editorSelection{d.View, d.CompareView} {
			if v != nil && (v.Start < 0 || v.End < v.Start || v.End > editorDocumentLimit*2 || v.Top < 0 || v.Left < 0 || v.Top > 1e9 || v.Left > 1e9 || (v.Direction != "none" && v.Direction != "forward" && v.Direction != "backward")) {
				return errors.New("Invalid editor view position.")
			}
		}
		b := d.Binding
		switch b.Kind {
		case "local", "localDraft":
			if b.SessionID != "" || b.ConnectionID != "" || b.Revision != 0 {
				return errors.New("Local editor binding contains unrelated connection fields.")
			}
		case "terminal":
			if b.SessionID == "" || len(b.SessionID) > 128 || b.ConnectionID != "" || b.Revision > 9007199254740991 {
				return errors.New("Invalid terminal editor binding.")
			}
		case "sshConnection":
			if b.ConnectionID == "" || len(b.ConnectionID) > 128 || b.SessionID != "" || b.Revision != 0 {
				return errors.New("Invalid SSH editor binding.")
			}
		default:
			return errors.New("This file binding cannot move to another window. Save a local copy first.")
		}
	}
	if !uids[batch.Active] {
		return errors.New("The active document is not in the transfer.")
	}
	if c := batch.Compare; c != nil {
		if c.Left == c.Right || !ids[c.Left] || !ids[c.Right] || (c.Scale != 0 && c.Scale != .5 && c.Scale != 1 && c.Scale != 2 && c.Scale != 4) {
			return errors.New("Invalid editor comparison.")
		}
	}
	if layout := batch.Layout; layout != nil {
		if len(layout.Groups) != len(batch.Documents) || (layout.ActiveSide != "left" && layout.ActiveSide != "right") {
			return errors.New("Invalid editor view layout.")
		}
		membership := map[string]string{}
		for _, group := range layout.Groups {
			if !uids[group.UID] || membership[group.UID] != "" || (group.Side != "left" && group.Side != "right") {
				return errors.New("Invalid editor tab group.")
			}
			membership[group.UID] = group.Side
		}
		if (layout.Left != "" && membership[layout.Left] != "left") || (layout.Right != "" && membership[layout.Right] != "right") {
			return errors.New("The selected tab is not in its editor view.")
		}
	}
	raw, e := json.Marshal(batch)
	if e != nil || len(raw) > editorTransferLimit {
		return errors.New("Editor transfer exceeds the 256 MiB total limit.")
	}
	return nil
}

func validateEditorLeases(batch editorBatch, leases map[string]uint64) error {
	allowed := map[string]bool{}
	for _, d := range batch.Documents {
		allowed[d.UID] = true
	}
	for uid, lease := range leases {
		if !allowed[uid] || lease > 9007199254740991 {
			return errEditorLease
		}
	}
	return nil
}
func editorSnapshotBytes(d editorDocumentSnapshot) int { b, _ := json.Marshal(d); return len(b) }
func editorCloneBatch(b editorBatch) editorBatch {
	// Strings are immutable; copy mutable containers rather than serializing all
	// document text and undo history twice while the registry lock is held.
	copy := b
	copy.Documents = append([]editorDocumentSnapshot(nil), b.Documents...)
	for i := range copy.Documents {
		d := &copy.Documents[i]
		d.Undo = append([]string(nil), d.Undo...)
		d.Redo = append([]string(nil), d.Redo...)
		if d.View != nil {
			v := *d.View
			d.View = &v
		}
		if d.CompareView != nil {
			v := *d.CompareView
			d.CompareView = &v
		}
	}
	if b.Compare != nil {
		v := *b.Compare
		copy.Compare = &v
	}
	if b.Layout != nil {
		v := *b.Layout
		v.Groups = append([]editorTabGroup(nil), b.Layout.Groups...)
		copy.Layout = &v
	}
	if b.Preferences != nil {
		v := *b.Preferences
		if b.Preferences.Colors != nil {
			v.Colors = make(map[string]string, len(b.Preferences.Colors))
			for k, c := range b.Preferences.Colors {
				v.Colors[k] = c
			}
		}
		copy.Preferences = &v
	}
	return copy
}
func (m *editorViewRegistry) makeGrant(caller string, d editorDocumentSnapshot) (*editorDocumentGrant, error) {
	grant := &editorDocumentGrant{snapshot: d, owner: caller, lease: 1, bytes: editorSnapshotBytes(d)}
	b := d.Binding
	if b.Kind == "local" || b.Kind == "localDraft" {
		return grant, nil
	}
	// Only the home capability can delegate an existing authenticated transport.
	// Child windows cannot introduce a guessed connection/session identifier.
	if caller != "" {
		m.mu.Lock()
		v := m.views[caller]
		var original *editorDocumentGrant
		if v != nil && v.explorer != nil && v.explorer.Allowed {
			original = v.explorer.Remotes[explorerBindingKey(b)]
		}
		m.mu.Unlock()
		if original != nil {
			if original.valid() && editorSameBinding(original.snapshot.Binding, b) {
				grant.run = original.run
				grant.valid = original.valid
				return grant, nil
			}
		}
		return nil, errors.New("A new remote file binding must originate in the main workspace or an approved file explorer.")
	}
	if b.Kind == "sshConnection" {
		entry, err := m.app.browserConnection(b.ConnectionID)
		if err != nil {
			return nil, errors.New("The original SSH file connection is unavailable.")
		}
		grant.valid = func() bool {
			m.app.browserMu.Lock()
			defer m.app.browserMu.Unlock()
			if m.app.browserConnections[b.ConnectionID] != entry {
				return false
			}
			select {
			case <-entry.client.Done():
				return false
			default:
				return true
			}
		}
		grant.run = func(ctx context.Context, req remoteBrowserRequest) (model.Result, error) {
			return entry.client.Browse(ctx, req.request())
		}
		return grant, nil
	}
	if m.app.views == nil || m.app.terminals == nil {
		return nil, errors.New("The terminal service is unavailable.")
	}
	m.app.views.mu.Lock()
	defer m.app.views.mu.Unlock()
	owner := m.app.views.ownerLocked(b.SessionID)
	if owner.ViewID != "" || owner.Revision != b.Revision || !owner.Ready || !m.app.views.homePresent || m.app.views.homeClosing {
		return nil, errEditorLease
	}
	client, lifetime, err := m.app.terminals.SFTPClient(b.SessionID)
	if err != nil {
		return nil, errors.New("The original SSH terminal is unavailable.")
	}
	grant.valid = func() bool {
		m.app.views.mu.Lock()
		defer m.app.views.mu.Unlock()
		current := m.app.views.ownerLocked(b.SessionID)
		return current == owner && lifetime.Err() == nil
	}
	grant.run = func(ctx context.Context, req remoteBrowserRequest) (model.Result, error) {
		operation, cancel := context.WithCancel(ctx)
		defer cancel()
		stop := context.AfterFunc(lifetime, cancel)
		defer stop()
		return diag.RunSFTPOnClient(operation, client, req.request(), nil)
	}
	return grant, nil
}
func (m *editorViewRegistry) expireLocked() {
	now := m.now()
	for _, t := range m.transfers {
		if t.state == "offered" && now.Sub(t.created) > 20*time.Second {
			m.failLocked(t, "failed", "The destination did not acknowledge the transfer. The source still owns the documents.")
		}
	}
	m.expireDragsLocked()
}
func (m *editorViewRegistry) failLocked(t *editorTransfer, state, note string) {
	if t.state != "offered" {
		return
	}
	for _, d := range m.documents {
		if d.pending == t.id {
			d.pending = ""
		}
	}
	t.state = state
	t.failure = note
	t.batch = nil
}
func editorTransferInfo(t *editorTransfer) map[string]any {
	leases := map[string]uint64{}
	for uid, lease := range t.leases {
		leases[uid] = lease
	}
	return map[string]any{"id": t.id, "source": t.source, "destination": t.destination, "state": t.state, "error": t.failure, "leases": leases}
}
func (m *editorViewRegistry) state(caller string) map[string]any {
	askSaveBeforeClose := m.app.editorAskSaveBeforeClose()
	m.mu.Lock()
	defer m.mu.Unlock()
	m.expireLocked()
	views := []editorViewInfo{}
	incoming, outgoing, recovery := []any{}, []any{}, []editorDocumentSnapshot{}
	owned := map[string]uint64{}
	for _, v := range m.views {
		if v.info.State != "closed" && v.info.State != "returned" {
			views = append(views, v.info)
		}
	}
	sort.Slice(views, func(i, j int) bool {
		if views[i].Name == views[j].Name {
			return views[i].ID < views[j].ID
		}
		return views[i].Name < views[j].Name
	})
	for _, id := range m.order {
		t := m.transfers[id]
		if t.source == caller {
			outgoing = append(outgoing, editorTransferInfo(t))
		}
		if t.destination == caller && t.state == "offered" && t.batch != nil {
			row := editorTransferInfo(t)
			row["batch"] = editorCloneBatch(*t.batch)
			incoming = append(incoming, row)
		}
	}
	for uid, d := range m.documents {
		if d.owner == caller {
			owned[uid] = d.lease
			if caller == "" && m.recovery[uid] {
				recovery = append(recovery, d.snapshot)
			}
		}
	}
	return map[string]any{"available": m.app.launchEditorView != nil, "askSaveBeforeClose": askSaveBeforeClose, "self": caller, "views": views, "incoming": incoming, "outgoing": outgoing, "owned": owned, "recovery": recovery, "ownerInstanceId": m.app.windowInstanceID, "dragRequests": m.dragRequestsLocked(caller), "explorerRequests": m.explorerRequestsLocked(caller), "savedConnectionRequests": m.savedConnectionRequestsLocked(caller)}
}
func (m *editorViewRegistry) offer(caller string, req editorTransferRequest) (map[string]any, error) {
	if !editorHexID.MatchString(req.ID) {
		return nil, errors.New("Invalid transfer identifier.")
	}
	if req.DragTicket != "" && (!editorHexID.MatchString(req.DragTicket) || req.DragTicket != req.ID) {
		return nil, errEditorDrag
	}
	if err := validateEditorBatch(req.Batch); err != nil {
		return nil, err
	}
	if err := validateEditorLeases(req.Batch, req.Leases); err != nil {
		return nil, err
	}
	if req.Destination == "main" {
		req.Destination = ""
	}
	if req.Destination == caller {
		return nil, errors.New("Choose another editor window.")
	}
	raw, _ := json.Marshal(req)
	digest := editorDigest(raw)
	// Capture new grants before taking the registry lock: terminal ownership has
	// its own lock, and all reusable grants pin an existing transport lifetime.
	grants := map[string]*editorDocumentGrant{}
	m.mu.Lock()
	for _, d := range req.Batch.Documents {
		if m.documents[d.UID] == nil {
			grants[d.UID] = nil
		}
	}
	m.mu.Unlock()
	for _, d := range req.Batch.Documents {
		if _, ok := grants[d.UID]; ok {
			grant, e := m.makeGrant(caller, d)
			if e != nil {
				return nil, e
			}
			grants[d.UID] = grant
		}
	}
	m.mu.Lock()
	m.expireLocked()
	if old := m.transfers[req.ID]; old != nil {
		if old.source != caller || old.digest != digest {
			m.mu.Unlock()
			return nil, errEditorLease
		}
		out := editorTransferInfo(old)
		m.mu.Unlock()
		return out, nil
	}
	if !m.callerLocked(caller) {
		m.mu.Unlock()
		return nil, errEditorLease
	}
	drag, dragErr := m.editorDragOfferLocked(caller, req)
	if dragErr != nil {
		m.mu.Unlock()
		return nil, dragErr
	}
	destination := req.Destination
	if destination == "" && m.homeClosing {
		m.mu.Unlock()
		return nil, errors.New("The main workspace is reviewing a close request. Try again after it finishes.")
	}
	var launch *editorViewLaunch
	if destination == "new" {
		live := 0
		for _, v := range m.views {
			if v.info.State != "closed" {
				live++
			}
		}
		if live >= editorViewLimit || m.app.launchEditorView == nil {
			m.mu.Unlock()
			return nil, errors.New("No additional native editor window is available.")
		}
		destination = randomHex(16)
	} else if destination != "" && !m.callerLocked(destination) {
		m.mu.Unlock()
		return nil, errors.New("The destination editor is unavailable.")
	}
	total, count := 0, len(m.documents)
	for _, d := range m.documents {
		total += d.bytes
	}
	for _, snapshot := range req.Batch.Documents {
		if m.explorerConflictLocked(snapshot.Binding, snapshot.Path) {
			m.mu.Unlock()
			return nil, errors.New("A file explorer operation is changing this path.")
		}
		d := m.documents[snapshot.UID]
		if d == nil {
			d = grants[snapshot.UID]
			if d == nil || req.Leases[snapshot.UID] != 0 {
				m.mu.Unlock()
				return nil, errEditorLease
			}
			count++
		} else {
			if d.owner != caller || d.lease != req.Leases[snapshot.UID] || d.pending != "" || d.busy != 0 || !editorSameBinding(d.snapshot.Binding, snapshot.Binding) {
				m.mu.Unlock()
				return nil, errEditorLease
			}
			total -= d.bytes
		}
		total += editorSnapshotBytes(snapshot)
	}
	if count > editorDocumentCountLimit || total > editorTransferLimit {
		m.mu.Unlock()
		return nil, errors.New("The workspace editor recovery budget is full. Close unused documents before transferring.")
	}
	if len(m.order) >= 64 {
		for i, id := range m.order {
			if m.transfers[id].state != "offered" && m.transfers[id].acked {
				delete(m.transfers, id)
				m.order = append(m.order[:i], m.order[i+1:]...)
				break
			}
		}
	}
	if len(m.order) >= 64 {
		m.mu.Unlock()
		return nil, errors.New("Too many pending editor transfers.")
	}
	if req.Destination == "new" {
		m.nextView++
		name := fmt.Sprintf("Editor %d", m.nextView)
		v := &editorWindow{info: editorViewInfo{ID: destination, State: "opening", Name: name}, token: randomHex(32), lastPulse: m.now()}
		m.views[destination] = v
		launch = &editorViewLaunch{Version: 1, OwnerHost: m.app.host, OwnerInstanceID: m.app.windowInstanceID, ViewID: destination, ViewToken: v.token, DataDir: filepath.Join(filepath.Dir(m.app.dataPath), editorViewDirectoryName, destination), Title: "Network Toolkit — " + name, NoBrowser: m.app.headless}
	}
	batch := editorCloneBatch(req.Batch)
	t := &editorTransfer{id: req.ID, source: caller, destination: destination, state: "offered", digest: digest, batch: &batch, leases: map[string]uint64{}, created: m.now()}
	for _, snapshot := range batch.Documents {
		d := m.documents[snapshot.UID]
		if d == nil {
			d = grants[snapshot.UID]
			m.registerLoadedLocked(snapshot.UID)
			m.documents[snapshot.UID] = d
		}
		d.snapshot = snapshot
		d.bytes = editorSnapshotBytes(snapshot)
		d.pending = t.id
		t.leases[snapshot.UID] = d.lease
	}
	m.transfers[t.id] = t
	m.order = append(m.order, t.id)
	if drag != nil {
		// Consume only together with the existing transfer. No new ownership,
		// launch or release path is introduced by a drag claim.
		drag.state = "consumed"
	}
	out := editorTransferInfo(t)
	m.mu.Unlock()
	if launch != nil {
		done, e := m.app.launchEditorView(m.app.ctx, *launch)
		if e != nil {
			m.mu.Lock()
			m.failLocked(t, "failed", "The native editor window could not be opened. The source still owns the documents.")
			delete(m.views, destination)
			out = editorTransferInfo(t)
			m.mu.Unlock()
			return out, nil
		}
		if done != nil {
			go func() { <-done; m.processExited(destination) }()
		}
	}
	if destination == "" {
		m.app.activatePrimary()
	}
	return out, nil
}
func (m *editorViewRegistry) accept(caller, id string) (map[string]any, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.expireLocked()
	t := m.transfers[id]
	if t == nil || t.destination != caller || !m.callerLocked(caller) {
		return nil, errEditorLease
	}
	if t.state == "accepted" {
		return editorTransferInfo(t), nil
	}
	if t.state != "offered" || t.batch == nil {
		return nil, errEditorLease
	}
	ownedCount := 0
	for _, d := range m.documents {
		if d.owner == caller {
			ownedCount++
		}
	}
	if ownedCount+len(t.batch.Documents) > 12 {
		return nil, errors.New("Close an editor tab before receiving more documents (12 per window).")
	}
	for _, s := range t.batch.Documents {
		d := m.documents[s.UID]
		if d == nil || d.owner != t.source || d.pending != id || d.lease != t.leases[s.UID] || d.busy != 0 {
			return nil, errEditorLease
		}
	}
	for _, s := range t.batch.Documents {
		d := m.documents[s.UID]
		d.owner = caller
		d.pending = ""
		d.lease++
		t.leases[s.UID] = d.lease
		delete(m.recovery, s.UID)
	}
	t.state = "accepted"
	t.batch = nil
	return editorTransferInfo(t), nil
}
func (m *editorViewRegistry) cancel(caller, id string) (map[string]any, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	t := m.transfers[id]
	if t == nil || t.source != caller {
		return nil, errEditorLease
	}
	if t.state == "offered" {
		m.failLocked(t, "cancelled", "The source cancelled the transfer; no destination ownership was granted.")
	}
	return editorTransferInfo(t), nil
}

func (m *editorViewRegistry) complete(caller, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	t := m.transfers[id]
	if t == nil {
		return errEditorTransferMissing
	}
	if t.source != caller || t.state == "offered" {
		return errEditorLease
	}
	t.acked = true
	return nil
}

var errEditorTransferMissing = errors.New("Editor transfer receipt is no longer retained.")

func (m *editorViewRegistry) processExited(id string) {
	m.mu.Lock()
	v := m.views[id]
	if v == nil || v.info.State == "closed" {
		m.mu.Unlock()
		return
	}
	delete(m.loaded, id)
	v.info.State = "closed"
	v.token = ""
	delete(m.views, id)
	for _, t := range m.transfers {
		if t.state == "offered" && (t.destination == id || t.source == id) {
			m.failLocked(t, "failed", "An editor process exited before transfer acceptance.")
		}
		if t.source == id && t.state != "offered" {
			t.acked = true
		}
	}
	recover := false
	for uid, d := range m.documents {
		if d.owner == id {
			d.pending = ""
			if d.busy > 0 {
				d.orphan = true
				continue
			}
			d.owner = ""
			d.lease++
			m.recovery[uid] = true
			recover = true
		}
	}
	m.mu.Unlock()
	if m.app.views != nil {
		m.app.views.mu.Lock()
		m.app.views.notifyLocked()
		m.app.views.mu.Unlock()
	}
	if recover {
		m.app.activatePrimary()
	}
}

// A renderer release cannot claim a process has died or reclaim its documents.
func (m *editorViewRegistry) release(caller string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, d := range m.documents {
		if d.owner == caller || d.pending != "" && m.transfers[d.pending].destination == caller {
			return errors.New("Return or close every owned document before releasing this editor window.")
		}
	}
	v := m.views[caller]
	if v == nil {
		return errEditorLease
	}
	delete(m.loaded, caller)
	v.info.State = "returned"
	return nil
}

func (m *editorViewRegistry) returned(id string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	v := m.views[id]
	return v != nil && v.info.State == "returned"
}
