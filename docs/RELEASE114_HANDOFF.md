# Network Toolkit 1.17.114 published and verified

Published 2026-10-29T17:43:50Z:
https://github.com/TechSavage1989/network-toolkit/releases/tag/v1.17.114
Release ID: 428199851
Exact reviewed/published source and tag: fd6cbbf287738320b51957d176dd2e63c63eece9
Tree: 0f5bd938f3bb54b6ef6892e3fc4d4eaae33a5293
Baseline main: 89590bca9649f337fc8966ce798977a3ca1ca9e1 (completed113 closure).
The user approved the combined attached prompt on 2026-10-29 at 13:16:20 UTC,
then explicitly added named Terminal tabs, reorder, scoped bulk actions and
intentional drag-away. Repository visibility/access is unchanged. The113 note
was archived byte-for-byte, and114 remains the sole root release note. This
later five-document closure does not rebuild or replace any published asset.

## Delivered behavior and limits

- Comparison panes and both maps synchronize continuously during wheel and map
  gestures. Visual gaps, native source gutters, edits, undo and source bytes are
  preserved. Explicit hunk navigation survives native virtual-height refinement;
  repair occurs outside active pointer/wheel/composition interactions.
- Editor tabs use the compact Terminal-style design. Files consolidates local
  path/create and active/saved SFTP controls; redundant Sessions and More path
  controls are removed. Accessible Windows drives and mounted volumes are listed;
  unavailable destinations remain visible rather than silently switching roots.
- Local and existing SFTP files/folders can be copied in after review and copied
  out through preparation followed by a native Windows Explorer drag. Remote
  content is staged first. Native FTP/FTPS drag is not supported; existing manual
  transfers remain. Browser-only use retains Download.
- Copy is the default. Folder collisions require Rename or Skip, not silent
  merging. Links, changed destinations and unsafe publication fail closed.
  Cancellation may retain completed files in a newly created folder; originals
  are not deleted. A held Windows drag can defer window closure until Escape,
  movement or release; staging stays retained until the gesture ends. If the
  backend unexpectedly exits, a private staged temporary copy may remain rather
  than risk deleting content still used by Windows. Detached Files access still
  requires explicit main-window approval; no credentials are delegated.
- Editor files support 16 MiB encoded (twice113), with 32 MiB decoded text and bounded
  transfer snapshots. Large-file mode pauses syntax/wrap and virtualizes gutters.
  Undo is capped at 128 MiB/80 entries. Comparison remains limited to 2,097,152
  characters / 30,000 lines; formatting and transfer limits remain explicit.
  A 32 MiB candidate was tested and rejected for insufficient interaction bounds;
  it is not claimed as supported. Search is not promised instantaneous.
- Terminal tabs sit directly above their panes. Explicit saved-profile names
  survive reconnect and window moves. Pointer or keyboard reorder preserves
  stable sessions/output; intentional captured-pointer drag-away uses the same
  ownership handshake, and Escape cancels. Disconnect all sessions retains tabs
  and output; Close all tabs removes them. Both clearly scope this window and
  confirm interruption, without closing unrelated detached sessions/profiles.
- Snapshot-copy reduction and prompt acknowledged handoff polling reduce avoidable
  Editor/Terminal transfer delay without reconnecting or releasing source state
  before destination acknowledgement. Cold launch remains variable.
- Graph recent spacing uses a 10-second reference, preserves natural startup fill
  and mature endpoint pinning, and does not thin as older history grows. Timeline
  wheel input pans; labels show actual elapsed time. Follow latest does not open
  Details. A resize-imposed packet-map scroll clamp is no longer mistaken for
  operator navigation after explicit selection opens Details.

Guides: EDITOR_FILES114.md, EDITOR_CAPACITY114.md, FILES_NATIVE114.md,
TERMINAL_NATIVE114.md and GRAPH_NAVIGATION114.md. Preserve all113 vault/host-key,
pager/output archive, macro, copy, icon, settings and updater safeguards. No user
files, private configurations, production devices or user desktop were used.

## Exact-source acceptance

Checks 40920901896: all seven jobs SUCCESS on fd6cbbf287738320b51957d176dd2e63c63eece9:
https://github.com/TechSavage1989/network-toolkit/actions/runs/40920901896
browser-editor112 123613518727; browser-security 123613519124; dependency-security 123613519186; browser-workflows107 123613519203; check 123613519214; browser 123613519261; windows-terminal 123613519317.
Local JavaScript aggregate: 1,244 tests passed; 42 Python tests and the four
relevant Go-race packages passed during integration. Final hosted application
checks run the complete repository suite on the exact published source.

Actual hosted evidence includes:
- 36 divider cases and 14,722 native source/gutter observations (maximum 1.140625 px
  offset), 48 continuous gestures / 1,920 inputs with zero unanswered inputs;
  maximum 4 animation frames / 78.3 ms input response, paired motion at least 96.296%,
  p95 physical offset at most 0.953125 px. Eight editable alignment cases preserve
  clipboard/source bytes/undo and synthetic composition boundaries. Nine legacy
  editor checks/five navigation snapshots retain explicit changed-line targets.
- 36 toolbar layouts and six behavior groups;11 Files/saved-connection checks,
  three activity/close-policy groups and36 independent-view checks. Actual
  normal/narrow/main/modal/detached images inspected; current SSH host verification,
  manual authentication and child request/cancel preserve dirty documents.
- 16 MiB browser normal/narrow/child cases: opens 1092–1141 ms, key-to-paint 113–173 ms,
  input wall 148–262 ms, exact save/search/undo/gutters and no page overflow.
  Final real native 4 MiB Editor moves preserve all state/exclusive ACK across six
  transfers, with medians 3131 ms detach / 1352 ms return. Earlier same-runner focused
 113→candidate medians were 5091→3357 ms detach and 2794→1344 ms return; 16 MiB native
  state/timing was also verified in that focused experiment. These are distinct
  measured samples, not a user-machine or universal latency guarantee.
-Real native Terminal tests retain one transport/session/transcript across three
  detach/return cycles, owner revisions 1–6, explicit profile name, selection and
  viewport; actual reorder/Escape/outside-client drag and scoped bulk guards pass.
  Final medians 2481 ms detach / 304 ms return are not a paired baseline comparison.
-Actual Windows Explorer file/folder inbound/outbound, empty folders, junction
  refusal, quick release and Escape tests pass. Pending-close cancellation retains
  owner/staging, returns effect 0 after real input, creates no cancelled-target
  output, and confirms authenticated cleanup; post-input return 89.852 ms.
-Graph eight responsive/theme/mode layouts and 64 actual wheel steps pass. Four
  startup cases verify exactly three samples. Normalized recent gap stays 0.1 of
  plot width for 120/706/7201 samples in normal/narrow views. All legacy 94–98
  geometry gates pass. Temporal evidence covers 64 cases: the published 109 negative
  control reproduces flicker while all 32 candidate states remain stable.
-Real five-minute updater scenario 302,136 ms; actual check-start
  interval 300020.224917 ms, no overlapping checks. Startup,
  opt-out, backoff and cancellation pass; released 64 remains an explicit negative
  control. Existing native vault/DPAPI/bootstrap and capture privilege gates pass.

Go scans report zero reachable/imported-package findings; the unchanged unused
OpenPGP module advisory GO-2026-5932 is disclosed. Pinned CodeMirror verification
and audits pass. Synthetic hosted browser/native evidence is not a live fleet,
user-machine reboot/performance guarantee or independent security audit.

## Corrected development failures retained

Earlier candidate failures are retained in the development evidence, not reported
as passes. Important corrections include measured 16 MiB capacity instead of 32 MiB,
native OLE lifecycle/staging cleanup, native comparison gap recovery preserving
hunk intent, and packet map geometry-clamp recognition. Legacy tests were changed
to explicitly open Details before testing Details-only actions. The last startup
fixture reused one synthetic job ID for different recordings and measured the
preceding 7,201 samples instead of its requested three; distinct job IDs and exact
rendered identity/count/mode/time readiness fixed that fixture. Endpoint/motion
thresholds were not relaxed and polling was not disabled. Final application bytes
are identical to the preceding accepted native/editor executable; the last change
is fixture-only. All seven final gates nevertheless ran on the final exact source.

## Published asset and updater proof

Automatic Release 40923712203 completed SUCCESS:
https://github.com/TechSavage1989/network-toolkit/actions/runs/40923712203
All four returned public download URLs were downloaded and verified against API
size/digest, ZIP CRC/safe paths, both portable manifests and exact source bytes.
The public portable and EXE equal the reviewed artifact; PE is AMD64/GUI114 with
all nine canonical icon payloads and 12 resources. Portable contains 1,109 exact
tracked source files / 1,116 members. Update ZIP contains only the existing four-file
allowlist. Latest release/tag/body agree with the frozen source/note. Prior 113
body/tag and all four asset identities/digests/URLs/timestamps are unchanged.

- `Network_Toolkit.exe`: 20,665,344 bytes; SHA-256 `9081958a4bd5d19cb08013b765f804755571b22d2052c55911bb6c530233f69d`; asset 689381270.
- `Network_Toolkit_1.17.114_Portable.zip`: 13,571,632 bytes; SHA-256 `b0b6bcf8b4516f60dd3f15cb967689fafd7f2d8679f96a55e5b76c37d917c1c3`; asset 689381280.
- `Network_Toolkit_1.17.114_Update.zip`: 7,911,189 bytes; SHA-256 `b042215318b6977825f283c6a5761a650222361ca2a27821516eb0c065c87101`; asset 689381274.
- `SHA256SUMS`: 292 bytes; SHA-256 `f718cd668f88b1a66b2b4144d923cb470ee4918a62c0b8e1d9b59d0bb292b0d9`; asset 689381278.

## Duplicate main Checks and closure

Automatic duplicate main Checks 40923712637 ended FAILURE by 2026-10-29T18:00:09Z:
https://github.com/TechSavage1989/network-toolkit/actions/runs/40923712637
Six jobs succeeded; editor 123623487320 failed its continuous-motion timing step.
The final long browser 123623487445 succeeded at 18:00:08Z. All 48 gestures ran;
47 passed. One of 1,920 delivered inputs, in detached 390px left-map dragging at 2x,
responded in 307.7 ms (two observed animation frames), exceeding the unchanged 200 ms
bound. Both panes moved together: 31/31 moving observations paired, zero peer
stalls, maximum offset 0.125px; the other 1,919 inputs responded within 76.7 ms.
All source/undo/ownership guards passed. Seven sampled animation-frame gaps of
307–412.5ms occurred across the duplicate scenarios, versus none in the accepted
review. This establishes an observation pause, not its cause: host scheduling,
browser/application/layout/GC work or sampling overhead cannot be distinguished
without profiling. A transient approximately 0.3s product responsiveness stall
cannot be excluded. No evidence here shows text loss or persistent pane drift.
The duplicate is not labelled passed, fixture-only or runner-only. Original
exact-source review 40920901896 passed all seven gates before publication; Release
and public asset proof remain unchanged. No retry, threshold relaxation, rebuild,
replacement asset or additional publication was performed. The failed artifact
12919508516 was rehashed (SHA-256
1a58fa328f44bfec43f2d77b2c657fef709ec2d74c82fa38199989fde62fe521;
13,230,932 bytes), CRC/extracted bytes and its passive timing trace were verified.

The automatic main Checks run is a duplicate of the already accepted exact-source
review, not a new build. This docs-only closure preserves the published source tag
and assets. Any later feature requires separate user scope; no optional rerun,
access change, live device action or security-setting change is authorized here.
