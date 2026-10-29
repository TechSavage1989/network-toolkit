package main

// Saved profiles are display metadata, not capabilities. Detached editors may
// ask the main window to review a profile only after existing explorer approval.
// No route here unlocks a vault, probes a host, connects, or grants a transport.
import (
	"crypto/sha256"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type editorSavedProfile struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Protocol  string `json:"protocol"`
	Host      string `json:"host"`
	Port      int    `json:"port"`
	signature [32]byte
}
type editorSavedConnectionRequest struct {
	id      string
	profile editorSavedProfile
	access  *editorExplorerAccess
	expires time.Time
}

func (a *application) editorSavedProfiles() []editorSavedProfile {
	a.stateMu.Lock()
	data := append([]byte(nil), a.state...)
	a.stateMu.Unlock()
	var state map[string]any
	rows := []editorSavedProfile{}
	if json.Unmarshal(data, &state) != nil {
		return rows
	}
	// Reuse existing nonsecret schema validation before selecting a tiny display
	// allowlist. The full settings object and credential references never leave.
	sanitizeTerminalSettings(state)
	sanitizeFTPProfiles(state)
	seen := map[string]bool{}
	for _, collection := range []string{"terminalProfiles", "ftpProfiles"} {
		list, _ := state[collection].([]map[string]any)
		for _, value := range list {
			id, _ := value["id"].(string)
			name, _ := value["name"].(string)
			host, _ := value["host"].(string)
			protocol, prefix := "ssh", "ssh:"
			if collection == "terminalProfiles" {
				if value["type"] != "ssh" {
					continue
				}
			} else {
				protocol, prefix = "ftp", "ftp:"
				if value["security"] == "explicit" || value["security"] == "implicit" {
					protocol = "ftps"
				}
			}
			if id == "" || len(id) > 100 || name == "" || host == "" || len(host) > 255 {
				continue
			}
			port := 0
			switch n := value["port"].(type) {
			case float64:
				port = int(n)
			case int:
				port = n
			case string:
				port, _ = strconv.Atoi(n)
			}
			if port < 1 || port > 65535 || seen[prefix+id] {
				continue
			}
			encoded, _ := json.Marshal(value)
			rows = append(rows, editorSavedProfile{ID: prefix + id, Name: name, Protocol: protocol, Host: host, Port: port, signature: sha256.Sum256(encoded)})
			seen[prefix+id] = true
		}
	}
	return rows
}
func (m *editorViewRegistry) savedConnectionAllowedLocked(caller string) bool {
	if !m.callerLocked(caller) {
		return false
	}
	if caller == "" {
		return !m.homeClosing
	}
	v := m.views[caller]
	return v != nil && v.explorer != nil && v.explorer.Allowed
}
func (m *editorViewRegistry) savedConnectionRequestLocked(v *editorWindow) *editorSavedConnectionRequest {
	if v == nil || v.savedConnection == nil {
		return nil
	}
	q := v.savedConnection
	if !m.now().Before(q.expires) || v.explorer != q.access || !q.access.Allowed || v.info.State == "closed" || v.info.State == "returned" {
		v.savedConnection = nil
		return nil
	}
	return q
}
func (m *editorViewRegistry) savedConnectionRequestsLocked(caller string) []map[string]any {
	rows := []map[string]any{}
	if caller != "" {
		return rows
	}
	for id, v := range m.views {
		if q := m.savedConnectionRequestLocked(v); q != nil {
			rows = append(rows, map[string]any{"viewId": id, "name": v.info.Name, "requestId": q.id, "profile": q.profile})
		}
	}
	return rows
}
func (m *editorViewRegistry) handleSavedConnections(w http.ResponseWriter, r *http.Request, caller, path string) bool {
	if path != "/saved-connections" && !strings.HasPrefix(path, "/saved-connections/") {
		return false
	}
	m.mu.Lock()
	allowed := m.savedConnectionAllowedLocked(caller)
	m.mu.Unlock()
	if !allowed {
		jsonError(w, 403, "Request and obtain file explorer access in the main window first.")
		return true
	}
	profiles := m.app.editorSavedProfiles()
	if path == "/saved-connections" && r.Method == http.MethodGet {
		m.mu.Lock()
		defer m.mu.Unlock()
		if !m.savedConnectionAllowedLocked(caller) {
			jsonError(w, 403, "File explorer access changed. Request approval again.")
			return true
		}
		rows := []editorSavedProfile{}
		for _, p := range profiles {
			if caller == "" || p.Protocol == "ssh" {
				rows = append(rows, p)
			}
		}
		pending := ""
		if caller != "" {
			if q := m.savedConnectionRequestLocked(m.views[caller]); q != nil {
				pending = q.id
			}
		}
		jsonResponse(w, 200, map[string]any{"profiles": rows, "detached": caller != "", "pendingRequest": pending})
		return true
	}
	if r.Method != http.MethodPost {
		methodError(w)
		return true
	}
	var req struct {
		ID        string `json:"id"`
		ViewID    string `json:"viewId"`
		RequestID string `json:"requestId"`
		Connect   bool   `json:"connect"`
	}
	if err := decodeEditorBody(w, r, 4096, &req); err != nil {
		jsonError(w, 400, err.Error())
		return true
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.savedConnectionAllowedLocked(caller) {
		jsonError(w, 403, "File explorer access changed.")
		return true
	}
	switch path {
	case "/saved-connections/request":
		if caller == "" || req.ViewID != "" || req.RequestID != "" || req.Connect {
			jsonError(w, 403, "Only the requesting editor may ask for its own saved SSH connection.")
			return true
		}
		var profile *editorSavedProfile
		for i := range profiles {
			if profiles[i].ID == req.ID && profiles[i].Protocol == "ssh" {
				profile = &profiles[i]
				break
			}
		}
		if profile == nil {
			jsonError(w, 404, "The saved SSH profile no longer exists. Refresh the list.")
			return true
		}
		v := m.views[caller]
		if q := m.savedConnectionRequestLocked(v); q != nil {
			if q.profile.ID != profile.ID || q.profile.signature != profile.signature {
				jsonError(w, 409, "Cancel the existing saved connection request first.")
				return true
			}
			jsonResponse(w, 200, map[string]any{"requestId": q.id, "pending": true})
			return true
		}
		v.savedConnection = &editorSavedConnectionRequest{id: randomHex(16), profile: *profile, access: v.explorer, expires: m.now().Add(2 * time.Minute)}
		jsonResponse(w, 200, map[string]any{"requestId": v.savedConnection.id, "pending": true})
	case "/saved-connections/cancel":
		if caller == "" || req.ViewID != "" || req.ID != "" || req.Connect {
			jsonError(w, 403, "Only the requesting editor may cancel its request.")
			return true
		}
		v := m.views[caller]
		q := m.savedConnectionRequestLocked(v)
		if q != nil && q.id != req.RequestID {
			jsonError(w, 409, "The saved connection request changed.")
			return true
		}
		v.savedConnection = nil
		jsonResponse(w, 200, map[string]any{"ok": true})
	case "/saved-connections/review":
		if caller != "" || req.ID != "" {
			jsonError(w, 403, "Review saved connections in the main window.")
			return true
		}
		v := m.views[req.ViewID]
		q := m.savedConnectionRequestLocked(v)
		if q == nil || q.id != req.RequestID {
			jsonError(w, 409, "The request expired, was cancelled, or its explorer approval changed.")
			return true
		}
		if req.Connect {
			valid := false
			for _, p := range profiles {
				if p.ID == q.profile.ID && p.signature == q.profile.signature {
					valid = true
					break
				}
			}
			if !valid {
				v.savedConnection = nil
				jsonError(w, 409, "The saved profile changed. Ask the editor to request it again.")
				return true
			}
		}
		v.savedConnection = nil
		// Acknowledging a request authorizes no transport; the main renderer must
		// still use the existing explicit host-key/authentication flow and later
		// approve Update access separately.
		jsonResponse(w, 200, map[string]any{"ok": true, "profile": q.profile, "connect": req.Connect})
	default:
		jsonError(w, 404, "Unknown saved connection action.")
	}
	return true
}
