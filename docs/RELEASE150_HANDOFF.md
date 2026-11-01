# v1.17.150 Beta 14 - publication verified

Published 2026-11-01T05:46:46Z as GitHub prerelease ID 430388745.
Release: https://github.com/TechSavage1989/network-toolkit/releases/tag/v1.17.150
Source/tag: c68d31e4cb72dcce4cc17435d4e5447961778db7.
Merged PR #26. Official Release workflow 41277868119: build SUCCESS and publish SUCCESS.

Four official GitHub assets and reported SHA-256 digests:
- Network_Toolkit.exe (21117952 bytes): 362d4b1e1e08dc07a8c07c8dc620070c3e7bd2a1370211cc0a76e305466fa38e
- Network_Toolkit_1.17.150_Portable.zip (14526893 bytes): 1b523e0498cf261386b9de19a6642359a05bd6d9d1a824e779f8dbe45e9b4be3
- Network_Toolkit_1.17.150_Update.zip (8069379 bytes): 72130c5c2f1c7d5ce716d2f1af8a4e49c0d02fe0b22c2d0fefcf41c5aad7c538
- SHA256SUMS (292 bytes): 65c4cfa4ed9a36d5f4bd146c60fc14a74497ae883c76e5d4776d265ea0e5a83b

Stable releases/latest remains v1.17.136; Beta 14 remains opt-in.
The latest release source passed its official Go, JavaScript, Python, Windows-resource, packaging and publication gates. A later documentation/cleanup commit on main does not alter the published tag or assets.
The per-response SFTP timeout guard prevents late callbacks closing a completed SFTP channel; a repeated regression test was added. Existing authentication, network permissions, settings, terminal ownership, fixed Network_Toolkit.exe and user update approval are preserved.

IMPORTANT: A historical Editor 390px common-row motion test remains outside its original 2 CSS pixel threshold. Live-device SFTP interruption, physical Windows multi-hour soak and full eleven-area resource acceptance remain open. At handoff update the full Checks workflow 41277834593 was not yet completed; do not report all CI green. This beta does not qualify for stable promotion.

Publication used the ordinary guarded GitHub release workflow. One-time candidate tag/resource helper workflows were subsequently removed from main, without rewriting the published tag or release.

## Next proposed Beta 15 v1.17.151

Continue all eleven original roadmap areas. Prioritize the 390px Editor continuous motion/sidebar defect with the 2px threshold intact, real-device SFTP interruption/cancel acceptance when authorized, and extended simultaneous-tool Windows resource testing. Preserve saved settings, credentials, permissions and portable updater.
Keep What's New plain-language and user-focused; put technical notes under Additional Details.

Previous candidate record follows, preserved as development history.

# Network Toolkit v1.17.150 Beta 14 - development candidate (NOT PUBLISHED)

Based on published v1.17.149 commit bf76d7506393d4a1902c55656a5490897abf8922.
Branch: beta/1.17.150-editor-map-precision. Preserve the eleven-area roadmap, Windows updater, user settings, terminal credentials, permissions, and established release process.

## Candidate scope
- Guard the shared-connection SFTP timeout callback against races after a successful response has returned.
- Added a twenty-cycle delayed-timer regression validating that already-completed responses are never aborted.
- Prepare application version, Windows resources and readable release notes for v1.17.150.
- No intentional modification to Editor rendering or the existing 390px motion acceptance threshold.

## Important gaps
- The historical Editor 390px continuous-map motion check failed in Beta 13 at the original 2 CSS pixel requirement. Not fixed or waived.
- Live-device SFTP interruption, physical Windows multi-hour soak, and broader eleven-area resource benchmarks remain pending.
- Full candidate CI results, release workflow, artifact checksums and updater-feed verification must be recorded before declaring publication.

Do not replace prior releases. Never publish as stable without the required acceptance. Keep user-facing What's New plain-language, and technical details under Additional Details.
