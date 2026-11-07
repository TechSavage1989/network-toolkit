# Network Toolkit UX Development Roadmap

**Authority:** [UX Polish Plan](UX_POLISH_PLAN.md). This roadmap replaces the previous eleven-area optimization roadmap. The [previous roadmap](archive/DEVELOPMENT_ROADMAP_PRE_UX_1.17.md) is preserved for historical context only.

## Current status
| Stage | Scope | Status |
| --- | --- | --- |
| Phase 0 | Documentation reset and authoritative UX plan | Complete, no release |
| Phase 1 | Comprehensive UI audit, screenshots and owner approval | Approved by repository owner; complete |
| Phase 2 | Shared visual foundations and native minimum window dimensions | Published v1.18.1 Beta 1; native Editor test follow-up outstanding |
| Phase 3 | Navigation, command palette, shortcuts and discoverability | Published v1.18.2 Beta 2 on 2026-11-01; physical Windows and live-device acceptance outstanding |
| Phase 4 | Unified settings, confirmations and reopen tabs | Published v1.18.3 Beta 3 (2026-11-01); duplicate CI timing/graph intermittency remains under review |
| Phase 5 | Language, empty states and first-run welcome | Published v1.18.4 Beta 4 (2026-11-01); full UI regression follow-up and physical Windows/live-device acceptance outstanding |
| Phase 6 | Owner-approved tool consolidation: Test a target, File Transfer, Terminal sidebar button removed | Published v1.18.5 Beta 5 (2026-11-02); side-by-side results view and physical Windows/live-device acceptance outstanding; see [handoff](UX_PHASE6_HANDOFF.md) |
| After Phase 6 | UI guidelines, layout freeze and 1.18.0 stable candidate | [UI guidelines](UI_GUIDELINES.md) adopted, layout frozen (changes that move or rename controls need owner approval), README screenshots added. **Published v1.19.0 stable on 2026-11-02** (owner chose 1.19.0 because a 1.18.0 stable would sort below the published 1.18.5 beta) |
| Consolidation addendum | Owner-approved post-1.19.0 merges ([UX_AUDIT.md section 12](UX_AUDIT.md)): Device check, Connection monitor and Device details into Test a target; HTTP + TLS into Test a website; Hosts file into Local network; Ping/TCP sweeps into Network discovery. Wake on LAN kept | **Published v1.19.1 Beta 1 (opt-in) on 2026-11-02**; stable remains v1.19.0 |
| Reliability follow-up | Flaky CI checks made behavior-based; Text Editor compare pane-height bug; compact Monitor graph chrome in short windows | **Published v1.19.2 Beta 2 (opt-in) on 2026-11-02**; stable remains v1.19.0 |
| IP configuration load order | Owner report: slow IP configuration page; Network actions moved below Current configuration and loaded after the fast adapter read | **Published v1.19.3 Beta 3 (opt-in) on 2026-11-02**; stable remains v1.19.0 |
| Results polish | Owner-approved: leftover-field dump removed, Recorded source only for a chosen adapter, Activity log collapsed last; per-tool review sheet to follow | **Published v1.19.4 Beta 4 (opt-in) on 2026-11-02**; stable remains v1.19.0 |
| Results polish, part 2 | Owner reviewed the per-tool sheet: Activity log and technical blocks off results pages, toolbars only on long lists, cleaner sample tables, DNS query table folded, discovery notes to Help | **Published v1.19.5 Beta 5 (opt-in) on 2026-11-02**; stable remains v1.19.0 |
| Paced paste pager | Owner report: One line at a time paste stalled at --More-- on a Cisco switch; the engine now answers pager prompts per the session Auto pager setting | **Published v1.19.6 Beta 6 (opt-in) on 2026-11-03**; stable remains v1.19.0 |
| Last batch output tools; no automatic switch to Runs | Owner requests: Last command / Last batch (renamed from Last paste), Select output and Copy output work for macro runs and Run commands lists; starting a paced paste or macro never moves the Macros panel to the Runs tab | **Published v1.19.7 Beta 7 (opt-in) on 2026-11-03**; stable remains v1.19.0 |
| No gaps in batch output | Owner report: pager erase residue (blank line plus a row of spaces) in copied AOS-CX batches; whitespace-only lines after a pager prompt are dropped | **Published v1.19.8 Beta 8 (opt-in) on 2026-11-03**; stable remains v1.19.0 |
| Updates noticed while running | Owner request: automatic checks every 30 minutes while running and on return after 10 minutes, with a one-time toast; last batch-output gap | **Published v1.19.9 Beta 9 (opt-in) on 2026-11-03**; stable remains v1.19.0 |
| 1.20.0 stable | Owner request: stable release combining the 1.19.1 to 1.19.9 betas; owner confirmed the paced paste and batch output fixes on a live switch | **Published v1.20.0 stable on 2026-11-03**; stable channel latest moves from v1.19.0 to v1.20.0 |
| Update progress animation | Owner report: install-and-restart progress bar no longer animated; element border-radius removed from the shared progress rule, browser regression check added | **Published v1.20.1 Beta 1 (opt-in) on 2026-11-03**; stable remains v1.20.0 |
| 1.20.2 stable | Owner request: stable release of the 1.20.1 progress-animation fix, confirmed by the owner on Windows | **Published v1.20.2 stable on 2026-11-03**; stable channel latest moves from v1.20.0 to v1.20.2 |
| Text Editor Save As dialogs | Owner request: Windows Save As dialog for local files, SFTP folder picker for remote files, explicit overwrite consent | **Published v1.20.3 Beta 1 (opt-in) on 2026-11-03**; stable remains v1.20.2 |
| Update notice in full | Owner request: do not truncate the main update details in the updater | **Published v1.21.5 Beta 6 (opt-in) on 2026-11-07**; stable remains v1.20.16; owner check on Windows pending |
| Sidebar swap buttons at the bottom | Owner request: move the sidebar swap button to the bottom for all tools, like the Text Editor | **Published v1.21.4 Beta 5 (opt-in) on 2026-11-07**; stable remains v1.20.16; owner check on Windows pending |
| Sidebars on either side | Owner requests: Text Editor sidebar on both sides like Terminal; all resizable sidebars swap sides where able | **Published v1.21.3 Beta 4 (opt-in) on 2026-11-07**; stable remains v1.20.16; owner check on Windows pending |
| Packet capture adapter list | Owner request: the network connection picker should match the other adapter pickers in the app | **Published v1.21.2 Beta 3 (opt-in) on 2026-11-07**; stable remains v1.20.16; owner confirmed on Windows ("it looks good") |
| Packet capture for beginners | Owner request: make Packet capture usable for troubleshooting without packet-analysis knowledge; owner chose everything in one beta | **Published v1.21.1 Beta 2 (opt-in) on 2026-11-07**; stable remains v1.20.16; owner used it on Windows, only feedback was the adapter list (fixed in v1.21.2) |
| Linux edition L1 (foundation) | Owner request: develop the app for Linux with full parity (docs/LINUX_PLAN.md) | **Published v1.21.0 Beta 1 (opt-in) on 2026-11-06**; stable remains v1.20.16; owner check on a Linux PC pending |
| 1.20.16 stable | Owner request: stable release of the 1.20.13 to 1.20.15 betas | **Published v1.20.16 stable on 2026-11-05**; stable channel latest moves from v1.20.12 to v1.20.16 |
| Discovery shortcut on Monitor and Trace route | Owner request: "Scan a network in Network discovery" button on Monitor and Trace route as on Ping | **Published v1.20.15 Beta 3 (opt-in) on 2026-11-05**; stable remains v1.20.12; owner check on Windows pending |
| Editor right-click menu, Send to terminal on the toolbar | Owner requests: always-visible Send to terminal icon; Text Editor right-click menu with Send to terminal; Send to Text Editor in the Terminal right-click menu; matching arrow icons | **Published v1.20.14 Beta 2 (opt-in) on 2026-11-05**; stable remains v1.20.12; owner check on Windows pending |
| Tab row scrolling, send between Text Editor and Terminal | Owner requests: wheel and arrow-button scrolling of Terminal/Text Editor tab rows; send editor selection/document to a terminal session; send terminal output to a new or open document at a chosen position | **Published v1.20.13 Beta 1 (opt-in) on 2026-11-05**; stable remains v1.20.12; owner confirmed on Windows |
| 1.20.12 stable | Owner request: stable release of the 1.20.3 to 1.20.11 betas with no open issues | **Published v1.20.12 stable on 2026-11-04**; stable channel latest moves from v1.20.2 to v1.20.12 |
| Drag Overview sections, restart tab as administrator | Owner requests: draggable Overview sections kept across restarts; convert the current local shell tab to administrator | **Published v1.20.11 Beta 9 (opt-in) on 2026-11-04**; stable remains v1.20.2; owner confirmed on Windows |
| Test tab order and layout survive restarts | Owner report: tab positions reset on restart; owner default order Monitor, Ping, Trace route, DNS, Ports, Device check, Details | **Published v1.20.10 Beta 8 (opt-in) on 2026-11-04**; stable remains v1.20.2; owner confirmed on Windows |
| Customizable Overview, reorderable test tabs | Owner requests: movable Test a target / Test a website tabs; customizable dashboard | **Published v1.20.9 Beta 7 (opt-in) on 2026-11-04**; stable remains v1.20.2; owner confirmed test tab dragging on Windows (2026-11-04) |
| Optional master password, strength gauge, drop lines | Owner requests: vault with or without a master password, any master password with a strength gauge, username with saved credentials, drop line only when a move changes order | **Published v1.20.8 Beta 6 (opt-in) on 2026-11-04**; stable remains v1.20.2; owner confirmed on Windows |
| Terminal requests (drive letters, admin tab, Close all, Saved reorder) | Owner requests: drive letters on Starting directory drives; elevated copy of a running local shell; Close all in the Disconnect menu; drag-and-drop Saved order | **Published v1.20.7 Beta 5 (opt-in) on 2026-11-04**; stable remains v1.20.2; owner confirmed on Windows |
| Strip "+" only | Owner clarification: no command bar New tab button; the strip "+" is the one New tab control and the empty strip stays visible | **Published v1.20.6 Beta 4 (opt-in) on 2026-11-04**; stable remains v1.20.2 |
| Shared tab strip | Owner request: Terminal and Text Editor tabs share one design with a tab-style "+" button; `ux-tab-strip.css` styles both from the app tokens, Terminal strip gains the "+" | **Published v1.20.5 Beta 3 (opt-in) on 2026-11-04**; stable remains v1.20.2; owner reviewed (follow-up in v1.20.6) |
| Terminal typing latency | Owner report: sluggish typing; per-keystroke and per-output work trimmed (immediate coalesced send, row-signature highlighting, no hidden-panel syncs, observers ignore terminal DOM) | **Published v1.20.4 Beta 2 (opt-in) on 2026-11-03**; stable remains v1.20.2 |

## Rules
- Phase 0 is documentation only, with **no release**.
- Phase 1 changes no application code and **stops for owner approval**.
- One **1.18.x beta per approved implementation phase** (Phases 2 through 6), not one beta per individual fix.
- Preserve all credentials, vault data, saved connections, profiles, settings, macros, unsaved documents and active sessions.
- Maintain UAC review and security boundaries for privileged configuration, file modification confirmations and verified, consent-based updates.
- Do not touch previously published GitHub tags, releases or assets.
- Native minimum windows: main 960×600; detached terminal/editor 640×400. The old 390px 2px comparison drift check is retired; unsupported widths cannot block releases.
- Prefer behavioral and data integrity tests with screenshot review to pixel-specific cosmetic gates.
- Never use the owner's devices or desktop. Rely on synthetic fixtures and hosted CI.
- After each phase, report user-visible changes, screenshots, deferrals and actual problems in plain language.
- At Phase 1 completion, submit proposed consolidation and confirmation choices; do not implement Phase 2 before explicit approval.
- During implementation, change automatic update checks to startup plus every six hours, retaining manual checks and explicit download/install consent.

## Release sequence
Phase 2: 1.18.x Beta 1, visual foundations.
Phase 3: 1.18.x Beta 2, navigation and discoverability.
Phase 4: 1.18.x Beta 3, settings and interruptions.
Phase 5: 1.18.x Beta 4, language and first-run.
Phase 6: 1.18.x Beta 5, only approved consolidation.
After approval and acceptance, prepare the stable candidate, numbered 1.19.0 by owner decision so it supersedes every 1.18 beta. The exact beta numbering and published state must always be verified against the live repository.
