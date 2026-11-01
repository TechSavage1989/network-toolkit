# Network Toolkit v1.17.148 Beta 12 - candidate, not published

**Status:** Source development and exact-source CI pending. Never claim a published release until the tag, four assets and channel are verified.
**Baseline:** v1.17.147 Beta 11 (b9bdaf40a099be557c09c8586c90bb687ea34340), stable v1.17.136. Development starts from the Beta 11 publication documentation on main.
**Branch:** beta/1.17.148-sftp-cancel-cross-tool-reliability.

Read AGENTS.md, docs/DEVELOPMENT_ROADMAP.md, docs/DEVELOPMENT_HANDOFF.md, this handoff and docs/BETA12_RELIABILITY_AUDIT.md. Preserve the original eleven areas.

## Implemented scoped changes

- Borrowed-terminal SFTP operations check cancellation before requesting a new subsystem, after negotiating SFTP, and before reporting success. A cancelled request that closes the channel and returns a late EOF is reported as a cancellation rather than a misleading transport failure or success.
- A synthetic SSH server blocks a directory listing; the regression cancels it, checks the borrowed SFTP channel closes, verifies the active interactive SSH shell still writes, then opens another SFTP subsystem on the same authenticated connection. A pre-cancelled request must not open another channel.
- A new extended six-tool synthetic snapshot scenario alternates 800 job-list refreshes, multiple UI pages, and 21 simulated engine interruptions. It verifies bounded retained result views, successful recovery after interruptions and unchanged saved filter preferences. It reports elapsed time, process CPU and memory deltas, not speed claims.
- Update the development-preview version to the application version, add a guard test, and retain unchanged Windows portable EXE/update packaging semantics.

No new credentials, connection permissions, scan targets, server capability, terminal ownership, source document content, history, updater consent, or update distribution access are authorized or changed.

## Eleven-area audit and acceptance

| Area | Beta 12 status |
| --- | --- |
| Application interface | Synthetic four-view navigation and shutdown/late-response gates. No layout redesign or native window soak. |
| Network diagnostics | Synthetic simultaneous Ping, Trace route, DNS and TCP interruption/recovery tests extended. No real network traffic. |
| Network discovery | Discovery result retirement covered by synthetic job-list cleanup. No live subnet scan. |
| Graphs and monitoring | Monitor job included in repeated six-tool snapshots. Native graph CPU and long physical soak not measured. |
| Packet capture | Existing browser/Windows regressions retained. No live sustained capture measurement. |
| File transfers | Scoped SFTP session cancellation checks added. Physical-device transfer cancellation and FTP/FTPS/TFTP interruption acceptance remain pending. |
| Text Editor | Existing source, 390px comparison-motion and sidebar geometry tests retained, 2px requirement unchanged; no Editor fix claimed. |
| Terminal | Synthetic shared SSH test checks active shell survives cancelled SFTP and a subsequent SFTP session. No live SSH/serial test. |
| Reports and results | Synthetic result-cache retention and saved-filter persistence checked through interruptions. Exact exports unchanged. |
| Network configuration | No change to adapter/Hosts permissions or operations; existing tests retained. |
| Settings and updater | Version metadata and preview fixture aligned. Beta opt-in, user approval, stable Network_Toolkit.exe and stored settings retained. |

## Validation and gates

Candidate CI must run full Go, JavaScript, Python, security, hosted browser, Windows-native tests, Windows x64 build/resource verification, portable/update ZIP and stable/beta feed checks. Preserve all failures and original Editor thresholds; Beta publication does not certify stable readiness. No real SFTP host, physical Windows multi-hour soak, or live network benchmark was performed. The source version is prepared as 1.17.148; no old tag or assets may be rewritten.

The previously published Beta 11 main Checks run 41272278565 finished with 14 successful groups and the unchanged browser-editor112 failure. Real-device and original Editor regression remain open.

## Next after publication

If Beta 12 publishes with known Editor failures retained, follow-up Beta 13 should focus on the original 390px comparison motion/sidebar defects, native SFTP interruption and long-running Windows/resource acceptance across the eleven areas. RC/stable promotion requires verified physical and integration acceptance, not a fixed beta count.
