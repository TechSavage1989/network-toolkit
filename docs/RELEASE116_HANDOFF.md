# Network Toolkit 1.17.116 published: Terminal interaction hotfix

Published 2026-10-29T21:17:18Z:
https://github.com/TechSavage1989/network-toolkit/releases/tag/v1.17.116
Release ID: 428404389
Exact reviewed/published source and tag: f13c4df906a0ed8124308f8f95db571a7a9be532
Tree: c0127280d174967bc15170537d4f2fe05f8ef7c3
Local equivalent source: d4d453ea54ed468037f3bf768653b5f51c27bfcb
Baseline main: 1a26e0168eb1e8af0cc46f6480090bacf7c8d6b3 (published115).
Public verification completed 2026-10-29T21:18:18.695Z.

The user requested the Terminal correction at20:48:15UTC and approved publication
at20:51:49UTC. The release handling used an expedited validation plan. The seven existing workflow jobs were
kept. Focused real Terminal interactions, Windows, application, security,
dependency and package checks were required before promotion; three unrelated
browser jobs were allowed to finish in the background. This was NOT an all-seven-
green release claim. Repository visibility/access and prior releases are unchanged.

## Scope and preserved behavior

Only the Terminal tab-drag helper has a production behavior change, plus version
metadata. Pointer capture now stays on the actual tab button so ordinary clicks
reach the selection handler; sibling X buttons are not captured. A fresh trusted
Terminal press clears suppression
from a prior drag; suppression of that drag's own trailing click remains. Existing
order reconciliation avoids re-appending already ordered DOM nodes during output
refresh, preserving live capture. Reorder/detach, confirmations, connection/output,
saved labels, ownership and acknowledged handoffs remain on existing paths.
No comparison, credential, transport, Files or updater behavior was changed.

## Required pre-promotion evidence

Checks40955139283, exact source above:
https://github.com/TechSavage1989/network-toolkit/actions/runs/40955139283
- check123733386519 SUCCESS, completed21:10:29Z.
- browser-security123733386889 SUCCESS, completed21:10:09Z.
- windows-terminal123733386906 SUCCESS, completed21:13:57Z.
- dependency-security123733387028 SUCCESS, completed21:09:05Z.
- Four actual Chromium main/separate-Terminal cases at1280/390 passed tab
  selection, X confirmation/Cancel/close, reorder followed immediately by X,
  retained remaining output and no reconnection. All four PNGs were inspected.
  Fresh post-drag close presses occurred10.7-22.5ms after release.
- Local integration:1,287 JavaScript tests and42 Python tests passed; independent
  focused Terminal source tests21/21 passed. Exact-source Windows/application
  tests, build and package checks passed before promotion.
- Windows evidence includes real three-route WebView2 bootstrap10.27s, native
  vault captures0.40s, capture navigation/cancel spy0.02s, dirty-close10.54s,
  Explorer files/folders/cancellation16.35s, Terminal transport/gestures27.31s,
  and editor full/4MiB transfer tests92.76s/25.96s. Existing vault/DPAPI/ACL,
  ownership, cancellation and source-retention guards passed.
- Full/runtime npm audits zero findings; Linux/Windows Go scans zero reachable
  or imported-package findings. Unused OpenPGP module advisory GO-2026-5932
  remains disclosed. The pinned offline comparison bundle reproduced unchanged.
- Reviewed package and public bytes match; details below. Main promotion was
  expected-head/non-force and verified at21:15:08.798Z. At promotion the browser,
  browser-editor112 and browser-workflows107 jobs were still running.

Earlier failed review1 is retained: a real drag-capture/output-refresh issue was
corrected by idempotent order reconciliation. An unchanged native Explorer
root-file timing failure also occurred; readiness was only a hypothesis, not a
proven cause. Corrected exact-source required gates passed; failures are not
retroactively relabeled harmless.

## Background failure and honest limits

The review's editor job123733386818 subsequently failed ONLY continuous motion.
Two of48 cases failed paired physical-row alignment in a detached390px editor,
with wrapped long lines and Fit-map dragging. Left/down had five alternating
outlier frames in the first159ms: maximum300.96875px (one sampled interval about
11.5ms), P95 offset6.03125px. Right/up had five3.671875px outliers. All1,920 inputs
were answered, every moving frame had paired movement, and maximum response was
75ms. Source content, undo, redo and edit-revision invariants passed. These
observations show no data mismatch in that test, not a universal no-data-loss
guarantee. The final screenshot is settled after later gestures, not a capture
of the transient maximum.

Comparison implementation and motion fixture are byte-identical to accepted115.
The115 reference run's same gestures passed at P95 offsets0.5625/0.734375px.
This supports an intermittent existing narrow wrapped-map issue; it does not
establish its root cause or prove the failure fixture-only. This Terminal hotfix
DOES NOT fix or dismiss it. See the background report preserved with release
validation evidence; no rerun, rebuild or threshold change was used to hide it.

Hosted synthetic Chromium/Windows fixtures are not live user-device, production
configuration, reboot/install or exhaustive security-audit certification. The
16MiB/editor/Files/drag/graph features and limits documented for114/115 remain.
The older114 duplicate latency-bound failure remains a separate unresolved
historical observation; this116 failure is alignment, not latency.

Review40955139283 is terminal FAILURE (run updated21:24:07Z; all seven job
outcomes independently read back21:26UTC). Six jobs succeeded; only the editor
continuous-motion job failed as described above:
- check:123733386519 SUCCESS.
- browser:123733386729 SUCCESS.
- browser-editor112:123733386818 FAILURE.
- browser-workflows107:123733386875 SUCCESS.
- browser-security:123733386889 SUCCESS.
- windows-terminal:123733386906 SUCCESS.
- dependency-security:123733387028 SUCCESS.

Automatic main Checks40956146534 is terminal FAILURE; all seven outcomes were
read back at2026-10-29T21:34:35Z. Six jobs succeeded and Windows failed:
https://github.com/TechSavage1989/network-toolkit/actions/runs/40956146534
- browser:123736659970 SUCCESS.
- browser-security:123736660380 SUCCESS.
- check:123736660383 SUCCESS.
- browser-workflows107:123736660399 SUCCESS.
- browser-editor112:123736660410 SUCCESS.
- windows-terminal:123736660428 FAILURE.
- dependency-security:123736660574 SUCCESS.

Windows123736660428 passed native dirty-close consent/editability9.79s,
then TestNativeEditorTransfer failed58.20s at editor_native_windows_test.go147:
actual Alt-mouse cross-WebView drag never produced a pending backend transfer.
Later native Files/Terminal/timing stages were skipped; missing artifacts were
secondary. The pre-publication same-source transfer test passed92.76s. Cause is
not established; this is not labeled harmless/timing-only or fixture-only. The
duplicate editor job, including motion, passed, but does not erase the review
failure or establish its cause. The remaining browser/workflow jobs succeeded.
Neither review nor duplicate was all-seven-green. No rerun or rebuild was made
for these background failures; both exact terminal inventories are retained.

## Published package and feed verification

Automatic Release40956146375: build123736659438 SUCCESS21:17:08Z and
publish123737756544 SUCCESS21:17:20Z.
https://github.com/TechSavage1989/network-toolkit/actions/runs/40956146375
All four public downloads were fetched and rehashed. Portable1,131 members and
1,124 exact source files, both manifests and ZIP CRC pass. Public EXE/portable
match the reviewed bytes. PE is AMD64 GUI version1.17.116, with nine exact
canonical ICO payloads and twelve resources. Latest feed, source/tag/main and
release body matched at verification. Update ZIP contains exactly the versioned
EXE, SHA256SUMS, THIRD_PARTY_NOTICES.txt and UPDATING.txt; no source/settings/secrets.

- Network_Toolkit.exe: asset689936884, 20,706,304 bytes; SHA256 `91a704df0ffe141c11ac49eade6620dc95db63f4a0996d82dccb626db7455169`.
- Network_Toolkit_1.17.116_Portable.zip: asset689936889, 13,659,063 bytes; SHA256 `8f1d69a008c7c5ad25cf2b7962f32bab90efe4b91434a58fbb57286fb1efaac0`.
- Network_Toolkit_1.17.116_Update.zip: asset689936887, 7,922,243 bytes; SHA256 `1a07db11cb8834c038d97a95eb385d8a35b4603b6c69252e9f48c4974f8b9608`.
- SHA256SUMS: asset689936883, 292 bytes; SHA256 `748636283b98bf146a3f77ef7104c62af44d007b04867a76046448530fe2f63b`.

Prior115 tag, body and all four original asset IDs/sizes/digests/URLs/timestamps
match before/after promotion. No old tag, asset, repository access or retired
update-feed destination was changed. The115 release note was archived unchanged;
116 remains the sole root release note.

## Documentation closure

This later six-file documentation-only closure records116 and includes the
reviewed115 historical handoff whose separate closure was deferred for this
urgent hotfix. It does not claim that a separate115 documentation push occurred.
AGENTS.md, DEVELOPMENT_HANDOFF.md, README.txt, VALIDATION.txt and the115/116
handoffs are the only files changed. No application/test/build source, version,
public asset or tag is modified. Source sync is expected-head/non-force with
[skip ci]; any concurrent application work must be reconciled by the release lead.
