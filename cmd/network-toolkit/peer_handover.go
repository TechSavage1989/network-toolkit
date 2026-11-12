package main

import (
	"context"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

// Document hand-over (owner decision): when the user pops the Text Editor out
// of the main Toolkit window while the standalone Text Editor of the same data
// root is already running on its own, the tabs that do not depend on the main
// window are added to that editor instead of a new connected window:
//   - untitled documents held by the page (their text and title), and
//   - local files, saved or with unsaved text held by the page, and saved
//     (clean) local files that the main engine holds disk-backed.
//
// A page-held tab also carries its undo history (owner decision, v1.21.50),
// within the same 8 MiB as unsaved text; the receiving editor restores it only
// when it opened exactly the text the history belongs to, and otherwise says
// that the tab moved without it. A tab whose history does not fit moves
// without it, and the main window says so; nothing is dropped silently.
// Disk-backed tabs move without history (their edit buffer stays in this
// engine), and the main window says so when they had any.
//
// Everything else (files opened over SFTP, SSH or FTP, binary files, unsaved
// changes in disk-backed documents, files that changed on disk) still opens in
// a connected detached window, as before. No standalone editor running: no
// change at all.
//
// This is the main window's only sending use of peer send, with the same
// guards as text (peer_send.go):
//   - The main page asks its own engine (session token, /api/peer-handover/).
//     Child windows use view tokens and never reach these routes; standalone
//     apps do not serve them.
//   - The engine finds the standalone editor through its protected instance
//     record, proves the endpoint is that instance (status probe) and sends to
//     that instance only (peerCallReceiver with the probed instance pinned).
//   - The editor's engine accepts /api/peer/documents/ only with its activation
//     token, only from loopback, never from a page, strict JSON, at most 8 MiB
//     of unsaved text and 12 tabs per request, one pending request (shared
//     with text), for at most a minute.
//   - Its page collects the request and opens each tab with its ordinary code
//     (the local open with all its checks, a new document), behind the same
//     blocked()/guard checks as text, keeps the unsaved text in its session
//     when keeping is on, and confirms each tab.
//   - The main page keeps every tab read-only until that confirmation, and
//     releases only the tabs the editor confirmed. A refusal, a timeout or a
//     receiver that closes leaves the tabs in the main window unchanged.
//
// File paths travel here, and only here: an owner-approved exception to "never
// file paths". A path must be an absolute path on this computer (a drive
// letter on Windows; no UNC, device or network-drive path, no "." or ".."
// element), and the receiving page opens it through its normal local open.
// Credentials, tokens, host keys, connection identities and remote paths never
// travel: a remote tab is never handed over.
const (
	peerHandoverKind      = "documents"
	peerHandoverPrefix    = "/api/peer-handover/"
	peerHandoverItemLimit = 12                  // the editor's own tab limit
	peerHandoverTextLimit = peerEditorTextLimit // unsaved text per request
	peerHandoverNameRunes = 255
	peerHandoverPathBytes = 4096
	// JSON can spend six bytes on one character; paths and names are small.
	peerHandoverBodyLimit = 6*peerHandoverTextLimit + 256<<10
	// A main page that stopped asking leaves its hand-over behind this long.
	peerHandoverAbandoned = peerRequestLifetime + 30*time.Second
	// peerHandoverStatusDeadline bounds how long a hand-over whose status
	// cannot be read is still asked about: one failed check (the editor busy
	// for a moment) never ends it. It ends before peerHandoverAbandoned and
	// after the editor's own request lifetime.
	peerHandoverStatusDeadline = peerRequestLifetime + 15*time.Second
)

var (
	peerHandoverLanguage = regexp.MustCompile(`^[A-Za-z0-9._+-]{1,64}$`)
	peerHandoverDrive    = regexp.MustCompile(`^[A-Za-z]:[\\/]`)
)

// peerHandoverFormat is a page-held document's encoding, byte order mark and
// line ending. Only formats a page-held document can hold travel: Windows-1252,
// mixed line endings and binary files exist only in disk-backed documents, and
// their unsaved text never leaves the main engine this way.
type peerHandoverFormat struct {
	Encoding string `json:"encoding"`
	BOM      bool   `json:"bom"`
	EOL      string `json:"eol"`
}

// peerHandoverItem is one tab. "file": a local file, with the digest of the
// bytes the main window read; a dirty one also carries its text and both its
// current and its file's format. "untitled": a new document's title, text and
// format.
type peerHandoverItem struct {
	ID          string              `json:"id"`
	Kind        string              `json:"kind"`
	Path        string              `json:"path,omitempty"`
	Name        string              `json:"name,omitempty"`
	Hash        string              `json:"hash,omitempty"`
	Dirty       bool                `json:"dirty,omitempty"`
	Text        string              `json:"text,omitempty"`
	Format      *peerHandoverFormat `json:"format,omitempty"`
	SavedFormat *peerHandoverFormat `json:"savedFormat,omitempty"`
	Language    string              `json:"language,omitempty"`
	// Undo and Redo are a page-held tab's undo history (owner decision,
	// v1.21.50): the texts the editor's own Undo and Redo return to, oldest
	// first, exactly as the editor keeps them. They count against the same
	// 8 MiB as unsaved text; a tab whose history does not fit moves without
	// it and the user is told. Disk-backed tabs never carry history: it lives
	// in the main engine's edit buffer and cannot be rebuilt exactly elsewhere.
	Undo []string `json:"undo,omitempty"`
	Redo []string `json:"redo,omitempty"`
}

// peerHandoverResult is what the receiving page did with one tab. HistoryLost
// says that a tab that moved did not keep the undo history it carried (the
// receiving editor could not apply it exactly); the user is told.
type peerHandoverResult struct {
	ID          string `json:"id"`
	OK          bool   `json:"ok"`
	Error       string `json:"error,omitempty"`
	HistoryLost bool   `json:"historyLost,omitempty"`
}

// peerHandoverHistoryEntries is the editor's own undo limit (file-editor.js).
const peerHandoverHistoryEntries = 80

// peerHandoverRequest is the body the main engine posts to the editor's engine.
type peerHandoverRequest struct {
	RequestID string             `json:"requestId"`
	Documents []peerHandoverItem `json:"documents"`
}

// peerHandoverOutbound is the main window's one waiting hand-over.
type peerHandoverOutbound struct {
	id         string // empty while the request is being delivered
	instanceID string
	ids        []string
	created    time.Time
}

type peerHandoverIneligible struct {
	ID     string `json:"id"`
	Reason string `json:"reason"`
}

func peerHandoverCarried(f *peerHandoverFormat) bool {
	if f == nil {
		return false
	}
	switch f.Encoding {
	case "UTF-8", "UTF-16LE", "UTF-16BE":
	default:
		return false
	}
	switch f.EOL {
	case "LF", "CRLF", "CR":
		return true
	}
	return false
}

// peerHandoverLocalPath accepts an absolute path on this computer only.
func peerHandoverLocalPath(p string) error {
	invalid := errors.New("A tab's file path is not a path on this computer.")
	if p == "" || len(p) > peerHandoverPathBytes || !utf8.ValidString(p) || strings.ContainsFunc(p, unicode.IsControl) {
		return invalid
	}
	if runtime.GOOS == "windows" {
		// A drive letter, and no other colon (no alternate data stream).
		if !peerHandoverDrive.MatchString(p) || strings.Contains(p[2:], ":") {
			return invalid
		}
	} else if !strings.HasPrefix(p, "/") || strings.HasPrefix(p, "//") {
		return invalid
	}
	for _, element := range strings.FieldsFunc(p, func(r rune) bool { return r == '/' || r == '\\' }) {
		if element == "." || element == ".." {
			return invalid
		}
	}
	if peerHandoverRemoteDrive(p) {
		return errors.New("A file on a network drive stays with " + appName + ".")
	}
	return nil
}

func peerHandoverName(name string) bool {
	trimmed := strings.TrimSpace(name)
	return trimmed != "" && utf8.ValidString(name) && utf8.RuneCountInString(name) <= peerHandoverNameRunes && !strings.ContainsFunc(name, unicode.IsControl) && !strings.ContainsAny(name, `/\`)
}

// validatePeerHandover checks a hand-over strictly, in the main engine and
// again in the receiving engine.
func validatePeerHandover(items []peerHandoverItem) error {
	if len(items) == 0 || len(items) > peerHandoverItemLimit {
		return errors.New("Hand over between 1 and 12 tabs at a time.")
	}
	seen := make(map[string]bool, len(items))
	total := 0
	for _, item := range items {
		if !peerIDPattern.MatchString(item.ID) || seen[item.ID] {
			return errors.New("A tab has an invalid identifier.")
		}
		seen[item.ID] = true
		if item.Language != "" && !peerHandoverLanguage.MatchString(item.Language) {
			return errors.New("A tab's language is invalid.")
		}
		if !utf8.ValidString(item.Text) {
			return errors.New("A tab's text is not valid UTF-8.")
		}
		if total += len(item.Text); total > peerHandoverTextLimit {
			return errors.New("Hand over no more than 8 MiB of unsaved text at a time.")
		}
		if len(item.Undo) > peerHandoverHistoryEntries || len(item.Redo) > peerHandoverHistoryEntries {
			return errors.New("A tab's undo history is longer than the editor keeps.")
		}
		for _, entry := range append(append([]string(nil), item.Undo...), item.Redo...) {
			if !utf8.ValidString(entry) {
				return errors.New("A tab's undo history is not valid UTF-8.")
			}
			if total += len(entry); total > peerHandoverTextLimit {
				return errors.New("Hand over no more than 8 MiB of unsaved text and undo history at a time.")
			}
		}
		switch item.Kind {
		case "file":
			if item.Name != "" {
				return errors.New("A file tab carries its path, not a name.")
			}
			if err := peerHandoverLocalPath(item.Path); err != nil {
				return err
			}
			if !editorHash.MatchString(item.Hash) {
				return errors.New("A file tab must name the file it was read from.")
			}
			if item.Dirty {
				if !peerHandoverCarried(item.Format) || !peerHandoverCarried(item.SavedFormat) {
					return errors.New("A tab's encoding or line endings cannot be carried exactly.")
				}
			} else if item.Text != "" || item.Format != nil || item.SavedFormat != nil {
				return errors.New("A saved file tab is opened from disk; it carries no text.")
			}
		case "untitled":
			if item.Path != "" || item.Hash != "" {
				return errors.New("An untitled tab has no file.")
			}
			if !peerHandoverName(item.Name) {
				return errors.New("An untitled tab's title is invalid.")
			}
			if !peerHandoverCarried(item.Format) || item.SavedFormat != nil && !peerHandoverCarried(item.SavedFormat) {
				return errors.New("A tab's encoding or line endings cannot be carried exactly.")
			}
		default:
			return errors.New("Only local and untitled tabs can be handed over.")
		}
	}
	return nil
}

// peerHandoverUnchanged confirms that a dirty file tab's file still has the
// bytes the main window read: its unsaved text applies to that file only.
func peerHandoverUnchanged(item peerHandoverItem) error {
	if item.Kind != "file" || !item.Dirty {
		return nil
	}
	name := filepath.Base(item.Path)
	data, _, err := editableFile(item.Path)
	switch {
	case errors.Is(err, os.ErrNotExist):
		return errors.New(name + " is no longer on disk")
	case err != nil:
		return errors.New(name + " could not be read on disk")
	case editorDigest(data) != item.Hash:
		return errors.New(name + " changed on disk since it was opened")
	}
	return nil
}

// peerHandoverResultsValid: a confirmation names every tab that was sent, once.
func peerHandoverResultsValid(ids []string, items []peerHandoverResult) bool {
	if len(items) != len(ids) || len(ids) == 0 {
		return false
	}
	want := make(map[string]bool, len(ids))
	for _, id := range ids {
		want[id] = true
	}
	for _, item := range items {
		if !want[item.ID] || !utf8.ValidString(item.Error) || utf8.RuneCountInString(item.Error) > peerMessageRunes || item.OK && item.Error != "" || item.HistoryLost && !item.OK {
			return false
		}
		delete(want, item.ID)
	}
	return len(want) == 0
}

// ---- Receiving side: the standalone Text Editor ----

// servePeerHandoverInbound handles /api/peer/documents/ in the standalone Text
// Editor. servePeerInbound has already checked loopback, page headers, the
// activation token, the data root and shutdown.
func (a *application) servePeerHandoverInbound(w http.ResponseWriter, r *http.Request) {
	base := peerInboundPrefix + peerHandoverKind + "/"
	switch r.URL.Path {
	case base + "status":
		if r.Method != http.MethodGet {
			a.peerRefuse(w, http.StatusMethodNotAllowed, "This method is not supported.")
			return
		}
		a.peer.mu.Lock()
		_, ready := a.peer.freshTargetsLocked(peerNow(), "editor")
		a.peer.mu.Unlock()
		a.peerReply(w, http.StatusOK, map[string]any{"ready": ready})
	case base + "handover":
		if r.Method != http.MethodPost {
			a.peerRefuse(w, http.StatusMethodNotAllowed, "This method is not supported.")
			return
		}
		var request *peerHandoverRequest
		if err := decodePeerBody(w, r, peerHandoverBodyLimit, &request); err != nil {
			a.peerRefuse(w, http.StatusBadRequest, err.Error())
			return
		}
		if request == nil || !peerRequestID.MatchString(request.RequestID) {
			a.peerRefuse(w, http.StatusBadRequest, "The request identifier is invalid.")
			return
		}
		if err := validatePeerHandover(request.Documents); err != nil {
			a.peerRefuse(w, http.StatusBadRequest, err.Error())
			return
		}
		// The main engine checked; check again here, nearer to the open.
		for _, item := range request.Documents {
			if err := peerHandoverUnchanged(item); err != nil {
				a.peerRefuse(w, http.StatusConflict, err.Error()+". Nothing was moved.")
				return
			}
		}
		if status, _, err := a.peerEnqueue(peerHandoverKind, peerPayload{RequestID: request.RequestID, Documents: request.Documents}); err != nil {
			a.peerRefuse(w, status, err.Error())
			return
		}
		a.activatePrimary() // the tabs arrive in this window: bring it forward
		a.peerReply(w, http.StatusOK, map[string]any{"requestId": request.RequestID, "state": peerQueued})
	case base + "result":
		if r.Method != http.MethodGet {
			a.peerRefuse(w, http.StatusMethodNotAllowed, "This method is not supported.")
			return
		}
		id := r.URL.Query().Get("requestId")
		var items []peerHandoverResult
		found, state, message := false, "", ""
		if peerRequestID.MatchString(id) {
			a.peer.mu.Lock()
			a.peer.expireLocked(peerNow())
			if req := a.peer.request; req != nil && req.id == id && req.kind == peerHandoverKind {
				found, state, message, items = true, req.state, req.message, append([]peerHandoverResult(nil), req.items...)
			}
			a.peer.mu.Unlock()
		}
		if !found {
			a.peerRefuse(w, http.StatusNotFound, "That hand-over is not known to "+peerAppName(a.windowMode)+".")
			return
		}
		a.peerReply(w, http.StatusOK, map[string]any{"requestId": id, "state": state, "message": message, "items": items})
	default:
		http.NotFound(w, r)
	}
}

// ---- Sending side: the main window ----

// peerHandoverRoot is the data root when this window may hand documents over:
// the main window only.
func (a *application) peerHandoverRoot() string {
	if a.windowMode != "main" {
		return ""
	}
	return a.peerRoot()
}

func peerHandoverReceiver(root string) peerReceiver {
	return peerReceiver{mode: standaloneEditorMode, dir: peerWorkspaceDir(root, standaloneEditorMode)}
}

// peerHandoverProbe proves that the standalone editor of this data root runs,
// takes documents (an older version has no such route) and that its page is
// running. It returns the instance that answered.
func peerHandoverProbe(ctx context.Context, rc peerReceiver) (string, bool, error) {
	reply, _, err := peerCallReceiver(ctx, rc, "", http.MethodGet, peerInboundPrefix+peerHandoverKind+"/status", nil, 2*time.Second)
	if err != nil {
		return "", false, err
	}
	return reply.InstanceID, reply.Ready, nil
}

// peerHandoverPendingLocked returns the waiting hand-over, dropping one that
// the main page abandoned long ago.
func (s *peerSendState) peerHandoverPendingLocked(now time.Time) *peerHandoverOutbound {
	if o := s.handover; o != nil && now.Sub(o.created) > peerHandoverAbandoned {
		s.handover = nil
	}
	return s.handover
}

func (a *application) handlePeerHandover(w http.ResponseWriter, r *http.Request) {
	root := a.peerHandoverRoot()
	if root == "" {
		jsonError(w, http.StatusNotFound, "Handing tabs to the Text Editor is available in the main window.")
		return
	}
	route := strings.TrimPrefix(r.URL.Path, peerHandoverPrefix)
	method := map[string]string{"receiver": http.MethodGet, "start": http.MethodPost, "result": http.MethodGet}[route]
	if method == "" {
		http.NotFound(w, r)
		return
	}
	if r.Method != method {
		methodError(w)
		return
	}
	rc := peerHandoverReceiver(root)
	name := rc.name()
	switch route {
	case "receiver":
		_, ready, err := peerHandoverProbe(r.Context(), rc)
		jsonResponse(w, http.StatusOK, map[string]any{"available": err == nil && ready, "name": name})
	case "start":
		var body *struct {
			Items []peerHandoverItem `json:"items"`
		}
		if err := decodePeerBody(w, r, peerHandoverBodyLimit, &body); err != nil {
			jsonError(w, http.StatusBadRequest, err.Error())
			return
		}
		if body == nil {
			jsonError(w, http.StatusBadRequest, "Send one JSON object.")
			return
		}
		if err := validatePeerHandover(body.Items); err != nil {
			jsonError(w, http.StatusBadRequest, err.Error())
			return
		}
		// One hand-over at a time: reserve it before anything leaves this engine.
		reservation := &peerHandoverOutbound{created: peerNow()}
		a.peer.mu.Lock()
		busy := a.peer.peerHandoverPendingLocked(peerNow()) != nil
		if !busy {
			a.peer.handover = reservation
		}
		a.peer.mu.Unlock()
		if busy {
			jsonError(w, http.StatusConflict, "Tabs are still being moved to "+name+". Wait for that to finish.")
			return
		}
		release := func() {
			a.peer.mu.Lock()
			if a.peer.handover == reservation {
				a.peer.handover = nil
			}
			a.peer.mu.Unlock()
		}
		instanceID, ready, err := peerHandoverProbe(r.Context(), rc)
		if err != nil || !ready || instanceID == "" {
			release()
			jsonError(w, http.StatusServiceUnavailable, name+" is not open or not ready. The tabs stay here.")
			return
		}
		send := make([]peerHandoverItem, 0, len(body.Items))
		sent := make([]string, 0, len(body.Items))
		ineligible := []peerHandoverIneligible{}
		for _, item := range body.Items {
			if err := peerHandoverUnchanged(item); err != nil {
				ineligible = append(ineligible, peerHandoverIneligible{ID: item.ID, Reason: err.Error()})
				continue
			}
			send = append(send, item)
			sent = append(sent, item.ID)
		}
		if len(send) == 0 {
			release()
			jsonResponse(w, http.StatusOK, map[string]any{"requestId": "", "state": "", "sent": sent, "ineligible": ineligible, "name": name})
			return
		}
		payload := peerHandoverRequest{RequestID: randomHex(16), Documents: send}
		reply, status, err := peerCallReceiver(r.Context(), rc, instanceID, http.MethodPost, peerInboundPrefix+peerHandoverKind+"/handover", payload, 20*time.Second)
		if err != nil || reply.RequestID != payload.RequestID {
			release()
			// The request may have reached the editor with its answer lost:
			// say plainly that the tabs stay here and where to look.
			message, code := name+" did not confirm that it took the tabs. The tabs stay here; check "+name+" in case they also opened there.", http.StatusServiceUnavailable
			// errPeerNotOpen also stands for a reply that never arrived.
			if errors.Is(err, errPeerOlder) {
				message = name + " did not take the tabs. The tabs stay here."
			} else if err != nil && !errors.Is(err, errPeerUnchecked) && status >= 400 && status < 500 {
				// An answer: the editor refused the request.
				message, code = strings.TrimSuffix(err.Error(), " Nothing was moved.")+" The tabs stay here.", http.StatusConflict
			}
			jsonError(w, code, message)
			return
		}
		a.peer.mu.Lock()
		reservation.id, reservation.instanceID, reservation.ids = payload.RequestID, instanceID, sent
		a.peer.mu.Unlock()
		jsonResponse(w, http.StatusOK, map[string]any{"requestId": payload.RequestID, "state": peerQueued, "sent": sent, "ineligible": ineligible, "name": name})
	case "result":
		id := r.URL.Query().Get("requestId")
		if !peerRequestID.MatchString(id) {
			jsonError(w, http.StatusBadRequest, "The request identifier is invalid.")
			return
		}
		a.peer.mu.Lock()
		o := a.peer.peerHandoverPendingLocked(peerNow())
		var pending peerHandoverOutbound
		if o != nil && o.id == id {
			pending = *o
		}
		a.peer.mu.Unlock()
		if pending.id == "" {
			jsonError(w, http.StatusNotFound, "That hand-over is no longer known. Check "+name+".")
			return
		}
		finish := func() {
			a.peer.mu.Lock()
			if a.peer.handover == o {
				a.peer.handover = nil
			}
			a.peer.mu.Unlock()
		}
		reply, _, err := peerCallReceiver(r.Context(), rc, pending.instanceID, http.MethodGet, peerInboundPrefix+peerHandoverKind+"/result?requestId="+id, nil, 2*time.Second)
		if err != nil || reply.RequestID != id {
			if peerNow().Sub(pending.created) < peerHandoverStatusDeadline {
				// Ask again on the page's next check.
				jsonResponse(w, http.StatusOK, map[string]any{"requestId": id, "state": peerTaken, "items": []peerHandoverResult{}, "name": name})
				return
			}
			finish()
			jsonResponse(w, http.StatusOK, map[string]any{"requestId": id, "state": "lost", "items": []peerHandoverResult{}, "name": name, "message": name + " closed or stopped answering before it confirmed the tabs. They stay here; check " + name + " for copies."})
			return
		}
		state, message, items := reply.State, peerPlainText(reply.Message, peerMessageRunes), []peerHandoverResult{}
		switch state {
		case peerQueued, peerTaken:
		case peerDone:
			if !peerHandoverResultsValid(pending.ids, reply.Items) {
				state, message = "lost", name+" sent an invalid confirmation. The tabs stay here; check "+name+" for copies."
				break
			}
			for _, item := range reply.Items {
				items = append(items, peerHandoverResult{ID: item.ID, OK: item.OK, Error: peerPlainText(item.Error, peerMessageRunes), HistoryLost: item.OK && item.HistoryLost})
			}
		case peerFailed:
			if message == "" {
				message = name + " did not take the tabs."
			}
		case peerExpired:
			message = name + " did not confirm the tabs in time. They stay here; check " + name + " for copies."
		default:
			state, message = "lost", name+" no longer knows this hand-over. The tabs stay here."
		}
		if state != peerQueued && state != peerTaken {
			finish()
		}
		jsonResponse(w, http.StatusOK, map[string]any{"requestId": id, "state": state, "items": items, "name": name, "message": message})
	}
}
