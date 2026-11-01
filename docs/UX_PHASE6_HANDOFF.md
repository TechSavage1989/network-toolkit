# Network Toolkit UX Phase 6 - tool consolidation (in development)

Status: Development branch beta/1.18.5-ux-phase6, built on the unmerged Phase 5 branch (PR #36). NOT YET PUBLISHED. The version still reads 1.18.4; bump to 1.18.5 Beta 5 only after v1.18.4 is merged and published.
Approval: the owner approved the UX audit's recommendations ("approved to proceed") and asked to continue the roadmap on 2026-11-01, including removing the Terminal sidebar's separate Sidebar button.

## What changed for users
- **Test a target.** Ping, Trace route, DNS lookup and TCP ports share one navigation entry. A tab strip at the top switches between them. The host you type is shared, and each tab shows a dot for its latest result for that host (green finished, yellow running, red failed, faint not yet run). Switching tabs never starts a test.
- **File Transfer.** SFTP, FTP/FTPS and TFTP share one navigation entry with a protocol picker. Each protocol's tooltip states its security (TFTP: no encryption or login). A running TFTP server still shows its activity dot on File Transfer.
- Both entries return to the test or protocol you used last, and that choice is saved.
- The former names (Ping, Trace route, DNS lookup, TCP ports, SFTP, FTP / FTPS, TFTP) are still found by command search (Ctrl+Shift+P) and open directly.
- **Terminal sidebar.** The separate "Sidebar" show/hide button is removed at the owner's request. Clicking the selected tab hides the sidebar; clicking any tab shows it again. The divider still works.

## Preserved
Each test and protocol keeps its own engine, options, jobs, history, report identifiers, saved profiles, host-key trust and overwrite confirmations. Saved preferences migrate without loss: the old fileTransferProtocol value (sftp/ftp) is read as before, tftp is now accepted, and a missing targetTest defaults to Ping.

## Not done in this phase
- The audit described results "side by side". This phase delivers the target-first tabs with per-test status; a combined multi-result layout is a possible follow-up that would need owner review of a design.
- Version bump, release notes and Windows version resources for 1.18.5 (after Phase 5 publishes).

## Validation
- New browser test scripts/ux-phase6.browser.cjs and workflow .github/workflows/ux-phase6.yml (screenshots at 1280x800 dark and 1920x1080 light). Synthetic browser tests only, not physical Windows or live-device acceptance.
- Updated existing tests that clicked the removed navigation entries or the removed Sidebar button.
- Local: all Node unit tests pass; Phase 3, Phase 5, Phase 6, UI consistency, navigation and transfer layout browser tests pass.
