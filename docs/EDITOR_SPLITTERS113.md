# Editor pane divider (113)

Split editing and comparison share a compact vertical resize rail. Drag it to
adjust document widths, or focus it and use Left/Right (20px), Home/End (the
available limits), and Enter (balanced). Double-click also resets. Escape or a
cancelled pointer gesture restores the pre-drag width. Pointer capture is released
on completion, cancellation or disposal.

The rail uses the existing file browser's border/accent palette, col-resize cursor
and keyboard separator pattern. It adds no toolbar row. Pane minimums adapt to
available width, including the comparison map, without horizontal page overflow.
The existing measured wrapping/scroll-map pipeline runs again after widths change;
document bytes, edit history, selections and tab ownership are not changed.

The last user ratio is window-local memory keyed to the stable editor root. It
survives switching between split editing and comparison in the same window. It is
not a new stored preference, transferred document field or cross-window setting.

Focused Node tests exercise bounds, both modes, pointer identity, cancellation,
keyboard limits/reset, hidden/zero width, remount and disposal. The hosted fixture
`editor-divider113.browser.cjs` uses synthetic documents and actual mouse/keyboard
input in main, detached and modal editors at1440/820/390px in both themes. It checks
wrapped-map resynchronization, dirty/history/selection retention and real rendered
geometry. Authoring the fixture does not establish a pass; the final release must
retain its exact-source hosted result and inspected pixels.
