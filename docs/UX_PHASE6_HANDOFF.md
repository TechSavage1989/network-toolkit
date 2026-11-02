# Network Toolkit UX Phase 6 - v1.18.5 Beta 5 (published)

Status: Published as opt-in prerelease https://github.com/TechSavage1989/network-toolkit/releases/tag/v1.18.5 at 2026-11-02T00:44:07Z. Merged PR #37; tag v1.18.5 points to merge commit 36ef57d14b078bf5c1f9b29b2073787cd3d38609. Guarded Release workflow https://github.com/TechSavage1989/network-toolkit/actions/runs/41368461612 completed build and publish successfully. Four assets: Network_Toolkit.exe, Network_Toolkit_1.18.5_Portable.zip, Network_Toolkit_1.18.5_Update.zip and SHA256SUMS; the SHA256SUMS entries match GitHub's asset digests. Stable latest remains v1.17.136.
Approval: the owner approved the UX audit's recommendations ("approved to proceed") and asked to continue the roadmap on 2026-11-01, including removing the Terminal sidebar's separate Sidebar button.

## What changed for users
- **Test a target.** Ping, Trace route, DNS lookup and TCP ports share one navigation entry. A tab strip at the top switches between them. The host you type is shared, and each tab shows a dot for its latest result for that host (green finished, yellow running, red failed, faint not yet run). Switching tabs never starts a test.
- **File Transfer.** SFTP, FTP/FTPS and TFTP share one navigation entry with a protocol picker. Each protocol's tooltip states its security (TFTP: no encryption or login). A running TFTP server still shows its activity dot on File Transfer.
- Both entries return to the test or protocol you used last, and that choice is saved. Code that opens a specific protocol (for example a device's "Open SFTP" action) still opens exactly that protocol.
- The former names (Ping, Trace route, DNS lookup, TCP ports, SFTP, FTP / FTPS, TFTP) are still found by command search (Ctrl+Shift+P) and open directly.
- **First-launch welcome** now stays on Overview, so it no longer shrinks File Transfer or other tools (this fix reached the Phase 5 branch after v1.18.4 was published, so it ships here).
- **Terminal sidebar.** The separate "Sidebar" show/hide button is removed at the owner's request. Clicking the selected tab hides the sidebar; clicking any tab shows it again. The divider still works.

## Preserved
Each test and protocol keeps its own engine, options, jobs, history, report identifiers, saved profiles, host-key trust and overwrite confirmations. Saved preferences migrate without loss: the old fileTransferProtocol value (sftp/ftp) is read as before, tftp is now accepted, and a missing targetTest defaults to Ping.

## Not done in this phase
- The audit described results "side by side". This phase delivers the target-first tabs with per-test status; a combined multi-result layout is a possible follow-up that would need owner review of a design.

## Validation
- New browser test scripts/ux-phase6.browser.cjs and workflow .github/workflows/ux-phase6.yml (screenshots at 1280x800 dark and 1920x1080 light). Synthetic browser tests only, not physical Windows or live-device acceptance.
- Updated existing tests that clicked the removed navigation entries or the removed Sidebar button.
- Local: all Node unit tests pass; Phase 3, Phase 5, Phase 6, UI consistency, navigation and transfer layout browser tests pass.

## CI evidence (2026-11-01, commit e8a1435)
- Phase 6 workflow: https://github.com/TechSavage1989/network-toolkit/actions/runs/41350832744 passed.
- Full Checks: https://github.com/TechSavage1989/network-toolkit/actions/runs/41350832709 passed on attempt 2. Attempt 1 failed only in `browser-workflows107` at `editor-layout-map` (Editor comparison marks). The same check also failed and then passed locally on identical code, so it is treated as the existing intermittent Editor check, not dismissed: watch it on later runs.
- The first Phase 6 Checks run found a real bug (explicit SFTP navigation could open a remembered TFTP choice); fixed in e8a1435.

## Release CI (2026-11-02)
- PR #37 Checks (pull_request https://github.com/TechSavage1989/network-toolkit/actions/runs/41366876843 and push https://github.com/TechSavage1989/network-toolkit/actions/runs/41366856615) and Phase 6 workflow https://github.com/TechSavage1989/network-toolkit/actions/runs/41366856618 passed on the exact release source.
