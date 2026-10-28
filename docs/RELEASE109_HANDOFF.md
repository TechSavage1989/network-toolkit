# 1.17.109 release handoff

## 1.17.109 published and independently verified

Published 2026-10-28T16:23:24Z:
https://github.com/TechSavage1989/network-toolkit/releases/tag/v1.17.109
Release ID: 426910194
Exact reviewed/published source and tag: d3494a879604fb2523e0867ad1fb175a2f9abbf5
Tree: 9dda3650d5c794a4fc1c734976619819ce87fbfa
Baseline: completed108 closure main00c290a78f354a31ae4638fc37ab85c7532a6fed.
All six exact-source Checks and bounded source/artifact/pixel reviews passed before
non-force expected-head promotion. Existing public repository visibility and access
were unchanged. This later docs-only handoff neither rebuilds nor replaces assets.

Completed scope:
- Remove the redundant global top bar; retain system identity beside the sidebar
  brand and original theme/settings/running-count controls in its footer. The
  compact navigation button reuses visible existing tool controls, including all
  six SFTP/FTP/TFTP server/client modes; update/offline notices remain intact.
- Replace the permanent editor Move row with explicit compact Move / merge access
  beside Compare. Pending/attention text and original destination/cancel actions
  remain visible and reachable; opening/dismissing the dialog never starts a move.
- Independently collapsible Open local file/Connected session forms default closed.
  Explicit main-window choices persist; fresh detached windows start collapsed and
  keep their own choices for that window lifetime. No child settings-write route
  or document-transfer preference fields were added.
- Reclaimed space belongs to actual tool/edit/compare viewports. Unsaved content,
  Undo, selection/scroll, existing acknowledged ownership, dirty close, vault,
  host-key, terminal pager/archive, graph and updater protections are preserved.

Checks40721887493 completed SUCCESS; final long job ended2026-10-28T16:18:13Z:
- Application122935005514: all required Go packages,983 JavaScript tests,42 Python
  publication guards, Windows GUI build and exact reviewed portable package.
- Dependency122935005590: canonical vendor match; zero reachable/imported-package
  findings on Linux/Windows. Unused module-only OpenPGP GO-2026-5932 remains
  disclosed, with no fixed version reported; this is not an independent audit.
- Windows122935005621: existing credential/vault/DPAPI/native bootstrap/capture/
  Explorer boundaries plus actual editor dirty-close8.99s and two-process move,
  merge, cancellation and failed-launch retention43.23s. Native setup/unlock/
  recovery painting and secret-entry tests pass. A first-run screenshot readiness
  failure was fixed only in its bounded test barrier/diagnostics, not vault code.
- Security browser122935005138: credential, pager, banner, discovery, macro and
  transfer-copy reports pass; actual normal/narrow evidence independently viewed.
- Feature browser122935005577: all existing107/108 and new109 scenarios pass.
  Chrome129 actual108-to109 comparisons and68 navigation checks pass; all21 chrome
  PNGs inspected. Normal1440 workspace gain61px;390 gain1.5-93px varies with wrapping;
  short820x540 gain12-49px; scoped shell modes unchanged, not smaller; railDPR2+61px.
  Editor24 paired text/compare observations pass in main/detached/terminal-modal,
  normal/narrow and light/dark contexts; all13 new editor/menu PNGs inspected.
  Main text gain237.5px at1440 and472.5px at390; comparison237.5/329.8125px.
  Detached gain176.5px desktop and329.8-446.8px narrow; modal71/126px.
  Exact75..100 selection,350px scroll, UID/text/dirty/history and saved false/true
  reload checks pass. Layout tests report no page/asset errors or horizontal overflow.
- Long browser122935005768: existing graph/editor/terminal/clipboard/layout gates
  and all explicit reports pass. Real native-timer scenario301,830ms, actual polling
  interval about300,012.295ms, unchanged timer functions and maximum one check.
  Released64 negative control reproduces its expected TypeError. The legacy timer
  fixture's icon404 stubs are not icon acceptance: final embedded UI/image checks
  and actual PE resources independently pass. No failed gate was skipped.

Automatic Release40725083880 succeeded: build122946057081, publish122947267271.
All four public downloads were verified at2026-10-28T16:24:58Z against GitHub sizes,
digests and manifests. Portable has999 members and992 exact source files matching
the reviewed tree. Public EXE and portable equal reviewed bytes. Actual AMD64 GUI
PE contains1.17.109, all9 canonical ICO payloads and12 expected resources. Update.zip
has exactly the four allowed updater files, no source/settings/credentials. Latest
feed/tag/notes/source agree;108 tag, notes and all four immutable asset metadata
remain unchanged before and after promotion. No retired feed or old asset was edited.

Public assets:
- Network_Toolkit.exe: 19,974,656 bytes; SHA-256
  6ba0b4326ff0563e3f27370a00aba7fc03c5cce1563dae7b53c660294ed0ad13
- Network_Toolkit_1.17.109_Portable.zip: 12,828,665 bytes; SHA-256
  2da21400cf6183fea8dc6f2de34abfacae2f2d2c6d4eb6faff68d8ebbe446b8e
- Network_Toolkit_1.17.109_Update.zip: 7,680,843 bytes; SHA-256
  3704bc4cd0a4bd88a847b04b1ef5c33bde74bbb46440a523a9287e4a1c8f86f1
- SHA256SUMS: 292 bytes; SHA-256
  27e638cd1861f69a3909ffbb915a72566807ad0c7e79491c7efe0b554f7d6681

Automatic duplicate main Checks40725083916 completed FAILURE at2026-10-28T16:38:48Z.
Five jobs passed; long browser122946057756 failed only the accelerated updater
startup/focus fixture at scripts/app-updater-startup.browser.cjs:89 (one release
POST observed, two expected). Its subsequent real five-minute native-timer step
passed, as did all six exact-source review gates before publication. The updater
and accelerated fixture were unchanged from108. The failure log lacks scheduler
state to distinguish delayed fixture processing from another cause; it is not
silently relabeled a pass or a proven harmless flake. A later focused build will
add completion-aware test synchronization and failure diagnostics, retaining the
same count/freshness/no-overlap assertions. No109 application, tag or asset was
changed, and no duplicate rerun was used to erase this result.

Post-release observations (separate investigation, no109 application change):
On2026-10-28 the user reported persistent graph-point outline flicker both during
and after scans. A subsequent class-state investigation identified an undefined
optional force value in hover-class toggling; temporal browser validation and any
fix belong to a later scope. The passed synthetic109 checks are not live-user
acceptance of flicker behavior. Editor tab dragging/reordering/drag-out was not
implemented in109; explicit Move / merge controls remain the supported route.
A later maximized detached-editor screenshot also exposed a blank reserved grid
column; the child shell omits the column-one reset used by detached Terminal.
That issue and the requested direct Detach action are separate follow-up scope.
These reported gaps are open, not silently marked resolved by this handoff.


Evidence is synthetic browser data and hosted Windows native tests, not the user's
desktop, real switch fleet, live capture, private files/settings/vault or a reboot/
installation test. DPR2 is browser scaling coverage, not native multi-DPI certification.
Four private visual references and earlier private configs remain outside source and
packages. Notepad++ supplied workflow inspiration only; no wholesale clone or extra
feature scope. Main form-state persistence and child transient-state limits are explicit.

## Workflow references

Read APP_CHROME109.md and EDITOR_LAYOUT109.md for control behavior and limits.
Official Notepad++ workflow references (compact actions and optional panels only):
- https://github.com/notepad-plus-plus/npp-usermanual/blob/master/content/docs/views.md
- https://github.com/notepad-plus-plus/npp-usermanual/blob/master/content/docs/user-interface.md

The user approved109 implementation/publication2026-10-28 at15:00:29UTC. Existing
public-source/release destination had been explicitly clarified; access stayed unchanged.
Request14:50:28UTC, public verification16:24:58UTC: about1h34m30s from request and
1h24m29s from development approval. These are elapsed durations, not billed usage.
Keep published tags/assets immutable. Future feature requests require their own scope.
