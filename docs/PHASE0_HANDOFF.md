# Phase 0 - Documentation reset

**Result:** Completed as a documentation-only change. No application release.

## What changed
- Preserved the owner's UX directive verbatim in docs/UX_POLISH_PLAN.md as the authoritative plan.
- Replaced the eleven-area roadmap with the approved six UX phases; archived the old roadmap under docs/archive/.
- Replaced the long AGENTS.md release chronology with short standing instructions, and required release history to stay out of AGENTS.md and README.md.
- Rewrote the public README as a user overview; preserved historical release documentation in CHANGELOG.md.
- Added diagnostics, terminal, editor, transfers, network configuration, and settings/updates topic guides.

## Screenshots
None: Phase 0 is documentation-only. Phase 1 owns the before-state screenshots.

## Deferred
- docs/README.txt remains a short compatibility guide index because existing release validation still expects that exact file and version header. Its prior release history is preserved in CHANGELOG.md.
- Historic version-suffixed engineering references remain accessible to avoid breaking old links. Topic-named user references have been added for common workflows and graph controls; a comprehensive reference-link cleanup needs a separate audit.
- CI's older compact-root guard originally rejected the owner-required CHANGELOG.md; the Phase 1 audit branch adjusts that repository-layout test without modifying application code.

## Known issues
No new application behavior was introduced. The new UX plan retires the old 390px cosmetic drift threshold. Future changes must preserve credentials, vault data, profiles, settings, macros, unsaved documents and active sessions.

## Next
Complete Phase 1 visual and interaction audit and stop for owner approval before Phase 2.
