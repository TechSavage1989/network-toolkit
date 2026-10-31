# Network Toolkit 1.17.137 Beta 1 - initial Phase 1 responsiveness (candidate)

## Baseline and scope
Branch beta/1.17.137-phase1 starts from the stable v1.17.136 source
commit. This beta is intentionally separate from the stable bridge and
uses docs/RELEASE_CHANNEL.txt="beta", matching
cmd/network-toolkit/updater.go updateBuildChannel="beta".
The release publisher must use GitHub --prerelease. Version/tag
v1.17.137 remains numeric for PE/resources, package and updater validation.

## Phase 1 implemented
- scheduleTerminalFit checks visible pane pending timers before calling
  sizeTerminalStage, avoiding repeated stage/layout measurements for
  ResizeObserver bursts while preserving the 16 ms bounded fit deadline.
- Local highlighter decoration refresh is skipped when terminal dimensions
  do not change, avoiding redundant DOM work during same-cell-size events.
- Real-browser terminal resize defers noncritical chrome refreshes through
  the existing bounded scheduleTerminalOutputChrome path. Actual xterm
  rows/columns and remote PTY queue timing are unchanged.
- The existing pane ownership, transfer, restore, input, selection,
  scrollback, keyboard, prompt pacing, output and updater safeguards
  remain unmodified.

## Required acceptance before publication
Validate source tests for 300 layout observer events without redundant
measurements, ongoing 16 ms pane fitting, geometry no-op highlighting,
deferred cosmetic refresh and existing resize129 guards. Run full JS,
Go, Python, native browser/security, Windows x64 resource/build/ZIP
checks. Verify that stable 1.17.136 has already been published and the
opt-in feed selects beta 1.17.137 while latest stable remains 1.17.136.
Verify exact four GitHub prerelease assets and hashes, source/tag and
no overwriting of the prior stable release. Retain historical Editor
comparison-motion, native SFTP and PowerShell CI observations. Do not
claim real-device benchmark improvements without physical testing.

Publication is pending and will be recorded only after verification.
