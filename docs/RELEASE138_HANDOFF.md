# Network Toolkit v1.17.138 Beta 2 - published pane and scroll stability

Published 2026-10-31T07:38:31Z as GitHub prerelease 429790502
(draft=false, prerelease=true).
Release: https://github.com/TechSavage1989/network-toolkit/releases/tag/v1.17.138
Tagged/validated source: 8e9052eb14ab9a5a5fc9363b0ac710a36203a133
Release tree: b3d58421cdbd3f7d39b170622931f0b13cf4b516
Pull request: https://github.com/TechSavage1989/network-toolkit/pull/13
Exact-source workflow 41181228389: build SUCCESS, publish SUCCESS.
Four official assets uploaded and latest GitHub stable feed unchanged at
v1.17.136. Beta v1.17.137 prerelease remains unchanged.

## Delivered

Phase 1 Beta 2 improves terminal pane interaction:
- Splitter pointermove events update the visual ratio at most once per
  animation frame. The final pointerup position commits even if a pending
  frame has not run. Live grid coordinates are used when a window/sidebar
  changes size mid-drag.
- Pointercancel, lost pointer capture or window blur restores the prior
  ratio, releases pending frames and clears drag event handlers. An
  intervening pane/layout context change prevents stale writes.
- Terminal pane content clips the xterm canvas during the time needed to
  fit new rows/columns, keeping content inside its assigned pane.
- When Follow Latest is disabled, an original viewport row is retained as
  a bounded fallback if xterm discards the normal-buffer marker during
  reflow/scrollback trimming. Explicit subsequent user scrolling and
  Follow Latest actions retain priority via the existing epoch check.
- Existing xterm resize/selected output behavior, remote PTY resize,
  SSH/Telnet/Serial/local input, session ownership, detached workspaces,
  saved settings, credentials and updater package remain unmodified.

## Exact release validation

The release workflow passed 44 Python tests, 1,524 JavaScript tests
(0 failures), application Go tests, Windows x64 GUI compilation,
Windows version/icon resource consistency, portable and update ZIP
verification. The source-inclusive portable bundle has 1,278 members.
Synthetic input/resize/scroll tests do not constitute a physical
network device or measured frame-time benchmark.

Public GitHub asset sizes and SHA-256 digests:
- Network_Toolkit.exe: 21,094,912 bytes;
  2962bc2556de8573f1abee938a40863f764291641d9b87b64105b7465294093a
- Network_Toolkit_1.17.138_Portable.zip: 14,384,100 bytes;
  8f768ade88330d8dc8102cbb354526183029f12c3e27c27c7dc511556b0bec3c
- Network_Toolkit_1.17.138_Update.zip: 8,062,464 bytes;
  6f4128a3bb9e6d461dad284804d9da9526ef0493d41fa9ac72500a9570e918ff
- SHA256SUMS: 292 bytes;
  b2c1e94a918790df8a3f4f544550cfe2e8eae31611b7a658f81f69f2654a8d83

GitHub release metadata confirms exact source/tag, prerelease flag and
asset presence/digests. No separate independent public-download rehash
was performed. v1.17.136 and v1.17.137 original release tags/assets
were not overwritten. Opted-in v1.17.137 installations can download and
install 1.17.138 using App settings > Updates > Check for updates,
Download update and Install and restart. No automatic install.

## Preserved CI observations

An initial candidate CI run 41180789061 failed the existing
TestBeta136DownloadRequiresMatchingOptedChannel because its simulated
future stable release was the same numeric version as this beta.
The test fixture was changed to compute a genuinely newer version from
model.Version; final review CI main application and JavaScript checks
passed and the exact-source release workflow succeeded.
Separate broad browser/native PR and post-merge Checks jobs remained in
progress at the time of this handoff. Do not count those pending checks
as passed. Historical Editor comparison, native SFTP and PowerShell
CI issues are not claimed fixed. No real-device performance benchmark
or universal overlapping-content fix was established.

This documentation-only post-release closure does not modify the
published v1.17.138 tag, executable, beta flag or asset bytes.

---

# Network Toolkit v1.17.138 Beta 2 - Phase 1 multi-pane stability (candidate)

## Purpose

Begin from published v1.17.137 Beta 1 and current main documentation-only
handoff. Focus Phase 1 Beta 2 on split-pane resizing and preserving a user's
scroll context. Keep the existing beta opt-in, stable channel, Windows portable
file name and verified update-package contract unchanged.

## Scoped implementation

- The terminal divider pointer handler now accumulates rapid movement and
  applies the last coordinate at a bounded animation-frame cadence. Pointerup
  commits the final coordinate even if the frame has not run. Pointer cancel,
  window focus loss or pointer-capture loss restores the pre-drag split ratio
  and clears pending work. Live grid geometry is read once per frame so changing
  window/sidebar dimensions during a drag do not cause stale ratios.
- Pane bodies and terminal display elements clip xterm canvas content while a
  resize is pending, keeping visual content within the assigned pane.
- When Follow Latest is off, beforeFit retains a viewport row as a fallback
  if the normal-buffer scroll marker disappears during xterm reflow. afterFit
  clamps that row to available scrollback. Epoch checks still protect a newer
  user scroll or Follow Latest action, and other panes are independent.
- The existing local xterm fitting, deferred cosmetic redraw, remote PTY
  resize sequencing, selected text, input/paste prompt guards, session
  ownership, detached windows, saved settings and updater are unchanged.

## Validation and release

Run new scripts/terminal-beta138-pane.test.cjs together with existing
terminal-fit/resize/output, terminal-pane-detach and full JS/Go/Python suite.
Use GitHub Checks to validate responsive browser and Windows CI, and the
existing exact-source Release workflow for Windows PE resource generation,
ZIP allowlist/integrity, release assets and updater metadata.

Build channel remains beta via docs/RELEASE_CHANNEL.txt and compiled
updateBuildChannel. Publish v1.17.138 as a GitHub prerelease, not stable.
Verify v1.17.136 remains GitHub stable latest, v1.17.137 is unchanged,
four uploaded assets have SHA-256, and the opt-in feed sees the beta.

Unrelated historical Editor alignment, Windows native SFTP and shell echo
CI issues must not be called fixed by this scoped change. The test fixtures
use synthetic pane and terminal behavior, not a physical SSH latency benchmark.

PUBLICATION PENDING.
