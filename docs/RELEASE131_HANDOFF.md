# Network Toolkit 1.17.131 - published Ctrl+A and terminal responsiveness

Published 2026-10-31T04:12:11Z. Release ID 429714081.
https://github.com/TechSavage1989/network-toolkit/releases/tag/v1.17.131
The exact source/tag is 9152e41f278f35719cff18993730f25a25050237;
tree 9d29bdb49a7819e3e434448bf3ebc70e2a51d0b5.
PR #5 source merged as 80e925494521c5dd8c73a5838341b1b4e6881fbe,
then PR #6 versioned source merged with the release commit above.

## Delivered behavior
Ctrl+A selects only the guarded echoed editable input range, including wrapped
CLI commands, not the prompt or earlier output. An unrecognized, unsafe,
full-screen, password or otherwise blocked edit retains the previous device
control-key behavior. No selection success sends remote command input.
Multi-terminal UI work is reduced by frame-coalesced active toolbar refreshes,
delayed cosmetic output-only chrome refreshes, suspended hidden-tab highlight
scans, no-op fit render avoidance and direct CSS splitter sizing. Session and
credential identity, input queues, transfers, privileges, settings and the
stable Network_Toolkit.exe updater contract remain unchanged. No physical
user-device speed benchmark, live SSH/serial device or visible-UAC proof.

## Acceptance and retained failure
Versioned review PR #6 run 41166017842 finished 13 SUCCESS / 1 FAILURE.
Core ran 1,488 JavaScript tests, 42 Python tests, Go package tests, Windows x64
build and metadata validation successfully. Windows terminal/native
130/129/127 tests, browser terminal 130/129/127, dependency, security, workflows
and browser updater tests passed.

The one failed job browser-editor112 recorded two narrow-390px comparison motion
cases, main left-map p95 4.140625px/max 4.609375px and detached right-map
p95/max 3.65625px; both delivered 40 inputs, zero stalls and zero unanswered
inputs. This legacy alignment behavior is unresolved. It was neither patched
nor excluded, and no fixture-only explanation is claimed. Prior source review
push 41164495366 passed all 14 jobs; PR counterrun 41164500428 failed a
separate 390px Editor gear-focus wait, while same-source push passed that case.
The original failures remain recorded, not erased by independent successes.

New release workflow 41167413746 completed both build and publish jobs
successfully. The release build regenerated 12 native resource leaves, ran
1,488 JS tests successfully, and verified/packaged the 14,293,665-byte portable
archive with 1,255 safe members, plus the strictly four-member binary-only
updater. The publishing job fetched the exact reviewed build artifact, then
published four assets without same-version overwrite.

## Public GitHub verification and prior immutability
The public release is stable, not draft/prerelease; release/latest resolves
v1.17.131 and the tag points to published source SHA above. At publication main
pointed to the same SHA. All four published assets have GitHub-reported
SHA-256 digests and nonzero sizes:
Network_Toolkit.exe (21,053,440 bytes):
332ec156e783fc553e0f322f10cb4b6331a60563c10163a780d14cf140805486
Network_Toolkit_1.17.131_Portable.zip (14,293,665 bytes):
efbff41a853e6f63666c3fb3936470fea12a3daff9b8d4934794a84094388e51
Network_Toolkit_1.17.131_Update.zip (8,048,777 bytes):
c71b3e9f51a781360e20264278eaeb30df8b1159f6ded891e9d03358bf624f64
SHA256SUMS (292 bytes):
f272fbe200b16d2474c02c5dca8dde5359aaa16c339296961721989229eded34

The earlier v1.17.130 release 429690714, source/tag
77ceeec0b9b258dc7e3a1dfc29f9992ad3d5d884 and its four original
asset IDs, sizes, SHA256 digests and timestamps match the pre-release snapshot
(download counts excepted). The new version retains all old releases.
This check verified public GitHub release metadata and the exact reviewed
package workflow. A separate external byte-for-byte download of each public
asset was not performed in this session, so no independent public-download
hash proof is claimed.

This closure is documentation-only. It neither changes v1.17.131 published
source/tag/asset bytes nor authorizes unrelated features or access changes.
