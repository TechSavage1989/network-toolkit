# Network Toolkit 1.17.113 published and verified

Published 2026-10-29T05:13:24Z:
https://github.com/TechSavage1989/network-toolkit/releases/tag/v1.17.113
Release ID: 427489688
Exact reviewed/published source and tag: 31f27994b9a3f7bae4442c255f63a65c8e21e0c8
Tree: a95bbbce6715f42b73c919c8e1a91aaa6c9b4619
Baseline: published112 plus completed docs closure402442e877acb29cd410af76e811c213aac65a78.
The user authorized development/publication and explicitly merged the graph,
editor activity/close-policy and concise-interface additions into this same build
on 2026-10-29 at02:12UTC. Repository visibility/access is unchanged. The112 note
was archived unchanged;113 remains the sole root release note. This later
five-document closure does not replace the immutable source/tag/assets.

## Delivered behavior and limits

- More and Sessions stay compact in the editor toolbar. The redundant local-file
  disclosure is removed; its path/create actions remain under More. Expanded
  More reveals real additional controls rather than an extra full-width heading.
- Active and saved file-capable connections use existing explicit authentication,
  host verification, native vault unlock and certificate checks. Main supports
  saved SSH/SFTP and FTP/FTPS. Separate editors request saved SSH connections
  through the main window; reviewed connection and renewed access approval stay
  separate. No credentials or connection-management capability are delegated.
- Tab Compare to chooses another document in the same window, retaining unsaved
  identity/view groups. Normal split and comparison dividers support pointer and
  keyboard resizing without changing source bytes or ownership.
- Editable comparison uses pinned offline CodeMirror state/view, the existing
  diff engine and Toolkit history. Visual gaps do not add source lines; source
  gutters, wrapping, intraline highlights, compact key and dual maps stay aligned.
  The final correction retains scroll intent through native height refinement
  and invalidates only changed measured native gutter layouts via public APIs.
- Graph and timeline share endpoint mapping after natural startup fills the view.
  The compact curve editor offers interactive/keyboard curve controls, recent
  uncompressed-point count, presets and reset. Width limits are explicit; saved
  requested counts and underlying history are not silently changed.
- The editor sidebar counts loaded documents across windows without counting
  untouched placeholders or transfer duplicates. Save-before-close defaults on;
  disabling it requires an explicit discard warning. Failed saves, pending saves,
  transfers and ownership protections remain. Child window close still returns
  documents through acknowledged ownership rather than silently discarding them.
- Ordinary Details/results remove duplicate facts and raw debug panels while
  retaining useful technical fields, errors, copy actions and explicit exports.

Guides: EDITOR_TOOLBAR113.md, EDITOR_SAVED_CONNECTIONS113.md,
GRAPH_CURVE113.md, EDITOR_ACTIVITY_CLOSE113.md and RESULT_READERS113.md.
Preserve all112 views/explorer and prior vault, host-key, pager/archive, banner,
macro, copy, port-action, settings and updater safeguards. No live devices,
private configurations, user files or user desktop were used in these tests.

## Exact-source acceptance

Checks40820131236: all seven jobs SUCCESS on31f27994:
https://github.com/TechSavage1989/network-toolkit/actions/runs/40820131236
Application123270407623; editor123270407833; Windows123270407875;
feature workflows123270407910; dependency123270407954;
security browser123270407955; long browser123270407996.
Local final:1,176 JavaScript tests,42 Python tests and all four relevant Go-race
packages passed. Hosted application checks include the full repository suite.

Actual hosted evidence includes:
-36 toolbar layouts and six behavior groups; Compare-to/duplicate-name/dirty
  chooser screenshots. Closed-toolbar editor height gains vs immutable112 are
  +37px desktop/tablet and+5px narrow; comparison+41.19px/+9.19px; narrow modal
  text+6px and comparison+1.5px. Expanded More intentionally displays additional
  controls; these are not claims of a height gain in every expanded state.
-36 divider cases,18 comparison cases and360 sustained paired observations
  before/through/after screenshots.12,360 visible source/gutter observations;
  max peer offset1.375px, gutter/source1.6875px and pinned drift0px. All12 divider
  PNGs inspected, including the formerly incorrect narrow gutter labels.
-8 aligned-editing cases,64 matched anchors(max0.234375px), exact clipboard,
  edit/undo/source-byte checks and all8 actual PNGs. Legacy editor9 checks and
  layout/map36 cases/15 navigation observations also pass.
-Saved connections9 checks and loaded-badge/close3 scenario groups; actual10
  normal/narrow/child images inspected. Native Windows completes14 close/save/
  transfer phases, including pending-save/failed-save refusal and child ACK-return.
-New graph12 responsive/theme/mode cases and8 checks, complete ui94-98 history/
  navigator matrices, graph1089 checks, and64 temporal records. All32 candidate
  temporal states have no flicker changes; published109 is a real negative control.
  Recent sample spacing remains13.53px desktop/7.87px narrow at706 to7,201 samples.
-Concise readers4 before/after layouts and15-tool inventories, preserved copy
  payloads, plus existing credentials, pager, capture and native bootstrap gates.
-Real five-minute updater scenario302,072ms; actual check-start interval
  300,023.513437ms. Startup/opt-out, backoff, cancellation and non-overlap pass.
  Released64 intentionally reproduces its old Illegal invocation negative control.

The pinned local CodeMirror bundle reproduces byte-for-byte; full/runtime npm
scans report zero findings. Go scans report zero reachable/imported-package
findings; the unchanged unused OpenPGP module advisoryGO-2026-5932 remains
reported, not hidden. Synthetic browser/native evidence is not a live fleet,
physical multi-DPI/trackpad test, user-machine reboot test or independent audit.
Existing large-document work limits remain explicit. Legacy updater fixtures use
synthetic version/feed data and have known missing-logo route noise; actual final
feature/native pixels and packaged icons were separately verified. Two graph
capture frames caught responsive transitions; settled exact-source frames and
measured bounds provide acceptance, not those frames as polished examples.
Earlier unpublished candidates exposed real comparison geometry/gutter defects
and fixture assumptions. They were corrected and the final full exact-source
suite rerun; earlier partial passes were not substituted for final acceptance.

## Publication and package verification

Automatic Release40821963373 SUCCESS (build123276247397, publish123276686013):
https://github.com/TechSavage1989/network-toolkit/actions/runs/40821963373
Expected-head non-force main promotion followed all seven review successes.
Public verification completed2026-10-29T05:14:57Z. All four returned public URLs
were downloaded; metadata sizes/digests, ZIP CRCs and both manifests match.
All1,070 packaged source files match frozen source bytes (1,077 portable members).
Public EXE and portable equal reviewed artifact bytes. PE is AMD64/Windows GUI,
version1.17.113, all9 canonical icon frames and12 resource leaves verified.
The updater ZIP contains exactly the executable, SHA256SUMS, THIRD_PARTY_NOTICES
and UPDATING files. Latest release/tag/main identify31f27994. Prior112 tag, body
and all four asset identities/digests/timestamps/URLs were verified unchanged
both before promotion and after publication. No old tag/asset was replaced.

- Network_Toolkit.exe: 20,471,808 bytes; SHA-256 dd7ae7651b94df31f12e0b975606f51c0dccd508b770b2db6f6849070ad30706
- Network_Toolkit_1.17.113_Portable.zip: 13,359,231 bytes; SHA-256 5a10b0d4ea3b3e324963fc9005dd1911ec32db599531ac2dd2d1756223409e9e
- Network_Toolkit_1.17.113_Update.zip: 7,841,133 bytes; SHA-256 f88d756b9b720d394f925af544a5f83975dddca1c9ee4062012271e0b21bc17c
- SHA256SUMS: 292 bytes; SHA-256 aa93be9240e96bf5203edfbdeebe99f4bd148ccd0b41e346e3f5e9ee566b4bd1

## Duplicate main checks and closure

Duplicate main Checks40821963354 is terminal, overall FAILURE. Its last job
completed2026-10-29T05:30:24Z. Six jobs succeeded, including the complete editor,
feature, security and real-timer browser suites; Windows job123276248045 failed
early in TestWindowsLocalShellColorTranscript/powershell. The fixture waited10s
for a prompt, proceeded without asserting readiness, then sampled typed input
800ms later. Output contained only console initialization/title, not a prompt.
The exact-source review9 case passed3.39s; the duplicate's later fresh interactive
PowerShell case passed6.05s with a30s prompt bound. The entire terminal code/test
and dependency tree are byte-unchanged from112. Startup/readiness timing is the
leading explanation, not proof that an intermittent existing shell-start stall
is impossible. No new113 production regression is demonstrated by this log.

The duplicate's downstream vault/bootstrap/Explorer/editor/capture native stages
were skipped after that early failure; missing-artifact upload errors are
secondary. Their full exact-source pre-promotion review9 acceptance remains
recorded above. This duplicate is not reported as green, was not rerun, and did
not cause a code rebuild, tag move or asset replacement. The observation is
retained for future investigation. This five-document [skip ci] closure preserves
the reviewed source and public release; it is not another application build.
