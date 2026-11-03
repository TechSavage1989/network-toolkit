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
