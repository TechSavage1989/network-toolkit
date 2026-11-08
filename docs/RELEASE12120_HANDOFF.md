# Network Toolkit v1.21.20 Beta 1 - Text Editor large-file performance candidate

**Status:** Candidate on branch fix/1.21.20-large-editor-comparison-performance. Not published until the release workflow succeeds.

## What changed
- Large-file gutter repaint work is coalesced to once per animation frame and skips unchanged visible line ranges. The editable textarea and exact source data remain unchanged.
- A browser beforeinput edit plan updates the cached source line index incrementally for ordinary typing, paste, and deletions. Any unexpected input safely falls back to rebuilding exact line positions.
- The window recovery checkpoint version no longer changes merely because a large document scrolled. Explicit moves and edits still snapshot the complete document with existing security, ownership, and size checks.
- Comparison's source view tracks applied document revisions so a simple refresh does not serialize or replace a multi-megabyte CodeMirror document.
- Comparisons now accept up to 16 MiB and 30,000 lines on each side. Bounded unique-line anchors allow large mostly similar device configurations to compare without a quadratic matrix for the whole changed region.
- Diff calculation above 2 MiB runs in a same-origin Worker with a 20-second bounded completion. Worker failure gives an actionable limited result rather than blocking UI. No document text is transmitted to an external service.

## Tests
- Node: scripts/editor-large-index.test.cjs, scripts/editor-large-worker.test.cjs, scripts/editor-compare.test.cjs, existing editor and security tests.
- Hosted Chromium: scripts/editor-capacity114.browser.cjs checks 16 MiB scrolling, gutter coalescing, unchanged scroll-only recovery checkpoint versions and byte-exact editing/undo/save.
- Hosted Chromium: scripts/editor-alignment113.browser.cjs checks an actual >2 MiB worker comparison, source integrity, hunk navigation readiness and map rendering in addition to the existing wrapped/edited compare cases.
- Existing Go, Python, native host and browser regression suites remain active. Main branch and release workflow use the verified Windows and Linux portable pipeline.

## Limitations
- Files beyond the comparison's 16 MiB or 50,000-line limit, and changed regions exceeding the safe algorithm budget, report that they cannot be compared fully. No partial result is mislabeled as exact.
- A physical Windows desktop and real network files were not accessed for tests.
- No network adapter, Hosts file, SSH host trust, credential, vault, file overwrite, updater consent, or saved state authorization changes were made.
- Existing stable v1.21.19 and all previous tags/assets must remain unchanged.

## Publication
- Publish as a **new opt-in v1.21.20 Beta 1**, not a revision to an existing release.
- Only after review, green required CI and guarded release validation, merge and verify new Windows executable, portable, updater and Linux artifacts plus SHA256SUMS.
