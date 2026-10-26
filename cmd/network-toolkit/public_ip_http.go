package main

import (
	"context"
	"errors"
	"net/http"
)

func (a *application) handlePublicIP(w http.ResponseWriter, r *http.Request) {
	// Independent terminals have no network overview. Scoped linked terminals
	// are already rejected by serveDetachedAPI before this handler is reached.
	if a.windowMode != "main" {
		http.NotFound(w, r)
		return
	}
	force := false
	switch r.URL.Path {
	case "/api/public-ip":
		if r.Method != http.MethodGet {
			methodError(w)
			return
		}
	case "/api/public-ip/refresh":
		if r.Method != http.MethodPost {
			methodError(w)
			return
		}
		force = true
	default:
		http.NotFound(w, r)
		return
	}
	if a.publicIP == nil {
		jsonError(w, http.StatusServiceUnavailable, "Public IP lookup is unavailable.")
		return
	}
	result, err := a.publicIP.get(r.Context(), force)
	if err != nil {
		if a.ctx.Err() != nil {
			jsonError(w, http.StatusServiceUnavailable, "The application is closing.")
		} else if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			jsonError(w, http.StatusRequestTimeout, "The public IP request was canceled.")
		} else {
			jsonError(w, http.StatusServiceUnavailable, "Public IP lookup is unavailable.")
		}
		return
	}
	jsonResponse(w, http.StatusOK, result)
}
