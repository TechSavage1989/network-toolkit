package main

import (
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"os"
	"strings"
	"time"
)

// Bringing the receiving window to the front (owner decision, v1.21.50): when
// Send to delivers text, or Edit opens a remote file, the window that takes it
// comes forward, the way peer send raises the other standalone app
// (peerAllowForeground before the request, activatePrimary in the receiver).
//
// Windows lets a process bring its own window to the front only while it may
// set the foreground window (it is the foreground process, or such a process
// allowed it with AllowSetForegroundWindow). The windows of one engine are
// several processes: the engine with its primary window, and one separate
// program per detached window (edition.go), which the engine started and whose
// process identifiers it keeps while they run (trackWindowProcess).
//
// After a request is queued for a window, and never for a refusal:
//   - The engine allows the receiving child's process (it holds the right when
//     its own window sent the text, or when the other standalone app allowed it
//     for a peer send) and tells that child to raise its window.
//   - When the sender is a detached window (then the foreground process), that
//     child is told to allow the receiving process: the child, or the engine
//     for its primary window.
//   - The primary window is raised with activatePrimary, and once more shortly
//     after so the sending child's permission, which arrives through its own
//     request, is used; a raised window that is already in front stays there.
//
// A child learns this through GET /api/window-send/activation with its own view
// token (a long poll, never from another window, carrying only "raise" and one
// process identifier). The native host then restores and activates its window
// (desktop.Config.Activate: SW_RESTORE or SW_SHOW, SetForegroundWindow, or a
// taskbar flash when Windows refuses). Elsewhere nothing is allowed and no
// child window exists.
const (
	windowRaiseWait  = 20 * time.Second       // one activation long poll; under the server's write timeout
	windowRaiseFresh = 3 * time.Second        // a raise not collected by then is dropped
	windowRaiseRetry = 450 * time.Millisecond // the second raise, after the sender's permission
)

var (
	// windowAllowForeground is peerAllowForeground; tests record the calls.
	windowAllowForeground = peerAllowForeground
	// windowRaiseLater runs the second raise; tests run it at once.
	windowRaiseLater = func(f func()) { time.AfterFunc(windowRaiseRetry, f) }
)

// windowSendActivation is what one child window has still to collect.
type windowSendActivation struct {
	raise   bool
	raiseAt time.Time
	allow   int // a process this child is to allow to come to the front
	wake    chan struct{}
}

func (s *windowSendState) activationLocked(key string) *windowSendActivation {
	if s.activations == nil {
		s.activations = map[string]*windowSendActivation{}
	}
	act := s.activations[key]
	if act == nil {
		act = &windowSendActivation{wake: make(chan struct{})}
		s.activations[key] = act
	}
	return act
}

func (s *windowSendState) signalLocked(key string, raise bool, allow int, now time.Time) {
	act := s.activationLocked(key)
	if raise {
		act.raise, act.raiseAt = true, now
	}
	if allow > 0 {
		act.allow = allow
	}
	close(act.wake)
	act.wake = make(chan struct{})
}

// trackWindowProcess records the process of a detached window the engine
// starts, for as long as it runs. launch reports the process identifier once
// the program has started; the returned channel is the launch's own.
func (a *application) trackWindowProcess(key string, launch func(started func(int)) (<-chan error, error)) (<-chan error, error) {
	forget := func() {
		a.windowSend.mu.Lock()
		delete(a.windowSend.pids, key)
		delete(a.windowSend.activations, key)
		a.windowSend.mu.Unlock()
	}
	done, err := launch(func(pid int) {
		if pid < 1 {
			return
		}
		a.windowSend.mu.Lock()
		if a.windowSend.pids == nil {
			a.windowSend.pids = map[string]int{}
		}
		a.windowSend.pids[key] = pid
		a.windowSend.mu.Unlock()
	})
	if err != nil || done == nil {
		if err != nil {
			forget()
		}
		return done, err
	}
	out := make(chan error, 1)
	go func() {
		err, ok := <-done
		forget() // the process has ended: its identifier may be reused
		if ok {
			out <- err
		}
		close(out)
	}()
	return out, nil
}

func windowSendChild(key string) bool {
	return strings.HasPrefix(key, windowSendTerminal) || strings.HasPrefix(key, windowSendEditor)
}

// windowSendRaise brings the window that has just been given text or a file to
// the front. Callers call it only after the request was queued for that window.
// from is the sending window ("peer" for the other standalone app).
func (a *application) windowSendRaise(from, to string) {
	now := windowSendNow()
	pid := 0
	if to == windowSendMain {
		pid = os.Getpid()
	} else if windowSendChild(to) {
		a.windowSend.mu.Lock()
		pid = a.windowSend.pids[to]
		a.windowSend.signalLocked(to, true, 0, now)
		a.windowSend.mu.Unlock()
		if pid > 0 {
			windowAllowForeground(pid)
		}
	} else {
		return
	}
	a.windowSendAllowFrom(from, pid)
	if to == windowSendMain {
		a.activatePrimary()
		windowRaiseLater(func() {
			if a.windowSendAlive(windowSendMain) {
				a.activatePrimary()
			}
		})
	}
}

// windowSendAllowFrom tells a sending child window, the foreground process
// while its user sends, to allow pid to come to the front.
func (a *application) windowSendAllowFrom(from string, pid int) {
	if pid < 1 || !windowSendChild(from) {
		return
	}
	a.windowSend.mu.Lock()
	defer a.windowSend.mu.Unlock()
	if a.windowSend.pids[from] == pid {
		return // the sender itself
	}
	a.windowSend.signalLocked(from, false, pid, windowSendNow())
}

// windowSendActivationWait is a child window's long poll: it returns what that
// window has to do, or nothing after windowRaiseWait.
func (a *application) windowSendActivationWait(ctx context.Context, key string) (raise bool, allow int) {
	timer := time.NewTimer(windowRaiseWait)
	defer timer.Stop()
	for {
		a.windowSend.mu.Lock()
		act := a.windowSend.activationLocked(key)
		if act.raise && windowSendNow().Sub(act.raiseAt) > windowRaiseFresh {
			act.raise = false
		}
		if act.raise || act.allow > 0 {
			raise, allow = act.raise, act.allow
			act.raise, act.allow = false, 0
			a.windowSend.mu.Unlock()
			return raise, allow
		}
		wake := act.wake
		a.windowSend.mu.Unlock()
		select {
		case <-wake:
		case <-ctx.Done():
			return false, 0
		case <-a.ctx.Done():
			return false, 0
		case <-timer.C:
			return false, 0
		}
	}
}

// ---- Child window side ----

// watchWindowActivation runs in a detached window's process while its native
// window is open: it asks the owner engine what to do and allows or raises.
// A failed request is retried after a pause; it ends with ctx.
func watchWindowActivation(ctx context.Context, ownerHost, token string, activate chan<- struct{}) {
	client := &http.Client{
		Transport: &http.Transport{Proxy: nil, DialContext: (&net.Dialer{Timeout: 3 * time.Second}).DialContext, MaxResponseHeaderBytes: 16 << 10, MaxIdleConnsPerHost: 1, IdleConnTimeout: 10 * time.Second},
		Timeout:   windowRaiseWait + 10*time.Second,
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	defer client.CloseIdleConnections()
	signal := func() {
		select {
		case activate <- struct{}{}:
		default:
		}
	}
	for ctx.Err() == nil {
		raise, allow, ok := windowActivationRequest(ctx, client, ownerHost, token)
		if !ok {
			select {
			case <-ctx.Done():
				return
			case <-time.After(2 * time.Second):
			}
			continue
		}
		if allow > 0 {
			peerAllowForeground(allow)
		}
		if raise {
			signal()
			// Once more, after a sending window's permission has arrived.
			time.AfterFunc(windowRaiseRetry, func() {
				if ctx.Err() == nil {
					signal()
				}
			})
		}
	}
}

func windowActivationRequest(ctx context.Context, client *http.Client, ownerHost, token string) (raise bool, allow int, ok bool) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://"+ownerHost+windowSendPrefix+"activation", nil)
	if err != nil {
		return false, 0, false
	}
	request.Header.Set("X-Session-Token", token)
	response, err := client.Do(request)
	if err != nil {
		return false, 0, false
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return false, 0, false
	}
	var reply struct {
		Raise bool `json:"raise"`
		Allow int  `json:"allow"`
	}
	if json.NewDecoder(io.LimitReader(response.Body, 4096)).Decode(&reply) != nil || reply.Allow < 0 {
		return false, 0, false
	}
	return reply.Raise, reply.Allow, true
}
