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
