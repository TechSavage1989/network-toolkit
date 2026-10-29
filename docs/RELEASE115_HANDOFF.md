# Network Toolkit 1.17.115 published and verified

Published 2026-10-29T20:44:46Z:
https://github.com/TechSavage1989/network-toolkit/releases/tag/v1.17.115
Release ID: 428375650
Exact reviewed/published source and tag: 1a26e0168eb1e8af0cc46f6480090bacf7c8d6b3
Tree: d29fe28ca9aa46570ce26856496846ce69b758a1
Baseline main: 69624b92155817accf5f344739aa0ccc5968fc80 (completed114 closure).
Public verification completed 2026-10-29T20:46:10.984623+00:00.
The user requested this batch on2026-10-29 at18:03:55UTC and explicitly approved
public source/release publication plus the npm dependency-metadata audit at18:40.
Repository visibility/access is unchanged. The114 note was archived unchanged;
115 was the sole root release note at publication;116 later archived it unchanged.
This historical record is included in the six-document116 closure: the planned
separate115 documentation push was deferred for the urgent Terminal hotfix.
No115 public asset, tag or release note is rebuilt or replaced.

## Delivered behavior and limits

- Named drive buttons show available volume labels and drive/mount identities,
  with clear fallbacks and unavailable-drive errors. The permanent, collapsible
  editor sidebar separates Files browsing from Connections setup/saved/active
  session controls. Windows Files/SFTP remain browsing selectors. Narrow windows
  auto-collapse; opening the drawer is explicit, retaining documents and forms.
- Compare Options stays in its original toolbar. Optional horizontal scrolling
  synchronization works with wrap off in both directions; a shorter peer clamps
  without pulling the source back. Vertical alignment remains independent.
- Added/removed/changed gutter symbols match the legend without advancing real
  line numbers on spacers or entering copied/saved text. Changed-text underline
  and extra highlighting are independently configurable; line colors remain.
  Actual fractional width, line height and painted source/widget geometry are
  used for resized wrapped alignment; edits, undo and exact source bytes remain.
- Ordinary captured-pointer tab dragging reorders within a strip or deliberately
  detaches on release outside the window, through the existing acknowledged
  ownership handoff. Escape cancels. Alt+drag retains cross-window HTML5 movement;
  explicit Detach and Move / merge remain. Ordinary clicks activate only the
  requested tab. Unsupported bindings remain in their original window.
- Graph wheel-down zooms inward by default, with a persisted inversion setting;
  timeline wheel still pans. Inward limits retain at least five source samples
  and two seconds when available, with honest short/startup history. Outward zoom
  remains usable at the limit. Fit/reference scale is normalized before applying
  the floor;10-second recent density, curve/cluster/endpoints/clipping remain.
- Existing16MiB encoded files and bounded large-file/comparison modes remain.
  Local/SFTP native file/folder copies still require review/staging/collision
  choices; FTP/FTPS retains manual transfers. Detached access requires explicit
  main-window approval and receives no credentials. Existing dirty/failed-save,
  vault/host-key, Terminal/pager/macros, settings, icons and updater guards remain.

Guides: EDITOR_FILES115.md, EDITOR_COMPARE115.md, EDITOR_DRAG115.md,
GRAPH_ZOOM115.md; retained114 transfer/capacity/native limits still apply.
No private configurations, production devices or user-desktop automation were
used. Hosted browser/Windows fixtures are synthetic, not a live-device fleet,
user reboot/installation test, independent security audit or universal latency
promise.

## Exact-source acceptance

Checks40948378207: all seven jobs SUCCESS on the source above:
https://github.com/TechSavage1989/network-toolkit/actions/runs/40948378207
- browser-workflows107: 123709632008 SUCCESS (2026-10-29T20:34:15Z).
- check: 123709632383 SUCCESS (2026-10-29T20:23:42Z).
- browser-editor112: 123709632429 SUCCESS (2026-10-29T20:30:11Z).
- dependency-security: 123709632456 SUCCESS (2026-10-29T20:22:41Z).
- browser: 123709632462 SUCCESS (2026-10-29T20:39:54Z).
- browser-security: 123709632589 SUCCESS (2026-10-29T20:23:39Z).
- windows-terminal: 123709632639 SUCCESS (2026-10-29T20:27:20Z).

Local integration:1,282 JavaScript tests and42 Python tests passed; the four
relevant Go1.26.10 race packages passed during integration. Final hosted checks
run the complete repository suite against the exact source, not an earlier tree.

Actual rendered/native evidence includes:
- Files six main/child/normal/narrow/resize contexts, eight checks and14 images;
  no horizontal overflow. Directory heights201/363px main and311/400px child.
- New comparison eight layouts with real horizontal scrollbar/wheel gestures,
  symbol/independent emphasis preferences, no Options-trigger displacement.
- Ordinary click/drag four browser layouts plus five actual Windows gestures,
  maximized/restored windows, dirty/undo/source ownership and ACK preservation.
- Historical toolbar36 cases and six functional scenarios retain all original
  height/overflow thresholds with both versions explicitly collapsed through the
  real Files control. Workspace widths match after the intentional27px rail.
  Main/child comparison gains41.1875px versus112; modal gains7.5px or9.5px.
  Default-open115 Files behavior is tested separately, not hidden to claim space.
- Legacy aligned comparison eight cases,36 divider cases, and48 sustained motion
  gestures /1,920 inputs: zero unanswered, all moving frames paired; maximum
  response69.2ms/four animation frames. Maximum map P95 offset0.859375px, with
  an instantaneous3.671875px sample retained honestly; matching anchors0px.
- Graph eight new normal/narrow/light/dark layouts plus startup/sparse/persistence;
  legacy94-98 and114 navigation/density/Follow behavior pass. File-start12 cases
  pass actual tab selection, caret/scroll origin, background loads and no writes.
- Windows152 top-level tests pass, including real three-route WebView2 private
  bootstrap10.42s, vault captures0.43s, capture navigation/cancel spy0.03s and
  native close10.43s. Close uses actual owned OK ID2 and preserves editing after
  Cancel, without invoking OnClosing prematurely. Native file/drive/Terminal
  and handoff regressions remain required and passed.
- Native updater timers pass; five-minute scenario302,276ms, measured check-start
  interval300,011.957901ms, maximum one overlapping check. Scenario duration is
  not the polling interval; the released64 negative control reproduces its bug.
- Offline comparison bundle reproduces; full/runtime npm audits zero findings.
  Linux/Windows Go scans report zero reachable/imported-package vulnerabilities.
  One unused OpenPGP module advisory GO-2026-5932 remains disclosed, unchanged.

Earlier failed candidates are retained, not relabeled green: real wrapped
fractional/virtual geometry, floor zoom state and ordinary tab-click capture
bugs were corrected and rechecked. A historical toolbar comparison originally
used unlike Files widths; only that fixture was aligned, with thresholds kept.
A focused native-close timeout's original cause remains uncertain; subsequent
bounded actual-dialog tests passed, not a retroactive claim it was harmless.

## Published package and feed verification

Automatic Release40951375829: build123720076241 and publish123721283981 SUCCESS.
https://github.com/TechSavage1989/network-toolkit/actions/runs/40951375829
All four public downloads were independently fetched and rehashed. The portable
archive has1,129 members and1,122 exact source files; both manifests and CRC pass.
Public EXE/portable bytes match the final reviewed artifact. PE is AMD64 GUI,
version1.17.115, with nine exact canonical ICO payloads and twelve resources.
Latest feed, source/tag/main and release body agree with the current root note.
The updater ZIP contains exactly the versioned EXE, SHA256SUMS,
THIRD_PARTY_NOTICES.txt and UPDATING.txt, without source, settings or credentials.

- Network_Toolkit.exe: asset689855279, 20,705,792 bytes; SHA256 `60acb4c2088a36130011bc6bf144d7637ab4d643771c3a5d6050f3fa9c026d0d`.
- Network_Toolkit_1.17.115_Portable.zip: asset689855278, 13,654,132 bytes; SHA256 `70c6b38235b7abbdfcd834826a156ce77c23fc505083dd13233385974a5a634c`.
- Network_Toolkit_1.17.115_Update.zip: asset689855280, 7,922,195 bytes; SHA256 `a0f6af87f91eca876277aa98552a007f9e6b29374abbe241917ea987f08b0f2a`.
- SHA256SUMS: asset689855276, 292 bytes; SHA256 `489193734c372029c49ac18c8ae54e01952b2436013be2fbf643e9b8748e15e9`.

Prior114 remains unchanged: tagfd6cbbf287738320b51957d176dd2e63c63eece9,
release428199851, body and all four original asset IDs/sizes/digests/URLs/
timestamps match before/after promotion. The archived114 note remains byte-for-
byte SHA2563a8d6436183bbd3fa909e45c7a99901e8555e308bde5baa897b079ed60cddfcd.
No old tag/asset or repository access was changed.

## Duplicate main and documentation closure

Automatic main Checks40951375749 completed with all seven jobs SUCCESS:
https://github.com/TechSavage1989/network-toolkit/actions/runs/40951375749
- browser-editor112:123720076129 SUCCESS.
- check:123720076578 SUCCESS.
- dependency-security:123720076594 SUCCESS.
- browser:123720076616 SUCCESS.
- browser-workflows107:123720076624 SUCCESS.
- browser-security:123720076681 SUCCESS.
- windows-terminal:123720076685 SUCCESS.
The saved terminal inventory was independently rechecked before this record was
included in116 closure. It is not substituted for later116 background results.
The planned separate five-file115 docs sync was deferred for the urgent116
hotfix; this handoff is instead one of its six documentation-only files.
Published115 source/tag/assets remain unchanged.
