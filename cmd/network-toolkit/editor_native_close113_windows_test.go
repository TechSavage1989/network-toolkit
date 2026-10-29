//go:build windows

package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// This extends the existing real application/native-process fixture, not the
// small native-close handshake HTML fixture. All documents and bindings are
// synthetic; the deferred failing write is an in-memory test binding only.
type nativeEditorCloseTransferCheck struct {
	BeforeACK   func(destination string) error
	SourceExits bool
}

func (h *nativeEditorFixture) requestClose113(view, phase string) error {
	if err := h.capture(view, phase); err != nil {
		return err
	}
	h.mu.Lock()
	raw := append([]byte(nil), h.captures[len(h.captures)-1]...)
	h.mu.Unlock()
	var window struct {
		HWND  uint64 `json:"hwnd"`
		PID   int    `json:"pid"`
		Class string `json:"class"`
	}
	if err := json.Unmarshal(raw, &window); err != nil {
		return err
	}
	pid, _, err := h.identity(view)
	if err != nil {
		return err
	}
	if window.HWND == 0 || window.PID != pid || window.Class != "NetworkToolkit.WebView2.Window" {
		return errors.New("captured close target does not match exact fixture host")
	}
	command := exec.CommandContext(h.ctx, "powershell.exe", "-NoProfile", "-NonInteractive", "-File", filepath.Join(h.root, "scripts", "native-editor-close.ps1"), "-WindowHandle", strconv.FormatUint(window.HWND, 10), "-OwnerProcessID", strconv.Itoa(pid), "-Action", "RequestClose")
	output, err := command.CombinedOutput()
	if err != nil {
		return fmt.Errorf("exact native close request: %w: %s", err, output)
	}
	if !json.Valid(bytes.TrimSpace(output)) {
		return fmt.Errorf("native close action evidence was not JSON: %s", output)
	}
	return nil
}

func (h *nativeEditorFixture) waitClosed113(view string) error {
	h.mu.Lock()
	child := h.children[view]
	h.mu.Unlock()
	if child == nil {
		return errors.New("expected actual child process is absent")
	}
	select {
	case <-child.done:
		if child.err != nil {
			return fmt.Errorf("native child closed unsuccessfully: %w", child.err)
		}
		return nil
	case <-time.After(12 * time.Second):
		return errors.New("native child did not exit after acknowledged return")
	case <-h.ctx.Done():
		return h.ctx.Err()
	}
}

func (h *nativeEditorFixture) homeRemainsOpen113() error {
	h.app.editorViews.mu.Lock()
	closing, approved := h.app.editorViews.homeClosing, h.app.editorViews.homeCloseApproved
	h.app.editorViews.mu.Unlock()
	if closing || approved || h.app.shutdownRequested.Load() {
		return errors.New("refused/cancelled editor close changed backend lifetime")
	}
	return nil
}

func (h *nativeEditorFixture) waitPersistedClosePolicy113(enabled bool) error {
	deadline := time.Now().Add(8 * time.Second)
	for time.Now().Before(deadline) {
		raw, err := os.ReadFile(h.app.dataPath)
		var saved struct {
			EditorPreferences struct {
				AskSaveBeforeClose *bool `json:"askSaveBeforeClose"`
			} `json:"editorPreferences"`
		}
		if err == nil && json.Unmarshal(raw, &saved) == nil && saved.EditorPreferences.AskSaveBeforeClose != nil && *saved.EditorPreferences.AskSaveBeforeClose == enabled {
			return nil
		}
		select {
		case <-h.ctx.Done():
			return h.ctx.Err()
		case <-time.After(80 * time.Millisecond):
		}
	}
	return errors.New("explicit editor close policy was not persisted to the isolated fixture settings")
}

func (h *nativeEditorFixture) exerciseClose113() error {
	before, err := h.idle("")
	if err != nil {
		return err
	}
	if len(before.Documents) != 2 || !before.AskSaveBeforeClose {
		return errors.New("native close extension requires two dirty documents and default-enabled close preference")
	}
	if err = h.requestClose113("", "close113-before-enabled"); err != nil {
		return err
	}
	if _, err = h.waitState("", "production unsaved dialog after native WM_CLOSE", func(s nativeEditorSnapshot) bool { return s.CloseDialog }); err != nil {
		return err
	}
	if err = h.capture("", "close113-enabled-confirmation"); err != nil {
		return err
	}
	cancelled, err := h.command("", nativeEditorCommand{Action: "answer-close", Mode: "cancel"})
	if err != nil {
		return err
	}
	if err = nativeEditorSameDocuments(before, cancelled); err != nil {
		return fmt.Errorf("enabled native close cancellation: %w", err)
	}
	if err = h.homeRemainsOpen113(); err != nil {
		return err
	}
	// A real additional edit and existing Undo/Redo prove the cancelled native
	// window remains usable, rather than merely retaining a static document.
	if _, err = h.command("", nativeEditorCommand{Action: "edit-history"}); err != nil {
		return err
	}
	h.phases = append(h.phases, "113 default-enabled native WM_CLOSE shows production dirty confirmation; Cancel retains exact documents/history and backend, and editing/Undo/Redo continue")

	disabled, err := h.command("", nativeEditorCommand{Action: "close-preference", Enabled: false})
	if err != nil {
		return err
	}
	if disabled.AskSaveBeforeClose || disabled.CloseDialog {
		return errors.New("explicit Allow discard did not disable the close preference")
	}
	if err = h.waitPersistedClosePolicy113(false); err != nil {
		return err
	}
	discarded, err := h.command("", nativeEditorCommand{Action: "close-tab"})
	if err != nil {
		return err
	}
	if discarded.CloseDialog || len(discarded.Documents) != len(disabled.Documents)-1 {
		return errors.New("disabled close preference did not discard exactly the selected unsaved tab")
	}
	for _, d := range discarded.Documents {
		old, ok := nativeEditorDoc(disabled, d.UID)
		if !ok {
			return errors.New("tab close created an unexpected document")
		}
		if err = nativeEditorEquivalent(old, d); err != nil {
			return err
		}
	}
	h.phases = append(h.phases, "113 explicit persisted Allow discard closes only the selected dirty tab without a prompt; other document contents/history are retained")

	saving, err := h.command("", nativeEditorCommand{Action: "start-synthetic-save"})
	if err != nil {
		return err
	}
	saveDoc, ok := nativeEditorDoc(saving, saving.Active)
	if !ok || !saveDoc.Saving || !saveDoc.Dirty {
		return errors.New("actual Save did not enter pending synthetic write")
	}
	blocked, err := h.command("", nativeEditorCommand{Action: "close-tab"})
	if err != nil {
		return err
	}
	if blocked.CloseDialog || len(blocked.Documents) != len(saving.Documents) {
		return errors.New("disabled prompt bypassed active-save tab guard")
	}
	if err = h.requestClose113("", "close113-pending-save"); err != nil {
		return err
	}
	pendingSave, err := h.waitState("", "native window refuses active save without dirty confirmation", func(s nativeEditorSnapshot) bool {
		return !s.CloseDialog && !s.Window.Closing && strings.Contains(s.Window.Error, "pending")
	})
	if err != nil {
		return err
	}
	if err = nativeEditorSameDocuments(saving, pendingSave); err != nil {
		return err
	}
	if err = h.homeRemainsOpen113(); err != nil {
		return err
	}
	failed, err := h.command("", nativeEditorCommand{Action: "fail-synthetic-save"})
	if err != nil {
		return err
	}
	failedDoc, ok := nativeEditorDoc(failed, saveDoc.UID)
	if !ok || failedDoc.Saving || !failedDoc.SaveFailed || !failedDoc.Dirty || !strings.Contains(failed.Message, "Save failed:") {
		return errors.New("failed save was hidden or unsaved contents cleared")
	}
	if err = h.capture("", "close113-visible-save-failure"); err != nil {
		return err
	}
	warning, err := h.command("", nativeEditorCommand{Action: "close-tab"})
	if err != nil {
		return err
	}
	if !warning.CloseDialog || len(warning.Documents) != len(failed.Documents) {
		return errors.New("disabled preference hid failed-save close warning")
	}
	retained, err := h.command("", nativeEditorCommand{Action: "answer-close", Mode: "cancel"})
	if err != nil {
		return err
	}
	if err = nativeEditorSameDocuments(failed, retained); err != nil {
		return err
	}
	if _, err = h.command("", nativeEditorCommand{Action: "close-tab"}); err != nil {
		return err
	}
	if _, err = h.command("", nativeEditorCommand{Action: "answer-close", Mode: "accept"}); err != nil {
		return err
	}
	h.phases = append(h.phases, "113 disabled prompt cannot bypass an actual pending Save; synthetic write rejection stays visible and forces explicit close confirmation, whose Cancel preserves text")

	remaining, err := h.idle("")
	if err != nil {
		return err
	}
	if len(remaining.Documents) != 1 {
		return errors.New("native close preparation lost the remaining transferable document")
	}
	child, detached, err := h.transferAction("", "new", remaining.Documents, remaining.Active, func() error {
		_, e := h.command("", nativeEditorCommand{Action: "move", Mode: "all", Destination: "new"})
		return e
	}, nativeEditorCloseTransferCheck{BeforeACK: func(_ string) error {
		if e := h.requestClose113("", "close113-pending-transfer"); e != nil {
			return e
		}
		protected, e := h.waitState("", "native window refuses pending transfer", func(s nativeEditorSnapshot) bool {
			return !s.CloseDialog && !s.Window.Closing && strings.Contains(s.Window.Error, "pending")
		})
		if e != nil {
			return e
		}
		if len(protected.Documents) != 1 || !protected.Documents[0].Frozen {
			return errors.New("pending transfer close discarded or unfroze source")
		}
		return h.homeRemainsOpen113()
	}})
	if err != nil {
		return fmt.Errorf("disabled close during native transfer: %w", err)
	}
	if _, err = h.waitState(child, "child receives read-only global disabled close policy", func(s nativeEditorSnapshot) bool { return !s.AskSaveBeforeClose }); err != nil {
		return err
	}
	_, returned, err := h.transferAction(child, "main", detached.Documents, detached.Active, func() error { return h.requestClose113(child, "close113-child-before-return") }, nativeEditorCloseTransferCheck{SourceExits: true})
	if err != nil {
		return fmt.Errorf("actual child WM_CLOSE returns unsaved document through ACK: %w", err)
	}
	if len(returned.Documents) != 1 {
		return errors.New("native child close did not return exactly one unsaved document")
	}
	if err = h.homeRemainsOpen113(); err != nil {
		return err
	}
	if err = h.capture("", "close113-child-returned"); err != nil {
		return err
	}
	h.phases = append(h.phases, "113 disabled prompt cannot bypass a pending transfer; actual child WM_CLOSE keeps source frozen until main ACK, returns exact unsaved text/history/selection and only then exits")
	if _, err = h.command("", nativeEditorCommand{Action: "close-preference", Enabled: true}); err != nil {
		return err
	}
	if err = h.waitPersistedClosePolicy113(true); err != nil {
		return err
	}
	h.closePreferenceComplete = true
	return nil
}
