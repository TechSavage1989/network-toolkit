package main

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode/utf16"
	"unicode/utf8"
)

const localWiFiXMLLimit = 1 << 20

type localWiFiConnection struct {
	ProfileName    string `json:"profileName"`
	SSID           string `json:"ssid"`
	SSIDHex        string `json:"ssidHex"`
	Authentication string `json:"authentication"`
	Encryption     string `json:"encryption"`
	SignalQuality  uint32 `json:"signalQuality"`
}

type localWiFiProfile struct {
	Name           string `json:"name"`
	SSID           string `json:"ssid"`
	SSIDHex        string `json:"ssidHex"`
	Authentication string `json:"authentication"`
	Encryption     string `json:"encryption"`
	Scope          string `json:"scope"`
	KeyKind        string `json:"keyKind"`
	CanReveal      bool   `json:"canReveal"`
	Current        bool   `json:"current"`
	Note           string `json:"note,omitempty"`
}

type localWiFiInterface struct {
	Time          string               `json:"time,omitempty"`
	ID            string               `json:"id"`
	Description   string               `json:"description"`
	State         string               `json:"state"`
	Current       *localWiFiConnection `json:"current"`
	CurrentError  string               `json:"currentError,omitempty"`
	Profiles      []localWiFiProfile   `json:"profiles"`
	ProfilesError string               `json:"profilesError,omitempty"`
}

type localWiFiSnapshot struct {
	Time       string               `json:"time,omitempty"`
	TimeKind   string               `json:"timeKind,omitempty"`
	Available  bool                 `json:"available"`
	Elevated   bool                 `json:"elevated"`
	Interfaces []localWiFiInterface `json:"interfaces"`
	Note       string               `json:"note,omitempty"`
}

type localWiFiRevealRequest struct {
	InterfaceID string `json:"interfaceId"`
	ProfileName string `json:"profileName"`
	Elevate     bool   `json:"elevate"`
}

// A reveal response is intentionally not a member of localApplyResult or a
// diagnostic model. It exists only for this HTTP response/private pipe reply.
type localWiFiRevealResponse struct {
	InterfaceID    string `json:"interfaceId"`
	ProfileName    string `json:"profileName"`
	SSID           string `json:"ssid"`
	SSIDHex        string `json:"ssidHex"`
	Authentication string `json:"authentication"`
	Encryption     string `json:"encryption"`
	Scope          string `json:"scope"`
	KeyKind        string `json:"keyKind"`
	Secret         string `json:"secret"`
}

type localWiFiResult struct {
	Reply      *localWiFiRevealResponse `json:"reply,omitempty"`
	Error      string                   `json:"error,omitempty"`
	ErrorCode  string                   `json:"errorCode,omitempty"`
	CanElevate bool                     `json:"canElevate"`
}

type localWiFiBackend struct {
	read   func(context.Context) (localWiFiSnapshot, error)
	reveal func(context.Context, localWiFiRevealRequest) localWiFiResult
}

// Helper-only input. Flags, account and fingerprint are captured from the
// caller's visible profile before UAC. None is accepted from an HTTP request.
type localWiFiHelperRequest struct {
	InterfaceID   string `json:"interfaceId"`
	ProfileName   string `json:"profileName"`
	ProfileFlags  uint32 `json:"profileFlags"`
	RequesterSID  string `json:"requesterSid"`
	Fingerprint   string `json:"fingerprint"`
	MetadataKnown bool   `json:"metadataKnown"`
}

func validateLocalWiFiRequest(r localWiFiRevealRequest) error {
	if !validAdapterID(r.InterfaceID) {
		return errors.New("Select a wireless adapter before showing a stored key.")
	}
	// WlanGetProfile names are exact, case-sensitive UTF-16 strings. Keep
	// localized characters and whitespace; never turn a profile into shell text.
	if r.ProfileName == "" || !utf8.ValidString(r.ProfileName) || strings.ContainsRune(r.ProfileName, 0) || len(utf16.Encode([]rune(r.ProfileName))) > 255 {
		return errors.New("Select a saved Wi-Fi profile with a valid Windows profile name.")
	}
	return nil
}

func validateLocalWiFiHelper(r *localWiFiHelperRequest) error {
	if r == nil {
		return errors.New("Select exactly one Wi-Fi profile.")
	}
	if err := validateLocalWiFiRequest(localWiFiRevealRequest{InterfaceID: r.InterfaceID, ProfileName: r.ProfileName}); err != nil {
		return err
	}
	if r.ProfileFlags & ^uint32(3) != 0 || !validLocalRevision(r.Fingerprint) || !validLocalWiFiSID(r.RequesterSID) {
		return errors.New("The selected Wi-Fi profile identity is invalid. Refresh saved profiles.")
	}
	return nil
}

func validLocalWiFiSID(s string) bool {
	if !strings.HasPrefix(s, "S-1-") || len(s) < 7 || len(s) > 184 {
		return false
	}
	parts := strings.Split(s[4:], "-")
	if len(parts) < 2 || len(parts) > 16 {
		return false
	}
	for i, part := range parts {
		bits := 32
		if i == 0 {
			bits = 48
		}
		if part == "" || strings.HasPrefix(part, "+") {
			return false
		}
		if _, err := strconv.ParseUint(part, 10, bits); err != nil {
			return false
		}
	}
	return true
}

func writeLocalWiFiSecret(w http.ResponseWriter, reply *localWiFiRevealResponse) {
	data, err := json.Marshal(reply)
	if err != nil {
		jsonResponse(w, 500, localWiFiFailure("failed", "Unable to encode the Wi-Fi key response.", false))
		return
	}
	defer clear(data)
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}

func localWiFiFailure(code, message string, canElevate bool) localWiFiResult {
	return localWiFiResult{Error: message, ErrorCode: code, CanElevate: canElevate}
}

func localWiFiStatus(code string) int {
	switch code {
	case "validation":
		return http.StatusBadRequest
	case "permission":
		return http.StatusForbidden
	case "not_found":
		return http.StatusNotFound
	case "busy", "conflict", "no_key", "cancelled":
		return http.StatusConflict
	case "unsupported":
		return http.StatusNotImplemented
	case "unavailable":
		return http.StatusServiceUnavailable
	default:
		return http.StatusInternalServerError
	}
}

func (a *application) handleLocalWiFi(w http.ResponseWriter, r *http.Request, m *localToolsManager, path string) {
	w.Header().Set("Cache-Control", "no-store, no-cache, private, max-age=0")
	w.Header().Set("Pragma", "no-cache")
	w.Header().Set("Expires", "0")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	if path == "wifi" {
		if r.Method != http.MethodGet {
			methodError(w)
			return
		}
		if m.wifi.read == nil {
			jsonResponse(w, http.StatusOK, localWiFiSnapshot{Interfaces: []localWiFiInterface{}, Note: "Wi-Fi profiles are available in the Windows app."})
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 12*time.Second)
		defer cancel()
		stop := context.AfterFunc(m.ctx, cancel)
		defer stop()
		snapshot, err := m.wifi.read(ctx)
		if err != nil {
			jsonResponse(w, localWiFiStatus("unavailable"), localWiFiFailure("unavailable", err.Error(), false))
			return
		}
		if ctx.Err() != nil {
			return
		}
		stampLocalWiFiSnapshot(&snapshot, time.Now())
		jsonResponse(w, http.StatusOK, snapshot)
		return
	}
	if r.Method != http.MethodPost {
		methodError(w)
		return
	}
	var request localWiFiRevealRequest
	if err := decodeLocalJSON(w, r, &request, 4096); err != nil {
		jsonResponse(w, 400, localWiFiFailure("validation", err.Error(), false))
		return
	}
	if err := validateLocalWiFiRequest(request); err != nil {
		jsonResponse(w, 400, localWiFiFailure("validation", err.Error(), false))
		return
	}
	if m.wifi.reveal == nil {
		jsonResponse(w, 501, localWiFiFailure("unsupported", "Wi-Fi key viewing is available in the Windows app.", false))
		return
	}
	// Only this bounded, explicit reveal response can span UAC. No secret is
	// put into an operation cache or a polling endpoint to survive the request.
	if request.Elevate {
		if err := http.NewResponseController(w).SetWriteDeadline(time.Now().Add(125 * time.Second)); err != nil {
			jsonResponse(w, 500, localWiFiFailure("failed", "This HTTP host cannot keep a private Wi-Fi reveal request open for administrator approval.", false))
			return
		}
	}
	m.mu.Lock()
	if m.wifiBusy {
		m.mu.Unlock()
		jsonResponse(w, 409, localWiFiFailure("busy", "Another Wi-Fi key request is still in progress.", false))
		return
	}
	if m.ctx.Err() != nil || !a.homeCanConnect() {
		m.mu.Unlock()
		jsonResponse(w, 409, localWiFiFailure("cancelled", "The workspace is closing.", false))
		return
	}
	m.wifiBusy = true
	m.workers.Add(1)
	m.mu.Unlock()
	timeout := 20 * time.Second
	if request.Elevate {
		timeout = 2 * time.Minute
	}
	ctx, cancel := context.WithTimeout(r.Context(), timeout)
	defer cancel()
	stop := context.AfterFunc(m.ctx, cancel)
	defer stop()
	completed := make(chan localWiFiResult)
	go func() {
		defer func() { m.mu.Lock(); m.wifiBusy = false; m.mu.Unlock(); m.workers.Done() }()
		result := m.wifi.reveal(ctx, request)
		select {
		case completed <- result:
		case <-ctx.Done():
			if result.Reply != nil {
				result.Reply.Secret = ""
			}
		}
	}()
	var result localWiFiResult
	select {
	case result = <-completed:
	case <-ctx.Done():
		if r.Context().Err() == nil && m.ctx.Err() == nil && a.homeCanConnect() {
			jsonResponse(w, 409, localWiFiFailure("cancelled", "The Wi-Fi key request timed out. Refresh and try again.", false))
		}
		return
	}
	// Cancellation/navigation must never deliver a key into a stale page.
	if ctx.Err() != nil || !a.homeCanConnect() {
		if result.Reply != nil {
			result.Reply.Secret = ""
		}
		return
	}
	if result.Error != "" || result.Reply == nil {
		if result.Reply != nil {
			result.Reply.Secret = ""
			result.Reply = nil
		}
		if result.Error == "" {
			result = localWiFiFailure("failed", "Windows did not return a Wi-Fi key.", false)
		}
		jsonResponse(w, localWiFiStatus(result.ErrorCode), result)
		return
	}
	if !strings.EqualFold(result.Reply.InterfaceID, request.InterfaceID) || result.Reply.ProfileName != request.ProfileName {
		result.Reply.Secret = ""
		jsonResponse(w, 409, localWiFiFailure("conflict", "The selected Wi-Fi profile changed. Refresh saved profiles.", false))
		return
	}
	writeLocalWiFiSecret(w, result.Reply)
	result.Reply.Secret = ""
}
