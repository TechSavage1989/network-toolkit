# Network Toolkit development handoff

## 1.17.90 authorized UI consistency update

User authorized implementation, build and publication of larger proportional result headers, two-way main-sidebar tab double-click, a more visible/wider divider, and consistent Status colors/shared heading actions. Preserve previous fixes. Publication pending full validation.

## 1.17.89 published Local Network layout and refresh fixes

The user explicitly authorized building and publishing these fixes.
Published source/tag: eb318708454130b09d93204021b23b138657d6a2
Tree: 9be08c4672db7b0bfc6ea08360c137329f22ef44
Official release: https://github.com/TechSavage1989/network-toolkit/releases/tag/v1.17.89
Release ID: 423987946

Local Network's Status colors control now belongs to the stable page-heading
actions. It is no longer inserted as a separate row above the local tabs during
full-page rendering. Initial navigation, tab selection and direct renderLocal
updates therefore retain the same tab and content positions, with one Status
colors control. The heading override removes that control's old row margins.

Explicit selection of Routing table, ARP / neighbors or Connections starts a
fresh local snapshot through the existing startJob path, including another
click on an already selected tab after its previous snapshot completes.
Rendering, job polling and main-page navigation do not trigger automatic scans.
IP configuration and Wi-Fi keep their existing refresh behavior.

The selection helper checks the active local page, supported subtool, executable
mode, engine availability, shutdown state, pending POST and running job. A
per-tool pending-request Set is updated synchronously before POST and released
in finally, including on errors. Rapid clicks and clicks during an active job
do not create duplicate snapshots. Local Run controls reflect pending requests;
offline selections make no diagnostic requests. A failed refresh can be retried.

Full Checks 40289268615 passed application 121460230971, native
Windows 121460230960 and browser 121460230557. The required
JavaScript tests, release guards, root/terminal/desktop/diag/deviceinfo Go suites,
native cmd/PowerShell color and interactive lifecycle, installer progress and
Windows GUI/resource build/package checks pass.

The new Local Network browser report passes all 5 groups and 12 initial/repeated
layout cases across desktop, compact and narrow widths in dark and light themes.
It verifies stable positions and a single heading Status colors control, fresh
snapshots on selections, pending/active deduplication, refresh after completion,
render/poll/offline suppression and retry after a failed request. The previous
88 layout report passes all 4 groups / 9 cases. All previous covered editor,
shell, copy, Hosts, client/server, SMTP, folder and selection regressions pass.
All 7 native updater timer cases pass, including the real five-minute poll.
Current reports have no runtime or asset failures; the released64 negative
control's expected Illegal invocation is retained as a regression control.

Release 40290524884 passed build 121464243288 and publish
121464741674. All four actual public assets match GitHub digests
and SHA256SUMS. EXE/portable match the reviewed local build; standalone, portable
and update EXEs are identical. All 749 packaged source files match the tag.
ZIP CRCs, manifests, PE GUI/version, strict update inventory and
14 vendored editor-library digests verify. All
28 older releases/assets and 28 older tags remain unchanged.

Released88 production handlers checked/downloaded/verified/staged live89 using
isolated settings through real HTTPS in 14.56s. The staged 17600000-byte
candidate matches the independently verified official executable. Original test
EXE unchanged. No actual installation, restart or full native WebView2 UI
interaction was performed.

Review artifact 12635079929: 40fe36a4c034e81788a9f4fb58c4c766b2f5e048380e8b63916b9b75cddba577
Browser artifact 12636036648: 12f23fd71224502f9a941702408b3c252640ed1a21b4ae888c630d1855107c2e
Final screenshots inspected: local89-ipconfig-1440.png, local89-routes-1440.png, local89-ipconfig-390.png, local89-routes-390.png
Network_Toolkit.exe SHA256: e157e71b8b6ed64eb88c1ad393df17cc69fa85541109cfba40d8b63b4da1ebc7
Network_Toolkit_1.17.89_Portable.zip SHA256: aa7c6f12bb378d4e3ec84b1021ba5d3a7b963b3668c33e070b705de399578937
Network_Toolkit_1.17.89_Update.zip SHA256: 412b708f49751afdb440beeb77d769c5bc3ab71e93297ef0ab88c8f63950a255
SHA256SUMS SHA256: 0e6f0867f10ea84361bd3b465380a261c7d750902b24367c57134f12da265c15

Main network-toolkit remains the sole public source/update feed. Do not move old
tags, replace assets, change access or write the retired feed. Publication is
complete; future releases require authorization. This final three-file
documentation completion uses [skip ci], preserving published source, runtime
and release assets. Continue from latest main and preserve previous fixes.

## 1.17.88 published layout and navigation update

The user explicitly authorized build and publication of all confirmed changes.
Published source/tag: 4a99e2b51042dbc6291d1d2b4c2708fc0c4ac377
Tree: 84a63470de6e7f760f955272fe076a6f7e4478e1
Official release: https://github.com/TechSavage1989/network-toolkit/releases/tag/v1.17.88
Release ID: 423968146

Remote editor Browse/Open remote uses one table for aligned headers and details,
with sticky headers inside the same scroll region. Headers show sort direction
and aria-sort; sorting keeps folders first, supports both directions for Name,
Type, Size and Modified, preserves file selection and keyboard focus, and formats
modification timestamps locally. Keyboard Enter opens rows and Space selects.
File/folder double-click and existing connection/open actions are preserved.

Public IP is inside Local Network's scrollable IP configuration content and also
on Overview beside the network context. Both views share the transient cached
IPv4/IPv6 result, refresh/copy controls and partial-failure feedback. The public
address follows the computer's current Internet route, including VPN routes; it
is not the selected adapter's private IPv4 address and is not persisted in state.

Possible networks is compact and directly below the calculator in the left
column, beside details on wide screens, stacking on narrow screens. Its table
has bounded scrolling and sticky headers; filtering, order, copy, CSV and current
subnet actions remain. The calculator retains compact spacing.

Collapsed main navigation supports double-click on a tool icon to expand, even
when the first click redraws navigation. Single clicks still navigate without
expanding. The brand/status icon also supports double-click. The divider has a
visible line and grip; its existing double-click/keyboard toggle remains.
Local Network tabs use 13px text and proportionate 18px icons.

Full Checks 40286388008 passed application 121451147242, native
Windows 121451146928, browser 121451146573. 510 JS tests,
31 Python release guards, all root/terminal/desktop/diag/deviceinfo Go suites,
native cmd/PowerShell color and interactive lifecycle, installer progress and
Windows GUI/resource build/package pass. Local container network-binding tests
could not complete due blocked netlink operations; all passed on hosted Linux.
New browser layout checks cover 4 groups and 9 desktop/compact/narrow layouts;
all prior editor/shell/copy/Hosts/client/server/SMTP/folder/selection regressions
and 7 native update timer cases including the real five-minute poll pass.
Final layout screenshots inspected; no current runtime errors in reports.
An initial browser check caught an inherited planner margin; final source fixes
that margin and restores compact input-panel spacing.

Release 40287626301 passed build 121454930033 and publish
121455460993. All four actual public assets match GitHub digests
and SHA256SUMS. EXE/portable equal the reviewed local build; standalone, portable
and update EXEs are identical. All 747 packaged source files match the tag.
ZIP CRCs, manifests, PE GUI/version, strict update inventory and 14 vendored
editor-library digests verify. All 27 older releases/assets and 27 tags remain unchanged.
Released87 production handlers checked/downloaded/verified/staged live88 using
isolated settings via real HTTPS in 12.04s. Original test EXE unchanged.
No install/restart or full physical Windows UI interaction was performed.

Review artifact 12633669616: 62cf10a3e50653e7009b44d160a849dd68404e2a4fe738c1d40607975cb98b93
Browser artifact 12634926316: 613c40244aaebb52ed5a2b4689aef1dbb0f8da234e78380fdab8fb484b48423c
Network_Toolkit.exe SHA256: 30972651b79fcfee0ceb5019dd8ad5553c4b165db324906cd32bbd2e7c7b472d
Network_Toolkit_1.17.88_Portable.zip SHA256: 0b9ef31e65d6ec1d9ed70213a475332e5699cfafd5b940a02ac4b1b5602af802
Network_Toolkit_1.17.88_Update.zip SHA256: 1f5ca4d8f600963d652ba03daf0fda367ac46ab55fbe93468cda67d2e60ccc90
SHA256SUMS SHA256: 8eac3b5a55da67e629f308878b7cfce245ff0e22242f0980e98d55eaa4100bbc

Main network-toolkit is the sole public source/update feed. Do not move old tags,
replace assets, change access or write the retired feed. Publication is complete;
future releases require authorization. This final three-file documentation
completion uses [skip ci], leaving published source/runtime/assets intact.

## 1.17.87 published update

1.17.87 publication verified
Source/tag: d7cd098b8ab639fc418954b38047fe254ab4ac5e
Release: https://github.com/TechSavage1989/network-toolkit/releases/tag/v1.17.87
Full Checks 40280780231 passed. Initial Windows lifecycle interruption timed out; retry passed without source changes. 510 JS tests and browser/Windows/build checks pass. Direct screenshot empty-field/heading checks and all four picker sort columns in both directions pass, preserving selection. Browser artifact 12632209025 verifies with no current runtime errors. Release 40281905179 build 121436851024 and publish 121437289706 passed.
All four public downloads match GitHub digests/checksums, local EXE and portable package. All 745 source files match the tag. All 26 older releases/assets/tags remain unchanged. Released85 production updater checked/downloaded/verified/staged 87 in 11.11 seconds. Original executable unchanged; no install/restart performed. Full physical Windows UI interaction is not claimed.
EXE SHA256: e05b4c60010e1f870cc3b7c736b3d38fef06c400c5299628398d363692621dcf
Portable SHA256: 9caa907135e62d4dc9ebed5db45c035b908a73e2ab0a55e684878d29219c78e8
Update SHA256: 5757aef8d84f56355954c0219aea7e81dfef4660ed79c6f78c186fd0b576d51b
Publication authorized by user. Do not replace tags/assets or write the retired feed. Final documentation uses [skip ci]. Future releases require authorization.


User requested shell coloring corrections and sortable remote editor directory details, with publication. Empty CMD labels now color; adapter headings remain headings. Remote structured banner labels use existing field colors. Remote picker sorts Name, Type, Size and Modified, toggling direction while retaining selection. Native ANSI and underlying terminal content remain unchanged.

## 1.17.86 published: editor corrections and structured shell output

The user explicitly requested "publish the build", lifting the earlier hold for
these changes. Publication is complete. Future work needs its own authorization.
Main network-toolkit remains the sole public source/update feed. Preserve all
older tags/assets and repository access; never write the retired feed.

Published source/tag: 5e1e303066e8cd89b1b2e4ce06c204566d57e2e3
Tree: 2c16347f61593ca746f3b90218f4e2554418f9ba
Main fast-forwarded without force from 2914db942015742a26d03c9822f1a2efe6645fab.
Full Checks 40278209364 passed check 121425449651, native
Windows 121425449827 and browser 121425449949. Release
40279270997 passed build 121428705226 and publish
121429161901, creating stable release 423914966.

The editor preserves selections and both scroll positions for case changes,
line tools, sorting and undo/redo. Wrap retains worker syntax colors, with an
aligned overlay and measured logical-line gutter; a fixed gutter frame prevents
line numbers disappearing on scroll. Matching unitless line heights avoid drift
through deep wrapped documents. In-app dialogs replace blocked browser prompts
for Close, Reload, Save As and Go to line (Ctrl+G). Cancel/Enter/Escape and stale
revision checks preserve unsaved work. New local documents acquire a real reader
and canonical local IDs after Save As. Late saves keep newer edits dirty and
cannot steal another tab's focus. Find typing keeps the query field focused.

Local PowerShell and Command Prompt recognize output headings, fields, values,
validated addresses/MACs and status words, including ipconfig-style results.
Seven output-role colors are configurable separately for each shell in Display,
follow the terminal palette and preserve native ANSI by default. Unrecognized
prose and full-screen programs keep their rendering. These are recognition cues,
not a health judgment for Yes/No fields. Copies, buffer cells, input and transport
remain unchanged. Output colors persist through Go sanitization and detached /
standalone terminal settings transfers. All 85 and earlier fixes are preserved.

Full validation: 510 JS tests, 31 Python release guards, root/terminal/desktop/
discovery/deviceinfo Go suites, Windows x64 GUI/resource build and package pass.
Native ConPTY typing/lifecycle and visible installer progress pass separately.
All browser steps pass, including the new editor/output suite's 5 groups / 6
layouts, editor open/import/reload/connected-source lifecycle, existing 84/85
features, all earlier UI/copy/Hosts/client/server/SMTP/folder/selection checks and
all seven native update timer cases with the real five-minute poll. Native script
dialogs deliberately throw in editor tests; all editor controls use built-in UI.
Actual DOM text colors are checked for both shell types using synthetic addresses.
Current reports have no runtime/asset errors; released64's negative-control
Illegal invocation is expected. Final editor and shell screenshots inspected.

All four actual public downloads match GitHub asset digests and SHA256SUMS.
Portable ZIP and EXE equal the reviewed/local package. Standalone, portable and
update EXEs are identical. All 744 packaged source files equal the tag;
ZIP CRCs, both checksum inventories, Windows x64 GUI/version, strict update
allowlist and all 14 vendored editor-library digests pass. All
25 older releases' asset IDs/names/sizes/digests and 25 older tag SHAs
remain unchanged.

Unchanged released85 handlers from 427e50d765835f632f44ca4a9b185241169dd91c
checked/downloaded/verified/staged live86 via real HTTPS in 10.97s, using
isolated settings. The staged 17593344-byte candidate matches the independently
verified official EXE. Original test EXE unchanged; no install or restart. Users
can open App settings, Updates, Check for updates. An identical 86 review build
already has this version and correctly sees no newer update.

Review artifact 12630839084: 3746137e97c837df4d86f0dc5448c521acb383cfefae33473f59ae3d734d4eff
Browser artifact 12631801051 (128 members):
5aa8a10ff472e232c071c7c52e03ee1d95ad0347df9fb9c2ecf1c6ede44cb270
Official release: https://github.com/TechSavage1989/network-toolkit/releases/tag/v1.17.86
Network_Toolkit.exe SHA-256: d998fe81dbdc99c003d929d6f60ca42f790bfec5ecd8addbe59a6347ebdf38bb
Network_Toolkit_1.17.86_Portable.zip SHA-256: d48a5a1fb6d8313b685a65150ba06e8f54cb3f3e7d46aa66d9660ad078784041
Network_Toolkit_1.17.86_Update.zip SHA-256: d7ca28e2e30fb2593c3a751362b4bc9d04c4fd6b70a6fdfd749df4abd78bc4cb
SHA256SUMS SHA-256: 31370619f731ed3219f4c37f81af6d376cde3f5023f70ac31ef13478833ad299

No full native WebView2 UI interaction, physical hardware or actual installation /
restart was tested. This completion changes AGENTS.md, this handoff and
VALIDATION.txt only with [skip ci]. Do not move the 86 tag, replace assets or
republish. Continue from latest main and preserve previous fixes.

The build hold recorded below was subsequently lifted for published 86.

## Historical source-only review before the authorized 86 build

The user explicitly said not to proceed to a new build until told to do so.
Published 85 remains unchanged. Work is on fix/editor-shell-review-no-build,
without a version bump, executable/package build, main push or release.
Checks excludes this exact source-review branch. Its separate source workflow
runs JS tests and static browser fixtures only, without Go, Windows resources,
application compilation, packaging, install or publication. Normal Checks for
other branches are preserved. When a future build is authorized, use a normal
review branch and run the full required native/Go/build checks.

The user's screenshot shows a colored cmd command and monochrome ipconfig
results. The old renderer deliberately recognized only prompt syntax. Source
changes add recognized output headings, field names/values, validated addresses,
MACs and status words for both local shells. Seven explicit output-role colors
follow the terminal palette, preserve program ANSI by default and have per-role
controls. Free prose and full-screen programs keep their formatting. These are
recognition colors, not a health judgment about Yes/No or DHCP settings.

Editor source changes preserve selection and both scroll positions across case,
line transforms, sorting and undo/redo; sort respects selections ending at a
newline. Wrapped syntax remains active with the textarea's client width, and
logical-line gutter heights are measured using the same font/wrap/tab metrics.
Resize/font changes remeasure the wrapped gutter. In-editor dialogs replace all
native prompt/confirm calls for dirty close, close-all, reload, Save As and Go to
line (also Ctrl+G). Cancel preserves the docked editor; stale document/revision
checks guard discard/reload. The embedded Close button now actually closes the
editor. Empty-editor actions are guarded. Find typing keeps query focus, and
wrapped search uses measured logical-line positions. Late saves retain newer
edits as unsaved and cannot activate a different tab on completion.

Source validation COMPLETE on 947a65a422280afaffeafb395169a49a298f98e2,
tree bff0be0f6993ec2c97e20d12905de7be6064715f. Source-only workflow
40277446727 / job 121423082359 passes: all 510 JS tests and four static browser
scripts. New editor/output tests have 5 groups and 6 dark/light layouts at 1440,
820 and 390 pixels; file-open/import/connected-source/reload tests cover 12 cases;
existing 84 and 85 browser suites also pass. Current reports have no runtime
errors. Native script prompt/confirm functions deliberately throw in the new
browser fixture, proving editor controls use the in-app dialogs instead.

The gutter now has a fixed clipping frame and separately translated line-label
content. It stays visible while scrolling, including wrapped logical lines.
Measurements use the same unitless line-height as the textarea to avoid rounding
drift, checked through logical line 150. A long wrapped line can naturally have
its label above the viewport when only its continuation is visible. Final wide /
compact editor and PowerShell output screenshots were visually inspected.

Newly saved Untitled documents now use the real local-file reader for Reload,
and local Save As IDs match normal local-file opens. A late save preserves newer
edits as dirty; keyboard Enter on Cancel and Escape preserve unsaved files.
Source regression fixtures use synthetic documentation-range IP addresses. The
new output roles are included in both JS normalization and Go's settings allowlist.
The Go whitelist has been formatted but not compiled under the build hold.

Browser artifact 12630099280 has 22 verified members (digest, CRC, report checks):
acbff75c63224c2ab5543159f33dee05beb0594362f958fa5648af4f24a29442
Screens inspected: editor-review-1440-dark.png, editor-review-390-light.png,
shell-output-review-powershell.png. Actual DOM text colors are checked for both
local shell types, preserving native ANSI red and buffer bytes. Rendering checks
wait for the native xterm repaint rather than assuming a decoration element alone
means its foreground is already painted.

No version/resource bump, Go/Windows application build, package, installation,
restart, main push, tag change or release was made. Published 85 remains current;
its existing local EXE still has SHA-256
94f2c1c7abfd76434fbafafbea806010eb3d7c67d509ec4eeb6d8b5a13ae5c8e.
Full Go/native Windows/WebView2/build/package validation must run after the user
explicitly authorizes a new build. Use a normal review branch then, bump the next
version and preserve old tags/assets; do not replace 85. This final source-review
record changes documentation only and uses [skip ci]. The user's build hold
remains active; do not infer authorization from these passing source tests.

## 1.17.85 published: shell colors, close preferences and tool navigation

The user explicitly authorized publishing the completed 85 update. Publication
is complete. Future releases need their own authorization. Main network-toolkit
remains the sole public source/update feed; preserve old releases/assets/tags,
repository access and the retired feed.

Published source/tag: 427e50d765835f632f44ca4a9b185241169dd91c
Tree: 358b647ddf32872ab6a6ca5a5bd9ee185790da54
Main fast-forwarded without force from b0ee0c9eb1ed171d834eef7aae944506f65246c2
to the exact runtime verified by full Checks 40271392085.
All three jobs pass: check 121404772023, native Windows
121404772187 and browser 121404772175.
Release 40272271825 passed build 121407447605 and
publish 121407837540, creating stable release 423876100.

All four public downloads match GitHub asset digests and SHA256SUMS. The portable
ZIP and standalone/portable/update EXEs equal the reviewed package and local
build. All 741 packaged source files equal the tag; ZIP CRCs, both checksum
inventories, Windows x64 GUI/version, the strict four-member update allowlist and
14 bundled editor-library file hashes pass. All 24 older releases' assets
(IDs, names, sizes, digests) and 24 older tag SHAs remain unchanged.

Unchanged released84 production handlers from source
3e1aa2ee2bca8e632e00e7fdc0c52ca1fb7fc766 checked/downloaded/verified/staged
live85 through real HTTPS in 10.41s. The staged 17581056-byte candidate
matches the independently verified official EXE. Temporary isolated settings,
original test EXE unchanged, no installation/restart. Users can open App settings,
Updates, Check for updates. An identical 85 review build already has this version.

Terminal Display now has separate PowerShell and Command Prompt switches,
previews, nine role colors and per-shell reset. Defaults follow the terminal
palette and preserve distinct native colors. A custom role can recolor that
recognized command token. Local syntax is independent of Highlight device output
across startup/theme/workspace/restore paths. Ordinary program output, custom
unrecognized prompts, alternate screens, buffers/copy/transcripts and input stay
intact. There are no startup/profile/execution-policy changes.

The old default-theme renderer skips syntax foregrounds under SGR 37;40 / 97;40
because ANSI black differs from the app background. The released84 negative
control fails and the fix passes. Parsed local ANSI-black backgrounds now permit
syntax colors without changing backgrounds or remote-device rendering. This is
a reproduced case, not a claim to have captured the user's exact environment.
Native Windows transcript probes capture plain cmd input and native PSReadLine
colors. Samples are typed without Enter, then replayed through real app sessions.
Browser assertions inspect actual per-character rendered colors, including live
custom role overrides over native ANSI; they do not mistake the empty decoration
overlay for rendered text. Early browser assertion corrections changed tests only.

The saved close-confirmation checkbox defaults on in Display. Turning it off
skips active-session and Close other tabs prompts while disconnecting before
removal. Both dialogs offer Do not ask again, saved only on confirmation. Current
ownership/revision guards reject stale or transferred targets. Saved/Recent entry
removal keeps its separate preference and explicit label. Normalized shell and
close settings persist and transfer into detached single/workspace displays and
standalone terminal seeds; malformed/unknown values are discarded.

The collapsed main tool rail remains clickable for navigation. The arrow or
divider double-click / Enter / Space toggles it; ordinary surface/icon clicks do
not reveal it. The divider is a toggle boundary, not a new drag-resize feature.
The compact-screen drawer keeps navigation, close, focus and inert behavior.
Display settings are clamped inside the viewport; the new browser layout test
reproduced the former left-edge overflow at 820 pixels before this fix.

503 JS tests, 31 Python release guards and required root/terminal/desktop/
discovery/deviceinfo Go suites pass. Native Windows ConPTY startup/typing,
interactive output/resize/Ctrl+C/shutdown and native installer progress pass in
separate steps. Browser's six new groups cover actual local sessions, white/
black attributes, native token overrides, independent switches, persistence and
workspace settings, default/cancel/opt-out/restored single/bulk close dialogs,
six shell-settings layouts and collapsed/compact tool navigation. Existing 84
editor/workers/formatting/colors, 83 macros/discovery/copy/progress, 138 UI layouts,
17 actual-app selection regressions and all previous browser steps pass. All seven
native Chromium update timer cases pass, including the real five-minute poll.
The released64 negative control's Illegal invocation is expected; current covered
reports have no runtime/asset failures. Final settings/console screenshots were
inspected. Earlier local source-binding limits are covered by passing hosted Go.

Review artifact 12627993521 outer SHA-256: 2b0f8cd7cf105c31320ffc189b45154a773f4ec893c02ab614ebd6675b7c9651
Browser artifact 12628067982 (121 members) SHA-256:
7dcf4f86a7290d027aa20a6eea006de6898d27428eeed51d23ea59a1ad2c1d02
Official release: https://github.com/TechSavage1989/network-toolkit/releases/tag/v1.17.85
Network_Toolkit.exe SHA-256: 94f2c1c7abfd76434fbafafbea806010eb3d7c67d509ec4eeb6d8b5a13ae5c8e
Network_Toolkit_1.17.85_Portable.zip SHA-256: 16ea7a4697c086b07ed319de0cc32c339735e3c63b14691a491e00df0c31e935
Network_Toolkit_1.17.85_Update.zip SHA-256: 67e5c2691290eecd2e009c3bdf44a0fc6258e8cc06b57e899ab836531be785c1
SHA256SUMS SHA-256: 2d594618dd45d58e985e39a11ed701b7787e61a87873a65b92ad2b0cfb7e55dc

No full native WebView2 interaction, physical hardware or actual installation /
restart was tested. This final record changes only AGENTS.md, this handoff and
VALIDATION.txt with [skip ci]. Do not move the 85 tag, replace assets or republish.
Continue from latest main and preserve all 84 and earlier fixes.

## 1.17.84 published: editor languages and local shell syntax

The user explicitly authorized publication after receiving the verified 84 review
build. Publication is complete. Future releases need their own authorization.
Main network-toolkit remains the sole public source/update feed. Never change
repository access, write the retired feed, replace older assets or move old tags.

Published source/tag: 3e1aa2ee2bca8e632e00e7fdc0c52ca1fb7fc766
Tree: 2d4c033c40ac2105491c88eb1caadb03c775907c
Main fast-forwarded without force from ce5bf2d0f0c452fb0089703ab1fe15f5b61d7879
to the exact source already validated by full Checks 40264715184. Release
40266573099 passed build 121390177026 and publish 121390518234, creating stable
release 423845092. The duplicate automatic main Checks can run independently
on that identical runtime; the completed review is its full runtime validation.

All four actual public downloads match GitHub asset digests and SHA256SUMS.
Standalone/portable/update EXEs are identical and equal the reviewed EXE; the
portable ZIP equals the reviewed build. ZIP CRCs, outer/inner checksum manifests,
Windows x64 GUI PE/version and the strict four-member update ZIP pass. All
738 packaged source files equal the exact published commit. The IDs/names/
sizes/digests of all 23 older releases' assets and all 23 older tag SHAs
remain unchanged. This is a new version, not replacement of an earlier release.

Unchanged released83 production feed handlers from source
a384a47d5b8782fb821eda492fa9ad92a3ca540d used real HTTPS to check/download/verify/stage
live84 in 11.79 seconds. The staged 17567232-byte candidate hash equals the
independently downloaded official EXE. Isolated temporary settings were used;
the original test executable stayed unchanged. This tests the platform-independent
feed handlers without installing or restarting. Users can choose App settings,
Updates, Check for updates. A user running the identical 84 review build already
has this version and correctly sees no newer version after publication.

Official release: https://github.com/TechSavage1989/network-toolkit/releases/tag/v1.17.84
Network_Toolkit.exe SHA-256: 4086c823766e960c67de29f1661bf3d3f55b4cd675dfff60d30db43cf7c9a6ef
Network_Toolkit_1.17.84_Portable.zip SHA-256: 4911a592dc20bc9192fd53043778ef73a45fc5d698f498b074731ec270ebf1b4
Network_Toolkit_1.17.84_Update.zip SHA-256: 1b826f6224c169e87fd9157d0597980680dc30d3f4833cb3791fd98f5be7c0fd
SHA256SUMS SHA-256: 90ed94eb03886ee327e09c9af507bcade5d3eba7ff315cdb64d8d488aaf837da

Exact tested build source: 3e1aa2ee2bca8e632e00e7fdc0c52ca1fb7fc766
Tree: 2d4c033c40ac2105491c88eb1caadb03c775907c
Full Checks 40264715184 passed on that exact source:
- check 121384817713: 497 JS tests, 31 Python release guards, root / terminal /
  desktop / discovery / deviceinfo Go suites, resource/version validation and
  Windows x64 GUI build/package.
- windows-terminal 121384817665: separate passing steps; detailed logs inspected.
  Cmd lifecycle passes in 0.54 s and PowerShell in 3.52 s (output, resize, Ctrl+C,
  shutdown and owned children). Native progress-window lifecycle passes in 0.36 s.
- browser 121384817479: all new and existing browser steps pass. Current app
  reports have no runtime/asset failures in their covered cases. The released64
  negative control deliberately reproduces its historical Illegal invocation;
  this is the expected control, not an error in the current build.

The editor offers 300 language choices using 292 bundled Prism grammars plus
HTML/XML, JSONC/JSON Lines, registry, Vue, network config and plain text modes.
File extensions, exact filenames/dotfiles and script headers select the language.
A per-document override or Auto remains available. Save As immediately refreshes
automatic recognition. The official Notepad++ language list informed coverage;
this does not claim full Notepad++ editor feature parity.

Prism 1.30.0 / Prettier 3.9.10 are bundled offline with their MIT licenses/notices,
verified npm tarball integrity and per-file digests in editor-vendor/SOURCE.json.
There is no runtime CDN, package install or external language service. Explicit
asset allowlists and embedding cover all worker/parser dependencies; metadata and
directory paths stay private. Language processing runs in separate local workers
for syntax and formatting, with debounce, stale-document/text/language/generation
checks, timeout termination and worker-identity guards on late errors. File text
is escaped for rendering and is never executed. Opening/highlighting do not rewrite
source, encoding, BOM or EOL, and the existing save/revision safeguards remain.

Explicit parser formatting covers JS/TS/JSX/TSX, JSON/JSONC/JSON5, HTML/Vue,
CSS/SCSS/Less, Markdown, YAML and GraphQL. PowerShell and the listed C-like code
families offer Indent document, which adjusts leading whitespace only, preserves
multiline strings/comments/macros and rejects incomplete delimiters. It is not a
full structural beautifier. Formatting is one undo action, preserves view/file
metadata, and cannot replace a changed, closed or switched document. Enter,
Tab/Shift+Tab selection indentation and language-specific comments are supported.
XML/HTML comments close correctly; Batch uses REM and JSON stays comment-free.
Files above 250,000 characters and word wrap retain plain-text display; the 8 MiB
open limit and document-start behavior remain. Clear controls no longer cover
the editor's source, compact font value or custom color hex values.

Local shell fallback syntax reads parsed xterm cells only, not keyboard events.
It sends no session data and does not rewrite ANSI, input, copy, selection or
transcripts. Standard PS / drive / UNC prompts select dedicated PowerShell/cmd
roles, including variables inside double-quoted strings. Shell-provided colors
take precedence. Fallback colors map to the selected terminal ANSI palette,
update live and have no underlines. Ordinary output and full-screen applications
keep their rendering; custom prompts without a recognized standard prefix keep
shell colors. Transport/startup/user profiles/execution policies are unchanged.
Fit console is removed from toolbar/context choices; automatic fitting remains.

The six new browser scenario groups cover real local workers, language families,
format/undo/invalid syntax/late replies, six editor layouts (1440/820/390, dark /
light), real xterm syntax colors, ANSI precedence, live palette edits and unchanged
buffers. Final wide/compact editor and shell screenshots were inspected; font
size is readable and the source has no overlaid clear control. Existing 138 UI
layouts / 18 copies, 20 Hosts editor cases, scan/macro/local shell/update progress
checks, client/server/SMTP/folders/dialog/command input/editor-start tests and the
17-case real app / loopback terminal selection regression pass. All seven native
Chromium timer cases pass, including the real five-minute poll/focus collision.
Local JS also passes all 497; local selected-source discovery/device tests remain
blocked by netlink permissions, with the full suites passing on the hosted runner.

Review artifact 12626345743 outer SHA-256: df23730f7bc0d6a06a0e3fbddb29fbe25face409a9267e1e412cc173ee246137
Portable ZIP SHA-256: 4911a592dc20bc9192fd53043778ef73a45fc5d698f498b074731ec270ebf1b4
EXE SHA-256: 4086c823766e960c67de29f1661bf3d3f55b4cd675dfff60d30db43cf7c9a6ef (17567232 bytes)
All 738 packaged source files equal the exact tested commit.
Outer/inner ZIP CRC, both checksum inventories, Windows x64 GUI PE/version,
strict four-member update ZIP and 14 vendored file digests pass.
Hosted/local portable ZIP and EXE bytes match. Delivered review ZIP is
Network_Toolkit_1.17.84_Portable.zip in editor84-qa.
Browser artifact 12626476464 has 115 verified members and SHA-256
18d061e828d36aba31044a1d47fb968b1a553ab903171c16fc64e79198f51675; CRC/reports/screens pass.

No full native WebView2 UI interaction, physical hardware or actual installation /
restart was tested. This final record changes only AGENTS.md, this handoff and
VALIDATION.txt with [skip ci]. It must not change the reviewed runtime, move the
84 tag, replace assets or trigger a second publication. Continue from latest main
and preserve all prior fixes.

## 1.17.83 published: folders, local shells, discovery and update progress

The user explicitly authorized publication on 2026-10-26 after reporting no
update appeared. The diagnosis was an unpublished review build: the public feed
still correctly listed 82. Publication of this scoped 83 release is complete.
Future releases need their own authorization. Main network-toolkit remains the
sole public source/update feed. Never change repository access, write the retired
feed, move older tags or replace their assets.

Published source/tag: a384a47d5b8782fb821eda492fa9ad92a3ca540d
Tree: ebec06684689f0ad48cfcd8e5661c6da4a740232
Main fast-forwarded without force from 12156a2897604bc7c78292f76c3aed6c9f227d29
to the exact runtime already validated by full Checks 40258417658.
All three review jobs pass, with detailed native Windows logs inspected.
Release 40260010077 passed build 121370127211 and publish
121370438727, creating stable release 423808053.
Duplicate automatic main Checks can finish independently on that identical
runtime; full completed review is the runtime validation.

All four actual public assets match GitHub digests and official SHA256SUMS.
Standalone, portable and update executables are identical and equal the reviewed
EXE. The portable ZIP equals the reviewed build. Both ZIP CRCs/checksum
inventories, Windows x64 GUI PE/version and strict four-member update ZIP pass.
All 713 packaged source files match the published commit. All
22 prior release IDs/assets/size/digests and all
22 older tag SHAs remain unchanged.

Unchanged released82 production feed handlers (source
0ebd276e651eaf40bca4d80e317f68ff35b650aa) used real HTTPS to check/download/verify/stage
live83 in 11.75 seconds. The staged hash equals the independently
downloaded official EXE. The harness used isolated temporary settings; the
original test executable remained unchanged. No installation/restart occurred.
The user can use App settings, Updates, Check for updates. A user already running
the identical 83 review build correctly has no newer version to detect.

Official release: https://github.com/TechSavage1989/network-toolkit/releases/tag/v1.17.83
Network_Toolkit.exe SHA-256: dd50538a54ea21c2a563497525ad59fd1cc23c18134cce0cd34a34b5026d1698
Network_Toolkit_1.17.83_Portable.zip SHA-256: edcfd279c94eaa3b18662748bf915f959052f65007300505ed9a77e8eea04023
Network_Toolkit_1.17.83_Update.zip SHA-256: cb853b13a2c27ba836baffbe6286c534691a41660baa976e0a58feaadf7a043d
SHA256SUMS SHA-256: 1159e4d0d4ecda425dc85a30949764cd01f7252c21c046af2ffe715062b087b8

Macro folders share the saved-session helper with the existing 120-character
macro limit separate from 100-character session labels. All folders, Unfiled,
named folders, choose/create inputs and direct/context-menu moves preserve IDs,
commands, shortcuts, metadata and startup bindings. Revision/dialog checks and
stale-refresh guards protect completed moves.

Local Command Prompt and built-in Windows PowerShell use ConPTY, trusted system
paths, current user/policies and home-folder startup. They share xterm theme,
font, cursor, ANSI/highlight colors, profiles/recent entries, macros, panes,
reconnect and linked workspaces. Local requests discard remote credentials and
host/serial fields. Children join a kill-on-close job before resuming; output
keeps draining separately from ConPTY shutdown. STARTF_USESTDHANDLES with empty
handles prevents redirected parent streams being duplicated into the shell,
matching Microsoft Terminal's ConptyConnection implementation. Shell fixtures
wait for the initial prompt. Windows test commands are separate steps so a
later passing command cannot mask an earlier failure.

Discovery overlaps up to four independent checks per host and at most 64 scan
operations. Concurrency one and Ping/TCP sweeps keep sequential behavior. Early
immutable reachability snapshots update the same row when metadata finishes;
counts change only once. UPnP-only devices, all ports, selected-source contexts,
partial/error/silent rows and cancellation evidence remain. Automatic Windows
PTR preserves OS DNS policy with bounded waits and at most 64 outstanding
native calls. No field-network speed multiplier has been measured.

Scan tables size intrinsically over all rendered rows and fit again on live
updates, without surplus stretching or sampled-row widths. Long hostname/vendor
values wrap. Copy controls reserve space, including last-column long answers.
TCP badges show exact ports with conventional service labels, not detected apps;
copying a port group returns comma-separated numbers.

Downloads expose received/total bytes, keep size/hash/PE checks and report
verification/staging separately. A bounded observer polls only explicit pending
downloads and ignores late replies. Unknown totals use activity bars. The native
installer keeps an activity window visible through file replacement and restart
until interface readiness/rollback. The first update from 82 uses its older
helper; this native progress window applies to updates launched from 83 onward.
Explicit install choice and all busy/settings/version/rollback guards remain.

Validation passes 475 JavaScript tests, 31 Python release guards and required
root/terminal/desktop plus full discovery/device Go suites. Five selected-source
transport tests blocked locally by netlink permissions pass on the hosted
runner. The DNS fixture now reserves TCP first and retries a matching UDP bind,
removing an independently allocated port collision without weakening assertions.
Focused discovery race tests cover bounds, early results, deadlines and retained
cancellation evidence. A separate race-enabled full production download-handler
fixture passes partial progress, responsive status, package verification, staged
bytes/hash and no installation. It uses isolated synthetic test data.

Detailed native Windows logs explicitly pass cmd and PowerShell output, resize,
Ctrl+C, shutdown, owned child-process cleanup, and native progress-window
visibility/lifecycle including hidden-start behavior. Earlier iterations had a
masked Windows command failure; that was corrected in code, workflow and the
user-facing status. Do not rely on the old green job or the obsolete D6B build.

The full browser run passes all new feature cases, six populated scan layouts
with 60 rows including late long values, long unbroken answers, exact port copy,
live updates, macro persistence and local custom themes. Existing 138 layouts
across 23 pages/two themes/three widths, 18 copies, six Hosts cases and the
17-case real-app loopback terminal selection test remain. Existing client/server,
SMTP, folders, dialog, command input and editor cases pass. All seven native
Chromium updater-timer cases pass, including the actual five-minute poll/focus
collision and the deliberately failing released negative control. Current app
reports have no runtime/asset failures in their covered cases.

Reviewed artifact 12623039560: SHA-256 3e29a1978562dd31af0a53fdc73ff710d906f9efe781cdd36ed20372420da9b0
Portable ZIP: edcfd279c94eaa3b18662748bf915f959052f65007300505ed9a77e8eea04023
EXE: dd50538a54ea21c2a563497525ad59fd1cc23c18134cce0cd34a34b5026d1698 (14260736 bytes)
All 713 packaged source files match the exact tested commit;
outer/inner ZIP CRC, both checksum inventories, x64 GUI PE/version and strict
four-member update ZIP verification pass. Local and hosted portable/EXE bytes
match. Browser artifact 12623776589 digest/CRC/reports pass:
ffff4e90628bbabf352182b5a06b6e94f67a1175d9b793a7db2abb2deb6bbbd0
New scan, long-copy and update progress screenshots were inspected.

Native full WebView2 interaction,
physical serial/network hardware and actual installation/restart were not tested.
The UI survey covers its listed states, not every possible application state.

This final publication record changes only AGENTS.md, this handoff and
VALIDATION.txt and uses [skip ci]. It must not move the 83 tag, replace assets
or trigger another publication. Continue from latest main and preserve all fixes.

## 1.17.82 published: preceding whitespace in terminal selections

Prepared from main c0a3c3ebe4558de38ad6c7e007803e6de3d47bdc after the user
reported selection deletion also fails with preceding spaces and explicitly
requested fix/publication on 2026-10-25. This authorizes this scoped release
only. Preserve all previous tags/assets and main network-toolkit as the sole
public source and updater feed.

The 81 fix clamped trailing padding but rejected a start before the tracked
input anchor. A native highlight can begin in the spacing after the prompt.
Normalize that start to the input anchor only if every selected pre-anchor cell
on the same logical line is visible whitespace/empty and has width one. Reject
prompt glyphs, wide-character continuations, hidden cells and previous hard
lines. Prompt spacing never adds movement/deletion; real spaces typed inside
the command remain editable. Partial prefix selections, both whitespace edges,
wrapped prompts/commands, typing and approved paste share the corrected helper.
Keep the existing trailing clamp, input range and remaining mode, revision,
ownership, queue, paste-review and size guards. No transport, credential,
settings storage or updater logic changes. The shared gear and all older UI,
editor, sidebar, folder/macro and SMTP fixes remain.

Seven focused input regressions cover both deletion keys/cursor movement,
partial prefixes, actual leading input spaces, wrapped prompts/commands, wide/
combining characters, typing/paste and protected selections. Exact released81
assets reproduce six failures (17 pass); candidate passes all 23 input tests.
Local and hosted validation passes 466 JavaScript tests, 31 Python release
guards, required root/terminal/desktop Go suites, release/resources, syntax,
Windows x64 GUI build and portable/update verification. All versions are 1.17.82.

Full review Checks 40252863225 passed on exact published source/tag
0ebd276e651eaf40bca4d80e317f68ff35b650aa (tree
3a8f507f74a3316fa861542a5e8d30a66a7c6560). Both jobs succeeded. The real app/browser/
loopback-Telnet regression passes all 17 cases. Eight added cases cover leading
whitespace with Backspace/Delete, native mouse drag, partial prefix deletion,
actual leading spaces, wrapped commands, typing and native clipboard paste.
Original trailing-space/cursor cases remain. Exact transmitted edits and echoed
text match; prompts/previous output remain, protected selections send no edits
and no command is submitted. Leading-selected and deleted-state screenshots
were inspected. These use local test sessions, not real hardware.

The UI survey passes 138 layouts across 23 pages in both themes at
1600/820/390px, 18 copies and six Hosts cases, with no Hosts writes or reported
clipping/overflow, browser errors or missing assets. Existing real DNS/TCP copy,
Hosts/client/server/SMTP/folders/terminal-dialog/command-input/editor/install
browser tests and all seven native Chromium updater-timer cases pass, including
the actual five-minute poll/focus collision.

Review artifact 12621178816 matches local portable/EXE bytes, and all
699 packaged source files equal the exact reviewed commit.
Outer SHA-256: a63bba3064c6f03686a51d0dbc5bf3714bfb21708f4c23b782e9572697844e63.
Browser artifact 12621269872 digest/CRC pass.
Outer SHA-256: 7ecaef648ae35a4f0128db181ace13dab0f1da2fb394f187c342a073e7fd50aa.

Main was fast-forwarded without force from the checked expected c0a3c3eb SHA to
the identical tested source, with no intervening source/doc changes. Release
40253776095 succeeded and created stable release 423774404.
Completed review is runtime validation; duplicate automatic main Checks may
continue independently on that same source.

All four actual public downloads match GitHub asset digests. Outer/inner
manifests, both ZIP CRCs, strict four-member update allowlist, Windows x64 GUI
PE/version and standalone/portable/update EXE identity pass. All
699 packaged source files match the published commit; official portable and
EXE equal the reviewed build. All 21 earlier release IDs, asset
IDs/sizes/digests and 21 older source-tag SHAs remain unchanged.

Unchanged released 1.17.81 production feed handlers checked the live feed,
verified/downloaded/staged 1.17.82 in 11.97 seconds. Staged bytes match the
independently downloaded official EXE. The harness uses platform-independent
handlers, real HTTPS and isolated temporary settings. No installation/restart
occurred and the original test executable hash remained unchanged.

Official release: https://github.com/TechSavage1989/network-toolkit/releases/tag/v1.17.82
EXE SHA-256: 8148791b34539f2d2edd88333b4197aef4f6bee893e831ca8f1b17ea2ab55159
Portable ZIP SHA-256: 598bb5c5a7d92a7194407e3cff98f3f869b9daffd975de68cb447f0ee28df523
Update ZIP SHA-256: ad40ea9e9a0e708d369dfd8e6b4cefc2151e747a116a8e6c6a9abe3b57d62a62
SHA256SUMS file SHA-256: 43836eaaea6c809a71f7c21b988b703fb28532e497f3bb99d48e0994dbc04914

This final record changes only AGENTS.md, this handoff and VALIDATION.txt and
uses [skip ci]. Do not move the 82 source tag, replace assets or republish it.
Continue from latest main. Native Windows/WebView2, real hardware and actual
installation/restart remain unverified. The UI survey covers its listed scenarios,
not every possible application state. Do not claim unperformed checks.

## 1.17.81 published: settings gear and terminal selection padding

Prepared from main d311c9a6def119c20de6345ecee6b0139151190d, retaining its
README/navigation refresh and all 1.17.80 changes. The user requested a
recognizable settings icon throughout the app, terminal deletion with trailing
empty selection cells fixed, and publication on 2026-10-25. This authorizes this
scoped release only. Preserve previous tags/assets and main network-toolkit as
the sole public source and updater feed.

The shared settings SVG is now a symmetric eight-tooth gear with a circular
center. Existing sizes, strokes, labels and handlers remain across App settings,
terminal tools/context settings, client/server connection controls and other
configuration/detail tools. No icon dependency or separate asset is introduced.

terminal-input.js rejected a selection whose end was outside known command
character positions. Clamp the selected end to the editable command end when
the overrun is only empty cells on that same logical line. The exclusive
next-row-zero boundary is allowed, including wrapped commands; later hard
lines remain excluded. The start-position, prompt/output, revision, mode,
hidden-input, queue, ownership, paste-review and size checks remain. Keep the
existing cursor/text range detection: spaces before the cursor are included,
while empty/erased cells do not add movement or deletion. Backspace/Delete,
typing and approved paste share the fixed helper. No transport, credential,
settings storage or updater logic changes.

Six focused regressions cover padded Delete/Backspace with cursor movement,
wrapped boundaries, real trailing spaces, explicit-space device erase echoes,
typing/paste and protected selections. Exact released80 assets reproduce five
failures (11 pass); all 16 terminal-input tests pass with the fix. Full local
and hosted validation passes 459 JavaScript tests, 31 Python release guards,
required root/terminal/desktop Go suites, metadata/resources, JavaScript syntax,
Windows x64 GUI build and portable/update verification. All versions are 1.17.81.

Early live-test reviews exposed fixture races, not additional runtime changes:
the console's initial fit cleared a synthetic selection; an overly strict wait
then depended on asynchronous resize metadata; the peer's full-history redraw
and forced cursor move at an exact column boundary invalidated its command
anchor. The fixture now waits for stable rendered dimensions, redraws only the
editable line and preserves delayed wrap. A fragmented real-xterm echo check
also verified its anchor and exact deletion count before the final hosted run.

Full review Checks 40250670925 passed on exact published source/tag
4d948b7124d9af71b8251390a6e378df8e9fabf5 (tree
b4d466590f805c3dcd9f7a0a56a2f87b35d8cb7b). Both jobs succeeded. The new real-app browser
regression passes nine live cases through unchanged app assets/APIs and a
loopback Telnet peer: Backspace, Delete, native mouse drag, cursor movement,
wrapped selection, trailing spaces, typing, native clipboard paste and protected
selection guards. Echoed text and exact transmitted edits match; previous output
remains and no command is submitted. These are local test sessions, not real
hardware or native Windows execution.

The application survey passes 138 layouts across 23 pages at 1600/820/390px in
both themes, 18 clipboard copies and six Hosts menu cases, with no Hosts writes,
reported clipping/overflow, browser error or missing asset. Gear/settings and
terminal selection/result screenshots were inspected in both themes. Existing
real DNS/TCP copy, Hosts/client/server/SMTP/folders/terminal-dialog/command-input/
editor/install browser tests and all seven native Chromium updater-timer cases
pass, including the real five-minute poll/focus collision.

Review artifact 12620496048 matches the local portable and EXE
byte-for-byte, with all 698 source files equal to the exact reviewed commit.
Outer SHA-256: 98da70d4e348574b497b19173c06d3c00a003c6061fd1028acd993d3db1ec518.
Browser artifact 12620721891 digest and ZIP CRC pass.
Outer SHA-256: effd8b04acaa41359b755da6dc6f9a19b1a39660e64f2fe6bd2e4fe1572c19de.

Main was fast-forwarded without force from the checked expected SHA d311c9a6 to
the identical tested source, with no intervening source/doc changes. Release
40251443716 succeeded and created stable release 423761885.
Full review above is runtime validation; duplicate automatic main Checks may
continue independently and do not change the reviewed source.

All four actual public downloads match GitHub asset digests. Outer/inner
manifests, both ZIP CRCs, strict four-member update allowlist, Windows x64 GUI
PE/version and standalone/portable/update EXE identity pass. All
698 packaged source files match the published commit; official portable
and EXE match the reviewed build. All 20 earlier release IDs, asset
IDs/sizes/digests and 20 older source-tag SHAs remain unchanged.

The unchanged released 1.17.80 production feed handlers checked the live feed,
verified/downloaded/staged 1.17.81 in 11.65 seconds. Staged bytes match the
independently downloaded official EXE. The harness uses platform-independent
handlers, real HTTPS and isolated temporary settings. No installation or restart
occurred, and the original test executable hash remained unchanged.

Official release: https://github.com/TechSavage1989/network-toolkit/releases/tag/v1.17.81
EXE SHA-256: 738eb863d3f165fcbe89ae39a34e79981f13e7e2d002100bc59c5c56041d080a
Portable ZIP SHA-256: 71a2fc9392467eb3eddf7c1339bd0ddfcd6a967a0fc52c792a153b309a8b6c85
Update ZIP SHA-256: 9875a67623f9efc0f5c777747f0580e7cf87d4be96a5f37bd45853098bd347cf
SHA256SUMS file SHA-256: d7e3e2555acce42192ed0cb606e5df76a9402596fcbe935984c387fb0e3cfd32

This final record changes only AGENTS.md, this handoff and VALIDATION.txt and
uses [skip ci]. Do not move the 81 source tag, replace assets or republish it.
Continue future work from latest main. Preserve all previous Hosts/Overview/copy,
terminal/sidebar/detach/Connect, editor, folders/macros/credentials, command-input,
SMTP and updater fixes. Native Windows/WebView2, real hardware and actual
installation/restart remain unverified. The UI survey covers the listed scenarios,
not every possible application state.

## README navigation refresh (2026-10-25)

Documentation-only: README.md now introduces the app, links verified release
assets and existing guides, and explains setup, updates and safety boundaries.
No application code, version, workflow, release asset or tag changed. The commit
uses [skip ci] to avoid triggering builds or release automation for this prose
change. Relative links were checked against the repository tree and download
names against the current stable release. No new runtime validation is claimed;
the published-version validation and its limits below remain unchanged.

## 1.17.80 published: consistent copy actions and UI layout

Prepared from main 8769ab4ae7cb311a2d54d97fabb15d29a357b22d after the user
provided Hosts/Overview screenshots and requested aligned controls, removal of
redundant Overview tools, consistent useful copy actions and an application UI
review, explicitly authorizing publication on 2026-10-25. This authorizes this
scoped new release only. Preserve older tags/assets and main network-toolkit
as the sole public source/update feed.

Hosts More inherited generic details/summary spacing and borders. Scoped CSS
resets the outer disclosure and aligns its summary with toolbar buttons. Native
click/Enter behavior, viewport-bounded menu positioning and focus return remain;
Escape now closes the menu and focuses More.

Overview removes the repeated diagnostic shortcut tile grid and per-IP Tools
buttons. Main navigation, adapter address context menus/keyboard access, network
scan actions and View details remain. Each address and adapter name has an explicit copy payload. Hosts paths and
expanded DNS metadata fields also have direct value copy actions. Hosts keeps
its editor opt-out and uses an explicit native path copy action.

The hover-only legacy ntkOfferCopy helper wrapped value DOM and created inline
buttons. It overlapped the newer result-copy helper, skipped addresses containing
Tools actions and inherited generic minimum button sizing. Remove the old helper
and its styles; NetworkToolkitValueCopy now also covers Overview, adapter and
reusable diagnostic/detail values. Controls reserve a fixed slot alongside the
first text line and stay visible. Text DOM, selection, current-value copying,
DNS whitespace, keyboard use, result reconciliation and context actions remain.
Nested value groups get no extra aggregate copy control. Existing public-IP
copy buttons remain sole owners. Inputs, terminals, secrets, statuses, action
cells and placeholders remain excluded. Shared toolbar SVGs retain their shape
when labels wrap. Compact terminal recipient labels now wrap instead of clipping with an ellipsis,
while retaining the command input scrollbar suppression. No transport,
credentials or updater logic changes.

Review found and corrected a CRLF export-fixture expectation and a clipped
View recipients label at 390px. Icon fixture requests now use the same shared
asset location as the app. Those issues were resolved before the successful hosted review below.

Three focused JavaScript regressions cover new copy availability, individual
addresses, retained nodes, deduplication, native controls and exclusions.
The coverage test fails against released79 and all 13 copy-helper tests pass
with the fix. A new isolated full-page browser review surveys navigation pages
at 1600/820/390px in both themes, checks visible labels/icons, viewport overflow,
copy placement and actual clipboard contents, and exercises Hosts menu keyboard
use/export. The existing real app DNS/TCP clipboard regression is now run in CI
against a freshly built Linux headless app. Browser tests use isolated/local
fixtures, not external devices or Windows system files.

Final visual review caught identical clipboard success notifications piling up
during rapid copies. Identical message/severity pairs now reuse one existing
notification and reset its original expiry; distinct messages remain separate.
Two focused timer/identity regressions cover this behavior, and the real-page
clipboard check asserts one visible copy notification.

Go/UI/package/Windows resources are consistently 1.17.80. Local required
validation passed 453 JavaScript tests, 31 Python release guards, required
root/terminal/desktop Go suites, release/resource metadata, inline/browser
syntax, Windows x64 GUI build and portable/update package verification.

Full review Checks 40219794786 passed on exact published source/tag
e7b6e1855789402f22ca56b37f26836a3155fc05 (tree
cd4ae18f3f9dc7a6665b8f3a775d45c43b9ce3f3). Both jobs succeeded. The new full-page
review passes all 138 layouts across 23 navigation pages, at 1600/820/390px in
both themes, with no reported button text/icon clipping, viewport overflow,
copy-control containment failure, browser error or missing asset. It verifies
18 actual clipboard copies, six Hosts menu keyboard/alignment cases, Escape
focus return and CRLF export without any Hosts write. Representative Overview,
Hosts and terminal screenshots were inspected in both themes/sizes.

The newly enabled real headless-app regression also passes actual local DNS
and TCP traffic, browser clipboard contents, meaningful TXT whitespace,
native selection, context menus, keyboard entry detail selection, subnet
values, retained live monitor control identity/focus and imported PCAP actions.
It confirms inputs, terminal content and secrets retain their exclusions.
An outdated optional expectation for copy controls on DNS record-type badges
was corrected to the already established helper/unit-test contract; no app
behavior was changed for that expectation.

Existing Hosts/client/server/SMTP/folders/terminal-dialog/command-input/editor/
install browser checks and all seven native Chromium updater-timer scenarios
pass, including the real five-minute poll/focus collision. Review artifact
12609232136 matches the local portable and EXE byte-for-byte. Outer SHA-256:
820b9391b71f936147d0515efae8353f46df641957b3f437e10e84ec5e196d03.
Browser artifact 12609522485 digest and ZIP CRC pass; outer SHA-256:
b6c3aab4fd300f57931717f91f6c7b09af702dfc546159804c66d180ec80eb53.

Main was fast-forwarded to the identical tested source using expected old SHA
8769ab4ae7cb311a2d54d97fabb15d29a357b22d without force. No code or docs changed
between full review and promotion. Release 40220639811 succeeded and created
stable release 423578162. Completed review remains full runtime validation;
this documentation may supersede automatic duplicate main runs.

All four actual public downloads match GitHub asset digests. Outer/inner SHA-256
manifests, both ZIP CRCs, strict four-member updater allowlist, Windows x64 GUI
PE/version and standalone/portable/update EXE identity pass. All 696
packaged source files match the exact published commit. Official portable and
EXE match the reviewed build byte-for-byte. All nineteen earlier release IDs
and asset IDs/sizes/digests remain unchanged, with all nineteen older source-tag
SHAs preserved. Verification snapshots and logs are retained with local QA.

The unchanged released 1.17.79 production feed handlers checked the live feed,
verified/downloaded/staged 1.17.80 in 11.23 seconds. Staged bytes match the
independently downloaded official EXE. The harness uses real HTTPS, isolated
temporary settings and only platform-independent feed handlers. No installation
or restart occurred; its original test executable hash remained unchanged.

Official release: https://github.com/TechSavage1989/network-toolkit/releases/tag/v1.17.80
EXE SHA-256: 3d165ebc258de65c8f5caa447303f2c272ea41f8bbc22ec2db3be7f3861ea733
Portable ZIP SHA-256: 2d459c02e98fe41d977bbcb2ac23c5ea1fe8c4d14d2784a8482c40b9921f61e2
Update ZIP SHA-256: 40e7e71db7baf35a7f64e6516ed45690ad237037f7ca61f3a4c03d240d40ad6b
SHA256SUMS file SHA-256: b71500b467e9c1f041791cd583708362efeec54ff6b9445aaf52bd3887729ab7

The final publication record changes only AGENTS.md, this handoff and
VALIDATION.txt. Do not move the released 80 tag, replace assets or republish.
Continue from latest main. Native Windows/WebView2, real hardware and actual
installation/restart remain unverified. Browser fixtures, Linux headless tests
and live-feed staging are not native Windows execution proof. This review covers
the listed layouts and scenarios, not every possible application state.

## 1.17.79 published: remove the overview target divider

Prepared from main 7ef55a25f00429324d02e556d3fe39112328e36e after the user
provided an overview screenshot and requested removing the line below Enter to
run, explicitly authorizing build/publication on 2026-10-25. This authorizes
this scoped new release only. Preserve prior tags/assets and use main
network-toolkit as the sole public source/update feed.

The line is the top border of the diagnostic source control, immediately below
the target entry row. A scoped source-interface.css rule removes that border
only inside the Overview workspace. Target entry, the Enter to run hint,
source-interface picker, padding and run controls retain their existing behavior.
Other source-control contexts keep their styling. No application JavaScript
handlers, transport, ownership, credentials or updater code changes.

Go/UI/package/Windows resources are consistently 1.17.79. No new cosmetic tests
were added for this one-rule change. The existing server browser script only
adds four Overview target-form screenshot captures, at 1600/570px in both themes,
for manual visual review before its unchanged existing layout checks.
All four were inspected: the divider is absent; the target field, save/run
buttons, source selector and hint remain contained and readable. The existing
desktop Enter to run hint remains; compact layouts keep their existing hiding
behavior. These isolated browser UI/API fixtures create no real transports.

The first review (40178869013) forced online state for its screenshot capture,
which triggered live polling in the isolated server fixture and disrupted a
later layout check. The capture now retains the existing offline fixture state.
No application polling or layout logic changed. Final hosted review below
passes with that capture setup corrected.

Local validation passed 448 JavaScript tests, 31 Python release guards, required
root/terminal/desktop Go suites, release/resource metadata, inline HTML/browser
script syntax, Windows x64 GUI build and portable/update package verification.

Full review Checks 40179023641 passed on exact published source/tag
fccf4f93a9405878a3772fe24c0ab35e21c5b5c6 (tree
36616f132d1542e2eac2bf72f4c0463d1f6cade5). Both jobs succeeded. Existing
Hosts/client/server/SMTP/folders/terminal/command-input/editor/install browser
checks pass, together with all seven native Chromium updater-timer scenarios
including the real five-minute poll/focus collision. No cosmetic unit tests
were added. Review artifact 12593475795 matches the local portable and EXE
byte-for-byte. Outer SHA-256:
2ea10de95d54583a2e5ec7560970e70b5726df23dab0b8a3a468369f9c0bd372.
Browser artifact 12594205818 digest and ZIP CRC pass; outer SHA-256:
217519d29203fb8afcb7277556dc444094374ecd66ca72d6841d657bf3426283.

Main was fast-forwarded to that identical tested commit using expected old SHA
7ef55a25f00429324d02e556d3fe39112328e36e without force. No code or docs changed
between full review and promotion. Release 40179754298 succeeded and created
stable release 423280580. Completed review above remains full runtime
validation; this documentation may supersede automatic duplicate main runs.

All four actual public downloads match GitHub asset digests. Outer/inner SHA-256
manifests, both ZIP CRCs, strict four-member updater allowlist, Windows x64 GUI
PE/version and standalone/portable/update EXE identity pass. All 693
packaged source files match the exact published commit. Official portable and
EXE match the reviewed build byte-for-byte. All eighteen earlier release IDs
and asset IDs/sizes/digests remain unchanged, with all eighteen older source-tag
SHAs preserved. Verification snapshots and raw logs are retained with local QA.

The unchanged released 1.17.78 production feed handlers checked the live feed,
verified/downloaded/staged 1.17.79 in 9.49 seconds. Staged bytes match the
independently downloaded official EXE. The harness used real HTTPS, isolated
temporary settings and only platform-independent feed handlers. No installation
or restart occurred and its original test executable hash remained unchanged.

Official release: https://github.com/TechSavage1989/network-toolkit/releases/tag/v1.17.79
EXE SHA-256: 7c90288a829c443c2cf08d9004759127dde6f224619b3f1ca2c249d419632874
Portable ZIP SHA-256: 517490729efcba4b1bbdf0a3b55500a09cf8e3f6736bfc6edaa38ba5a7ac5094
Update ZIP SHA-256: e71f8efb5856d3440dfaf420fb979e589fbe30a217aafc296df70b89e8a4fffb
SHA256SUMS file SHA-256: 0ceec207192c8c2ebf4df917dbd51e4b435a028b48deee48555175adc9821783

This final publication record changes only AGENTS.md, this handoff and
VALIDATION.txt. Do not move the released 79 source tag, replace assets or
republish it. Continue future work from latest main. Native Windows/WebView2,
hardware and actual installation/restart remain unverified here; browser
fixtures and live-feed staging are not native Windows execution proof.

## 1.17.78 published: detached terminal sidebar side

Prepared from main 7e54d7ee0b3a42af9f384453e0742bf9458d9e29 after the user
reported detached terminals forgetting which side the sidebar occupied in the
main window and explicitly requested build/publication on 2026-10-25. This
authorizes this scoped new release only. Preserve prior tags/assets and use
main network-toolkit as the sole source/update feed.

Both the single-session display filter and linked-workspace display filter
omitted terminalSidebarSide although the sidebar already persisted/emitted it.
Both filters now include the field. The shared settings sanitizer retains only
left/right, using the existing right-side default for invalid values. A pane
moved out of a linked workspace inherits that workspace's current side. Detached
display saves and workspace checkpoints/return snapshots retain the choice.

Before a single-session/pane detach, await the existing queued settings flush
so an immediate detach after swapping sides observes the current selection
rather than the 550ms delayed autosave. A failed save leaves the session in its
original window and reports the error. Whole-workspace capture already includes
current sidebar preferences directly. No transport, ownership or updater logic
changes. Existing connection/modal, credentials/folders/macros and all prior
UI fixes remain. Go/UI/package/Windows resources are consistently 1.17.78.

Focused Go endpoint/round-trip regressions reproduce the omission in released77
and pass with the fix, covering single views, linked workspaces, nested pane
moves, detached preference edits, reattach and safe left/right validation.
Two JavaScript regressions reproduce the immediate-detach race and failed-save
behavior in released77 and pass with the fix. Local validation passed all
448 JavaScript tests, 31 Python release guards, required root/terminal/desktop
Go suites, release/resource metadata, inline HTML/browser script syntax,
Windows x64 GUI build and portable/update package verification.

Full review Checks 40177180488 passed on exact published source/tag
7477c0ca7a5f076576754d91c68c06971a35c195 (tree
15b0d80ee1b4651ab5a3585254134f7ba07eaf31). Both jobs succeeded. The extended
real-page terminal browser test retains all 36 modal layout cases and checks
both sidebar positions, workspace capture/apply, actual rendered placement and
single-session detach immediately after swapping sides in the same event turn.
Both detaches reach the fixture endpoint with the latest saved side. Inspected
left/right screenshots show the sidebar beside the retained terminal area.
These use isolated UI/API fixtures; native detached Windows windows are not
launched and no actual external transport is created by the browser test.

Existing Hosts/client/server/SMTP/folders/command-input/editor/install browser
checks pass, together with seven native Chromium updater-timer scenarios
including the real five-minute poll/focus collision. Review artifact 12592197603
matches the local portable and EXE byte-for-byte. Outer SHA-256:
e733633c3f7943226d834c31b05fc8f06132eb956e184705db910e3844759666.
Browser artifact 12592412894 digest and ZIP CRC pass; outer SHA-256:
25a28be13b2b56934cbeba008b0c24855d889faf0cb7f72c50f09ce2d4646e7f.

Main was fast-forwarded to that identical tested commit using expected old SHA
7e54d7ee0b3a42af9f384453e0742bf9458d9e29 without force. No code or docs changed
between full review and promotion. Release 40177805807 succeeded and created
stable release 423263218. Completed review above remains full runtime
validation; this documentation may supersede automatic duplicate main runs.

All four actual public downloads match GitHub asset digests. Outer/inner SHA-256
manifests, both ZIP CRCs, strict four-member updater allowlist, Windows x64 GUI
PE/version and standalone/portable/update EXE identity pass. All 692
packaged source files match the exact published commit. Official portable and
EXE match the reviewed build byte-for-byte. All seventeen earlier release IDs
and asset IDs/sizes/digests remain unchanged, with all seventeen older source-tag
SHAs preserved. Verification snapshots and raw logs are retained with local QA.

The unchanged released 1.17.77 production feed handlers checked the live feed,
verified/downloaded/staged 1.17.78 in 9.65 seconds. Staged bytes match the
independently downloaded official EXE. The harness used real HTTPS, isolated
temporary settings and only platform-independent feed handlers. No installation
or restart occurred and its original test executable hash remained unchanged.

Official release: https://github.com/TechSavage1989/network-toolkit/releases/tag/v1.17.78
EXE SHA-256: 0769801672739056876f7aab426a425e22b4ee1bbb1c9c39ebb4c333f1b7ee7f
Portable ZIP SHA-256: 7e0d1271cd63d50b55dac295573c84305ff5876bce1d74fe6aa8bae2e7427016
Update ZIP SHA-256: 0a0a5e1fbef6a8afb782079bb0c36b41b39b31c0bffa52c7e21c24395c9971b7
SHA256SUMS file SHA-256: 13da9f3ba7a4c444bd303e0261ee04105408fd9ba82fd6bfe85e78eab81fe1c6

This final publication record changes only AGENTS.md, this handoff and
VALIDATION.txt. Do not move the released 78 source tag, replace assets or
republish it. Continue future work from latest main. Native Windows/WebView2,
hardware and actual installation/restart remain unverified here; browser
fixtures and live-feed staging are not native Windows execution proof.

## 1.17.77 published: Connect labels in client and server sidebars

Prepared from main 6b32d2fadd9fd2f076309a3a379784a6b2e93590 after the user
requested Setup/settings tabs in client/server mode be called Connect and
explicitly instructed publication on 2026-10-25. This authorizes this scoped
new build/release only. Preserve all prior tags/assets. Main network-toolkit
remains the sole source/update destination.

The shared client rail label changes Setup to Connect; the shared server rail
label changes Settings to Connect. This covers SFTP, TFTP and FTP/FTPS in both
modes. Internal settings tab IDs, data attributes and stored preference keys
remain unchanged, so saved selections continue working. Panel headings and
existing connection/transfer controls are retained. No handlers, server behavior,
credential storage or updater logic changes. Keep the 76 shared tab styles and
all earlier editor/terminal/folder/macro/SMTP functionality.

Go/UI/package/Windows resources are consistently 1.17.77. No cosmetic unit tests
were added for this small label change. Local validation passed 446 JavaScript
tests, 31 Python release guards, required root/terminal/desktop Go suites,
release/resource metadata, HTML/browser syntax, Windows x64 GUI build and
portable/update package verification.

Full review Checks 40175759939 passed on exact published source/tag
690084bb8910ecbffb4c792e117ec609e0fd8af1 (tree
bee333bf1705795fd42294bcc062896ff8e6be3d). Both jobs succeeded. The server
browser suite passes 90 existing layout cases plus 18 selected-tab cases across
SFTP/TFTP/FTP, both themes and 570/820/1366px. All 54 normal/hover/keyboard-focus
computed-style and rail-relative geometry comparisons match client mode.
Connect labels stay on one line inside their buttons. Click/Enter/Space
collapse/expand, saved state, adapter selection, pointer/keyboard resizing,
persistence and viewport recovery pass. Inspected server/client selected-tab
screenshots both show Connect with matching highlight, outline, spacing and
height. These use isolated UI/API fixtures; no real server starts.

Existing Hosts/client/SMTP/folders/terminal-dialog/command-input/editor/install
browser checks pass, together with seven native Chromium updater-timer scenarios
including the real five-minute poll/focus collision. Review artifact 12592580148
matches the local portable and EXE byte-for-byte. Outer SHA-256:
5558f149e5d5768986b0f1c32ad2ef3769ebe207f8cce9714e52fa0054147e43.
Browser artifact 12593015019 digest and ZIP CRC pass; outer SHA-256:
3db5c9e08f959ac5676cbd8c766abb2ae8fc6af99154e824888c2d8c96b88b3b.

Main was fast-forwarded to that identical tested commit using expected old SHA
6b32d2fadd9fd2f076309a3a379784a6b2e93590 without force. No code or docs changed
between full review and promotion. Release 40176334534 succeeded and created
stable release 423249789. Main duplicate Checks 40176334533 passed its check
job and server-style browser step. Completed review above remains full runtime
validation; this documentation may supersede automatic duplicate runs.

All four actual public downloads match GitHub asset digests. Outer/inner SHA-256
manifests, both ZIP CRCs, strict four-member updater allowlist, Windows x64 GUI
PE/version and standalone/portable/update EXE identity pass. All 691 packaged
source files match the exact published commit. Official portable and EXE match
the reviewed build byte-for-byte. All sixteen earlier release IDs and asset
IDs/sizes/digests remain unchanged, with all sixteen older source-tag SHAs
preserved. Verification snapshots and raw logs are retained with local QA.

The unchanged released 1.17.76 production feed handlers checked the live feed,
verified/downloaded/staged 1.17.77 in 11.10 seconds. Staged bytes match the
independently downloaded official EXE. The harness used real HTTPS, isolated
temporary settings and only platform-independent feed handlers. No installation
or restart occurred and its original test executable hash remained unchanged.

Official release: https://github.com/TechSavage1989/network-toolkit/releases/tag/v1.17.77
EXE SHA-256: 8aab091415e580940c2e4a209ad99f1a611b12f174a1758214a271c7d1f7ac3c
Portable ZIP SHA-256: 24fe857dbe765f778247c3070193342653dc49418b6115f7c3281d1d0f2f9012
Update ZIP SHA-256: e8faa994885796f9f995d696beac843cc9ba742c8388e0701e4d440e18b16bc8
SHA256SUMS file SHA-256: 4f57940bfcda7f3a83ba93a52d0070c4dc1228e78b0ae0d752ad12888b7caa25

This final publication record changes only AGENTS.md, this handoff and
VALIDATION.txt. Do not move the released 77 source tag, replace assets or
republish it. Continue future work from latest main. Native Windows/WebView2,
hardware and actual installation/restart remain unverified here; browser
fixtures and live-feed staging are not native Windows execution proof.

## 1.17.76 published: match selected server and client sidebar tabs

Prepared from main 11c52273cb62f066ff563352d667b8eeb2346c89 after the user
provided server/client screenshots and requested matching server selected-tab
appearance, explicitly authorizing build/publication on 2026-10-25. This covers
this scoped release only; preserve prior tags/assets. Main network-toolkit
remains the sole source/update destination.

Server Settings previously combined a generic panel-visibility-toggle class
with separate rail/button rules, introducing a rounded outline and different
spacing/focus styling. Server rail/button markup now uses terminal-sidebar-rail
and terminal-sidebar-tab, the existing client components. Remove the duplicate
server rail/button CSS and its span font/letter-spacing override. Shared selected
styling accepts aria-expanded=true for the server disclosure control as well as
aria-selected=true for client/terminal tabs. Short tab labels stay on one line.
Native disclosure semantics, labels, toggle handlers, server settings, resizing,
transfers and updater logic are retained. Existing client short labels retain
their appearance. Go/UI/package/Windows resources are consistently 1.17.76.

The 75 editor initial-view fix, restored terminal connection modal, saved folders/
credentials/macros, command-input scrollbar suppression and SMTP lifetime remain.
No cosmetic unit tests were added. Local validation passed all 446 JavaScript
tests, 31 Python release guards, required root/terminal/desktop Go suites,
release/resource metadata, HTML/browser syntax, Windows x64 GUI build and
portable/update package verification.

The first rendered comparison caught the longer Settings label wrapping to two
lines. Shared short labels now stay on one line with containment checks. The
next run passed all style comparisons but its following existing activity-resize
test inherited a compact viewport. That test now explicitly restores its
original 1600x1100 viewport. No application resize behavior changed.

Full review Checks 40174421236 passed on exact published source/tag
a441493cecd7c92a0951bbb8ae1531348654ef11 (tree
7dbb47d4918ccb2d69c2fbd6716826031543d2e1). Both jobs succeeded. The extended
real-page server browser suite passes 90 existing server layout cases plus
18 selected-tab cases across SFTP/TFTP/FTP, both themes and 570/820/1366px.
All 54 normal/hover/keyboard-focus comparisons match actual computed client
styles and rail-relative button geometry, including background, border, radius,
padding, typography, icon size and focus outline. Labels stay on one line inside
the button. Click/Enter/Space collapse/expand, saved state, adapter selection,
pointer/keyboard resizing, persistence and viewport recovery pass. Inspected
server/client selected-tab screenshots have matching square highlight/outline,
spacing and height. These are isolated UI/API fixtures; no real server starts.

Existing Hosts/client/SMTP/folders/terminal-dialog/command-input/editor/install
browser checks also pass, together with seven native Chromium updater-timer
scenarios including the real five-minute poll/focus collision. Review artifact
12591657018 matches the local portable and EXE byte-for-byte. Outer SHA-256:
9257dfb25e229644a7e7e818e3a0472ea586eeaa8a802983c8bbbb63d787678c.
Browser artifact 12591897343 digest and ZIP CRC pass; outer SHA-256:
67305f0f3076185a97064c389572a0151c64bdab92dd5c4791c7a38f121d7282.

Main was fast-forwarded to that identical tested commit using expected old SHA
11c52273cb62f066ff563352d667b8eeb2346c89 without force. No code or docs changed
between full review and promotion. Release 40175002006 succeeded and created
stable release 423236946. Main duplicate Checks 40175001980 passed its check
job and server-style browser step. Completed review above remains full runtime
validation; this documentation may supersede automatic duplicate runs.

All four actual public downloads match GitHub asset digests. Outer/inner SHA-256
manifests, both ZIP CRCs, strict four-member updater allowlist, Windows x64 GUI
PE/version and standalone/portable/update EXE identity pass. All 690 packaged
source files match the exact published commit. Official portable and EXE match
the reviewed build byte-for-byte. All fifteen earlier release IDs and asset
IDs/sizes/digests remain unchanged, with all fifteen older source-tag SHAs
preserved. Verification snapshots and raw logs are retained with local QA.

The unchanged released 1.17.75 production feed handlers checked the live feed,
verified/downloaded/staged 1.17.76 in 8.23 seconds. Staged bytes match the
independently downloaded official EXE. The harness used real HTTPS, isolated
temporary settings and only platform-independent feed handlers. No installation
or restart occurred and its original test executable hash remained unchanged.

Official release: https://github.com/TechSavage1989/network-toolkit/releases/tag/v1.17.76
EXE SHA-256: 91151251b449a8212a13a9c683ffed47267b7ee4f385fcd7d811f0612719b00d
Portable ZIP SHA-256: f7ea56460a9104c253de022da06dab0081e12901cdff29ee59ba443e9f1e424f
Update ZIP SHA-256: b97e45af24abea48a3d4e9907dad909710e6fbf485f81eedcb1f7fb9d4d57879
SHA256SUMS file SHA-256: 9b729ca1d248a2c9ecaeded20c1ad80190ae6cdd8f247dcb8e2baf7e20c5b34b

This final publication record changes only AGENTS.md, this handoff and
VALIDATION.txt. Do not move the released 76 source tag, replace assets or
republish it. Continue future work from latest main. Native Windows/WebView2,
hardware and actual installation/restart remain unverified here; browser
fixtures and live-feed staging are not native Windows execution proof.

## 1.17.75 published: text files open at the document start

Prepared from main 59cd8fb4348be28fa2efa91570de6c221f5162e6 after the user
reported that the text editor loads files scrolled to the bottom and explicitly
requested fix, build and publication on 2026-10-25. This authorized this scoped
new release only; preserve all prior releases/tags/assets. Main network-toolkit
remains the sole source/update destination.

The shared file editor previously assigned loaded text and focused its textarea
without resetting selection/scrolling. A one-time per-document showStart flag
is now set when loading file values or explicitly reopening an existing file.
The next display places the caret at offset zero, focuses the text field and
sets scrollTop/scrollLeft to zero before synchronizing highlighting, line numbers
and position status. Draw avoids assigning an unchanged textarea value. This
keeps the initial view stable while ordinary editing, Find and navigation remain
usable. Local files, desktop imports and local/remote reader callbacks share
this path. No file bytes, encoding/line endings, save/transport handlers or
updater code changes. Go/UI/package/Windows resources are consistently 1.17.75.

Reload completion redraws only a document that is still present and active.
Inactive loads retain the pending initial view until their tab is shown, so a
background completion cannot move another document's view or steal its focus.
Saved folders/credentials/macros, restored terminal connection modal, command-
input scrollbar suppression, SMTP lifetime and the update flow are retained.

Local validation passed 446 JavaScript tests, 31 Python release guards,
required root/terminal/desktop Go suites, release/resource metadata, HTML/browser
syntax, Windows x64 GUI build and portable/update package verification. Two
focused regressions fail against exact released74 editor source and pass with
the fix. Initial browser validation caught a synthetic HTTP origin lacking
Web Crypto and a caret reset when a background-loaded tab was first displayed.
The fixture now uses HTTPS; the final focus/unchanged-value handling addresses
the background-tab behavior. Both were resolved before publication.

Full review Checks 40172396873 passed on exact published source/tag
5f41a1176ba2212c49ad1274a4833125c3f130f8 (tree
86b8b1c52b4be670c95d91ccc7658a03f3320b08). Both jobs succeeded. The new
real-page browser regression passed twelve cases at 390/768/1280px, in both
themes and embedded/overlay views. The first line is visibly on screen, with
caret offset zero, both scroll axes zero and aligned highlighting/line numbers.
Loading/reloading/reopening, local and connected-SSH readers, desktop UTF-8 BOM/
CRLF import, background reads/reloads, Find/typing and large/wrapped documents
pass without file writes. Inspected wide/compact screenshots show line one
and Ln 1, Col 1. These use isolated API fixtures and synthetic File objects;
they do not prove native Windows file-dialog or real-server behavior.

Existing Hosts/client/server/SMTP/folders/terminal-dialog/command-input/install
browser checks also pass, along with seven native Chromium updater-timer
scenarios including the real five-minute poll/focus collision. Review artifact
12590927264 matches the local portable and EXE byte-for-byte. Outer SHA-256:
6deb30281acc55752074dceb1bf2a4109d4a65bdfd60b7c0ce241bbaf8c357fd.
Browser artifact 12590753464 digest and ZIP CRC pass; outer SHA-256:
1984181d10b105aa6d015b25b4f0aa0ec3db733e2ee8968b11181f3022cefda2.

Main was fast-forwarded to that identical tested commit using expected old SHA
59cd8fb4348be28fa2efa91570de6c221f5162e6 without force. No code or docs changed
between full review and promotion. Release 40172945610 succeeded and created
stable release 423217913. Main duplicate Checks 40172945578 passed its check
job and editor browser step. Completed review above remains full runtime
validation; this documentation may supersede automatic duplicate runs.

All four actual public downloads match GitHub asset digests. Outer/inner SHA-256
manifests, both ZIP CRCs, strict four-member updater allowlist, Windows x64 GUI
PE/version and standalone/portable/update EXE identity pass. All 689 packaged
source files match the exact published commit. Official portable and EXE match
the reviewed build byte-for-byte. All fourteen earlier release IDs and asset
IDs/sizes/digests remain unchanged, with all fourteen older source-tag SHAs
preserved. Verification snapshots and raw logs are retained with local QA.

The unchanged released 1.17.74 production feed handlers checked the live feed,
verified/downloaded/staged 1.17.75 in 10.51 seconds. Staged bytes match the
independently downloaded official EXE. The harness used real HTTPS, isolated
temporary settings and only platform-independent feed handlers. No installation
or restart occurred and its original test executable hash remained unchanged.

Official release: https://github.com/TechSavage1989/network-toolkit/releases/tag/v1.17.75
EXE SHA-256: 935c1c74a27777dcb350de20a021a8de37b9f7f94ad63ef5fc9748fad41d057f
Portable ZIP SHA-256: 23cd86caabc2e1c8012a9ca8b3dad1a5116a2f73e5a9a62fc7818f0558573e05
Update ZIP SHA-256: 9d67eac962ff941dbfabebc3e5cc46275cf69410b933bb34a3d169a56cd79893
SHA256SUMS file SHA-256: 94f09a3bec1e8785fd6aaaf98c1ca17f9aa83a3f9d4457a48adea55c4d12b001

This final publication record changes only AGENTS.md, this handoff and
VALIDATION.txt. Do not move the released 75 source tag, replace assets or
republish it. Continue future work from latest main. Native Windows/WebView2,
file dialogs/real servers and actual installation/restart remain unverified;
browser fixtures and live-feed staging are not native Windows execution proof.

## 1.17.74 published: restore the original terminal connection window

Prepared from main c07ef169a90c326ce205a37a34b6223eeacdba41 after the user
confirmed the requested scope: restore the original New Session style from
before Setup was added to the sidebar and remove Setup/Connection settings
from the Terminal sidebar. On 2026-10-25 the user explicitly authorized build
and publication. This covers this scoped release only; preserve prior assets.

Compared the pre-Setup 1.17.54 source retained in its earlier portable package.
Its centered wide modal and responsive two-column connection form remain in
current source. Removed the redirect/inline branch, parked-form module, its
asset route and sidebar-specific form styles. Restored openTerminalConnect
matches the pre-Setup function after replacing its inline form with the current
folder-enabled form helper. Existing New tab empty-tab/protocol selection,
modal title/actions, profile/credential handlers, SSH/Telnet/serial fields,
destination/ownership checks and busy/detached guards are retained.

New connections and saved-profile edits use that modal. The terminal rail now
starts with Saved and retains Recent/Macros/SFTP. No Setup button or Connection
settings panel is mounted. Frontend preference loading and Go sanitization
migrate an old settings-tab selection to Saved while keeping dock/width/collapse
preferences. File-transfer Setup tabs are outside this request and retained.
Saved-session folder assignment remains available in the dialog. No transport
or updater code changes; command-input scrollbar suppression and SMTP's form-only
password lifetime remain intact. Go/UI/package/Windows resources are 1.17.74.

Local validation passed 444 JavaScript tests, 31 Python release guards,
required root/terminal/desktop Go suites, release/resource metadata, HTML/browser
syntax, Windows x64 GUI build and portable/update package verification. Existing
sidebar expectations were updated for the removed tab and migration has both
persisted-state/workspace-display and frontend coverage. The optional live
credential browser script was adapted to the modal and syntax-checked only.

Full review Checks 40170178327 passed on exact published source/tag
feb1467a5db9fa5d49a8d454ee9402e80ed82de6 (tree
dbf77129fd50c5938fe134be620824fbc0613a44). Both jobs succeeded. The new
real-page browser regression passed 36 theme/dock/protocol cases across
390/768/1280px, including the short 600px-high wide layout. Modal containment,
responsive columns and reachable footer actions pass. New tab/protocol and
Saved/New actions, profile editing, folders/credential links, cancel/Escape,
keyboard focus, retry, SSH/Telnet/serial fixture connections, retained terminal
DOM, one form instance and secret exclusion pass. Inspected wide/compact
screenshots show the restored centered modal. These use isolated APIs and do
not connect to real terminal hardware or authenticate to real servers.

Existing Hosts/client/server/SMTP/folders/command-input/install browser checks
also pass, along with seven native Chromium updater-timer scenarios including
the real five-minute poll/focus collision. Review artifact 12589993784 matches
the local portable and EXE byte-for-byte. Its outer SHA-256 is
653dacc3e3598effd776814b06f97019a65839f19209b299cd09f31bcc7fce42.
Browser artifact 12590245421 digest and ZIP CRC pass; outer SHA-256 is
900669dee1f305be24eb1c1392814a270fa8a3514c78e9d3db5a0662626aeb90.

Main was fast-forwarded to that identical tested commit using expected old SHA
c07ef169a90c326ce205a37a34b6223eeacdba41 without force. No code or docs changed
between full review and promotion. Release 40170716790 succeeded and created
stable release 423199653. Main duplicate Checks 40170716795 passed its check
job and restored-dialog browser step. Completed review above remains full
runtime validation; this documentation may supersede automatic duplicate runs.

All four actual public downloads match GitHub asset digests. Outer/inner SHA-256
manifests, both ZIP CRCs, the strict four-member updater allowlist, Windows x64
GUI PE/version and standalone/portable/update EXE identity pass. All 686 packaged
source files match the exact published commit. Official portable and EXE match
the reviewed build byte-for-byte. All thirteen earlier release IDs and asset
IDs/sizes/digests remain unchanged, with all thirteen older source-tag SHAs
preserved. Verification snapshots and raw logs are retained with local QA.

The unchanged released 1.17.73 production feed handlers checked the live feed,
verified/downloaded/staged 1.17.74 in 10.09 seconds. Staged bytes match the
independently downloaded official EXE. The harness used real HTTPS, isolated
temporary settings and only platform-independent feed handlers. No installation
or restart occurred and its original test executable hash remained unchanged.

Official release: https://github.com/TechSavage1989/network-toolkit/releases/tag/v1.17.74
EXE SHA-256: c9625795dcbae4825e46deefd0a7289670d9310db3846def8cbf4b7e6eeda620
Portable ZIP SHA-256: 38efe88cce31e652d8991737b9f7d4901822e091354ab515fb66f28614e434ea
Update ZIP SHA-256: d5bf02593ad353f64207e4aad7d4b4a4a6dc8a0d723b959c8960a5ea89e24eec
SHA256SUMS file SHA-256: ace2ac344d97ec451f623fdf541be44847dfc85fc71695471467bb2f30f7109f

This final publication record changes only AGENTS.md, this handoff and
VALIDATION.txt. Do not move the released 74 source tag, replace assets or
republish it. Continue future work from latest main. Native Windows/WebView2,
hardware and actual install/restart remain unverified here; browser fixtures
and live-feed staging are not proof of native Windows execution.

## 1.17.73 published: command input without scrollbar tracks

Prepared from main fff6000c413bd01566891b4b0ea5072e71da3587 after the user
reported native scrollbar tracks inside the terminal command input and explicitly
requested the fix and publication on 2026-10-25. This authorizes this scoped
1.17.73 source/build/release. Future releases need their own authorization.
Main network-toolkit remains the sole source/update feed; preserve prior assets.

The focused fix changes only command-composer textarea scrollbar styling:
scrollbar-width:none plus a WebKit scrollbar pseudo-element with display:none
and zero width/height. It applies to compact and expanded modes. overflow:auto,
white-space:pre, existing dimensions/expansion/resize, selection, caret/keyboard/
wheel scrolling and command text are retained. No macro/transport JavaScript
changes. Other macro editors, outputs and application scrollbars remain governed
by their existing rules. Go/UI/package/Windows resource versions are 1.17.73.

Local validation passed 443 JavaScript tests, 31 Python guards, required root/
terminal/desktop Go suites, release/resource metadata, browser syntax, Windows
x64 GUI cross-build and portable/update verification. The first browser control
assumed classic reserved scrollbar gutters; headless Chromium uses overlays
here, so the test was corrected to verify restored track styles and both logical
overflow axes. The runtime CSS did not change during that test correction.

Full review Checks 40167818500 passed on exact published source/tag
752f40225ef47e7208948fd264efc0e5493530b7 (tree
1a9d74f8173851e2e9b2a9300b6e3cda4fb17ee2). Both jobs succeeded. The new
browser regression checks twelve layouts across 768/1024/1280px, both themes
and both docks. Empty/short/long/multiline commands have zero reserved gutters,
scrollbar-width:none and hidden WebKit tracks. Long-command caret movement,
multiline caret/wheel scrolling, hide/reopen and exact text preservation pass.
No terminal command is sent by these isolated real-page/API-fixture tests.
Inspected compact/expanded screenshots show no scrollbar tracks.

Existing Hosts, client/server, SMTP, saved-folder and final-install browser
checks also pass, along with seven native updater-timer scenarios including the
real five-minute poll/focus collision. Review artifact 12589190184 matches the
local portable and EXE byte-for-byte. Artifact outer SHA-256:
fbd831792a5cf6002c1b116bf79b40af0fd08f3fa9397ddd77594bad7b7a52a1.

Main was fast-forwarded to that identical tested commit using expected old SHA
fff6000c413bd01566891b4b0ea5072e71da3587 without force. No code or docs changed
between completed review and promotion. Release 40168358427 succeeded and
created stable release 423183483. Automatic main Checks 40168358460 passed
its check job and command-bar browser step. The completed review above remains
full runtime validation; documentation may supersede automatic duplicate runs.

All four actual public downloads match GitHub asset digests. Outer/inner SHA-256
manifests, both ZIP CRCs, the strict four-member updater allowlist, Windows x64
GUI PE/version and standalone/portable/update EXE identity pass. All 685 packaged
source files match the exact published commit. Official portable and EXE match
the verified review build byte-for-byte. All twelve earlier release IDs and
asset IDs/sizes/digests remain unchanged, and all twelve older source tags
retain their original SHAs. Verification snapshots and raw logs are retained
with local QA records.

The unchanged released 1.17.72 production feed handlers checked the live feed,
verified/downloaded/staged 1.17.73 in 10.58 seconds. Staged bytes match the
independently downloaded official EXE. The harness used real HTTPS, isolated
temporary settings and only platform-independent feed handlers. No install or
restart occurred and its original test executable hash remained unchanged.

Official release: https://github.com/TechSavage1989/network-toolkit/releases/tag/v1.17.73
EXE SHA-256: f294d9582361f6859fb4aa9a94c2e7658b1e4258be59ae5ca58635fa6d993b16
Portable ZIP SHA-256: 5fca57cf554152454d6038b52c6fabd43d4c8ba306c28bba39a35d073775e053
Update ZIP SHA-256: 9d53622eb90b1815b677ffde2ea5bf1975a76f5d6370d9f35d5da9ef3e0f6e1e
SHA256SUMS file SHA-256: 78fbec5c6f1b75ade1763ddc612d4c5b20f00a45c162b155f768dc1307e00913

This final publication record changes only AGENTS.md, this handoff and
VALIDATION.txt. Do not move the released 73 source tag, replace assets or
republish it. Continue future work from latest main. Preserve saved-session
folders, SMTP password lifetime, protected credentials and single final update
installation. Native Windows/WebView2, hardware and actual install/restart
remain unverified here; browser fixtures and feed staging are not native proof.

## 1.17.72 published: folders for saved sessions

Prepared from main 1c9dea8ffd50bf51dc0596550c1e3c6a0e4f4505 after the user
requested folders for saved sessions similar to macros and explicitly instructed
build and publish on 2026-10-25. This authorizes this scoped 1.17.72 source and
release; future releases need their own user authorization. Preserve all earlier
releases/tags/assets. Main network-toolkit is the sole source/update feed.

Folders follow the macro library's flat named-label approach. An optional
folder field is stored on saved SSH, Telnet, serial and FTP/FTPS profiles. The
Saved sidebar offers All folders, Unfiled and named folders, with folder-name
search and labels on rows. Existing sessions remain unfiled. Folder choices
are derived from saved records; an unused folder disappears automatically.
The selected filter is temporary UI state, like the macro folder filter.

The connection form and Save/Duplicate/Rename dialogs can choose or create a
folder by typing. Existing saved terminal/FTP rows have a Move to folder button;
terminal context menus offer the same action. Blank returns a session to Unfiled.
Moving preserves profile IDs, protected credential references and macro bindings.
Duplication inherits the folder. Both SFTP/FTP client profile rewrites and the
FTP startup loader preserve assignment. No macro library data is modified.

saved-session-folders.js normalizes/escapes labels and shares choices/filtering.
Labels are bounded to 100 Unicode characters with ASCII controls removed.
The backend named-profile and FTP allowlists accept only that bounded string.
Terminal connection configurations exclude folder metadata. Existing workspace
sanitization preserves named profile folders. Labels never become filesystem
paths or transport options. The new asset is explicitly allowlisted for main
and detached routes. The 71 SMTP form-only password lifetime and existing
single final updater installation action are preserved.

Local validation passed all 443 JavaScript tests, 31 Python release guards,
root/terminal/desktop Go suites, release/resource validation, browser syntax,
Windows x64 GUI cross-build and portable/update package verification. Coverage
includes filter/search, Unfiled, moves, stale-dialog protection, duplicate/edit
preservation, both file-client rewrites, HTTP settings/disk reload, credential
exclusion and linked-workspace sanitization. The initial browser run caught
FTP startup normalization dropping the new field; that path is corrected.
Native selects report normal line-height in Chromium, so layout checks use
measured font bounds rather than parsing that word as a number.

Full review Checks 40166424785 passed on exact published source/tag
3ca1db2fb52132e36a7610e006cfb606e6d9a973 (tree
7b10c925b7f3620ada6bfcb7e40d81f1df5099da). Both jobs succeeded, including
folder actions, app reload, saved-session opening with the retained credential
reference and no folder in the connection request. Twelve folder layouts cover
390/768/1280 pixels, both themes and both docks, with selector containment and
font fit. Inspected wide/compact screenshots show the folder selector/actions.
The real page uses isolated API fixtures, not real hardware or network sessions.
Existing Hosts/client/server/SMTP browser checks, final-install cases and all
seven native updater-timer scenarios passed, including the real five-minute
poll/focus collision. Review artifact 12588292955 matches local portable/EXE
bytes exactly; its outer SHA-256 is
0d5e7c475918ac8c62e40059faeb17804a6b8893b410cd107de16ca4ea938a57.

Main was fast-forwarded to that identical tested commit using expected old SHA
1c9dea8ffd50bf51dc0596550c1e3c6a0e4f4505, without force. No code or docs
changed between completed review and promotion. Release 40166916669 succeeded
and created stable release 423175191. Automatic main Checks 40166916735 passed
its check job and folder browser step. The completed review run above is full
runtime validation; docs follow-ups may supersede automatic duplicate runs.

All four actual public downloads match GitHub asset digests. Outer/inner SHA-256
manifests, both ZIP CRCs, the strict four-member updater allowlist, Windows x64
GUI PE/version and standalone/portable/update EXE identity pass. All 683 packaged
source files match the exact published commit. Public portable and EXE match
the verified review build byte-for-byte. All eleven earlier release IDs and
asset IDs/sizes/digests remain unchanged, with all eleven older source-tag SHAs
preserved. QA snapshots, raw logs and verification results are retained locally.

The unchanged released 1.17.71 production feed handlers checked the live feed,
verified/downloaded/staged 1.17.72 in 17.95 seconds. The staged bytes match the
independently downloaded official EXE. The harness used real HTTPS, isolated
temporary settings and only platform-independent feed handlers. No install or
restart occurred and the original test executable hash remained unchanged.

Official release: https://github.com/TechSavage1989/network-toolkit/releases/tag/v1.17.72
EXE SHA-256: 2e55969c271778fff89592e8aa47dbc1fa6f4c6ede1ab0ff9af777a19a55920d
Portable ZIP SHA-256: a47abe6e0cb8851d750c9d0b50f28efcf12cb433cb9266e78ea17c92a7cacfaa
Update ZIP SHA-256: 3cf05e34d1b0b6ba069c8f7becc5c04205c4e15d43c56f7736ce10cdb3847e32
SHA256SUMS file SHA-256: 3f3e3ba691f87a3484155671728a6e8273b5aa45dd945b20596b8b63c60fb075

This final publication record changes only AGENTS.md, this handoff and
VALIDATION.txt. Do not move the released 72 source tag, replace assets or
republish it. Continue future work from latest main. Native Windows/WebView2,
real terminal hardware and native installation/restart remain unverified here.
Do not treat isolated browser tests or feed staging as native runtime proof.

## 1.17.71 published: retain SMTP password for repeat tests

Prepared from latest main df712385f61108ca43271d22ed9e73d3f6e88629 in response
to the request to keep the SMTP password filled after a test and clear it when
switching tools. After the completed review build was delivered, the user
explicitly instructed publish on 2026-10-25. This authorizes the new 1.17.71
source and release; no additional approval is needed for this scoped publication.
Use network-toolkit only. The previous published release is 1.17.70 and all
its source/tag/assets, plus every earlier release, must be preserved.

The only SMTP runtime behavior change removes both explicit password-field
resets in startJob, on accepted responses and errors. The masked password stays
in the current form for repeat tests. A tool switch discards that form and
returning to SMTP creates an empty field. Temporary request options are still
scrubbed after the request settles. No password is added to app state, saved
settings or history; no backend SMTP, TLS, mail-delivery or updater code changes.
The SMTP note explains this form-only lifetime. Go/UI/package/Windows version
resources are consistently 1.17.71 for the new review build.

Seven focused tests exercise the real field renderer, startJob and navigation
handler: successful and failed jobs, request failures/retries, explicit clearing,
input/change state exclusion, navigation and late success/failure responses
without clearing or restoring the new form's password. The new hosted browser
regression uses the actual page and mock SMTP/settings APIs at 390/1280 pixels,
including repeated runs, failed jobs/requests, saved-state exclusion, navigation
and an in-flight old-form response. It sends no actual email or credentials.

Local validation passed all 432 JavaScript tests, 31 Python release guards,
root/terminal/desktop Go suites, release/resource metadata validation and browser
script syntax. Running the new lifecycle tests against exact main's old page in
an isolated fixture reproduces five expected failures (retention, retries and
late responses); all seven pass with the focused fix.

Full hosted Checks 40163863165 passed on exact runtime source
90433f9617e2568e4cb8d00a8287f68ba1d9168d (tree
df12c88886c05f28f54cc6bcd62a73dd62d66590). Both check and browser jobs
completed successfully. This includes the new real-page SMTP regressions at
390/1280 pixels, existing Hosts/client/server browser checks, single-final-install
cases and all seven native Chromium timer scenarios. The real five-minute
poll/focus collision passed with two release checks. No real SMTP requests or
email delivery occurred in the browser fixtures.

Windows x64 GUI cross-build, release/resource validation, portable/update
packaging and package verification passed locally and in the hosted check job.
The downloaded review artifact 12587098378 and local portable match byte-for-
byte; both contain the identical verified EXE. ZIP CRCs and the release
packager's outer/inner manifests, PE/version and updater allowlist checks pass.
Review artifact outer SHA-256:
deb9f34b92c5d1ce59475833e2bdc19936a4e9151d58097027c970d32480a48d
Review Portable.zip SHA-256 (9,052,210 bytes):
541254db32531f5faacf06b1f657a907239ef0539a82cf61d9731a756a2d0a53
Review EXE SHA-256 (14,183,424 bytes):
8f078b44d5bf2b165c749754aaac41cd60a194b19bf3487d769f2a18052833fa

Published source/tag is 0b3ffe7cbc12be9778429651507b442f33073aa1 (tree
f79fcf76ffbca0866245d0aa64c1a3c98a031bd3). It changes only README.txt,
AGENTS.md, this handoff and VALIDATION.txt from reviewed documentation commit
082ca75078458efd989a2cdc61a9474205b5f33b. No application, version, resource
or test code changes after full runtime validation. Main was promoted using
an expected-SHA non-forced update from
df712385f61108ca43271d22ed9e73d3f6e88629. The source portable is repackaged
with current publication documentation; the approved EXE stays byte-identical.

Release 40164554695 completed successfully and created stable release
423161784. Automatic main Checks 40164554729 passed its check job and the
browser steps before the duplicate native timer suite. The completed runtime
run 40163863165 above remains the full validation. Documentation follow-ups
may supersede automatic duplicate checks without changing the tagged runtime.

All four actual public downloads match GitHub asset digests. Outer/inner
SHA-256 manifests, both ZIP CRCs, the strict four-member updater allowlist,
Windows x64 GUI PE/version and standalone/portable/update EXE identity pass.
All 677 packaged source files match the exact published commit. The public
portable matches the local publication-source package byte-for-byte, and its
EXE is identical to the reviewed build. All ten earlier release IDs and asset
IDs/sizes/digests remain unchanged; all ten older source-tag SHAs are preserved.

The unchanged released 1.17.70 production feed handlers checked the live feed,
verified/downloaded/staged 1.17.71 in 11.35 seconds. The staged EXE matches the
independently downloaded official file. The harness used real HTTPS, isolated
temporary settings and only platform-independent feed handlers. No installation
or restart occurred and its original test executable hash stayed unchanged.
Verification records, baseline snapshots and the raw log remain in local QA.

Official release: https://github.com/TechSavage1989/network-toolkit/releases/tag/v1.17.71
EXE SHA-256: 8f078b44d5bf2b165c749754aaac41cd60a194b19bf3487d769f2a18052833fa
Portable ZIP SHA-256: 6afc63126af28e83f00b2e9d6356a18d242cb684f1c3d47d92974e71d54b326b
Update ZIP SHA-256: 5ddf1eb922b2b10e80f304f27ca226a3a2c0a8ee2f3daf6f7d65a35d1a3e54f3
SHA256SUMS file SHA-256: 6150a3a661b814633fd4724cc1f235d88fe3caf90a0929414b79ce06e2019e39

This final publication-record follow-up changes only AGENTS.md, this handoff
and VALIDATION.txt. Do not move the released 1.17.71 tag, replace its assets
or republish it. Continue future work from latest main, preserve SMTP's form-only
password lifetime and keep all earlier features. Future releases need their own
user authorization; the current publish instruction covers this version only.

Native Windows/WebView2 and live SMTP authentication remain untested here.
The unchanged 70 updater and prior native install/restart limits remain as
recorded below. Full runtime validation above applies to this documentation
follow-up; automatic duplicate runs may be superseded without repeating the
already-passed runtime validation.


## 1.17.70 published: small update-flow test release

Prepared from main 61999347587b321dde1f498517dd6970450c0ad6. The user requested
a small change to test whether the new updater flow works and explicitly
authorized publication when complete on 2026-10-25. No additional approval is
needed for the new 1.17.70 source and release after validation.

The only functional-file change is the App settings update heading in
app-updater.js: App updates becomes Network Toolkit updates. Consistent
Go/UI/package/Windows resources are 1.17.70. This is a visible confirmation
marker after installing from 1.17.69. No tests are added for this cosmetic text
change; run the existing required suites, including final-install-flow browser
coverage. Preserve the 69 single-action install flow and all earlier features.

Exact source/tag 538657ac04477b9192cfcd547f5ddc34d2f3eaed (tree
93ab65056c28425a0fd0336dac6b183d7f161c99) passed full Checks 40161679755 before
the identical commit was promoted to main. Its completed results include all
425 JavaScript tests, 31 Python guards, required Go suites, Windows x64 GUI
build/packaging, Hosts/client/server browser checks, seven explicit final-install
browser cases and seven native timer scenarios including a real five-minute
poll/focus collision. Review artifact 12586428949 matched the local verified
portable byte-for-byte. No new test was added for the cosmetic heading.

Release 40162206418 passed and created release 423148288. Main also automatically
reruns Checks 40162206424 on the same tagged source; the completed review run
above is the full runtime validation. Documentation may supersede automatic
duplicate checks without changing the released code.

All four actual public downloads passed GitHub asset digests, outer/inner
SHA-256 manifests, both ZIP CRCs, the strict four-member Update.zip allowlist,
Windows x64 GUI PE/version checks and standalone/portable/update EXE identity.
All 674 packaged source files match the exact published commit. The public
portable and EXE match the tested review build byte-for-byte. All nine older
release IDs and asset IDs/sizes/digests remain unchanged, and all nine older
source-tag SHAs match the prepublication snapshot.

The unchanged released 1.17.69 production feed handlers checked the live feed,
verified/downloaded/staged 1.17.70 in 8.25 seconds. The staged bytes match the
independently downloaded official EXE. The harness used real HTTPS, isolated
temporary settings and only platform-independent feed handlers. No install or
restart occurred and its original executable hash was unchanged. Raw logs and
verification snapshots are retained with the local publication QA files.

Official release: https://github.com/TechSavage1989/network-toolkit/releases/tag/v1.17.70
EXE SHA-256: 1fcd379da1751d082cbc419bfff3a3cb40812f34846832d1716e3d733a811371
Portable ZIP SHA-256: 572a3be4f36e37320b16b8a92345eabf17e95b802971092e076a0afb491bda42
Update ZIP SHA-256: 773e6f128321579deebc481d51d7bfb4f6796d0b3ff40dabc939e3a8debc2879
SHA256SUMS file SHA-256: 542c506c91728839a8d4aab8e9cd4bb0e93746bf36bd040b02f15c0a92ea7478

This final documentation follow-up must not move the released 1.17.70 tag,
replace any published asset or republish it. Continue future work from latest
main and preserve the single final install action and earlier improvements.

User test: run installed 1.17.69, check for updates, approve download, and choose
the single final Install and restart action. After restart, App settings should
show Network Toolkit updates and Installed version: 1.17.70. Native Windows
installation/restart cannot be exercised here, so actual user confirmation is
still required before claiming a complete native end-to-end update test.

## 1.17.69 published: one final update-install action

Prepared from main 74d4a1103c6887fce793ed8ce24e298526d715b0 after the user
requested removal of the redundant Review install and restart option, then
explicitly said to publish when finished. This authorizes the new 1.17.69 source
and release after validation; no further approval is needed for this build.
The prior published baseline is 1.17.68, which is preserved.

app-updater.js has no install-review action. A successful explicit download
automatically changes its still-current notes dialog into the final installation
prompt. Install and restart there is the single installation confirmation.
If that dialog was closed or replaced during download, it is not reopened and
does not replace unrelated work. The staged banner and App settings buttons
also call the final version-bound install action directly without another
dialog. They show the staged version and Save your work reminder.

Explicit download approval, settings flush, fresh staged-version/status checks,
active-operation checks and single-flight installation guards are retained.
No timer or download completion calls install automatically. Failed downloads
stay retryable. Backend installation, staging, recovery and scheduling code are
unchanged, as are the Hosts editor improvements.

Focused UI/controller tests cover direct final confirmation, cancellation then
single-click banner installation, settings installation, repeated clicks,
failed downloads, stale versions, settings-save failures, active operations and
closing a pending download without reopening or installing. Browser regressions
exercise real page DOM transitions and mock install requests at 390/768/1280px,
checking exactly one explicit install POST from dialog/banner/settings. Other
startup/native-timer cases still forbid all download/install calls.

Full Checks 40160319478 passed on exact source
fd9341f48c67204c9987ff4f923c08f30cc4b0ac (tree
2ebc70874caae34ac42fc313f64b57170bde6a23) before main was fast-forwarded to
that identical commit. This is the published v1.17.69 source/tag. No code or
documentation changed between reviewed source and publication. The completed
Checks run includes all 425 JavaScript tests, 31 Python guards, required Go
suites, Windows x64 GUI build, packaging, Hosts/client/server browser checks,
all seven explicit final-action browser cases, and seven native timer scenarios
including the real five-minute poll/focus collision. Inspected 390/1280-pixel
screenshots show a readable final prompt with one Install and restart action.
Review artifact 12586001236 matches the local portable byte-for-byte.

Release 40160830448 passed and created release 423140565. The main push also
starts Checks 40160830439 on that same source; the completed prepublication
run above is the full runtime validation. Documentation follow-ups may supersede
automatic duplicate main checks without changing the tagged runtime.

All four actual public downloads passed GitHub asset digests, outer/inner
SHA-256 manifests, both ZIP CRCs, strict four-member Update.zip validation,
Windows x64 GUI PE/version checks and standalone/portable/update EXE identity.
All 673 packaged source files match the exact published commit. The public
portable and executable match the tested review build byte-for-byte. All eight
older release IDs and asset IDs/sizes/digests are unchanged, and all eight
older source-tag SHAs match the prepublication snapshot.

The unchanged released 1.17.68 production feed handlers checked the live feed,
verified/downloaded/staged 1.17.69 in 11.48 seconds. The staged EXE matches the
independently downloaded official EXE. The harness used real HTTPS and isolated
temporary settings and only the platform-independent feed handlers. No install
or restart occurred; the original executable hash was unchanged. Raw logs and
verification snapshots are retained with local publication QA records.

An already-running older client uses its embedded old prompts once to install
1.17.69. The simpler flow takes effect after 69 launches. Native Windows/WebView2
installation, restart and actual user confirmation of the new flow remain
unverified here; mocked browser install requests are not native installation.

Official release: https://github.com/TechSavage1989/network-toolkit/releases/tag/v1.17.69
EXE SHA-256: cd4dca22be0dc8c1bc1f3381fab058fe40b7f2e3d2db90bd1cce9f1f427de33e
Portable ZIP SHA-256: 0df37e50c7a4139b43c794a718838e1d14c16441d1ed48ae3870e3de6c9de8cb
Update ZIP SHA-256: 34afdad86ff9b66553d43cfba53a258227b1674ed85aa774ee0134648dec65f5
SHA256SUMS file SHA-256: 60405a6b0513bd89a87c1c26adde7ece02a82500cd7a70869a9b6913384e7b91

This final documentation follow-up changes no runtime code and must not move
the released 1.17.69 tag, replace its assets or republish it. Continue from latest
main for future work, preserving one final install action and earlier features.

## 1.17.68 published: Hosts filter text clipping

Prepared from exact main 99ff403c866953706290e312c425a58429f9567e after the
user reported clipped text in the Hosts Entries status dropdown. The completed
review build on review/hosts-editor-1.17.68 was delivered and the user approved
public source upload and publication on 2026-10-24. This approval covers only
the new 1.17.68 release; preserve all earlier tags, releases and assets. The main
network-toolkit repository remains the only source/update destination.

The 32-pixel selector inherited 9-pixel vertical padding and a 1.5 line height,
leaving only 12 pixels for 16.5-pixel text. hosts-editor.css now supplies compact
padding and explicit line height for the selector and adjacent search field.
The selector keeps its 170-pixel width instead of shrinking to 130 pixels, so
the longest With validation issues label also fits. The 31-pixel find/replace
inputs receive equivalent vertical clearance for the same inherited-padding
problem. No editing, file-write, privilege or updater behavior is changed.

The hosted Hosts browser test now checks the content box against measured
font/line height and all four status labels' widths, with space reserved for
the native arrow. It reproduces the released padding defect as a negative
control. Existing twenty responsive light/dark cases remain; additional
light/dark pages cover device scales 1, 1.25, 1.5 and 2. Toolbar screenshots
show the longest label at each scale. Final review source
9fe18dfff0a3fc37b116807be2601361858a9d6b passed Checks 40158776871, including
all required Go suites, 422 JavaScript tests, 31 Python guards, twenty responsive
cases, four status labels, all four device scales in both themes, existing
client/server checks and all seven native update-timer scenarios. The actual
five-minute poll passed. Inspected normal/light and 150-percent/dark toolbar
screenshots show the complete longest label; the 390-pixel layout is readable.
The downloaded review artifact 12584858650 matches the verified local portable
byte-for-byte. Its SHA-256 is
4b7cb87393194bf9188fe49f902148d994601b4cbdc70a73b8708c51af5352a0.
Native Windows/WebView2 remains unverified; browser/DPI results are Chromium
coverage, not Windows runtime.

Version metadata and compile-time Windows resources are 1.17.68. The existing
review-branch workflow supplies a Windows x64 portable source/build artifact.
Local release/resource validation, browser-script syntax, thirty Hosts model
tests and the Windows x64 GUI cross-build passed before source upload.
Published source/tag is 04bffc583445fa48656c40dc44a80af4c54e5edc (tree
e41000868502bd10d1855c05072dcaaea1011da2). It changes only AGENTS.md, this
handoff and VALIDATION.txt from the reviewed runtime source. Release
40159396027 passed and created release 423132212. Main source Checks
40159396034 passed both jobs, including all compact text/DPI checks, existing
client/server and updater startup checks, and all seven native Chromium timer
scenarios. Its real five-minute poll and focus collision passed.

All four actual public release downloads passed their GitHub asset digests,
outer/inner SHA-256 manifests, both ZIP CRCs, the strict four-member updater
allowlist, PE/version checks and standalone/portable/update EXE identity. All
672 packaged source files match the exact published source commit. The official
EXE is byte-for-byte identical to the approved review build. All seven older
release IDs and asset IDs/sizes/digests are unchanged, and all seven older
source-tag SHAs match the prepublication snapshot.

The exact released 1.17.67 source's unchanged production feed handlers checked
the live main feed, verified/downloaded/staged 1.17.68 in 10.76 seconds. The
staged bytes match the independently downloaded official EXE. The harness used
real HTTPS and isolated temporary settings and called only the platform-
independent feed handlers. No installation/restart occurred and its original
executable hash was unchanged. The raw log, baseline snapshots and download
verification record are retained with this publication's local QA files.
Native Windows execution, hosts-file save/elevation, installation and restart
remain unverified. The user's approval is not native runtime confirmation.

Official release: https://github.com/TechSavage1989/network-toolkit/releases/tag/v1.17.68
EXE SHA-256: 5fda37ea59822b2131febe39c685743cbcdbfbac4020cac31e55828d5af597b5
Portable ZIP SHA-256: b81701b207444e8012a6ca153884386d9fe67cd9a6a24b237d62e1e4424b9e30
Update ZIP SHA-256: 0c5837f10f3c6a30ac7070767d80de3624a8edfa7e9478c6ad581bc400e99ee1
SHA256SUMS file SHA-256: db1fb17890f1df0496709d36577abd866d1e75a85523118ee8e8071533f77d01

This final documentation follow-up changes no runtime code and must not move
the released 1.17.68 tag, replace its assets or republish it. Continue from latest
main for future development, preserving the corrected compact Hosts controls.

## 1.17.67 published: expanded Hosts editor

Prepared from exact main 2d071bec99979397e3d80cec6db5d7d4716c4e3e on
review/hosts-editor-1.17.67 in response to the request for more Hosts editor
features. The user approved public source upload and publication on 2026-10-24
after the completed review build was delivered. The published baseline before
this publication is 1.17.66. This approval covers only the new 1.17.67 release;
preserve every existing tag, release and asset. The main repository remains the
only source and update feed. The retired second repository must not be changed.

New code is hosts-editor-model.js, hosts-editor.js and hosts-editor.css. The
existing index delegates Hosts rendering, controls and validation to the module.
All actual file saving still uses reviewLocalConfiguration/applyLocalConfiguration
and the original local-tools hosts request, revision check, elevation helper,
encoding/newline preservation, atomic replacement and automatic backup code.
No backend file-write or privilege behavior is changed.

Features: Entries/Text views; add/edit mappings with aliases/comments; individual
and bulk enable/disable/delete; text/status filters; duplicate/conflict/invalid
line diagnostics; undo/redo; literal find/replace with case/whole-host options;
line navigation/wrapping; format/sort/deduplicate previews; import/restore or
append UTF-8/UTF-16/Windows-1252 text; draft/loaded-copy UTF-8 exports. Ordinary
comments and unrelated lines are preserved. Exact deduplication retains distinct
comments, alias order and disabled copies. IPv4 plus IPv6 for one name is valid.

Drafts, import bytes and bounded undo history stay in memory and are not placed
in saved app state or reports. Entry lists show 100 rows per page; validation
counts all rows and caps displayed issues at 200. Undo history is bounded to
100 checkpoints and approximately 16 MiB of text. The backend retains its 1 MiB
hosts-file limit. Exported text copies are not byte-for-byte original backups;
original automatic backups remain under the existing protected save process.

Local validation passed all 422 JavaScript tests (30 new Hosts model tests),
31 Python packaging tests, root/desktop/terminal Go suites, release/resource
validation, Windows x64 cross-build and portable/update packaging verification.
Hosted Hosts browser interactions passed on fe9189c (Checks 40156261618):
entry edits, aliases/comments, inactive/active/bulk changes, reviewed saving,
operation locks, reload protection, literal find/replace, validation navigation,
UTF-16 backup import, explicit export and paginated selection. Twenty rendered
cases passed at five window sizes in both themes. The first run caught only a
browser-added export extension; exports now use explicit hosts-draft.txt and
hosts-loaded-copy.txt names. Subsequent polish excludes the source textarea
from global one-click input clearing and supplies complete tab semantics.

Final reviewed runtime source 3c379fb1653b1f24de3f820bc5bf525acfaa9b52 passed
Checks 40156697029, including all twenty Hosts rendered cases, existing client
and server layout checks, automatic startup/release notes, and all seven native
Chromium timer scenarios. The real five-minute poll and focus collision passed
with two checks and no overlap. Review portable artifact 12585125922 matched
the local verified build byte-for-byte and passed ZIP/checksum verification.

Published source/tag is c88448fd1b4e2290b5614e35f8485968c5786413 (tree
656a06eb33bbd5cbbbbf19c8d66994789d1516bc). It changes only AGENTS.md,
this handoff and VALIDATION.txt from the reviewed runtime source. Release
40157685254 passed and created release 423122247. Main source Checks
40157685265 passed both jobs, including all twenty Hosts rendered cases,
existing client/server layouts, updater startup and all seven native Chromium
timer scenarios. Its real five-minute poll and focus collision passed.

All four actual public release downloads passed GitHub asset digests, external
and internal SHA-256 manifests, both ZIP CRCs, strict four-member Update.zip
validation, PE/version checks and standalone/ZIP EXE identity. Every one of the
671 packaged source files matches the published source commit. The official
EXE is byte-for-byte identical to the approved review build. All six older
release IDs and their asset IDs/sizes/digests are unchanged; the 63/64/65/66
tags retain their original verified source commits.

The exact released 1.17.66 source's production feed handlers checked the live
main feed, verified/downloaded/staged 1.17.67 in 9.80 seconds. The staged EXE
matches the separately downloaded official EXE. The harness exercised only
the platform-independent feed handlers with real HTTPS and isolated temporary
settings; no installation/restart occurred and its original executable hash
was unchanged. The verification record and raw log are retained with this
publication's local QA files. Native Windows hosts-file save/elevation,
installation/restart and actual runtime remain unverified.

Official release: https://github.com/TechSavage1989/network-toolkit/releases/tag/v1.17.67
EXE SHA-256: eb16ac88a4b92030516df84f4b6b810e9e3e1afcee8101193efbc001926b7192
Portable ZIP SHA-256: 4fb58cd86f01926e1c73b8c5656056ac81106e7da1ddb964a7dc7f677541e3ff
Update ZIP SHA-256: 8b0ead9ee3826abd0f9e7cef609c64be55128f14e1314a1c75a249059af0605d
SHA256SUMS file SHA-256: dff3f03dee787fb22a07f78e8bf68c173f3133cad18d29abce76516b25c3f2c4

This final documentation follow-up changes no runtime code and must not move
the released 1.17.67 tag, replace its assets or republish it. Continue from latest
main for future work, preserving the new Hosts editor and existing updater.


## 1.17.66 published: concise review and faster detection

Prepared from exact main ed078e0bced8a4d0dbc86a7bcaf9532795cc9da6, including
the final 65 documentation. The user approved public source upload, hosted
checks and publication after those checks pass on 2026-10-25. The install
review removes only its long explanatory paragraph and replaces the large
warning with "Save your work before restarting." Notes/version badges, explicit
confirmation, error messages and all active-operation guards remain. Full
installation/recovery details remain in UPDATING.txt. Release notes show complete
highlights only once; Read more contains additional content, retaining complete
text for shortened highlights. Equal installed/release versions use one badge.

Automatic checks now run every five minutes while visible and online. Focus,
visibility and reconnect events share a two-minute freshness gate, one timer
and the controller single-flight guard. Retries and server cooldowns cannot be
accelerated by focus. The root-bound native timers, saved opt-out, 24-hour
reminders and independent startup are preserved. Hidden/offline checks pause;
this is periodic detection, not server push or OS notifications.

Release metadata uses conditional requests and validated cached release details.
GitHub rate and polling headers govern the next allowed check, including manual
checks. Unauthenticated conditional requests still count against the shared-IP
quota. No account credentials, new infrastructure, or notification permission is
introduced. Installer, staging, cleanup and restart behavior are unchanged.

Local validation passed: 392 JavaScript tests, 31 Python tests, root/terminal/
desktop Go tests, focused updater race tests, release/resource validation and
Windows x64 GUI cross-build. Independent review cleared the changes.
Local browser launch was
blocked by the executor sandbox. Hosted review Checks 40149250883 and main
Checks 40149810832 both passed. The inspected 390/1280px screenshots show the
short warning, explicit confirmation, additional-only Read more and single badge
for equal versions. Seven native Chromium scenarios passed without replacing
native timers; the actual periodic check ran after 300,075.68 ms and never
exceeded one in-flight request. Native Windows/WebView2 and real installation
have not been tested here; user runtime confirmation remains pending.

Release 40149810863 published v1.17.66 at source/tag
de0a635eb0a3270c69e3c704beab1ea1001b55af (tree
45547eaa2635ffafa65287338269a827ee6404f6). All four downloaded official assets
passed their hosted SHA-256 values, outer/inner manifests, both ZIP CRCs,
strict four-member updater validation, PE/version validation and EXE identity.
All 665 source files match the reviewed tree. The exact released 65 production
handlers checked the live main feed, verified/downloaded/staged 66 in 11.21
seconds without installing or modifying the original executable. Its raw result
log is retained alongside the verification records. The 65 tag and all its asset IDs
and digests remain unchanged. This final documentation commit changes no runtime
code and must not republish or alter the released 66 tag/assets.

## Published 1.17.65 baseline

Previous released version: **1.17.65**, source/tag
97a0b4c3902e10acc11b1c78715e1ab7ba5de6c4 (tree
445b2b768df738dd5fb9fbaf33e8f2da4e3fe320), built from exact prior public main
f4754d11a9df4d93c7028a1a65b20536921dc9ef. The user approved this narrow timer
fix and publication on 2026-10-25. Review Checks 40147301635 and 40147544940,
main Checks 40147728836 and Release 40147728832 all passed. Publication and
asset verification are complete; see the final record below. The user's native
Windows/WebView2 confirmation of automatic startup in 65 is still pending.

## 1.17.65 native update timers

The production scheduler environment now wraps both setTimeout and clearTimeout
through the Window root. Previously, bare native functions were invoked as
methods of the scheduler environment, giving them the wrong receiver. The
receiver-enforcing integration fixture reproduces the resulting exception after
nextAt is set but before any live timer is registered. The wrapper preserves
injected test clocks while providing the correct receiver to native browsers.

The native browser suite uses the actual index page and native timers, without
page.clock or timer replacement. Its negative control restores only the exact
released 64 timer wiring; the candidate must demonstrate the positive counterpart.
Hosted review Checks 40147301635 passed on bb2fc58. In Chromium 155.0.8059.0,
the exact released wiring throws Illegal invocation at schedule(), makes zero
feed checks and leaves the stale scheduled state. All six native scenarios pass
with the wrappers, unchanged native timer identities and at most one active
feed check. The real retry occurred about 30.02 seconds after failure. Manual
and toggle cancellation were observed past the old retry deadline. This confirms
the native Chromium defect, not a direct inspection of the user's Windows runtime.

Retained unchanged: saved opt-out and startup preference loading, six-hour checks,
30-second/2-minute retries and bounded backoff, manual checks, per-stage/version
24-hour reminders, safe release notes, stable executable naming, settings backup,
all revised 63 features, and explicit download/install consent. No installer,
cleanup permissions, file-storage or restart behavior is modified. The user
independently removed a stale folder to resolve their earlier cleanup warning;
this release does not claim to fix that separate warning.

Local validation passed: 379 JavaScript tests, 31 Python tests, root/terminal/
desktop Go tests, release/resource validation and Windows x64 GUI cross-build.
Independent review found no confirmed must-fix and passed 75 focused JS tests.
The final test scrolls to update status before screenshots: inspected images
show the old stale schedule versus the fixed last attempt and next check. Native
Windows/WebView2, real installation and restart remain untested here. Retain all
old release tags/assets and use the main repository only; do not write to the
retired update repository or change repository access.

## Published 1.17.64 baseline

The current released application is **1.17.64**, based on verified public main
71df414ea97496c26565df12e9e2ac5a0d3c3203. The last published/user-confirmed
baseline is revised 1.17.63, source tag 6bb00ea7224ce1e9e0b4b2db4b806915fc9947aa.
Repository: https://github.com/TechSavage1989/network-toolkit. Preserve README.md.
Release/source tag v1.17.64 resolves to 991f32387c52a35d32f86d13257ab8ef86014b38.
The source tree exactly matches the privately backed-up local build d1e81be.

## 1.17.64 automatic checks and release notes

The actual index initializer starts the updater after saved update preferences
are read, independently of /status, /system and /jobs. The first scheduler task
has zero delay. Local startup/status reads abort after 10 seconds; feed checks
have a 20-second backend deadline and a 25-second frontend deadline. Timed-out
responses cannot overwrite later results. Feed requests ask for cache
revalidation. Automatic checks ignore cached check timestamps and retain the
existing 30-second, 2-minute and bounded six-hour retry cadence.

A failed full-settings merge caused by intervening diagnostic work no longer
blocks reading the saved updater opt-out. Unsaved work remains unchanged, and
the existing full-state writer refuses to persist startup defaults. A missing
or failed preference read never assumes opt-in: retry the local read instead.
The main banner can render even while unrelated platform startup is pending.

The automatic banner opens a safe What's new review before explicit download;
installation still requires separate Install and restart confirmation. The new
app-update-notes.js helper renders four compact highlights, version badges and
keyboard-accessible full notes as escaped, inert text. It executes no release
body HTML or links. Install review uses notes only when the feed release
matches the staged version; otherwise it honestly reports unavailable notes.
The existing 24-hour per-version/stage snooze is preserved and now explicitly
labelled Remind me tomorrow, including the across-reopen behavior in settings.

Verified code gaps fixed here: the unnecessary five-second delay, indefinitely
pending local startup requests and an unrelated settings-edit guard blocking
updater preferences. These do not prove the cause of the earlier Windows
missing-banner report. Adapter/sidebar controls and installer/restart logic
remain unchanged.

Local validation: 377 JavaScript tests, 31 Python tests, root/desktop/terminal
Go suites, release/resource validation and Windows x64 GUI cross-build passed.
The real-index VM integration covers hung startup dependencies, saved opt-out,
preference timeout/retry, late feed replies, navigation/dialog preservation and
no automatic download/install. Independent scoped review passed 73 updater/notes tests and focused Go
updater checks, with no confirmed must-fix issue.

Rendered Chromium tests were extended for immediate startup and safe notes at
390/768/1280px, with keyboard disclosure and screenshots, and added to Checks.
They could not run locally because Chromium IPC creation was sandbox-blocked
and escalation failed environment setup. They subsequently PASSED on hosted
review Checks 40144964705 and main Checks 40145219932. Downloaded screenshots
at all three widths were inspected: readable version badges/highlights, no
card overflow, visible keyboard focus, disclosure and actions. Script/link
security fixtures remained inert. Native Windows/WebView2 remains unverified.

The user explicitly authorized this public source upload and v1.17.64 release
on 2026-10-24 at 23:56:13 UTC. A private complete prepublication backup was
saved before promotion. Review branch CI and screenshots were verified before
main was moved with a checked expected SHA. Release 40145219925 succeeded on
2026-10-25; main Checks 40145219932 also passed. Published63 remains unchanged.
Future publication remains a separate authorized step; main pushes publish new
versions. Use main network-toolkit only, never the retired update feed.

## Previous 1.17.63 baseline

This release starts from exact 1.17.63 commit
8df9b401111e09767591491bc973b4160a17a9bc, retaining its native server adapter
selectors, layout and behavior. Only update-feed routing, release packaging,
version metadata, regression tests and documentation are changed.

## Single public source and release destination

Use network-toolkit for future source and release updates. The owner requested
revised 1.17.63 itself switch feeds and be offered through the old repository
once, so 1.17.62 clients can migrate. Original 1.17.63 was already published on both repositories. The owner
authorized revised 63 replacement and revised source publication on 2026-10-24,
and stated nobody has 1.17.63. Keep the same-version installation caveat for
any copy downloaded earlier. Do not silently overwrite or move existing tags.
Do not delete repositories or change their access/security settings.

An already-installed original 1.17.63 cannot discover a same-version update;
it needs manual EXE replacement. Earlier clients can receive the revised 63
bridge now published on the old feed. No later release uses the second feed.

The existing main-push Release workflow publishes a new version once, with
standalone EXE, source Portable.zip, binary-only Update.zip and combined
SHA256SUMS. The updater accepts only the new repository's exact asset URLs.
The strict Update.zip has four members and the same executable bytes as the
standalone asset. Never use Portable.zip for the in-app updater.

## Build and checks

Use Go 1.23.2 as declared in `go.mod`, vendored modules (`-mod=vendor`), Node 24
and Python 3 (local validation used 3.12; CI selects 3.13). Windows x64 is the
shipping target. The application requires WebView2 for its Windows desktop UI.

From the repository root:

```sh
python3 scripts/github_release.py validate
python3 -m unittest discover -s scripts -p '*_test.py' -v
go test -mod=vendor . ./internal/terminal ./internal/desktop
node --test scripts/*.test.cjs
python3 scripts/build_windows_resources.py
sh ./build.sh
```

On Windows, `BUILD_WINDOWS.cmd` builds the same GUI executable:
`build/Network_Toolkit.exe`. `Open_Terminal.cmd` opens its terminal mode.
Keep `vendor/`, bundled web assets, the vendor database and the compile-time
`resource_windows_amd64.syso`; regenerate resources after version changes.
The Go build uses CGO_ENABLED=0, GOOS=windows, GOARCH=amd64, GOTOOLCHAIN=local,
`-buildvcs=false -trimpath -ldflags='-s -w -H windowsgui'`.

Complete release packaging:

```sh
python3 scripts/github_release.py package --tag v1.17.67 --exe build/Network_Toolkit.exe
```

Use the tracked checkout for release packaging; `--allow-untracked-source` is
only for a consciously reviewed local snapshot. Binary-only packaging is also available:

```sh
python3 scripts/package_public_update.py --exe build/Network_Toolkit.exe --output-dir public-dist
```

Do not commit generated EXEs, archives, build caches or checksum inventories.

## Updater and installation invariants

Official executable names normalize once to `Network_Toolkit.exe`; custom
names and existing stable-name collisions are preserved. Future updates retain
that stable launch path. Settings are retained. Updater journals, staging,
helpers, logs and the single immediately previous rollback executable belong
under the settings directory, separated by installation identity. Do not add
permanent sidecars beside the EXE. Brief atomic transaction/recovery files may
remain until verified cleanup is safe, especially across drives or with locks.

Migration requires committed installation, old helper exit and a genuinely
healthy canonical reopen. Cleanup is hash- and journal-verified, not a broad
filename deletion. Exactly matching shortcuts in the current user's Desktop
and Start Menu can be retargeted with original backups in settings; pinned,
shared and other-location shortcuts are not migrated. Preserve custom fields,
icons, arguments and unrelated or changed files. Relevant implementation and
regressions are `updater_*`, `lifecycle_*`, `terminal_views_lifecycle.go` and
`internal/desktop/startup*`.

The public feed uses GitHub Releases `/releases/latest`, stable `vX.Y.Z` tags,
`Network_Toolkit_X.Y.Z_Update.zip` and an external `SHA256SUMS`. The update ZIP
must contain exactly four members under `Network_Toolkit_X.Y.Z/`:

1. `Network_Toolkit_X.Y.Z_Windows_x64.exe`
2. `UPDATING.txt`
3. `THIRD_PARTY_NOTICES.txt`
4. `SHA256SUMS` (hashes the other three members)

The versioned filename **inside this ZIP is deliberate compatibility**, even
though the installed/direct-download executable is `Network_Toolkit.exe`.
Do not rename the ZIP member without testing all supported older updater
versions. The source-containing portable package uses the stable EXE name and is not the
public updater payload. Validate outer ZIP checksum, inner executable checksum,
PE x64 GUI resources, version and exact member allowlist. Checksums provide
integrity, not publisher authentication; releases are not Authenticode signed.
Automatic checks/reminders do not authorize automatic download or installation.

## Validation status and unresolved work

Revised 1.17.63 currently passes 359 JavaScript checks, 31 Python checks,
release metadata/resource validation, root/desktop/terminal Go suites and
Windows x64 GUI cross-compilation. Independent migration/revision-mode review
passed. Hosted source Checks, including rendered Chromium server/client layout
regressions, and the guarded main release revision completed successfully.
Read VALIDATION.txt and the release record below for verified provenance.

1.17.63 passed the required root/desktop/terminal Go suites, all 358 JavaScript
checks, fourteen Python checks, release/resource validation and Windows x64 GUI
cross-compilation. The then-private Checks workflow also passed rendered server and
client regressions with real Chromium: native selector/client style parity,
adapter/port alignment, mouse and keyboard selection, saved-address reload,
activity resizing, sidebar docking and five window sizes in both themes.
The public ZIP was checked against its exact four-member allowlist and both
internal and external SHA-256 checksums. Native Windows/WebView2 execution
remains unverified; this does not establish Windows updater runtime behavior.

1.17.62 passed focused/root/desktop/terminal Go checks, updater race checks,
Windows executable and test cross-compilation, 14 Python tests and 355 JS tests
when packaged. See `VALIDATION.txt` for precise limits. Some full `go test ./...`
network/interface tests need privileges unavailable in the Linux sandbox.
Native Windows/WebView2, real shortcut COM APIs, mapped-image replacement,
canonical reopen and elevated helper identity were not executed here.

The revised **1.17.63** update is now the user-reported working baseline.
On 2026-10-24 at 23:48:27 UTC, the user confirmed: "the update works."
This confirms the reported update experience, not exhaustive native Windows,
shortcut, rollback, cleanup, elevated-tool or feature testing. Keep the remaining
platform limits above. The missing-banner report still has no proven root cause.

The reported missing automatic update banner has **no proven root cause**.
Manual checking succeeding does not establish the original cause. Improvements
wait for saved preferences independently of unrelated system/job requests,
retry early transient failures and display automatic-check status. Reproduce
on Windows and record preferences, last/next check state and timings before
claiming the original issue is fixed. Never log credentials or private results.

## Continuing development

1. Start from latest `main`, inspect current changes and the user's next
   requested improvement, and retain existing unrelated work.
2. Make focused changes with regression tests. Increment the version for each
   new distributable build; keep Go/UI/package metadata, README/release notes
   and Windows resources consistent.
3. Run the checks above and applicable Windows runtime tests. Record honest
   validation limits and user-confirmed results.
4. Update this handoff whenever implementation, release routing, validation,
   known issues or next steps change. Keep it current so new development chats
   do not inherit stale assumptions. AGENTS.md points new chats here.
5. Sync authorized source/tests/build assets and handoff to network-toolkit.
   Inspect remote HEAD first; preserve unrelated work and never force-overwrite.
   Its release workflow builds all four assets together for the new version.
   Do not publish future builds to the second repository beyond any explicitly
   authorized migration exception, and never overwrite an old release.
6. Verify remote commit, terminal workflow outcomes, release tag/assets/hashes
   and updater metadata. Share the main repository release link and describe
   any one-time bootstrap requirement. Do not claim unperformed runtime tests.

## Explicit same-version repair and recovery

Ordinary main pushes never replace an existing release. A separately approved
same-version correction uses Release workflow_dispatch with revise_existing,
exact tag, original tag commit, original SHA256SUMS file digest, and exact new
source commit. It must be dispatched from main after new-source checks pass.
The revision helper is restricted to network-toolkit, not the binary bridge repo.
Before dispatch, retain original assets and metadata in durable private storage;
the job also verifies and saves a 90-day original-release recovery artifact
before replacing any asset. Publishing is blocked on backup failure.

Revised assets are verified locally, uploaded one at a time with SHA256SUMS
last, and rechecked against GitHub digests. Only then is the existing lightweight
tag moved to the tested new commit and the release notes refreshed. The release
record is retained. No repository permission or credential change is needed.

If an upload, verification, tag move or notes edit fails, STOP. Do not rerun the
workflow blindly: its strict original-state guard intentionally rejects partial
replacement. Preserve the failed run, original recovery bundle and built new
artifacts. Read current release ID, every asset name/size/digest and tag commit.
Classify each asset as exact original, exact approved new, missing or unknown.
Unknown bytes, an unexpected tag or extra assets require owner review; do not
overwrite them. Record exactly which actions completed, including uncertain
outcomes. Reconcile read-only before asking for the specific remaining repair.

With appropriate authorization, finish only the missing approved new uploads
(checksums last), or restore exact original asset bytes from the durable backup
and their original manifest. Verify all remote hashes, tag/source provenance
and release metadata before declaring recovery. A tag move or published asset
replacement is never inferred from a successful partial upload. Do not delete
an entire release or repository to recover. Existing clients reject mixed
ZIP/checksum combinations; nevertheless ask users to wait while reconciling.

When manual dispatch is unavailable, the one-time release-revision-v1.17.63
branch is a bounded alternative. Only that exact branch triggers request mode,
with .github/revision-request.json containing tag, old_commit, old_manifest and
new_commit. The first three fields are hard-pinned to the original63 release;
new_commit must equal current main. The request commit must descend from that
source and change only this request JSON. Build detaches the selected source.
Before replacement, both source main and request branch must still match their
recorded commits. Replay after successful replacement fails the original-tag
and original-asset guards. Ordinary main pushes never use request mode.

## Revised 1.17.63 publication record (2026-10-24)

The released application/source tag is
6bb00ea7224ce1e9e0b4b2db4b806915fc9947aa. Main source Checks run 40143510699
passed; main revision Release run 40143608659 passed. Request-only commit
cf38b17aa28402f0b17a5d4eafa326b8fa27c524 also passed Checks 40143608763.
The original main release ID 423021173 was preserved and its actual v1.17.63
tag now resolves to the revised source. GitHub's historical target_commitish
field may retain the original creation target; use the resolved tag for source
provenance, not that informational creation field.

Official release: https://github.com/TechSavage1989/network-toolkit/releases/tag/v1.17.63
EXE SHA-256: b2e9232330a0098134dcd8c41ba8a1fa6df02908f8acb879aab4bf98695b0f18
Update ZIP SHA-256: 6bf8305211166b14841505e5bca1e4a9358051ac47d45060a865a5dd64fac7d7
Source Portable ZIP SHA-256: 2b1d4dbb3a889925a04139c50657cc7ec0fbfc1fa3e28770533853915e455025
All three downloaded official assets passed external manifests, inner checksums,
strict updater allowlist, PE/version and matching-executable verification.
The exact hosted ZIP is the bridge payload; independently repackaging ZIPs can
change their hashes through timestamps even with identical executable bytes.

The main repository's consumed request-branch push trigger has been removed
from its current workflow. Historical request branches and releases are retained;
old-state checks prevent replay. Ordinary main pushes still publish only new
versions. Explicit manual revision mode remains guarded, not blanket overwrite
permission. This final handoff/validation maintenance does not rebuild or change
the already verified application assets or move their source tag.

Legacy bridge completion: old-feed revision run 40143885776 succeeded,
preserving release ID 423040841 and its historical tag. Its two published assets
are the exact same Update.zip hash above and a binary-only checksum manifest.
The archived original 1.17.62 updater fetched the real old feed and successfully
verified/staged the revised 63 EXE hash above. The unchanged revised 63 updater
then checked the real main feed, reported current/latest63 with no false update,
and rejected same-version download without staging or installation. These are
Go/API/feed compatibility checks, not native Windows GUI/restart tests.
The old repository is retained solely for this completed bootstrap and history;
all later updates use the main repository.


## 1.17.64 publication record (2026-10-25)

Release ID: 423054360
URL: https://github.com/TechSavage1989/network-toolkit/releases/tag/v1.17.64
Source/tag: 991f32387c52a35d32f86d13257ab8ef86014b38
Review Checks: 40144964705; main Checks: 40145219932; Release: 40145219925.
All terminal outcomes succeeded. This documentation follow-up does not rebuild,
replace release assets or move the application source tag.

Published EXE SHA-256:
9d0affc356695edd4bd748d25865fac707973fb68c6d2968bc5609137e9ece61
Published source Portable.zip SHA-256:
25a98904fb51e0db7c5340b5dc5770aa111a4b285d012470956a9d4b0b3d02e5
Published Update.zip SHA-256:
712737fbc25f6e3e06bd5364c3305e353b800789ead09246db2e633bf8d9bc32
Use the hosted Update.zip hash, not a local repackaged ZIP hash: timestamps can
change ZIP bytes even when the executable and members are identical.

All downloaded official assets were verified against the hosted outer manifest.
The Update.zip exact four-member allowlist, both ZIP CRCs, every inner manifest,
Windows x64 GUI/version resources and matching EXE bytes in both ZIPs passed.
The actual unchanged revised 63 updater (source 6bb00ea) fetched main release 64,
verified/downloaded/staged the expected EXE successfully in 12.63 seconds.
The installed executable was unchanged; no installation or restart was attempted.
This is production-feed/API/staging compatibility, not native Windows UI proof.

The improved automatic startup and notes interface runs only after installing
1.17.64; it cannot retroactively change the UI of installed 1.17.63. A subsequent
newer release will exercise that UI on the user's actual installation. No user
confirmation of the new 64 UI or native runtime is recorded yet.


## 1.17.65 publication record (2026-10-25)

Release ID: 423068312
URL: https://github.com/TechSavage1989/network-toolkit/releases/tag/v1.17.65
Source/tag: 97a0b4c3902e10acc11b1c78715e1ab7ba5de6c4
Tree: 445b2b768df738dd5fb9fbaf33e8f2da4e3fe320
Initial review Checks: 40147301635. Final review Checks: 40147544940.
Main Checks: 40147728836. Release: 40147728832. All succeeded.

Published EXE SHA-256:
de50ade7bf554e8d6140e2dca802c37deea2ee575e750f5945390404ee271d40
Published source Portable.zip SHA-256:
671d639e605d6ee745203f92d647ddacbb7d6d8e873c9d790ee8080155d5a3c6
Published Update.zip SHA-256:
6ba4c8b462b1026571e60744b1085216cb6d313db03e4c4c769f168a1e8fd47d

Downloaded official files passed every outer and inner manifest, both ZIP CRCs,
strict four-member Update.zip allowlist, Windows x64 GUI/version checks and exact
EXE identity across all packages. All 663 published source files match the exact
reviewed tree. The hosted EXE matches the privately backed-up local build. Use
the hosted ZIP hashes above, not a separately generated timestamped updater ZIP.
Release 64 ID, tag and all asset digests were verified unchanged after publication.

The unchanged released 64 production updater, sourced from the verified 64
Portable.zip, fetched the real main feed, verified/downloaded/staged the exact
65 EXE in 11.67 seconds. No installation/restart ran, and the original running
executable was unchanged. This proves feed and staging compatibility, not a
native Windows install. Full native UI/install/restart remains unverified here.

The user was told to use Check for updates once from installed 64, install 65,
then reopen and allow an automatic attempt without pressing Check. Only 65 has
the timer fix; publishing it cannot repair the already-running 64 scheduler.
User confirmation of that native automatic attempt is pending. Do not claim it
based on the hosted browser proof. The separate stale-folder cleanup warning
was resolved independently by the user and is not changed by this release.

This documentation-only follow-up does not change application code, rebuild the
released EXE, replace published assets, or move the source tag. Private complete
prepublication and verified-published backups preserve source, build assets,
validation and handoff. Continue using main network-toolkit only.
