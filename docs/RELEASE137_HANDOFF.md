# Network Toolkit 1.17.137 Beta 1 - published Phase 1 responsiveness

## Publication and channel proof
Published 2026-10-31T07:15:09Z as GitHub release ID 429781469,
a prerelease (prerelease=true, draft=false).
https://github.com/TechSavage1989/network-toolkit/releases/tag/v1.17.137
Tag/source: 928443966903e7b3c81ffd161f8a72c38ce2efa7
Tree: 35dea75bb4a99827f1d6cf584b28a51c58718fd6
The source build and publish jobs of workflow 41179423624 succeeded.
GitHub releases/latest still resolves stable v1.17.136, source
1069e8bbcc1ef97d96ac9d81d93e4d318b341934, release 429777651.
The official releases listing includes v1.17.137 with prerelease=true.
The stable bridge was published first. This beta uses the same
official repository and strict binary-only update package.

## Implemented
This first beta optimizes only responsiveness during rapid terminal
pane resizing. Repeated ResizeObserver events avoid redundant stage
measurement when all visible xterm panes have a fit scheduled. A fit
still runs against the latest layout at the existing bounded deadline.
Highlighter decoration refresh is skipped when xterm rows/columns do
not actually change. Supported browser windows defer noncritical
chrome redraws through the existing coalescing path. The user-facing
layout, remote PTY resize, input pacing, SSH sessions, output history,
credentials, workspace transfer and updater protections are unchanged.

A user on v1.17.135 must first install stable v1.17.136, then opt
in through App settings > Updates > Beta updates. This preference
is off by default and never triggers download or install on its own.
With beta enabled, Check for updates can select v1.17.137; Download
update and Install and restart require explicit confirmation. Turning
the beta preference off resumes stable-only checks, without an
automatic version downgrade.

## Exact release validation and published asset metadata
The exact-source release workflow passed application Go tests,
1,516 JavaScript tests (zero failures), 44 Python tests,
Windows x64 GUI compilation and version/resource regeneration.
Its source-inclusive portable package verified 1,275 safe members.
The binary-only update ZIP retained the strict updater format.
GitHub reports four assets and SHA-256 digests:
- Network_Toolkit.exe: 21,092,352 bytes;
  75ef8df29a7e1f346893727b1a7960ccba1f7062533d386da23ba3c89a95e796
- Network_Toolkit_1.17.137_Portable.zip: 14,369,865 bytes;
  7a3312ba1fb794c421f3b4183f27d8572d982e3068960f840e107245e431b595
- Network_Toolkit_1.17.137_Update.zip: 8,061,759 bytes;
  adbb23ad23acb198eeae41efa5a7487b597f381727b67320493f0df916796a31
- SHA256SUMS: 292 bytes;
  b01ee01a0fc4a0c47d17ae23d57de01cc65018d10dc053892c202980c2ef64cb

The reported asset digests, source/tag and stable/latest feed are
verified through GitHub. No separate external byte-by-byte public
download and rehash was completed. Synthetic JS/Go validation is
not physical-device timing evidence and does not establish a measured
performance improvement on every system.

## Preserved failures and limitations
The initial 137 release attempt 41179289940 did not publish:
TestBeta136DownloadRequiresMatchingOptedChannel used a same-version
stable fixture. The updater correctly rejected a same-version
download, and the fixture was changed to a genuinely newer version.
The corrected exact-release build 41179423624 passed and published.
This history is retained rather than mislabeling the original run
as successful.

At release verification, the separate post-merge Checks workflow
41179423608 remained partially in progress. Passed checks do not
convert pending jobs into success. Earlier Editor comparison motion,
PowerShell echo, native Explorer/SFTP drag and detached window CI
failures remain historically recorded and are not claimed fixed.
No live SSH hardware or physical serial devices were tested.

This is a post-publication documentation-only record. It changes
no v1.17.137 tag, release content, asset bytes or updater feed.

---

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
