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
