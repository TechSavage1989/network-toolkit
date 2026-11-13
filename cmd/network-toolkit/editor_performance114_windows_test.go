//go:build windows

package main

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"
)

// This measures real separate-process WebView2 transfers without the deliberate
// acceptance hold used by the security fixture. Automation and hashes are test-only.
func (h *nativeEditorFixture) exercisePerformance114() (failure error) {
	report := map[string]any{"passed": false, "syntheticOnly": true, "actualNativeWindows": true, "productionSeparateProcess": true, "measure": "button action until destination editable ACK and source release; test command round-trip included"}
	records := []map[string]any{}
	defer func() {
		report["records"] = records
		report["passed"] = failure == nil
		if failure != nil {
			report["error"] = failure.Error()
		}
		raw, _ := json.MarshalIndent(report, "", "  ")
		_ = os.WriteFile(filepath.Join(h.output, "native-performance114.json"), raw, 0600)
	}()
	size := 4 << 20
	if value := os.Getenv("NETWORK_TOOLKIT_NATIVE_PERFORMANCE_BYTES"); value != "" {
		n, err := strconv.Atoi(value)
		if err != nil || n <= 512<<10 || n > 16<<20 {
			return fmt.Errorf("synthetic performance size must exercise disk promotion and be at most 16 MiB")
		}
		size = n
	}
	initial, err := h.command("", nativeEditorCommand{Action: "performance-seed", Size: size})
	if err != nil {
		return err
	}
	if len(initial.Documents) != 1 || !initial.Documents[0].Disk || !initial.Documents[0].CanUndo || initial.Documents[0].Content != "length:0" {
		return fmt.Errorf("expected one synthetic disk document with undo and no inline source text")
	}
	expected, err := h.command("", nativeEditorCommand{Action: "performance-digest"})
	if err != nil {
		return err
	}
	row := "SYNTHETIC PERFORMANCE114 ONLY\n" + strings.Repeat("x", 1000) + "\n"
	seed := strings.Repeat(row, (size+len(row)-1)/len(row))[:size]
	undoDigest := fmt.Sprintf("%x", sha256.Sum256([]byte(seed)))
	fullDigest := fmt.Sprintf("%x", sha256.Sum256([]byte(seed+"\nSYNTHETIC SECOND EDIT")))
	historyDigests := []string{undoDigest, fmt.Sprintf("%x", sha256.Sum256(nil)), undoDigest, fullDigest}
	if len(expected.Documents) != 1 || expected.Documents[0].Content != fullDigest || expected.Documents[0].View == nil ||
		expected.Documents[0].View.Start != 11 || expected.Documents[0].View.End != 29 || expected.Documents[0].View.Direction != "backward" {
		return fmt.Errorf("initial native disk edit or logical selection differs")
	}
	uid := initial.Documents[0].UID
	for repeat := 0; repeat < 3; repeat++ {
		child := ""
		for _, direction := range []string{"detach", "reattach"} {
			source, destination := "", "new"
			if direction == "reattach" {
				source, destination = child, "main"
			}
			started := time.Now()
			if _, err := h.command(source, nativeEditorCommand{Action: "performance-move", Mode: "all", Destination: destination}); err != nil {
				return err
			}
			deadline := time.Now().Add(30 * time.Second)
			target := ""
			if destination == "new" {
				for time.Now().Before(deadline) {
					h.app.editorViews.mu.Lock()
					d := h.app.editorViews.documents[uid]
					if d != nil && d.pending != "" {
						target = h.app.editorViews.transfers[d.pending].destination
					} else if d != nil && d.owner != "" {
						target = d.owner
					}
					h.app.editorViews.mu.Unlock()
					if target != "" {
						break
					}
					time.Sleep(10 * time.Millisecond)
				}
				if target == "" {
					state, err := h.command(source, nativeEditorCommand{Action: "performance-state"})
					if err != nil {
						return fmt.Errorf("new native destination missing; source state: %w", err)
					}
					report["sourceLastState"] = state
					return fmt.Errorf("new native destination missing: %s", state.Window.Error)
				}
				child = target
			}
			// The precondition of the acknowledgement is the new window's page
			// having started (bounded by the move's own open limit). Count the
			// ownership deadline from there; elapsed above still measures from
			// the button, window start included.
			if err := h.awaitPage(target); err != nil {
				return fmt.Errorf("native %s: %w", direction, err)
			}
			deadline = time.Now().Add(30 * time.Second)
			for {
				state, err := h.command(target, nativeEditorCommand{Action: "performance-state"})
				if err != nil {
					return err
				}
				sourceState, err := h.command(source, nativeEditorCommand{Action: "performance-state"})
				if err != nil {
					return err
				}
				doc, found := nativeEditorDoc(state, uid)
				if found && !doc.Frozen && !doc.Staged && !state.Textarea.Disabled && state.Window.Owned[uid] > 0 && len(sourceState.Documents) == 0 && sourceState.Window.Outgoing == 0 {
					break
				}
				if time.Now().After(deadline) {
					report["sourceLastState"] = sourceState
					report["destinationLastState"] = state
					return fmt.Errorf("native %s ownership timed out: source=%s destination=%s", direction, sourceState.Window.Error, state.Window.Error)
				}
				time.Sleep(20 * time.Millisecond)
			}
			elapsed := time.Since(started).Seconds() * 1000
			actual, err := h.command(target, nativeEditorCommand{Action: "performance-digest"})
			if err != nil {
				return err
			}
			if len(actual.Documents) != 1 {
				return fmt.Errorf("unexpected destination document count")
			}
			if err := nativeEditorEquivalent(expected.Documents[0], actual.Documents[0]); err != nil {
				return fmt.Errorf("native exact text/history/position hashes: %w", err)
			}
			history, err := h.command(target, nativeEditorCommand{Action: "performance-history"})
			if err != nil {
				return err
			}
			if len(history.Documents) != 1 || !slices.Equal(history.HistoryDigests, historyDigests) {
				return fmt.Errorf("native disk undo or redo changed the expected text after %s", direction)
			}
			if history.Documents[0].View == nil || history.Documents[0].View.Start != 11 || history.Documents[0].View.End != 29 || history.Documents[0].View.Direction != "backward" {
				return fmt.Errorf("native disk history verification did not restore the logical selection")
			}
			expected = history
			records = append(records, map[string]any{"iteration": repeat + 1, "direction": direction, "encodedSyntheticBytes": size, "elapsedMs": elapsed, "sourceReleased": true, "exclusiveDestination": true, "exactTextUndoSelectionRetained": true})
			if repeat == 0 {
				if err := h.capture(target, "performance114-"+direction); err != nil {
					return err
				}
			}
		}
		if err := h.closeEmpty(child); err != nil {
			return err
		}
	}
	return nil
}
