# Network Toolkit 1.17.142 Beta 6 - publication verified

Published 2026-10-31T21:49:07Z as GitHub prerelease ID 430206177
(draft=false, prerelease=true).
URL: https://github.com/TechSavage1989/network-toolkit/releases/tag/v1.17.142
Tag/source/merge PR #17: 4a2d215ac6fe3b8014a3b386ed436f370035d34c.
Release workflow 41245081790: build SUCCESS, publish SUCCESS.

## Deliverables and safeguards

- Beta 6 is the reliability/optimization stage following Beta 4
  application-wide performance and Beta 5 interface polishing.
- Graph workspace filters unrelated Terminal/Packet Capture DOM updates,
  releases detached graph DOM/resize observations, cancels stale drags
  and preserves intended graph/results scroll state during replacement.
- Editor includes native gutter gap markers and bounded height-map
  recovery with source/selection/history protections.
- Preserves the stable Network_Toolkit.exe name, source-inclusive portable
  ZIP, strict binary-only updater package, credential ownership, saved
  settings, transport permissions and explicit update confirmation.

GitHub-reported official assets:
- Network_Toolkit.exe: 21,106,688 bytes;
  SHA-256 c53a2e9d4452301f7e9842e175a89fd086df5df1bba7935990a50034d333bf58
- Network_Toolkit_1.17.142_Portable.zip: 14,432,206 bytes;
  SHA-256 2d729c1e8070fd9cb8693066fb7f11dc99f35d0077346440ffe292cab766b1ec
- Network_Toolkit_1.17.142_Update.zip: 8,065,661 bytes;
  SHA-256 8602f6cf6c7cdfe5bb7cf2998f2717faf1ce0b352b3131597f62124325f4963f
- SHA256SUMS: 292 bytes;
  SHA-256 aa6d741e5393112e71f520e005185b2eeae658074df32f1df3fbc1ddc5f59fa5

The release target and version tag agree with the merge SHA.
The official GitHub release listing places v1.17.142 before v1.17.141,
while stable releases/latest remains v1.17.136. Asset digests above
are GitHub-reported; no independent public file-byte download and
rehash is claimed.

## Known failures and future Beta 6 follow-up

Published core Go, JS, Python, Windows PE/resource, ZIP structure and
release publication passed. Targeted synthetic Graph/Packet Capture/
remote-browser reliability tests passed 122/122 locally; browser divider
matrix and continuous-motion matrix also passed locally.

Earlier exact-source GitHub Checks showed intermittent Editor
aligned-comparison and continuous narrow-map motion failures, even
when the divider gate passed. Preserve the original CI evidence.
Publishing an opt-in beta is NOT a declaration that those unrelated
browser checks passed or that the Editor regressions are resolved.
Physical Windows multi-hour soak, full memory measurement and live
simultaneous device protocol testing remain unperformed.

This is a documentation-only post-publication closure, not a change
to the reviewed/tagged application source, release assets or checksums.
Follow-on improvements require a newer beta version, not a revision
of the published v1.17.142 bytes.

---

# Network Toolkit v1.17.142 Beta 6 - comparison gutter alignment candidate

Status: UNPUBLISHED candidate on beta/1.17.142-gutter-alignment, PR #17.
Based on published v1.17.141 (e2b8facd51834cfa93e2f587e01cd9d1c2696a89)
and documentation-only main commit 57d52c5bc3832239e2eeee1e2c30968111196491.

## Root cause and rejected first approach

- The v1.17.141 post-release Checks 41232846183 failed existing
  editor-divider113.browser.cjs in an 820px dark comparison view: the
  right native line-number gutter lagged actual source rows by 2.265625
  CSS pixels. The test's 2px threshold was not relaxed.
- The first v1.17.142 candidate moved source-free after-line spacer
  widgets before newline boundaries and revised map coordinate lookup.
  PR Checks 41237848219 demonstrated larger false scroll heights and
  failures in the divider and continuous motion gates. That code
  was superseded; do not represent that revision as validated or released.

## Revised candidate

- Keep the original source-free visual gap placement, comparison model,
  fractional map translation and content/selection transactions unchanged.
- Use an explicit native CodeMirror line-number gutter with the same
  numbered source line markers as before, and empty gutter markers
  for actual block gap widgets. CodeMirror now includes those fractional
  source-free heights in its normal gutter layout.
- Keep the existing diff-symbol gutter independent. No manual gutter
  DOM positioning, text insertion, viewport-copy workaround or source
  line count change.
- Added detection of grossly inflated CodeMirror virtual heights after
  wrapping or pane-width changes. Recovery captures the actual visible
  source line/fraction, then rebuilds the same immutable native editor
  state through the existing source-aligned reconciler. Native origin
  mismatch recovery permits at most three guarded attempts per pane and
  layout generation, only outside active gestures or composition.
  There are no manual gutter DOM position edits or document/history changes.
- Focused regression tests cover gap markers, source line identities,
  spacer widths, native height mismatch and guarded recovery attempts.
  The full real-Chromium synthetic divider matrix passed 36/36 locally,
  with zero reported geometry violations across main, detached and modal
  Editor windows at 1440/820/390 pixels and both themes. The local fixture
  loads the application's real HTML/JS/CSS but mocks network APIs; it
  is not physical Windows/WebView2 or live customer-device validation.
  Hosted exact-source CI remains a publication requirement.

## Acceptance and safeguards

- Run full Node/Go/Python, security, Windows, and browser suites against
  the exact versioned candidate; preserve the original divider gate
  across main/detached/modal, 1440/820/390 widths and both themes.
- Verify the continuous comparison motion gate, visible gutter geometry,
  native editing, source bytes, undo/redo, window ownership and rendering.
  Document any failures rather than waiving geometry thresholds.
- Windows version resources, the 1.17.142 Go and UI version, stable
  Network_Toolkit.exe, ZIP source allowlist and binary-only updater
  package must match exactly.
- Publish only as opt-in GitHub prerelease after reviewed CI succeeds.
  Verify release tag, four assets/digests, updater beta discovery and
  unchanged stable v1.17.136 feed and older release assets.
- No new stored credentials, network permissions, transfer destinations,
  packet capture behavior, automatic installation or source ownership.
  Historical SFTP cancellation issues are separate.
- No physical customer-device Windows, SSH, Telnet or serial measurements
  are claimed.

PUBLICATION PENDING.


## Phase 6: application-wide reliability continuation

The multi-phase roadmap calls for Beta 6 to cover extended operation, concurrent
tools, memory use, long-running activity, cancellation/recovery and cross-tool
interactions. Beta 6 must not be described as an Editor-only release.

The Graph workspace now filters document mutation notifications. Terminal,
Packet Capture and other unrelated DOM churn no longer schedules graph layout
work. Graph workspaces still respond when their own content or parent changes.
When a graph is removed, its ResizeObserver targets and detached DOM references
are released, stale divider drags are cancelled and the current scroll state
remains usable when the same run is reopened. Live replacements preserve the
currently inspected row anchor until restoration completes.

Focused tests include 1,200 unrelated mutation records across 30 batches,
actual graph changes, whole-panel replacement, detached workspace cleanup,
drag cancellation and scroll restoration. The combined Graph, Packet Capture
and remote-browser targeted suites passed 122/122 synthetic Node tests locally.

Existing tests and native Windows CI cover scan/capture/Terminal and transfer
cancellation, but do not establish an independent physical all-tools soak
with bounded memory across hours of operation. This remains a known limit of
this beta, not an assertion that every Phase 6 scenario is certified.

## Unresolved Editor checks

The initial exact-source CI runs for v1.17.142 continued to show intermittent
narrow-window Editor aligned comparison and continuous map-motion failures
despite divider-gate passes and local 36-layout/48-gesture acceptance.
Retain their failure reports and the original numeric thresholds. Disclose
these as beta limitations; do not claim the UI is fully regression-free or
promote to stable until resolved.
