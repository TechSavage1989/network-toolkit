# Network Toolkit 1.17.121 — Compact sidebar and direct file dragging

## Publication identity and authorization

Published 2026-10-30T05:45:41Z; actual public verification 05:48:05.062831Z.
Release 428692620: https://github.com/TechSavage1989/network-toolkit/releases/tag/v1.17.121
- Published source/tag/main: `a3a96b0e649c1560b919184e8ca336fda2dcc11b`.
- Frozen tree: `e2336287ecc5abab564e94e15a7b4ac1858192e0`.
- Reviewed local source: `89d1c5e6e736ea182688d38a07b88dbdddaca7a6`, same tree.
- Previous main lease: `389917dd94346edd86873b043f53ac3ddc2ffcfc`, completed120 closure.
- Expected-head/non-force main readback: **2026-10-30T05:43:43.316Z**.

The user authorized this build/source/publication at **04:16:35 UTC**, then added
direct SFTP-to-Windows dragging at **04:18:20 UTC**. Actual private reference
screenshots were inspected but excluded from published fixtures/docs. This records
that scope, not future blanket permission. No repository visibility/access,
live user files/credentials/devices, or user desktop was changed.

## Material post-publication reliability caveat

**Known reliability issue:** the same-source duplicate Windows run failed its
first detached-child SFTP file drag after all five main-window cases passed.
No data was requested/materialized and no destination was written. The cause is
unknown; matching geometry/approved metadata does not establish a fixture-only
or premature-release explanation. Original ten-case success is retained, not a
guarantee of reliable detached delivery. No rerun, rebuild or repair is claimed.

## Delivered scope

### Compact, functional sidebar

Redundant connection instructions, success text and upload guidance no longer
consume permanent file-list space. A **7 px accessible active-connection badge**
provides status without hiding actionable errors or transfer progress. More aligns
with adjacent controls, opens a reachable menu and performs its existing actions.
The main connection settings gear reveals existing settings; the child gear stays
capability-hidden instead of granting app settings or credential access.

All eight main/child 1440/390 light/dark cases passed with actual before/after/action
pixels reviewed. Measured file-list height gains against the exact 120 baseline:

| Window | 1440 px viewport | 390 px viewport |
| --- | ---: | ---: |
| Main | 168 px | 150 px |
| Detached child | 102 px | 84 px |

These are the tested sidebar states, not a claim that every window gains the same
height. The narrow open sidebar can cover most of the editor; real drop hit tests
use the exposed document target, never force visibility or drop through an overlay.

### Open files by dropping inside the editor

A local or SFTP row dragged from this window onto its document area opens through
the existing validated read pipeline. An already-open file activates without a
reread, retaining dirty text, document UID, undo and other tabs. The gesture pins
same-renderer row/selection/listing and transport identity, then revalidates at
drop. Folders are not text documents; binary/encoding/size and active-operation
guards still apply. Changed SFTP identity cannot trigger a new read/reconnect.
This is neither text insertion nor an implicit file transfer or new connection.

Four actual HTML-mouse cases (main/child, 1440-light/390-dark) passed. They included
local and SFTP row drops on the actual textarea, dirty-file activation, retained
undo/UID, folder rejection, provisional binary-tab cleanup and stale-transport
refusal. Only 12 expected reads occurred; no writes/transfers or page errors.
All four actual PNGs were viewed. These browser APIs/files were synthetic; the
native full-page source/Explorer tests below are separate evidence.

### Ordinary one-gesture native copy

Local and already-authorized SFTP files/folders use Windows OLE copy from the
native list without a separate Prepare prerequisite. Progress/cancel remain
visible; originals are never moved or deleted and Windows handles destination
conflicts. SFTP is materialized only when the destination requests its data, with
pinned transport/approval and bounded private staging during synchronous OLE.
The object does not expose asynchronous Shell capability. No caller-provided
native path/credential or master capability is accepted by the row-pointer adapter.

All **10** pre-release main/child SFTP scenarios used full production WebView2 pages,
a real retained loopback SSH/SFTP transport, real Win32 input/OLE and actual
Explorer folder destinations with source/target hit testing and matched integrity:

- File copies produced exact bytes; folder copies preserved nested files and an
  empty directory. Source originals remained unchanged.
- Escape before drop performed zero materialization and created no destination.
- Escape during throttled download removed staging, left no partial destination
  and reported **cancelled**, not a misleading EOF failure.
- Same-row pointer drops into the editor opened the remote file through normal
  validation while preserving the existing dirty document identity and undo.

Main and child each exercised all five scenarios. The action trace contains
**no prepare prerequisite**; successful copies show direct→arm→start→materialize→
finish, copied outcome and cleared staging. Four native PNGs were viewed. This
is actual Explorer-folder acceptance, **not independent validation of a separate
Desktop Shell surface, cross-integrity delivery, arbitrary third-party apps,
large-file throughput, live servers or user files**. Existing local-native and
other transport checks also ran in the full Windows job; the new ten-case proof
specifically uses retained loopback SFTP.

### Limits and final correctness fixes

- Native export retains **2 GiB/1,500 entries/32 levels** and no-symlink confinement;
  text open/save keeps the existing **16 MiB** encoding/binary limits. This release
  does not raise either limit or validate bulk throughput at the maximum.
- FTP/FTPS native outward drag is not included. The child requires its current
  approved remote transport; local access remains under Windows permissions.
  The owner backend must stay alive. There is no automatic login, credential
  delegation, permanent privilege, UAC bypass or independent-backend promise.
- Reusing Connect now performs an existing authenticated metadata refresh so
  already-approved child connections appear. Query/connection epochs remain;
  it does not connect, unlock, grant or change transport authority.
- A direct copy with explicit cancelRequested now normalizes subsequent staging
  failure to cancelled under existing locking/cleanup. Unrelated EOF failures
  remain failures; no staged paths are disclosed.
- Existing comparison, graph, Hosts, close/ownership and 120 tab-drag safeguards
  are not redesigned. Historical failures are retained rather than declared fixed.

## Executed checks and exact evidence

Final local **1,345 JavaScript tests** and **42 Python tests** passed. Initial full
Go race tests passed; the final cancel-source change passed focused Go race
**3.067 s** and Windows compilation. Final exact-source core test/build passed.
An independent 23-test connection-refresh run passed. No application tests or
artifact downloads were repeated merely to write this handoff.

Sidebar/HTML artifact **12956738285**, 2,258,792 bytes, SHA256
`fd3a28edb74f5e923b1b411d3e52b6ebae4199db0387700d4ade9e1ee064328f`,
was checked against exact-head metadata, CRC and extracted bytes. Eight sidebar
cases passed; 20 changed sidebar PNGs were freshly viewed and five matched already
inspected frames. All four HTML-drop images were freshly viewed separately.
Native artifact **12956353653**, 174,000 bytes, SHA256
`92cd0fc6f2016333d789ef161d4585bec15729e0bd1b583ed393bd72d997376e`,
was independently rehashed/CRC/extracted-byte checked; all ten cases and four
actual native source PNGs passed the assigned reviews.

Final full Windows passed three private-bootstrap routes **9.89 s**, vault captures
**0.31 s**, capture navigation/cancel launcher-spy **0.02 s**, direct SFTP **83.17 s**,
no-main local child **5.77 s**, dirty-close **10.09 s** and existing editor transfer
**70.10 s**. Delayed-data/cancel negative tests passed. Seven security reports passed;
seven changed credential frames and one pager frame were viewed, with other
credential/native/pager/banner bytes matching inspected prior frames. The assigned
capture review subsequently verified artifact 12957256200 (38,116,727 bytes,
SHA256 `82b72f2192fe8efeccfd91ccea87aa121510a1e8ad49e2f8aa5e521bf4435713`),
its five-check report and fresh 1440/390 pixels. Dependency scans reported zero reachable/imported
findings; unused OpenPGP advisory **GO-2026-5932** remains. Full/runtime npm audits
were zero and the pinned offline bundle reproduced.

The native shell helper ran with parent/child already elevated:
**UACInvoked=false**. A visible consent transition/cross-account elevation was
not tested. Synthetic temporary files/loopback traffic are not user-device proof.

## Failed observations and corrections remain recorded

1. Review 1 **41004774088**, source `f24056f5…`: all eight new sidebar cases passed.
   The narrow HTML target's geometric center was obscured by the sidebar. The
   first native gesture did not reach a source API because the row was clipped.
   Fixtures were corrected to require actual visible hit points. Removed-text
   expectations and historical-baseline preparation were corrected separately.
   A narrow legacy-divider measurement remained a failed observation, not an
   authorized comparison repair or blanket fixture-only verdict.
2. Review 2 **41006238654**, source
   `4e3ba7a8ed0effe9f9cb88849094b653ce41e415`: eight sidebar/four HTML cases
   passed, but native first-file destination was absent. The action trace reached
   direct/arm/start/finish without materialize; native delivery was **not accepted**.
   One of 48 old motion cases (main 390 wrapped left-map) also failed: maximum
   **4.609375 px**, p95 **4.140625 px**, 40/40 inputs answered, maximum response
   **27.1 ms**, no stall or page/asset errors. Comparison engines/map geometry and
   fixture are unchanged from 120; similar historical symptoms do not establish
   cause or make this fixture-only. No comparison repair is included.
3. Review 3 diagnostic source `641aac31…`: an earlier PowerShell color-transcript
   test failed and native SFTP was **SKIPPED**, so no COM conclusion was drawn.
   Later diagnostic stages were made independently runnable while the earlier
   failure remained red; no test outcome was hidden or reclassified as success.
4. Review 4 diagnostic source `f1e816a4…` reached actual DROP/materialize, then
   failed the synthetic SFTP fixture's canonical-root guard. The fixture had
   bypassed production EvalSymlinks(root); a Windows temporary-root alias was
   rejected as a reparse escape. Only the fixture root/preflight was corrected;
   production confinement was not weakened and outward copy remained unaccepted
   until the following real execution.
5. Review 5 **41008159721**, source `e5dadd6b…`: all five main native scenarios
   passed, but child setup exposed stale already-approved connection choices.
   That was a real product metadata-refresh defect, not waived. The final narrow
   refresh and explicit-cancel outcome fixes described above were independently
   reviewed and verified in the final ten native cases. Earlier failure logs remain.

### Final-review comparison failure retained

The exact final review also failed one of 48 legacy motion cases:
**main-390-left-map-0--1**, maximum peer offset **4.609375 px** and p95
**4.140625 px**. All 40 inputs were answered; maximum response **70 ms/two frames**,
zero stall, and no page/asset errors were recorded. The case/max offset matches
review 2, but neither matching symptoms nor source comparison establishes the
cause. This is a real failed alignment check, not a fixture-only dismissal.
Comparison engines/map geometry and the motion fixture retain their prior logic;
**the entire file-editor asset is not claimed unchanged**, because 121 adds the
drop-open integration. No comparison repair was included, and the original
narrow-alignment/Hosts/Alt-transfer history remains intact.

## Run inventories and release sequencing

Final source for review 6, promotion and duplicate is
`a3a96b0e649c1560b919184e8ca336fda2dcc11b`.

| Job | Review 41009283786 | Duplicate 41010025845 |
| --- | --- | --- |
| check | SUCCESS, 123911250897 | SUCCESS, 123913586648 |
| browser-security | SUCCESS, 123911250985 | SUCCESS, 123913586340 |
| dependency-security | SUCCESS, 123911250921 | SUCCESS, 123913586688 |
| windows-terminal | SUCCESS, 123911251100 | FAILURE, 123913586571 |
| browser-editor112 | FAILURE, 123911251076 | SUCCESS, 123913586630 |
| browser-workflows107 | SUCCESS, 123911250885 | SUCCESS, 123913586673 |
| browser | SUCCESS, 123911250788 | CANCELLED, 123913586623 |

At the 05:43:36.740Z promotion decision, **four required jobs were SUCCESS and
three legacy browsers still running**; all 22 new cases and package proof had
passed. No all-seven-terminal claim was made. All normal jobs remained enabled.
Final review: **six SUCCESS/one FAILURE (comparison motion), last job completed 05:54:57Z**.
The run metadata became terminal at 05:54:58Z. The long-browser log records the
real-five-minute-poll-and-focus-collision case PASS at 05:54:52.722645Z; this is
a logged outcome, not a newly measured interval-duration claim.
Duplicate: **five SUCCESS/one Windows FAILURE/one browser CANCELLED, last job completed 06:04:03Z**.
Run metadata became terminal at 06:04:04Z. The duplicate long browser is
**CANCELLED, not SUCCESS**: six preceding native-timer cases logged PASS, but
there is **no real-five-minute-poll-and-focus-collision PASS** before cancellation
at 06:03:59Z. The job ran 05:43:46–06:04:03 with a configured 20-minute timeout;
the log states only that the operation was canceled. Timeout is consistent with
the duration, not an established cancellation cause/actor. Remaining assertions
and the timer stage were not proved complete. No retry/rerun was made.
Automatic Release 41010025996: build and publish SUCCESS.
Later successes do not erase prior failed observations or resolve historical
comparison/Hosts/Alt-transfer/UAC limits without corresponding evidence.

### Same-source duplicate native failure: unresolved detached-copy risk

Duplicate Windows job **123913586571** failed the first **child-file** scenario
at 05:46:56Z, after all five main scenarios passed. Exact-source artifact
**12957410751**, 169,859 bytes, SHA256
`60735f258376ddfdb3960d1b3fabef9669f9f95a164942601800357b755a820c`,
was independently tied to the head, rehashed, CRC/extracted-byte checked and its
actual child-ready/failure PNGs viewed. The prior review passed all ten scenarios;
this later failure is a real reliability limitation, not erased by that pass.

The child action sequence was **direct→arm→start→finish(cancelled)** with
**cancelRequested=false, renderStarted=false, zero bytes/items and no staging or
destination output**. Approved connection metadata was present. Source/target
coordinates, target UIItemsView bounds and source/target integrity matched the
passing child case exactly. No user cancellation was recorded in that failed case.

There is no per-action timing or child OLE observer trace: the recorded native
observer belongs only to the main window. The evidence cannot distinguish native
admission timing from target/OLE nonacceptance, nor prove premature release. It
must not be labelled fixture-only, proven unrelated, successful cancellation or
an established earlier-release cause. Originals were unchanged in observed evidence because
failure preceded materialization/write; this is not a general no-data-loss claim.
No new source patch, rerun or rebuild was made for this finding. The user was
informed at approximately 05:51 UTC; the four public assets remain unchanged. Later native
stages stopped after the primary child-file failure; five subsequent upload
failures reflect missing downstream evidence, not additional proven product
defects or passed native checks.

## Reviewed package and public proof

The exact-source reviewed package contains **1,170 source files/1,177 members**.
Its portable SHA256 is
`e98c7b404bb64886186a88d9c536d2d9be1183447b978e377a52ae3e1ce1d36d`
(13,896,406 bytes); EXE SHA256 is
`73a98b9ab82cb77d9522bb6e3dee8a304194e55c0bdc5370671b485c493c38fe`
(20,834,304 bytes). Package checks include manifests, ZIP CRC/safe paths and
exact source bytes; PE is AMD64/GUI 121 with nine canonical icons/12 resources.

| Public asset | ID | Bytes | SHA-256 |
| --- | --- | ---: | --- |
| Network_Toolkit.exe | 691084286 | 20,834,304 | `73a98b9ab82cb77d9522bb6e3dee8a304194e55c0bdc5370671b485c493c38fe` |
| Network_Toolkit_1.17.121_Portable.zip | 691084278 | 13,896,406 | `e98c7b404bb64886186a88d9c536d2d9be1183447b978e377a52ae3e1ce1d36d` |
| Network_Toolkit_1.17.121_Update.zip | 691084279 | 7,969,301 | `db969ea6a531610c1100719d31170b48281acbcf2570fdfe7feecf1d7b03f175` |
| SHA256SUMS | 691084285 | 292 | `7e6259f0223f1d35d452326ab889adebd01cf1dccb64a1aa18cb969663d5bcab` |
All four actual public downloads match the reviewed bytes; 1,170 exact source
files/1,177 portable members, manifests, PE 121/nine canonical icons/12 resources,
update four-file allowlist and latest feed/tag/main/notes are verified. Prior 120
release **428643711**, tag `45ee25b15c1c88fdb7c77b2f06057d38ae6b5357`, body and original four asset IDs/sizes/hashes/URLs/timestamps remain unchanged.

This five-document closure changes no production source, reviewed package,
release body, published tag or repository access. Historical 120/119 records remain
intact below current summaries. No private screenshots/configuration content is
copied into public source, fixtures or evidence images; synthetic inputs are used.

## Durable local evidence

Private engineering workspace `toolkit121-evidence/` retains the six review
iterations and original logs, `review6/sidebar/bounded-review/review.json`,
`review6/editor-drop-review/review.json`,
`review6/native-sftp/bounded-review/review.json`, `native-source/ACCEPTANCE.md`,
`review6/security-review/REVIEW.json`, `review6/package/proof.json`,
`review2/motion-caveat.json`, `promotion.json` and later public/terminal receipts.
These are local provenance paths, not user-desktop files or public download links.
