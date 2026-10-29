# Network Toolkit 1.17.117 published: sidebar rail and local-shell options

Published 2026-10-29T23:33:10Z:
https://github.com/TechSavage1989/network-toolkit/releases/tag/v1.17.117
Release ID: 428498861. Public verification 2026-10-29T23:35:30.794Z.
Exact reviewed source: 51aa026f07e201979b3adb44d048c66c06508826.
Tree: 793cc319121ed3586654a580a20738852b442a83.
Local equivalent: 2afcf1a14dc67331ea3dbfaa4bc2ff9a68f8d6f6.
Baseline main: 525c741f51ae4b0b941fa36a98539a0960640718, the completed 116 documentation
closure, after published 116 source f13c4df906a0ed8124308f8f95db571a7a9be532.

The user approved 117 source upload/publication at 2026-10-29T21:49:53Z and the
specific interrupted source-upload retry at 22:57:44Z. Publication used
expected-head/non-force promotion and unchanged repository visibility/access.
This is not authority for unrelated work or future releases. Private references
were inspected locally; no private screenshot/configuration is a public fixture.

## Delivered scope and boundaries

- Editor Files/Connections uses the existing Terminal vertical rail rather than
  a heading row. Shared rail 43 px, buttons 42 px, minimum button height 65 px; visible
  Connect label has accessible Connections name. Existing Windows/SFTP browsing,
  saved/active connections, credentials/host verification and detached approval
  remain on their existing paths. The rail remains visible when content collapses;
  selected-tab click, keyboard selection, pointer/keyboard resizing and collapsed
  content inertness preserve width, focus, forms and dirty documents.
- Wheel over an overflowing drive strip scrolls horizontally. Pixel/line/page
  deltas are handled; edge chaining, horizontal touchpad input, modifiers/zoom,
  keyboard drive selection and ordinary file-list wheel behavior are retained.
- CMD and PowerShell accept an existing absolute starting directory or the
  authenticated directories-only picker. Empty means the account home folder.
  Paths are validated/canonicalized and passed literally as CreateProcessW's
  current-directory parameter, never concatenated into a shell command. Invalid,
  unreadable and non-directory paths fail visibly. CMD UNC folders are refused;
  no drive mapping is silently created. An administrator-only unreadable path can
  fail ordinary-user validation before UAC; no elevated read fallback is added.
- Run as administrator is default-off and per local-shell session. Only explicit
  Connect/Reconnect can launch via ordinary Windows runas. Loading a profile,
  browsing a directory or reading status never launches/elevates. Cancel does not
  automatically retry. The option does not elevate the rest of Toolkit or alter
  UAC/system policy; a Toolkit already launched elevated remains that way.
- Admin badges reflect the actual shell token, not the checkbox. The dedicated
  same-image helper starts before any HTTP/settings/vault initialization and
  permits fixed system CMD/PowerShell only. A random first-instance local named
  pipe, ACL, exact retained peer PID/image checks, strict versioned bounded frames
  and one startup grant protect the channel. Thereafter only input/resize/close
  are allowed. EOF/cancel/parent loss close the shell job. It is an interactive
  administrator shell, not a sandbox or rollback of intentionally installed
  services/scheduled tasks. Late consent cannot recover a canceled startup grant.
- Existing attach/detach keeps the transport and does not request UAC again;
  explicit reconnect creates a new process. The 116 click/X/reorder fixes remain.
  No comparison, graph, file-transfer capability or close/ownership change is
  included. Existing 116 unresolved comparison/Alt-drag findings remain recorded.

Guides: EDITOR_SIDEBAR117.md and TERMINAL_LOCAL117.md. The sidebar guide contains
its original implementation-stage candidate/unexecuted-test snapshot; the final
runtime/publication record in this handoff supersedes that historical status,
not its behavior or safeguards. All historical release notes/tags/assets remain.

## Exact-source observed acceptance

Review Checks 40970618456:
https://github.com/TechSavage1989/network-toolkit/actions/runs/40970618456
Review is terminal FAILURE, with five SUCCESS/two FAILURE:
- check 123786722040 SUCCESS.
- browser-security 123786722006 SUCCESS.
- dependency-security 123786721924 SUCCESS.
- windows-terminal 123786722137 SUCCESS.
- browser-workflows107 123786721996 SUCCESS.
- browser-editor112 123786721604 FAILURE: continuous-motion alignment.
- browser 123786721905 FAILURE: Hosts editor initial input focus.

Both exact failures are described below. All new 117 feature/native/security/
package checks passed. At 23:30 UTC release handling retained the scoped publication
with both disclosed caveats; this was not a new user quote waiving CI, not a
workflow/gate change, not an all-seven-green assertion, and not a comparison or
Hosts fix. Expected-head main promotion 525c741f → 51aa026f was non-force and verified
at 2026-10-29T23:30:56.340Z. Automatic Release 40973657237 succeeded and public
verification completed. Duplicate-main Checks 40973657061 also ended five
SUCCESS/two FAILURE; exact terminal outcomes follow below.

Completed evidence on this exact source:
- Files 117 eight paired main/detached 1440/390 light/dark cases, 24 behavior groups:
  actual before/after explorer gain 39 px in every case, preserved total width,
  shared 43 px rail/42 px buttons/minimum 65 px button height. Actual drive-wheel,
  keyboard, collapse, resize and wide-narrow-wide interactions passed. Reviewers
  inspected normal/narrow screenshots and authenticated artifact identity.
- Terminal 117 four main/separate-window 1280/390 synthetic browser cases and eight
  screenshots passed default-off/no-auto-launch, invalid-folder retention,
  simulated UAC cancel/retry, pending cancellation and actual-token badge states.
  Existing 116 four real click/X/reorder cases also passed without reconnect;
  fresh post-drag close presses 7.6-9.8 ms after release.
- Actual Windows ordinary CMD/PowerShell literal cwd/token cases 0.07/0.55 s;
  explicit approval/late-cancellation launcher spy 0.01 s. Real separate same-image
  helper executed (not skipped): CMD 0.12 s/PowerShell 0.63 s. Parent and child already
  elevated, cwdConfirmed=true, UACInvoked=false. This is NOT proof of a visible
  UAC dialog, an unelevated-to-elevated transition or cross-account behavior.
- Windows vault/DPAPI/ACL/credential boundaries passed; real three-route WebView2
  bootstrap 9.71 s, native vault captures 0.34 s, capture navigation/cancel spy 0.02 s,
  dirty-close 11.10 s, editor transfer 93.93 s, Explorer files 15.91 s and Terminal
  transfer 26.87 s passed. New helper/cwd tests did not skip. These synthetic hosted
  tests do not prove live-user devices, real privileged workflows or reboot/install.
- Combined local JavaScript 1,298/1,298 and 42 Python tests passed. Relevant Go-race
  application/terminal/desktop/vault packages passed during integration. Exact
  final application/Windows tests and package checks passed in the core jobs.
- Full/runtime npm audits zero findings; Linux/Windows Go scans zero reachable or
  imported-package findings. Unused OpenPGP module advisory GO-2026-5932 remains.
  The pinned offline comparison bundle reproduced; no external document upload.

Earlier failures are preserved, not relabeled wholesale fixture-only. Review 1
revealed real drive-Enter/event handling and local retry-form placement defects
plus rail CSS correction; focused regressions were added and actual cases passed
on corrected source. Review 2 had correct native CMD prompt/state but an unquoted
synthetic diagnostic echo parsed an ampersand as command syntax; a separate
welcome fixture selected a nonexistent direct CMD button. Three test lines were
corrected for literal quoting/real type selection, with no production change
between corrected review 2 and review 3. Final runtime evidence, not that diagnosis
alone, is required.

## Existing comparison failure observed in 117 review

The existing continuous-motion check genuinely failed one of 48 cases:
detached 390 px, wrapped long lines, Fit-map right/down gesture. Five sampled
frames across 47.7 ms had 3.65625 px paired physical-row offset. All 1,920 inputs were
answered, maximum response 72.5 ms, zero unanswered inputs/stalls; source content,
undo/redo and edit-revision invariants showed no mismatch in those tests. This is
not a universal data-loss guarantee and not a passed motion gate.

The aligned comparison/editor implementation and motion fixture are byte-identical
to 116. The observation is in the previously recorded narrow wrapped-map family
(116 included approximately 3.67 px in a different direction), but its root cause
is unproven; neither a 117-specific regression nor harmless fixture-only behavior
has been established. This scoped sidebar/local-shell update does not fix it.
The scoped release decision retains this disclosed caveat; no all-seven-green
claim is made and no comparison code or threshold was changed.

## Existing Hosts initial-focus failure

The Hosts interaction check found focus on the modal Close button rather than
its address-entry input. Read-only source extraction deterministically reproduced
the generic openModal zero-delay callback selecting the first eligible button
after Hosts explicitly focuses its input. The generic function, Hosts source and
fixture are unchanged from 116. The actual failed screenshot and source ordering
are consistent with that existing focus race, but no captured event trace proves
the exact hosted ordering. It is not established as a new 117 rail-keyboard
regression, and it is not dismissed as a harmless fixture-only failure. No Hosts
or modal fix, rerun, test relaxation or workflow change is included. The scoped
publication decision retained this second caveat alongside comparison alignment.

## Published package and feed verification

Automatic Release 40973657237 build 123796300733 and publish 123797061789 SUCCESS:
https://github.com/TechSavage1989/network-toolkit/actions/runs/40973657237
All four public downloads were fetched and rehashed against API metadata and the
reviewed proof. Exact 1,139 source files/1,146 portable members, both manifests/CRC
and PE AMD64 GUI version 1.17.117 with nine canonical ICO payloads/twelve resources
pass. Public EXE and Portable bytes equal the reviewed artifacts.

- Network_Toolkit.exe: asset 690256678, 20,772,864 bytes; SHA256 `d801b55921a9b59c0875b4282ee18ce6785926eb26b9bfa4eacc16c22eadf5d3`.
- Network_Toolkit_1.17.117_Portable.zip: asset 690256675, 13,734,687 bytes; SHA256 `3d14cdc8667e99946db03bbeb7d0a9b301bf1d6028e48f1f973c112a7e529772`.
- Network_Toolkit_1.17.117_Update.zip: asset 690256676, 7,946,707 bytes; SHA256 `aa2d6bd10a237d361992623fd91f6998cdba79b9693669edd576ee9fa10ac3ae`.
- SHA256SUMS: asset 690256674, 292 bytes; SHA256 `fda298b0a9a265932939ce7e1857f39889a2cd699d14ac802047dffb6a0ce5b4`.

The updater ZIP contains exactly the versioned EXE, SHA256SUMS,
THIRD_PARTY_NOTICES.txt and UPDATING.txt; no settings/source/credentials. Latest
feed, release body, source/tag/main matched at verification. Prior 116 tag/body
and all four original asset IDs/sizes/digests/URLs/timestamps matched before/after;
no old asset/tag, access or retired update feed changed. The 116 release note was
archived unchanged; 117 remains the sole root release note. This later docs-only
closure changes no version, package, source artifact or release content.

## Historical caveats and closure

116 review 40955139283 and duplicate 40956146534 each ended six SUCCESS/one FAILURE:
review editor motion had two detached 390 wrapped-map alignment failures, while
its duplicate editor passed and duplicate Windows Alt-cross-WebView drag failed
to record a pending transfer. Root causes remain unproven. Source/history checks
in those tests observed no data mismatch, not a universal no-data-loss guarantee.
This 117 scope does not fix either finding or 114's separate motion-latency failure.
Do not erase history or equate a later pass with a diagnosed fix. The full records
and deferred 115 historical closure stay unchanged in RELEASE116_HANDOFF.md.

Duplicate-main Windows 123796300692 failed. Native dirty-close consent/editability passed 10.45 s; the next
actual Alt-mouse cross-WebView editor transfer failed 76.22 s because the backend
never recorded a pending transfer. Later native Files/Terminal/4 MiB stages were
skipped. The source/helper/fixture are unchanged from 116, which also recorded
this failure; the pre-publication 117 native transfer passed 93.93 s. Cause remains
unknown, not established fixture-only/timing-only or a new 117 regression. Partial
native evidence and the complete log are retained. Ordinary drag-away, reorder
and maximize phases passed before Alt-transfer failed; both failure PNGs show
the synthetic unsaved documents retained, not proof of every possible failure
path. No rerun or rebuild is used to hide the failure; a pre-publication pass does not erase it.

The duplicate editor job also failed its continuous-motion check: one of 48
cases, main 390 px, wrapped Fit-map right/down, had P95/maximum 4.546875 px offset
in five frames across a 47.7 ms sampled span. All 1,920 inputs were answered,
maximum response 103 ms, with no page errors; the source hash matches final
review. This is distinct from the pre-release detached 3.65625 px case above.
Its cause remains unknown; no comparison fix, threshold change or rerun occurred.
The final long browser job, including Hosts and the real five-minute timer,
passed. Its success does not erase the pre-release Hosts failure or prove a fix.

Automatic main Checks 40973657061 is terminal FAILURE, updated at
2026-10-29T23:48:49Z and verified at 23:49 UTC. Five jobs succeeded and two failed:
https://github.com/TechSavage1989/network-toolkit/actions/runs/40973657061
- check 123796300653 SUCCESS, completed 23:33:13Z.
- browser-security 123796300660 SUCCESS, completed 23:32:29Z.
- dependency-security 123796300673 SUCCESS, completed 23:31:39Z.
- browser-workflows107 123796300731 SUCCESS, completed 23:42:36Z.
- browser 123796300884 SUCCESS, completed 23:48:49Z.
- browser-editor112 123796300342 FAILURE, completed 23:41:32Z: motion alignment.
- windows-terminal 123796300692 FAILURE, completed 23:34:26Z: native Alt transfer.

Neither the review nor automatic main run was all-seven-green. Both full terminal
inventories and failed-case evidence are preserved. No arbitrary rerun, rebuild,
workflow change or threshold relaxation was used; fixes for these unrelated
findings require a separately scoped follow-up rather than rewriting this release.


This five-document closure changes only AGENTS.md, DEVELOPMENT_HANDOFF.md,
README.txt, VALIDATION.txt and this new RELEASE117_HANDOFF.md. It does not rebuild,
replace assets, move tags, change access or authorize further implementation.
No remote action has been performed by the documentation worker.
