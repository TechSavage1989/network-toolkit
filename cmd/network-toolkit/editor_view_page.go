package main

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
)

// serveEditorView renders only the exact private, scoped editor document. The
// native host injects the capability; HTML and URLs never contain document data.
func (a *application) serveEditorView(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodError(w)
		return
	}
	id := strings.TrimPrefix(r.URL.Path, "/editor-view/")
	if len(id) != 32 || strings.Trim(id, "0123456789abcdef") != "" {
		http.NotFound(w, r)
		return
	}
	if a.editorViews == nil {
		http.NotFound(w, r)
		return
	}
	token, ok := a.editorViews.token(id)
	if !ok {
		jsonError(w, http.StatusGone, "This editor window is no longer available.")
		return
	}
	if _, ok = a.editorViews.info(id); !ok {
		http.NotFound(w, r)
		return
	}
	b, err := web.ReadFile("web/index.html")
	if err != nil {
		jsonError(w, 500, "Interface unavailable.")
		return
	}
	bootstrap, _ := json.Marshal(map[string]string{"id": id, "ownerInstanceId": a.windowInstanceID})
	html := strings.ReplaceAll(string(b), "__SESSION_TOKEN__", httpBootstrapToken(token))
	html = strings.ReplaceAll(html, "__WINDOW_MODE__", "editor")
	html = strings.ReplaceAll(html, "__EDITOR_VIEW__", string(bootstrap))
	html = strings.ReplaceAll(html, "__TERMINAL_VIEW__", "null")
	html = embeddedAssetPaths(html)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	_, _ = io.WriteString(w, html)
}
