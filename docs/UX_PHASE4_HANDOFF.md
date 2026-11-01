# Phase 4: v1.18.3 Beta 3 - published

Published 2026-11-01T19:26:12Z as opt-in prerelease at https://github.com/TechSavage1989/network-toolkit/releases/tag/v1.18.3. Source and merged PR #34: bec58aa724e1ad6b33237eb6c6a4aff783bf0c85.

## Changes
- Searchable centralized Settings modal: General, Appearance, Terminal, Editor, Graphs, Transfers, Network, Updates and Confirmations. Existing preference controls are moved without cloning or changing persistence. Specialized controls remain in their respective tools with direct links from the matching Settings section.
- Memory-only Ctrl+Shift+T restores eligible clean local Editor files from their original path, or unused Terminal draft tabs. Limit of 10 entries; never captures a credential, console transcript, unsaved document or active remote session. Terminal console focus is respected.
- Low-risk deletion of one saved diagnostic target now has 10-second Undo restoring exact data and order if the ID remains free. Credential, host trust, file overwrite, network/UAC, live-session and updater confirmations remain.

## Tests and screenshot
Six hosted synthetic browser tests passed on https://github.com/TechSavage1989/network-toolkit/actions/runs/41335427629; includes Settings, theme, updater controls, category search, Undo, Editor and Terminal recovery and device-input protection. Phase 4 screenshot and test results are in the run artifact.
The exact-source push Checks run 41340055916 completed successfully with all 15 jobs. Official release workflow 41341463401 completed build and publish successfully. Four assets with GitHub SHA-256 digests are present: Network_Toolkit.exe, Network_Toolkit_1.18.3_Portable.zip, Network_Toolkit_1.18.3_Update.zip and SHA256SUMS. Stable latest remains v1.17.136. One duplicate PR Checks run 41340062114 reported a 5.254 second 16 MiB Editor input test against the 5-second threshold and a graph range assertion; neither reproduced in the all-green primary run. These intermittencies remain for investigation and must not be represented as universal CI success. No physical Windows or live-device acceptance is claimed.

## Next
Next UX Phase 5: plain-language labels and actionable errors, helpful first-run welcome and empty states, subject to owner approval. Investigate duplicate CI variability without relaxing data-integrity or security gates; retain previous stable release and all user data.
