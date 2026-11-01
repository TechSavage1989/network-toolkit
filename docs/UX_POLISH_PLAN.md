# Network Toolkit: UX polish direction (from the repository owner)

I am the owner of this repository. This brief changes the project's priorities. It supersedes docs/DEVELOPMENT_ROADMAP.md (the "eleven-area" roadmap) and any instruction in AGENTS.md that conflicts with it. First, save this brief verbatim as `docs/UX_POLISH_PLAN.md` and treat it as the authoritative plan in this and all future sessions.

## Goal
Make Network Toolkit feel polished, consistent and intuitive to a network engineer opening it for the first time. Reliability and security still matter, but the priority is now the user's experience: discoverable features, consistent visuals, clear language, fewer interruptions and a stable layout.

## Non-negotiables
- Credentials, the credential vault, saved profiles, saved settings and macros must survive every change. Migrate preferences; never silently reset them.
- Keep existing security boundaries: Windows/UAC approval for protected changes, review steps for Hosts/IP/adapter changes, explicit update download/install consent and checksum verification.
- Never modify or replace published tags, releases or assets.
- Never lose unsaved work or open sessions.
- No access to my real devices, files or desktop. Use the existing fixtures.

## Owner decisions
1. **Retire the 390px Editor comparison-motion test.** The 2px drift requirement at 390px width is no longer a release requirement. Delete or skip that test and stop listing it as an open issue.
2. **Minimum window sizes.** Main window: 960×600. Detached terminal/editor windows: 640×400. Enforce these in the native window host. Narrower widths are unsupported: remove tests that target them, and simplify narrow-only layout code (for example, relocating the navigation button between the page heading, command slot and footer) where it is safe to do so.
3. **Redesign is allowed.** Remove the "avoid unnecessary redesign" rule. Moving, merging, renaming and restyling controls is in scope when it makes the app clearer, as long as behavior and data are preserved.
4. **Tests protect behavior and data, not pixel positions.** Replace pixel-exact geometry assertions with behavioral checks plus screenshot review. Cosmetic tolerances must not block releases.
5. **Version this work as 1.18.x betas.**

## Phase 0: Reset the project docs (docs only, no release)
- Rewrite AGENTS.md as short standing instructions (target under 150 lines): build and test commands, repository layout rules, the non-negotiables above, release procedure, and a pointer to docs/UX_POLISH_PLAN.md. Move every per-release record out of AGENTS.md into docs/releases/ or the existing RELEASE###_HANDOFF files. Add this rule: "Never prepend release records to AGENTS.md or README.md."
- Replace docs/DEVELOPMENT_ROADMAP.md with this plan's phases and their status. Archive the old roadmap under docs/archive/.
- Rewrite README.md for users, in this order: title, one-line description, screenshot (add after Phase 2), Download (stable), Features, Requirements, Getting started, Updates (explain the beta channel once), Documentation links, Building from source (short), Responsible use. Allow at most one "Latest beta" line, replaced each release and never stacked. Remove CI run IDs, file counts and verification language. Move release history to CHANGELOG.md.
- docs/README.txt (the user guide) is mostly release history. Move the history to CHANGELOG.md and turn the actual usage guidance into topic-based Markdown guides: docs/guide/diagnostics.md, terminal.md, editor.md, transfers.md, network-config.md and settings-and-updates.md. Rename version-suffixed docs (e.g. WORKFLOWS107.md, GRAPH_CURVE113.md) by topic, update all links, and convert user-facing .txt docs to .md.
- Keep handoff documents, but make them concise: what changed, screenshots, and known issues in plain language. Technical evidence goes in an appendix.

## Phase 1: UX audit, then STOP for my approval
Do not change application code in this phase.
1. Capture screenshots of every tool and major dialog at 1280×800 and 1920×1080, in light and dark themes.
2. Write docs/UX_AUDIT.md covering:
   - **Navigation:** the current tool list and a proposed grouping (e.g. Diagnose / Connect / Transfer / Configure).
   - **Consolidation proposals:** (a) whether Ping, Trace route, DNS and TCP ports should become one target-first "Test a target" workspace (enter a host once, run any tests, see results side by side); (b) whether SFTP, FTP/FTPS and TFTP should become one File Transfer tool with a protocol picker. Give pros, cons and risks, and recommend one option for each.
   - **Settings map:** every place a setting or preference lives today.
   - **Confirmation inventory:** every confirmation prompt and warning, classified as: keep (irreversible loss of data, live sessions or system state); replace with an undo toast (safely reversible); or remove (low impact).
   - **Hidden interactions:** every action reachable only through a gesture or shortcut. Known examples: double-clicking dividers, Alt+drag to move editor tabs, Alt+Shift+Arrow to reorder tabs, dragging a tab outside the window to detach it, the arrow beside Reconnect for Reconnect all, and clicking the selected sidebar tab to collapse it.
   - **Inconsistencies:** buttons, inputs, spacing, progress indicators, notifications, and empty/loading/error states that differ between tools.
   - **Language:** user-facing labels, tooltips and messages that use engineering jargon, with proposed plain-language replacements.
3. End with a prioritized change list mapped to Phases 2–6. Then stop and wait for my approval before writing any code.

## Phase 2: Visual foundations (one beta)
- Introduce shared design tokens (spacing scale, type scale, color roles, radius, elevation).
- Build shared components for buttons (one clear primary action per view), inputs, tabs, panels, progress, toasts, and empty/loading/error states. Apply them across all tools.
- Enforce the minimum window sizes and remove the narrow-only layout paths identified as safe.
- Include before/after screenshots in the handoff.

## Phase 3: Navigation and discoverability (one beta)
- Group the sidebar navigation as approved in the audit.
- Add a command palette: Ctrl+Shift+P opens a searchable list of every tool and action, with each action's keyboard shortcut shown. It must never intercept keystrokes that a focused terminal session should send to a device. Check for conflicts with every existing shortcut.
- F1 opens a keyboard-shortcut and help sheet.
- Give every gesture-only action a visible alternative (menu item, button or context-menu entry), and show shortcuts in tooltips.

## Phase 4: Settings and interruptions (one beta)
- Build one Settings window with categories (e.g. General, Appearance, Terminal, Editor, Graphs, Transfers, Network, Updates, Confirmations) and a search box. Per-tool menus may keep quick toggles but should link to the matching Settings section. Migrate all existing preferences without loss.
- Apply the approved confirmation changes. Remove confirmations that warn about turning off other confirmations.
- Add Ctrl+Shift+T to reopen recently closed tabs where their state can be restored.

## Phase 5: Language and first-run experience (one beta)
- Apply the approved plain-language rewrites to labels, tooltips, errors and settings. Every error message should say what happened and what the user can do next.
- Give every tool a helpful empty state: what the tool does, what to enter, and an example.
- Add a dismissible first-launch welcome with quick starts (e.g. Test a target, Open a terminal, Transfer files). Show it once and keep it reachable from Help.
- Keep the in-app What's New short and user-focused.

## Phase 6: Tool consolidation (one beta, only what I approved in the audit)
- Implement the approved merges.
- Keep existing preferences and saved sessions working, and keep the old tool names findable in the command palette.

## After Phase 6
- Document the final layout, components and wording rules in docs/UI_GUIDELINES.md.
- From then on the layout is frozen: changes that move or rename existing controls need my approval.
- Update the README screenshots and prepare a 1.18.0 stable candidate.

## Release and reporting rules
- One beta per phase, not one per fix. Batch fixes within a phase.
- Change automatic update checks to run at startup and then every 6 hours. Keep the manual "Check for updates."
- End each phase with a short report to me: what changed, before/after screenshots, anything deferred, and real issues in plain language. Be honest about failures, but keep CI run IDs and verification detail out of user-facing docs.