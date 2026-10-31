//go:build windows

package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

type nativeTerminalResize129Snapshot struct {
	Cols, Rows              int
	Buffer, FirstLine       string
	Dragging, Settled, Fits bool
	Divider                 *nativeEditorRect
}
type nativeResize129Size struct {
	Cols, Rows int
	AtMS       int64
}
type nativeTerminalResize129Evidence struct {
	mu      sync.Mutex
	writeMu sync.Mutex
	conn    net.Conn
	alt     bool
	sizes   []nativeResize129Size
	checks  []string
	during  []nativeTerminalResize129Snapshot
}

func (r *nativeTerminalResize129Evidence) attach(c net.Conn) { r.mu.Lock(); r.conn = c; r.mu.Unlock() }
func (r *nativeTerminalResize129Evidence) read(c net.Conn) {
	_ = readNativeResize129NAWS(c, func(cols, rows int) {
		r.mu.Lock()
		r.sizes = append(r.sizes, nativeResize129Size{cols, rows, time.Now().UnixMilli()})
		alt := r.alt
		r.mu.Unlock()
		if alt {
			_ = r.frame(cols, rows)
		}
	})
}
func (r *nativeTerminalResize129Evidence) last() (nativeResize129Size, int) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.sizes) == 0 {
		return nativeResize129Size{}, 0
	}
	return r.sizes[len(r.sizes)-1], len(r.sizes)
}
func (r *nativeTerminalResize129Evidence) write(value string) error {
	r.mu.Lock()
	c := r.conn
	r.mu.Unlock()
	if c == nil {
		return errors.New("no synthetic Telnet connection")
	}
	r.writeMu.Lock()
	defer r.writeMu.Unlock()
	_, err := io.WriteString(c, value)
	return err
}
func (r *nativeTerminalResize129Evidence) frame(cols, rows int) error {
	r.writeMu.Lock()
	defer r.writeMu.Unlock()
	r.mu.Lock()
	c, alt := r.conn, r.alt
	r.mu.Unlock()
	if !alt {
		return nil
	}
	if c == nil {
		return errors.New("no synthetic Telnet connection")
	}
	_, err := io.WriteString(c, fmt.Sprintf("\x1b[2J\x1b[HALT129 %dx%d\x1b[%d;1HSIZE129\x1b[H", cols, rows, rows))
	return err
}
func (r *nativeTerminalResize129Evidence) alternate(enabled bool) error {
	r.mu.Lock()
	r.alt = enabled
	r.mu.Unlock()
	if !enabled {
		return r.write("\x1b[?1049l")
	}
	if err := r.write("\x1b[?1049h"); err != nil {
		return err
	}
	size, _ := r.last()
	return r.frame(size.Cols, size.Rows)
}
func (h *nativeTerminalFixture) resize129Settled(view string) (nativeTerminalSnapshot, error) {
	return h.wait(view, func(s nativeTerminalSnapshot) bool {
		v := s.Resize129
		size, _ := h.resize129.last()
		return s.Editable && v != nil && v.Fits && v.Settled && s.GeometryFits && size.Cols == v.Cols && size.Rows == v.Rows
	})
}
func (h *nativeTerminalFixture) resize129Drag(delta float64) error {
	before, err := h.resize129Settled("")
	if err != nil {
		return err
	}
	v := before.Resize129
	if v.Divider == nil || v.Divider.Width <= 0 {
		return errors.New("no actual visible split divider")
	}
	from := nativeEditorRect{Left: v.Divider.Left + v.Divider.Width/2, Top: v.Divider.Top + v.Divider.Height/2}
	to := nativeEditorRect{Left: from.Left + delta, Top: from.Top}
	shape := nativeEditorSnapshot{Geometry: before.Geometry}
	_, count := h.resize129.last()
	done := make(chan error, 1)
	go func() { done <- h.nativeInput("Drag", "Resize", "", "", shape, shape, from, to) }()
	observedLocal, observedWire := false, false
	until := time.Now().Add(12 * time.Second)
	for time.Now().Before(until) {
		select {
		case err = <-done:
			goto finished
		default:
		}
		sample, e := h.command("", "snapshot")
		if e != nil {
			return e
		}
		if d := sample.Resize129; d != nil && d.Dragging && d.Cols != v.Cols {
			observedLocal = true
			h.resize129.during = append(h.resize129.during, *d)
			size, n := h.resize129.last()
			if n > count && size.Cols != v.Cols {
				observedWire = true
			}
		}
		time.Sleep(10 * time.Millisecond)
	}
	return errors.New("native divider drag did not terminate")
finished:
	if err != nil {
		return err
	}
	if !observedLocal || !observedWire {
		return fmt.Errorf("continuous pointer drag starved local/wire resize: local=%v NAWS=%v", observedLocal, observedWire)
	}
	after, err := h.resize129Settled("")
	if err != nil {
		return err
	}
	if after.Transcript != before.Transcript {
		return errors.New("resizing changed normal scrollback text")
	}
	if v.Buffer == "normal" && (after.Selection != before.Selection || after.Viewport != before.Viewport) {
		return errors.New("normal resize changed paused selection/viewport")
	}
	if owner := h.app.views.state().Owners[h.session]; owner.ViewID != "" || !owner.Ready {
		return errors.New("divider resize changed transport ownership")
	}
	h.resize129.checks = append(h.resize129.checks, fmt.Sprintf("real pointer continuous drag delta%.0f: local and Telnet NAWS changed before pointerup; latest=%dx%d", delta, after.Resize129.Cols, after.Resize129.Rows))
	return nil
}
func (h *nativeTerminalFixture) exerciseResize129(accepted *atomic.Int32) (result error) {
	r := h.resize129
	defer func() {
		r.mu.Lock()
		sizes := append([]nativeResize129Size{}, r.sizes...)
		r.mu.Unlock()
		failure := ""
		if result != nil {
			failure = result.Error()
		}
		raw, _ := json.MarshalIndent(map[string]any{"passed": result == nil, "error": failure, "checks": r.checks, "actualTelnetNAWS": sizes, "duringPointerDown": r.during, "connections": accepted.Load(), "scope": "Actual native WebView2 and Win32 pointer with synthetic loopback Telnet NAWS. No live SSH/device, real remote application or universal TUI claim."}, "", "  ")
		_ = os.WriteFile(filepath.Join(h.output, "native-resize129-report.json"), raw, 0600)
	}()
	if _, err := h.command("", "resize129:prepare"); err != nil {
		return err
	}
	if _, err := h.resize129Settled(""); err != nil {
		return err
	}
	for _, delta := range []float64{-90, 150, -120, 100} {
		if err := h.resize129Drag(delta); err != nil {
			return err
		}
	}
	anchor, err := h.command("", "snapshot")
	if err != nil {
		return err
	}
	for _, action := range []string{"maximize-pane", "restore-pane"} {
		if _, err := h.command("", "resize129:"+action); err != nil {
			return err
		}
		if _, err := h.resize129Settled(""); err != nil {
			return err
		}
	}
	if _, err := h.command("", "resize129:hide"); err != nil {
		return err
	}
	_, hiddenCount := r.last()
	time.Sleep(250 * time.Millisecond)
	_, afterHidden := r.last()
	if hiddenCount != afterHidden {
		return errors.New("hidden terminal sent unexpected size changes")
	}
	if _, err := h.command("", "resize129:reveal"); err != nil {
		return err
	}
	if _, err := h.resize129Settled(""); err != nil {
		return err
	}
	r.checks = append(r.checks, "pane maximize/restore and page hidden/reveal retain nonzero final geometry; hidden sends no size changes")
	for _, action := range []string{"Maximize", "Restore"} {
		if err := h.nativeInput(action, "Resize", "", "", nativeEditorSnapshot{}, nativeEditorSnapshot{}, nativeEditorRect{}, nativeEditorRect{}); err != nil {
			return err
		}
		if _, err := h.resize129Settled(""); err != nil {
			return err
		}
	}
	restored, err := h.resize129Settled("")
	if err != nil {
		return err
	}
	if restored.Transcript != anchor.Transcript || restored.Selection != anchor.Selection || restored.Viewport != anchor.Viewport {
		return errors.New("maximize/restore or hide/reveal changed normal history, paused selection or viewport")
	}
	r.checks = append(r.checks, "actual Win32 maximize/restore converges to newest acknowledged Telnet size and preserves normal history/selection/viewport")
	normal, err := h.command("", "snapshot")
	if err != nil {
		return err
	}
	if err = r.alternate(true); err != nil {
		return err
	}
	if _, err = h.wait("", func(s nativeTerminalSnapshot) bool {
		return s.Resize129 != nil && s.Resize129.Buffer == "alternate" && strings.HasPrefix(s.Resize129.FirstLine, "ALT129 ")
	}); err != nil {
		return err
	}
	for _, delta := range []float64{-90, 90} {
		if err = h.resize129Drag(delta); err != nil {
			return err
		}
	}
	alt, err := h.resize129Settled("")
	if err != nil {
		return err
	}
	want := fmt.Sprintf("ALT129 %dx%d", alt.Resize129.Cols, alt.Resize129.Rows)
	if _, err = h.wait("", func(s nativeTerminalSnapshot) bool { return s.Resize129 != nil && s.Resize129.FirstLine == want }); err != nil {
		return err
	}
	if err = h.capture("", "terminal-resize129-alternate"); err != nil {
		return err
	}
	if err = r.alternate(false); err != nil {
		return err
	}
	if _, err = h.wait("", func(s nativeTerminalSnapshot) bool {
		return s.Resize129 != nil && s.Resize129.Buffer == "normal" && s.Transcript == normal.Transcript
	}); err != nil {
		return err
	}
	r.checks = append(r.checks, "synthetic server redraws alternate screen from received NAWS, normal scrollback restored unchanged")
	if _, err = h.command("", "resize129:single"); err != nil {
		return err
	}
	if os.Getenv("NETWORK_TOOLKIT_NATIVE_TERMINAL_SELECTION") == "1" {
		selected, selectErr := h.command("", "selection")
		if selectErr != nil {
			return selectErr
		}
		if selected.Selection == "" {
			return errors.New("resize129 transfer selection was not established after leaving alternate screen")
		}
	}
	before, err := h.resize129Settled("")
	if err != nil {
		return err
	}
	if _, err = h.command("", "detach"); err != nil {
		return err
	}
	view := ""
	until := time.Now().Add(15 * time.Second)
	for time.Now().Before(until) && view == "" {
		for _, v := range h.app.views.state().Views {
			if v.SessionID == h.session && v.State != "returned" {
				view = v.ID
				break
			}
		}
		time.Sleep(20 * time.Millisecond)
	}
	if view == "" {
		return errors.New("resize129 detach child not created")
	}
	child, err := h.resize129Settled(view)
	if err != nil {
		return err
	}
	if child.Transcript != before.Transcript || child.Selection != before.Selection || child.Viewport != before.Viewport {
		return errors.New("resize129 child changed normal history, selection or viewport")
	}
	if _, err = h.command(view, "reattach"); err != nil {
		return err
	}
	returned, err := h.resize129Settled("")
	if err != nil {
		return err
	}
	if returned.Transcript != before.Transcript || returned.Selection != before.Selection || returned.Viewport != before.Viewport {
		return errors.New("resize129 return changed normal history, selection or viewport")
	}
	if accepted.Load() != 1 {
		return errors.New("resize129 reconnected the synthetic transport")
	}
	r.checks = append(r.checks, "native child detach/return converges to each window's final NAWS size without reconnecting")
	return h.capture("", "terminal-resize129-returned")
}
