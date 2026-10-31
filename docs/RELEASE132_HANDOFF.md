# Network Toolkit 1.17.132 - published Reconnect all and clearer update notes

Published 2026-10-31T04:50:09Z, release ID 429728832.
https://github.com/TechSavage1989/network-toolkit/releases/tag/v1.17.132
Source/tag db432733d9b5065d782fbd0685c5fcf78a6011e4.
Tree 2e66bdbddc63ee594047b124708a083c394a15b4.
GitHub latest release resolves 1.17.132 and its tag matches source/main
at verification. The existing release workflow 41169978531 completed both
build and publish jobs successfully without overwriting a previous release.

## Delivered

The active terminal's Reconnect action retains its single-session behavior.
A small adjacent arrow opens Reconnect all as a secondary action below.
Reconnect all also appears in the terminal tab right-click menu. It requires
confirmation, warns if live sessions will be restarted, runs sequentially,
rechecks ownership and session revisions, skips busy/unowned/other-window
sessions, and pauses for interactive authentication. Tabs and retained
output are preserved; active work or unsent commands can be interrupted.

The published release body now has explicit What's New and Additional Details
sections. The first shows short user-impact summaries, the second preserves
technical behavior and compatibility in an expandable view. Older unstructured
releases remain readable. Future releases follow docs/RELEASE_NOTES_STYLE.md,
also linked from AGENTS.md and docs/DEVELOPMENT.md.

## Validation and retained limits

Exact-source release packaging passed 1,497 JavaScript tests, Go package tests,
the Windows AMD64 GUI build, resource generation and portable ZIP source/member
validation. The versioned review core included 42 Python tests. Focused
rendered Reconnect all tests passed at 1280px and 390px, including tab menus,
keyboard dismissal, cancel protection, current-window targets and sequential
synthetic reconnects. The native Windows terminal, security and dependency
review jobs passed.

PR #7 review run 41168981266 recorded an existing unrelated failure in
browser-editor112: detached 390px comparison motion p95/max peer offset
3.65625px with 40 input events, zero stalls and zero unanswered inputs.
No Editor comparison implementation or test threshold was changed; root
cause remains unresolved and no repair is claimed. Source push run
41168977300 passed the Editor browser job independently. The final PR run 41168981266 completed 13 SUCCESS / 1 FAILURE;
its long Chromium updater-timer browser check passed after publication.
The exact same-source push run 41168977300 completed all 14 jobs SUCCESS.
The PR's narrow Editor motion failure remains recorded and is not resolved
or erased by the successful source push.
These mocks do not establish compatibility with every physical device,
full native user workflow, or independently measured speed improvement.

## Published assets (GitHub-reported SHA256)

- Network_Toolkit.exe, 21,061,120 bytes:
  3c8a6509432f4c50cd2093b87dfb3ffad5cd489add58a0eba5265600f11489cd
- Network_Toolkit_1.17.132_Portable.zip, 14,312,384 bytes:
  725ad26d930ab3925974cbd8d8f684f2380e54587b65826a7d50fd9b04c2e409
- Network_Toolkit_1.17.132_Update.zip, 8,051,849 bytes:
  0b924e20ebb1a987587a8d08d1b2645402037b8e54cf2f3e5e303823348273bb
- SHA256SUMS, 292 bytes:
  d1b83f8cbe24939302297ff9cd4d35defdb8bcfa102e5768653a7c902766a334

The release packager validated a 1,259-member portable source archive and
the strictly allowlisted four-member binary-only update ZIP. All four assets
exist in the public stable release with nonzero sizes and GitHub digests.
No independent byte-for-byte public download was made here; do not claim
an external download hash check from metadata alone.

Previous v1.17.131 release ID 429714081, source/tag
9152e41f278f35719cff18993730f25a25050237, and its original four
asset IDs, digests, sizes and timestamps remain unchanged. The release
retains Network_Toolkit.exe as the stable name, preserving updater/shortcut
and settings behavior. No visibility, credential or authorization changes.

