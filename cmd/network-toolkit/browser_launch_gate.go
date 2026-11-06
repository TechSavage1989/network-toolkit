package main

import (
	"crypto/subtle"
	"net/http"
	"sync"
	"sync/atomic"
	"time"
)

// browserLaunchGate protects the HTML documents that carry the session token
// when the interface runs in an ordinary browser window (the Linux edition).
// On Windows the token reaches WebView2 through an in-memory host bootstrap and
// is never served by the loopback server. A browser cannot receive it that way,
// so the app opens a one-time launch link instead:
//
//	GET /launch?ticket=<single-use, short-lived>  ->  Set-Cookie (HttpOnly,
//	SameSite=Strict, Path=/) and a redirect to the page.
//
// Only a request carrying that cookie receives a page with the token. Any other
// local process, or another user on the same PC, that reads the loopback port
// gets a refusal page and no token. API requests still need the token in the
// X-Session-Token header, so the cookie alone cannot call the API.
type browserLaunchGate struct {
	mu      sync.Mutex
	cookie  string
	tickets map[string]time.Time
	now     func() time.Time
}

const (
	browserLaunchCookie   = "ntk_window"
	browserLaunchPath     = "/launch"
	browserLaunchLifetime = 2 * time.Minute
	browserLaunchMaxOpen  = 8
)

func newBrowserLaunchGate() *browserLaunchGate {
	return &browserLaunchGate{cookie: randomHex(32), tickets: map[string]time.Time{}, now: time.Now}
}

// Ticket returns a new single-use launch path ("/launch?ticket=...").
func (g *browserLaunchGate) Ticket() string {
	g.mu.Lock()
	defer g.mu.Unlock()
	now := g.now()
	for t, expires := range g.tickets {
		if !now.Before(expires) {
			delete(g.tickets, t)
		}
	}
	for len(g.tickets) >= browserLaunchMaxOpen {
		oldest, when := "", time.Time{}
		for t, expires := range g.tickets {
			if oldest == "" || expires.Before(when) {
				oldest, when = t, expires
			}
		}
		delete(g.tickets, oldest)
	}
	ticket := randomHex(32)
	g.tickets[ticket] = now.Add(browserLaunchLifetime)
	return browserLaunchPath + "?ticket=" + ticket
}

func (g *browserLaunchGate) consume(ticket string) bool {
	if len(ticket) != 64 {
		return false
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	for t, expires := range g.tickets {
		if subtle.ConstantTimeCompare([]byte(t), []byte(ticket)) == 1 {
			delete(g.tickets, t)
			return g.now().Before(expires)
		}
	}
	return false
}

// Allowed reports whether the request comes from a window opened by this app.
func (g *browserLaunchGate) Allowed(r *http.Request) bool {
	if g == nil {
		return true
	}
	c, err := r.Cookie(browserLaunchCookie)
	return err == nil && subtle.ConstantTimeCompare([]byte(c.Value), []byte(g.cookie)) == 1
}

// HandleLaunch serves /launch. It reports whether the request was handled.
func (g *browserLaunchGate) HandleLaunch(w http.ResponseWriter, r *http.Request) bool {
	if g == nil || r.URL.Path != browserLaunchPath {
		return false
	}
	if r.Method != http.MethodGet {
		methodError(w)
		return true
	}
	if !g.consume(r.URL.Query().Get("ticket")) {
		writeBrowserLaunchRefusal(w, "This launch link has already been used or has expired.")
		return true
	}
	http.SetCookie(w, &http.Cookie{Name: browserLaunchCookie, Value: g.cookie, Path: "/", HttpOnly: true, SameSite: http.SameSiteStrictMode})
	http.Redirect(w, r, "/", http.StatusSeeOther)
	return true
}

// Refuse writes the page shown to a request without the window cookie.
func (g *browserLaunchGate) Refuse(w http.ResponseWriter) {
	writeBrowserLaunchRefusal(w, "This page opens only in the window that Network Toolkit started.")
}

func writeBrowserLaunchRefusal(w http.ResponseWriter, reason string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusForbidden)
	_, _ = w.Write([]byte(`<!doctype html><meta charset="utf-8"><title>Network Toolkit</title><body style="font:15px system-ui,sans-serif;margin:3rem;max-width:40rem;color:#1f2937"><h1 style="font-size:1.3rem">Open Network Toolkit from its launcher</h1><p>` + reason + `</p><p>Start Network Toolkit again (or choose it in your applications menu) and it opens its own window.</p></body>`))
}

// appUIHostOverride records how a Linux window was actually opened
// ("linux-window" or "browser-tab"); the status API reports it so the page
// can offer Quit Network Toolkit where a browser tab cannot close the app.
var appUIHostOverride atomic.Pointer[string]

func setAppUIHost(host string) { appUIHostOverride.Store(&host) }

func (a *application) currentUIHost() string {
	if host := appUIHostOverride.Load(); host != nil {
		return *host
	}
	return a.uiHost
}

// lastPageRequest is the time of the latest authorized API request from a
// page. The Linux window launcher uses it to tell a closed window from one
// the browser handed to another of its processes.
var lastPageRequest atomic.Int64
