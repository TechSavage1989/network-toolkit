# Network Toolkit development handoff

## 1.17.78 candidate: detached terminal sidebar side

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
behavior in released77 and pass with the fix. The existing real-page terminal
browser fixture now checks both sidebar placements, workspace capture/apply and
single-session detachment immediately after swapping sides. It uses isolated
APIs, not native windows or actual external transports.

Local full checks passed: 448 JavaScript tests, 31 Python release guards,
required root/terminal/desktop Go suites, release/resource metadata, browser
script syntax and Windows x64 GUI build. Hosted browser/build artifacts,
publication and released77 live update staging are pending. Native Windows/WebView2, hardware and actual
installation/restart remain unverified.

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
