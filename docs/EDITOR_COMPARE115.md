# Comparison controls and source-safe indicators

The comparison Options trigger stays in its original toolbar row when opened.
Only the compact settings region expands below it; its controls remain keyboard
accessible and the trigger reports the controlled region and expanded state.

- **Sync horizontal scrolling** is independent of existing vertical/map sync.
  It defaults off. When enabled with word wrap off, either pane can become the
  gesture source. The peer receives the same horizontal pixel offset, clamped
  to its own range. A shorter peer never pulls the longer source backward.
  Wrap disables the control without clearing the preference.
- The native comparison gutter displays **+ Added**, **− Removed**, and
  **~ Changed** beside real source lines. Icons use CodeMirror's public gutter
  geometry, including wrapped blocks; alignment gaps remain unnumbered and
  contain no source text or icon. Tooltips and labels supplement color.
- **Underline changed text** and **Highlight changed text** are independent.
  Both default on, preserving earlier presentation. Disabling either never
  removes the basic line-level difference colors or gutter indicators.

The three exact boolean preferences are carried by the existing bounded editor
window snapshot. Omitted underline/highlight values from older snapshots mean
on; explicit false survives round trips. No document, file, credential, session,
or ownership field is added. Persisted app settings contain preferences only.

The pinned CodeMirror state/view versions are unchanged. The offline bundle now
exports the existing public `gutter` and `GutterMarker` APIs; the checked-in build
script, provenance hashes and integrity smoke tests cover the exact bundle.
No runtime download, new package, secondary undo manager or shadow document was
introduced. Existing vertical alignment and continuous-motion gates remain
required; the 114 duplicate-run latency caveat is not claimed fixed here.

Focused browser acceptance: `scripts/editor-compare115.browser.cjs`, with
`PLAYWRIGHT_MODULE` and `TOOLKIT_TEST_OUTPUT`. It uses synthetic documents and
checks actual Options bounds, both horizontal wheel/scrollbar sources, differing
scroll ranges, wrap toggling, independent emphasis, native gutter placement,
real source numbers and exact source retention in main/detached light/dark
normal/narrow layouts. Browser execution/pixel inspection is a hosted gate, not
implied by the local helper tests.
