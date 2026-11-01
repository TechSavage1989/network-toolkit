# Phase 1 - UX audit handoff

**Status:** Audit ready for owner review; no application-code changes, no beta release, and Phase 2 remains blocked pending explicit approval.

## What changed
- Captured and reviewed the existing interface using hosted Chromium and synthetic API fixtures at 1280×800 and 1920×1080, light and dark, covering all navigation tools, client/server transfer modes, detached Editor/Terminal views and representative major dialogs.
- Documented current and proposed navigation, two consolidation options, settings locations, confirmation classifications, hidden gestures, inconsistent controls and terminology, with a prioritized Phase 2–6 plan: [UX audit](UX_AUDIT.md).
- Retired the old 390px cosmetic motion CI gate under the owner's decision, retaining source-integrity, editing and behavioral checks.
- Updated the compact-root repository test to allow the owner-required CHANGELOG.md; unrelated root files are still rejected.

## Screenshots
[Download the 176 hosted fixture screenshots](https://github.com/TechSavage1989/network-toolkit/actions/runs/41279311855/artifacts/13072242569). The workflow reported 176 images and no page errors or missing assets.

Representative views:
- main-1280x800-dark-ping.png: navigation visibility, primary action and target placement.
- main-1280x800-dark-terminal.png: excessive session controls before connection.
- main-1280x800-dark-sftp.png: dense server setup and file operations.
- dialog-1280x800-light-app-settings.png: long uncategorized settings modal.
- dialog-1280x800-dark-data-deletion.png: destructive approval that must remain.

Screenshots are synthetic UI evidence, not physical Windows/UAC or real-device proof. The GitHub artifact has limited retention; retain the downloaded archive for later comparisons.

## What is deferred
- No navigation, visual, settings, wording, transfer, diagnostic or updater behavior was changed.
- Native minimum window enforcement, the first 1.18.x beta and new six-hour automatic update schedule are implementation work after approval.
- The old user-guide compatibility path remains because current release tooling validates it.
- More exact item-by-item mapping will be required when migrating preferences and confirmations; do not infer that grouped settings entries are safe to drop.

## Real issues
- Left navigation hides lower groups below the fold at 1280×800.
- Terminal shows multiple controls before a session exists.
- Transfer workflows have dense and inconsistent panel structure.
- Settings are spread across different views and lack one searchable window.
- Some advanced dialogs add an interruption but little useful content.
- The prior 390px pixel drift requirement is retired, not a release defect.

## Approval requested before Phase 2
1. Proposed sidebar grouping: Home, Diagnose, Connect, Transfer and Configure.
2. Consolidate Ping/Trace route/DNS/TCP into "Test a target" while preserving the individual engines.
3. Consolidate SFTP/FTP/FTPS/TFTP under a File Transfer tool with an explicit protocol selector.
4. Apply the proposed Keep / Undo toast / Remove confirmation classifications, subject to security and data guards.
5. Implement Phases 2–6 as one beta per phase, starting with visual foundations and 960×600 / 640×400 native minimum dimensions.

Do not start Phase 2 until the owner explicitly approves the proposed changes.
