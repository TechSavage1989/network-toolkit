package main

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"networktoolkit/internal/terminal"
)

func editorLifecycleChild(a *application, id, state string) {
	a.editorViews.mu.Lock()
	a.editorViews.views[id] = &editorWindow{info: editorViewInfo{ID: id, State: state}, token: "synthetic-editor-capability", lastPulse: time.Now()}
	a.editorViews.mu.Unlock()
}

func editorLifecycleRun(t *testing.T, a *application) <-chan struct{} {
	t.Helper()
	done := make(chan struct{})
	dataDir := t.TempDir()
	go func() {
		defer close(done)
		a.runPrimaryLifecycle("http://"+a.host+"/", true, dataDir, "Synthetic editor lifecycle")
	}()
	t.Cleanup(func() {
		a.stop()
		select {
		case <-done:
		case <-time.After(3 * time.Second):
			t.Error("primary lifecycle did not stop during cleanup")
		}
	})
	viewTestWait(t, func() bool { a.primaryMu.Lock(); defer a.primaryMu.Unlock(); return a.primaryCancel != nil })
	return done
}

func TestEditorLifecycleEmptyChildKeepsBackendUntilProcessExit(t *testing.T) {
	for _, state := range []string{"opening", "ready", "returned"} {
		t.Run(state, func(t *testing.T) {
			a := testApplication(t)
			editorLifecycleChild(a, "editor-child", state)
			done := editorLifecycleRun(t, a)
			if err := a.editorViews.beginHomeClose(); err != nil {
				t.Fatal(err)
			}
			a.beginPrimaryClose()
			if a.shutdownRequested.Load() {
				t.Fatal("editor child was ignored by shutdown decision")
			}
			a.closePrimary()
			viewTestWait(t, func() bool { return !a.views.state().HomePresent })
			if a.ctx.Err() != nil || !a.hasSecondaryViews() {
				t.Fatal("closing home discarded an editor process")
			}
			a.editorViews.mu.Lock()
			present := a.editorViews.homePresent
			a.editorViews.mu.Unlock()
			if present {
				t.Fatal("confirmed home close was not reflected in editor registry")
			}
			// Renderer release is not process-exit evidence, even for an empty window.
			if err := a.editorViews.release("editor-child"); err != nil {
				t.Fatal(err)
			}
			if !a.hasSecondaryViews() {
				t.Fatal("renderer release prematurely stopped owner")
			}
			a.editorViews.processExited("editor-child")
			select {
			case <-done:
			case <-time.After(3 * time.Second):
				t.Fatal("last editor exit did not wake and stop the waiting owner")
			}
			if a.ctx.Err() == nil {
				t.Fatal("owner remained running with no windows or recovery")
			}
		})
	}
}

func TestEditorLifecycleExitedDocumentReopensHomeForRecovery(t *testing.T) {
	a := testApplication(t)
	editorLifecycleChild(a, "editor-child", "ready")
	a.editorViews.mu.Lock()
	a.editorViews.documents["synthetic-document"] = &editorDocumentGrant{snapshot: editorDocumentSnapshot{UID: "synthetic-document", Binding: editorBinding{Kind: "localDraft"}, Content: "synthetic unsaved text"}, owner: "editor-child", lease: 2}
	a.editorViews.mu.Unlock()
	editorLifecycleRun(t, a)
	if err := a.editorViews.beginHomeClose(); err != nil {
		t.Fatal(err)
	}
	a.beginPrimaryClose()
	a.closePrimary()
	viewTestWait(t, func() bool { return !a.views.state().HomePresent })
	a.editorViews.processExited("editor-child")
	viewTestWait(t, func() bool { return a.views.state().HomePresent })
	if a.ctx.Err() != nil || a.shutdownRequested.Load() {
		t.Fatal("recoverable document was mistaken for final shutdown")
	}
	a.editorViews.mu.Lock()
	defer a.editorViews.mu.Unlock()
	if !a.editorViews.homePresent || !a.editorViews.recovery["synthetic-document"] || a.editorViews.documents["synthetic-document"].snapshot.Content != "synthetic unsaved text" {
		t.Fatal("reopened home lost its recovery checkpoint")
	}
}

func TestEditorLifecycleKeepsRecoveryAndInflightOrphanTransports(t *testing.T) {
	for _, kind := range []string{"recovery", "orphan"} {
		t.Run(kind, func(t *testing.T) {
			a := testApplication(t)
			var keptClosed, unusedClosed atomic.Int32
			a.browserConnections = map[string]*browserFileConnection{
				"kept":   {client: &terminal.FileConnection{}, cancel: func() { keptClosed.Add(1) }},
				"unused": {client: &terminal.FileConnection{}, cancel: func() { unusedClosed.Add(1) }},
			}
			d := &editorDocumentGrant{snapshot: editorDocumentSnapshot{UID: "synthetic-document", Binding: editorBinding{Kind: "sshConnection", ConnectionID: "kept"}}, lease: 2}
			a.editorViews.mu.Lock()
			a.editorViews.documents["synthetic-document"] = d
			if kind == "recovery" {
				a.editorViews.recovery["synthetic-document"] = true
			} else {
				d.owner = "exited-child"
				d.orphan = true
				d.busy = 1
			}
			a.editorViews.mu.Unlock()
			a.beginPrimaryClose()
			if a.shutdownRequested.Load() {
				t.Fatal("recovery/orphan was ignored before close")
			}
			if !a.finishPrimaryClose() {
				t.Fatal("recovery/orphan did not retain backend")
			}
			if keptClosed.Load() != 0 || unusedClosed.Load() != 1 {
				t.Fatal("close retained the wrong standalone SSH transports")
			}
			if !a.hasSecondaryViews() {
				t.Fatal("recovery/orphan was omitted from waiting predicate")
			}
			a.closePrimary()
			if a.ctx.Err() != nil {
				t.Fatal("close without primary cancelled recovery/orphan owner")
			}
			a.closeBrowserConnections()
			if keptClosed.Load() != 1 || unusedClosed.Load() != 1 {
				t.Fatal("full final cleanup no longer closes all standalone transports exactly once")
			}
		})
	}
}

func TestEditorLifecycleRetainsOnlyDocumentAndDetachedTerminalSessions(t *testing.T) {
	a := testApplication(t)
	// Existing loopback-only fixture; no device, credential or external network.
	editorSession := viewTestConnected(t, a)
	detachedSession := viewTestConnected(t, a)
	homeSession := viewTestConnected(t, a)
	a.launchTerminalView = func(context.Context, terminalViewLaunch) (<-chan error, error) { return nil, nil }
	if _, err := a.views.create(detachedSession.ID, viewTestSnapshot(a, detachedSession)); err != nil {
		t.Fatal(err)
	}
	editorLifecycleChild(a, "editor-child", "ready")
	a.editorViews.mu.Lock()
	a.editorViews.documents["owned"] = &editorDocumentGrant{snapshot: editorDocumentSnapshot{Binding: editorBinding{Kind: "terminal", SessionID: editorSession.ID}}, owner: "editor-child"}
	a.editorViews.documents["home-only"] = &editorDocumentGrant{snapshot: editorDocumentSnapshot{Binding: editorBinding{Kind: "terminal", SessionID: homeSession.ID}}}
	a.editorViews.mu.Unlock()
	if err := a.editorViews.beginHomeClose(); err != nil {
		t.Fatal(err)
	}
	if err := a.editorViews.approveHomeClose(); err != nil {
		t.Fatal(err)
	}
	a.beginPrimaryClose()
	if !a.finishPrimaryClose() {
		t.Fatal("detached views did not retain owner")
	}
	for _, id := range []string{editorSession.ID, detachedSession.ID} {
		info, ok := a.terminalSession(id)
		if !ok || info.State != "connected" {
			t.Fatal("referenced transport was disconnected")
		}
	}
	viewTestWait(t, func() bool { info, ok := a.terminalSession(homeSession.ID); return ok && info.State == "closed" })
	a.editorViews.mu.Lock()
	_, exists := a.editorViews.documents["home-only"]
	a.editorViews.mu.Unlock()
	if exists {
		t.Fatal("ordinary home-only editor record survived confirmed home close")
	}
}

func TestEditorLifecycleNeverWaitsForRegistryWhileHoldingTerminalViews(t *testing.T) {
	for _, operation := range []string{"begin", "finish", "liveness"} {
		t.Run(operation, func(t *testing.T) {
			a := testApplication(t)
			a.editorViews.mu.Lock()
			done := make(chan struct{})
			go func() {
				defer close(done)
				switch operation {
				case "begin":
					a.beginPrimaryClose()
				case "finish":
					a.finishPrimaryClose()
				default:
					a.hasSecondaryViews()
				}
			}()
			// Whether or not the worker has reached the registry lock, the terminal
			// ownership lock must remain independently available throughout this wait.
			time.Sleep(10 * time.Millisecond)
			free := a.views.mu.TryLock()
			if free {
				a.views.mu.Unlock()
			}
			a.editorViews.mu.Unlock()
			select {
			case <-done:
			case <-time.After(time.Second):
				t.Fatal("lifecycle lock-order deadlock")
			}
			if !free {
				t.Fatal("lifecycle acquired terminal ownership before the editor registry")
			}
		})
	}
}

func TestEditorLifecyclePrimaryExitRequiresExplicitDiscardApproval(t *testing.T) {
	for _, approved := range []bool{false, true} {
		t.Run(map[bool]string{false: "unapproved-recover", true: "approved-discard"}[approved], func(t *testing.T) {
			a, m, _ := editorFixture(t)
			done := editorLifecycleRun(t, a)
			batch := editorFixtureBatch()
			if _, err := m.checkpoint("", batch, nil); err != nil {
				t.Fatal(err)
			}
			if approved {
				if err := m.beginHomeClose(); err != nil {
					t.Fatal(err)
				}
				if err := m.approveHomeClose(); err != nil {
					t.Fatal(err)
				}
			}
			a.beginPrimaryClose()
			a.closePrimary()
			if approved {
				select {
				case <-done:
				case <-time.After(3 * time.Second):
					t.Fatal("approved final close did not stop")
				}
				m.mu.Lock()
				remaining := len(m.documents)
				m.mu.Unlock()
				if remaining != 0 {
					t.Fatal("explicitly discarded home documents remained")
				}
			} else {
				viewTestWait(t, func() bool { return m.hasRecovery() && a.views.state().HomePresent })
				if a.ctx.Err() != nil || a.shutdownRequested.Load() {
					t.Fatal("unapproved native exit stopped recovery owner")
				}
				m.mu.Lock()
				d := m.documents[batch.Active]
				valid := d != nil && d.owner == "" && d.lease == 2 && d.snapshot.Content == batch.Documents[0].Content
				m.mu.Unlock()
				if !valid {
					t.Fatal("primary recovery lost last acknowledged document")
				}
			}
		})
	}
}
