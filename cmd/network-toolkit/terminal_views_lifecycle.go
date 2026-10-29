package main

import (
	"context"
	"networktoolkit/internal/terminal"
)

// A workspace owns its transports. Native windows are disposable views of that
// workspace, so closing the home window must not cancel a detached connection.
func (a *application) runPrimaryLifecycle(url string, noBrowser bool, dataDir, title string) {
	for a.ctx.Err() == nil {
		if a.editorViews != nil {
			a.editorViews.setHomePresent(true)
		}
		viewContext, cancel := context.WithCancel(a.ctx)
		a.primaryMu.Lock()
		a.primaryCancel = cancel
		a.primaryMu.Unlock()
		a.views.mu.Lock()
		a.views.homePresent, a.views.homeClosing = true, false
		select {
		case <-a.primaryWake:
		default:
		}
		a.views.notifyLocked()
		a.views.mu.Unlock()
		runLifecycle(viewContext, url, a.token, cancel, noBrowser, dataDir, title, a.windowActivate, a.beginPrimaryClose, a.updateStartupReady)
		a.finishExplorerOwner("") // confirmed native host return; no active OLE remains
		cancel()
		a.primaryMu.Lock()
		a.primaryCancel = nil
		a.primaryMu.Unlock()
		if a.ctx.Err() != nil {
			return
		}
		if !a.finishPrimaryClose() {
			a.stop()
			return
		}
		for a.ctx.Err() == nil {
			if a.views.hasReturned() || (a.editorViews != nil && a.editorViews.hasRecovery()) {
				break
			}
			if !a.hasSecondaryViews() {
				a.stop()
				return
			}
			select {
			case <-a.ctx.Done():
				return
			case <-a.primaryWake:
				goto reopen
			case <-a.views.changed:
			}
		}
	reopen:
	}
}

func (a *application) beginPrimaryClose() {
	// Registry methods may acquire transport locks; never call them with views.mu held.
	editorLive := a.editorViews != nil && a.editorViews.hasLiveViews()
	a.views.mu.Lock()
	a.views.homeClosing = true
	if len(a.views.views) == 0 && !editorLive {
		a.shutdownRequested.Store(true)
	}
	a.views.notifyLocked()
	a.views.mu.Unlock()
}

func (a *application) closePrimary() {
	a.primaryMu.Lock()
	cancel := a.primaryCancel
	a.primaryMu.Unlock()
	if cancel != nil {
		cancel()
	} else if !a.hasSecondaryViews() {
		a.stop()
	}
}

// A native editor process, recovery checkpoint or in-flight orphan operation is
// an owner too. Process-exit notifications wake the primary wait loop; a missed
// renderer heartbeat must never cause this predicate to discard its documents.
func (a *application) hasSecondaryViews() bool {
	editorLive := a.editorViews != nil && a.editorViews.hasLiveViews()
	return a.views.count() > 0 || editorLive
}

// Returns whether a reserved, detached, or returning view still needs its
// owner. Home-only connections and diagnostics close with their home window.
func (a *application) finishPrimaryClose() bool {
	keepEditors, keepBrowsers := map[string]bool{}, map[string]bool{}
	editorLive := false
	if a.editorViews != nil {
		// The primary close has been confirmed. Retain only document-bound
		// transports, not every connection ever opened by the home window.
		a.editorViews.setHomePresent(false)
		keepEditors, keepBrowsers = a.editorViews.retainedTransports()
		editorLive = a.editorViews.hasLiveViews()
	}
	a.views.mu.Lock()
	a.views.homePresent, a.views.homeClosing = false, false
	hasViews := len(a.views.views) > 0
	keep := make(map[string]bool, len(a.views.views)+len(keepEditors))
	for sessionID := range keepEditors {
		keep[sessionID] = true
	}
	for _, view := range a.views.views {
		for _, sessionID := range view.sessionIDs() {
			keep[sessionID] = true
		}
	}
	// Capture the exact home-only connections while holding the ownership
	// lock. A detached workspace may connect after this lock is released.
	var homeSessions []terminal.SessionInfo
	if a.terminals != nil {
		for _, session := range a.terminals.List() {
			if !keep[session.ID] {
				homeSessions = append(homeSessions, session)
			}
		}
	}
	a.views.notifyLocked()
	a.views.mu.Unlock()
	a.closeBrowserConnectionsExcept(keepBrowsers)
	if !hasViews && !editorLive {
		return false
	}
	for _, session := range homeSessions {
		_ = a.terminals.Disconnect(session.ID)
	}
	a.mu.RLock()
	for _, job := range a.jobs {
		if job.Status == "running" && job.cancel != nil {
			job.cancel()
		}
	}
	a.mu.RUnlock()
	return true
}

func (a *application) activatePrimary() {
	if a.ctx.Err() != nil || a.shutdownRequested.Load() {
		return
	}
	a.views.mu.Lock()
	wake := !a.views.homePresent || a.views.homeClosing
	if wake {
		select {
		case a.primaryWake <- struct{}{}:
		default:
		}
	}
	a.views.mu.Unlock()
	select {
	case a.windowActivate <- struct{}{}:
	default:
	}
}

func (a *application) homeCanConnect() bool {
	a.views.mu.Lock()
	defer a.views.mu.Unlock()
	return a.views.homePresent && !a.views.homeClosing && !a.shutdownRequested.Load() && a.ctx.Err() == nil
}

func (a *application) connectHomeTerminal(request terminal.ConnectRequest) (terminal.SessionInfo, error) {
	a.views.mu.Lock()
	defer a.views.mu.Unlock()
	if !a.views.homePresent || a.views.homeClosing || a.shutdownRequested.Load() || a.ctx.Err() != nil {
		return terminal.SessionInfo{}, terminal.ErrClosed
	}
	session, err := a.terminals.Connect(request)
	if err == nil {
		a.views.pruneWorkspaceHistoryLocked()
	}
	return session, err
}
