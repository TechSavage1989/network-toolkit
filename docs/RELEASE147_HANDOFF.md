# Network Toolkit v1.17.147 Beta 11 - candidate, not published

Baseline: v1.17.146 Beta 10. Stable: v1.17.136.
Branch: beta/1.17.147-sftp-framing-reliability.

## Scope
SFTP protocol frames are now fully written even when an underlying writer accepts only part of a frame. A short-write regression test validates 1, 3, and 7-byte increments. This is a transport-framing reliability fix, not a claim that live SFTP interruption, cancellation or recovery is fully accepted.

## Eleven-area status
Application interface: unchanged. Network diagnostics: SFTP transport robustness improved. Network discovery: deferred. Graphs and monitoring: deferred. Packet capture: deferred. File transfers: SFTP frame reliability improved. Text Editor: 390px motion failure remains. Terminal: unchanged. Reports and results: unchanged. Network configuration: no permission change. Settings/updater: beta opt-in, update consent and stable EXE filename retained.

## Release gates
Exact-source Go, JS, Python, hosted Windows/browser/security tests, version-resource checks and release asset verification are required. Do not weaken or conceal the historical Editor failure. Physical Windows multi-hour endurance and native SFTP interruption remain open. Do not change old releases or use the user's desktop.
