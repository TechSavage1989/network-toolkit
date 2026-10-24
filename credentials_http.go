package main

import (
	"errors"
	"net/http"
	"os"
	"strings"
)

func (a *application) handleCredentials(w http.ResponseWriter, r *http.Request) {
	if a.credentials == nil {
		jsonError(w, http.StatusServiceUnavailable, "The Windows-protected credential vault is unavailable.")
		return
	}
	suffix := strings.TrimPrefix(r.URL.Path, "/api/credentials")
	if suffix == "" || suffix == "/" {
		switch r.Method {
		case http.MethodGet:
			jsonResponse(w, http.StatusOK, map[string]any{"available": true, "entries": a.credentials.list(), "protection": credentialProtectionLabel()})
		case http.MethodPost:
			var input credentialRecord
			if err := decodeBody(w, r, 512<<10, &input); err != nil {
				jsonError(w, http.StatusBadRequest, err.Error())
				return
			}
			saved, err := a.credentials.put(input)
			if err != nil {
				jsonError(w, http.StatusBadRequest, err.Error())
				return
			}
			jsonResponse(w, http.StatusOK, map[string]any{"ok": true, "entry": saved})
		case http.MethodDelete:
			if err := a.credentials.clear(); err != nil {
				jsonError(w, http.StatusInternalServerError, "Credentials were not cleared: "+err.Error())
				return
			}
			jsonResponse(w, http.StatusOK, map[string]bool{"ok": true})
		default:
			methodError(w)
		}
		return
	}
	id := strings.Trim(strings.TrimPrefix(suffix, "/"), " ")
	if id == "" || strings.Contains(id, "/") {
		http.NotFound(w, r)
		return
	}
	switch r.Method {
	case http.MethodGet:
		item, ok := a.credentials.get(id)
		if !ok {
			jsonError(w, http.StatusNotFound, "Saved credential was not found.")
			return
		}
		jsonResponse(w, http.StatusOK, map[string]any{"id": item.ID, "name": item.Name, "username": item.Username, "kind": item.Kind, "secret": item.Secret, "updatedAt": item.UpdatedAt})
	case http.MethodDelete:
		if err := a.credentials.delete(id); err != nil {
			if errors.Is(err, os.ErrNotExist) {
				jsonError(w, http.StatusNotFound, "Saved credential was not found.")
			} else {
				jsonError(w, http.StatusInternalServerError, "Credential was not removed: "+err.Error())
			}
			return
		}
		jsonResponse(w, http.StatusOK, map[string]bool{"ok": true})
	default:
		methodError(w)
	}
}
