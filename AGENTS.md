# Network Toolkit 1.17.142 Beta 6 - published application-wide reliability

Published 2026-10-31T21:49:07Z as prerelease 430206177.
Release: https://github.com/TechSavage1989/network-toolkit/releases/tag/v1.17.142
Tagged and merged PR #17 source: 4a2d215ac6fe3b8014a3b386ed436f370035d34c
Exact release workflow 41245081790: build SUCCESS, publish SUCCESS.
Four official assets have GitHub SHA-256 digests. Beta is opt-in;
GitHub stable releases/latest remains v1.17.136.
This release continues the multi-phase plan: Beta 4 performance,
Beta 5 interface, Beta 6 reliability. It reduces unrelated graph
reflows during concurrent tools, releases detached graph DOM and
resize observers, cancels stale divider drags, and improves Editor
comparison layout recovery. No profile, credential, permission,
transport or updater consent semantics are changed.
Known limitation: hosted narrow Editor aligned-comparison and
continuous map-motion CI intermittently fails. This is an opt-in
beta, not a claim of full Editor regression closure or physical
Windows/live-device soak acceptance. Read docs/RELEASE142_HANDOFF.md
and docs/BETA6_RELIABILITY_AUDIT.md before further changes.
Do not replace any existing release tag, asset or checksum.

# Network Toolkit v1.17.142 Beta 6 - application-wide reliability candidate, NOT PUBLISHED

Development PR #17: https://github.com/TechSavage1989/network-toolkit/pull/17
Based on published v1.17.141 Beta 5 and its documentation-only main handoff.
Read docs/RELEASE142_HANDOFF.md and docs/DEVELOPMENT_HANDOFF.md.

Phase 6 scope: simultaneous workflows, long-running refresh, memory use,
cancellation, recovery and cross-tool interaction. Graph workspace now ignores
unrelated mutations, releases detached DOM/observers and stale divider drags,
and preserves scroll/row state across legitimate workspace replacements.
See docs/RELEASE142_HANDOFF.md. Narrow Editor comparison-motion failures from
prior CI remain documented and must not be described as resolved.

The revised fix leaves comparison gaps in their original source-free positions
and gives native CodeMirror line-number gutters blank markers for those gaps.
A bounded native height-map recovery detects grossly inflated virtual height and
allows at most three source-anchored rebuilds per pane/layout generation when
alignment still diverges. The full synthetic Chromium divider matrix passed
36/36 locally; hosted CI is still required before publication.
The initial change moving block widgets after source lines was superseded
after divider and motion CI failures. Preserve source, selection, saved
settings, credentials, network/transfer permissions, opt-in beta updates
and stable Network_Toolkit.exe. Do not publish without exact-source browser,
native, core, package and security checks. Stable remains v1.17.136.

# Network Toolkit v1.17.141 Beta 5 - published Editor and file explorer polish

Published 2026-10-31T19:06:59Z as GitHub prerelease 430129340.
Release: https://github.com/TechSavage1989/network-toolkit/releases/tag/v1.17.141
Tag/exact-source/merged PR #16: e2b8facd51834cfa93e2f587e01cd9d1c2696a89
Release workflow 41232480174: build SUCCESS, publish SUCCESS.
All four assets have SHA-256 digests and the beta feed lists v1.17.141.
Normal stable releases/latest remains v1.17.136. Installation still
requires explicit user approval under App settings > Updates.
Read docs/RELEASE141_HANDOFF.md and docs/DEVELOPMENT_HANDOFF.md.
Earlier tags and release assets remain immutable. Broad Editor and native
Windows CI observations do not imply universal physical device success.

# Network Toolkit v1.17.140 Beta 4 - published application-wide optimization

Published 2026-10-31T18:35:55Z as GitHub prerelease 430114376.
Release: https://github.com/TechSavage1989/network-toolkit/releases/tag/v1.17.140
Exact tagged source/merged PR #15: 9dc5a75696d2e816da8ef13409d4e267f490fdcb.
Release workflow 41229976291: build SUCCESS, publish SUCCESS.
Four assets uploaded with SHA-256 hashes; stable latest remains v1.17.136.
Beta updates are opt-in. Do not replace prior tags or asset bytes.
Read docs/RELEASE140_HANDOFF.md and docs/PERFORMANCE_AUDIT_140.md.
No physical Windows benchmark has been performed. Historical Editor and
detached native SFTP CI failures are not claimed fixed.

# Network Toolkit v1.17.139 Beta 3 - published high-output batching

Published 2026-10-31T15:12:33Z as GitHub prerelease 430009298.
Source/tag: 28d51f149b3246a8b824a90e515f79064b3298b4.
Release: https://github.com/TechSavage1989/network-toolkit/releases/tag/v1.17.139
Exact-source build and publish workflow 41213506790 passed. Four assets uploaded.
Opt-in beta feed resolves 1.17.139; stable releases/latest remains 1.17.136.
Read docs/RELEASE139_HANDOFF.md and docs/DEVELOPMENT_HANDOFF.md before editing.
The binary update retains the stable Network_Toolkit.exe name and does not
auto-install. Earlier Editor comparison-motion and physical-device limitations
are not claimed fixed by the beta.

# Network Toolkit development

Current published prerelease: v1.17.138 Beta 2, published
2026-10-31T07:38:31Z. Source/tag: 8e9052eb14ab9a5a5fc9363b0ac710a36203a133.
Release: https://github.com/TechSavage1989/network-toolkit/releases/tag/v1.17.138
Phase 1 Beta 2 adds frame-coalesced pane splitter dragging, clips
unfitted xterm canvas content within its pane, and preserves a
scrollback viewport fallback during resize. Exact-source release
build/publish passed, four assets exist and stable latest remains
v1.17.136. Beta updates are opt-in; installation is user confirmed.
The 1.17.137 prior prerelease assets and tag remain unchanged.
Read docs/RELEASE138_HANDOFF.md and docs/DEVELOPMENT_HANDOFF.md before editing.
Broad Windows/browser CI pending observations and historical failures
are not claimed fixed; no physical-device performance measurements
were performed.

Current candidate: v1.17.138 Beta 2 continues Phase 1 terminal responsiveness,
based on v1.17.137 Beta 1. The change coalesces pointer-driven divider layout,
clips unfitted canvases to their own panes and retains scrolled-back viewport
anchors when xterm discards a resize marker. It does not change transport,
prompt pacing, credentials, settings, stable executable name or beta opt-in.
Read docs/RELEASE138_HANDOFF.md and docs/DEVELOPMENT_HANDOFF.md before editing.
The stable v1.17.136 and published beta v1.17.137 remain untouched until
exact-source release checks and the new prerelease publication succeed.

Current published prerelease: v1.17.137 Beta 1, published
2026-10-31T07:15:09Z.
Release: https://github.com/TechSavage1989/network-toolkit/releases/tag/v1.17.137
Tag/source: 928443966903e7b3c81ffd161f8a72c38ce2efa7.
Current latest stable: v1.17.136, published 2026-10-31T07:04:53Z.
Release: https://github.com/TechSavage1989/network-toolkit/releases/tag/v1.17.136
Source: 1069e8bbcc1ef97d96ac9d81d93e4d318b341934.
Opt-in to beta updates is OFF by default. Users on v1.17.135
must update to stable v1.17.136 first, then enable Beta updates
under App settings > Updates. Beta v1.17.137 begins Phase 1
responsiveness with scoped terminal fit/layout work. The exact
prerelease build and publication passed with all four assets,
and stable releases/latest remains v1.17.136. Separate pending
browser/Windows CI and historical failures are not claimed fixed.
Read docs/RELEASE137_HANDOFF.md, docs/RELEASE136_HANDOFF.md
and docs/DEVELOPMENT_HANDOFF.md before making new changes.

Current candidate: v1.17.137 Beta 1 begins Phase 1 responsiveness work,
based on the v1.17.136 beta-channel bridge. This prerelease is opt-in only
through App settings > Updates. It coalesces redundant terminal pane resize
layout work without altering session behavior or the native updater.
Read docs/RELEASE137_HANDOFF.md and docs/DEVELOPMENT_HANDOFF.md.
Do not describe this beta as stable or published before the GitHub
--prerelease flag, assets, hashes and update feed are verified.

Current candidate v1.17.136 is a stable beta-channel compatibility bridge
from the published 1.17.135 release. It adds an off-by-default opt-in for
future GitHub prereleases and preserves the normal stable-only updater,
release checksums, Windows EXE and saved application settings. The first
Phase 1 beta will follow separately as v1.17.137, flagged prerelease.
See docs/RELEASE136_HANDOFF.md and docs/DEVELOPMENT_HANDOFF.md.
Do not call the bridge published until release/feed/assets verify.

Current published version: 1.17.135. Published 2026-10-31T06:37:53Z.
Source/tag: 2cf90ac3216ee5f3e3ce59d9b0959bb4109faba6.
Release: https://github.com/TechSavage1989/network-toolkit/releases/tag/v1.17.135
Fixes Copy/Select for Last paste and Last command: Include commands shows
each submitted paced-paste command once without synthetic numbering or
duplicate device echoes; disabling it retains captured device output only.
The release build and publication passed, with four official assets,
unchanged v1.17.134 assets and a consistent latest updater feed.
Separate browser/Windows CI observations are retained without claiming
unrelated failures were fixed. Read docs/RELEASE135_HANDOFF.md and
docs/DEVELOPMENT_HANDOFF.md before continuing development.

Current published version: 1.17.134. Published 2026-10-31T06:12:51Z.
Source/tag: 71750e8b48512735716a6f86146dd9722deff30a.
Release: https://github.com/TechSavage1989/network-toolkit/releases/tag/v1.17.134
Fixes Last paste and Last command for prompt-paced SSH multiline pastes,
including associated output and macro save shortcuts. It preserves
accepted-command-only tracking, prompt pacing, credentials, ownership and
the portable updater. The release build/publish jobs succeeded, and latest
GitHub feed and all four asset records are verified. Review CI retained an
unrelated Windows detached SFTP acknowledgment failure; earlier Editor
and PowerShell CI issues are not claimed fixed.
Read docs/RELEASE134_HANDOFF.md and docs/DEVELOPMENT_HANDOFF.md.

Previous published version: 1.17.133. The exact release metadata, source,
original four assets and checksums remain unchanged.

Current published version: 1.17.133. Published 2026-10-31T05:27:14Z.
Source/tag: 2b320d7e89f47d3bb1652e273f37277059957671.
Tree: c0ae209c5a127c6bdd1006d3a5a772826f6e8c7b.
Release: https://github.com/TechSavage1989/network-toolkit/releases/tag/v1.17.133
Adds adjacent Disconnect/Reconnect split actions and independent optional
confirmations for Reconnect all, Disconnect all and Close all tabs. Defaults
remain on; each can be restored in Terminal display settings. Profiles,
credentials, window ownership and updater safeguards are unchanged.
The exact release build and publish workflow succeeded; all four official
assets exist, latest feed/tag/source agree. Source CI passed all 14 jobs.
Duplicate PR CI retained a Windows PowerShell echo failure, and post-merge
main CI retained an unrelated Editor browser failure. Do not call either fixed.
See docs/RELEASE133_HANDOFF.md. The prior 132 release remains unchanged.

Read docs/DEVELOPMENT_HANDOFF.md before editing or building. Start from latest main
and preserve working features, settings and previous release safeguards.
For all future releases follow docs/RELEASE_NOTES_STYLE.md: What's New must
describe concise user-visible benefits; Additional Details gives specific
technical behavior, scope, safeguards and limitations.

Previous published version: 1.17.132, released 2026-10-31T04:50:09Z.
Source/tag db432733d9b5065d782fbd0685c5fcf78a6011e4;
tree 2e66bdbddc63ee594047b124708a083c394a15b4.
Release: https://github.com/TechSavage1989/network-toolkit/releases/tag/v1.17.132
Includes current-window Reconnect all and user-benefit What's New summaries
with expandable technical Additional Details. Read docs/RELEASE132_HANDOFF.md.
Review PR #7 retained one unrelated narrow Editor comparison-motion failure;
do not claim Editor alignment was fixed. Prior 131 assets remain unchanged.

Previous published version: 1.17.131. Published 2026-10-31T04:12:11Z;
source/tag 9152e41f278f35719cff18993730f25a25050237,
tree 9d29bdb49a7819e3e434448bf3ebc70e2a51d0b5.
Release: https://github.com/TechSavage1989/network-toolkit/releases/tag/v1.17.131
The exact release build and GitHub publication succeeded, 4 assets exist,
and latest release/feed resolves 131. Review CI:13 pass/1 retained existing
narrow Editor comparison-motion failure. No live device speed measurement.
Read docs/RELEASE131_HANDOFF.md before further work. All 130 release bytes,
assets and tags remain unchanged.

Read docs/DEVELOPMENT_HANDOFF.md before editing or building. Start from latest main
and preserve working features, settings and previous release safeguards.

For every future release, follow docs/RELEASE_NOTES_STYLE.md. In-app "What's New"
must briefly explain practical user-visible benefits, not engineering jargon;
"Additional Details" must provide accurate specific technical behavior, scope,
compatibility and limitations. Use the exact two headings in release notes.
The GitHub release body is shown by the updater, so validate both disclosure
sections before publishing. Preserve historical published release notes.

Previous published version: 1.17.130. Published 2026-10-31T03:13:29Z;
actual public download/source/feed proof 03:14:21.181667Z, independent 03:15:04.117528Z.
Source/tag 77ceeec0b9b258dc7e3a1dfc29f9992ad3d5d884;
tree 6b88d9a83a666f2363c496b21e7ec45cb2c6fd9a.
Release: https://github.com/TechSavage1989/network-toolkit/releases/tag/v1.17.130
Automatic blank-profile naming, five type badges and new-Recent revision protection
passed 1,477 JS / 42 Python, full Go/race/compile, all 14 accepted PR jobs and actual
browser/native checks. Public 1,244-source/1,251-member bytes/feed verified; prior 129
unchanged. Synthetic Telnet/serial fixtures are not live SSH/physical COM/full restart.
Narrow 390px sidebar can horizontally overflow; no simultaneous-control claim.
Original failed jobs/error pixels retained; final review push legacy editor transfer
failed. Final main duplicate 41163257051 passed all 14 jobs; API updated 03:31:00Z,
confirmed 03:31:50Z. Earlier failed observations remain retained.
See docs/RELEASE130_HANDOFF.md. This source-only closure changes no release bytes.

Previous published version:1.17.129. Published 2026-10-31T01:59:27Z;
actual public downloads/source/feed verified 02:01:35.447379Z and independently
02:02:26.029257Z. Source/tag0de3c5708ce7885ae105ff5ee3601b18336941d4;
tree 00ab469d2ef3f2bff8da7d8ecf94474b2a1bdbee.
Release: https://github.com/TechSavage1989/network-toolkit/releases/tag/v1.17.129
Includes manual/learned-name corrections and bounded pane/PTY resize scheduling.
Version 128 remains untagged/unpublished, subsumed by129; no128 failure was waived.
Local 1,451JS/42Python, all 12 accepted PR jobs, actual browser/native runtime and
1,233-source/1,240-member public package/feed proof passed. Prior 127 unchanged.
Original failed runs remain. Post-merge duplicate41157694962 completed02:15:36Z:
11 passed/1 legacy editor-motion failure (p95 offset4.140625px); no all-main pass
or unrelated fix claimed. See docs/RELEASE129_HANDOFF.md before proceeding.
Native fixtures use synthetic loopback Telnet, not live SSH/universal TUI proof.
This source-only closure changes no published bytes, tags or access.

Previous published version: 1.17.127. Published 2026-10-30T23:12:03Z;
actual public downloads/source/feed verified 23:13:27.407082Z,
independently rechecked against fresh public receipts at 23:14:28Z.
Source/tag 3085e0729be9f7cbbe372f9959c8985f5255b220;
tree ca4095f5bad98c1f2b3c64232a5582062a3fa9d1.
Release: https://github.com/TechSavage1989/network-toolkit/releases/tag/v1.17.127
Paced paste preserves literal commands with Pause/Stop and uncertain-start
protection. Learned CLI names appear above smaller endpoints, with endpoint-only
fallback and saved identity/authentication unchanged. All nine exact-source review
jobs passed, plus 1,407 local JS and 42 Python tests. Actual browser/native WebView2
and 1,213-source/1,220-member public package/feed proof passed; prior126 unchanged.
Native coverage uses synthetic loopback Telnet, not a live SSH device. Original
failed duplicate observations remain; see docs/RELEASE127_HANDOFF.md, including
the final duplicate main cancellation metadata. This source-only closure changes
no release bytes, access or general authorization requirements.


## Source and release destination

TechSavage1989/network-toolkit is public and must remain public. The owner
requested the128 naming correction/prompt survey and then the129 pane-resize
correction, following the published127 task. Requested completed Toolkit
updates use the existing source/release destination under the applicable current
authorization requirements. This record is not a blanket future approval rule;
future actions
must satisfy the applicable authorization requirements. Never change repository
access/visibility, expose client inputs, secrets or credentials, or replace an
already-published tag/asset without specific authorization.

Previous published version: 1.17.126. Published 2026-10-30T19:14:20Z;
actual downloads/source/feed verified 19:15:13Z, independently 19:15:42Z.
Source/tag ed90d1e5d4769fc4bbc97484bf7e263d1794fd4e;
tree 58e9852b3b8b74c580f6d29a0cf71c59abbb7e91.
Release: https://github.com/TechSavage1989/network-toolkit/releases/tag/v1.17.126
Actual Windows administrative inventory is repaired by separate JSON/diagnostic
streams and bounded cold-start budgets. Published125 failed its actual query;
corrected read-only cold inventory, authenticated GET and preflight passed.
Unknown state, identity/revision, UAC and cancellation safeguards remain. No live
NIC changes or visible UAC transition were tested. Core/security/Windows, seven
synthetic UI groups and 1,201-source/1,208-member public package/feed proof passed.
Read RELEASE126_HANDOFF.md for exact terminal outcomes and retained limitations.
No unrelated fixes or permission changes are implied by this historical closure.

Previous published version:1.17.125. Published2026-10-30T18:24:37Z;
actual downloads/source/feed verified18:25:29Z, independently18:25:56Z.
Source/tag0f5b46718ad133d2ae740112b797bbd46eea3449;
tree94cb3eaa9ea575a4107973c22518691d9b3cf535.
Release: https://github.com/TechSavage1989/network-toolkit/releases/tag/v1.17.125
Post-release user evidence shows administrative-state detection can leave both
new actions unavailable. A separate 126 investigation reproduced a query timeout
and stderr-contamination risk. Do not infer real-device success from mocks.
Only selected-adapter Enable/Disable is added, retaining Restart, disabled-adapter
selection, administrative/link distinction, current identity and one-action UAC.
Seven rendered groups/12 simulated requests,12 Windows mock groups, core/security/
package acceptance passed. No live NIC mutation or visible UAC was tested.
Public1,198-source/1,205-member/PE/manifests/feed and prior124 immutable assets
verified. Read docs/RELEASE125_HANDOFF.md for exact original/duplicate terminal
outcomes and the retained detached keyboard-resize/other historical observations.
No unrelated fix or broad permission is implied; this closure changes no binaries.

Previous published version: 1.17.124. Published 2026-10-30T16:50:53Z;
actual public downloads/source/feed verified 16:51:36.213209Z, independently 16:51:49Z.
Source/tag 043851be8c8a23fa4711699855d74aa4c01afce1;
tree e88776b3f1748f1b47b6f338c5c40e1542d4d2b6.
Release: https://github.com/TechSavage1989/network-toolkit/releases/tag/v1.17.124
Mature Recent graph endpoints passed 48 advancing frames plus legacy curve checks;
TCP Ports has a distinct icon. Six network maintenance actions preserve strict
scope/current identity and JIT UAC, with static/unsaved/disruption safeguards.
No live DHCP/DNS/restart/reset or visible UAC testing was performed: network
validation is mocked/construction and intercepted UI only. Final core 1,384 JS,
42 Python / eight Go packages and security/Windows/package gates passed; see exact
terminal CI outcomes in docs/RELEASE124_HANDOFF.md: review all seven passed;
duplicate six passed/one existing comparison-motion failure remains. Public 1,193-source /
1,200-member/manifests/PE/feed and prior 123 immutability verified. Preserve all
historical comparison/Alt-transfer/Hosts/gear/UAC observations; no unrelated fix
or broad permission is implied. This source-only closure changes no release bytes.

Previous published version: 1.17.123. Published 2026-10-30T13:39:16Z;
public downloads/source/feed verified 13:41:28.278568Z.
Source/tag c0530bbb710e1023c68b835a565fbf89e19d74a8;
tree 71d2000215acbb2d027df22d47c090b31720c5f9.
Release: https://github.com/TechSavage1989/network-toolkit/releases/tag/v1.17.123
Terminal SFTP direct native copies passed eight main/workspace cases and ten
existing SFTP/Editor cases, with a real122 Save-dialog negative control. Full
Windows/core/security/dependency/package passed; public source/manifests/PE/feed
and prior122 immutable assets verified. Review41063349840: all seven SUCCESS; last job completed 2026-10-30T13:46:46Z.
Duplicate41064715467: six SUCCESS/one FAILURE (known Windows Alt-editor transfer); last job completed 2026-10-30T13:56:30Z.
Duplicate scoped Terminal8/Editor10 passed; later known Alt-editor transfer failed.
Preserve earlier gear-keyboard/comparison/Hosts/Alt-transfer observations and UAC
limits; no unrelated fix or universal native reliability claim. Detached workspace
files are own-session read/export only; single-session child sidebar is unchanged.
Read docs/RELEASE123_HANDOFF.md and DEVELOPMENT_HANDOFF.md. No future authority
or repository-access changes are granted by this historical record.

Previous published version: 1.17.122. Published 2026-10-30T12:27:30Z;
public downloads/source/feed verified 12:30:55.984763Z.
Source/tag 60b02cae25c1bfaf14d0c15a038706d193c99a37;
tree 5913c34e268e2ecbdae773854ded61f14206d5bb.
Release: https://github.com/TechSavage1989/network-toolkit/releases/tag/v1.17.122
Focused native file-drag repair passed 40 repeated actual Windows cases and ten
final-version cases, retaining cancellation/source/dirty/undo/approval guards.
Review 41021649359: five SUCCESS/two FAILURE (Alt-editor transfer and comparison
gutter alignment); later skipped native stages are not passes. Neither issue is
called fixture-only or fixed. Release build/publish and public proof passed:
1,178 exact source files, reviewed EXE/portable identity, manifests, PE/resources,
updater allowlist and prior 121 immutability. The local broad app race was killed,
not passed. Duplicate 41055236523: five SUCCESS/two FAILURE
(Windows Alt-editor transfer and comparison motion), last job completed
2026-10-30T12:44:55Z.
Read docs/RELEASE122_HANDOFF.md and DEVELOPMENT_HANDOFF.md for exact limits.
Finite synthetic Explorer-folder evidence is not universal timing, Desktop Shell,
cross-integrity, visible UAC or live-user-device proof. Preserve all old releases,
remote approval/owner backend and existing export/text limits. This historical
record is not authorization for unrelated work or access changes.

Previous published version: 1.17.121. Published 2026-10-30T05:45:41Z; actual public verification 05:48:05.062831Z.
Release 428692620: https://github.com/TechSavage1989/network-toolkit/releases/tag/v1.17.121
Source a3a96b0e649c1560b919184e8ca336fda2dcc11b;
tree e2336287ecc5abab564e94e15a7b4ac1858192e0. Compact sidebar, validated internal
file opens and ordinary local/authorized-SFTP native copy retain security and
ownership bounds. All 22 focused cases and exact core/security/dependency/full
Windows checks passed; three legacy browsers were running at promotion.
Review 41009283786: six SUCCESS/one FAILURE (comparison motion), last job completed 05:54:57Z.
Duplicate 41010025845: five SUCCESS/one Windows FAILURE/one browser CANCELLED, last job completed 06:04:03Z.
Public package/feed/prior 120 proof:
All four actual public downloads match the reviewed bytes; 1,170 exact source
files/1,177 portable members, manifests, PE 121/nine canonical icons/12 resources,
update four-file allowlist and latest feed/tag/main/notes are verified. Prior 120
release/tag/body/original four assets remain unchanged.
Same-source duplicate first child-file native drag failed after main five passed;
detached delivery reliability remains unresolved. No materialization/write occurred,
but cause is unknown, not proven fixture-only or premature release. Original ten
passes do not erase it; no rerun/rebuild/repair is claimed.
Read docs/RELEASE121_HANDOFF.md for exact run history and limits. Explorer folder
destinations are tested, not a separate Desktop Shell surface/cross-integrity/
arbitrary-app promise. Remote approval/owner backend, 2 GiB export and 16 MiB text
limits remain. Retain comparison/Hosts/Alt-transfer/UAC caveats and failed
observations; this docs-only record grants no unrelated work or future release.

Previous published version: 1.17.120. The shared non-tab dragstart cancellation
and Condensed long-window baseline-density defects are corrected in the scoped
published source/tag 45ee25b15c1c88fdb7c77b2f06057d38ae6b5357;
tree 6e2206995eac69d43f356aea62cce14c332a4321. Public assets/source/feed verified
2026-10-30T04:09:16.444676Z; prior 119 remains unchanged. Actual file-row/graph
acceptance and exact-source core/security/dependency/full Windows/package passed.
Review 40999859524 terminal: all seven SUCCESS, last job completed 04:12:48Z.
Duplicate 41000889198 terminal: six SUCCESS/one FAILURE (Windows Alt-editor transfer), last job completed 04:24:00Z.
The duplicate Windows Alt-transfer failure is unresolved; do not call the whole
editor-drag asset unchanged or the failure fixture-only. Read RELEASE120_HANDOFF.md
under docs/ for exact scope, evidence and retained comparison/Hosts/UAC limits.
Native Explorer outward copies retain their prepared-copy workflow. No new
transfer/authentication privilege or future publication is granted by this record.

Previous published version: 1.17.119. Graph time windows/follow controls, elapsed
HH:MM:SS/12-hour defaults and direct detached-local Files are published. Source/tag
a4b8c56f4c8692d4676d508381ec4c5a2b504aee; tree 9bd09a18eab01de56176e072bb0a06b4c969ba81.
Four public assets/source/feed verified 2026-10-30T03:22:27.122867Z; prior118 unchanged.
New graph/local/native, core/security/dependency/package checks passed. Final
review ended six SUCCESS/one FAILURE (known narrow wrapped-map comparison).
Duplicate-main 40996639878: five SUCCESS/one FAILURE (native Alt-editor transfer)/one CANCELLED (long browser), last job completed 03:38:53Z.
Read docs/RELEASE119_HANDOFF.md for exact failures and limits. A genuine
post-release non-tab dragstart cancellation defect was reproduced read-only;
119 does not fix it, and separate120 work must not be confused with this source.
The owner backend remains required for detached-local Files; remote approvals,
credential/lease guards and Windows permissions remain. At narrow widths Custom
may wrap Curve within existing responsive controls, not preserve toolbar height.
This docs-only record authorizes no unrelated fix or future publication.

Previous published version: 1.17.118. Editor Connect is first and existing
Connect sidebars share a distinct plug icon; selected-panel preferences and
connection/security/ownership handlers are unchanged. Source/tag
2be7a62d18865275bd323c26b07f1520b689b560; tree e4fbfeaed734c5f556cbbab4e4ed3a1ce9202d52.
Public assets/source/feed were verified at 2026-10-30T01:08:14.010Z. All 14 focused
rendered cases and exact-source core/full-Windows/package checks passed. First
review ended five SUCCESS/two FAILURE; same-source package review all seven
SUCCESS does not erase the earlier native Alt-transfer and resize-keyboard
failures. Duplicate-main outcome: six SUCCESS/one FAILURE (native Alt-editor transfer), last job completed 01:11:35Z. Read RELEASE118_HANDOFF.md
under docs/ for exact outcomes and unchanged UAC/legacy limits. No unrelated
fix, access change or future publication is authorized by this historical record.

Previous published version: 1.17.117. Vertical Files/Connections rail, drive-wheel
scrolling and literal local-shell folders/explicit per-session administrator
launch are published and public assets/feed verified. Source/tag
51aa026f07e201979b3adb44d048c66c06508826; tree 793cc319121ed3586654a580a20738852b442a83.
New-feature/native/core gates passed. Normal review 40970618456 ended five SUCCESS/
two FAILURE: existing comparison alignment and Hosts-entry focus. Scoped
publication retains both disclosed caveats; no all-seven-green or fixed claim.
Real high-token helper passed on an already-elevated runner, not a visible UAC
transition/cross-account test. Read docs/RELEASE117_HANDOFF.md for exact evidence.
Duplicate main 40973657061 also ended five SUCCESS/two FAILURE at 23:48:49Z:
Windows Alt-editor transfer and editor motion failed; Hosts/long browser passed.
No unrelated fix or
future publication is authorized by this record. Preserve all 116/historical
failures, published tags/assets, repository access and credential/ownership guards.

Previous published version:1.17.116. The expedited Terminal-only interaction
hotfix is published and public assets/feed verified. Source/tag
f13c4df906a0ed8124308f8f95db571a7a9be532; treec0127280d174967bc15170537d4f2fe05f8ef7c3.
Focused actual Terminal/Windows/application/security/dependency/package gates
passed before promotion under the expedited validation plan for the approved
publication. The unchanged seven
workflow jobs continued; background editor motion failed two detached narrow
wrapped-map alignment cases. Root cause is not established; do not call them
fixture-only or claim all seven jobs passed. Duplicate-main Windows native editor
also failed. Both runs ended six SUCCESS/one FAILURE; read
docs/RELEASE116_HANDOFF.md for exact terminal outcomes and limits.
No comparison fix, repository-access change or asset replacement is included.
The deferred115 historical handoff is included in this six-doc closure; its
review and duplicate jobs all passed. No separate115 docs push is claimed.

Earlier published version:1.17.114. The authorized comparison/Files/Terminal/graph
batch is published and verified. Source/tag fd6cbbf287738320b51957d176dd2e63c63eece9,
tree 0f5bd938f3bb54b6ef6892e3fc4d4eaae33a5293. All seven exact-source review Checks
and automatic Release passed; public assets,1,109 source files, PE icons and
latest feed are verified. Read docs/RELEASE114_HANDOFF.md for exact evidence,
limits and duplicate-main closure. The duplicate main run recorded one motion
latency-bound failure; its cause is unresolved and is not labelled fixture-only. Preserve all published tags/assets, repository
access and existing credential/native/pager safeguards. This docs-only record
is not another build or authorization for unrelated work.

Previous published version:1.17.113. The authorized combined editor/graph/activity/
concise-interface batch is published and verified. Source/tag31f27994b9a3f7bae4442c255f63a65c8e21e0c8,
treea95bbbce6715f42b73c919c8e1a91aaa6c9b4619. All seven exact-source review
Checks and automatic Release passed; actual sustained comparison/gutter/native
close evidence, four public assets,1,070 exact source files, reviewed binary
identity, icons and latest feed are verified. Read docs/RELEASE113_HANDOFF.md for
behavior, limits and duplicate-main closure. This docs-only handoff is not a new
build and authorizes no additional feature. Keep published tags/assets immutable.

Previous published version: 1.17.112. Independent editor views and the Windows/SFTP
sidebar are published and independently verified. Source/tag: 4d05f6a5a4eb1c315d0a19ccf2eb8daedf2dcb1c;
tree 3f9278fe6a5a3b1781d71efbd9354e4decf7ac8e. All seven review Checks, automatic Release and
all seven duplicate main Checks passed. Four public assets,1,017 exact source files,
reviewed EXE/portable identity, PE icons and latest feed are verified. Read
docs/RELEASE112_HANDOFF.md and docs/EDITOR_VIEWS112.md for behavior and limits.
This docs-only record is not another build. Preserve every old tag and asset.
The user prohibits local desktop automation or test windows without advance
approval for a scheduled session. Never stop normal app sessions or lose unsaved work.

Previous published version: 1.17.111. Compact interface and two-document comparison
maps are published and independently verified. Source/tag: bf414aec01a63a4b492cdfc62f32fa52910f3b69;
tree 250294f0ece6dcc3397a9f9ed899770015da3523. All six review Checks, automatic Release and
all six duplicate main Checks passed. Four public assets,1,008 exact source files,
reviewed EXE/portable identity, PE icons and latest feed are verified. Read
docs/RELEASE111_HANDOFF.md and docs/EDITOR_LAYOUT_MAP.md for behavior and limits.
This docs-only record is not another build. Preserve all old tags/assets.
The user prohibits local desktop automation or test windows without advance
approval for a scheduled session. Run visual/native tests in isolated environments;
never stop normal app sessions or discard unsaved work.

Previous published version: 1.17.110. The focused graph/editor correction batch
is published and independently verified. Source/tag:
a71f98c4f418077cfeefa5546cfb77de1d8e7742; tree
c0f4ee07ab4687d50c5d4b2e5afc2bdf6a1f545b. All six exact-source Checks and automatic
Release passed; actual temporal frames/native drag/maximize, four public assets,
1,005 source files, reviewed binary/package identity, PE icons and latest feed
verified. Read docs/RELEASE110_HANDOFF.md and DEVELOPMENT_HANDOFF.md for limits
and duplicate-main closure. This docs-only record is not another build. Preserve
all tags/assets, including the separately authorized109 Toolkit-Icons.zip extra.

Previous published version: 1.17.109. The authorized vertical-space layout batch
is published and independently verified. Source/tag:
d3494a879604fb2523e0867ad1fb175a2f9abbf5; tree
9dda3650d5c794a4fc1c734976619819ce87fbfa. All six exact-source Checks and automatic
Release passed; actual normal/narrow/native editor evidence, four public assets,
992 exact source files, reviewed package/binary identity, PE icons and latest feed
were verified. Read docs/RELEASE109_HANDOFF.md and DEVELOPMENT_HANDOFF.md for
measured gains, limits and duplicate-main closure. This docs-only handoff is not
a new build. Preserve every published tag/asset and all existing safeguards.
Post-release graph-outline flicker and unsupported editor tab dragging are open
follow-up issues; see the109 handoff. Do not treat synthetic checks as user-runtime
acceptance or mix a later fix into the immutable109 source/tag/assets.

Previous published version: 1.17.108. The authorized activity/bookmark/graph/editor
batch is published and independently verified. Source/tag:
59a82b13bb31899206d383ea58166ec9a4f4faf0; tree
c2e0b5db0b51ef4501bf4e72d5ccd61db671483e. All six exact-source Checks and automatic
Release passed; actual hosted editor windows/close consent, normal/narrow pixels,
four public assets,981 exact source files, reviewed binary/package identity and
latest feed verified. Read docs/RELEASE108_HANDOFF.md and
DEVELOPMENT_HANDOFF.md for evidence, limits and duplicate-main closure. This
handoff-only change is not a new build. Preserve all published tags/assets,
repository access and existing credential/native/host-key/pager safeguards.

Previous published version: 1.17.107. The authorized discovery/appearance/capture/
file-workflow batch is published and independently verified. Source/tag:
bafabd88f9dd728acf45862226995e654821082d; tree
68f7aafdec7652507134ef1ae358bb96ad8298b7. All six exact-source Checks and automatic
Release passed; actual hosted Explorer/OLE, new-feature/legacy pixels, four public
assets, 937 source files, reviewed binary/package identity and latest feed verified.
Read docs/RELEASE107_HANDOFF.md and docs/WORKFLOWS107.md for scope, compatibility
limits and duplicate-main closure. This later [skip ci] handoff is not a new build.
Keep every published release immutable; this record authorizes no new feature.

Previous published version: 1.17.106. The authorized transfer-location/listener
copy controls and macro Library/recording fixes are published and independently
verified. Source/tag: b8be5353877911fa422e69880f6b7363cda47509;
tree da3fd8490a74aa5d80be4b6708b98e3bedeabc26. All five exact-source required
Checks and automatic Release passed. Four public assets, 903 source files,
reviewed EXE/portable identity and latest update feed verified. Read
RELEASE106_HANDOFF.md and DEVELOPMENT_HANDOFF.md under docs/ for full evidence,
limits and duplicate-main closure. This handoff-only commit is not another build.

Published 105 banner/discovery fixes and 104 vault, pager/defaults/archives, graph
and canonical icon safeguards remain intact. Read docs/CREDENTIAL_SECURITY.md,
docs/SECURITY104_HANDOFF.md and docs/CONDENSED_GRAPH_SCALING.md as applicable.
No new feature/release or live user devices/credentials access is authorized by
this historical handoff. Keep every published tag/asset immutable.

The user expressly chose master-password encryption with Windows-account
remembered unlock across restarts and reboots after same-user-risk disclosure.
Remember stores a DPAPI-protected random key, not the password. Established
sessions stay connected on manual Lock and forget; new saved-credential uses
are blocked. Recovery is optional and migration is explicit native setup.
The retained old DPAPI vault requires separate user-confirmed cleanup. No actual
user vault/settings access or migration is authorized by this development record.

The prior 104 reviewed/published source is 554f360613aa960a28ee459a44018da39a0bbcff.
This handoff-only closure does not replace published assets or authorize a new
feature/release. Keep future work scoped, preserve explicit settings and existing
release safeguards, and coordinate concurrent files before editing.

Never replace old tags/assets, change access or write the retired update feed.
Record honest retention/legacy/partial-summary limits; no unbounded raw history.
Final handoff-only records use [skip ci].

## Release-note organization

Keep only the current version's `RELEASE_NOTES_<version>.txt` at the repository
root: release validation, packaging and publication require that path. Use the
version in `internal/model/model.go` (and its matching UI version), not a guessed
filename. When an authorized build advances the version, move the prior note
unchanged into `docs/releases/`, update that directory's index and any references,
and leave the new current note at root. Preserve every historical note; do not
recreate the release-note wall in the root directory. This organization does not
authorize a build, new release, tag move, asset replacement or access change.

## Compact repository layout

Keep application Go source and tests together in `cmd/network-toolkit/`; put
new developer and user guides in `docs/`. `resources.go` at root owns the
embedded UI assets; preserve its embed patterns and paths. Build the command
with `go build ./cmd/network-toolkit`, not `go build .`. Use the root build
wrappers and read `docs/DEVELOPMENT.md` for tests and packaging. Update all
consumers when moving paths. Keep release-note rotation above unchanged.

The exact tracked root allowlist is enforced by `scripts/repository_layout_test.py`
in existing Checks and release CI. Put new files under the documented directories,
not at root. Intentional new root entries require a reviewed allowlist change and
justification; the sole current release note is derived from the application version.
