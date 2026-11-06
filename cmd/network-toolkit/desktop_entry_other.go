//go:build !linux

package main

import "net/http"

func (a *application) handleDesktopEntry(w http.ResponseWriter, r *http.Request) {
	jsonResponse(w, http.StatusOK, map[string]bool{"supported": false})
}
