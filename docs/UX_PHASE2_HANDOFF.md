# UX Phase 2 Beta 1 - published

**Release:** [Network Toolkit v1.18.1 Beta 1](https://github.com/TechSavage1989/network-toolkit/releases/tag/v1.18.1), published 2026-11-01T06:33:31Z.
**Source/tag:** a9ac3ba65591a67008dd4008912759b7d11880fb, from merged PR #29. Official release build and publication succeeded. Four expected assets were uploaded with GitHub SHA-256 metadata.
Stable latest remains v1.17.136, and Beta 1 is opt-in. Existing Network_Toolkit.exe filename and explicit user approval for download and installation are preserved.

## Outcome
Shared visual style roles and common control presentation, 960×600 main and 640×400 detached native minimum window sizes, and automatic update checks at startup and then six-hour intervals. Manual checking remains available. Existing saved data and permission/security workflows have not intentionally changed.

## Before and after
Before: https://github.com/TechSavage1989/network-toolkit/actions/runs/41279311855/artifacts/13072242569
After: https://github.com/TechSavage1989/network-toolkit/actions/runs/41280365529/artifacts/13073296255
Both capture sets contain 176 synthetic UI screenshots. These are not physical Windows or live-network certifications.

## Validation status and real issues
Release Go, JavaScript, Windows cross-compiled executable, packaging and publication gates passed. Some longer native/browser Checks jobs were still in progress at release verification; do not claim an entirely green matrix.
A hosted Windows native Editor test failed its old assertion that maximizing must increase viewport width by more than 100 pixels. On the hosted runner, the maximized window remained at 1024 pixels wide, while the editor filled the available client. A separate test-only PR #30 replaces the pixel-size assumption with proportional layout/content assertions while preserving Editor document and session tests. Until that PR passes, this native-test failure remains open as test/environment ambiguity, not a certified fix.
No real-device or owner's desktop testing was performed. The older 390px pixel drift threshold was retired by owner decision; no security or data tests were retired.

## Follow-up and compatibility
Phase 3 navigation/discoverability is next. Phase 4 settings/consolidated confirmations and Phase 6 approved mergers remain pending. Do not modify the published v1.18.1 tag, notes or assets.
Versioning caveat: the current toolchain uses numeric three-part versions. A future planned v1.18.0 stable release would sort below v1.18.1 Beta 1. Establish a non-downgrading stable candidate plan before stable promotion.

## Candidate evidence (historical)
# Phase 2 UX foundations: 1.18.1 Beta 1 candidate

**Status:** Development candidate, not published. Based on v1.17.150, with the Phase 1 UX audit and owner approval already merged.

## Visible changes
- Shared interface tokens and consistent styles for buttons, inputs, tabs, panels, progress, alerts, toasts and empty states.
- More consistent focus outlines and reduced-motion handling.
- Native minimum sizes: main 960×600, detached Terminal and Editor 640×400. Saved positions remain subject to existing monitor-fit behavior.
- Automatic checks at startup and every six hours with existing manual Check for updates, rate limits, user-approved downloads/installation and checksum checks preserved.

## Screenshots
- Before: https://github.com/TechSavage1989/network-toolkit/actions/runs/41279311855/artifacts/13072242569
- After: https://github.com/TechSavage1989/network-toolkit/actions/runs/41280365529/artifacts/13073296255
- Each archive contains 176 hosted synthetic UI captures across 1280×800 and 1920×1080 and both themes. Browser screenshots cannot certify native Windows minimum sizing, device connections or OS/UAC behavior.

## Deferred
- Navigation changes, command palette and discoverability to Phase 3.
- Unified Settings, confirmation changes and reopen tabs to Phase 4.
- Language, first-run and empty-state content improvements to Phase 5.
- Approved target-first diagnostics and File Transfer consolidation to Phase 6.
- No other layout moves or tool merges in this phase.

## Known limitations
- Prior CSS dependency was rejected by the explicit asset allowlist; that was corrected without exposing other files.
- The deprecated 390px pixel drift check is no longer a release gate. Editing functionality and data-preservation checks remain enabled.
- Exact candidate CI, Windows resource generation, final screenshots and publication verification must be recorded after the build.
- This project's release tooling currently supports numeric three-part versions. Beta 1 uses 1.18.1; a later 1.18.0 stable candidate would compare as older and requires a version strategy before promotion. Do not overwrite any prior published tag.

## Security and data
No planned changes to credential storage, saved settings, profiles, macros, active sessions, protected system operations or updater approval. No access to owner devices or desktop.
