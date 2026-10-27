package main

import (
	"net/http"
	"strings"
)

// Source HTML uses relative assets so offline calculators work from file URLs.
// Native main and detached routes resolve those same files at the engine root.
func embeddedAssetPaths(html string) string {
	return strings.NewReplacer("src=\"assets/", "src=\"/assets/", "href=\"assets/", "href=\"/assets/", "src=\"../assets/", "src=\"/assets/", "href=\"../assets/", "href=\"/assets/").Replace(html)
}

// Browser assets are compiled into the executable. The exact allowlist avoids
// exposing source, license manifests, local files, or directory listings through
// the local engine. ServeHTTP checks the request Host before reaching this code.
func serveWebAsset(w http.ResponseWriter, r *http.Request) {
	contentType := ""
	embeddedPath := "web" + r.URL.Path
	switch r.URL.Path {
	case "/assets/terminal-shell-syntax.js", "/assets/editor-languages.js", "/assets/editor-language-service.js", "/assets/editor-language-worker.js",
		"/assets/editor-vendor/languages.js", "/assets/editor-vendor/prism.js", "/assets/editor-vendor/prettier.js",
		"/assets/editor-vendor/prettier-babel.js", "/assets/editor-vendor/prettier-estree.js", "/assets/editor-vendor/prettier-typescript.js",
		"/assets/editor-vendor/prettier-html.js", "/assets/editor-vendor/prettier-postcss.js", "/assets/editor-vendor/prettier-markdown.js",
		"/assets/editor-vendor/prettier-yaml.js", "/assets/editor-vendor/prettier-graphql.js":
		contentType = "text/javascript; charset=utf-8"
	case "/assets/graph-navigator.js", "/assets/graph-history.js", "/assets/result-filters.js", "/assets/graph-workspace.js", "/assets/result-time.js", "/assets/result-timeline.js", "/assets/hosts-editor-model.js", "/assets/hosts-editor.js", "/assets/transfer-client-layout.js", "/assets/panel-controls.js", "/assets/input-clear.js", "/assets/value-copy.js":
		contentType = "text/javascript; charset=utf-8"
	case "/assets/graph-navigator.css", "/assets/result-filters.css", "/assets/graph-workspace.css", "/assets/result-timeline.css", "/assets/result-typography.css", "/assets/hosts-editor.css", "/assets/workspace-layout.css", "/assets/transfer-client-layout.css", "/assets/panel-controls.css", "/assets/input-clear.css", "/assets/value-copy.css", "/assets/responsive-controls.css", "/assets/text-containment.css":
		contentType = "text/css; charset=utf-8"
	case "/assets/Network_Toolkit_Icon.svg":
		contentType = "image/svg+xml"
		embeddedPath = "assets/Network_Toolkit_Icon.svg"
	case "/favicon.ico", "/assets/Network_Toolkit_Icon.ico":
		contentType = "image/x-icon"
		embeddedPath = "assets/Network_Toolkit_Icon.ico"
	case "/assets/xterm.js", "/assets/addon-fit.js", "/assets/addon-search.js", "/assets/addon-serialize.js", "/assets/terminal-state.js", "/assets/terminal-highlight.js", "/assets/subnet-planner.js", "/assets/report-export.js", "/assets/scan-scopes.js", "/assets/discovery-workspace.js", "/assets/source-interface.js", "/assets/result-details.js", "/assets/continuous-sampling.js", "/assets/scan-routes.js", "/assets/app-navigation.js", "/assets/app-updater.js", "/assets/app-update-notes.js", "/assets/configuration-layout.js", "/assets/terminal-interaction.js", "/assets/terminal-copy.js", "/assets/terminal-extras.js", "/assets/terminal-macro-model.js", "/assets/terminal-macros.js", "/assets/terminal-appearance.js", "/assets/transfer-server.js", "/assets/transfer-explorer.js", "/assets/remote-browser.js", "/assets/tftp-connections.js", "/assets/remote-drag.js", "/assets/file-editor.js", "/assets/terminal-output-archive.js", "/assets/terminal-input.js", "/assets/terminal-tabs.js", "/assets/saved-session-folders.js", "/assets/session-sidebar.js", "/assets/terminal-sftp-sidebar.js", "/assets/packet-capture.js", "/assets/credential-manager.js":
		contentType = "text/javascript; charset=utf-8"
	case "/assets/xterm.css", "/assets/discovery-workspace.css", "/assets/source-interface.css", "/assets/result-details.css", "/assets/scan-routes.css", "/assets/app-navigation.css", "/assets/configuration-layout.css", "/assets/configuration-surfaces.css", "/assets/terminal-interaction.css", "/assets/terminal-copy.css", "/assets/terminal-macros.css", "/assets/session-sidebar.css", "/assets/terminal-sftp-sidebar.css", "/assets/packet-capture.css", "/assets/diagnostic-colors.css", "/assets/transfer-server.css", "/assets/transfer-explorer.css", "/assets/remote-browser.css", "/assets/file-editor.css":
		contentType = "text/css; charset=utf-8"
	default:
		http.NotFound(w, r)
		return
	}
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		methodError(w)
		return
	}
	b, err := web.ReadFile(embeddedPath)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, "Interface asset unavailable.")
		return
	}
	w.Header().Set("Content-Type", contentType)
	if r.Method == http.MethodGet {
		_, _ = w.Write(b)
	}
}
