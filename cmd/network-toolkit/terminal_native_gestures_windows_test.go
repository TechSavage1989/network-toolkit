//go:build windows

package main

import (
	"errors"
	"fmt"
	"sync/atomic"
	"time"
)

// Explicit114 extension: real Win32 captured-pointer motion, not drag event
// synthesis. The113 baseline never invokes this method or needs new tab code.
func (h *nativeTerminalFixture) exerciseTerminalGestures(accepted *atomic.Int32) error {
	empty := nativeEditorSnapshot{}
	point := nativeEditorRect{}
	if err := h.nativeInput("Arrange", "Tab", "", "", empty, empty, point, point); err != nil {
		return err
	}
	before, err := h.command("", "drafts")
	if err != nil {
		return err
	}
	if len(before.Geometry.Tabs) != 3 {
		return fmt.Errorf("expected live+two draft tabs, got%d", len(before.Geometry.Tabs))
	}
	if !before.ReorderPointsVisible {
		return errors.New("native reorder source or target is outside clipped visible terminal strip")
	}
	convert := func(s nativeTerminalSnapshot) nativeEditorSnapshot { return nativeEditorSnapshot{Geometry: s.Geometry} }
	from, err := nativeEditorPoint(convert(before), h.session, false)
	if err != nil {
		return err
	}
	last := before.Geometry.Tabs[2].UID
	to, err := nativeEditorPoint(convert(before), last, true)
	if err != nil {
		return err
	}
	if err = h.nativeInput("Drag", "Tab", "", "", convert(before), convert(before), from, to); err != nil {
		return err
	}
	reordered, err := h.wait("", func(s nativeTerminalSnapshot) bool {
		return len(s.Geometry.Tabs) == 3 && s.Geometry.Tabs[2].UID == h.session && s.Editable
	})
	if err != nil {
		return err
	}
	if reordered.Transcript != before.Transcript || reordered.Selection != before.Selection {
		return errors.New("native reorder changed terminal output or selection")
	}
	if err = h.capture("", "terminal114-reordered"); err != nil {
		return err
	}
	h.phases = append(h.phases, "Actual Win32 pointer reorders live tab among two drafts without reconnecting")
	from, err = nativeEditorPoint(convert(reordered), h.session, false)
	if err != nil {
		return err
	}
	to = nativeEditorRect{Left: reordered.Geometry.Width * .6, Top: reordered.Geometry.Height * .75}
	if err = h.nativeInput("Drag", "Escape", "", "", convert(reordered), convert(reordered), from, to); err != nil {
		return err
	}
	cancelled, err := h.wait("", func(s nativeTerminalSnapshot) bool { return s.Editable && s.Session == h.session })
	if err != nil {
		return err
	}
	time.Sleep(300 * time.Millisecond)
	if len(h.app.views.state().Views) != 0 || cancelled.Transcript != before.Transcript {
		return errors.New("Escape detached or altered the terminal")
	}
	h.phases = append(h.phases, "Actual Escape during drag keeps the source and transport")
	from, err = nativeEditorPoint(convert(cancelled), h.session, false)
	if err != nil {
		return err
	}
	if err = h.nativeInput("Drag", "Outside", "", "", convert(cancelled), convert(cancelled), from, to); err != nil {
		return err
	}
	var view string
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		for _, v := range h.app.views.state().Views {
			if v.SessionID == h.session && v.State != "returned" {
				view = v.ID
				break
			}
		}
		if view != "" {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if view == "" {
		return errors.New("actual outside-client pointer release did not detach")
	}
	child, err := h.wait(view, func(s nativeTerminalSnapshot) bool {
		return s.Editable && s.Session == h.session && s.Transcript == before.Transcript
	})
	if err != nil {
		return err
	}
	if child.Selection != before.Selection {
		return errors.New("drag detach changed selected output")
	}
	if child.Owner.ViewID != view || !child.Owner.Ready {
		return errors.New("native drag child missing accepted ownership")
	}
	if _, err = h.wait("", func(s nativeTerminalSnapshot) bool { return !s.Editable }); err != nil {
		return err
	}
	if err = h.capture(view, "terminal114-drag-detached"); err != nil {
		return err
	}
	h.phases = append(h.phases, "Actual outside-client pointer release opens separately bootstrapped child and ACK releases source")
	h.allowBulk.Store(true)
	if _, err = h.command("", "bulk-foreign"); err != nil {
		return err
	}
	retained, err := h.command(view, "snapshot")
	if err != nil {
		return err
	}
	if !retained.Editable || retained.Transcript != before.Transcript {
		return errors.New("main-window Close all affected detached session")
	}
	owner := h.app.views.state().Owners[h.session]
	if owner.ViewID != view || !owner.Ready {
		return errors.New("main bulk action changed foreign ownership")
	}
	h.phases = append(h.phases, "Main Close all cancellation preserves drafts; confirmed action closes only its drafts and leaves detached transport editable")

	if _, err = h.command(view, "reattach"); err != nil {
		return err
	}
	if _, err = h.wait("", func(s nativeTerminalSnapshot) bool { return s.Editable && s.Transcript == before.Transcript }); err != nil {
		return err
	}
	if _, err = h.command("", "bulk-disconnect"); err != nil {
		return err
	}
	if info, ok := h.app.terminalSession(h.session); !ok || info.State != "closed" {
		return errors.New("Disconnect all did not retain closed session")
	}
	if err = h.capture("", "terminal114-disconnected-retained"); err != nil {
		return err
	}
	if _, err = h.command("", "bulk-close"); err != nil {
		return err
	}
	if _, ok := h.app.terminalSession(h.session); ok {
		return errors.New("Close all retained the owned closed session")
	}
	h.phases = append(h.phases, "Explicit native menu Disconnect all confirmation retains tab/output; Close all confirmation removes owned tab, cancel paths retain source")
	if accepted.Load() != 1 {
		return errors.New("native gesture caused a transport reconnect")
	}
	return nil
}
