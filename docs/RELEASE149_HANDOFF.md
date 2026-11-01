# v1.17.149 Beta 13 - published release handoff

Published 2026-11-01T05:06:03Z as opt-in GitHub prerelease ID 430374696.
Release: https://github.com/TechSavage1989/network-toolkit/releases/tag/v1.17.149
Tag/source and merged PR #24: bf76d7506393d4a1902c55656a5490897abf8922.
Official Release workflow 41275438541: build SUCCESS, publish SUCCESS.
Stable releases/latest is still v1.17.136. The release is prerelease=true and draft=false.

## Four official assets

| Asset | Bytes | GitHub SHA-256 |
| --- | ---: | --- |
| Network_Toolkit.exe | 21117440 | 327ae7b8c0c51e3b6688c50ccddbdaf9ac7273fcf389f145942009f63f14cf7d |
| Network_Toolkit_1.17.149_Portable.zip | 14517418 | c9ecf2f1fea02d27638c243295886a317543ac3e3867a2118aa4e09d067b1819 |
| Network_Toolkit_1.17.149_Update.zip | 8068921 | c0c16c753fade872d1d49f20e8a47c5729d567033df5ca1d08c538beacee00d1 |
| SHA256SUMS | 292 | 2e75041fe3532ee129c9512c5e35fba0d0c88029ab55d5de013e3c2e192ffabf |

These are GitHub-reported asset digests; public bytes were not
independently downloaded and rehashed in this chat. The release
workflow verified packaged files and hashes. The tag target and
GitHub release source match the merged PR. Previous tags/assets
were not replaced.

## Delivered changes

A borrowed Terminal SSH session now uses a new timeout for each SFTP
protocol response. A hung file-browsing response closes only that
SFTP subsystem channel and returns a deadline error. The same
authenticated terminal remains available, and the timer does not
restrict total large-transfer duration. The SFTP timeout regression
passed on hosted Windows as well as the core Go suite. Host-key trust,
credentials, settings, network permissions, terminal ownership,
transfer approval and the user-controlled updater were unchanged.

## Actual integration results and limitations

Candidate Checks 41275225328: 11 job groups passed, four hosted
browser/Windows groups still running at publication verification
(browser-editor112, browser, windows-terminal, browser-workflows107).
Main Checks 41275438525: 11 groups passed, four similarly pending.
The specific new Windows SFTP timeout test passed. The broader
Windows job was not yet completed when this record was written.
No claim of a fully green CI matrix is made.

The original 390px Editor comparison-motion test remains unchanged
at a 2 CSS pixel acceptance threshold. Earlier betas reproduced
3-5 pixel deviations and this beta does not claim to fix them.
Real-device SFTP server timeouts/cancellation, physical Windows
multi-hour endurance and full simultaneous network-tool benchmarks
remain required before stable promotion.

## Roadmap continuation

Preserve the original eleven-area performance/interface/reliability
plan in docs/DEVELOPMENT_ROADMAP.md. The next proposed release is
v1.17.150 Beta 14, focused on Editor motion/sidebar regression,
native SFTP interruption and physical Windows multi-tool soak.
Earlier candidate handoff follows.

# v1.17.149 Beta 13 - release candidate handoff

**NOT PUBLISHED.** Based on v1.17.148 (published source
72d11fd8ffd5ccf719a028e7786dbc7b26868d75), stable v1.17.136.
Branch: beta/1.17.149-shared-sftp-timeout-reliability.

## Implemented changes

- Per-response idle timeout applies to SFTP exchanges on a borrowed
  authenticated terminal SSH connection. Timeout closes only the SFTP
  subsystem and returns a deadline error.
- Per-response timer resets for subsequent requests instead of imposing
  an overall transfer duration limit. No shared SSH socket deadline change.
- Isolated synthetic SSH test covers stalled directory reply, timeout,
  scoped channel cleanup, shell continuity and a new SFTP request.
- Hosted Windows CI runs this same SFTP regression.
- Versions, embedded Windows resource, portable package and root release
  notes prepared for 1.17.149.

## Preserved behavior and open gaps

Credentials, saved options, permissions, terminal ownership, SSH trust,
transfer approvals, report results, fixed Network_Toolkit.exe, beta opt-in
and explicit update installation consent remain unchanged. Original
eleven-area roadmap is in docs/DEVELOPMENT_ROADMAP.md.

The Editor 390px common-row comparison motion still exceeds its original
2px requirement; not a fixed regression or waived test. Real SFTP device
interruption, native cancellation and physical Windows multi-hour soak
remain open. Synthetic Windows tests are not real-device proof.

## Release verification required

Record exact GitHub branch/merge/tag SHA, CI job successes and failures,
Windows build, four official assets, checksums and beta/stable feeds. Never
relabel pending checks as passed or overwrite previous releases.
