# Editor tab gestures (115 candidate)

Ordinary mouse/pen tab dragging follows Terminal: move within a tab strip to
reorder; move away and release to open a separate Editor window. The gesture
starts after six pixels of movement. The detach area begins 24 pixels above or
below a strip, or 40 pixels beyond its horizontal ends. Feedback names the
pending action; Escape cancels. There is no mandatory drop box.

Pointer capture also handles a genuine release outside the source window. A
lost capture, pointer cancellation, window blur, changed document object/lease,
or newly busy editor cancels without offering a transfer. Clicking alone sends
no ownership or drag request. Existing explicit Detach actions remain available.

Hold **Alt before starting a drag** to use the existing native drag-and-drop
path between editor windows. This path carries only a short-lived opaque ticket
and retains the existing offer/accept/acknowledgement protocol. An unsuccessful
OS drop is never interpreted as a detach. **Move / merge** also provides an
accessible destination selector, without requiring a drag gesture.

No new native bridge or credential capability is introduced. Both drag modes
reuse the existing document transfer, retain source state while ownership is
uncertain, and preserve dirty text, undo/redo, selection and scroll. The browser
mode can reorder; native detachment requires the desktop window capability.

Validation authored for this candidate includes fixed-UID/lease/cancellation
unit tests, real browser mouse gestures in embedded/detached normal/narrow
layouts, and the existing hosted Windows fixture with actual captured-pointer
outside-window release, Escape, failed Alt-drop and Alt cross-window merge.
Actual browser/native acceptance must be recorded against the integrated source;
local unit tests and Windows cross-compilation do not establish that acceptance.
