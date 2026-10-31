# Network Toolkit v1.17.142 Beta 6 - comparison gutter alignment candidate

Status: development candidate, NOT PUBLISHED.
Base: published v1.17.141 Beta 5 (tag e2b8facd51834cfa93e2f587e01cd9d1c2696a89) plus the documentation-only main handoff commit 57d52c5bc3832239e2eeee1e2c30968111196491.
Branch: beta/1.17.142-gutter-alignment.

## Evidence and scope

- After v1.17.141 publication, main Checks workflow 41232846183 finished with 13/14 jobs successful. The browser-editor112 job failed its existing editor-divider113.browser.cjs gate.
- In main comparison at 820px, dark theme, the right native line-number gutter was persistently about 2.265625 CSS pixels above visible source lines 138-144. Rendered source content and map positions were otherwise consistent. The divider gate records the failure instead of waiving the 2px alignment threshold.
- The patch anchors each visual, source-free after-line alignment gap at the preceding CodeMirror source line end with block widget side +1, instead of inserting it before the next line at its source start. This avoids making the next native gutter row a compound leading-widget/source block.
- The aligned-to-native pixel mapping now finds the trailing gap at that same preceding line end. Leading gaps and final-line gaps retain source-free behavior.
- Focused tests model the preceding-line boundary, fractional gap pixels, and aligned/native round trips. Existing rendered acceptance still determines whether the correction works.

## Preservation requirements

Keep document text, UTF-16 offsets, undo/redo, dirty state, source ownership, credentials, transfer destinations, authorized native SFTP, saved settings, terminal transports, diagnostics, packet capture, beta opt-in and installation confirmation unchanged. Do not adjust gutter DOM element positions manually or relax native geometry assertions.

## Validation before release

- Run focused source tests, full JavaScript/Go/Python checks and browser/native Windows CI.
- Observe editor-divider113.browser.cjs for all main/detached/modal, split/comparison, 1440/820/390 widths and both themes; inspect fractional gutter alignment and screenshot stability.
- Keep editor-alignment113.browser.cjs, editor-motion114.browser.cjs, editor-compare115.browser.cjs, browser security and native editor tests in scope.
- Any failure requires investigation, not a waived threshold or a claim that earlier native SFTP cancellation issues are solved.
- If all release checks eventually pass, coordinate version 1.17.142 in Go/web/Windows resources and release notes, publish opt-in prerelease only under separate release authorization, verify four assets and digests, and keep stable v1.17.136 and all earlier tags/assets unchanged.

No physical Windows end-user, real SSH/serial hardware, or network-device benchmark is claimed.

PUBLICATION PENDING.
