package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"strconv"
	"strings"
	"time"

	"networktoolkit/internal/terminal"
)

// The app's Host, Origin, fetch-site and session-token checks run before all
// terminal routes. A session is opened only by an explicit POST, never by a
// list, event poll, saved profile, or startup request.
func (a *application) handleTerminals(w http.ResponseWriter, r *http.Request) {
	if a.terminals == nil {
		message := "The terminal service is unavailable."
		if a.terminalError != "" {
			message += " " + a.terminalError
		}
		jsonError(w, http.StatusServiceUnavailable, message)
		return
	}
	path := strings.TrimPrefix(r.URL.Path, "/api/terminals")
	switch path {
	case "":
		switch r.Method {
		case http.MethodGet:
			jsonResponse(w, http.StatusOK, map[string]any{"sessions": a.terminalListForRequest(r), "viewState": a.terminalViewStateForRequest(r)})
		case http.MethodPost:
			if !a.terminalCanConnect(r) {
				terminalHTTPError(w, terminal.ErrClosed)
				return
			}
			var req terminal.ConnectRequest
			if err := decodeTerminalBody(w, r, 512<<10, &req); err != nil {
				jsonError(w, http.StatusBadRequest, err.Error())
				return
			}
			// Decoding may wait for a slow caller. Check window ownership again
			// atomically with registration so close or return cannot orphan it.
			session, err := a.connectTerminalForRequest(r, req)
			// The terminal manager keeps a memory-only reconnect request. The
			// returned session, jobs, reports and persistent state never contain it.
			req.Password, req.PrivateKey, req.Passphrase = "", "", ""
			if err != nil {
				terminalHTTPError(w, err)
				return
			}
			jsonResponse(w, http.StatusCreated, map[string]any{"session": session, "viewState": a.terminalViewStateForRequest(r)})
		default:
			methodError(w)
		}
		return
	case "/events":
		if r.Method != http.MethodGet {
			methodError(w)
			return
		}
		after, wait, err := terminalEventQuery(r)
		if err != nil {
			jsonError(w, http.StatusBadRequest, err.Error())
			return
		}
		batch, err := a.terminals.Events(r.Context(), after, wait)
		if err != nil {
			terminalHTTPError(w, err)
			return
		}
		jsonResponse(w, http.StatusOK, struct {
			terminal.Batch
			ViewState terminalViewState `json:"viewState"`
		}{a.terminalBatchForRequest(r, batch), a.terminalViewStateForRequest(r)})
		return
	case "/local-directories":
		a.handleTerminalLocalDirectories(w, r)
		return
	case "/serial-ports":
		if r.Method != http.MethodGet {
			methodError(w)
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()
		ports, err := terminal.ListSerialPorts(ctx)
		if err != nil {
			terminalHTTPError(w, err)
			return
		}
		if ports == nil {
			ports = []terminal.SerialPort{}
		}
		jsonResponse(w, http.StatusOK, map[string]any{"ports": ports})
		return
	case "/known-hosts":
		switch r.Method {
		case http.MethodGet:
			hosts, err := a.terminals.KnownHosts()
			if err != nil {
				terminalHTTPError(w, err)
				return
			}
			jsonResponse(w, http.StatusOK, map[string]any{"hosts": hosts})
		case http.MethodDelete:
			var req struct {
				Host string `json:"host"`
			}
			if err := decodeTerminalBody(w, r, 2048, &req); err != nil {
				jsonError(w, http.StatusBadRequest, err.Error())
				return
			}
			if req.Host == "" || len(req.Host) > 512 {
				jsonError(w, http.StatusBadRequest, "Choose one remembered SSH host.")
				return
			}
			if err := a.terminals.RemoveKnownHost(req.Host); err != nil {
				terminalHTTPError(w, err)
				return
			}
			jsonResponse(w, http.StatusOK, map[string]bool{"ok": true})
		default:
			methodError(w)
		}
		return
	}
	parts := strings.Split(strings.TrimPrefix(path, "/"), "/")
	if len(parts) < 1 || len(parts) > 2 || parts[0] == "" || len(parts[0]) > 128 {
		http.NotFound(w, r)
		return
	}
	id := parts[0]
	if len(parts) == 1 {
		if r.Method != http.MethodDelete {
			methodError(w)
			return
		}
		if err := a.withTerminalOwner(r, id, "remove", func() error { return a.terminals.Remove(id) }); err != nil {
			terminalHTTPError(w, err)
			return
		}
		jsonResponse(w, http.StatusOK, map[string]bool{"ok": true})
		return
	}
	action := parts[1]
	switch action {
	case "input", "resize", "disconnect", "reconnect", "respond", "control", "rename", "line-ending":
	default:
		http.NotFound(w, r)
		return
	}
	if r.Method != http.MethodPost {
		methodError(w)
		return
	}
	var err error
	var reconnected terminal.SessionInfo
	switch action {
	case "rename":
		var req struct {
			Label string `json:"label"`
		}
		if err = decodeTerminalBody(w, r, 2048, &req); err == nil {
			err = a.withTerminalOwner(r, id, action, func() error { return a.terminals.Rename(id, req.Label) })
		}
	case "input":
		var req struct {
			Data   []byte `json:"data"`
			Origin string `json:"origin,omitempty"`
		}
		if err = decodeTerminalBody(w, r, 96<<10, &req); err == nil {
			if len(req.Data) == 0 || len(req.Data) > 64<<10 {
				err = errors.New("Terminal input must contain 1 to 65536 bytes.")
			} else {
				err = a.withTerminalOwner(r, id, action, func() error {
					if a.terminalAutomation != nil {
						if req.Origin == "pager" {
							if a.terminalAutomation.OwnsPager(id) {
								return nil
							}
						} else {
							a.terminalAutomation.ManualInput(id, req.Data)
						}
					}
					return a.terminals.Write(id, req.Data)
				})
			}
		}
	case "resize":
		var req struct {
			Cols int `json:"cols"`
			Rows int `json:"rows"`
		}
		if err = decodeTerminalBody(w, r, 2048, &req); err == nil {
			err = a.withTerminalOwner(r, id, action, func() error { return a.terminals.Resize(id, req.Cols, req.Rows) })
		}
	case "line-ending":
		var req struct {
			LineEnding string `json:"lineEnding"`
		}
		if err = decodeTerminalBody(w, r, 1024, &req); err == nil {
			err = a.withTerminalOwner(r, id, action, func() error { return a.terminals.SetLineEnding(id, req.LineEnding) })
		}
	case "disconnect":
		var req struct{}
		if err = decodeTerminalBody(w, r, 1024, &req); err == nil {
			err = a.withTerminalOwner(r, id, action, func() error { return a.terminals.Disconnect(id) })
		}
	case "reconnect":
		var req struct{}
		if err = decodeTerminalBody(w, r, 1024, &req); err == nil {
			ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
			defer cancel()
			err = a.withTerminalOwner(r, id, action, func() error {
				var reconnectErr error
				reconnected, reconnectErr = a.terminals.Reconnect(ctx, id)
				return reconnectErr
			})
		}
	case "respond":
		var req terminal.PromptReply
		if err = decodeTerminalBody(w, r, 64<<10, &req); err == nil {
			err = a.withTerminalOwner(r, id, action, func() error { return a.terminals.Reply(id, req) })
		}
	case "control":
		var req terminal.ControlRequest
		if err = decodeTerminalBody(w, r, 2048, &req); err == nil {
			if req.DTR == nil && req.RTS == nil && req.Break == nil {
				err = errors.New("Choose a serial line or break control.")
			} else {
				err = a.withTerminalOwner(r, id, action, func() error { return a.terminals.Control(id, req) })
			}
		}
	}
	if err != nil {
		terminalHTTPError(w, err)
		return
	}
	if action == "reconnect" {
		jsonResponse(w, http.StatusOK, map[string]any{"session": reconnected, "viewState": a.terminalViewStateForRequest(r)})
		return
	}
	jsonResponse(w, http.StatusOK, map[string]bool{"ok": true})
}

func terminalEventQuery(r *http.Request) (uint64, time.Duration, error) {
	var after uint64
	var err error
	if value := r.URL.Query().Get("after"); value != "" {
		after, err = strconv.ParseUint(value, 10, 64)
		if err != nil {
			return 0, 0, errors.New("The terminal event cursor must be a nonnegative integer.")
		}
	}
	waitMs := 15000
	if value := r.URL.Query().Get("wait"); value != "" {
		waitMs, err = strconv.Atoi(value)
		if err != nil || waitMs < 0 || waitMs > 20000 {
			return 0, 0, errors.New("Terminal event wait must be between 0 and 20000 milliseconds.")
		}
	}
	return after, time.Duration(waitMs) * time.Millisecond, nil
}

func decodeTerminalBody(w http.ResponseWriter, r *http.Request, limit int64, out any) error {
	mediaType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || mediaType != "application/json" {
		return errors.New("Send an application/json request.")
	}
	r.Body = http.MaxBytesReader(w, r.Body, limit)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err = decoder.Decode(out); err != nil {
		return fmt.Errorf("Invalid terminal request: %v", err)
	}
	if err = decoder.Decode(new(any)); !errors.Is(err, io.EOF) {
		return errors.New("Request must contain one JSON value.")
	}
	return nil
}

func terminalHTTPError(w http.ResponseWriter, err error) {
	if errors.Is(err, terminal.ErrAuthenticationRequired) {
		jsonResponse(w, http.StatusConflict, map[string]any{"error": err.Error(), "code": "authentication_required"})
		return
	}
	status := http.StatusBadRequest
	switch {
	case errors.Is(err, terminal.ErrNotFound):
		status = http.StatusNotFound
	case errors.Is(err, terminal.ErrQueueFull), errors.Is(err, terminal.ErrLimit):
		status = http.StatusTooManyRequests
	case errors.Is(err, terminal.ErrBusy), errors.Is(err, terminal.ErrNotConnected):
		status = http.StatusConflict
	case errors.Is(err, terminal.ErrClosed):
		status = http.StatusServiceUnavailable
	case errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
		status = http.StatusRequestTimeout
	}
	jsonError(w, status, err.Error())
}
