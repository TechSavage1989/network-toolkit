package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func gateRequest(a *application, path string, cookie *http.Cookie) *httptest.ResponseRecorder {
	r := httptest.NewRequest(http.MethodGet, "http://"+a.host+path, nil)
	if cookie != nil {
		r.AddCookie(cookie)
	}
	w := httptest.NewRecorder()
	a.ServeHTTP(w, r)
	return w
}

func TestBrowserLaunchGateServesTokenOnlyToTheLaunchedWindow(t *testing.T) {
	a := testApplication(t)
	a.launchGate = newBrowserLaunchGate()

	// Any other local reader of the loopback port gets no token.
	if w := gateRequest(a, "/", nil); w.Code != http.StatusForbidden || strings.Contains(w.Body.String(), a.token) {
		t.Fatalf("page without the window cookie: %d, token leaked=%v", w.Code, strings.Contains(w.Body.String(), a.token))
	}
	if w := gateRequest(a, "/", &http.Cookie{Name: browserLaunchCookie, Value: strings.Repeat("0", 64)}); w.Code != http.StatusForbidden {
		t.Fatalf("a guessed cookie was accepted: %d", w.Code)
	}
	if w := gateRequest(a, "/launch?ticket="+strings.Repeat("a", 64), nil); w.Code != http.StatusForbidden || len(w.Result().Cookies()) != 0 {
		t.Fatalf("an unknown ticket was accepted: %d", w.Code)
	}

	launch := a.launchGate.Ticket()
	w := gateRequest(a, launch, nil)
	if w.Code != http.StatusSeeOther || w.Header().Get("Location") != "/" {
		t.Fatalf("launch link: %d to %q", w.Code, w.Header().Get("Location"))
	}
	cookies := w.Result().Cookies()
	if len(cookies) != 1 || cookies[0].Name != browserLaunchCookie || !cookies[0].HttpOnly || cookies[0].SameSite != http.SameSiteStrictMode || cookies[0].Path != "/" {
		t.Fatalf("window cookie is not HttpOnly, SameSite=Strict, Path=/: %#v", cookies)
	}
	if page := gateRequest(a, "/", cookies[0]); page.Code != http.StatusOK || !strings.Contains(page.Body.String(), a.token) {
		t.Fatalf("the launched window did not receive its page: %d", page.Code)
	}
	// A launch link works once.
	if again := gateRequest(a, launch, nil); again.Code != http.StatusForbidden || len(again.Result().Cookies()) != 0 {
		t.Fatalf("a used launch link was accepted again: %d", again.Code)
	}
	// The cookie alone cannot call the API.
	if api := gateRequest(a, "/api/state", cookies[0]); api.Code == http.StatusOK {
		t.Fatal("the window cookie authorized an API request without the session token")
	}
}

func TestBrowserLaunchTicketsExpireAndAreBounded(t *testing.T) {
	g := newBrowserLaunchGate()
	now := time.Unix(1_800_000_000, 0)
	g.now = func() time.Time { return now }
	first := strings.TrimPrefix(g.Ticket(), browserLaunchPath+"?ticket=")
	now = now.Add(browserLaunchLifetime + time.Second)
	if g.consume(first) {
		t.Fatal("an expired launch ticket was accepted")
	}
	var issued []string
	for i := 0; i < browserLaunchMaxOpen+3; i++ {
		issued = append(issued, strings.TrimPrefix(g.Ticket(), browserLaunchPath+"?ticket="))
	}
	if len(g.tickets) != browserLaunchMaxOpen {
		t.Fatalf("open tickets are not bounded: %d", len(g.tickets))
	}
	if !g.consume(issued[len(issued)-1]) {
		t.Fatal("the newest ticket was dropped")
	}
}

func TestNoLaunchGateKeepsDevelopmentAndWindowsBehaviour(t *testing.T) {
	a := testApplication(t)
	if w := gateRequest(a, "/", nil); w.Code != http.StatusOK {
		t.Fatalf("headless/development page changed without a gate: %d", w.Code)
	}
	if w := gateRequest(a, "/launch?ticket=x", nil); w.Code == http.StatusSeeOther {
		t.Fatal("the launch route exists without a gate")
	}
}
