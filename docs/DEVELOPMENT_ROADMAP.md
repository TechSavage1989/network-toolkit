# Network Toolkit - Persistent Development Roadmap

This is the **authoritative, ongoing development plan**. Preserve it across beta releases and future chats. The eleven areas and three-phase sequence originated in the approved application-wide optimization plan. Do not replace them with an Editor-only scope or assume a published beta certifies all its planned work.

## Current publication and continuation - Beta 9

v1.17.145 published 2026-11-01T01:47:19Z as an opt-in prerelease from
84c3c53d5c2dcdee13fd04842fafa5c8693679d2; public assets and channels verified.
Shutdown/stale-response guards, retired diagnostic caches and finite synthetic
resource/lifecycle evidence: [handoff](RELEASE145_HANDOFF.md),
[audit](BETA9_RELIABILITY_AUDIT.md). Candidate CI: 14 successful groups and
one unchanged failed Editor motion group. Not all CI is green or stable-ready.

**Next proposed:** v1.17.146 Beta 10 (not started), further extended concurrent
operation/native interruption/repeated-window checks across all eleven areas,
plus diagnosis of remaining Editor motion/sidebar/alignment defects. Physical
Windows multi-hour/live-device and native SFTP acceptance remain open.
No beta publication implies completion of these phases or stable promotion.

## Prior publication and continuation record (v1.17.144)

Published v1.17.144 Beta 8 as an opt-in prerelease on
2026-10-31T22:56:54Z. Source/tag/PR #19:
866b64ef49cfd2ef40d7b4c8a5fa2aa24f1f3397.
Release: https://github.com/TechSavage1989/network-toolkit/releases/tag/v1.17.144
Release build/publish workflow 41250238391 completed successfully.
Detailed measured changes and deferrals: docs/BETA8_APPWIDE_AUDIT.md.
Published binary metadata and open validation gaps:
docs/RELEASE144_HANDOFF.md.

**Next proposed development:** v1.17.145 Beta 9, extended simultaneous
operation, memory/CPU and multi-tool stress/cancellation testing, followed
by further betas or release candidate only as evidence supports. Preserve
the original eleven-area roadmap, historical 390px Editor motion CI
failure and physical Windows/native SFTP acceptance gaps.

## Current baseline

- Repository: https://github.com/TechSavage1989/network-toolkit
- Latest published beta when created: v1.17.143 Beta 7. Published since: v1.17.144 Beta 8 on 2026-10-31.
- Latest stable when created: v1.17.136.
- Completed next phase: v1.17.144 Beta 8, published as opt-in prerelease, with measured shared-results/discovery/monitoring/report/UI improvements; full eleven-area and live-Windows certification remains incomplete.
- Current published phase: v1.17.145 Beta 9, scoped finite concurrent lifecycle reliability; full acceptance remains pending.
- Proposed next: v1.17.146 Beta 10, further concurrent/native reliability and unresolved Editor diagnosis.
- Release candidate and stable promotion are determined by validation, not a fixed release schedule. Additional betas are acceptable.
- In every new chat, **recheck** GitHub branch, release, CI, version metadata and newest handoff. Versions above are a snapshot, not permanent claims.

## Revised development roadmap: all eleven areas

| Area | Performance and polishing priorities |
| --- | --- |
| Application interface | Faster startup and navigation, smoother window resizing, consistent layouts, better use of screen space, responsive sidebars, and reduced unnecessary redraws |
| Network diagnostics | Improve Ping, Trace route, DNS, TCP port testing, and other diagnostic tools. Optimize execution, cancellation, progress reporting, and result presentation |
| Network discovery | Improve scanning efficiency, parallel processing, device information presentation, filtering, sorting, and large result lists |
| Graphs and monitoring | Smoother real-time graphs, efficient historical data rendering, responsive zooming and panning, accurate scaling, and lower CPU usage |
| Packet capture | Improve capture processing, filtering, large capture handling, packet inspection, and navigation between views |
| File transfers | Optimize SFTP, FTP/FTPS, and TFTP workflows, directory browsing, transfer progress, drag-and-drop behavior, and responsiveness during transfers |
| Text Editor | Improve large-file editing, syntax highlighting, comparison alignment, scrolling, window management, and file sidebar responsiveness |
| Terminal | Continue output, scrolling, resizing, and multi-session optimization without making Terminal the primary focus |
| Reports and results | Faster filtering, sorting, exporting, copying, and navigating large result sets |
| Network configuration | Polish IP configuration, adapter controls, Hosts editing, validation, and confirmation workflows |
| Settings and updater | Consistent controls, reliable preference persistence, clearer update workflows, and improved feedback |

All eleven remain in scope. A beta may implement targeted changes, but its handoff must state what was reviewed, improved, deferred, or not measured.

## Multi-phase strategy and actual progress

| Phase | Original version | Progress and unfinished work |
| --- | --- | --- |
| Phase 1: Application-wide performance | v1.17.140 Beta 4 | Published. Graph-history and file-list caching optimized, but broad startup, diagnostics, discovery, capture, reports and navigation benchmarks remain incomplete |
| Phase 2: Interface and workflow polishing | v1.17.141 Beta 5 | Published. Editor Files/Connections and transfer-browser divider improved; general cross-tool visual/progress/notification consistency still requires review |
| Phase 3: Reliability and final optimization | v1.17.142 Beta 6 | Published. Some concurrent UI work and graph cleanup improved; full simultaneous-tool, memory, cancellation and native Windows soak remain unfinished |
| Additional regression work | v1.17.143 Beta 7 | Published. Improved editor rendering; intermittent 390px comparison map motion drift remains recorded |
| Phase completion / broad optimization | v1.17.144 Beta 8 | Published opt-in prerelease; measured improvements across shared diagnostics/results, discovery, Ping/Monitor, report export and app chrome. Remaining areas are tracked, not marked certified |
| Extended reliability and stress | v1.17.145 Beta 9 | Published scoped opt-in beta. Finite six-tool lifecycle, 80-navigation, shutdown/retirement and resource observations passed; multi-hour physical Windows/live-device and broader native interruption acceptance remain pending |
| Continued reliability and regression diagnosis | v1.17.146 Beta 10 (proposed) | Not started. Continue all eleven areas, native interruption/repeated-window evidence and unresolved Editor diagnosis without relaxing acceptance |
| Release candidate | TBD | Pending. Complete native Windows and live-device acceptance and release-blocking regression fixes |
| Stable | TBD | Pending. Promote only after required acceptance; never just because a beta number was reached |

## Beta 8 contract

1. Start with AGENTS.md, this roadmap, docs/DEVELOPMENT_HANDOFF.md, the latest release handoff, and the current GitHub repo/release/CI source. Confirm version before editing.
2. Review all eleven areas. Include a per-area status in the handoff. Prioritize meaningful work across several areas, not just Text Editor or Terminal.
3. Make low-risk, measurable improvements to large result lists, discovery/diagnostics, live graphs/capture, file handling, reports and interface refreshes where inspection supports them.
4. Record before/after operation counts, comparator/extractor calls, DOM writes, CPU/time/memory numbers where possible. A call-count reduction is evidence for reduced work, **not** a measured wall-clock or CPU improvement.
5. Preserve list order, filtering, line history, selection, reports, exact source bytes, existing layouts, permissions, credentials, scan concurrency, user settings, and updater behavior.
6. Run focused regression tests and full JavaScript, Go, Python, security, browser/native Windows, packaging and update-validation gates against the exact candidate.
7. Preserve every failed CI assertion, including the original Editor 390px motion threshold. Do not silence, relax or relabel failures. Beta limitations must be disclosed; stable promotion requires actual acceptance.
8. Publish an explicitly authorized opt-in beta only from the intended source and tag. Verify EXE, portable ZIP, binary-only updater ZIP, checksums, beta feed, stable feed and immutable previous releases. Update this roadmap, AGENTS.md, development/release handoff and README.

## Beta 9 reliability and final acceptance matrix

- Application lifecycle: cold/warm start, repeated tool navigation, windows/sidebars, saved geometry and preferences, redraw work and long-running working-set growth.
- Diagnostics: Ping, Trace route, DNS and TCP running concurrently; progress, cancellation, restarting and partial results.
- Discovery: large lists and approved subnet scans, bounded parallelism, sorting/filtering, incomplete work and cancellation.
- Monitoring and graphs: long history, zoom/pan, follow latest, concurrent windows, CPU/memory and latency.
- Packet capture: sustained high-volume capture, filters, inspection, start/stop/restart, navigation and cleanup.
- Transfers: SSH/SFTP, FTP/FTPS and TFTP with large folders, progress, interrupted operations, cancellation/reconnect and permissions.
- Text Editor: large files, source/history integrity, multiple windows, syntax, comparison and continuous map motion.
- Terminal: sustained output from multiple SSH/Telnet/serial/local sessions, split panes, saved names, disconnect/reconnect and redaction.
- Reports: large result copy/filter/sort/export, correct formatting and memory.
- Network configuration: IP/adapter/Hosts validation, confirmations, failures, unchanged system state when rejected.
- Settings/updater: persistent preferences, stable Network_Toolkit.exe, beta opt-in, update confirmation, safe download/replace/restart.
- Cross-tool tests: simultaneous scanning, capture, graph monitoring, Terminal output and file transfer.

Document missing Windows/physical device access accurately. Synthetic tests and hosted CI are not substitutes for live-device certification.

## Original development standards

- **Measure before and after.** Startup times, memory, CPU, UI responsiveness and execution times where applicable.
- **Optimize real workflows.** Test multiple tools running concurrently, not only in isolation.
- **Improve consistency.** Shared controls, visual language, progress indicators and error handling.
- **Preserve functionality.** Working features, settings, credentials, security boundaries and portable updater behavior must remain intact.
- **Avoid unnecessary redesign.** Improve established layouts rather than replacing them solely for appearance.

## Open issues and truthfulness

- The hosted Editor comparison map has shown transient 3-5 CSS pixel mismatch in some 390px gestures, exceeding its unchanged 2px requirement. See docs/RELEASE143_HANDOFF.md and CI run 41246461674.
- Historical native SFTP cancellation issues must be independently checked; do not assume resolution.
- No independent multi-hour physical Windows soak, full memory-growth study, or simultaneous real-device networking benchmark has yet been established.
- A successful beta release build is not equivalent to a clean all-jobs integration run.

## New chat handoff rule

Always read **AGENTS.md**, **docs/DEVELOPMENT_ROADMAP.md**, **docs/DEVELOPMENT_HANDOFF.md**, and the latest **docs/RELEASE###_HANDOFF.md**. Verify live GitHub releases, main, PRs, CI and update channel before further changes. Continue all eleven areas using this roadmap, current test evidence and constraints. Update these documents after each beta. Do not discard past handoffs or rewrite history.

Copyable future-chat request:

> Continue Network Toolkit from the latest published release at https://github.com/TechSavage1989/network-toolkit. Read AGENTS.md, docs/DEVELOPMENT_ROADMAP.md, docs/DEVELOPMENT_HANDOFF.md, and the newest release handoff first. Preserve the eleven-area application-wide performance, interface and reliability roadmap, its outstanding CI evidence, portable Windows updater and security boundaries. Verify GitHub source/releases/CI, develop the next measured phase, validate and publish the authorized beta, and keep the roadmap and handoffs current.
