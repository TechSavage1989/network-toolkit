# v1.17.147 Beta 11 - published release handoff

Published v1.17.147 Beta 11 on 2026-11-01T04:17:17Z as opt-in prerelease ID 430357367.
Release: https://github.com/TechSavage1989/network-toolkit/releases/tag/v1.17.147
Tag/source and merged PR #22: b9bdaf40a099be557c09c8586c90bb687ea34340.
Release workflow 41272278556: build SUCCESS and publish SUCCESS.
Four release assets with GitHub SHA-256 digests; normal stable releases/latest remains v1.17.136.
SFTP sends complete protocol frames across partial writer writes, with regression tests for 1-, 3- and 7-byte chunks and no-progress failure.
Credentials, settings, network permissions, protocol authorization and Windows portable updater behavior remain unchanged.
Checks run 41271825934 had 11 successful job groups with four hosted browser/Windows groups still pending at the publication verification point. Do not claim all CI passed. Historical Editor 390px motion regression, live SFTP interruption testing and physical Windows multi-hour endurance remain open.
No previous release tags/assets were replaced.

## GitHub asset metadata

| Asset | Size | SHA-256 |
| --- | ---: | --- |
| Network_Toolkit.exe | 21112320 | 8a12d29d4e868db78d4129cdb9ad76ca3a3a83dc11ac8ed5c752c9cd4276b8f1 |
| Network_Toolkit_1.17.147_Portable.zip | 14495389 | 627c946f8fcb3e1d02cb08c738fc4fe2ee4c04c7795cb933b063791fb275549e |
| Network_Toolkit_1.17.147_Update.zip | 8067503 | 3342ca5e8c22b8fde4193bd24bd477410335a85c21fd344793a5ca93b09421c3 |
| SHA256SUMS | 292 | a7eb69b52132b4aa691169ed39313db04dade4158a377324852c1087e84b7e6f |

GitHub metadata and package-verification workflow passed. Public asset bytes were not independently downloaded and rehashed in this session. Beta feed and GitHub prerelease registration are verified; stable releases/latest still resolves v1.17.136. Exact release tag points at the merged source. The historical Editor 390px motion threshold remains 2px and has not been relaxed.

# Network Toolkit v1.17.147 Beta 11 - candidate, not published

Baseline: v1.17.146 Beta 10. Stable: v1.17.136.
Branch: beta/1.17.147-sftp-framing-reliability.

## Scope
SFTP protocol frames are now fully written even when an underlying writer accepts only part of a frame. A short-write regression test validates 1, 3, and 7-byte increments. This is a transport-framing reliability fix, not a claim that live SFTP interruption, cancellation or recovery is fully accepted.

## Eleven-area status
Application interface: unchanged. Network diagnostics: SFTP transport robustness improved. Network discovery: deferred. Graphs and monitoring: deferred. Packet capture: deferred. File transfers: SFTP frame reliability improved. Text Editor: 390px motion failure remains. Terminal: unchanged. Reports and results: unchanged. Network configuration: no permission change. Settings/updater: beta opt-in, update consent and stable EXE filename retained.

## Release gates
Exact-source Go, JS, Python, hosted Windows/browser/security tests, version-resource checks and release asset verification are required. Do not weaken or conceal the historical Editor failure. Physical Windows multi-hour endurance and native SFTP interruption remain open. Do not change old releases or use the user's desktop.
