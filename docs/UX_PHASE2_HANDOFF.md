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
