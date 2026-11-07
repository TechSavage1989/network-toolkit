# Network Toolkit standing instructions

## Authority and scope
Read [docs/UX_POLISH_PLAN.md](docs/UX_POLISH_PLAN.md) first. It is the owner's authoritative UX plan; follow [docs/DEVELOPMENT_ROADMAP.md](docs/DEVELOPMENT_ROADMAP.md) for phase status. Historical plans in docs/archive/ are not active requirements. Read docs/DEVELOPMENT_HANDOFF.md for current progress.

**Never prepend release records to AGENTS.md or README.md.** Keep release history in CHANGELOG.md and per-release technical evidence in docs/releases/ or existing docs/RELEASE###_HANDOFF.md files.

## Working with the owner
- When the owner's input or decision is needed, when a release has been published, or when work is waiting on the owner, start that message with a bold, clearly marked line (for example **⚠️ ACTION NEEDED:**, **✅ PUBLISHED:** or **⏳ WAITING ON YOU:**) so it cannot be missed (owner request, 2026-11-07).
- Terminal settings modeled on MobaXterm are planned in [docs/TERMINAL_SETTINGS_PLAN.md](docs/TERMINAL_SETTINGS_PLAN.md).

## Owner's non-negotiables
- Never lose credentials, vault entries, saved profiles, preferences, macros, unsaved work or active sessions. Migrate settings explicitly.
- Preserve Windows/UAC authorization, Hosts/IP/adapter review, explicit download and install consent, and checksum verification.
- Never modify or overwrite previously published tags, releases or binary assets.
- Never access the owner's actual network devices, desktop or files. Use fixtures and hosted CI.
- Redesign, consolidate and rename controls when owner-approved; preserve actions and data.
- Native minimum dimensions in Phase 2: main 960×600, detached Editor/Terminal 640×400.
- The old 390px Editor comparison-motion drift test and its 2px threshold are retired. Narrower widths are unsupported; use behavior/data tests and screenshot review instead of pixel-specific cosmetic gates.
- No application-code changes in Phase 1. After the UX audit, **stop for explicit approval** before implementation.
- Work in 1.18.x betas, one per approved implementation phase (2 through 6).
- Update checks must eventually run at startup and every six hours, with manual checking and explicit user approval to download/install retained.
- Keep What's New short, plain-language and focused on the experience; technical detail belongs under Additional Details. The updater shows every What's New item in full (owner request, v1.21.5), so keep the list to a few clear sentences.

## Repository layout
- cmd/network-toolkit/: Go application and Windows native host; internal/: platform, network operations, security, Terminal and transfer logic.
- web/: UI sources; assets/ and themes/: resources; scripts/: tests, generators and release tooling.
- docs/guide/: user-facing topic guides; docs/releases/: archived release notes; docs/archive/: superseded plans; docs/RELEASE###_HANDOFF.md: historic engineering handoffs.
- Keep root documentation concise, with at most one latest-beta reference in README.
- CLAUDE.md sits in the root (allowlisted in scripts/repository_layout_test.py) because Claude reads it there; it only imports this file and docs/REPOSITORY_STANDARDS.md.
- README images live in docs/images/ (hero.png plus kebab-case names) and are regenerated with `node scripts/readme-screenshots.browser.cjs` (synthetic data only; see its header for the packet capture shot).
- Preserve the portable executable filename Network_Toolkit.exe and the existing update feed semantics.
- Dropdowns (v1.21.10): `web/assets/dropdowns.js` converts every `<input list="…">` into an app-styled combobox and removes its `list` attribute (kept as `data-combobox-list`), so never rely on `input.list`. Selects stay native while closed and switch to the app-colored list (`.ntk-picker`, base-select) only while open; do not style closed selects with `appearance: base-select`, because it changes their size and CI compares layouts with pinned older builds.
- Terminal settings (v1.21.11) has tabs: Appearance, Highlighting, Features, Mouse & keyboard. Put a new terminal option in the matching tab (`[data-terminal-settings-panel="…"]`, or mark Tools markup with `data-terminal-settings-target`); never add app-level buttons (theme, app settings) to it, and keep Tools for connections and help.
- Universal highlighting (v1.21.12): its rule switches and colors live on the Customize dialog's Universal page (`universalEditorHTML()`, `[data-terminal-universal-editor]` is in `SECTIONS` of terminal-appearance.js, so the same change/sync handlers serve it). `highlightOptions()` without a language returns the active rule set; pass `ruleset:null, ruleSet:null` to `legend()` when you need Universal's own colors.
- New saved UI settings must survive a restart: `mergeState` in web/index.html (or the module's `mergePreferences`) has to restore the key, and keys with a Go sanitizer (`transferClientLayout`, `transferServerSidebar` in transfer_client_settings.go) must allow the new field. The Text Editor sidebar preferences were silently dropped until v1.21.3 for this reason.

## Build and test
Use the repo's existing scripts and vendored dependencies:
- go test -mod=vendor ./...
- To check that the app compiles, use go build -mod=vendor -o /dev/null ./cmd/network-toolkit (or go vet). A bare go build ./cmd/... writes a 25 MB ./network-toolkit binary at the root; it is in .gitignore and must never be committed (it once slipped into a PR via git add -A). Browser tests run locally write screenshots and reports to ./test-results/ (also ignored); stage files by name instead of git add -A.
- node --test scripts/*.test.cjs
- python3 -m unittest discover -s scripts -p '*_test.py'
- python3 scripts/github_release.py validate
- python3 scripts/build_windows_resources.py
- sh ./build.sh

Where fixtures/browser/native host tests need platform-specific setup, run the established .github/workflows/checks.yml jobs in hosted GitHub Actions. Checks runs twice for a pull request from a branch in this repository (push and pull_request events); merge only when both runs are green, re-running a failed job rather than relying on the other run. Never claim physical Windows or live-device acceptance from synthetic tests.

## Linux edition
- Plan and phase status: docs/LINUX_PLAN.md. Windows file names, the Windows updater, saved data and behaviour must not change for Linux work.
- In Linux window mode the session token reaches only the window the app opened (one-time `/launch` link and `ntk_window` cookie, `browser_launch_gate.go`). Never serve the token in a page without that check, and never launch a browser with `--no-sandbox` or similar.
- Releases include `Network_Toolkit_<ver>_Linux_x86_64.tar.gz` and `_Linux_aarch64.tar.gz`; `build.sh` builds them and `github_release.py package` needs `--linux-x86-64` and `--linux-aarch64`.
- `build.sh` pins `GOTOOLCHAIN=local`; if the system Go is older than go.mod, put the matching toolchain first on PATH.

## Release procedure
1. Inspect the current main branch, release notes, handoff, tags, CI and updater channel.
2. Make one reviewed beta per approved UX phase, with relevant behavior/data regressions and screenshots.
3. Record status and known limitations in concise handoff; keep technical CI evidence in an appendix.
4. Run release guards and required CI, never silence genuine security/data failures.
5. Publish via the existing guarded .github/workflows/release.yml process only after the phase is approved.
6. Verify source/tag, the executable, portable and updater ZIPs, SHA256SUMS, update-feed selection and stable-channel isolation.
7. Never replace a published release or change saved state and authorization boundaries.

## Owner approval gate
Phase 0: docs only, no release. Phase 1: audit all tools at 1280×800 and 1920×1080 in light/dark, document consolidations/settings/confirmations/discoverability, and **stop**. Implement Phases 2 to 6 only after owner approval; after Phase 6 establish docs/UI_GUIDELINES.md and freeze layout pending owner approval for later moves.

## Repository standards (all contributors and AI tools)

This repository follows the shared TechSavage1989 standard in [`docs/REPOSITORY_STANDARDS.md`](docs/REPOSITORY_STANDARDS.md), the same file in every app repository. Read it before changing anything. In short:

- **README:** centered header (real icon, one `<h1>`, tagline, quick links Download · Get started · Features · Screenshots · Updates · Development, hero image), then the standard section order. Update it in the same pull request as any user-visible change.
- **Screenshots:** real captures with fictional data in `docs/images/` (`hero.png` + kebab-case names). Recapture any screenshot a UI change makes stale, in the same pull request.
- **Layout:** compact root; respect the root allowlist and checks above. Never commit build output, binaries, archives, logs, caches or secrets.
- **Branches:** short-lived `feat/` `fix/` `docs/` `chore/` `release/` branches, one focused change per pull request, merge only on green checks, delete the branch after merging.
- **Instructions:** this file is the single source of project rules for every tool; `CLAUDE.md` imports it. Add new rules and gotchas here.

Where this file's project-specific rules differ from the shared standard, this file wins; an enforced check script wins over both.
