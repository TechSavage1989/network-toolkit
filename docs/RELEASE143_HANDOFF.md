# Network Toolkit v1.17.143 Beta 7 - published, limitations preserved

Published at 2026-10-31T22:21:57Z, prerelease=true, draft=false,
GitHub release ID 430220837.
Release: https://github.com/TechSavage1989/network-toolkit/releases/tag/v1.17.143
Source/tag/merged PR #18: cdef07de894b68b472f083bf3082fb905c7ffa9e.
Exact source Release run 41247610887 passed both build and publish jobs.

## Delivered and protected

- Comparison map synchronizes native scroll geometry using bounded painted
  offsets from visible matched comparison rows, including unequal source
  line indices caused by insertions or removals.
- Editor browser source-geometry tests wait for mounted virtual source rows
  without raising or waiving the existing visual tolerances.
- Original source text, selections, history, saved connections, credential
  storage, transport authorization, desktop icon, update consent, and stable
  Network_Toolkit.exe filename are unchanged.
- Release build passed application Go tests, JavaScript regression tests,
  Python release guards, Windows x64 PE resources/build, portable source ZIP
  and strict binary-only update ZIP verification.

GitHub-reported release asset metadata:
- Network_Toolkit.exe: 21,109,248 bytes;
  SHA-256 aa896aa49615d6258afb06f565978134127d7b3e2424fae02a759b2de62d3f72
- Network_Toolkit_1.17.143_Portable.zip: 14,440,778 bytes;
  SHA-256 edac84b3ffb6aecf06e9f915ef35885016ae9416de6bb8ad6544482e00fa1543
- Network_Toolkit_1.17.143_Update.zip: 8,066,191 bytes;
  SHA-256 0afa573eeec7348898b2e8411ed410f63201ee3f930cb5e0afb3655db68ee0a1
- SHA256SUMS: 292 bytes;
  SHA-256 3e3d8684c755a1c98fa5fca44d89c6b855b772bc93987bb32c26030a6f1b8db0

Tag, release target and merge commit agree; latest listed prerelease
is v1.17.143, normal stable latest remains v1.17.136. Asset sizes and
digests are GitHub metadata, not an independent public-download rehash.

## Unresolved hosted CI

Checks run 41246461674 passed twelve job groups, including Windows native,
security, core app/JavaScript and source-alignment browser tests; the
browser-editor112 job failed the existing continuous comparison-map motion
test. Three 390px scenarios reported brief 3-5 CSS px common-row offsets
(p95 approximately 3.31 to 4.45), above the unchanged 2px threshold,
although the panes converged after the movement. No source loss was shown.
The independent broad browser job had not completed when last checked.
Do not call all 14 jobs passed, do not waive these observations, and do
not claim full Editor regression closure from a successful release build.

Real hardware, sustained multi-hour Windows load, memory-growth soak,
live SSH/Telnet/Serial concurrency and native SFTP cancellation remain
separate acceptance work. Beta 7 is not a stable certification.

This post-publication documentation closure changes no published source,
tag, binary, asset or release body. The candidate notes following this
section are retained as historical context, not a pending publication.

---

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
