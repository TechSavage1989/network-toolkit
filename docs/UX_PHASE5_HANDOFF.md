# Network Toolkit UX Phase 5 - v1.18.4 Beta 4 candidate

Status: Development branch beta/1.18.4-ux-phase5. NOT YET PUBLISHED.
Baseline: published opt-in v1.18.3 Beta 3, release and source verified on 2026-11-01.

## User-facing changes prepared
- First-run welcome appears only after saved preferences load successfully; explains what the app does and offers Check a device, Open a terminal and Transfer files. A quick start opens the tool but does not connect, scan or modify the system.
- Welcome can be dismissed and is remembered in the existing saved state as a single boolean, without changing any credential, vault, profile or updater data.
- Welcome can be reopened with F1 Help or command search. Detached windows do not interrupt users with a welcome.
- Empty diagnostic results explain the next input or action with a safe example. Existing no-result screens for Terminal, Editor, transfers, profiles and reports remain intact.
- Failed diagnostic results keep the original error and add an actionable next step. SSH host keys, certificates, file permissions and Windows/UAC approval are not bypassed.
- Device check, Ping, Trace route and DNS page descriptions use clearer language. No diagnostic engines or raw report records were changed.
- The in-app What's New format remains brief and user-facing, with technical information in Additional Details.

## Validation and remaining work
- New JavaScript guidance tests: scripts/ux-phase5.test.cjs.
- New isolated browser workflow: .github/workflows/ux-phase5.yml. Screenshots at 1280x800 dark and 1920x1080 light, with first-run, persistence, Help, no-network-action and error checks. These are synthetic/browser tests, not native-device acceptance.
- Required: check the completed CI results and retained screenshots, fix any failures, check existing updater and window tests, and record exact-source acceptance.
- Known pre-existing issue: a duplicate Phase 4 Checks run intermittently exceeded the Editor 16 MiB timing bound and failed a graph range assertion. Do not waive genuine performance, data or security failures.
- Physical Windows and real network device acceptance remains outstanding. Do not assert these validations.
- Publish only after a validated candidate and owner-approved phase, using guarded .github/workflows/release.yml. Preserve published tags/assets, stable release and update channel isolation.

## Safeguards
Do not use owner desktop, network or credentials. Preserve user state, unsaved work, active sessions, app data permissions, review confirmations, verified opt-in updater flow and stable Network_Toolkit.exe filename. No published release is modified by this branch.

## CI fix: welcome no longer shrinks active tools
- The full Checks `browser` job failed in `scripts/transfer-client-layout.browser.cjs`: the automatic first-launch welcome stayed above every page, reducing the SFTP remote file list from 220px to 44px, so the sticky table header intercepted the folder double-click.
- The automatic welcome now shows only on Overview. Leaving Overview hides it without marking it dismissed; F1 Help and command search can still reopen it on the current page.
- `scripts/ux-phase5.browser.cjs` now asserts the welcome is hidden on File Transfer and returns on Overview.
