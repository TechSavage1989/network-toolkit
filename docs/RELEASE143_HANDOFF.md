# Network Toolkit v1.17.143 Beta 7 - comparison motion reliability

Status: release candidate, not yet published.
Base: published v1.17.142 Beta 6 (tag 4a2d215ac6fe3b8014a3b386ed436f370035d34c),
plus main post-release handoff at 0730b532cba6619d4025a3bb4ae45bf3d77112da.
Branch: beta/1.17.143-editor-reliability.

## Why this beta

The approved plan runs Beta 4 application-wide performance, Beta 5 interface
polish, Beta 6 reliability and optimization with additional betas as needed.
Beta 7 follows up the remaining narrow Editor CI failures instead of
promoting the previous prerelease to stable.

Earlier GitHub browser-editor112 runs showed two separate issues:
- The alignment113 browser fixture could assume source line 3 was mounted
  even when CodeMirror virtualized it after a narrow wrap/reveal gesture.
  The failure was a null querySelectorAll, not demonstrated lost source bytes.
- The motion114 browser fixture observed a p95 common-row mismatch around
  3 to 5 CSS pixels during 390px map gestures. Original 2px map motion
  threshold was not relaxed; some sequences converged afterward.

## Delivered source behavior

- Reuse authoritative shared comparison geometry and actual CodeMirror
  line blocks for primary scroll positions.
- During synchronized input, compare painted positions of real visible
  source lines that share a comparison row, including when the source
  line numbers differ due to added or removed lines.
- Keep a bounded, at-most 12px residual calibration while the gesture is
  active. Ignore large or missing row differences. Reset after a new
  alignment layout/source revision; never adjust independently scrolled
  panes.
- The hosted alignment113 fixture now awaits the actual relevant mounted
  source rows after navigation before evaluating source, color, gutter
  and pixel geometry. This is an asynchronous readiness check, not a
  waived assertion.
- Existing app settings, source bytes, caret, undo/redo, network
  scanning, capture, terminal sessions, credentials, file-browser
  transfers and opt-in updater safeguards are unchanged.

## Validation and open limits

Focused Node tests cover changed-row identity, missing inserts, offscreen
rows, bounded calibration, cancellation/reset and independent scrolling.
The full local JavaScript suite passed 1,547 tests; local Python release
checks passed 44 tests after 143 metadata and resource generation.
The Windows COFF resource is compiled with the standard generator, not
patched into an already-built executable.

Required before publication: exact-source GitHub Go/JS/Python, Windows
x64 resource/build/ZIP gates, Editor motion, comparison geometry,
native files, terminal/prompt, security and update feed checks.
Report any historical/intermittent CI failure rather than masking it
or raising the 2px expectation. Physical Windows endurance/memory
benchmarks and simultaneous real devices remain unverified; this is
an opt-in beta only.

PUBLICATION PENDING.
