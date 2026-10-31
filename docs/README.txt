NETWORK TOOLKIT 1.17.144
======================

CURRENT RELEASE - 1.17.144 (BETA 8 CANDIDATE)

Beta 8 continues the full application-wide performance and polishing
roadmap. Diagnostic/discovery lists use one derived key per sort row,
and list counts and visible content reuse one filtered result per refresh.
Long Ping and Monitor histories avoid unbounded RTT argument arrays.
Unfiltered discovery avoids unused search-field construction. Large
reports use fewer temporary allocations, and unchanged UI status text
does not trigger redundant accessibility writes.

This is an opt-in prerelease candidate, not yet published. Stable remains
1.17.136. Credentials, network permissions, settings and Network_Toolkit.exe
stay unchanged. The historical narrow Editor map-motion CI issue and
physical Windows stress acceptance remain open and are not declared fixed.

Read docs/DEVELOPMENT_ROADMAP.md, docs/BETA8_APPWIDE_AUDIT.md and
docs/RELEASE144_HANDOFF.md.
Details: RELEASE_NOTES_1.17.144.txt.
Previous release descriptions follow.

NETWORK TOOLKIT 1.17.143
======================

CURRENT RELEASE - 1.17.143 (BETA 7)

This opt-in beta improves synchronous scrolling while comparing wrapped
source files in narrow Editor windows. It uses visible matching source
rows to adjust small native layout differences, while keeping all
source bytes, independent scroll behavior, credentials and history intact.

The included Editor browser regression checks wait for requested source
lines to be rendered before inspecting native colors and geometry. The
original position and color assertions are not relaxed.

Published as opt-in GitHub prerelease October 31, 2026. Stable remains
1.17.136. The release build and publication passed; source alignment and
divider integration checks passed, but the narrow continuous-map motion CI
retains intermittent 3-5px offsets against the existing 2px expectation.
Beta 7 is not a stable promotion; physical Windows endurance and live
network-device validation remain necessary.

Details: RELEASE_NOTES_1.17.143.txt and docs/RELEASE143_HANDOFF.md.
Previous release descriptions follow.

NETWORK TOOLKIT 1.17.142
======================

CURRENT RELEASE - 1.17.142 (BETA 6)

This beta focuses on Editor comparison line-number alignment in wrapped
text and source-free visual gaps. The native gutter includes blank rows
for those gaps without changing source text, saved files or undo history.

Published as an opt-in prerelease October 31, 2026. Latest stable:
1.17.136. Installation requires explicit approval. Existing credentials,
permissions, diagnostics, terminal sessions and the portable executable name
are preserved. Narrow Editor motion CI failures remain tracked.

Details: RELEASE_NOTES_1.17.142.txt and docs/RELEASE142_HANDOFF.md.
Previous release descriptions follow.

NETWORK TOOLKIT 1.17.141
======================

CURRENT RELEASE - 1.17.141 (BETA 5)

This beta improves the Editor Files/Connections controls, more precise
comparison-map scrolling, and TFTP/SFTP file explorer divider interaction.
The file explorer supports keyboard resizing and reset in addition to
dragging. Cancellation restores the previous divider position.

Published as a prerelease on October 31, 2026. Enable Beta updates
in App settings > Updates to receive this build. Previous beta: 1.17.140;
latest stable: 1.17.136. Credentials, network permissions, saved settings
and the stable portable executable name remain unchanged.

Details: RELEASE_NOTES_1.17.141.txt and docs/RELEASE141_HANDOFF.md.
Previous release descriptions follow.

NETWORK TOOLKIT 1.17.140
======================

CURRENT RELEASE - 1.17.140 (BETA 4)

Beta 4 improves performance beyond Terminal. SFTP, FTP and server file
browsers avoid sorting unchanged directories repeatedly during selection
and navigation. Time-range graphs calculate the first and last recorded
samples without sorting the entire history; already ordered data retains
the existing display order.

Published as an opt-in beta on October 31, 2026. Enable Beta updates
under App settings > Updates to receive v1.17.140. This retains existing
credentials, settings, network diagnostics, transfer behavior and the
Windows portable executable name. Previous beta: v1.17.139; latest
stable: v1.17.136. Live Windows performance measurements remain pending.

Details: RELEASE_NOTES_1.17.140.txt and docs/RELEASE140_HANDOFF.md.
Previous release descriptions follow.

NETWORK TOOLKIT 1.17.139
======================

CURRENT RELEASE - 1.17.139 (BETA 3)

Beta 3 reduces terminal display overhead during high-volume output by
combining output-only event bursts per session into bounded parser writes.
Original byte ordering and per-event history and macro capture remain
unchanged. Previously buffered output also uses batched display writes
when opening a terminal view.

Connection status and authentication prompts stay immediate. The
existing output-loss warning, 1 MiB pending-render cap, input pacing,
session isolation, credentials and opt-in Beta updates are preserved.

Published as a prerelease on October 31, 2026. Enable Beta updates
under App settings > Updates to receive it through the opt-in channel.
No physical network-device throughput benchmark has been run.
Previous beta: v1.17.138; latest stable: v1.17.136.

Details: RELEASE_NOTES_1.17.139.txt and docs/RELEASE139_HANDOFF.md.
Prior release descriptions follow.

NETWORK TOOLKIT 1.17.138
======================

CURRENT RELEASE - 1.17.138 (BETA 2)

Beta 2 improves terminal pane resizing. Rapid splitter pointer movements
are frame-coalesced, changing window/sidebar geometry is respected during
dragging, and the final pointer position is applied when released.
A cancelled drag restores its previous ratio, including pointer-capture
loss. Oversized terminal canvas content is clipped to the assigned pane
while xterm refits.

When scrolling is paused, a saved viewport row is used as a bounded
fallback if the xterm scrollback marker is discarded during reflow.
A user's later scroll/Follower Latest choice still takes priority.
Existing SSH/Telnet/Serial/local shell, terminal output, selection, input,
prompt pacing, remote PTY resize and saved settings behavior is retained.

This is an opt-in prerelease. Enable Beta updates under App settings >
Updates and check for updates from v1.17.137 or the stable v1.17.136 bridge.
The latest normal stable release remains v1.17.136.

Details: docs/releases/RELEASE_NOTES_1.17.138.txt and docs/RELEASE138_HANDOFF.md.
Prior release descriptions follow.

NETWORK TOOLKIT 1.17.137
======================

CURRENT RELEASE - 1.17.137 (BETA 1)

The first Phase 1 responsiveness beta reduces redundant layout work
during rapid terminal pane resizing. It skips repeated stage measurements
while every visible pane already has a scheduled fit, avoids unnecessary
highlight refreshes when terminal dimensions have not changed, and defers
cosmetic chrome redraws while resizing in real browser windows.
Terminal cell fitting, remote PTY resize, prompt pacing, output and
input guards remain in place.

This release is an opt-in GitHub prerelease. Install stable 1.17.136 first,
then enable Beta updates in App settings > Updates and check/download.
A later stable version must be newer than this beta to switch back without
downgrading. Current beta release details: RELEASE_NOTES_1.17.137.txt.
Previous stable releases follow.

NETWORK TOOLKIT 1.17.136
=====================

CURRENT RELEASE - 1.17.136

App settings > Updates now offers Beta updates (opt in). It is off by
default, so existing users continue receiving only stable releases.
When enabled, Check for updates searches both stable and official GitHub
prereleases. Every downloaded package is verified and only installed after
the existing Install and restart confirmation. Turning the option off
returns to stable-only checks; installed betas are not downgraded and will
receive the next newer stable release.

This stable update is the bridge from v1.17.135 to Phase 1 beta builds.
Details: RELEASE_NOTES_1.17.136.txt and docs/RELEASE136_HANDOFF.md.
Previous published releases follow.

NETWORK TOOLKIT 1.17.135
=====================

CURRENT RELEASE - 1.17.135

Copy and Select from Last paste or Last command now list each submitted SSH
paced-paste command once, with no artificial "Command N:" heading.
With Include commands off, command lines and recognized initial echoes
are excluded while all captured device responses remain available.
The change does not affect command execution, pacing or captured history.

Details: RELEASE_NOTES_1.17.135.txt and docs/RELEASE135_HANDOFF.md.
Previous published releases are retained below.

NETWORK TOOLKIT 1.17.134
=====================

CURRENT RELEASE - 1.17.134

SSH one-line-at-a-time pastes now populate Last paste and Last command with
accepted commands and their output. The console's Copy/Select actions and
the macro library's Last paste/Last command controls use this history.
Stopped, interrupted or rejected runs do not include unsent commands.

Details: RELEASE_NOTES_1.17.134.txt and docs/RELEASE134_HANDOFF.md.
Previous published releases are retained below.

NETWORK TOOLKIT 1.17.133
======================

CURRENT RELEASE - 1.17.133

Disconnect and Reconnect now appear side by side in the Terminal active
commands. Each has an arrow for Disconnect all or Reconnect all.
The confirmations for Reconnect all, Disconnect all and Close all tabs are
independent, optional and enabled by default. Turn them back on in the
Terminal display settings, or turn them off from their individual prompts.

Details: RELEASE_NOTES_1.17.133.txt and docs/RELEASE133_HANDOFF.md.
Previous published releases are retained below.

NETWORK TOOLKIT 1.17.132
======================

CURRENT RELEASE - 1.17.132

Reconnect one terminal as before, or use the new Reconnect all action from
a tab's right-click menu or the small arrow beside the main Reconnect button.
The new action confirms before restarting eligible sessions in this window.
The updater now shows practical improvements under What's New, with technical
information available under Additional details.

Release: https://github.com/TechSavage1989/network-toolkit/releases/tag/v1.17.132
Details: RELEASE_NOTES_1.17.132.txt and docs/RELEASE132_HANDOFF.md.

PREVIOUS PUBLISHED RELEASE - 1.17.131

NETWORK TOOLKIT 1.17.131
======================

CURRENT RELEASE - 1.17.131

Ctrl+A selects the current recognized, echoed terminal command without
including its prompt or prior output, even when the command wraps. Unsafe,
uncertain, password or full-screen input keeps the original device control key.
Multi-terminal performance changes coalesce cosmetic toolbar/status redraws,
avoid hidden-tab highlighting work, skip unchanged full chrome redraws and update
splitter CSS during dragging. Existing session transports, secrets, settings
and portable updater remain unchanged. Actual user-device speed is unmeasured.

Release details: RELEASE_NOTES_1.17.131.txt and docs/RELEASE131_HANDOFF.md.

PUBLISHED130 CLOSURE (retained)

NETWORK TOOLKIT 1.17.130
======================

CURRENT RELEASE - 1.17.130
Published 2026-10-31T03:13:29Z; actual public downloads/source/updater feed verified
03:14:21Z and independently 03:15:04Z. Blank profile names remain automatic:
manual override first when explicitly assigned, otherwise learned name then
IP/configured host or COM endpoint. SSH/Telnet/Serial/CMD/PowerShell badges show
type across tabs, Saved and Recent. New-Recent revision races are guarded.
All 14 accepted PR checks passed; previous failed observations are retained.
Narrow sidebar rows can require horizontal scrolling. No live-device/full restart
claim. Final main duplicate checks passed all 14 jobs at 03:31 UTC; earlier failed
observations remain documented in RELEASE130_HANDOFF.md.
Release: https://github.com/TechSavage1989/network-toolkit/releases/tag/v1.17.130

PUBLISHED129 CLOSURE (retained)

NETWORK TOOLKIT 1.17.129
======================

CURRENT RELEASE - 1.17.129
Published 2026-10-31T01:59:27Z; actual public downloads/source/updater feed verified
02:01:35Z, independently 02:02:26Z. Includes bounded terminal pane/PTY resize
scheduling plus manual-name precedence, explicit clear/revision handling and
bounded CLI-prompt recognition. Saved connection addresses/credentials unchanged.
Version 128 is subsumed and was not separately published/tagged. Full accepted
PR checks passed; duplicate run outcomes and runtime limits remain documented.
No live SSH or universal remote-program compatibility claim.
Release: https://github.com/TechSavage1989/network-toolkit/releases/tag/v1.17.129
Post-merge duplicate checks completed02:15:36Z:11 passed/1 retained legacy editor-
motion failure. See docs/RELEASE129_HANDOFF.md; no unrelated fix is claimed.

HISTORICAL128 CANDIDATE RECORD (subsumed by published129, not separately released)

NETWORK TOOLKIT 1.17.128
======================

CURRENT RELEASE - 1.17.128 (candidate) (not yet published)
Manual names take precedence over recognized device names, with endpoint below.
Documented numeric/network/shell prompt support is expanded. Ambiguous custom
prompts use a known name or endpoint fallback; command readiness is unchanged.
Validation pending. See docs/RELEASE128_HANDOFF.md and RELEASE_NOTES_1.17.128.txt.

PUBLISHED127 RECORD (retained)

NETWORK TOOLKIT 1.17.127
======================

CURRENT RELEASE - 1.17.127

Release details: RELEASE_NOTES_1.17.127.txt
Adds explicit prompt-paced multiline paste with literal indentation/blank lines,
Pause/Stop and uncertain-start duplicate protection. Tabs, Saved and Recent show
a learned CLI name over a smaller endpoint, or only the endpoint when unknown.
Saved names, addresses and credentials remain unchanged. Raw paste is available.
Published2026-10-30T23:12:03Z; actual public downloads, checksums, portable source,
binary-only updater and latest feed verified23:13:27Z. Exact-source browser/native
Windows/package validation passed. No live SSH device was tested; retained
failures and duplicate-run status are in docs/RELEASE127_HANDOFF.md.
Release: https://github.com/TechSavage1989/network-toolkit/releases/tag/v1.17.127

PREVIOUS RELEASE INFORMATION (retained)

NETWORK TOOLKIT 1.17.126
Portable network diagnostics and device terminals in a native app window

CURRENT RELEASE - 1.17.126

Release details: RELEASE_NOTES_1.17.126.txt
Previous release: docs/releases/RELEASE_NOTES_1.17.125.txt
Full release-note archive: docs/releases/README.md

126 repairs the read-only Windows administrative inventory: JSON and diagnostic
streams are separated and bounded cold-start time is allowed. Actual Windows
inventory, authenticated refresh and read-only action preflight passed, alongside
mocked action guards and synthetic UI checks. Unknown state remains blocked;
no live adapter changes or visible UAC transition were tested. Published 19:14:20Z,
public bytes/feed verified 19:15:13Z and independently 19:15:42Z. Prior 125 unchanged.
Read RELEASE126_HANDOFF.md for exact evidence and historical regression limits.

Known 125 post-release issue: unknown administrative state can leave both new
actions unavailable. Actual read-only query diagnosis and repair are tracked
separately in 126; mocked validation did not prove this discovery path.

125 adds explicit Enable adapter / Disable adapter to Network actions. Disabled
adapters remain selectable, administrative state is separate from link status,
and existing Restart / IP / DNS settings are preserved. Review the connection-loss
warning before disabling; normal Windows administrator approval is used if needed.
Published18:24:37Z; public downloads/source/feed verified18:25:29Z and independently
18:25:56Z. New UI/Windows mock/core/security/package checks passed. No live NIC
changes or visible UAC were tested. All four public files and prior124 immutability
verified. Read RELEASE125_HANDOFF.md for exact original/duplicate CI outcomes and
unchanged historical editor resize/comparison/other limitations.

124 corrects mature Condensed Recent graph endpoints and adds reviewed DHCP,
DNS, adapter restart and Winsock maintenance actions. TCP Ports has a distinct
inspection icon. Network actions are validated with isolated mocks/UI, not live
network changes; review each action's scope and disruption warning before use.
Published 16:50:53Z; actual public bytes/source/feed verified 16:51:36Z and
independently 16:51:49Z. Graph 48-frame/legacy, network simulated actions/errors,
icon/full/collapsed/narrow, core/security/Windows/package acceptance passed.
Four public files and prior 123 immutability verified. See RELEASE124_HANDOFF.md
for exact closure: review all seven passed; duplicate six passed/one comparison
motion failure. Historical limitations remain unresolved.

123 gives Terminal SFTP rows the same direct native Windows-copy gesture as
the SFTP tool and Editor. Detached workspace browsing/export is scoped to its
own existing session; unsupported remote-write controls are not offered.
Published13:39:16Z; actual public bytes/source/feed verified13:41:28.278568Z.
Eight real main/workspace Terminal cases and ten existing SFTP/Editor cases passed;
full Windows/core/security/dependency/package and prior122 invariance verified.
Review41063349840: all seven SUCCESS; last job completed 2026-10-30T13:46:46Z. Duplicate41064715467:
six SUCCESS/one FAILURE (known Windows Alt-editor transfer); last job completed 2026-10-30T13:56:30Z. Existing Alt-editor transfer and historical comparison/
Hosts/gear-keyboard/UAC limits are not fixed. See RELEASE123_HANDOFF.md.

122 improves ordinary native local/authorized-SFTP file and folder drag-out.
The exact active gesture transfers capture safely before copy admission; late or
cancelled gestures do not revive. Source files, dirty text/undo, Windows conflict
handling and existing approval/limits remain protected. No Prepare dialog is
required for the supported native workflow introduced in121.
Published 12:27:30Z; actual public bytes/source/feed verified 12:30:55.984763Z.
Forty repeated and ten final-version real Windows scenarios passed. Review
41021649359 ended five SUCCESS/two FAILURE: Alt-editor tab transfer and comparison
gutter alignment remain documented, not fixed by this release. Release/package
verification passed; prior121 remains unchanged. Duplicate main41055236523:
five SUCCESS/two FAILURE
(Windows Alt-editor transfer and comparison motion), last job completed
2026-10-30T12:44:55Z. See RELEASE122_HANDOFF.md for exact scope and limitations.

121 compacts the Editor Files sidebar and improves its file-drag workflows.
See the current release notes for supported behavior and validation limits.

120 restores file/folder drag starts by keeping the editor-tab drag listener
out of other tools' drag events. Transfer and document safeguards are unchanged.
Condensed Recent scaling preserves the baseline recent spacing and curve for
long intervals; short populated intervals expand only as needed to fill the plot.

120 published 2026-10-30T04:07:44Z; actual downloads/source/feed verified
04:09:16.444676Z. Source/tag45ee25b15c1c88fdb7c77b2f06057d38ae6b5357.
Actual file-row baseline/candidate and four graph layouts, core/security/dependency,
full Windows and package checks passed. Review 40999859524: all seven SUCCESS, last job completed 04:12:48Z.
Duplicate 41000889198: six SUCCESS/one FAILURE (Windows Alt-editor transfer), last job completed 04:24:00Z.
The duplicate native Alt-transfer failure remains unresolved; historical comparison,
Hosts and visible-UAC limits are not fixed by this focused update. Outward Windows
Explorer copies still use explicit preparation; this restores ordinary application
drag starts without adding a new transfer capability. Prior 119 assets are unchanged.
See RELEASE120_HANDOFF.md for exact evidence, public hashes and limitations.

119 corrects selected Condensed time windows, preserves Details visibility on
Follow latest, and keeps the right anchor when only the history start is resized.
Total elapsed uses HH:MM:SS; clock defaults are 12-hour unless explicitly saved.
Detached Editor local files are available without a main-window approval step;
remote connection and credential permissions remain separately protected.

119 published 2026-10-30T03:21:18Z; actual public downloads/source/feed verified
03:22:27.122867Z. Source/tag a4b8c56f4c8692d4676d508381ec4c5a2b504aee.
Four focused graph and four detached-local browser layouts, actual native
no-main-GUI local read/lease-save, core/security/dependency/package passed.
Final review: six SUCCESS/one FAILURE (known narrow wrapped-map alignment).
Duplicate main: five SUCCESS/one FAILURE (native Alt-editor transfer)/one CANCELLED (long browser), last job completed 03:38:53Z.
No all-seven-green or comparison repair claim. Prior118 remains unchanged.
A user-reported file-row drag defect was reproduced after publication: the
unchanged editor-tab handler cancels non-tab dragstart. It is OPEN in119 and
tracked for separate120 repair; explicit staged Explorer copying is distinct.
Read RELEASE119_HANDOFF.md for exact results and security/native limits.

118 places Editor Connect above Files and gives existing Connect sidebars one
dedicated connection icon. Selected panels, handlers and security remain unchanged.
Historical validation caveats below remain unresolved; this is not a related fix.

Published 2026-10-30T00:56:56Z; four actual downloads/source/feed verified
01:08:14.010Z. Source/tag 2be7a62d18865275bd323c26b07f1520b689b560. All 14 focused
rendered cases and exact-source core/full-Windows/package checks passed.
First review: five SUCCESS/two FAILURE (native Alt-transfer and resize keyboard).
Same-source package review: all seven SUCCESS; earlier failures remain unresolved.
Duplicate main: six SUCCESS/one FAILURE (native Alt-editor transfer), last job completed 01:11:35Z. See RELEASE118_HANDOFF.md for honest
run-by-run outcomes, hashes and native/UAC limits. Prior 117 assets are unchanged.

117 published 2026-10-29T23:33:10Z; public assets/source/feed verified 23:35:30.794Z.
Source/tag 51aa026f07e201979b3adb44d048c66c06508826. New feature/native/core checks
passed. Normal review finished five SUCCESS/two FAILURE (existing comparison
alignment and Hosts-entry focus); publication retains both disclosed caveats,
not an all-seven-green or fixed claim. Actual high-token helper proof did not
exercise visible UAC consent/cross-account elevation. See RELEASE117_HANDOFF.md.
Duplicate main Checks 40973657061 ended five SUCCESS/two FAILURE at 23:48:49Z
(Windows Alt-editor transfer and comparison motion); Hosts/long browser passed.
No comparison/Hosts fix or all-seven-green result is claimed.

New117 uses the shared vertical Files/Connections rail and scoped drive-wheel
scrolling. CMD/PowerShell starting directories and explicit per-session
administrator launches retain normal Windows consent and actual-token badges.
See EDITOR_SIDEBAR117.md and TERMINAL_LOCAL117.md for behavior and limits.

Previous116 is a Terminal-only interaction hotfix: ordinary tab selection and close
clicks work without losing drag reorder/detach or existing close safeguards.

Published2026-10-29T21:17:18Z, sourcef13c4df906a0ed8124308f8f95db571a7a9be532.
Focused actual Terminal and required core checks passed under the expedited
validation plan for the approved publication; public assets/source/updater/feed were verified. Unrelated
background comparison tests found intermittent narrow detached wrapped-map
misalignment on unchanged115 code. No data mismatch was observed in those tests,
but the cause is unresolved and this hotfix does not fix it. Duplicate native
editor validation also failed. Review and duplicate each ended six SUCCESS/one
FAILURE; see RELEASE116_HANDOFF.md for exact outcomes.
Do not interpret publication as all seven background checks passing.
The deferred115 handoff is included in this116 documentation-only closure.

Previous115: named drive buttons and a persistent Files/Connections sidebar; compact
comparison Options, optional horizontal sync and difference gutter symbols;
independent changed-text emphasis; reversible wheel zoom with safe data/time
limits; intentional pointer drag-away editor detaching. See EDITOR_FILES115.md,
EDITOR_COMPARE115.md, EDITOR_DRAG115.md and GRAPH_ZOOM115.md for behavior.
Exact release verification and publication status are recorded in the handoff.

Previous114 consolidates editor Files controls, expands bounded local/SFTP editing,
adds scoped native Explorer copies and Terminal tab actions, and improves
comparison and graph navigation. See EDITOR_CAPACITY114.md, FILES_NATIVE114.md,
TERMINAL_NATIVE114.md and GRAPH_NAVIGATION114.md for explicit limits.

Published114 source/tagfd6cbbf287738320b51957d176dd2e63c63eece9 is verified.
All seven exact-source review checks and automatic Release passed. Public assets,
source/package identity, updater allowlist and latest feed are verified.
Read docs/RELEASE114_HANDOFF.md for evidence, explicit limits and duplicate-main closure.

Historical published113 source/tag31f27994b9a3f7bae4442c255f63a65c8e21e0c8 is verified.
Read docs/RELEASE113_HANDOFF.md for exact gates, public hashes and limitations.
This later documentation record does not rebuild or replace published downloads.

New113: compact More/Sessions, saved file connections through existing approval,
tab Compare to, resizable editable aligned comparisons, configurable graph
compression, loaded-editor activity and default-on configurable close prompts.
Ordinary Details/results are concise while useful copy/export actions remain.
Turning prompts off does not bypass failed saves, active saves or ownership.

The editor has independent left/right tab groups and a normal split view. Tabs
sit above their own text pane. Files opens a resizable Windows/SFTP browser.
From119, a detached editor accesses local Files directly while its owner backend
remains alive. Only remote SFTP explorer access requires main-window approval;
existing approved transports are shared without saved credentials.

Comparison uses separate Left and Right maps, visible Fit/zoom levels and
wheel/click/drag navigation. Compact controls reclaim document space; Options
holds comparison settings and More holds secondary editing actions.

Editor tabs can be reordered with pointer dragging or Alt+Shift+Arrow keys.
A deliberate captured-pointer drag away and release outside the window detaches;
Escape cancels. Alt+drag or Move / merge transfers tabs between editor windows.
Explicit Detach editor/Open separately remains available. Cross-window moves
keep one acknowledged editable owner, and unsupported file bindings remain
safely in their original window.
Detached editors fill their own window, including maximize/restore. Graph point
outlines are stable through background refreshes, including stopped scans.

The redundant top bar is removed: computer identity is in the sidebar brand,
and Theme/App settings are in its footer. Text Editor uses compact Move / merge
access instead of a permanent destination row. More and comparison Options stay
compact in their original toolbars; only their contents expand below. Local
open/create and saved/active connection actions live in the Files/Connections
sidebar without redundant toolbar disclosures. Collapsing it returns space to
the actual editing and comparison surfaces.

Copy server/client file locations in explicitly labeled network, remote-relative
or local-disk formats; listener addresses and endpoints are copyable without
starting a transfer. Wildcard listeners require a concrete interface choice,
not an assertion that every address is remotely reachable. Copy values never
include passwords, saved credentials or authentication tokens.
Macros keep the selected Library/Runs tab when starting. Recording Pause and
Stop & review stay actionable during input/status refresh, and review failures
retain captured steps for retry. No actual user device testing is claimed.

SSH login banners with bare line feeds are aligned without changing normal PTY
output, terminal controls or paging. Discovery filters accept lists separated by
newlines, spaces or commas; quote multi-word phrases to match them together.
Complete IP/MAC entries match exact normalized addresses, while ordinary text
matches searchable device details. Existing status/type controls remain combined.
Click an open TCP port to prepare the appropriate existing connection tool or
open the browser/RDP route; unknown services require an explicit tool choice.
Port mappings are hints, not proof of a detected service. No port action runs
merely because scan results arrive. Existing connection security checks remain.

Set up the master-password credential vault explicitly in the native Windows dialog.
Remembered unlock is on by default and survives app restarts and Windows reboots
using a CurrentUser DPAPI-protected random unlock key, not a saved password.
Same-user software can access remembered unlock. Optional offline recovery must
be stored separately. Configuration reset forgets unlock, not encrypted vaults;
Lock and forget preserves established connections but blocks new credential use.
Legacy DPAPI copies remain until separately confirmed cleanup. Read
docs/CREDENTIAL_SECURITY.md for migration, backup and security limits.

Automatic terminal paging and Ctrl+C copy-selection are defaults only for new or
unset settings; existing explicit choices remain. Full paged output belongs to
its originating command/paste. See docs/TERMINAL_PAGING.md.
The supplied Toolkit SVG is the canonical source for current and future icons.
Condensed All-mode newest spacing is calibrated to Linear 30 seconds and remains
fixed as older history accumulates; explicit time ranges still control geometry.

Condensed RTT uses a fixed recent-time scale from startup and one accumulating
left history cluster. Sparse startup leaves empty past space; no Fit warm-up
rescaling. RTT traces follow the current accent. See docs/CONDENSED_GRAPH_SCALING.md.

Local Network Status and Type stay beside the search filter while results scroll.
The pinned controls wrap on narrow windows without hiding table headings.

Graphs offer Linear and Condensed, with Timeframe, zoom and navigation controlling
the range. Old mode preferences migrate to Linear. Run details are in entry details.
Condensed spacing is smoothly nonlinear even for short Fit ranges; endpoints have
safe gutters.

Fit selects All retained history. The compact navigator beneath the main plot
supports proportional time-window dragging and independent boundary resizing.

Graphs offer condensed history, keeping recent results individual and grouping
older retained history into wider intervals. Time-range controls support saved
durations, chosen end times and explicit start/end intervals.
Graph points and result rows share a held selection in the entry details pane.
Follow latest releases the hold and resumes linked following without resetting filters.

Scan results show actual recorded local times where relevant. Interactive graphs
show timestamped measurements and additional statistics grounded in the data.

The main navigation divider follows the shared sidebar style. Tool icons retain
their position between expanded and collapsed navigation. Possible networks
grows to show the full generated list in the subnet calculator.

Editor actions preserve selection and scroll position. Wrapped text retains
syntax colors and aligned line numbers. Editor close, reload, Save As and Go to
line use built-in dialogs. Saves preserve edits made while a write is pending;
newly saved files reload through the actual local file reader.

PowerShell and Command Prompt color recognized output headings, fields, values,
addresses and status words. Output colors have separate role controls in Display
and follow the terminal palette. Program-provided ANSI colors remain intact.

PREVIOUS RELEASE - 1.17.85

Local PowerShell and Command Prompt have explicit syntax-color controls.
Closing active terminal tabs and sessions has a saved confirmation preference.
Open Terminal, Display for separate PowerShell and Command Prompt switches,
syntax color previews and per-role colors, plus the close confirmation checkbox.
Local command coloring works independently of Highlight device output. Default
colors follow the terminal palette and preserve native shell colors; custom role
colors override matching command tokens. Ordinary output keeps its own format.

Collapsed main navigation icons select tools without opening the sidebar. Click
the arrow or double-click the divider to expand it; Enter and Space also work on
the focused divider. The compact-screen drawer keeps its existing behavior.

PREVIOUS RELEASE - 1.17.84

The editor detects file languages and supplies offline grammar-based syntax
colors, explicit parser formatting for web/data formats and safe code indentation
for PowerShell and C-family languages. Manual language selection, comment
syntax and selection indentation follow the current document language.

Local PowerShell and Command Prompt commands use the configured terminal ANSI
palette, preserving shell-provided colors. Fit console is removed from the UI;
console dimensions continue to update automatically.

PREVIOUS RELEASE - 1.17.83

Macro folders now use the saved-session folder controls: All folders, Unfiled,
choose or create a folder, and Move to folder. Commands, shortcuts and automatic
bindings stay with the macro.

Start local Command Prompt and Windows PowerShell sessions in the terminal on
Windows 10 version 1809 / Server 2019 or later. They share terminal theme, font,
cursor, ANSI colors, saved profiles, macros, split panes and linked workspaces.
They run as your current Windows user, starting in your home folder.

Scan columns fit the displayed contents with compact spacing. Copy actions keep
reserved space beside values. Discovery shows open TCP ports as compact badges
with conventional service labels. Independent discovery checks overlap with a
64-operation ceiling; reachability appears before optional device details.
Windows reverse lookups respect OS DNS policy and bounded caller timeouts.

Update downloads show bytes and percentage when the server provides a total,
and an activity bar when it does not. Verification has its own phase. The new
installer helper keeps a native activity window visible through replacement and
restart until the interface is ready. The first update from an older build uses
that older build's helper. Installation still requires your explicit choice.

PREVIOUS RELEASE - 1.17.82

Terminal selections can include spaces before the command as well as trailing
empty cells. Backspace, Delete, typing and paste edit the selected command text.
Prompt spacing is preserved; actual spaces typed into the command are editable.
Wrapped selections work with both leading and trailing whitespace.

PREVIOUS RELEASE - 1.17.81

Use a recognizable gear for settings throughout the app. In the terminal,
Backspace and Delete remove selected command text even when the highlight
includes empty cells after it. Typing and paste can replace the same selection.
Trailing spaces in the current input remain part of the edit. Prompts and
previous output remain outside selection editing.

PREVIOUS RELEASE - 1.17.80

Clean up the Overview page by removing repeated tool shortcuts and per-address
Tools buttons. Copy controls consistently sit beside displayed values, including
adapter IP and MAC addresses. Hosts editor More aligns with the toolbar and its
menu supports Escape. Repeated copy notifications share one message. Existing
navigation, adapter actions and diagnostics remain.

PREVIOUS RELEASE - 1.17.79

Remove the divider below the target entry row on the Overview page for a
cleaner appearance. Target entry, source interface selection and run controls
keep their existing behavior.

PREVIOUS RELEASE - 1.17.78

Detached terminal sessions and workspaces retain the sidebar side chosen in
the main window. Detaching immediately after moving the sidebar also uses the
latest selection.

PREVIOUS RELEASE - 1.17.77

The first sidebar tab is now labelled Connect in both client and server modes
for SFTP, TFTP and FTP/FTPS. The matched tab appearance and existing connection,
settings and transfer behavior are retained.

PREVIOUS RELEASE - 1.17.76

The selected Settings tab in SFTP, TFTP and FTP/FTPS server modes now uses the
same appearance as client mode, including highlight, spacing, hover and keyboard
focus. Existing server settings, collapse/expand actions and resizing are retained.

PREVIOUS RELEASE - 1.17.75

Text files open with the start of the document visible. Opening or reloading
a file resets the caret and both scroll positions, including desktop imports
and files opened through connected sessions. Background reads leave the
currently active document view in place. File contents remain unchanged.

PREVIOUS RELEASE - 1.17.74

Terminal connection setup returns to the original centered window. Choose SSH,
Telnet or Serial from a new tab, or New in Saved. The Setup sidebar tab and
Connection settings panel are removed. Saved profiles, folders, credentials,
macros and existing terminal sessions are retained.

PREVIOUS RELEASE - 1.17.73

The terminal command input no longer shows horizontal or vertical scrollbar
tracks, including expanded multiline input. Typing, caret movement, keyboard
and wheel scrolling, manual resizing and command execution are retained.

PREVIOUS RELEASE - 1.17.72

Saved sessions now support folders, like the macro library. Choose or create a
folder while saving or editing a terminal profile, or use Move to folder on
an existing terminal or FTP/FTPS session. Filter by All folders, Unfiled or a
named folder and search by folder name. Assignments survive app restarts.

PREVIOUS RELEASE - 1.17.71

SMTP passwords stay filled after successful and failed tests for repeat runs.
Switching to another tool clears the field. Credentials are not saved in
application settings or diagnostic history.

PREVIOUS RELEASE - 1.17.70

App settings now labels its update section Network Toolkit updates. This small
visible change lets you confirm the 1.17.69-to-1.17.70 update after restart.
The single final Install and restart flow from 1.17.69 is retained.

PREVIOUS RELEASE - 1.17.69

Removes the redundant Review install and restart action. A verified download
opens the final Install and restart prompt directly. The staged-update banner
and App settings also offer that same final install action in one click.
Downloading still requires approval and does not start installation by itself.

PREVIOUS RELEASE - 1.17.68

Fixes clipped text in the Hosts entry-status dropdown. Compact Hosts search and
find/replace fields also have enough vertical text space. Every status label,
including With validation issues, fits without shrinking the selector.

PREVIOUS RELEASE - 1.17.67

The Hosts file editor now has Entries and Text views, add/edit forms, per-entry
and bulk enable/disable/delete, filters, duplicate/conflict validation, undo/redo,
find/replace, line navigation, draft previews and paginated large-file viewing.
Import UTF-8, UTF-16 or Windows-1252 text, including a hosts backup, into the
draft. Choose replace or append and review before saving. Export the draft or
loaded copy as UTF-8 text. Automatic local-engine backups retain the original
file bytes when saving; original encoding and newline handling are preserved.

Drafts and undo history are kept only in memory. Entry actions change the draft;
Review and save uses the existing revision-checked, administrator-approved save
path. Importing or exporting does not itself replace the Windows hosts file.

PREVIOUS RELEASE - 1.17.66

Checks for new releases every five minutes while the app is visible and online,
and when you return after at least two minutes. Service limits may delay checks.
The install review is shorter, with clear notes and a Save your work reminder.
Nothing downloads or installs without your approval.

PREVIOUS RELEASE - 1.17.65

Fixes native browser timer binding for automatic update checks and their
cancellation. Saved preferences, release notes and explicit update approval
remain unchanged.

PREVIOUS RELEASE - 1.17.64

Checks for updates immediately at launch and offers an automatic main-window
banner. Review a simple What's new card before downloading and installing.
Explicit disabled checks and 24-hour reminders are respected.

PREVIOUS RELEASE - 1.17.63 (REVISED UPDATE FEED)

REVISED 1.17.63 UPDATE DESTINATION

Updates now use https://github.com/TechSavage1989/network-toolkit exclusively.
Releases retain the standalone EXE and complete-source Portable.zip, and add
strict four-member binary-only Update.zip with a combined SHA256SUMS manifest.
The old feed offers this revised 1.17.63 once for installed 1.17.62 clients;
subsequent update checks from this build use the original main repository.
An already-installed original 1.17.63 cannot detect a same-version replacement:
close the app and manually replace its EXE with this revised build, keeping
its folder, launch options and settings/data folder unchanged. See UPDATING.txt.
All server adapter selector improvements below are retained.

- SFTP, FTP/FTPS and TFTP servers use the same compact dropdown control as
  client connection sources for Listen on adapter.
- Adapter and listening port stay aligned on one row. Opening adapter choices
  does not expand the settings sidebar or reduce file and activity space.
- Mouse and keyboard selection, saved listening addresses, complete option
  labels and selected-address tooltips use the native dropdown behavior.
- Network controls remain disabled while the server is running. Existing
  loopback, all-adapter and individual IPv4 listening choices are retained.
- The main public update feed, stable executable name, settings retention
  and confirmation-based download/install workflow are retained.

RELEASE HISTORY

NETWORK TOOLKIT 1.17.62
Portable network diagnostics and device terminals in a native app window

PREVIOUS RELEASE - 1.17.62

- Update staging, helpers and rollback storage now use the resolved settings
  location, including custom data folders. Only the immediately previous
  version is retained after a successful update.
- Official names migrate to stable Network_Toolkit.exe, without new persistent
  sidecar files. Verified personal Desktop/Start Menu shortcuts are retargeted;
  pinned/shared/other-location shortcuts may need manual retargeting. Custom
  names and unrelated files are unchanged. Original shortcut backups use settings.
- Startup checks no longer wait for unrelated system/jobs discovery; saved
  preferences load first, early transient errors retry sooner, and automatic
  attempt/result/next-check details are visible in settings.
- Legacy updater folders migrate after confirmed success and helper exit.
  Unrelated files and unverified/manual copies are preserved.
- Cross-drive settings use short-lived same-volume replacement files, with
  checksum verification and resumable backup rotation. Locked cleanup is
  reported, never forced. First migration safely finishes reopening before
  allowing new work, preserving elevated-tool identity checks.
- The successful 1.17.60 visible restart fix, settings and profiles are retained.
- Windows binary and native tests were cross-compiled; live Windows/WebView2
  updater behavior could not be executed here. Read UPDATING.txt and VALIDATION.txt.

NETWORK TOOLKIT 1.17.57
Portable network diagnostics and device terminals in a native app window

PREVIOUS RELEASE - 1.17.57

- Shared compact window margins, full usable width and consistent panel gaps.
- SFTP, FTP/FTPS and TFTP sidebars can swap sides and retain their docking.
- Terminal Open SFTP opens Client mode on the active SSH connection.
- SFTP/FTP session bars share Terminal's styling and Top/Bottom preference.
- Run Network_Toolkit_1.17.57_Windows_x64.exe. Complete editable source is
  in source. Open_Terminal.cmd opens the terminal window.
- See docs/releases/RELEASE_NOTES_1.17.57.txt and VALIDATION.txt for details.

RELEASE HISTORY

NETWORK TOOLKIT 1.17.56
Portable network diagnostics and device terminals in a native app window

PREVIOUS RELEASE - 1.17.56

- Standalone SFTP now keeps one authenticated SSH connection open, using
  Terminal's shared SSH transport and SFTP operation code.
- Eliminates repeated SSH connections/logins when browsing and opening files.
- Added active-session Disconnect, Reconnect, Open terminal, Refresh files
  and Connection settings controls outside the sidebar.
- Open terminal uses the same SSH connection. Borrowed Terminal sessions can
  be detached from the browser without disconnecting Terminal.
- Run Network_Toolkit_1.17.56_Windows_x64.exe. Complete editable source is
  in source. Open_Terminal.cmd opens the terminal window.
- See docs/releases/RELEASE_NOTES_1.17.56.txt and VALIDATION.txt for details.

RELEASE HISTORY

NETWORK TOOLKIT 1.17.55
Portable network diagnostics and device terminals in a native app window

PREVIOUS RELEASE - 1.17.55

- Setup is the first Terminal and file-transfer client sidebar tab.
- Click the selected tab to close/open its sidebar; select another tab to
  open it directly. Separate visibility buttons have been removed.
- Terminal Setup includes its existing connection/profile/credential form.
- SFTP and FTP/FTPS include the same Save / update credential workflow.
- Credential updates keep profile links and survive application restarts.
- Run Network_Toolkit_1.17.55_Windows_x64.exe. Complete editable source is
  in source. Open_Terminal.cmd opens the terminal window.
- See docs/releases/RELEASE_NOTES_1.17.55.txt and VALIDATION.txt for details.

RELEASE HISTORY

NETWORK TOOLKIT 1.17.54
Portable network diagnostics and device terminals in a native app window

PREVIOUS RELEASE - 1.17.54

- Faster SFTP text-file opening with one verified connection per read.
- Immediate editor loading feedback and fewer duplicate folder requests.
- SFTP, FTP/FTPS and TFTP client activity has the server-style draggable
  divider, keyboard controls and separately saved height preferences.
- Client connections use a collapsible, resizable terminal-style sidebar
  with Saved, Recent and Setup tabs.
- Run Network_Toolkit_1.17.54_Windows_x64.exe. It works without the source
  directory. Open_Terminal.cmd opens its terminal window.
- Complete editable source and Windows build scripts are in source.
- See docs/releases/RELEASE_NOTES_1.17.54.txt and VALIDATION.txt for details.

RELEASE HISTORY

NETWORK TOOLKIT 1.17.53
Portable network diagnostics and device terminals in a native app window

PREVIOUS RELEASE - 1.17.53

WHAT IS NEW IN 1.17.53 - RESIZABLE ACTIVITY AND SERVER ADAPTER FIXES

- Larger SFTP, TFTP and FTP/FTPS server activity panels with a draggable
  horizontal divider above Live server activity.
- Drag upward to enlarge activity, downward to enlarge Shared files.
  Double-click the divider to reset its size. Arrow keys also resize it.
- Activity and sidebar sizes are retained separately for each protocol.
- Long server adapter options wrap fully without clipping or overlap.
- The adapter remains on the left and the listening port on the right.
- The working saved SSH/SFTP connection behavior from 1.17.52 is retained.
- See docs/releases/RELEASE_NOTES_1.17.53.txt and VALIDATION.txt for details.

PREVIOUS RELEASE - 1.17.52

WHAT IS NEW IN 1.17.52 - COLLAPSIBLE SERVER SIDEBAR AND SHARED SSH POLICY

- SFTP, TFTP and FTP/FTPS server configuration moved to a collapsible,
  resizable side panel with a saved position and compact adapter/port row.
- Server file managers and transfer activity logs remain in the main area.
- Saved SSH profile connections in SFTP use the same SSH algorithm policy
  as Terminal, avoiding false SSH host-key changes due to different server
  key algorithms. No host-key trust safeguards were bypassed.
- See docs/releases/RELEASE_NOTES_1.17.52.txt for details and testing limitations.

PREVIOUS RELEASE - 1.17.51

WHAT IS NEW IN 1.17.51 - COMPACT SERVER CONTROLS, SHARED SESSIONS

- Adapter selector LEFT and compact listening port RIGHT on one responsive row.
- SFTP and FTP/FTPS now have searchable Saved and Recent session panels.
- Saved SSH profiles are shared with Terminal; SFTP reuses matching connected
  SSH sessions and prioritizes Terminal's trusted host-key records.
- FTP/FTPS saved/recent sessions can also be opened from Terminal's library.
- Home and Root controls for SFTP/FTP file browsers and terminal SFTP sidebar.
- See docs/releases/RELEASE_NOTES_1.17.51.txt for details and validation limits.

PREVIOUS RELEASE - 1.17.50

WHAT IS NEW IN 1.17.50 - CLIENT PROFILES, FILE BROWSING, AND TERMINAL COLORS

- SFTP Client: Saved SSH profiles, active terminal connections, recent SSH
  terminal connections and recent SFTP connections in one source picker.
  Recent SFTP connections remain visible after refreshing the terminal list.
- SSH host key changes are blocked until the NEW fingerprint is independently
  verified and entered exactly. A confirmed key is saved only for the matching
  SFTP endpoint. An unexpected fingerprint is a security warning and must
  not be accepted without independent verification.
- TFTP Client: Reusable named saved and recent transfer configurations, with
  target, direction, paths, ports and retry options. TFTP has no authenticated
  persistent sessions; entries are configurations, not connected sessions.
- Text Editor > Browse / Open remote: browse directories on active SSH, SFTP,
  FTP and FTPS connections, and double-click a file to open it.
- Double-clicking a file or folder in the main SFTP/FTP browser and the
  terminal SFTP browser now works reliably. Selection no longer replaces file
  rows before the native double-click event can be delivered.
- TFTP transfer direction has room for full selection labels.
- SFTP and TFTP Server: Listening port is immediately to the left of the
  network adapter selection, instead of occupying a separate vertical row.
- Reduced automatic Copy icons on irrelevant UI elements and file manager
  contents. Copy actions remain available for useful diagnostic data.
- Previously selected terminal presets (including MobaXterm import, pastel,
  Nord, Catppuccin, Gruvbox and other presets) now survive settings validation
  and reloading. The Files sidebar tab is retained as well.
- Saved SFTP/TFTP metadata is validated on import and save. Passwords,
  passphrases and private keys are never stored in connection history.

PREVIOUS RELEASE - 1.17.49 - SAVED SSH PROFILE CONNECT FIX

- Fixed SFTP Client > Connection source > Saved SSH profile > Use connection.
  The selected profile now initiates a complete SFTP connection instead of
  stopping after populating hidden connection settings.
- Profiles with linked protected credentials connect automatically when the
  SSH host key is trusted. Profiles without a credential show a password or
  private-key sign-in dialog; credentials stay in memory only.
- New or unrecognized SSH host keys require independently verified SHA-256
  fingerprints before connecting. A mismatch blocks the connection.
- Password and private-key inputs are retained in memory for retry after
  transient failures. Changing to another profile, manual mode or an active
  terminal discards those transient credentials.
- Fixed the Use connection button remaining disabled after a connection.
  Active terminal session SFTP reuse remains available and unchanged.
- Keeps connection settings collapsed after successful saved-profile sign-in.
- See docs/releases/RELEASE_NOTES_1.17.49.txt and VALIDATION.txt for test coverage.

PREVIOUS RELEASE - 1.17.48

WHAT IS NEW IN 1.17.48 - LOCAL NETWORK LAYOUT, TRANSFER SETTINGS, TEXT EDITOR

- The Public IP card is shown only under Tools > Local network > IP Configuration.
  It no longer appears on the Overview or other Local network tabs.
- Corrected FTP/FTPS client Connection settings toggle. Its initial button label
  and expanded/collapsed state now agree, including for manual connections.
- Renamed the left navigation group LOCAL TOOLS to TOOLS. Text Editor is a
  dedicated navigation item under Tools, not another Local network tab.
  Older saved states that selected the Local network editor tab migrate to
  the new Text Editor page.
- Text Editor can now select a connected SSH terminal, the SFTP file manager,
  or the FTP/FTPS file manager and directly open an absolute remote text-file
  path. The SFTP/FTP/FTPS toolbar and terminal SFTP sidebar can open the
  standalone editor with the appropriate active connection preselected.
- The existing local Windows file support, multi-tab editor, language/syntax
  options, and file revision checks remain available.
- Added distinct icons for Text Editor, IP Configuration, Routes, ARP and
  Connections, and corrected fallback icons used by certain context actions.
  Separate FTP and SFTP controls now have matching transfer-protocol icons.
- See docs/releases/RELEASE_NOTES_1.17.48.txt and VALIDATION.txt for implementation details,
  test coverage and limitations.

PREVIOUS RELEASE - 1.17.47

WHAT IS NEW IN 1.17.47 - THEMES, FILE-TRANSFER PROFILES AND LOCAL TEXT EDITOR

- Terminal text color themes include an imported preset matching the supplied
  MobaXterm .mxtcolors file, additional dark/light and pastel presets, and
  editable hexadecimal ANSI colors. The original .mxtcolors file is included
  in the source/themes folder. App accent and diagnostic-result color controls
  also offer presets, swatches and hex values.
- File-transfer navigation groups FTP/FTPS under SFTP rather than treating
  FTP as a separate workspace tool. FTP/FTPS connection profiles can be
  created, saved, loaded, renamed and deleted; profiles may link credentials
  in the existing protected Windows credential vault, rather than recording
  passwords in application preferences. The SFTP page continues to share
  active terminal connections and saved SSH profiles.
- The standalone Text Editor is under LOCAL TOOLS, not Workspace. It also
  opens text files from the full SFTP/FTP file manager and the SSH terminal
  sidebar browser, and can edit Windows files with supported writable handles
  or local paths. Explorer drag-in imports desktop files into document tabs.
- Text Editor features include multi-tab editing, syntax highlighting across
  common programming, shell, web, data and network configuration formats,
  manually selectable language, line numbers, search/replace with regex,
  go-to-line, undo/redo, line operations, case conversion, JSON formatting,
  line-ending selection, indentation settings, word wrap, font-size controls
  and customizable editor color themes and individual syntax colors.
- UTF-8 and BOM-marked UTF-16 LE/BE text files can be read and saved while
  preserving encoding, with an 8 MiB text-file limit. File revision checks
  warn before overwriting content changed by another process. Binary or
  unsupported encodings are not edited as text; the editor does not autosave.
- Added OS drag-out integration for ONE regular file at a time from the full
  file browser's local/remote panes and the terminal's SFTP sidebar. Drag-in
  from Windows remains available. Drag-out uses short-lived, single-use
  download tickets; native Windows Explorer operation must still be confirmed
  on a physical Windows system. Use the Download action or local file pane
  as fallback. In-app file transfers, including folders, remain supported.
- See docs/releases/RELEASE_NOTES_1.17.47.txt and VALIDATION.txt for implementation details,
  test coverage, and environment limitations.

PREVIOUS RELEASE - 1.17.46
WHAT IS NEW IN 1.17.46 - FILE BROWSER, EDITOR AND SIDEBAR LAYOUT

- Combined saved SSH terminal profiles and active authenticated SSH sessions
  in the SFTP connection selector; detailed login settings can be collapsed.
- Reduced the SFTP connection panel footprint to prioritize file browsing.
- Added persistent left/right terminal sidebar docking with SFTP below Macros.
- Added in-app drag/drop, file-management context menus and a remote text
  editor with guarded saves. This previous version did not support native
  Explorer drag-out and limited editor files to 2 MiB.

PREVIOUS RELEASE - 1.17.45
WHAT IS NEW IN 1.17.45 - SFTP CLIENT INTEGRATION AND TERMINAL FILE SIDEBAR

- Removed the separate SSH Browser navigation item. SFTP > Client hosts the
  complete two-pane SSH/SFTP file browser. Legacy navigation is redirected.
- Existing saved SSH terminal profiles are available from SFTP Client.
  Their host/port/username are reused, and stored Windows credentials or
  private keys can be used without copying them into browser preferences.
- The terminal workspace sidebar is now on the right. After SSH connects,
  a session-scoped SFTP tab appears automatically if subsystem negotiation
  succeeds. Non-SFTP-capable SSH sessions do not display the tab.
- The right-side SFTP tab supports remote folder listing, navigation,
  upload, download, rename, create folder, delete and preview. Expand
  opens the full SFTP Client browser and keeps the terminal connected.
- The read-only capability check does not enumerate remote files; listing
  starts when the SFTP tab is opened. The terminal shell is never replaced.
- Sidebar controls, sizing, and keyboard resize direction now fit the
  right-side layout. Long filenames wrap inside their available width.
- Release details and testing are in docs/releases/RELEASE_NOTES_1.17.45.txt.

PREVIOUS RELEASE - 1.17.44
WHAT IS NEW IN 1.17.44 - SINGLE ADAPTER PICKER AND SHARED SSH BROWSING

- Removed the duplicate server adapter display. The selector now shows one
  full-width, wrapping description and IPv4 address, with a wrapping list
  of adapters when expanded. Applies to SFTP, TFTP, and FTP server settings.
- SSH File Browser can reuse an already-connected SSH Terminal session
  without asking for login credentials or establishing a second SSH TCP
  connection. From Terminal Tools choose Browse SFTP, or select a session
  from the SSH File Browser.
- SFTP operations use separate channels over the live SSH connection;
  browsing and transfers do not interrupt the shell. Closing the browser
  does not disconnect the terminal. Disconnected SSH sessions cannot
  continue file operations.
- Terminal ownership and window state are verified for each file operation;
  detached terminal sessions are not offered to the main window browser.
- Retained the 1.17.43 fixes for Start server confirmation, resolved-IP
  diagnostics, and responsive controls.

PREVIOUS RELEASE - 1.17.43
WHAT IS NEW IN 1.17.43 - FTP/FTPS AND REMOTE FILE MANAGEMENT

- Added FTP / FTPS to Workspace. Server opens by default, and listeners
  remain OFF until manually started. FTP Server supports password-based
  logins, plain FTP and explicit FTPS (TLS), passive and active connections,
  binary upload/download, remote directories and file operations.
- Added FTP Client with plain FTP, explicit FTPS and implicit FTPS, passive
  transfers, per-server TLS validation or verified SHA-256 certificate pin,
  transfer operations and remote file management.
- Added SSH Browser to Workspace: a WinSCP-inspired local/remote two-pane
  file explorer using authenticated SFTP over SSH with mandatory host-key
  fingerprint verification before file operations, password or key login,
  remote/local navigation, back/forward/up, address fields, sorting,
  filtering, multi-select, drag/drop transfers, recursive folder copies,
  create, rename/move, confirmed delete, and text preview.
- Retained SFTP Server/Client, TFTP Server/Client and their functional file
  transfers. SFTP Client now opens the SSH-based two-pane file browser;
  TFTP Client remains a simple send/receive interface because the TFTP
  protocol cannot enumerate remote directories.
- Simplified the SFTP, TFTP and FTP Server interfaces. Removed the redundant
  root-path display from the Client/Server selector and removed the large
  dual-pane local explorer from server mode. Each server displays one clear
  root path, a compact shared-files list, folder actions, and Open in
  Windows Explorer for advanced management.
- Each server has a live activity/output viewer, showing connections,
  logins (where supported), file transfers, failure details, and controls
  to refresh, copy, and clear the activity output.
- Corrected the server Start/Stop button state after transitions, corrected
  remote-browser connection controls clipping at shorter window heights,
  and added Reset layout to the remote file browser.
- Passwords and passphrases remain transient and are not saved into app
  settings or diagnostic job history. FTP/FTPS certificates and SSH server
  keys are retained locally for consistent server identity.

IMPORTANT PROTOCOL AND SECURITY LIMITS

- FTP without TLS is unencrypted, including the login password. Prefer
  FTPS or SFTP whenever the remote equipment supports it.
- This FTP server supports explicit FTPS. Implicit FTPS is supported by the
  client. The self-signed FTPS server certificate should be verified and
  pinned by the connecting client before use.
- FTP/FTPS passive transfers require firewall access to the dynamically
  allocated data ports in addition to the chosen FTP listening port.
- SSH Browser uses the remote SFTP subsystem; it is not an SSH shell,
  and servers without SFTP support cannot provide file browsing.
- TFTP has no authentication or encryption and supports no remote directory
  listings. Use only on trusted networks and controlled interfaces.
- File transfers are bounded and sequential; interrupted-transfer resume,
  background synchronization and unrestricted recursion are not promised.
- The GUI is built for Windows x64. Cross-platform validation cannot prove
  Windows Firewall or individual device interoperability.

PREVIOUS RELEASE - 1.17.40
WHAT IS NEW IN 1.17.40 - SERVER STARTUP AND CONTEXT

- TFTP Start server no longer uses the browser's native confirm() dialog,
  which is suppressed in Network Toolkit's Windows WebView host. It now opens
  a compatible in-app security confirmation before network-visible listening.
- SFTP Start server validates required username and password/authorized keys,
  visibly explains missing settings, and displays server start/bind errors in
  the server workspace instead of relying on brief toast notifications.
- Both tools show the selected server root folder, listening address/port,
  configured remote access permissions, active sessions/transfers, and service
  state even when Server settings is collapsed. The header also shows the
  selected server root in Client mode.
- Server settings opens expanded by default and stays in the user's selected
  expanded/collapsed state across folder browsing, page redraws, status
  refreshes, mode switches, server start and stop. Only clicking the Server
  settings summary toggles the section during routine interaction.
- Settings remain viewable after server startup, with network/configuration
  controls locked while running. The local dual-pane file explorer remains
  available. Session passwords survive transient UI rerenders in memory but
  are never saved and are cleared after successful server start.
- Backend integration tests verify real loopback TFTP and SFTP Start/Stop
  operations through the authenticated API, with secrets excluded from status.

PREVIOUS RELEASE - 1.17.39
WHAT IS NEW IN 1.17.39 - DNS, RESPONSIVE LAYOUT, EXPLORER

- The SFTP/TFTP server root-folder picker can now create new folders in the
  selected location, browse drives and paths, filter folders, and select the
  newly created folder directly. The dual-pane explorer also has quick New
  folder and Refresh controls on both sides.
- All diagnostic tool pages include Reset layout to restore default panel
  position/sizes and responsive result-table widths; SFTP/TFTP also resets
  the dual-pane file explorer split.
- Result tables estimate useful column widths from visible data, distribute
  spare space at wider window sizes, and reflow when the workspace resizes.
  Manual resizing remains supported; horizontal scroll is retained if data
  is genuinely too wide to display without truncation.
- Connection Monitor's sample count, unlimited-run option, check method and
  TCP port now have a consistent vertical configuration layout.
- DNS Lookup uses Windows DnsQuery_W for system-resolver lookups, obtaining
  actual cached/resolver TTLs for supported record types. Direct wire DNS
  queries remain available and preserve TTLs; native Go fallback results
  explicitly indicate when TTL is unavailable.
- DNS record details now expose MX priority/server, SOA serial and timing,
  SRV priority/weight/port, CAA fields, and TXT SPF/DMARC purpose, using an
  expandable summary to keep the DNS table compact. The record details are
  also retained in raw results and exports.
- Copy controls are restricted to actual values, pinned to the first line,
  and do not duplicate buttons or appear below a wrapped value.

PREVIOUS RELEASE - 1.17.38
WHAT IS NEW IN 1.17.38 - EXPANDED FILE EXPLORER AND WORKSPACE CONTROLS

SFTP and TFTP now live under Workspace and open with Server mode selected.
They have distinct navigation icons; SMTP Test uses a mail icon. The new
file explorer offers dual panes and local/server file management, described
below. No file server is automatically started.

PREVIOUS 1.17.37 UPDATE - SFTP AND TFTP SERVER MODES

Both transfer tools gained Client/Server selectors in 1.17.37. The selector
remains visible regardless of the selected tool panel.

TFTP SERVER:
- Start and stop an actual TFTP listener (UDP 69 by default), bound to a
  chosen local interface. Network switches, routers and firewalls can initiate
  uploads of configurations or downloads of firmware/configuration files.
- Read only, write only, or read/write access, overwrite protection, a chosen
  root folder, per-transfer UDP sockets, negotiated block size and timeout,
  retries, and successful/failed transfer history with remote addresses.
- TFTP is not authenticated or encrypted. The server is OFF at startup. If
  network-facing, explicitly review Windows Defender Firewall inbound rules
  for UDP 69 and the dynamically allocated UDP transfer ports.

SFTP SERVER:
- Start and stop an authenticated SFTP v3-over-SSH server (TCP 2222 by default)
  on a chosen local interface. Only the SFTP subsystem is available: shell,
  terminal, and arbitrary command execution requests are rejected.
- Set a username and a session password and/or authorized SSH public keys.
  The application creates a persistent Ed25519 host key in its data folder
  and displays the SHA-256 host-key fingerprint for verifying SFTP clients.
  Passwords are held in memory only and are not saved to application settings.
- The configured root is the remote SFTP root. Remote users can browse, upload,
  download, create folders, rename/move and remove files as permitted. Symbolic
  link traversal and access outside the root are blocked.
- Read-only, write-only and read/write modes are available. Overwrite is off
  by default. The listening service is OFF at startup and stops when the
  Network Toolkit app closes.

FILE EXPLORER (NEW IN 1.17.38):
- Server mode is the default view for SFTP and TFTP. Servers remain STOPPED
  until you explicitly select Start server. Client mode remains available.
- Both tools are now under Workspace and have distinct navigation icons.
  SMTP Test uses an envelope icon in Diagnostics.
- Server settings now include an interface/IP dropdown with actual adapter
  names and assigned IPv4 addresses, plus Loopback and All interfaces.
- A WinSCP-inspired two-pane explorer places This computer on the left and
  the configured server's shared folder on the right. This is a local file
  manager; the left pane is not an unauthenticated remote client connection.
- Independent folder address bars, Back/Forward/Up/Root navigation, column
  sorting, text filtering, visible/hidden toggles, multi-select (Ctrl/Shift),
  select all, keyboard shortcuts, and a resizable divider are supported.
- Copy files and folders between the computer and server root using Upload /
  Download or drag a selected file between panes. Multiple selected items are
  processed sequentially with a stop-pending control and failure reporting.
- Both panes support create folder, copy, rename/move, and deletion with
  confirmation. Server-side operations remain restricted to the server root;
  filesystem/drive roots cannot be deleted using the local pane.
- Select a text file for a read-only preview up to 128 KiB; copy file/folder
  paths to clipboard. Folder copies are recursive (10,000 entry / 64 level
  safety limit), skip symlinks, and require overwrite permission to merge.
- The file explorer works without starting either listening service.
- All controls use the app's existing styles and persistent preferences. The
  Server settings section can collapse, giving more space to the files.
- TFTP cannot browse remote device folders. The server pane represents the
  local TFTP root directory. It does not imply remote directory-listing support.
- Background concurrent transfers, resume, bidirectional synchronization,
  integrated text editing and full WinSCP feature parity are not provided.

LIMITS: SFTP v3 subset, individual files (maximum 4 GiB), 16 concurrent remote
sessions per server, and max 10,000 files per listed directory. TFTP does not
support authenticated access or remote directory browsing. Windows Firewall
and other endpoint security settings can block inbound traffic.

PREVIOUS UPDATE - 1.17.36 - SFTP AND TFTP CLIENT TOOLS


Two file-transfer client tools were introduced in Diagnostics, alongside SMTP
Test introduced in version 1.17.35. Each tool has its own configuration,
recorded runs, cancellation, transfer progress, and structured results.
Use Browse local files to choose a file for upload or a destination for
download directly from the application window.

SFTP uses SSH and the SFTP version 3 subsystem (default TCP port 22). Actions
include inspecting the server host key, browsing remote directories, getting
file information, uploading/downloading individual files, creating folders,
renaming/moving remote paths, deleting files, and deleting empty folders.
Authenticate with an SSH password or a local SSH private key, with an optional
passphrase. Before the first authenticated operation, inspect the fingerprint
and compare it with a trusted reference, then supply that SHA-256 fingerprint
as an explicit host-key pin. Connections with unexpected host keys are
rejected. Passwords and key passphrases are not saved to settings or job
history. They must be reentered when performing subsequent operations.
Destructive SFTP delete actions require a separate confirmation checkbox.
Uploads and downloads do not overwrite existing files unless Overwrite is
selected. File transfers run as individual jobs, not a persistent interactive
SFTP shell; folder synchronization and interrupted-transfer resume are not
implemented.

TFTP supports binary (octet) upload/download (default UDP port 69), timeout
and retry controls, and block-size and file-size option negotiation when the
server supports them. It falls back to the standard 512-byte block size on
servers that ignore options. Transfer activity and negotiation details are
shown in the run results. Note that TFTP has no encryption, authentication,
or remote directory listing. Use it only on appropriate trusted networks.

The SMTP Test tool is retained: greeting and EHLO capabilities, connection,
TLS or STARTTLS, optional authentication, and an explicit opt-in test message.
Sending mail remains disabled unless selected. SMTP credentials are excluded
from stored results and UI settings.

PREVIOUS UPDATES - LIVE PACKET CAPTURE AND DIAGNOSTIC COLORS (1.17.34)


Packet capture now uses the same backend-assigned severity for each packet row,
the fine packet map, and the overview's aggregated alert coloring. The mixed
map mode prioritizes event severity with the user's selected red/yellow/blue
palette; Protocols-only mode continues to show protocol colors by design.

The full overview map and packet-level map have independent Show/Hide controls
under Colors & maps. Either map can be hidden on its own, automatically
reclaiming its width for the packet list. Settings persist between sessions.

Normal unfiltered packet-window requests now copy only the requested slice
rather than snapshot/filtering the entire capture. The full-capture event/map
review is loaded separately and is throttled during active capture, so slow
map analysis will not block visible packet rows. The Follow new packets option
now scrolls the actual packet list to the newest retained frames as they arrive,
including after each live capture refresh.

Ping, Connection monitor, and other applicable result tables now emphasize
negative and missing results using shared status colors. The Ping and monitor
response-time chart shades missing samples and draws distinct error markers.
Use the compact Status colors action on a diagnostic tool to customize Error /
Missing, Warning, Success, Information, and Latency line colors. Changes
persist and can be restored to default values. Other existing application
controls, capture filters, packet-inspection and reporting remain available.

WHAT IS NEW IN 1.17.33 - DIRECT MAP DRAGGING AND HIGH-CONTRAST COLORS

The secondary fine packet map now drags its visible-range indicator directly
with the pointer. At the default one-pixel-per-packet scale, moving the mouse
one pixel scrolls by one packet. At 2, 3 or 4 pixels per packet the pointer
remains synchronized to packet rows at the chosen scale. It no longer
recenters the map while dragging. Clicking a line still selects that frame.

The full-capture map now has its own "Full map pixels per packet" control.
Its default Fit mode keeps the entire capture visible in a small bar. Switch
to 1, 2, 3 or 4 pixels per packet to see a virtualized, scrollable full map
with a separate line for each frame. Clicking, scrolling and dragging track
individual frames in these modes without rebuilding the packet list.

Default expert event colors are pure red (#FF0000), yellow (#FFFF00), and
blue (#0000FF) for errors, warnings and notes. Previously saved untouched
1.17.32 event defaults are migrated, while distinct custom choices remain.
The new "Full strength" row mode colors the entire packet row at 100%
intensity with automatically contrasting text. Existing Off, Marker, Subtle
and Strong modes remain available. All preferences are saved automatically.
The Colors & maps popup overlays the workspace so it consumes no packet-list
height when closed.

WHAT IS NEW IN 1.17.32 - CUSTOM COLORS AND PACKET MAP SCALING

Packet Capture's "Colors & maps" control opens a compact floating settings panel
that does not use permanent space in the packet table. Choose colors for
individual protocols and for decoder error, warning, and note events. Packet
rows support no coloring, narrow markers, subtle full-row tinting, or a
stronger tint. Both maps can show protocols and alerts together, protocol
colors only, or highlight alerts against neutral traffic.

The scrollable detailed map now shows one CSS-pixel-tall line per displayed
packet by default. Optional line heights are 2, 3, and 4 pixels. Customize
the width of the full-capture overview map (8 to 24 px) and the packet-level
map (10 to 32 px) independently. Clicking, dragging, viewport indicators,
packet selection, and continuous virtualized scrolling follow the selected
scale. Colors and map settings are saved locally with the app preferences.
"Restore default colors & maps" resets the palette, widths, scale, and
highlighting modes. The packet list retains its full space while the
settings are closed.

WHAT IS NEW IN 1.17.29 - PACKET INSPECTION

Packet Capture uses a continuous, virtualized packet list rather than page
buttons. Scrolling retrieves bounded packet windows while preserving the full
filtered scrollbar range. Use Top, Latest, or click the narrow packet overview
map to navigate. The colored map is based on actual retained packet types and
conservative packet-level decoder clues.

The collapsible Interesting traffic sidebar groups decoder errors, warnings
and notable events (such as TCP resets or DNS failures), gives occurrence
counts and links directly to a representative packet. These are investigation
clues, not proof of a fault. The separate Troubleshooting view retains its
guided request/reply analysis and next steps.

Packet details now begin with a Frame layer, including local and UTC capture
timestamps, epoch time, time since first/previous frame, frame number, original
and captured lengths, interface, direction (if known), encapsulation, and
decoded protocol stack. The plain-language explanation remains under an
expandable section to leave more space for technical layers and bytes.

The Packet Capture command and tab controls have tighter spacing, the packet
list has fixed-width columns and compact rows, and the details pane continues
to support right/bottom docking, resizing and collapsing.

These features use Network Toolkit's built-in decoder; they do not import
Wireshark dissectors or provide full protocol support or decrypted payloads.


Overview information cards now use one consistent visual component style.
Computer, Active adapters, Local IPv4, Diagnostics, Default gateways,
Configured DNS servers and Public IP share matching card shells, icon
treatment, typography, spacing and responsive behavior while preserving
their existing data and controls.

The Macros Library More menu opens a floating list with Import macros and
Export library. Last command and Last paste remain separate buttons.

In Macros / Runs, use Clear beside a finished run or Clear all above the list.
Clearing removes the selected history and its retained output. Running and
paused runs remain available; stop them before clearing them. All retained
runs are listed, including older entries.

RETAINED DETAILS AND PACKET CAPTURE FEATURES (1.17.20)

Selected-entry details across the diagnostic tools now resize and collapse
like the other app panels. Drag the divider, double-click it to hide/restore,
or focus it and use the arrow keys, Home, Enter or Space. The leading arrow
and collapsed rail control the same panel. Its expanded width, bottom height,
visibility and docking preference are remembered. Auto docking moves details
below the results when space is limited.

Packet capture adds a visible DHCP / address assignment filter and guided
task. Practical filter hints and empty-view assistance help explain what to
look for. DHCPv4 decoded messages include plain-language context; DHCPv6 can
be located by its UDP ports and retains its available UDP fields and bytes.

Text controls wrap and grow to fit their labels in narrow panels, including
the Single target and Networks and hosts switch. Common DNS names and values
have more initial column space; long records still wrap and columns remain
resizable.
The terminal command bar also sizes Insert only and Run command to their
labels when the window is narrow, preserving space for the command input.

RETAINED INPUT AND VALUE ACTIONS (1.17.19)

The subnet calculator shows its binary subnet mask with the other details.
Editable fields share a consistent clear cross, including the subnet address
input. Read-only output and terminal emulator input retain their own behavior.
DNS result values and other relevant result/detail fields also offer the same
individual value-copy behavior without covering their text.
Hover or focus a value to reveal its copy icon. Relevant entry and packet
context menus also offer Copy value for the clicked cell. Editable inputs,
secret fields and the terminal console keep their own interaction behavior.

Clear crosses appear in filled editable fields and keep focus in the field.
Clearing removes the entered value without submitting a form or sending a
terminal command. Blank numeric fields remain blank until you enter a value.

Click the collapsed panel rail to reopen it. Hide/show controls use the same
arrow at the leading edge of panel headers, before the other panel actions.

Packet actions provide related-traffic filters and endpoint copying from the
selected packet or its right-click menu. Field values can also be copied
without losing byte-range highlighting. These actions use the retained
recording and do not change recording filters or discard captured packets.
An active-view status and Show all packets action make filtered views clear.

RETAINED PACKET INSPECTOR FIXES (1.17.18)

The packet capture Quick guide stays open across automatic refreshes. Ordinary
UDP packets now show their decoded headers, available payload and original
bytes even when a payload layer has no named fields.

Packet details now resize and collapse like the other app panels. Drag the
divider, double-click it to hide/restore, or use Enter/Space and arrow keys
with the divider focused. The shared arrow button hides/restores the pane.
Expanded dimensions and visibility are remembered separately from recording
settings. The pane moves below the packet list on narrow windows.

RETAINED GUIDED PACKET CAPTURE (1.17.17)

Packet capture now starts with a guided troubleshooting view. Choose a task
and adapter, start recording, reproduce the problem, then stop and review.
Plain-language findings show what was observed, why it may matter, practical
next checks and example packets. Copy or save the findings for follow-up.

The review examines all retained packets. It distinguishes DNS responses,
connection attempts and resets, repeated TCP data, ICMP delivery/path messages,
local address lookups, plaintext web errors and supported local drop evidence.
It explains missing traffic, capture limits and encrypted-content visibility.
An absence of supported warnings is not presented as a clean bill of health.

Recording filters and other technical options remain available under Advanced
capture settings. Suggested task views only filter the displayed packet list;
they do not discard recorded traffic. Selected packets now have a plain-language
explanation before their technical fields and original bytes. See
source/docs/PACKET_TROUBLESHOOTING.txt for the workflow and interpretation limits.

All sidebar show/hide buttons now use the same chevron without visible words.
Hover hints and accessibility labels still name the action and panel.

RETAINED USABILITY CHANGES (1.17.16)

The macro editor now shows Automatically continue at --More-- prompts beside
the commands, outside the collapsed timing settings. The run preview exposes
the same choice for that run. Auto scroll follows incoming output; automatic
paging sends continuation keys when the device pauses its output. Existing
macro paging choices and the terminal's separate settings are retained.
Confirm runs labels the destination/command confirmation setting explicitly.

The command bar identifies its purpose with Send to, a destination selector,
recipient names and a command entry. Run command executes the entered text;
Insert only types it without pressing Enter. The Command bar control hides or
restores this row, with multiline expansion available when needed.

The terminal sidebar now resizes and collapses like the other app panels.
Drag its divider to resize, or double-click it to hide/restore the panel.
Keyboard controls and the previous expanded width are retained. Hide/Show
controls use consistent chevrons and hover hints beside the panel they control.
Local network now uses a connected-network icon distinct from Device details.

RETAINED SCROLL AND CONFIRMATION FIXES (1.17.15)

Macro runs now apply the destination's current Auto scroll setting when they
start, including before a delayed first command. Enabled sessions follow the
bottom; paused sessions retain their viewing position. Manual wheel or
scrollbar use remains paused through later steps and paging. Terminal resizing
preserves the visible top line while paused and the bottom while following.

The Macros header has a visible Confirm runs setting in both Library and Runs:
On, Multiple terminals only, or Off. The run dialog also has a Show this
confirmation before running checkbox. Missing required variables still prompt.
Changing the checkbox is saved only after a successful Run request.

Removed the duplicate Sidebar and Macros toolbar buttons. Use the sidebar's
collapse control to hide it, then select a vertical tab to reopen it.

RETAINED COMPACT SIDEBAR (1.17.14)

Saved sessions, Recent sessions and Macros now share a vertical tab sidebar.
Only the selected pane is shown. Macro library and run views stay in the
sidebar, leaving console height available for output. Collapsing the sidebar
keeps its tab rail visible; selecting a tab reopens it. The chosen tab and
collapsed state are retained.

The Send to composer uses a compact row with expandable recipient details;
it wraps on narrow windows. It can expand for multiline commands or be hidden
using the Command bar control.
Recording and active runs have a compact activity button beside the terminal
controls, including while another sidebar tab is selected or collapsed.

Run confirmation can be set to On, Multiple terminals only, or Off.
Required variable values still open an input dialog when missing. Explicit
Preview and run remains available from a macro's right-click menu. Composer
visibility and the confirmation preference are saved with terminal display
preferences and retained when panes move between linked windows.

RETAINED MACRO DELIVERY FIX (1.17.13)

Fixed missing macro controls in version 1.17.12. The macro scripts and styles
were embedded but were not served by the application's asset handler. They
now load through the normal app URL. Open Terminal and choose the Macros
sidebar tab for New, Record, Save last command and Save last paste.
The Send to composer is directly below the terminal search field.

Terminal macros can be recorded, edited, organized, pinned, imported and
exported. Steps support commands, text, keys, prompt/text waits, delays and
review pauses. Variables prompt once and expand for each destination. Preview
shows the fixed destination sessions before running. Pause, Resume, Step,
Stop and Interrupt remain available while other terminal panes are active.

The visible Send to composer targets the current terminal, selected terminals
or all connected terminals in this workspace. Insert types one line without
Enter. Run command executes command lines with prompt waits and retains complete
per-device output. View output offers selection, copy and export independently
of visible scrollback. The main workspace can target its linked detached panes;
independent application processes have separate sessions and macro libraries.

Saved-session startup macros are optional, with separate first-connect and
reconnect choices. Runtime secret variables are not stored as defaults and
are masked in previews. Macro runs stop on disconnect or reconnect rather
than sending remaining steps to a replacement connection. See
source/docs/TERMINAL_MACROS.txt for usage, timing and retention details.

Double-click a configuration or device-details resize bar to collapse or
restore the panel. Enter and Space do the same when the bar has focus. The
previous panel dimensions are retained. Hide/show configuration is beside its
heading and remains accessible from the adjacent rail while collapsed.

Opening Packet Capture loads adapters automatically. Refresh adapters is
beside the selector. The Windows inventory now matches Packet Monitor sources
to real configurable network interfaces and shows interface names instead of
unassociated driver components. Ambiguous shared-driver sources are omitted
from individual live choices; available exact adapter matches remain usable
with compatibility file capture. Native Windows enumeration needs validation
on the destination computer.

RETAINED TERMINAL RENDERER CORRECTION (1.17.11)

The persistent plain block when scrolling has been reproduced and corrected
in the terminal renderer's decoration index. Marker disposal changed a row key
before queued deletion completed, leaving removed entries in the sorted index
and hiding valid colors. Removal now uses object identity and rejects duplicate
queued deletes, retaining the normal cleanup batches. No terminal text or
device attributes are rewritten.

The previous build fails the same direction-changing scroll sequence in an
actual browser, even after every repaint settles. This build passes that
sequence, further upward scrolling, mouse wheel, scrollbar dragging, continuing
output, refresh and highlight off/on checks. These are actual Linux Chromium
DOM/color checks; the native Windows host itself is not executed here.

Optional development verification uses node scripts/terminal-browser-scroll.cjs
with separately installed Playwright and a browser. CHROME_EXECUTABLE can select
an installed Chrome/Headless Shell. The script downloads nothing, exits nonzero
on missing token colors or index corruption, and retains screenshots/reports
only when an explicit --output-dir is provided.

Neutral ANSI foregrounds now receive automatic syntax colors. Explicit white,
gray and RGB normal-text formatting no longer leaves a permanent plain block
before a device pager's formatting reset. Distinct device colors, custom
chromatic palette slots, backgrounds, inverse and hidden text are preserved.
The actual bundled row renderer is checked across this reset during scrolling.

The terminal search field has a clear cross inside the entry. It clears the
query, match indicators and search selections in the terminal panes, then
keeps focus in the search entry. Match-case and search visibility are kept.

Windows IEEE HTTPS failures now include the secure-failure reason and flags,
distinguishing trust, hostname, date, revocation and channel errors. A confirmed
channel-only failure gets one fresh TLS 1.2 attempt with the same route and
remaining deadline. Certificate or unknown failures are not retried. This
build cannot verify the affected PC's network or certificate policy; the new
diagnosis is needed if the update still fails. The old database remains intact.

Manual scrolling now prepares every supported visible highlight before the
scroll repaint. Newly revealed rows above or below the previous view no longer
wait for the background scan timer. Rapid scrolls and output jumps use this
same path, while overlapping marks stay in place. Incoming-output scrolling
and scrollback trimming retain the batched path to avoid work per newline.

On Windows, a failed Go TLS compatibility retry now has a separate Windows
HTTPS fallback. It uses Windows networking and system/per-user proxy discovery
when no environment proxy is specified. An explicit supported environment
proxy or NO_PROXY route is respected. Certificate validation, official IEEE
HTTPS destinations, download-size limits and the shared deadline remain.
HTTP refusals and certificate errors do not trigger the fallback. Any failure
keeps the previous database; Windows failures identify the failing API/error.
No companion DLL, external downloader or PowerShell script is required.
See source/docs/VENDOR_DOWNLOADS.txt for the download path and verification limits.

New and rewritten terminal output starts highlighting in the parsed-write turn.
Changed highlight rows explicitly request a terminal repaint, so colors do not
wait for another command or a scroll away and back. Large screens continue in
bounded batches; even a slow frame advances at least one logical line.
Unchanged decorations remain in place and do not keep repainting the console.

Jump to output start/end selects the destination line as well as scrolling.
Wrapped lines are selected across all their display rows. Destinations use the
same command-echo preference and prompt boundaries as Select/Copy output.
When output has left scrollback, the retained-output preview selects its first
or last line and scrolls to that selection.

Vendor database downloads now retry TLS protocol/handshake failures once with
verified TLS 1.2 and HTTP/1.1. Both attempts share the existing 20-second file
deadline, official IEEE HTTPS host restrictions and proxy route. Certificate
errors and HTTP refusals are not retried. Failure messages explain the secure
connection problem and the browser-download/Import IEEE CSV alternative.
The previous database is preserved on disk and in memory if any registry fails.

Display sits immediately beside Tools. Find in terminal output is visible when
the terminal workspace opens, without taking keyboard focus. Its close control
and search shortcut remain available.

Terminal highlighting now has twelve named roles: addresses, MACs, interfaces,
commands, setting names, protocols/services, modes/named values, numbers/units,
comments/descriptions, success, warning and error. Config verbs and field/value
positions cover common FortiOS, Cisco, HPE/Aruba, Junos, PAN-OS and SonicOS forms.
Arbitrary setting names after set/unset are recognized without a fixed list.
Interface tables recognize named interfaces, one-space/tab-separated states
and an address joined directly to up/down. A zero address octet no longer
suppresses state colors. Dense screens can retain up to 8192 decorations.
The new roles have purpose labels and examples in Display and the live preview.
Existing custom colors and the optional underline preference are preserved.
Syntax colors do not add click-to-copy targets for every command/setting word.
See source/docs/TERMINAL_HIGHLIGHTING.txt for scope, examples and vendor references.

Select output and Copy output now retain complete parsed command output past
the former byte and row cutoffs. Completed rows are stored as text chunks while
the parser keeps a bounded editable tail. Both actions wait for output already
received at click time to finish parsing and keep the originally chosen block.
Wrapped Unicode text is preserved without introducing wide-character padding.

Terminal color controls now explain what they affect. Console colors cover
background, normal text, cursor and selected-text background. Automatic
highlights have separate named controls and examples for IP/CIDR, MAC address,
interface, up/success, warning/pending and error/down. Device colors live in a
collapsed advanced section. Those 16 ANSI slots are chosen by the device and
do not have fixed status meanings; light themes can use darker shades for
traditionally named white slots to keep output readable.

In Display, turn off Underline highlighted text for color-only automatic
highlights. Colors and recognition remain enabled. The option is saved and
follows terminal workspaces; existing settings default to keeping underlines.
Device-supplied colors and formatting remain intact. Changing only underline
visibility retains current highlight decorations without rebuilding them.

The live preview now uses the same automatic highlight palette and underline
setting as the terminal. It also shows a distinct device-color sample, selected
text and cursor. Color edits select Custom colors; choosing a preset restores
its console/device/highlight colors while retaining the underline preference.

Recent and Saved now have dedicated header rows. The collapse control and
count occupy the left column; Clear or New occupies its own right column.
Both actions are in normal layout flow, with no floating or absolute position.
List content is below the header, and collapsed sections retain their action.
The existing collapse preferences, context menus and removal behavior remain.

Output start/end buttons now show a block of text with its top/bottom boundary,
with matching output-start context-menu artwork. Console scroll controls keep
their simple up/down arrows, making the two navigation functions distinct.

Each terminal pane has its own Detach button in its header. It moves only
that connected session, including its retained display and command output,
into a separate linked window. Other sessions and pane positions stay intact.
The source pane offers Reattach for an individually detached session. Finish
a pending sign-in before detaching an individual pane. Pane detachment also
works inside a linked workspace; in that case, Reattach in the new session
window returns the session to the original home workspace. The remaining
linked workspace and its layout stay open. A failed window launch restores
the selected session to its source workspace.

Session entries retain their right-click and Shift+F10 action menus. The
per-entry settings button is removed. A remove cross appears when an entry
is hovered or keyboard-focused; it remains visible on touch devices. It
removes only that Recent or Saved entry, using the configured confirmation.

The Recent header now contains Clear, and the Saved header contains New.
Both actions stay available when their section is collapsed. Clear is disabled
when Recent is empty; the configured removal confirmation still applies.

The output controls now use a two-button Last command / Last paste switch.
Exactly one button stays selected. Select output, Copy output and output
start/end arrows use that selection for the focused session.

The application icon is now used inside the main navigation, including its
collapsed view, and in the header of independent and detached terminal
windows. The interface icon, native window/taskbar icon and startup splash
use the same artwork, compiled into the EXE without companion image files.

The exact editable Illustrator original is preserved in
assets/source/Network_Toolkit_Icon.original.svg. The application uses the derived
safe assets/Network_Toolkit_Icon.svg, without editor metadata, DTDs or external
resources. Every drawing element remains a vector shape. The matching Windows
ICO contains 16, 20, 24, 32, 40, 48, 64, 128 and 256 pixel sizes; a 256-pixel PNG
is also included. To update the artwork, edit the preserved original, run
python scripts/generate_icon_assets.py (Inkscape is needed for regeneration),
then python scripts/build_windows_resources.py and rebuild with BUILD_WINDOWS.cmd
or build.sh. The interface loads the SVG directly; Windows uses the ICO.

Active session commands now appear above the console by default. Use the
Position selector in that row to choose Top or Bottom. The selection is saved
and follows detached and independent terminal workspaces. Output selection,
Auto pager, Auto scroll and scroll arrows remain below the console.

Display now includes terminal appearance settings: installed font family,
font size, regular/bold weight, line spacing, an independent color scheme and
a custom palette. Presets include Network Toolkit Dark/Light, Dracula,
Solarized Dark/Light and Windows Campbell, plus Follow app theme. Customize
background, text, cursor, selection and all sixteen ANSI colors. Changes apply
to existing and new terminals and persist across restarts and window moves.
Fonts must be installed on the PC. Reset appearance restores its defaults.

Select a row in Connection Monitor, Ping, TCP ports, Trace route, DNS and
other diagnostic result tables to open an Entry details side panel. It shows
the recorded fields, run context, nested data and applicable device tools.
Copy details/JSON, Expand details, Close and Escape are available. The selected
record stays readable and keeps its scroll/expanded sections during live
refresh, even when that sample leaves the current list or activity window.
Smaller windows show the panel as a drawer. Existing Discovery device details
and detail dialogs within other dialogs retain their own presentation.

Ping and Connection Monitor support Run until stopped. Connection Monitor
supports both ICMP and TCP. Sampling runs until the requested count or Stop,
without the previous 1,000-sample and 2-hour sampling limits. Complete-run
statistics remain accurate while older individual samples roll out of bounded
memory. The final result retains the latest 10,000 sample rows; live activity
retains its separate 2,200-event window. The UI identifies a trimmed view.
Live HTML/text exports include lifetime statistics and retained observations.

Console scrolling uses vertical up/down arrows. Selected output navigation
uses text-block boundary icons for its start and end, including the paste-start
context-menu action.

A native splash screen now appears while the interface and saved workspace
load. It shows the app icon, version and an animated loading bar. The main
window appears when initialization finishes and restores its saved bounds
and maximized state. Cancel or Escape closes startup. Startup failures show
a readable error with a Close button instead of leaving the splash running.

The terminal has a collapsible Sessions sidebar with Recent and Saved lists,
search, and collapsible sections. Double-click an entry or press Enter to
open it. Recent entries select an existing live tab or reconnect its retained
disconnected tab. After that tab is closed, or after restarting the app, the
entry starts a new connection using the retained settings. Saved sessions
also reuse matching open tabs. Open in new tab always starts a separate
connection. Linked saved credentials are used when
available; otherwise SSH asks only for the missing password or private key.
Settings, including serial options, remain available through Edit connection.

Right-click an entry or use Shift+F10 for Open, Open in
new tab, Edit, Copy connection details and Remove. Saved entries also offer
Rename and Duplicate; recent entries offer Save connection. A terminal tab's
context menu also includes Save connection. Existing saved profiles appear in
Saved automatically. Up to 50 recent and 100 saved entries are supported.

Clear Recent removes the history list. Both lists support individual removal.
Confirmation is enabled by default. Use Confirm before removing sessions at
the bottom of the sidebar, or Do not ask again in a removal dialog, to choose
the behavior. Removing a list entry leaves open connections and their output
available. Recent settings and list preferences persist across app restarts;
passwords, private keys and transcripts are excluded from recent history.

Reconnect starts a fresh line after the previous input/output in the same
console. Earlier text remains intact. The reconnect notice, new prompt and
subsequent commands append in order instead of restoring an old saved cursor.
Reconnect also resumes Auto scroll and shows the latest prompt at the bottom.
This applies to active and disconnected sessions and restores the normal
screen correctly when reconnecting from a full-screen application.

Packet details now stay in place as the capture list refreshes. The details
actions stay visible above a separate scrolling area for fields and bytes.
Long values wrap, hex/ASCII uses compact eight-byte rows, and expanded sections
and scroll positions remain intact. In smaller windows, setup and filters
scroll independently so the packet list and details remain accessible.

Packet capture provides interface selection, start/stop, duration and retained
packet-data limits, circular retention, keep filters, search and display
filters, packet fields with hex/ASCII inspection, TCP/UDP conversation follow,
endpoints, protocol counts, traffic charts and expert notes. Import PCAP and
PCAPNG; export either capture format, CSV, JSON or text; save packet bytes.
Windows Save As preserves capture and packet-byte filename extensions.

Automatic recording uses the native Windows Packet Monitor stream API when
available. Administrator approval is requested for recording and Load adapters.
Compatibility mode uses Windows Packet Monitor files and displays packets
after Stop. Its recording file is always circular. The native API limits
captured bytes per packet to 9000. Keep filters apply to retained packets;
compatibility mode applies them after Stop. Existing capture sessions and
global Packet Monitor filters are not cleared by this tool.

The display-filter help lists the supported language and examples. Conversation
follow orders TCP payload, skips duplicate/overlapping bytes and marks gaps.
Encrypted payload is not decrypted. IP fragments and application messages are
not reassembled. Only traffic visible to the selected adapter can be captured.
Three captures are retained in memory; a fourth replaces the oldest completed
capture. Packet data is excluded from normal settings and diagnostic reports.
Export captures you need before clearing them or closing the application.

Terminal behavior:
- Highlighted text retains unchanged colors while output is parsed or redrawn.
- Wheel or scrollbar use pauses Auto scroll, including during Auto pager.
  Scroll to bottom and accepted terminal input, including Ctrl+C and Ctrl+Z,
  resume following. Queued output cannot restore an older viewport position.
- Long command output is retained separately from console scrollback. Start,
  end, select and copy remain available after the original rows are trimmed.
- Active-session detach sends the validated connection handoff schema and
  transfers retained command output without saved-credential references.
- Right-click a terminal tab, or use Shift+F10, for reconnect/disconnect,
  input/output tools, rename, duplicate settings, detach/reattach and close
  actions. Actions apply to the chosen tab, including inactive tabs.

Reconnect uses the existing session settings and original authentication in
the same tab, pane and window. It preserves the console output and does not
open the connection form or ask you to choose a profile again. A disconnected
console clearly shows Disconnected with Reconnect in the center, together
with SSH, Telnet and Serial buttons for new connections. View output reveals
the retained transcript. Connection options brings the centered actions back.
Single linked-session windows keep their existing connection scope.

Click within the visible command being entered to position its cursor.
Select input text and press Backspace or Delete to remove that selection,
or type or paste to replace it. Wrapped input and wide characters are
supported. Editing sends ordinary cursor and backspace keys to the device,
which must support command-line editing. It does not submit a command.
Output and prompt selections retain their existing copy behavior. Full-screen
applications and native mouse reporting retain their own mouse handling.

Ctrl+C and other console control keys are sent to the device. They do not
close the app or remove its terminal tab. If the device ends a shell or drops
the connection, the tab and output remain available for reconnecting.

The original connection request is held only in the terminal manager memory
for explicit reconnects. It is released when the tab is removed or the app
closes. Passwords and keys remain excluded from settings, events and reports.
SSH trust decisions and interactive authentication still follow the device
protocol. Input queued for an earlier connection is never replayed after a
reconnect.

SSH connection fields are ordered Username, Password, then Saved credential.
Authentication method, SSH compatibility and connection timeout follow below.

The connection dialog now contains a Saved connections selector. Selecting a
profile loads its protocol, host/port, serial settings and protected credential
link without connecting automatically. Save profile remains beside the name.

The compact console bar below the panes provides:
- Last command, Last paste, or an earlier retained output block.
- Jump to output start/end, Select output, Copy output, Include commands.
- Auto pager, pager key, Auto scroll and Review paste for the active session.
- Scroll to top and Scroll to bottom. Bottom resumes following new output.

The visible Active session row contains Disconnect, Reconnect, Ctrl+C, Ctrl+D,
Ctrl+Z, Clear scrollback, Export transcript and Fit console. Serial sessions
also show DTR, RTS, Send break, Local echo and Enter line endings directly.
Display opens display settings. Tools
retains credential/profile management, SSH trust management and default input
preferences. Copy line remains in the terminal context menu. Hovering a line
only highlights it; no Copy line button covers the output. Highlighted tokens
still copy with a click. Ctrl+Shift+C and normal selection remain available.

Auto pager, Review paste, Auto scroll and Include commands in the console bar
affect only the focused session. New sessions inherit input defaults from
Tools. Session overrides and output positions survive normal detach/reattach.

Output blocks are associated with the entered command or paste until the
next manual input begins. The app observes a terminal byte stream rather than
a structured command protocol. Asynchronous notifications may appear within a
block. Recognized echoed commands are excluded unless Include commands is on.
If removing interleaved paste commands would create a discontinuous console
selection, Select output opens a selected, plain-text output preview. Copy
output always uses the same filtered text. No text from that preview is sent
to the remote device.

Jumping or selecting earlier output pauses Auto scroll. Manual wheel and
scrollbar use do the same. Enable Auto scroll, use Bottom, or send terminal
input to follow again; automatic pager responses keep the paused position.
Each session keeps at most 50 block records. Complete parsed output is retained
as text chunks for those blocks while the view stays open; the parser keeps
only an editable tail and uses writes of at most 4096 bytes or UTF-16 units.
There is no fixed per-command byte or row cutoff for live selection/copy.
Archive memory grows with captured output. Clear scrollback removes it.
Window handoffs retain up to 4 MiB total archived command text per session
within the workspace transfer cap. Larger transfers keep a suffix ending at
the final captured output and explicitly mark that transferred archive partial.
Creating a shortened handoff does not change the complete source-view archive.
If the console rows are gone, start/end/select opens the retained text preview.
Command output and block positions remain transient and are not written into
ordinary app settings. Typed password keystrokes are never retained by the
block tracker. Copy gestures and automatic pager responses are not commands.

GET STARTED

1. Extract the ZIP into a normal folder or a USB drive. App data defaults
   to %LOCALAPPDATA%\NetworkToolkit, separate from the executable.
2. Double-click Network_Toolkit_1.17.29_Windows_x64.exe. Its interface opens
   in its own application window. No companion DLL is required.
3. Choose a tool and enter a target. For Discovery or the Networks and hosts mode
   in Ping or TCP ports, click local network cards or enter multiple IPv4
   networks, hosts and ranges. Click a selected network again to remove it.
   Open Add networks / review scopes to choose local or routed networks or
   review the selected list. Source interface optionally chooses the local
   adapter and source IP.
   Review the unique address and probe counts, then start the scan.
4. Use Reports to export results before closing the application.
5. Use Terminal for SSH, Telnet, or a serial console. Right-click a device
   row or use its Tools button to prepare a connection or run a diagnostic.
6. Choose Separate terminal window, or double-click Open_Terminal.cmd,
   to use the terminal in its own independent application window.
7. Use Detach in Terminal before connecting or with existing tabs.
   Reattach returns the same workspace, sessions, screens and pane layout.
   With multiple sessions or panes, the single Detach control offers the
   active session or the workspace. Choose a pane layout to display up to four
   sessions with resizable dividers.
   All pane layouts fit the available console area, including short windows.
   Maximize a pane for more working space, then restore the same layout.
   The compact command bar keeps common actions visible; Tools opens the
   remaining settings without reducing or rebuilding the console screens.
8. App settings lets you choose the data folder. SSH, Telnet and Serial use
   the integrated terminal. Closing the home window preserves its detached
   connections.

RUNTIME REQUIREMENTS

- Windows 10 or Windows 11, 64-bit x64.
- Microsoft Edge WebView2 Runtime must already be installed. This is the
  embedded web engine used by WebView2 desktop apps. The ordinary Edge
  browser and the WebView2 Runtime are distinct components.
- No installer and no additional Python, Node.js, Go, .NET, Nmap, Npcap,
  JavaScript packages, online fonts, or CDN assets.
- The HTML, CSS, JavaScript, and diagnostic engine are included in the EXE.
- The Go WebView2 loader is compiled into the EXE. The application does not
  require, extract, or look for WebView2Loader.dll beside the executable.
  It uses the existing Microsoft WebView2 Runtime registered with Windows.
  It does not install a runtime or download components when opened. If the
  Runtime is missing, the app displays an explanation.
- Normal use runs with your existing Windows account. An explicit Apply in
  IP configuration, Save in Hosts file, or administrator retry for a Wi-Fi
  key can request Windows approval. The same EXE handles that specific action.
  Read-only tools and scanning do not change the computer's configuration.
- Internet access is needed only when the selected target or lookup requires
  it. Local tools and local network checks work without internet access.

APP WINDOW

The interface is hosted in an embedded WebView2 control inside a native
Windows window. It has its own profile and does not open the normal Edge
browser or load the user's regular browser profile. It provides no browser
tabs, address bar, extension management, or regular Edge browser menus.

The host restricts navigation to the app's local address and handles new
windows, browser permission requests, developer tools, default context menus,
autofill/password-save prompts, and report downloads as an app. Report
downloads use a native Windows Save As dialog. Standard text copy and paste
remain available. Closing the home window closes its home-only connections
and diagnostics, while detached terminal windows retain their live sessions.
Reattach activates or reopens the home window. The engine exits after its final
window closes. An independent Terminal workspace has its own engine. Window
placement and size are stored under the selected app-data folder.

The app frame stays within the window. Navigation, system identity, theme and
settings remain accessible in the sidebar without a separate top bar. Diagnostic
tools place configuration beside the
results, with separate scrolling for each area. Other pages scroll inside
their workspace. Long terminal output, authentication prompts and dialog forms
scroll within their own areas. Terminal tabs, pane controls and the command
bar use compact spacing so the consoles fill the remaining area. Saved connections are available in the connection dialog. Session opens active
controls and display settings. Tools contains profile management, SSH keys and
interaction defaults. Output controls and session toggles remain beside the console.
The panel overlays the console and scrolls independently when needed.
Detached windows omit repeated app headings and the footer; their native
window title identifies the app. Sessions, New tab and Reattach remain
accessible, and opening Tools preserves the terminal screens and output.
Scrollbars appear only where content overflows. Focus a scroll area and use Page Up, Page Down, Home or End.

TOOL WORKSPACE

Use the navigation button beside the sidebar brand to collapse the sidebar to
icons or restore labels. The preference is remembered on wide windows. On
narrow windows, the same button moves into existing tool controls and opens a
navigation drawer. Escape, its Close
button, or a click outside the drawer closes it and returns focus.

Tool configuration keeps the target, source interface, primary options and a
live summary visible. Timing and limits, additional request options and help
are grouped in expandable sections. A closed section shows its current values
and a Modified indicator when they differ from the defaults. Important
validation messages remain visible in the summary above the configuration
scroller. Expand options opens every section for the current tool.

Add networks / review scopes groups Local networks, Routed networks and
Selected scopes into tabs with counts. Local and route cards still toggle
scopes with a click; the selected tab holds the detailed scope plan and input
formats. Target input and Clear remain directly available. TCP ports offers a
Port list dropdown with editable custom ports, as Network discovery does.

Section choices and the selected network tab are remembered. The available
workspace width controls whether configuration and results fit side by side;
collapsing navigation can make room for both. Compact windows arrange primary
configuration fields in columns when space allows. Local IP/Wi-Fi controls,
calculators, reports, settings and terminal connection forms use compact,
responsive spacing with their own scrolling areas.

On wider windows, the left panel holds the current tool's target, adapter,
range and options. Results fill the right panel. The command bar above both
panels keeps the current target or scan range and Run/Stop accessible while
you review either panel. Changing settings during a run prepares the next
run; it does not change the diagnostic already in progress.

Use the Clear button inside the target input to empty the current network
range, hostname/IP or Wake-on-LAN target. The input keeps focus so you can
enter a replacement immediately. Clearing with the button or keyboard is
remembered across navigation, adapter refresh and app restarts. Selecting a
local adapter or a saved target explicitly fills the input again. Single-device
and network-range entries remain independent, and running scans keep their
original target and results. Run requires a nonempty input.

Drag the divider to adjust configuration width, or focus it and use the Left
and Right arrow keys. Hold Shift for larger steps. Hide configuration gives
the results more space; Show configuration restores the same entries. The
panel width and collapsed preference are saved with the other app settings.

In smaller windows, use Configuration and Results in the command bar to
switch between full-width panels. The target summary and Run/Stop stay above
both. Switching preserves the entered options and displayed results. Starting
a diagnostic shows Results automatically; preparing a target opens
Configuration so you can review it before running.

In compact windows, navigation moves into the menu at the top left. Short
windows use less decorative space and omit the secondary footer when needed.
The layout follows the available window space and display scaling. Larger
windows keep the side navigation and show configuration beside results when
space permits.

Discovery keeps the scan controls, response summary, completed probe counts,
tested-address counts and progress bar above the scrolling list. They remain
visible at the bottom of a long list, during updates and after completion.
Cancellation keeps partial progress. Outcome, search, sorting, column
selection and entry tools remain usable while results arrive. List filters
and column headings also stay visible as results scroll. Additional explanations,
the activity log and raw data are available in expandable sections. Local
network opens with IP configuration first, followed by read-only snapshot
tabs. Controls stay above the results. Wi-Fi output uses the results scroll
area, avoiding nested vertical scrollbars.

When an older result is selected while the same tool is running, Stop still
controls the active run. Return to the active run from the result selector to
review live progress. Layout controls and navigation do not start diagnostics.

Start and Stop are on the left of the command bar. Discovery results have a
selected-device pane: click a row or focus it and press Enter to inspect the
recorded identity, hostname, MAC/vendor, response evidence, ICMP statistics
and individual TCP outcomes. Selecting a row does not run a diagnostic or
open a connection. Use its explicit SSH, Telnet, HTTP, HTTPS, RDP, Ping,
Trace route or device-information actions when needed. Copy details, Copy IP
and Copy MAC provide quick access to the recorded values.

The selected device remains selected when results update, sort or filter.
A note identifies a selection outside the current filter or page. Use Details
to hide or restore the pane, choose Auto, Right or Bottom for its position,
and drag its divider to resize. The layout adapts to the available space.
Pane position and size are stored with normal app preferences.
On narrow or short result areas, device details open as a drawer. Use its
close button to return to the list. The scan controls and response/progress
summary stay above both views.

Table column edges can be dragged throughout the interface. Double-click an
edge to fit visible contents. A focused edge also accepts Left/Right keys,
Shift for larger steps, Enter to fit and Home to restore its default width.
Each table has one Reset widths control in its own toolbar. It restores only
that table's default widths. Nested context tables do not add controls to the
main Discovery toolbar. Widths are remembered per view and column, independently of visible columns, sorting and filtering.
Exported reports retain their own readable layout and all recorded rows.

Device icons describe the observed open-port group or neighbor-cache entry.
They do not establish a device model, operating system or verified service.
Hover over an icon for its meaning; text labels and outcomes remain available.

SOURCE INTERFACE AND ROUTED NETWORKS

Source interface is available in Device check, Ping, Trace route, TCP ports,
Connection monitor, DNS, HTTP, TLS, Network discovery and Device details.
Automatic routing is the default. Each selectable address shows the adapter
name, description and source IP. Active VPN and virtual adapters are included
when they have a usable assigned address. One remembered selection is shared
across these diagnostics and their entry-tool Run actions. Changing it during
a run changes only the next run. It does not change Windows adapter settings,
the routing table, terminal sessions or external connection applications.

Each new run validates that the chosen adapter is up and still owns the IP.
A missing adapter, stale selection, wrong address family or binding failure
is reported; the diagnostic never silently switches to Automatic. An
unavailable saved source remains visible until you select an available
address or explicitly choose Automatic routing. Refresh updates the choices.
Network discovery, Device details and network-wide Ping/TCP require IPv4.

When a source is selected, network DNS lookups use a compatible DNS server
configured on that adapter and bind to the same source. The DNS tool may use
an explicitly entered compatible literal server address instead. Without a
compatible resolver, literal-IP checks remain available and hostname lookup
reports why it could not run. The chosen source and resolver are recorded with
each result, available from its pinned Source label and included in exports.
Recorded source metadata describes the validated selection, not packet capture
proof that a remote reply arrived through a particular physical interface.

Selected-source HTTPS and certificate checks use locally available trust
material to prevent certificate downloads through a different interface.
If a required issuer or root is not available locally, verification reports
that failure. Automatic routing retains the existing Windows trust behavior.

Routed networks appears in Discovery and the Networks and hosts modes of Ping
and TCP ports. It reads the local routing table and offers additional IPv4
destinations with their gateway and adapter description. Search and pagination
keep long route lists usable. Duplicate destinations share one card with
their available route paths. Default and split-default, loopback, link-local,
multicast, reserved/system destinations, local-address/broadcast host routes
and connected subnets already listed above are omitted.

Clicking a route card adds or removes its exact prefix from the scan scopes;
it does not start a scan or change the selected source. Routes may describe
networks that are not currently reachable. Large routes retain their prefix
and show a narrowing hint. The existing address and probe limits still apply;
enter a smaller subnet or range before scanning an oversized destination.
Refreshing route suggestions does not erase manually entered scan scopes.

RESULT ITEM DETAILS

Click a result row, or focus it and press Enter or Space, to inspect the
recorded item and its available tools. Discovery retains its selected-device
pane. Other result lists open a details dialog with the complete record,
copy controls and applicable diagnostics or connection actions. Examples
include Ping samples, traceroute hops, DNS records, TCP ports, neighbors,
routes and active connections. Records without a usable endpoint still show
their details without inventing a connection target. Selecting an item does
not run a diagnostic or connect; those actions remain explicit buttons.
Existing right-click menus, filters, sorting, resizable columns and text
selection remain available.

AVAILABLE TOOLS

Overview
  Local hostname, network adapters, IPv4/IPv6 addresses, MAC addresses, MTU,
  adapter state, DNS servers, gateways, and Windows-reported link speeds.
  Default gateways and configured DNS servers appear immediately below the
  top summary cards, before Public IP and the diagnostic tools.
  A Public IP panel shows the Internet-facing IPv4 and IPv6 observed for this
  computer, with individual copy buttons, Refresh and the last check time.

Device check
  Troubleshoot one hostname or IPv4/IPv6 address using local adapter
  information, target name resolution, a reverse hostname lookup for the
  first tested address, up to three ICMP echoes, and selected TCP port checks.
  Hostname is informational and does not imply reachability. Each check
  reports its own outcome. The default TCP ports
  are 53, 80 and 443. Use Network discovery for a range of devices.

Ping
  IPv4/IPv6 ICMP samples, response time, loss, minimum/maximum/average, and
  variation between consecutive successful samples. Count, interval, and
  timeout are adjustable. Windows IP Helper APIs perform ICMP probes.
  Networks and hosts mode sends one echo per usable IPv4 address and lists each
  started address. Single-target count and interval do not apply to a sweep.

Trace route
  Hop-by-hop ICMP probing with adjustable maximum hops, samples, and timeout.
  Each responding hop shows its IP address and hostname when reverse DNS is
  available. Resolve hop hostnames is enabled by default and can be turned off.
  Lookups are limited to 1.5 seconds per distinct address and cached for the
  run. Missing PTR records or lookup failures leave IP addresses and probe
  timings visible. Hostname and IP columns can be sorted and filtered.
  Intermediate routers may filter probes or omit replies.

DNS
  A, AAAA, CNAME, MX, NS, TXT, PTR, SRV, SOA, and common-record lookup.
  Use the computer's resolver or explicitly select a DNS server. Direct
  queries display TTL and handle truncated UDP replies with TCP retry.
  Native resolver results do not expose TTL. SOA and preformatted reverse
  names use a configured resolver when a server was not explicitly entered.

TCP ports
  Individual ports and comma-separated lists/ranges. Results distinguish
  open connections, refused connections, timeouts, and other errors.
  Conventional service labels identify the usual port purpose; they do not
  identify the application actually listening there. UDP ports are not tested.
  Networks and hosts mode checks every selected port on each address, including
  devices that do not answer ICMP. Optional model identification uses SSDP
  separately; it is not a UDP port scan.

HTTP
  GET/HEAD checks with response headers, status, remote address, redirects,
  DNS/connect/TLS/first-byte timing, and certificate details for HTTPS.
  Connections are direct, without the system/browser proxy configuration.
  No cookies are retained; returned Set-Cookie headers are redacted.
  Body content is discarded after a bounded read. This is not a browser
  rendering test or a full bandwidth speed test.

TLS certificates
  Certificate subject, issuer, serial, SANs, validity dates, fingerprint,
  negotiated protocol and cipher, and a separate trust/name/time validation
  result. The certificate inspection tool can retrieve an invalid certificate
  so that its actual problem is visible. It does not send application data.
  This is a certificate/handshake check, not a complete cryptographic audit.

Discovery
  Combine local adapter networks and entered IPv4 scopes in one scan. Each
  address receives one ICMP echo, every selected TCP port, optional unicast
  UPnP and a reverse hostname lookup. Local adapter context is collected once.
  Every address where a probe started is
  retained, including no-response, error and partially tested cancelled rows.
  Responded only is the default list view. Choose All tested addresses to
  inspect the other recorded addresses; switching views does not rescan.
  Per-address details retain ICMP outcomes, individual TCP port outcomes,
  timings, errors and response evidence. Reports retain the recorded dataset
  independently of the visible list filter.
  Model identification and hostname lookup are enabled by default and can
  each be disabled before starting. Hostname is a default column and supports
  search, sorting and export; aliases are retained in row details and reports.
  Not resolved is shown when no name is available. Results also show exact-IP
  neighbor MAC/vendor values and reported models where available. The default
  TCP ports are 22, 80, 443, 445 and 3389.
  The Port list dropdown offers groups for common services, network
  management, web services, remote access, Windows / Active Directory,
  file sharing, printers, email and databases. The TCP discovery ports
  field stays editable. Modify a preset or choose Custom to enter individual
  ports and inclusive ranges, such as 22,443,8000-8010. Commas, semicolons,
  spaces, tabs and new lines separate entries. Duplicate ports count once.
  Custom values and the selected list are remembered with the app settings.
  Selecting Custom after a preset restores the last custom list when one
  was entered. Existing custom lists are preserved when upgrading.
  Port presets only prepare the next run. They do not start a scan, change
  a running scan, or change the current list filters. These are TCP checks;
  UDP services are not tested by selecting a port with the same number.
  The scan summary updates its port and connection counts, and the existing
  64-port and 16,384 TCP-check limits still apply.
  Every address is tested directly, even when ICMP does not respond. Silent
  devices may still exist, and a firewall can return a refused connection.

NETWORK AND HOST SCOPES

Discovery and the Networks and hosts modes of Ping and TCP ports accept multiple
IPv4 entries in the same scan. Separate entries with new lines, commas,
semicolons, spaces or tabs. Supported examples:

  192.168.1.10                     One IPv4 host
  192.168.1.0/24                   Network with CIDR prefix
  192.168.1.10/255.255.255.0        Network with dotted subnet mask
  192.168.1.1-254                  Inclusive last-octet range
  192.168.1.250-192.168.2.10        Inclusive range across subnet boundaries
  192.168.1.*                      Usable addresses in this /24

Network cards show both the adapter name and description with the local IP
and subnet. Clicking a card adds its subnet to the scope list; clicking it
again removes that subnet. Manual scopes remain editable. Network selection
prepares targets. Automatic routing lets Windows choose the source; Source
interface binds supported diagnostics to the chosen adapter and address.
Selecting a network does not change adapter settings or start probes.

Overlapping entries are merged before work is scheduled. Each unique address
is scanned once, with one combined progress bar. CIDR and wildcard scopes
exclude their network and broadcast addresses through /30; /31 includes both
addresses and /32 includes its single address. An explicitly entered host or
range retains all requested endpoints, including .0 and .255.

The limit remains 1,024 unique addresses and 16,384 selected TCP host-port
checks per run. Discovery accepts up to 64 distinct TCP ports; TCP ports accepts
up to 1,024, subject to the same overall connection limit in network mode.
The input is validated as a complete list before any probe begins. Scope lists
accept literal IPv4 addresses. Use a single-target tool for hostnames or IPv6.

Device details
  Resolve the selected IPv4 device's hostname and request its reported
  manufacturer, model name/number and friendly name through unicast SSDP/UPnP.
  Hostname lookup is independent of UPnP support. MAC/vendor are also read from
  this computer's neighbor cache. Missing values remain explicit. The tool
  uses one unicast request and at most one same-IP description retrieval.
  Devices that do not implement or allow this protocol may not report a model.

Monitor
  Repeated ICMP or TCP connection samples with a live response-time view,
  loss statistics, and cancellation. More than one diagnostic can run at
  the same time, within the application limits below.

Local network
  IP configuration first, then routing table, IPv4/IPv6 neighbor cache,
  listening/active endpoints with process IDs and Wi-Fi information.
  Select an adapter to inspect current addresses, DHCP state, gateway and
  DNS servers. Configure DHCP or a static IPv4 address with subnet mask or
  prefix, optional gateway, and automatic or manual IPv4 DNS servers.
  The selector contains only adapters the engine marks configurable. Genuine
  disconnected and virtual/VPN adapters remain available when configurable;
  adapters that permit only independent DNS changes remain available too.
  Review changes shows the selected adapter and proposed changes. Choose
  Apply to adapter to submit the change.
  The app checks for configuration changes before applying and refreshes
  the actual state afterward. A multi-step failure reports partial changes.
  Multi-address adapters restrict address replacement to avoid silently
  removing additional addresses; independent DNS changes remain available.
  Multiple default routes or unreadable route settings can also restrict
  address changes; the form explains the reason. Clearing manual DNS is
  verified against the refreshed state before success is reported.
  Native Windows IP Helper enumeration supplies structured IPv4/IPv6 neighbor
  rows with interface association and offline MAC-vendor lookup. Other
  read-only snapshots and raw output use Windows' included route, arp, netstat,
  netsh, and ipconfig utilities. Output/errors remain available for inspection.
  Refreshing ARP / neighbors reads the local cache without probing devices;
  its Device details action can request a reported model for one IPv4 row.
  Wi-Fi lists current connection information and saved profiles using native
  Windows WLAN APIs. Select Show for a specific profile to view its saved
  passphrase, WEP key or raw PSK, with the key type clearly labeled. If Windows
  denies access and allows elevation, choose Show with administrator approval
  for a separate retry. Hide clears
  the displayed value. Profile changes and navigation also discard it.
  A per-user profile requires approval under that same Windows account;
  entering a different administrator account cannot reveal its private key.
  Loading or refreshing the Wi-Fi page never requests plaintext keys. Revealed
  keys stay in memory and are excluded from settings, reports and exports.
  Copy is explicit; a copied value then belongs to the Windows clipboard.
  Open and enterprise profiles have no shared key for this viewer to reveal.
  A raw PSK is shown as stored; its original passphrase cannot be recovered.
  Windows can restrict current-connection information based on adapter/service
  state and Location permission; saved profiles are listed independently.

Hosts file
  View and edit the Windows system hosts file from Local tools. Reload reads
  the current file; Review and save validates addresses and hostnames and
  shows the proposed change. Save hosts file submits the reviewed change.
  Comments, spacing, encoding and existing line endings are preserved.
  A backup is created when an existing file changes,
  and its path is shown after saving. If another program changes the file
  while it is open, reload before saving. Unsaved text remains in memory
  only and is excluded from app settings and diagnostic reports.
  Unrecognized encodings and linked files are not overwritten.

Wake-on-LAN
  Send a standard magic packet to an entered MAC address and broadcast
  address. A sent packet does not confirm that a machine woke successfully.

Utilities and reports
  Enter an IPv4 address, CIDR, or dotted mask such as
  192.168.10.25 255.255.255.192 in the subnet calculator. Select any prefix
  from /0 through /32. Changing it keeps the entered host address and
  recalculates its network, mask, wildcard and address ranges. A bare IPv4
  address uses the selected prefix, initially /24. A /0 covers all IPv4;
  /31 uses point-to-point endpoint accounting; /32 is one host route.
  Traditional Class A, B or C and its default /8, /16 or /24 mask are shown
  separately from the selected CIDR. Class follows the entered address,
  even when the chosen network crosses class boundaries. Special-use notes
  cover 0/8 and 127/8; Class D multicast and Class E reserved addresses have
  no unicast default mask. These labels do not change the chosen CIDR.

  Expand Possible networks, initially collapsed, to see the complete list
  calculated from the entered IP address and mask/CIDR. The mask's fully
  fixed octets stay the same; the partially masked octet steps through every
  possible subnet boundary. No separate parent range needs to be selected,
  and the address's traditional class does not control this list.
  For example, 192.168.19.1/21 or 192.168.19.1 255.255.248.0 shows all 32
  possible /21 networks for 192.168.*.*: 192.168.0.0, 192.168.8.0,
  192.168.16.0 and so on through 192.168.248.0. Each row shows its usable
  host range and broadcast address. The entered address's subnet is marked.
  A /26 shows four networks for the same first three octets. An exact /8,
  /16 or /24 has one network for its fully fixed octets; /0 covers all IPv4,
  and /32 shows the entered host address. /31 rows use point-to-point endpoint
  accounting and have no separate broadcast. At most 128 networks appear;
  all are shown together without paging or truncation.
  Filter by IP/CIDR, choose ascending/descending order, or use Current subnet
  to return to the entered address's row, clearing an excluding filter when
  needed. Copy one CIDR, copy the matching list, or export the matching rows
  as CSV. These calculated ranges do not indicate live or assigned networks.
  The separate Split into smaller subnets section still divides the selected
  subnet into at most 256 smaller ranges and has its own CSV export.

  Paste multiple MAC addresses into the MAC formatter, one per line or
  separated by commas/semicolons. Each entry is validated independently;
  valid entries can be copied together in the selected format while invalid
  entries remain visible for correction. Input order is retained.
  Other utilities include address classification, bandwidth conversions,
  saved targets, readable HTML reports and TXT/JSON/CSV exports.

NETWORK ADAPTER IDENTIFICATION

Network adapter labels include the friendly name and Windows-reported
Description when available, for example Ethernet 5 - Intel Ethernet Adapter.
The same format is used in network scan selection, Overview, IP configuration
and Wi-Fi adapter views. A repeated description is shown only once; a missing
description leaves the existing name available. Long labels wrap in detail
areas, and dropdown selections have supporting text for the complete label.

Descriptions help distinguish physical, USB, wireless and virtual adapters.
Selection and configuration still use the adapter's original identifier;
display labels are not submitted as adapter IDs. Duplicate descriptions do
not merge separate adapters.

Structured route and neighbor results retain adapter descriptions captured
with that result. Older or imported results without this metadata keep their
recorded labels. Historical data is not attributed to a currently connected
adapter simply because it has the same name or interface index. Raw operating
system command output remains available in its original form.

PUBLIC IP ADDRESS

Overview and Local network include the same Public IP panel. Opening either
view checks the computer's Internet-facing IPv4 and IPv6 in the background.
The existing adapter/configuration tools remain usable while the check runs.
Each address has a Copy button; Refresh requests a new check. The panel shows
when the result was checked and identifies ipify as the lookup provider.

The value is the address seen by the HTTPS lookup service for this computer's
current Internet route. A VPN, NAT gateway or upstream carrier gateway can
change it. Adapter selection in IP configuration remains a separate control;
this observation is not bound to that selected adapter. Refresh after changing
a VPN connection, Wi-Fi network or Internet gateway to request a new result.
This check does not establish whether incoming connections can reach the PC.

The engine requests https://api.ipify.org over IPv4 and https://api6.ipify.org
over IPv6 using direct HTTPS connections. It does not configure or use an
application HTTP proxy. Existing operating-system routing, including VPN
routes, determines the path. Internet access and access to these services are
required for the public-address lookup. No provider API key or extra installed
software is required.

A result is reused for up to five minutes within the current app process;
explicit Refresh checks again immediately. Concurrent requests share one
check, with an eight-second total limit. There is no periodic background
polling. Opening a terminal-only window does not perform a public IP lookup.

IPv4 and IPv6 have independent outcomes. Unavailable means that family could
not be confirmed by the lookup service at that time; it does not establish
that the Internet is down or that IPv6 is disabled. Error details remain
visible. A completed failed refresh clears the earlier address instead of
presenting it as current. Public-address results stay in memory and are not
automatically added to settings, diagnostic jobs or reports.

Provider documentation: https://www.ipify.org/

LOCAL NETWORK WORKFLOW

Choose Discovery, or switch Ping or TCP ports to Networks and hosts.
Click local network or routed-network cards to add or remove target scopes,
then review or edit the networks, hosts and ranges. Target cards do not select
the sending interface. Automatic routing lets Windows choose the source;
Source interface binds supported diagnostics to the chosen adapter and
address. VPN and virtual adapters remain available. Oversized scopes must
be narrowed before scanning. Selecting a scope never starts a scan.

The form shows usable hosts and requested work before starting. Follow a
result row with Device details, Device check, Ping, TCP, PTR/DNS, Trace route,
HTTP, TLS, or Monitor. The device menu provides explicit Run actions for
immediate diagnostics, Configure actions for reviewing options first,
SSH/Telnet connection forms, and Open HTTP, Open HTTPS and Remote Desktop
actions. Selecting a row alone starts no network work. Single and network targets are
stored separately. Cancel stops further dispatch and retains observations
from completed and partially tested hosts. Discovery and Ping/TCP subnet scans
retain all started addresses. Discovery initially shows only response-bearing
rows; All tested addresses exposes the full recorded set. Addresses that were
never started are counted separately and are not fabricated as result rows.

Device check replaces the former single-target Quick check. The former Quick
check Local network mode is consolidated into Discovery. Saved Quick subnet
targets open in Discovery without starting a scan; single-device settings are
retained. Older reports remain readable. Legacy Discovery reports recorded
only responding addresses, so they cannot supply missing silent-host rows.

RESULT FILTERS, SORTING AND COLUMNS

Structured result lists have search fields and sortable column headings.
Click a heading again to reverse its order. IPv4/IPv6 addresses, endpoint
ports, latency, counts and dates use their corresponding value types rather
than simple alphabetical ordering. Empty values remain at the end. Matching
and sorting cover the whole list before paging, including rows beyond the
first 100. The footer shows matching and total counts; Reset clears the
list's search/status choices and restores its default ordering.

These controls also cover saved targets, adapter cards, report jobs, activity
logs, subnet calculations, terminal sessions, saved connection profiles and
remembered SSH hosts. Cards use a sort selector. List search, sort and status
preferences are remembered; the current page is not saved.

Responded only hides scan rows with no response evidence. In Discovery, choose
All tested addresses to show every recorded row. Ping/TCP scans use a toggle.
ARP / neighbors uses Resolved only to hide entries without a
resolved MAC; a cached mapping is not proof of a fresh device response.
Existing search and status filters remain available.

Columns controls the visible fields independently for scan results and ARP
neighbor results. Hostname is visible by default in scan results; Model is
hidden by default. This upgrade enables Hostname once in older saved scan
column preferences; hiding it afterward is remembered. The selected columns are saved
with app settings in the selected data folder. Device identity
and actions remain available through each row's Tools menu even when an
optional column is hidden.

ROW TOOLS

Right-click a device or an applicable result entry to open its quick menu.
The visible Tools button provides the same access. With an entry focused,
Shift+F10 or the keyboard Menu key opens its menu; Escape closes it.

Actions use that entry's own address and, where present, its observed port.
Available actions include Open HTTP, Open HTTPS, Remote Desktop (RDP), SSH,
Telnet, opening a connection form in the independent Terminal window, Ping,
Trace route, PTR/DNS, TCP ports, HTTP check, TLS check, device details and
copying an address, endpoint, MAC or row details. Wildcard socket addresses
and network routes are not treated as discovered device endpoints.

Open HTTP and Open HTTPS open the selected device in the default Windows
browser. Remote Desktop opens Windows Remote Desktop Connection (mstsc.exe).
These actions are available in both the right-click menu and Tools dialog.
The visible destination includes its port. A selected TCP port or service
endpoint retains that port, including custom ports. A general device row
uses a relevant known open web port, or defaults to HTTP 80, HTTPS 443 and
RDP 3389. An SSH/Telnet profile does not reuse its terminal port for these
actions. UDP endpoints are not offered as web or RDP connections.

Hostnames, IPv4 and ordinary IPv6 addresses are supported. IPv6 addresses
with a scope ID are not supported by these three external launch actions;
use the device's DNS name or another reachable address. For URL-derived
entries, only the host and port are used, without credentials, path or query.
The external browser and Remote Desktop client handle their own credentials,
certificates, permissions and settings. Launch acceptance does not confirm
that the device is reachable or that a remote session connected.

Explicit Run actions start diagnostics; Open HTTP, Open HTTPS and Remote
Desktop launch their client immediately when clicked. Configure and terminal
connection actions prepare a form for review and a separate Connect action.
Opening either menu or selecting an entry starts no connection or probe.

INTEGRATED TERMINAL

Open Terminal in the sidebar, choose SSH, Telnet, or Serial, and complete the
connection form. New terminal tabs and empty panes use the same welcome view
with all three connection buttons. Empty split layouts show a welcome in each
pane, including in independent and detached workspace windows. Select Connect
to start a session in the pane where its connection form was opened. Device-row
SSH and Telnet
actions fill the discovered IP address and conventional port. SSH credentials
are entered in the connection panel; they are never inferred from a scan.
Up to eight sessions can be active at once, each in its own terminal tab.
Changing tools or tabs does not disconnect an active session. Disconnect
closes a connection; Reconnect restarts it in the same tab and window using
its previous settings and original authentication. No saved profile connects
automatically at startup.

Independent Terminal window
  Choose Separate terminal window, or Open in terminal window from a
  connection or device menu. Open_Terminal.cmd launches the same EXE directly
  in Terminal mode, without first opening the diagnostics window. You can
  also launch Network_Toolkit_1.17.29_Windows_x64.exe --terminal.

  This window continues running when the main diagnostics window closes.
  Repeated launch requests bring back the existing Terminal window, where
  up to eight sessions can run in tabs. A connection request prepares the
  form for your review and Connect action. If another form or authentication
  prompt needs attention, the new request waits behind a Review button.
  Separate terminal window opens an independent workspace. Detach moves the
  current terminal workspace with a return path to its original window. When
  the Detach choice is shown, Active session moves only the focused connection.

  The independent workspace keeps its own profiles, SSH host trust, theme,
  terminal preferences and window placement. On first use it starts with
  the main window's saved nonsecret connection preferences and public SSH
  trust. Later changes in either window are independent. Passwords, private
  keys, passphrases and terminal input are never passed to a new window.

Workspace detach and reattach
  Select Detach even before opening a connection. The separate
  app window retains the connection defaults, terminal tabs and pane layout.
  Create SSH, Telnet or serial sessions there, or bring existing sessions
  with the workspace. Reattach workspace returns at any time, including
  while connecting, at an authentication prompt, or after disconnecting.
  Passwords, private keys and passphrases in an unsubmitted connection form
  are cleared when moving the workspace and must be entered again.

  The original engine owns every connection throughout the move. Scrollback
  and split panes return without reconnecting. Closing the home window keeps
  the linked terminal workspace available, including when it has no sessions.
  Reattach activates or reopens the home window. Closing the detached view
  normally reattaches; Disconnect ends a session explicitly. The independent
  Terminal workspace remains available as a separate launch option.

Live detach and reattach
  Focus a connected SSH, Telnet or serial session and select Detach. When the
  scope choice is shown, choose Active session. Its socket or COM handle stays
  in the original engine; the new native window
  displays that exact session. No extra connection or authentication occurs.
  Reattach in the detached window returns it to its original workspace. The
  home toolbar offers the same return action when that session is selected.
  Closing the detached window normally returns
  the session; Disconnect remains an explicit separate action.

  Scrollback, parsed screen, cursor, alternate screen, terminal modes and
  partial UTF-8/escape sequences transfer in memory. A scoped ownership
  revision prevents the previous window from typing or resizing after a move.
  If a window crashes, its last display checkpoint is recovered with an
  explicit notice. Output older than the bounded replay buffer may be missing;
  the connection itself is not reopened. A transfer can be refused while an
  unsupported control string is incomplete or its snapshot exceeds a limit.
  Complete the output or reduce scrollback and retry; the transport stays live.

Split panes
  Select Single pane, Two columns, Two rows, Three panes, or Four panes.
  Choose a session in each pane, or choose SSH, Telnet or Serial from an empty
  pane to prepare a new connection there. New tab opens the same welcome view
  without connecting or discarding the previous session. Selecting a session
  already visible elsewhere
  swaps the panes. Click a pane or use Ctrl+Alt+1 through Ctrl+Alt+4 to focus it.
  The focused border and title identify where toolbar actions, search, paste
  and keyboard input apply. Input is never broadcast to all panes.

  Drag a divider to resize. Focused dividers support arrow keys, Shift for
  larger changes, and Home to center. Maximize temporarily enlarges one pane;
  Restore returns the saved layout. Hide pane leaves the connection running.
  Closing an unused New connection tab removes only that empty tab. Closing a
  session tab keeps the normal disconnect confirmation. Up to 64 unused tabs
  can be prepared; these are temporary and have no live connection. Empty
  pane positions transfer with a detached workspace, while unused tab names
  are local to their window. Layout and divider proportions are saved; live
  session identities and unused tabs are not persisted.

Terminal application
  All SSH, Telnet and Serial actions use the integrated terminal. Separate
  terminal windows are instances of Network Toolkit. Previous preferences
  for another terminal application are removed when settings are loaded or
  saved. Independent windows, live detach/reattach and split panes remain
  available. HTTP, HTTPS and Remote Desktop keep their dedicated client actions.

SSH
  Supports an interactive PTY shell with password, OpenSSH/PEM private key,
  encrypted private key/passphrase, and keyboard-interactive authentication.
  The terminal sends window-size changes and normal control/function keys.
  SSH protocol and cryptography use the bundled official Go x/crypto library,
  compiled into the EXE. No ssh.exe, PuTTY, agent, or DLL is required.

  SSH compatibility defaults to Modern (recommended). If an older device
  offers only obsolete algorithms, select Legacy devices for that connection
  and then Connect. A matching negotiation failure also offers Configure
  legacy connection, which prepares the same endpoint for review. It does
  not reconnect automatically; enter credentials again and choose Connect.

  Legacy devices keeps modern algorithms first and additionally permits
  diffie-hellman-group14-sha1, diffie-hellman-group-exchange-sha1,
  diffie-hellman-group1-sha1, raw ssh-rsa signatures and aes128-cbc.
  These older methods provide weaker protection. The setting belongs to the
  connection/profile, including independent Terminal window requests; it
  does not change other profiles. New connections default to Modern.
  Save the profile to remember the choice. Host verification remains active.

  The first connection to an unknown host shows its key type and SHA-256
  fingerprint. Verify it with the device's administrator or known record,
  then choose Trust once or Trust and save. A changed remembered key requires
  explicit fingerprint verification before replacement. The app does not
  silently accept or replace a changed key. Remembered host entries can be
  inspected and removed from the terminal interface.

Telnet
  Provides an interactive Telnet session with terminal-type, window-size,
  suppress-go-ahead, echo and binary negotiation, IAC escaping, and NVT line
  handling. Telnet traffic and login input are unencrypted. Local echo follows
  the negotiated Telnet echo state.

Serial
  Refresh the port list and select a Windows COM port, or enter its name.
  Select baud rate, data bits, parity, stop bits, and flow control for the
  device. Native Windows communication APIs open the port directly. Terminal
  controls provide local echo, Enter line endings, DTR, RTS, and break. Flow
  control and supported settings depend on the adapter and its driver.
  DTR/RTS labels show requested settings rather than measured electrical
  state. Manual RTS is disabled when the driver controls RTS/CTS flow.
  Send break requests a short pulse; the engine also clears an asserted
  break after one second if the interface's clear request is interrupted.

Terminal controls
  ANSI/VT rendering, colors, cursor movement, Unicode text, scrollback, search,
  adjustable font size, fit-to-window resizing, selection/copy, paste, clear,
  and common control keys are provided by the locally bundled xterm renderer
  and the compact command bar or Tools panel. Transcript export writes the
  selected terminal's retained display text through the normal Save As flow.

  In Terminal, open Tools, then Mouse and keyboard to configure behavior.
  Ctrl+C sends the interrupt character to the device by default, including
  when text is selected. The optional Copy selection; otherwise send Ctrl+C
  mode keeps copy-on-selection behavior for that shortcut. Ctrl+Shift+C always
  copies selected text. Selecting output with the mouse automatically copies
  it by default; this can be disabled independently.

  Ctrl+V, Ctrl+Shift+V and Shift+Insert paste directly into the focused console.
  Single-line text is sent without adding Enter. With Review paste enabled,
  any pasted CR or LF, including a trailing line break, opens an editable review. The
  review names the destination and shows line-break and byte counts. Nothing
  is sent until Send to this console is selected. Cancel sends nothing.
  Editing the review does not send keystrokes to the device. A changed pane,
  session, ownership or connection invalidates the pending paste; start again
  in the intended console. The existing 1 MiB input limit remains in effect.

  The Paste button and right-click paste use the same review. If clipboard
  reading is unavailable, focus the console and use Ctrl+V directly; there is
  no separate empty paste box. The Windows app allows clipboard reads only
  for an explicit interaction in its own local interface and does not save a
  general clipboard permission. The terminal's bracketed-paste mode remains
  available when the remote program supports it.

  Right-click pastes by default. Choose Context menu to make right-click open
  terminal tools instead. Hold Shift while right-clicking to use the opposite
  action for that click. Choose context-menu tools controls the available
  selection/line/value copy, paste, search, transcript, display and session
  actions. Mouse and keyboard settings always remain reachable in the menu.
  The Menu key or Shift+F10 opens the same menu from the console.

  Ctrl+mouse wheel increases or decreases console text size from 10 to 24 px.
  It changes terminal text rather than app zoom. The current workspace's
  consoles refit to the new size and retain their output and connections.

Click to copy terminal output
  Hovering a console line adds a subtle full-line highlight. There is no
  overlay button. Use Copy line in the terminal context menu to copy the
  complete logical line, joining wrapped rows without adding a newline.
  The Menu key or Shift+F10 also exposes the copy actions at the cursor.
  Highlighted IP/CIDR, endpoint, MAC, interface and status tokens show a copy
  cursor and tooltip. Click to copy that value. A brief confirmation names
  the copied item. ANSI/OSC codes are excluded from the copied display text.

  Dragging selects text normally. Colored-token copying yields to remote
  applications that enable mouse tracking. Context-menu copy remains
  available. Token click-copy can be disabled under Mouse and keyboard
  defaults. A changed record is not silently copied from a different position.

Network highlighting
  Highlights are enabled by default and can be switched off from the
  Session panel. The legend explains address, MAC, interface, error,
  warning and success markings. Recognition checks address formats and
  common diagnostic words. A highlight is a reading aid, not a diagnosis
  of device health or proof that a command succeeded.

  Blue identifies IP/CIDR text, teal MAC addresses, violet interface labels,
  amber warning/pending labels, red error/down labels and green up/success
  labels. Both dark and light themes have matching palettes. Existing ANSI
  foreground/background colors and inverse/hidden cells are preserved.
  Distinct underline patterns add a cue beyond color. Up/success uses its
  text color and underline without a vertical side bar. The display layer does
  not rewrite output, commands, selections or exported transcripts.
  Processing is bounded to the visible viewport. Highlights pause while
  a full-screen program uses the alternate terminal buffer, then resume
  when normal output returns. Hidden panes pause recognition too. Plain
  terminal rendering remains available. Format/severity recognition follows
  primary terminal and protocol references listed below; the semantic palette
  is an application convention, not a mandatory protocol color standard.

Saved connection profiles retain names, addresses, usernames, authentication
method, SSH compatibility and serial/display preferences. Passwords, private
keys, passphrases,
and keyboard-interactive answers are not saved in profiles or settings.
Terminal output remains in memory unless explicitly exported. A shared bounded
event buffer preserves responsiveness with multiple sessions; the interface
reports if older output was discarded. Terminal history is transient and ends when its owning
workspace finally closes. The diagnostic report does not silently include terminal sessions.

DEVICE IDENTITY AND VENDOR DATABASE

The executable includes 58,808 unique prefixes from the IEEE MA-L, MA-M,
MA-S, and legacy IAB public registries retrieved on 2026-10-20. Lookups choose
the longest matching prefix and need no internet connection. The few historic
prefixes with multiple registered organizations show every listed name.

In App settings, open Vendor database:

- Update from IEEE downloads and validates all four official HTTPS CSVs.
  Updates occur only when requested. The entire operation has a 60-second
  deadline; a download, validation, or save failure keeps the previous table.
- Import IEEE CSV accepts a local UTF-8 file with Registry, Assignment, and
  Organization Name columns, up to 16 MiB. It replaces matching prefixes and
  retains all other current entries, so one registry can be imported without
  discarding the others. Imported database results are labeled as local CSV.
  The complete merged table must also fit the 24 MiB serialized-data limit;
  an oversized merge is rejected before replacing the active saved table.
- Reset to bundled removes the saved override and restores the embedded table.

The settings section shows the source, source/retrieval date, number of
entries, and bundled or saved status. Imported CSV has an unknown source date;
the Last updated field records its import time. Successful changes apply to subsequent
scans and neighbor refreshes. Existing results remain historical snapshots.

An updated/imported table is saved as Network_Toolkit.vendors.json.gz in the
selected app-data folder. Copy that optional file to another computer's chosen
data folder to carry the updated database. A corrupt saved table produces a visible
status error and falls back to the embedded table. An unwritable folder cannot
persist an update; the active previous database is retained.

A MAC assignment identifies its registered organization, which may be the
network adapter maker. It does not establish the device brand or model.
Locally administered/randomized, multicast, broadcast, unavailable, and
unregistered addresses are shown without an invented vendor. Routed devices
usually do not expose their own MAC in this computer's neighbor table. The
app never substitutes a gateway MAC for a remote target and suppresses
conflicting MAC assignments for the same IP in consolidated scan results.
The neighbor view retains the separate interfaces so they can be inspected.

Models and reported manufacturers come only from the device's own UPnP
root-device description. That information is self-reported, not authenticated.
TCP subnet scans offer this as an opt-in check; Discovery enables it by
default. Ping sweeps do not send model probes. Identification is bounded to
2.5 seconds per host, inside the existing host pool, and retrieves at most
512 KiB of XML. It accepts only a description at the selected literal IP,
uses no HTTP proxy, follows no redirects, sends no credentials, and invokes
no UPnP configuration action. A missing model is displayed as Not reported.

Hostnames come from reverse lookup through configured DNS servers and local
hosts entries. Discovery allows up to 1.5 seconds per address for this lookup
inside its existing worker pool and includes the lookup in progress counts.
A cached name can exist for a silent or reassigned address. It never counts as
a device response and does not change the Responded only filter. Connection
tools continue to use the discovered IP. UPnP friendly names, vendor names
and model strings are not substituted for DNS hostnames. NetBIOS discovery
and special Windows name-resolution policies are not implemented by this
reverse lookup; devices without an available reverse record may remain unnamed.

APP DATA, LOCATION AND DELETION

The default Windows data folder is %LOCALAPPDATA%\NetworkToolkit. The EXE
can remain in a separate portable or read-only program folder. App data is
not written beside it by default. There is no executable-folder fallback.
The Unix development build uses the operating system's user config folder.

App settings shows the current workspace, root folder, default folder and any
next-launch selection. Enter an absolute data-folder path or choose the default.
A location change takes effect after closing and reopening the app. Copy existing
app data copies the known settings, public SSH trust, optional vendor database
and native placement files for main and independent Terminal workspaces.
Live browser caches are not copied. On orderly close the copy is refreshed
only while its destination files are unchanged, preserving later edits made
before closing. Existing destination files and data owned by another running
workspace are never overwritten. A conflict is reported with both copies kept.

The selected folder is remembered in the default AppData location. --data-dir
provides an explicit per-run override for scripts/testing; the location selector
is disabled during an overridden run. Main and independent Terminal processes
hold shared data-folder leases. Close the other workspace before migration or
deletion; maintenance returns a clear conflict while another workspace is open.

Delete stored app data and close removes this app's settings, saved targets and
profiles, remembered SSH host keys, vendor override, window placement and app
WebView profiles from the selected data root and its owned workspaces. It first
closes this engine's windows/connections and drains pending writes. Only known
app paths are removed; unrelated files and exported reports are retained.
Symbolic links and reparse points are not followed. The confirmation check
uses directory information without reopening active browser files, so an open
Cookies file does not block the close request. After shutdown, cleanup validates
the paths again and briefly retries Windows sharing/lock errors. If a browser handle
remains open, the app finishes closing and retains a cleanup marker; the next
launch retries before opening that workspace.
The selected folder choice and coordination lock files remain.

Network_Toolkit.settings.json holds saved preferences and targets.
Network_Toolkit.ssh_known_hosts.json contains remembered SSH public trust only.
Network_Toolkit.vendors.json.gz is the optional updated vendor database.
Network_Toolkit.window.json holds placement; Network_Toolkit.WebView2 is the
isolated app browser profile. Network_Toolkit.Terminal is the independent
workspace. Network_Toolkit.Views contains isolated detached-window profiles.
No terminal output, passwords, private keys, passphrases or authentication
answers are stored as app settings. Diagnostic output is kept only when exported.

Upgrading from an earlier release: the previous EXE-folder files are not imported
automatically. With every app window closed, copy the settings, known-host and
optional vendor JSON/gzip files listed above into the selected new data folder;
copy the independent workspace's corresponding files if needed. Do not copy a
running WebView profile or instance record. Alternatively select the previous
data folder, restart, then use Copy existing app data to move to the desired
folder. Review the new folder before removing your old copy manually.

Only the EXE is needed to copy the program to another computer with the Runtime
installed. Preferences and public trust can be copied separately into that
computer's selected data folder. The app's regular browser profile is isolated
from the user's normal Edge profile.

REPORTS

Diagnostic jobs are held in memory for the current app session. Export a
report to keep their results. Up to 100 recent jobs are retained per session;
older finished jobs are removed as new jobs are added. Running jobs are
not removed by clearing history.

Use Export on a single result, select results in Reports, or leave all
unchecked to export the session. HTML and TXT include every recorded row
from those results, regardless of app list filters, visible columns or pages.
Running diagnostics are saved as a snapshot, including retained early hosts.

HTML reports have a linked summary, focused result tables and full-width
expandable device/record details. Options, scan context and activity remain
available. Open the Details sections you want included before printing.
Run status describes diagnostic completion, not the health of every device.
Choose Export > TXT list for readable plain-text results and recorded details.
HTML/TXT timestamps are shown in UTC. Both formats can be read offline;
the HTML report includes its own styles and needs no external assets.
Subnet calculations and report formatting run locally without additional
runtime dependencies or network requests.

JSON reports retain structured data for import, and CSV export remains
available. JSON import accepts files up to 10 MiB; select a smaller group of
results when exporting a very large session if you intend to import it again.

The standalone HTML in the source package is also the embedded interface.
Opening that HTML directly provides the local utilities and report viewer.
Live network diagnostics require the supplied EXE to be running because
the browser interface delegates network and operating-system operations
to the included native engine.

LIMITS

- Up to 8 simultaneous diagnostics.
- Up to 1,024 TCP ports in the TCP ports tool; up to 64 in Device check.
- Up to 1,024 usable IPv4 addresses in a network scan (/22 or smaller CIDR).
- Up to 16,384 host-port connections in one network scan.
- One shared pool per scan: up to 64 simultaneous host probes for Discovery
  and subnet Ping/TCP. Probes inside a host are sequential.
- Up to 64 selected TCP ports per discovery run, subject to the total
  16,384 host-port connection limit. Discovery defaults to 16 concurrent hosts
  and a 400 ms ICMP/TCP timeout; the timeout can be set from 100 to 10,000 ms.
- Ping and Connection Monitor accept a finite sample count or Run until stopped.
  Lifetime totals remain complete; up to 10,000 final sample rows are retained.
  Live activity keeps its separate 2,200-event window.
- Up to 64 hops and 3 probes per hop in trace route.
- Ping and Connection Monitor stop at the requested count or when stopped.
  Other diagnostic jobs retain their 2-hour duration limit.
- Per owning workspace, up to 8 live or shutting-down terminal sessions, with
  64 retained records, a shared 4 MiB event buffer, and bounded input queues.
- Up to four visible terminal panes per workspace, and eight live-view moves
  or detached displays. A screen handoff permits 8 MiB of serialized data plus
  256 KiB of emulator state, within a 10 MiB JSON request. A full-workspace
  transfer permits 32 MiB of combined screen/emulator data and 64 retained
  tabs, with up to four visible panes. Display checkpoints are transient;
  they do not provide unlimited crash-recovery history.
- One independent Terminal window per app-data folder. Its connection-request
  inbox retains the latest 16 requests and reports when older requests expire.
- The hosts-file editor accepts file bytes, submitted UTF-8 text and encoded
  saves up to 1 MiB each.
- Static IPv4 configuration accepts prefixes 1 through 32 and up to six
  distinct manual IPv4 DNS servers. Network/broadcast addresses are rejected
  for prefixes through /30.
- One local configuration change runs at a time.
- Settings file size is limited to 2 MiB.

These limits keep interactive work responsive. A timeout means that the
probe received no answer before its deadline. It does not prove a device
is offline, a route is broken, or a port is closed.

LOCAL SERVICE

The engine chooses a free local port and listens on 127.0.0.1 only. It does
not open a LAN listener or create a Windows firewall rule. Each run has a
new session token; the interface must be opened from the current executable
session. The URL may change between runs. Saved targets/settings use the
configured settings file rather than relying on a stable browser origin.

Network Toolkit has no cloud account, application telemetry endpoint,
automatic app updater, or external hosting. Probes run only when the user
starts a tool. The installed Microsoft Runtime is serviced separately from
this app by Microsoft or the computer's administrator.

PACKAGE CONTENTS

Network_Toolkit_1.17.29_Windows_x64.exe
  The diagnostic engine, embedded interface, native window, and Go loader.
Open_Terminal.cmd
  Optional double-click launcher for the same EXE's independent Terminal mode.
WEBVIEW_LOADER_LICENSE.txt
  ISC license for the compiled-in Go loader adaptation.
README.txt
  Usage and operational notes.
VALIDATION.txt
  Build and test information for this release.
THIRD_PARTY_NOTICES.txt
  Go runtime, Go SSH/system libraries, xterm renderer/addons, native loader,
  and IEEE registry notices.
SHA256SUMS.txt
  SHA-256 hash of the supplied executable.
source/
  Editable Go engine, HTML interface and locally bundled terminal assets,
  native WebView2 host,
  icon files, prebuilt link-time resources, and build scripts. Loader
  provenance and licensing are recorded in source/internal/desktop.
  Vendor source URLs, retrieval dates, and input hashes are recorded in
  source/internal/identity/data/SOURCE.txt; the embedded gzip is included.

BUILDING FROM SOURCE

The supplied application requires no compiler. To change and rebuild it,
install Go 1.26 or newer. This release was built with Go 1.27.3.
The pinned Go dependencies are included in source/vendor and the supplied
build scripts use vendor mode, so normal builds need no dependency download.
The JavaScript/CSS bundles are also included and need no npm install or build.
Go x/crypto v0.58.0 supplies SSH and x/sys v0.49.0 supplies supporting platform
code. xterm 6.0.0, addon-fit 0.11.0, addon-search 0.16.0 and
addon-serialize 0.14.0 are embedded
browser assets. Source URLs, integrity records and licenses are included.
The small loader adaptation is included as source under its ISC license.
It follows the
Go-native loader in Wails go-webview2 v1.0.23 and uses the installed Runtime
directly. It is not Microsoft's static loader library and does not include
the Wails UI framework. The included native .syso resource supplies the
icon, version information, and application manifest
during normal linking. The EXE is not modified afterward.

On Windows, open a command prompt in source and run BUILD_WINDOWS.cmd.
The single built EXE is written to source\build.

On Linux/macOS with Go available, run sh build.sh from the source folder.
Only the Windows x64 executable is the primary supported deliverable.
Unix platform code exists to enable controlled development tests.

The embedded vendor table is already included, so ordinary source builds
need no network download or Python. The optional source-maintenance script
scripts/build_vendor_database.py rebuilds the table from previously downloaded
IEEE CSVs and their provenance. In-app Update is the normal way to refresh it.

For future artwork updates, edit assets/source/Network_Toolkit_Icon.original.svg
and run python scripts/generate_icon_assets.py with Inkscape on PATH. This derives
the safe runtime SVG, PNG, nine-size ICO and source/asset integrity manifest.
Then run python scripts/build_windows_resources.py and rebuild with Go.
Ordinary builds need only Python's standard library for resource generation;
the freshness guard rejects stale or changed icon inputs instead of silently
compiling a mismatched icon. Do not hand-edit the derived SVG, PNG, ICO or manifest.

TROUBLESHOOTING

Interface does not open
  Read any startup error about the configured app-data folder or the
  installed WebView2 Runtime or profile folder. No additional DLL belongs
  beside the EXE. The app does not fall back to an ordinary browser window.

Interface loses connection
  Reopen the executable. Every executable session uses a new local port and
  token, so an old browser page cannot be reused for a new session.

Settings do not persist
  Check write access to the app-data folder shown in App settings and retain
  that folder's settings JSON. The EXE folder does not need to be writable.

Windows Defender or SmartScreen blocks the app
  This executable is unsigned. Windows may check a new download separately
  from previous versions. The compiled-in loader does not establish
  Microsoft publisher reputation. Validation notes identify completed tests
  and Windows behavior that remains untested in the build environment.

Ping/trace receive no response
  Check TCP reachability and review the local route/adapter information.
  ICMP filtering is common. A successful TCP connection proves that service
  answered even when ICMP receives no reply.

HTTP differs from your browser
  The diagnostic uses a direct request, without browser cookies, integrated
  authentication, JavaScript rendering, or a browser/system proxy.

Packet capture availability
  Native recording uses Windows Packet Monitor and requires administrator
  approval. If recording is unavailable, open a PCAP or PCAPNG file for
  inspection and troubleshooting. No third-party capture driver is bundled.

Full internet speed testing
  Third-party speed-test services are not included. Connection latency is
  not presented as network throughput.

PRIMARY FORMAT AND INTEGRATION REFERENCES

Terminal SGR, cursor and buffer behavior:
https://invisible-island.net/xterm/ctlseqs/ctlseqs.html
https://xtermjs.org/docs/api/terminal/interfaces/ibuffercell/
https://xtermjs.org/docs/api/terminal/interfaces/idecorationoptions/
IPv4/IPv6 and scoped address forms:
https://www.rfc-editor.org/rfc/rfc3986#section-3.2.2
https://www.rfc-editor.org/rfc/rfc4291#section-2.2
https://www.rfc-editor.org/rfc/rfc4007#section-11
Syslog severity and HTTP status classes:
https://www.rfc-editor.org/rfc/rfc5424#section-6.2.1
https://www.rfc-editor.org/rfc/rfc9110#section-15
Color is supplemented by text and underline patterns:
https://www.w3.org/WAI/WCAG22/Understanding/use-of-color.html
https://www.w3.org/WAI/WCAG22/Understanding/contrast-minimum.html
Pane conventions and SSH compatibility:
https://learn.microsoft.com/en-us/windows/terminal/panes
https://www.openssh.org/legacy.html
Native Windows Wi-Fi profiles and access restrictions:
https://learn.microsoft.com/en-us/windows/win32/api/wlanapi/nf-wlanapi-wlangetprofile
https://learn.microsoft.com/en-us/windows/win32/api/wlanapi/nf-wlanapi-wlangetprofilelist
https://learn.microsoft.com/en-us/windows/win32/api/wlanapi/nf-wlanapi-wlanqueryinterface
https://learn.microsoft.com/en-us/windows/win32/nativewifi/wi-fi-access-location-changes

TERMINAL INTERACTION REFERENCE

MobaXterm documentation was reviewed for right-click paste, a modifier-key
context menu and confirmation before pasting multiple lines. Network Toolkit
adds the requested editable review and reversible Shift+right-click behavior.
https://mobaxterm.mobatek.net/documentation.html

NETWORK TOOLKIT 1.14.0 TERMINAL AND LAYOUT UPDATE

- Password manager: SSH connection profiles can link to credentials stored in a separate protected vault. On Windows the vault is encrypted for the signed-in Windows user with DPAPI. Ordinary settings contain only the credential reference, never the password, private key, or passphrase.
- Save profile is available directly from the connection form. The Password manager and Save/update credential actions are available beside the saved-credential selector.
- Multiline paste review is enabled by default and can be disabled in Terminal Tools. Single-line paste goes directly to the focused terminal.
- Paste history is tracked per session. Terminal controls can jump to the previous or next paste, jump to its start, select or copy its generated output, and optionally include the pasted commands. Copy commands + output is also available explicitly.
- Auto pager is optional and disabled by default. When enabled it can respond to recognized More-style prompts using Automatic, Space, or Enter behavior.
- Ctrl+C behavior remains configurable. In Copy selected text mode, a successful Ctrl+C copy clears the selection; with no selection Ctrl+C is sent to the device. Ctrl+Shift+C remains an explicit copy shortcut.
- Copy line remains available from the terminal context menu. Hovered lines receive a subtle visual highlight, with no overlay button. Clickable highlighted endpoint values include an IPv4 address and port as one copied value.
- Scroll navigation is available beside the bottom of the console in the current release.
- Primary navigation can be collapsed. Configuration panels use compact summaries and expandable secondary settings to preserve usable results space at both low and high window resolutions.
