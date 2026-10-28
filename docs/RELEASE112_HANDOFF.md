# Network Toolkit 1.17.112 published and independently verified

Published 2026-10-28T22:23:34Z:
https://github.com/TechSavage1989/network-toolkit/releases/tag/v1.17.112
Release ID: 427253456
Exact reviewed/published source and tag: 4d05f6a5a4eb1c315d0a19ccf2eb8daedf2dcb1c
Tree: 3f9278fe6a5a3b1781d71efbd9354e4decf7ac8e
Baseline: completed111 main 977adbfb9d7c4d0129306477033bdc3e40ad59d0.
Publication and release were explicitly authorized by the user. Version metadata
and Windows resources advance together. The 111 release note is archived unchanged
under docs/releases; 112 is the sole root release note. This later docs-only receipt
does not change the published source, tag or assets.

## Delivered behavior

Read [EDITOR_VIEWS112.md](EDITOR_VIEWS112.md) for the full behavior and limits.
Toolbar More/Options align with their button row. Tabs sit directly above their
own text pane, below all shared controls. Normal split editing and comparison use
independent left/right groups. Extra opens stay left unless explicitly moved.
Context menus, keyboard reordering and actual pointer drops move individual tabs;
swapping compares only the selected pair. Layout survives editor-window transfers.

Files adds a resizable Windows/SFTP sidebar using the established file manager.
It includes navigation/history/breadcrumbs, filter/sort, keyboard selection and
shortcuts, new files/folders, rename/move, copy/cut/paste, preview, confirmed delete,
upload/download, progress/activity and cancellation of pending work. Compact
padding, control heights, themes and a narrow overlay match the existing app.
Opening or closing the sidebar does not disconnect an existing terminal session.

Detached editors request main-window approval before file management. Approval
captures existing SFTP transports and can be revoked. It does not grant credential
management, settings, connection control or generic text writes. Owned documents
continue saving through their existing leases. Already accepted in-flight explorer
operations may finish after revocation. Mutation reservations protect registered
open document paths and Save As destinations. Remote symlink aliases and local
hard-link identity are not certified; revision-conflict checks remain in force.

## Hosted validation

All seven exact-source review jobs passed: https://github.com/TechSavage1989/network-toolkit/actions/runs/40776539772
- dependency-security: SUCCESS (job 123127759883).
- windows-terminal: SUCCESS (job 123127760131).
- browser-editor112: SUCCESS (job 123127760260).
- browser-security: SUCCESS (job 123127760356).
- browser-workflows107: SUCCESS (job 123127760371).
- browser: SUCCESS (job 123127760394).
- check: SUCCESS (job 123127760453).

The application gate passed eight Go packages, 1,042 JavaScript tests, 42 Python
release guards, current metadata/resources and the Windows GUI build. Existing
dependency, credential, terminal, updater, graph and native editor gates remain.
The new browser gate exercised main/detached/modal editors; actual tab dragging;
independent groups, undo/redo, new-left placement, comparison/swap/replacement;
layout restoration and first typing in an empty view; local/SFTP file operations;
keyboard navigation; sidebar resizing/reopening; and 1440/820/390px light/dark layouts.
Synthetic fixtures were used. Go tests separately exercised real temporary local
files and a local synthetic SFTP server, approval/revocation, stale transports,
unrelated-capability denial, document leases and mutation reservations.

Downloaded hosted screenshots were inspected. Native Windows/WebView2 tests ran
only on isolated hosted runners. No local desktop automation, test windows,
interruption of normal sessions, real user files/settings/credentials or live
devices were used. Browser emulation does not certify physical multi-monitor DPI,
trackpad hardware, reboot, installation or a user's particular device firmware.

Earlier review attempts exposed sidebar overlap, missing embedded asset entries,
stale selection-action state, selected-tab visibility after resize and the empty-view
tab-limit edge case; all were corrected. Real drag testing also found that the
separate-window drop target covered compact detached tabs. Its position now follows
the tab strips, and the tests prove a same-window view move does not detach a tab.
The initial cleanup hypothesis was superseded by this observed overlap diagnosis.
Native Windows input then exposed an 18px edge-autoscroll shift during tab detach;
tab gestures now retain pane scroll positions, with the exact-state native assertion
unchanged and a focused regression test added.
A legacy global-tab reorder
assertion was updated for independent groups. The alignment fixture now compares
buttons within their actual wrapped row. Superseded/cancelled runs are not counted
as passed. The final required gates were not skipped, waived or relabeled.

## Measured before and after

The paired hosted fixture loads the exact published 110 UI and the final 112 UI with
the same synthetic documents. Heights below are visible text-pane heights in CSS
pixels with Files closed. Both themes passed; this table lists light-theme values.
The Files-open screenshots separately show the requested sidebar and split views.

Mode | Viewport | Document | Before 1.17.110 height | After 1.17.112 height | Gain
--- | --- | --- | ---: | ---: | ---:
main | 1440 x 1000 | Text | 610.1px | 666.8px | +56.8px
main | 1440 x 1000 | Compare | 454.6px | 610.3px | +155.8px
main | 390 x 1000 | Text | 556.8px | 603.8px | +47.0px
main | 390 x 1000 | Compare | 329.8px | 430.3px | +100.5px
main | 820 x 540 | Text | 164.8px | 234.8px | +70.0px
main | 820 x 540 | Compare | 28.3px | 110.3px | +82.0px
detached | 1440 x 1000 | Text | 649.1px | 776.8px | +127.8px
detached | 1440 x 1000 | Compare | 493.6px | 686.3px | +192.8px
detached | 390 x 1000 | Text | 556.8px | 640.8px | +84.0px
detached | 390 x 1000 | Compare | 329.8px | 467.3px | +137.5px
detached | 820 x 540 | Text | 201.8px | 271.8px | +70.0px
detached | 820 x 540 | Compare | 52.5px | 147.3px | +94.8px
modal | 1440 x 1000 | Text | 749.0px | 788.0px | +39.0px
modal | 1440 x 1000 | Compare | 593.5px | 732.0px | +138.5px
modal | 390 x 1000 | Text | 672.0px | 703.0px | +31.0px
modal | 390 x 1000 | Compare | 470.0px | 534.0px | +64.0px
modal | 820 x 540 | Text | 237.4px | 307.4px | +70.0px
modal | 820 x 540 | Compare | 69.9px | 183.4px | +113.5px

All raw measurements and actual before/after screenshots are retained in the
user-facing evidence archive. No mockup or reconstruction is presented as a test.

Automatic Release: https://github.com/TechSavage1989/network-toolkit/actions/runs/40779370901
- build: SUCCESS (job 123136830956).
- publish: SUCCESS (job 123137530255).

All seven duplicate main jobs also passed: https://github.com/TechSavage1989/network-toolkit/actions/runs/40779370968
- browser-editor112: SUCCESS (job 123136831990).
- check: SUCCESS (job 123136832223).
- browser-security: SUCCESS (job 123136832257).
- browser: SUCCESS (job 123136832358).
- windows-terminal: SUCCESS (job 123136832378).
- dependency-security: SUCCESS (job 123136832398).
- browser-workflows107: SUCCESS (job 123136832538).

## Public download verification

Independently verified at 2026-10-28T22:24:00.327789+00:00. All four asset names, sizes, GitHub
digests, SHA256SUMS, nested manifests, ZIP CRCs and entry allowlists agree. The
latest release, exact source tag and release-note body agree. The portable package
contains 1,017 source files byte-identical to the reviewed commit.
Public EXE and portable ZIP match the hosted review artifact byte for byte.
The AMD64 GUI executable contains version 1.17.112, all 9 canonical icon payloads
and 12 native resources. The update ZIP contains only its four allowlisted runtime
files. Previous 109/110/111 tags, release bodies and asset IDs/sizes/digests remain
unchanged, including 109's authorized Toolkit-Icons.zip extra. Repository access,
visibility and the retired update feed were not changed.

- Network_Toolkit.exe: 20,102,656 bytes; SHA-256
  fdd26bc874e1f442270d2965696f144bba1c8663e3d2860348115fd51ba9d783
- Network_Toolkit_1.17.112_Portable.zip: 12,994,489 bytes; SHA-256
  dfbf6cd35d0880ea6d012916df85b3d2a7ca204db719a8fc37bb7fa2bddbb143
- Network_Toolkit_1.17.112_Update.zip: 7,723,515 bytes; SHA-256
  b52d5dfdbc3f3057c2139d99e3f31f16720bd969918ad99789779c340321e565
- SHA256SUMS: 292 bytes; SHA-256
  b22c8e48a47dbd426412daac4224053e5c1c0abb9efa8f264661d3b7ac957baf

Receipt prepared 2026-10-28T22:40:30.769590+00:00. Preserve all published tags/assets. Future local visual
testing requires advance approval for a clearly scheduled session. Never stop
normal user app sessions or discard unsaved work. Final handoff-only records use
[skip ci] and do not trigger another build.

Elapsed from original request: 3h 6m 8s (started 2026-10-28T19:34:22+00:00).
Elapsed from original implementation start: 3h 3m 29s (started 2026-10-28T19:37:01.104774+00:00).
These wall-clock totals include the original 111 release and this 112 follow-up, hosted CI and asset verification; they are not active typing time.
