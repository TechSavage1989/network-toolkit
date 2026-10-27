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
	if r.Method != http.MethodGet {
		a.credentialChanges.Lock()
		defer a.credentialChanges.Unlock()
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
			saved, err := a.saveCredentialFromRequest(r, input)
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
	if strings.HasSuffix(suffix, "/bindings") {
		id := strings.TrimSuffix(strings.TrimPrefix(suffix, "/"), "/bindings")
		if id == "" || strings.Contains(id, "/") {
			http.NotFound(w, r)
			return
		}
		if r.Method != http.MethodPost {
			methodError(w)
			return
		}
		var input struct {
			Binding credentialBinding `json:"binding"`
		}
		if err := decodeBody(w, r, 8192, &input); err != nil {
			jsonError(w, 400, "Invalid credential destination.")
			return
		}
		binding, err := normalizeCredentialBinding(input.Binding)
		if err != nil {
			jsonError(w, 400, err.Error())
			return
		}
		item, found := a.credentials.get(id)
		if !found {
			jsonError(w, 404, "Saved credential was not found.")
			return
		}
		if !hasCredentialBinding(item.Bindings, binding) {
			kept := make([]credentialBinding, 0, len(item.Bindings)+1)
			for _, prior := range item.Bindings {
				if !sameCredentialTarget(prior, binding) {
					kept = append(kept, prior)
				}
			}
			item.Bindings = append(kept, binding)
			saved, err := a.saveCredentialFromRequest(r, item)
			if err != nil {
				jsonError(w, 400, err.Error())
				return
			}
			jsonResponse(w, 200, map[string]any{"ok": true, "entry": saved})
		} else {
			jsonResponse(w, 200, map[string]any{"ok": true, "entry": credentialMetadataFor(item)})
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
		jsonResponse(w, http.StatusOK, credentialMetadataFor(item))
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

// Called with credentialChanges held: no browser request can replace a record
// while its native destination confirmation is outstanding.
func (a *application) saveCredentialFromRequest(r *http.Request, input credentialRecord) (credentialMetadata, error) {
	var before credentialRecord
	if input.ID != "" {
		var found bool
		before, found = a.credentials.get(input.ID)
		if !found {
			return credentialMetadata{}, errors.New("Saved credential was removed. Add a new credential instead.")
		}
		if input.Kind == "" {
			input.Kind = before.Kind
		}
		if input.Secret == "" {
			if input.Kind != before.Kind {
				return credentialMetadata{}, errors.New("Enter a new secret when changing the credential type.")
			}
			input.Secret = before.Secret
		}
		if input.Bindings == nil {
			input.Bindings = before.Bindings
		}
	}
	clean, err := sanitizeCredentialRecord(input, input.ID)
	if err != nil {
		return credentialMetadata{}, err
	}
	clean.Bindings, err = a.hydrateCredentialBindings(r.Context(), before.Bindings, clean.Bindings)
	if err != nil {
		return credentialMetadata{}, err
	}
	if err := a.approveCredentialBindings(r.Context(), clean.Name, before.Bindings, clean.Bindings); err != nil {
		return credentialMetadata{}, err
	}
	if err := r.Context().Err(); err != nil {
		return credentialMetadata{}, err
	}
	return a.credentials.put(clean)
}
