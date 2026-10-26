package main

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"networktoolkit/internal/identity"
)

// The main router applies the same session, Host and Origin checks as every
// other local API before reaching these explicit, user-initiated operations.
func (a *application) handleVendors(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/api/vendors/status" {
		if r.Method != http.MethodGet {
			methodError(w)
			return
		}
		jsonResponse(w, http.StatusOK, identity.DatabaseStatus())
		return
	}
	if r.URL.Path != "/api/vendors/update" && r.URL.Path != "/api/vendors/import" && r.URL.Path != "/api/vendors/reset" {
		http.NotFound(w, r)
		return
	}
	if r.Method != http.MethodPost {
		methodError(w)
		return
	}
	ctx, cancel := context.WithCancel(r.Context())
	stop := context.AfterFunc(a.ctx, cancel)
	defer stop()
	defer cancel()
	var status identity.Status
	var err error
	errorCode := http.StatusInternalServerError
	switch r.URL.Path {
	case "/api/vendors/update":
		// Four bounded HTTPS downloads may exceed the normal short API deadline.
		_ = http.NewResponseController(w).SetWriteDeadline(time.Now().Add(65 * time.Second))
		status, err = identity.UpdateDatabase(ctx)
		errorCode = http.StatusBadGateway
	case "/api/vendors/import":
		if !strings.HasPrefix(strings.ToLower(r.Header.Get("Content-Type")), "application/json") {
			jsonError(w, http.StatusUnsupportedMediaType, "Send an application/json request containing the CSV text.")
			return
		}
		var request struct {
			CSV string `json:"csv"`
		}
		if err := decodeBody(w, r, 24<<20, &request); err != nil {
			jsonError(w, http.StatusBadRequest, err.Error())
			return
		}
		status, err = identity.ImportDatabaseCSV(ctx, []byte(request.CSV))
		errorCode = http.StatusBadRequest
	case "/api/vendors/reset":
		status, err = identity.ResetDatabase()
	}
	if err != nil {
		if errors.Is(err, identity.ErrBusy) {
			errorCode = http.StatusConflict
		} else if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			errorCode = http.StatusRequestTimeout
		}
		jsonResponse(w, errorCode, map[string]any{"ok": false, "error": err.Error(), "database": status})
		return
	}
	jsonResponse(w, http.StatusOK, map[string]any{"ok": true, "database": status})
}
