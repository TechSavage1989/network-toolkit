package main

import (
	"context"
	"net/http"
	"time"
)

// Windows setup (scripts/installer/windows.nsi) registers the Terminal for
// ssh, telnet and serial links under these per-user names. The user picks the
// default in Windows Settings > Default apps; the app only opens that page.
const (
	terminalLinkAppName      = "Network Toolkit Terminal"
	terminalLinkCapabilities = `Software\Network Toolkit\Terminal\Capabilities`
	terminalLinkProgID       = "NetworkToolkit.TerminalLink"
	terminalLinkDefaultApps  = "ms-settings:defaultapps?registeredAppUser=Network%20Toolkit%20Terminal"
)

type terminalLinkRegistration struct {
	// Supported is true where links can be registered (Windows).
	Supported bool `json:"supported"`
	// Registered is true when Windows setup registered the Terminal for links.
	Registered bool `json:"registered"`
	// Current is true when that registration opens this copy of the program.
	Current bool `json:"current"`
}

var (
	readTerminalLinkRegistration = terminalLinkRegistrationState
	openTerminalLinkDefaultApps  = openTerminalLinkSettings
)

// GET /api/terminal-links reports the registration; POST
// /api/terminal-links/default-apps opens Windows Default apps. Nothing here
// changes a registration or a default: Windows asks the user to choose.
func (a *application) handleTerminalLinks(w http.ResponseWriter, r *http.Request) {
	switch r.URL.Path {
	case "/api/terminal-links":
		if r.Method != http.MethodGet {
			methodError(w)
			return
		}
		jsonResponse(w, http.StatusOK, readTerminalLinkRegistration())
	case "/api/terminal-links/default-apps":
		if r.Method != http.MethodPost {
			methodError(w)
			return
		}
		if !readTerminalLinkRegistration().Supported {
			jsonError(w, http.StatusNotImplemented, "Choosing a default app for links is available in Windows.")
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()
		if err := openTerminalLinkDefaultApps(ctx); err != nil {
			jsonError(w, http.StatusServiceUnavailable, "Windows Default apps could not be opened: "+err.Error())
			return
		}
		jsonResponse(w, http.StatusOK, map[string]bool{"ok": true})
	default:
		http.NotFound(w, r)
	}
}
