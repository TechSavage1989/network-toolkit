# Network Toolkit 1.17.111 published and independently verified

Published 2026-10-28T20:19:20Z:
https://github.com/TechSavage1989/network-toolkit/releases/tag/v1.17.111
Release ID: 427149188
Exact reviewed/published source and tag: bf414aec01a63a4b492cdfc62f32fa52910f3b69
Tree: 250294f0ece6dcc3397a9f9ed899770015da3523
Baseline: completed110 main 4747873292eca9f09577ac2803756113557d0bcf.
The user explicitly authorized publication and release. Version metadata/resources
advance together; the110 note is archived unchanged under docs/releases and the111
note is the sole current root note. This later docs-only closure is not a new build.

## Scope and regression contract

Read [EDITOR_LAYOUT_MAP.md](EDITOR_LAYOUT_MAP.md) for shared compact, left-aligned
controls, removal of repeated tool/editor headings, one New action in the tab strip,
normal file actions, More and Options disclosures, separate Left/Right maps,
alignment gaps, individual viewport markers, Fit and 1.5x/2x/3x/5x zoom, and real
wheel/click/drag/keyboard navigation. Editing, wrapping, swaps and resize refresh
geometry. Existing document ownership, save/undo/dirty protections and backend
security remain unchanged. Existing graph, native, updater and credential gates pass.

## Exact-source hosted acceptance

Review Checks: https://github.com/TechSavage1989/network-toolkit/actions/runs/40759383905
- browser-security: SUCCESS (job 123066858296).
- browser: SUCCESS (job 123066858464).
- windows-terminal: SUCCESS (job 123066858575).
- check: SUCCESS (job 123066858738).
- browser-workflows107: SUCCESS (job 123066858789).
- dependency-security: SUCCESS (job 123066858922).

The application job passes all eight required Go packages, 1,027 JavaScript tests,
42 Python release guards, metadata/resource checks and the Windows GUI build.
Linux/Windows scans report zero reachable or imported-package vulnerabilities.
The existing unused module-only OpenPGP GO-2026-5932 advisory remains disclosed;
this is not a security audit. Canonical vendored dependencies remain unchanged.

The new browser gate passes main/detached/modal editors, light/dark, normal/narrow/
short windows, actual pointer input on both maps, independent/synchronized scrolling,
fractional wheel input, all zoom choices, wrapping, edits/undo, swaps, resize and
empty/short/unequal/heavily changed inputs. No browser or asset errors. Native
Windows/WebView2 close consent, tab reordering, acknowledged cross-window transfers,
merge, cancellation, failed launch and maximize/restore pass on isolated runners.
Downloaded native and comparison screenshots were independently inspected.
The prior temporal graph negative-control gate and genuine five-minute updater
timer gate pass. The latter lasts301,831ms, polls after300,017ms, retains native
timer functions and allows only one active check. Legacy timer fixtures still
stub icons with404 responses; actual browser assets and PE icons were verified
separately. No required gate was skipped, waived or relabeled.

Hosted comparison viewport measurements at1440x1000, dark (CSS pixels):

| Context | Before110 | After111 | Gain |
|---|---:|---:|---:|
| main | 454.56 | 588.31 | 133.75 |
| detached | 493.56 | 664.31 | 170.75 |
| modal | 593.50 | 709.50 | 116.00 |

Local earlier measurements have different main-window/font geometry; the paired
gains agree. Browser device ratios1/1.25/1.5/2 were checked in earlier local evidence.
Physical trackpad hardware and physical multi-monitor Windows DPI are not certified.
Hosted native maximize uses the runner's1024px client workarea; wider browser
geometry is separate evidence. Synthetic documents only; no private references,
user files/vault, live devices, reboot or installation acceptance is claimed.

## Publication and downloaded assets

Main was promoted without force only after every review job passed. Automatic
Release: https://github.com/TechSavage1989/network-toolkit/actions/runs/40761854203
- build: SUCCESS (job 123075571089).
- publish: SUCCESS (job 123076731545).

All four public assets downloaded and verified at 2026-10-28T20:19:57.359214+00:00.
Sizes, GitHub digests, top-level and nested manifests, ZIP CRCs/entry allowlists,
latest release, exact tag/source and release-note body agree. Portable has1,015
members and1,008 source files matching the reviewed commit byte for byte. Public
EXE and portable ZIP exactly match the hosted review artifact. AMD64 GUI PE version
is1.17.111 with all9 canonical icon payloads and12 native resources. Update.zip
contains only its four allowlisted runtime files. No source or settings leak into
the updater. Previous109/110 tags, release bodies and asset IDs/sizes/hashes remain
unchanged, including the authorized109 Toolkit-Icons.zip extra. Repository access,
visibility and retired update feed were not changed.

- Network_Toolkit.exe: 20,017,152 bytes; SHA-256
  aededff651c8712e17f45d97fe62eec9abe81a51cf48fda4e799f8bf4fcd0035
- Network_Toolkit_1.17.111_Portable.zip: 12,922,314 bytes; SHA-256
  3aeb30b85960db1ccaf36d67d3fd0c03d4c042bededccb3d260de3b92b05dc4f
- Network_Toolkit_1.17.111_Update.zip: 7,694,413 bytes; SHA-256
  bd69132d967aec59020adf16bbb2ccc421359ec8dfcd12ef59a859a4d0be1666
- SHA256SUMS: 292 bytes; SHA-256
  425a2f3e366a1ed7b92944830157ea4d4a830b8e3029d56366f6b4b06321136a

Duplicate main Checks: https://github.com/TechSavage1989/network-toolkit/actions/runs/40761854209
- windows-terminal: SUCCESS (job 123085222221).
- check: SUCCESS (job 123085225731).
- browser: SUCCESS (job 123085283537).
- dependency-security: SUCCESS (job 123085286008).
- browser-security: SUCCESS (job 123085292296).
- browser-workflows107: SUCCESS (job 123085299905).
All main jobs reached terminal success before this closure. The initial Windows
job123075570841 failed one real cross-WebView mouse gesture: no pending backend
transfer was recorded. Both synthetic documents remained owned and intact; all
earlier close, transfer, cancellation, detach and maximize phases passed. The
review run had passed the same source and identical pointer coordinates. A focused
Windows job retry on the unchanged source passed before closure; no production
code, test assertion, release tag or asset was changed to obtain that result.
The first failure report/screenshots are retained. Its intermittent cause is not
established; this is a disclosed native-gesture reliability limitation, not a
claim that the initial attempt passed. No gate was waived.

## Desktop boundary and timing

After the user's stop instruction, no local UI automation, input, test windows,
normal-session termination or unsaved-work discard occurred. Local activity was
limited to noninteractive development/build/package/inspection. Visual/native
release checks ran in isolated hosted environments. Future local desktop tests
require advance user approval for a clearly scheduled session. Never stop the
user's normal app sessions or discard unsaved work.

Wall-clock record through verification and closure preparation (2026-10-28T20:39:56.947636+00:00):
- From recorded request 2026-10-28T19:34:22Z: 65 min 34 sec.
- From implementation start 2026-10-28T19:37:01.1047746Z: 62 min 55 sec.
- From release preparation 2026-10-28T19:59:06.126597+00:00: 40 min 50 sec.
These are elapsed wall-clock durations, not billed usage. The prior local review
ended19:57:00Z; this record includes the separately authorized hosted publication.
Preserve all published tags/assets. Future work and releases need their own scope;
handoff-only records use [skip ci].
