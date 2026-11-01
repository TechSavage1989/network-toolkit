# UX Phase 4 - Settings, confirmations and reopen tabs

**Candidate:** Network Toolkit 1.18.3 Beta 3, based on published v1.18.2 Beta 2. The owner approved this phase in the current conversation.

## Experience changes

- Searchable Settings categories: General, Appearance, Terminal, Editor, Graphs, Transfers, Network, Updates and Confirmations. Existing controls are rearranged into the respective categories without replacing underlying handlers, state keys, or stored preferences. Per-tool controls remain available.
- Ctrl+Shift+T and a visible action can reopen recently closed individual Text Editor documents, retaining in-memory unsaved edits. Empty Terminal drafts can be reopened without reconnecting. Never restore live sessions automatically, and never copy credentials into tab history.
- Saved diagnostic targets can be removed with a brief Undo action, with ID collision and list-capacity checks. The Editor close-confirmation preference is available in Settings and no longer produces a second prompt merely for changing that preference. Existing confirmations remain for live sessions, unsaved content, security, updates, Windows/network actions, and overwrites. No meta-confirmations about switching off confirmation options were introduced.

## Validation

- Add scripts/ux-phase4.test.cjs for bounded in-memory tab recovery and behavior guards.
- Add scripts/ux-phase4.browser.cjs and the hosted .github/workflows/ux-phase4.yml for actual UI interaction against synthetic APIs and screenshots in dark/light at 1280x800 and 1920x1080.
- Preserve existing Go, JavaScript, browser, native Windows and release guard checks. Report exact-source successes and failures after candidate CI runs.
- No network devices, passwords, real desktop, user files or live devices are used.

## Known limitations and deferred work

- Recovery is in-memory and limited to individually closed Editor documents and empty Terminal drafts. Full live-session restoration, multi-tab close recovery, process restart, and cross-window restoration are intentionally not performed to avoid silently reopening network connections or losing data.
- Settings retains advanced per-tool controls for functions that cannot safely be decoupled from their existing tool workflows. The unified window links into those tools without migrating or resetting their underlying state.
- Physical Windows acceptance, live-device connection testing, prolonged soak, and additional security review are still required before a stable release.
- Phase 5 covers plain-language errors, empty states and first-launch guidance. Phase 6 covers only approved tool consolidation. Maintain the 1.18.x beta sequence and do not replace previous tags or assets.

## Release record

Pending exact-source hosted CI, reviewed pull request, guarded release workflow and GitHub asset/updater verification. Complete this section only after publication.
