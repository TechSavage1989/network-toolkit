# Network Toolkit UX Phase 5 - v1.18.4 Beta 4 published

Status: Published opt-in prerelease https://github.com/TechSavage1989/network-toolkit/releases/tag/v1.18.4 on 2026-11-01T20:33:03Z.
Merged PR #36; tagged source 977e9bf6a954a16be2d8f74e8f6bd5f61fccbb19. The original v1.18.3 Beta 3 release is unchanged. Stable latest remains v1.17.136.

## User-facing changes delivered
- First-run welcome appears only after saved preferences load successfully; explains what the app does and offers Check a device, Open a terminal and Transfer files. A quick start opens the tool but does not connect, scan or modify the system.
- Welcome can be dismissed and is remembered in the existing saved state as a single boolean, without changing any credential, vault, profile or updater data.
- Welcome can be reopened with F1 Help or command search. Detached windows do not interrupt users with a welcome.
- Empty diagnostic results explain the next input or action with a safe example. Existing no-result screens for Terminal, Editor, transfers, profiles and reports remain intact.
- Failed diagnostic results keep the original error and add an actionable next step. SSH host keys, certificates, file permissions and Windows/UAC approval are not bypassed.
- Device check, Ping, Trace route and DNS page descriptions use clearer language. No diagnostic engines or raw report records were changed.
- The in-app What's New format remains brief and user-facing, with technical information in Additional Details.

## Publication and test evidence
- Official guarded Release workflow https://github.com/TechSavage1989/network-toolkit/actions/runs/41347071329 completed both build and publication successfully using merged source 977e9bf6a954a16be2d8f74e8f6bd5f61fccbb19.
- Four uploaded official assets: Network_Toolkit.exe, Network_Toolkit_1.18.4_Portable.zip, Network_Toolkit_1.18.4_Update.zip and SHA256SUMS. All have GitHub SHA-256 digests and the package verification succeeded. Executable name remains Network_Toolkit.exe.
- Phase 5 isolated hosted Chromium workflow https://github.com/TechSavage1989/network-toolkit/actions/runs/41346939400 succeeded; includes welcome/migration, no automatic network action, empty/error guidance, updater consent/timer regression tests, guarded version-resource validation, and screenshots at 1280×800 dark and 1920×1080 light.
- The comprehensive 15-job Checks run on the merge source was still in progress at publication verification. Earlier runs had an intermittent Editor pointer-resize alignment test and a remote file-browser double-click blocked by a sticky table header after resizing; the browser test now explicitly scrolls the row into view before exercising a real double-click. Do not claim full CI success without verifying the final Checks conclusion.
- Previous Phase 4 observations included a 16 MiB Editor timing outlier and a graph range test. Retain these regression observations until resolved with valid behavior/data evidence. Do not bypass security or data-integrity gates.
- Tests use hosted Windows and synthetic fixtures. Physical Windows desktop acceptance, live-device interruption testing and multi-hour soak were not performed.
- Previous published releases, original tags and binaries were not replaced. The beta updater channel is opt-in; stable latest remains v1.17.136.

## Next
- Phase 6: owner-approved consolidation of Test a target and File Transfer workspaces while keeping prior tool names findable, all connection/session data, protocol settings and security confirmations. Confirm remaining implementation constraints from docs/UX_POLISH_PLAN.md and docs/UX_AUDIT.md.
- Recheck the final main-branch Checks result and address failures as subsequent changes without modifying published v1.18.4.
- Keep What's New brief and user-focused; reserve technical details for Additional Details. Continue GitHub-hosted builds, never the owner's desktop.

## Safeguards
Do not use owner desktop, network or credentials. Preserve user state, unsaved work, active sessions, app data permissions, review confirmations, verified opt-in updater flow and stable Network_Toolkit.exe filename. All previously published releases and assets remain unchanged.
