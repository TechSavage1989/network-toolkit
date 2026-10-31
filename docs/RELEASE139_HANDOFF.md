# Network Toolkit v1.17.139 Beta 3 - publication verified

Published: 2026-10-31T15:12:33Z, GitHub release ID 430009298, prerelease=true, draft=false.
Release: https://github.com/TechSavage1989/network-toolkit/releases/tag/v1.17.139
Reviewed source / merged PR #14 / tag: 28d51f149b3246a8b824a90e515f79064b3298b4
Build and publication: workflow 41213506790, both jobs SUCCESS.

## Delivered and preserved

- Per-session bounded output-only xterm parser write batching, normally no more than 64 KiB per write. Preexisting individual chunks larger than 64 KiB are not split.
- Original per-event output and macro observation and byte order retained. Initial buffered output is rendered in batches. Status and authentication transitions remain immediate.
- The 1 MiB pending-render cap, warning when display output is omitted, scroll/follow controls, terminal resize behavior, sessions, credentials, settings, user-input pacing and the stable executable/updater contract remain unchanged.
- This is an opt-in beta and does not replace latest stable v1.17.136.

## Release validation and assets

The exact-source release workflow passed 44 Python tests, 1,529 JavaScript tests with 0 failures, application Go tests, Windows x64 GUI build, Windows icon/version resources, portable source package verification (1,281 members), and the binary-only update package checks.

Published GitHub asset digests:
- Network_Toolkit.exe: 21,096,448 bytes, sha256 7def98130f5bc933e256b712ee812e2d2861d60804b0253e5fc3714d3a3df59b
- Network_Toolkit_1.17.139_Portable.zip: 14,394,656 bytes, sha256 a23bfd06571ea4444d9290e175a246059c44fc4740eb59d0092994143f970ce5
- Network_Toolkit_1.17.139_Update.zip: 8,062,909 bytes, sha256 883096196f95a4599f1f38c2e847ae1ca2a9b9e531065b8f696c47d25626441d
- SHA256SUMS: 292 bytes, sha256 6d7a6b8a8757b957ddbcef2cb6033fdf337075dfa0138005e49ab4e58d319fd3

GitHub release metadata confirms all four assets and their SHA-256 digests; there was no independent public asset download and rehash in this documentation closure. The tag and main merge source agree. GitHub's releases list puts Beta 3 first and releases/latest remains stable v1.17.136. Original v1.17.138 assets, IDs and hashes remain unchanged.

## Limits and separate CI

Release workflow passed both build and publish jobs. Broad PR and post-merge Checks workflows were still in progress at this handoff, and are not counted as passing. PR duplicate checks 41212742845 reported an Editor comparison/two-pane motion assertion failure while unrelated jobs were running; this beta did not change or claim to repair that historical Editor behavior. No physical SSH/Telnet/Serial hardware load benchmark or independently measured frame-time improvement was performed. Synthetic concurrent-session fixtures do not establish universal output-display performance.

The original candidate plan below is retained as historical context. It is not an outstanding publication request. Release v1.17.139 is already published; no published tag, asset, checksum, note or executable was changed in this documentation-only follow-up.

---

# Network Toolkit v1.17.139 Beta 3 - high-output rendering candidate

## Baseline and scope

Based on published v1.17.138 Beta 2 (tag 8e9052eb14ab9a5a5fc9363b0ac710a36203a133).
Current development PR: https://github.com/TechSavage1989/network-toolkit/pull/14
Stable v1.17.136 and prerelease v1.17.138 remain unchanged.
This is a candidate, not a published beta.

- Coalesce output-only event bursts separately by session into bounded 64 KiB xterm parser calls.
- Preserve exact byte order and per-event macro/output-history observations.
- Batch previously buffered terminal output when opening a display.
- Keep status changes, authentication prompts and non-output transitions immediate.
- Retain the existing 1 MiB pending-render cap, output-loss warning, scroll/follow controls, session ownership and updater opt-in safeguards.
- No change to SSH, Telnet, Serial, local shells, credentials, network permissions, or saved profiles.

## Acceptance and remaining work

- Validate scripts/terminal-beta139-output.test.cjs and existing scripts/terminal-batch-render-performance.test.cjs, terminal-output-performance, terminal-fit, terminal-resize, output history and pane tests.
- Run full JS, Go, Python, browser/native Windows, security and package checks against the exact candidate.
- Validate Go/UI version 1.17.139, Windows COFF resources/icon, stable Network_Toolkit.exe filename and binary-only updater allowlist.
- If and when approved for publication, tag exact reviewed source v1.17.139 and publish with prerelease=true; verify all four assets/checksums and the opt-in beta update feed. Do not replace older tags/assets.
- Independent high-volume physical SSH/Telnet/Serial benchmarking remains unperformed. Historical Editor comparison/drag, PowerShell echo and native SFTP test failures remain outside this scoped change and must not be treated as fixed.

PUBLICATION PENDING.
