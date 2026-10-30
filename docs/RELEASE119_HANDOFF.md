# Network Toolkit 1.17.119 — Graph time windows and detached local Files

## Published identity and authorization

- Release: [v1.17.119](https://github.com/TechSavage1989/network-toolkit/releases/tag/v1.17.119), ID **428620405**.
- Published **2026-10-30T03:21:18Z**; actual public verification **03:22:27.122867Z**.
- Source/tag/main at publication: `a4b8c56f4c8692d4676d508381ec4c5a2b504aee`.
- Frozen tree: `9bd09a18eab01de56176e072bb0a06b4c969ba81`.
- Reviewed local source: `fdc69ba5b8d605c743372ef069ab71192cbd364c`, same tree.
- Previous main lease: `1949a1319a523dfdb6c4037cd5de7f08aabe9e19`, completed 118 documentation.
- Expected-head/non-force promotion read back **03:18:32.937Z**.

The user requested graph scaling/follow corrections after 118, approved the next
build/publication at **01:03:26 UTC**, then added elapsed/12-hour defaults and
direct detached-local Files. The bounded combined scope was reaffirmed at
01:10:45 UTC. This records that authorization, not a blanket future grant.
Two private reference screenshots were inspected but are not included here.
No repository access, live user device, credential, or user desktop was changed.

## Delivered behavior and boundaries

- Explicit Condensed Recent presets select the actual displayed time interval
  using the Fit curve's normalized proportions. Dropdown, graph/navigator bounds
  and visible-span label agree. Manual navigation indicates Custom; a safety
  floor expanding an undersized request reports its actual span rather than
  falsely retaining the preset. All/Fit retains ten-second recent spacing,
  older history clustering and existing five-record/two-second startup rules.
  Original timestamps, sample identities and retained source data are unchanged.
- Follow latest releases held graph selection without opening, closing or
  replacing Details. Left-only pointer/keyboard interval resizing preserves the
  right anchor and following/paused state. A following interval keeps the new
  duration as samples arrive; whole-window pan/right resizing may pause it.
- Total elapsed appears as **HH:MM:SS** in the existing graph statistics, not a
  new header row. It is independent of visible span, uses epoch differences,
  does not wrap after 24 hours and freezes at a recorded completion. A stopped
  legacy job without a finish timestamp says Not recorded rather than ticking.
- Fresh clocks default to **12-hour AM/PM**. Explicit saved 12/24-hour preferences
  remain authoritative. Display helpers share the formatter; raw timestamps,
  machine exports, nanoseconds and source measurements are not rewritten.
- An authenticated live detached Editor browses drives/folders, opens and manages
  local files without a main-window approval request. Windows account permissions
  apply; no elevation, main token, persistent grant or credential is delegated.
  The existing owner backend remains alive: this is **not** independence from
  backend-process loss. Text saves require the current UID/lease/revision hash;
  open-document/ancestor protection and staged-copy/no-clobber/cancel bounds remain.
  Closing/returning the child invalidates its local publication authority.
- SFTP, saved-credential use and connection management still need their separate
  approval/verification flow. Remote revocation does not revoke unrelated local
  access or a local copy; remote exports still pin transport and exact approval.

At 390 px, Custom may wrap Curve into the existing responsive controls' second
row. Actual controls are readable/reachable; no unchanged-height claim is made.
No comparison, Hosts-focus, cross-window Alt-transfer or user-reported file-row
drag repair is included in 119. See the open finding below.

## Executed acceptance and retained review history

Local final checks passed **1,316/1,316 JavaScript tests**, **42 Python tests**,
syntax/diff/metadata checks and 146 focused graph tests. Independent bounded
capability review passed focused Go race tests and checked live-child local
routes, separate remote approvals, lease/hash/path/owner guards. These local
checks do not substitute for the hosted/native evidence below.

| Job | Final review 40995231720 | Duplicate main 40996639878 |
| --- | --- | --- |
| check | SUCCESS, 123866699786 | SUCCESS, 123871196931 |
| browser-security | SUCCESS, 123866699821 | SUCCESS, 123871196873 |
| dependency-security | SUCCESS, 123866699733 | SUCCESS, 123871196661 |
| windows-terminal | SUCCESS, 123866699580 | FAILURE, 123871197055 |
| browser-editor112 | FAILURE, 123866699782 | SUCCESS, 123871196894 |
| browser-workflows107 | SUCCESS, 123866699834 | SUCCESS, 123871196791 |
| browser | SUCCESS, 123866699880 | CANCELLED, 123871196849 |

Final review ended **six SUCCESS/one FAILURE**, last job **03:22:34Z**. The
long browser was still running at promotion, then passed; publication was scoped
to passed new-feature/native/core/security/dependency/package checks with the
known comparison caveat retained. No workflow was disabled or failure hidden.
Duplicate main: **five SUCCESS/one FAILURE (native Alt-editor transfer)/one CANCELLED (long browser), last job completed 03:38:53Z**.

All four final graph layouts (1440/390, light/dark) passed exact interval/curve,
Follow Details, left-handle new-sample and keyboard assertions; actual PNGs were
viewed. Relevant legacy 98/97/96/95/94 reports passed, as did eight curve cases
and 64 temporal graph cases. Four detached-local browser layouts passed direct
browse/drives/open/lease-save, retained dirty sibling and independent remote
permission checks. Six local PNGs were inspected or byte-matched to inspected
frames. Synthetic browser data is not a claim of live-device use.

Final Windows native local test passed **25.60 s**: a real production child and
backend with **no main GUI**, main marked closed and remote approval false,
opened a temporary file and saved through its real lease; independent disk bytes
and an actual 1024×720 child capture were verified. Test-only DOM actions drove
that fixture, not actual Explorer drag gestures. Full Windows also passed dirty
close **9.59 s**, Alt-editor transfer **74.40 s**, Explorer files/folders/cancel
**17.66 s**, Terminal gestures **26.82 s** and 4 MiB editor transfer **22.01 s**.
Security/vault/bootstrap/capture boundaries passed. The local-shell helper still
ran on an already-elevated runner: **visible UAC transition/cross-account consent
was not tested**. No user files, real credentials or production devices were used.

Seven security reports and five capture checks passed, with current normal/narrow
pixels reviewed and digest/CRC-bound artifacts retained. Full/runtime npm audits
had zero findings; Go scans had zero reachable/imported-package findings. The
unused module-level OpenPGP advisory **GO-2026-5932** remains disclosed.

### Historical failed candidates are retained

1. Review 1 **40992597079**, head `3a0a564e42d9a74f4c6a3e1a640ca4d776d6c45e`,
   ended **three SUCCESS/four FAILURE**, last job 02:48:17Z. The new graph behavior
   passed but pixels exposed clipped select text and paused Fit was incorrectly
   labelled Custom. Several legacy assertions expected superseded Follow-closes-
   Details or endpoint-snapping behavior. Compact-local fixture Save was covered
   by the sidebar; native Save was attempted while read/loading correctly disabled
   it. The fixtures were corrected to use visible collapse/edit/reopen and real
   ready/dirty/disk conditions, not bypass readiness. A remote-approval mock missed
   its pending transition. A narrow wrapped-map comparison offset remained open.
2. Review 2 **40994205674**, head `1d899f259db81cc823b15952d3a76d10d1da8595`,
   ended **five SUCCESS/two FAILURE**, last job 03:10:14Z. Custom still had 42.14 px
   text in 38 px available space; later baseline-dependent checks lacked prepared
   historical fixtures after the earlier failure. The long browser's Fit navigator
   check also failed. Final changes retained resize anchors and readable controls;
   a later pass is not a deletion or reclassification of these original failures.
3. Final review retained a **main 390 wrapped right-map alignment failure**:
   4.546875 px maximum/p95 peer offset during 40 delivered inputs, zero unanswered,
   maximum response 27.9 ms/two frames. All 1,920 inputs across the suite were
   answered; no page/asset errors or source/history mismatch were observed.
   Comparison JS/CSS and the motion fixture match 118; the shared file-editor
   change affects saved/file-picker clock display, not comparison logic. This is
   the previously disclosed narrow-alignment category, **not proof of the same
   root cause, a fixture-only diagnosis or a fix**.
4. Duplicate Windows failed actual Alt-mouse transfer after **51.35 s**:
   **“real backend never recorded pending transfer.”** New native local Files
   passed 21.60 s and dirty-close passed 8.94 s first. The drag backend/coordinator/
   fixture are unchanged from 118. Cause remains unresolved; no rerun/rebuild
   erased it. Downstream skipped stages/missing uploads are not independent
   product failures. Duplicate Editor motion passed, which does not repair the
   final-review alignment failure.

5. Duplicate long browser ended **CANCELLED**, not SUCCESS, at **03:38:53Z**.
   Prior steps passed. Its log records the real-five-minute-poll-and-focus-collision
   case PASS at **03:38:48.320Z**, followed by **"The operation was canceled"**
   at **03:38:48.690Z**. That individual case is not proof the entire timer stage
   or any subsequent assertions completed. The duration is consistent with the
   configured 20-minute job budget, but the exact cancellation cause/actor was
   not established from an API annotation and is not claimed. Artifact upload
   completed afterward; no rerun/rebuild was requested to hide the cancellation.

### Genuine post-release file-drag defect, open in 119

At approximately **03:24 UTC** the user reported file-item dragging failing.
Read-only reproduction through the installed shared editor-tab dragstart handler
found that its document-wide capture listener called preventDefault for **non-tab
sources**. Local/SFTP file rows, selected editor text and an unrelated draggable
were all cancelled in that handler-level test. `editor-drag.js` is byte-identical
between 118 and 119 (SHA256
`18fd9358c9ef2061e246e4ca80fb9722ebd91c82073b30ac3b5b027e931035be`).
This is a real open handler defect; earlier local/native acceptance did not cover
this full-app file-row dragstart path. It is **not fixed by 119**. The minimal
non-tab pass-through repair is separate 120 work. No actual OS gesture was used
for this diagnosis; external Explorer-to-Toolkit drags do not originate this
listener and are not proven affected. Ordinary row dragging also does not create
a Windows-native payload: the existing explicit staged Copy to Windows Explorer
flow is separate. Do not generalize either successful local access or this
failure into a blanket all-file-drag claim.

### Post-release Condensed spacing regression, open in 119

At 03:34 UTC the user reported that changing Recent scale must preserve Fit's
baseline point spacing and configured curve. Inspection of both private reference
images confirmed roughly126 px recent one-second gaps in Fit versus8.6 px at5m,
at essentially the same1,262 px plot width. The119 normalization stretches the
curve's internal20.454-second detailed horizon to300 seconds (factor14.667),
changing both recent spacing and the real-time extent of the uncompressed count.
The earlier tests verified normalized shape and requested time bounds, not this
baseline-spacing invariant. This is a genuine reported regression, not corrected
by119. A separate mapping repair and the short-window policy are pending; no
private screenshots are published, and no graph repair is mixed into the focused
file-drag candidate. Do not describe the graph behavior as fully accepted by the
user merely because its earlier scoped tests passed.

## Public package and updater proof

Automatic Release **40996639911** passed build **123871196591** and publish
**123871960153**, completing 03:21:20Z. Reviewed package artifact **12951345701**
was 13,601,673 bytes, SHA256
`a444e50e0976abe06c1a41a14ba20840583b3fffcab3b5ff8b202f40ea8c8ce1`.
Public portable and embedded EXE exactly match those reviewed bytes.

| Public asset | ID | Bytes | SHA-256 |
| --- | --- | ---: | --- |
| Network_Toolkit.exe | 690775364 | 20,777,984 | `0bd1a6c68ba77441677a29fea727a912deb1ffddf404ae3e262deac8f3e7dfb5` |
| Network_Toolkit_1.17.119_Portable.zip | 690775366 | 13,790,994 | `b0b78bd2f8b8904e5d7a2ffe6fffc537aea9bf7e6d6556a6d76f9db4ce76dd56` |
| Network_Toolkit_1.17.119_Update.zip | 690775359 | 7,947,613 | `c3f838969a2d2f14062e37eb64d05f3d8d81773185faa5e7ad57f602175f90d6` |
| SHA256SUMS | 690775361 | 292 | `ad8d1c638d59cfeed866765a336cce3b74516983688e6f8b8f9de585f21354a3` |

All four actual downloads matched GitHub metadata and SHA256SUMS. Portable proof
verified **1,153 exact frozen-source files**, **1,160 members**, both manifests,
ZIP CRC/safe paths and source bytes. The EXE is AMD64/GUI, version 1.17.119, with
all nine canonical ICO payloads and 12 PE resources. The update archive retains
its exact four-file allowlist. Latest release feed, published tag/main and body
matching the source release note were verified at 03:22:27.122867Z.
Prior 118 release **428544757**, tag `2be7a62d18865275bd323c26b07f1520b689b560`,
body and four asset identities/sizes/hashes/URLs/timestamps remain unchanged.
No old release was rebuilt/replaced/retagged. This six-file docs-only closure
changes neither the published source archive nor any asset, tag or release body.

## Durable evidence and privacy

`toolkit119-evidence/` in the private engineering workspace retains exact run/job
inventories, original failures, source review, current graph/local/security pixel
acceptance, native logs, artifact SHA/CRC receipts and package/public verification.
Key records: `review1/failure-matrix.json`, `review3/graph-review/review3.json`,
`review3/local-acceptance/REVIEW.json`, `review3/motion-known-caveat.json`,
`review3/security-review/REVIEW.json`, `duplicate-native-caveat.json`,
`postrelease-drag-diagnosis/READ_ONLY_FINDING.json`, `public/proof.json`,
`public/verification.json` and the terminal closure job inventory. These are
local provenance paths, not public download links or user-desktop storage.
Private reference images/configuration content are not copied into source,
fixtures, public screenshots or these handoffs. Synthetic fixtures were used.
