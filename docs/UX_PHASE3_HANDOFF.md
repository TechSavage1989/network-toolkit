# UX Phase 3 - Navigation and discoverability

**Published version:** Network Toolkit 1.18.2 Beta 2 on 2026-11-01. Based on published 1.18.1 Beta 1 with the previously approved navigation and UX audit.

## What changed
- Home, Diagnose, Connect, Transfer and Configure now group the existing 23 tools without removing their entrypoints.
- Search tools and actions with Ctrl+Shift+P or the visible Search button. The palette finds every existing navigation route, global actions and currently available tool controls.
- Press F1 for a keyboard shortcut and help sheet; when the interactive Terminal console is focused, keyboard input is reserved for the connected device.
- Visible action menus on Editor and Terminal tabs include left/right reorder and the existing move, detach and close operations.
- Terminal sidebar now exposes Show/Hide, and File Transfer pane splitter exposes Reset panes. No behavior or data is changed by these navigation affordances.

## Screenshots
**Before:** Phase 2 synthetic screenshots, https://github.com/TechSavage1989/network-toolkit/actions/runs/41280365529/artifacts/13073296255.
**After:** Phase 3 synthetic screenshots, https://github.com/TechSavage1989/network-toolkit/actions/runs/41283276219/artifacts/13074431909. This captured 176 images at both approved resolutions in light and dark, with no browser page errors or missing assets. The same run passed ten focused interaction checks. The exact-source release candidate passed hosted build and publication checks after version/resource updates.
No native Windows, real devices or personal files were used.

## Deferred
- Unified searchable Settings and confirmation changes are Phase 4.
- First-launch welcome, expanded plain-language empty states and errors are Phase 5.
- Approved target-first diagnostic and unified transfer tool consolidation are Phase 6.
- The future 1.18.0 stable candidate would sort older than 1.18.x numeric betas; resolve version ordering before stable publication. Do not retag prior releases.
- Security, native OS confirmation and extended live-device acceptance remain necessary before stable.

## Validation and safeguards
The 390px pixel drift test is retired under the owner plan, while Editor document, Terminal session, credential protection and system-permission tests remain enabled. The existing menu actions retain original review/confirmation logic. Successful Go/JavaScript/browser/native and release-package gates must be recorded after the exact candidate build.

## Publication verification
- Release: https://github.com/TechSavage1989/network-toolkit/releases/tag/v1.18.2 (opt-in prerelease, 2026-11-01T16:47:21Z).
- Tagged commit: 8ae89ad005ac112a2ab68dfffedca628efca1d65, merged from PR #31.
- Official release workflow: https://github.com/TechSavage1989/network-toolkit/actions/runs/41327840149. Build and publish jobs both succeeded.
- Published assets: Network_Toolkit.exe, Network_Toolkit_1.18.2_Portable.zip, Network_Toolkit_1.18.2_Update.zip and SHA256SUMS; all have GitHub SHA-256 digests.
- Exact-source PR Checks run https://github.com/TechSavage1989/network-toolkit/actions/runs/41326780380 completed with all 15 jobs successful. Main-branch Checks run https://github.com/TechSavage1989/network-toolkit/actions/runs/41327840133 was still running at handoff; do not claim completion without checking it.
- Stable release remains v1.17.136 and the public beta feed includes 1.18.2. Existing tag/release assets have not been revised.
- Native host acceptance on a physical Windows desktop and live network devices was not performed.

## Next
Phase 4: unified Settings window, approved confirmation changes, and Ctrl+Shift+T tab restoration after its state-preservation rules are verified.
