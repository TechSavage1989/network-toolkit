# Network Toolkit UX Development Roadmap

**Authority:** [UX Polish Plan](UX_POLISH_PLAN.md). This roadmap replaces the previous eleven-area optimization roadmap. The [previous roadmap](archive/DEVELOPMENT_ROADMAP_PRE_UX_1.17.md) is preserved for historical context only.

## Current status
| Stage | Scope | Status |
| --- | --- | --- |
| Phase 0 | Documentation reset and authoritative UX plan | Complete, no release |
| Phase 1 | Comprehensive UI audit, screenshots and owner approval | Approved by repository owner; complete |
| Phase 2 | Shared visual foundations and native minimum window dimensions | Published v1.18.1 Beta 1; native Editor test follow-up outstanding |
| Phase 3 | Navigation, command palette, shortcuts and discoverability | Published v1.18.2 Beta 2 on 2026-11-01; physical Windows and live-device acceptance outstanding |
| Phase 4 | Unified settings, confirmations and reopen tabs | Approved for v1.18.3 Beta 3; candidate implementation and checks pending |
| Phase 5 | Language, empty states and first-run welcome | Awaiting approval |
| Phase 6 | Owner-approved tool consolidation | Awaiting approval |
| After Phase 6 | UI guidelines, layout freeze and 1.18.0 stable candidate | Not started |

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
After approval and acceptance, prepare 1.18.0 stable candidate. The exact beta numbering and published state must always be verified against the live repository.
