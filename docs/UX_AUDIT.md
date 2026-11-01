# Network Toolkit UX Audit - Phase 1

**Status:** UX audit review in progress, 132 screenshot captures complete and sampled. No application code has been changed and no 1.18.x release has been produced.

**Authority:** [Owner UX plan](UX_POLISH_PLAN.md), which supersedes the archived eleven-area roadmap. **Approval gate:** Do not start Phase 2 or alter application behavior without the repository owner's explicit approval of this audit.

## Method and evidence

- Current UI source: web/index.html and web/assets/; navigation registry in renderNavigation() and existing Playwright fixture patterns.
- Baseline: published v1.17.150. Screenshots requested for each tool at 1280×800 and 1920×1080, in light and dark themes, plus representative dialogs and detached Editor/Terminal windows.
- Screenshot collector: scripts/ux-audit-screenshots.browser.cjs, executed in hosted CI with synthetic localhost-style API responses only. It does not connect to devices, use real user credentials, access the owner's desktop or change the native application.
- **Evidence limitation:** Browser screenshots are synthetic rendering checks, not native Windows measurements. Capture completion, artifact inventory and visual inspection must be recorded before the audit can be called complete.
- Previous 390px Editor drift/pixel threshold is **retired by owner decision**, not an open release issue. No unsupported-width pixel test is an acceptance gate.

## 1. Navigation: existing tools

The current source renders these groups and labels:

| Existing group | Tools |
| --- | --- |
| Top level | Overview |
| Diagnostics | Device check; Ping; Trace route; DNS lookup; SMTP test; TCP ports; HTTP check; TLS certificate; Device details; Network discovery; Connection monitor; Packet capture |
| Tools | Local network; Text Editor; Hosts file; Wake on LAN; Network utilities |
| Workspace | Terminal; SFTP / FTP / FTPS; TFTP; Saved targets; Reports |

**Recommended grouping for owner approval:**

| Proposed group | Items | Reason |
| --- | --- | --- |
| Home | Overview; Saved targets; Reports | Entry points and previous work |
| Diagnose | Test a target (Ping / Trace route / DNS / TCP ports subject to approval); Device check; SMTP; HTTP; TLS; Device details; Discovery; Monitor; Packet capture | Troubleshooting organized by intent |
| Connect | Terminal; Wake on LAN | Start or manage device access |
| Transfer | File Transfer (SFTP / FTP / FTPS / TFTP subject to approval) | File operations in one place |
| Configure | Local network; Hosts file; Network utilities; Text Editor | Configuration, text and local tools |

The final placement of Text Editor, Wake on LAN, Saved targets and Reports needs owner review against the screenshots. Old tool names must stay searchable after navigation changes.

## 2. Consolidation proposals

### A. "Test a target" workspace

**Recommendation: approve a unified target-first workspace, without removing the individual test engines.**

- **Benefits:** Host/IP entered once; adjacent Ping, Trace route, DNS and TCP results; lower navigation overhead; easier comparison and repeat testing; same saved targets and report export.
- **Costs:** More complex layout, job concurrency and per-test option panels; more demanding progress and cancellation states.
- **Risks:** Accidental wider scanning, carrying an inappropriate target/port to another test, changed history semantics, losing per-tool advanced settings or report identifiers.
- **Safeguards:** Run each test only on explicit user action. Preserve individual options, job identifiers, cancellation, history, output and saved targets. Clearly label each result, target and port. Keep old tool names and shortcuts in command palette. Test separate and concurrent actions.

**Alternative:** Retain four separate pages with shared target history. This is lower risk, but leaves users switching pages to diagnose one host.

### B. Unified File Transfer tool

**Recommendation: approve one File Transfer workspace with an explicit protocol picker, preserving separate protocol engines and protocol-specific settings.**

- **Benefits:** One entry point; consistent local/remote panes, connection workflow, progress, retry, errors and history; protocol easier to discover.
- **Costs:** Extra protocol selector and more conditional options.
- **Risks:** Confusing encryption capabilities (SFTP is not FTPS), accidental insecure FTP/TFTP selection, data overwrite, session loss, SFTP borrowing an SSH session and different remote file semantics.
- **Safeguards:** Make protocol visible at all times; show its security characteristics; preserve user confirmation/overwrite rules, host-key trust and existing saved profiles. Do not silently change a selected protocol. Existing transfer modes and active sessions remain intact.

**Alternative:** Shared design components across existing SFTP and TFTP pages, with distinct navigation entries. Lower migration risk but duplicates concepts and settings.

## 3. Settings map

The source currently stores preferences across the global state and tool-specific modules. Locations to inspect and consolidate:

| Current surface | Settings or state |
| --- | --- |
| App settings modal | Theme, accent color, result time format and time zone, default report label, data folder, vendor data, update channel/checking and app termination |
| Main sidebar and global chrome | Navigation collapse, active page, target, layout preferences, light/dark switch |
| Diagnostics forms/advanced settings | Ping, Trace route, DNS, TCP, HTTP/TLS, SMTP and other per-tool options; persistent forms; target and source |
| Diagnostic tables and reports | Column selection/width, sort and filters, report labels/notes, time display, diagnostic status colors |
| Discovery and details | Discovery workflow settings, column visibility, details docking/width, recent probes |
| Graphs and monitor | Graph scale/zoom/curve and follow behavior, saved graph settings and colors |
| Capture | Capture interfaces, details docking, filters and capture operations |
| Terminal settings/child-window settings | Appearance, font size/highlighting, active action placement, sidebar position/width/collapse, pane layouts and composer |
| Terminal confirmation options | Delete session, close, Reconnect all, Disconnect all, Close all, macro execution |
| Saved connections and vault | Terminal/transfer profiles, recent lists, trusted SSH host keys, credential vault and password manager |
| Editor | Word wrap, syntax, font/theme, comparison preferences, tabs, saved connections and window state |
| Transfer tools | Protocol, mode, explorer layout, recent hosts and stored profiles |
| Network/local tools | Selected adapter and configuration tabs, review/approval workflow, utility selections |
| Updates | Manual checking, beta opt-in, download/install review and updater state |

**Phase 4 proposal:** a searchable Settings window with General, Appearance, Terminal, Editor, Graphs, Transfers, Network, Updates and Confirmations. Quick per-tool toggles remain, each with a link into its corresponding Settings section. No migration may silently reset existing preferences.

## 4. Confirmation and warning inventory

**Keep** when closing sessions or losing data or changing system/security state:

- Close unsaved Editor documents; close/disconnect Terminal or multiple active sessions; quit while work is in progress.
- Overwrite/replace remote or local files; delete credentials or vault contents; remove trusted host keys; delete or move stored app data.
- Change Windows adapters, IP settings or Hosts file; preserve review and UAC gates.
- Download/install an application update, with checksum verification and explicit authorization.
- Execute potentially destructive or multi-target macros, and confirm action scope.

**Candidate for undo toast**, only after behavior is proven reversible and state can be restored:

- Remove one saved target or a nonsecret saved list entry, when the full entry can be reinstated without overwriting concurrent edits.
- Dismiss a temporary result panel or hide a sidebar, with visible Undo or restore action.
- Clear locally retained results only if an immediate, memory-bounded restore is actually implemented; otherwise keep confirmation.

**Candidate to remove** (low-impact, no data/session loss):

- Optional informational warnings for opening a view, toggling a purely visual panel, selecting a tab or changing a cosmetic preference.
- Any prompt that only warns about disabling future confirmations. This is specifically prohibited by the owner direction.

**Not yet fully enumerated:** Confirmations generated by separate Terminal, transfer, native Windows and credential-vault modules require line-by-line inspection during the audit. A "remove" classification is not permission to drop any security, overwrite, session or UAC review.

## 5. Hidden interactions and discoverability

| Current interaction | Required visible alternative, Phase 3 |
| --- | --- |
| Double-click a divider to collapse/reset | Splitter context menu and explicit Collapse/Reset button |
| Alt+drag an Editor tab | Move tab menu / drag affordance |
| Alt+Shift+Arrow to reorder tabs | "Move left" / "Move right" context-menu commands and tooltip |
| Drag a tab out of a window to detach | "Move to new window" menu option |
| Arrow beside Reconnect for Reconnect all | Explicit labeled Reconnect all menu item |
| Click selected sidebar tab to collapse | Visible Hide sidebar control |
| Keyboard resizing of splitters | Context menu with Reset and resize help |
| Hover-only tool actions or icon-only buttons | Visible labels or consistently discoverable overflow menu |
| Shortcut-only commands | F1 help sheet and command palette entries |

All existing keyboard shortcuts require conflict checking, especially Ctrl+Shift+P, Ctrl+Shift+T and F1, before implementation. **Terminal focus must retain device keystrokes**: the command palette must not intercept text or control sequences intended for the remote terminal.

## 6. UI inconsistencies to review in captured screenshots

- Separate Diagnostics, Workspace and Tools navigation currently mixes operations, content and workflow concepts.
- Multiple tool-specific panels use different forms, progress and command placement. Establish one primary action per view and consistent placement.
- Modal-only App settings, per-tool settings and child Terminal settings need consistent categories and navigation.
- Persistent sidebar controls, tab actions, hover and drag-only behaviors need visible alternatives.
- Empty/first-run views vary in how clearly they explain required inputs, examples, expected results and required permissions.
- Loading and error indications need standard structure: what happened, how to retry and how to preserve existing work.
- Inspect 1280×800 and 1920×1080 in both themes for contrast, typography, alignment, status signals, scroll/overflow and split-pane behavior. Record specific visual findings with screenshot filenames rather than speculative pixel claims.

## 7. Language inventory and proposed replacements

| Current label | Proposed user-facing wording |
| --- | --- |
| TCP ports | Test a port (or "Ports" within Test a target) |
| Device check | Check a device |
| TLS certificate | Check website certificate |
| Connection monitor | Monitor connection |
| Network discovery | Find devices |
| Local network | This computer's network |
| Trace route | Trace route (keep recognizable term, describe as "See network hops") |
| SFTP / FTP / FTPS | File Transfer, with visible protocol picker |
| Advanced options | More options |
| Unresponsive SFTP subsystem | The remote file server did not respond. Try again or check the connection. |
| SFTP response deadline exceeded | File browsing timed out. Your terminal connection remains open; retry browsing. |
| A packet was rate limited | The device may be limiting responses. Try again later. |

For every error rewrite: state **what happened** and **what the user can do next** without hiding relevant protocol/permission detail. Avoid changing raw diagnostic data, logs or exported protocol facts.

## 8. Prioritized phase mapping

| Priority | Phase | Proposal |
| --- | --- | --- |
| 1 | 2 | Shared spacing/type/color tokens, buttons/inputs/tabs/toasts/empty states, minimum native windows, before/after screenshots |
| 2 | 3 | Navigation groups, Ctrl+Shift+P command palette with terminal focus safeguards, F1 shortcut sheet, visible equivalents for hidden actions |
| 3 | 4 | Searchable unified Settings, migration of every existing preference, approved confirmation classification, Ctrl+Shift+T restoration |
| 4 | 5 | Plain-language messages, helpful empty states and dismissible first-run welcome |
| 5 | 6 | Only owner-approved Test a target and File Transfer consolidation, with compatibility mapping |
| Final | After 6 | UI guidelines, frozen layout after approval, screenshots and 1.18.0 stable candidate |

## 9. Verified screenshot evidence and observed UI issues

The hosted synthetic-browser capture passed with **132 screenshots** and no JavaScript page errors or missing assets. It includes every navigation tool at both 1280×800 and 1920×1080, in light and dark themes, detached Editor/Terminal views, App settings, target saving, four diagnostics option dialogs and the protected app-data deletion confirmation.

[132-image audit artifact](https://github.com/TechSavage1989/network-toolkit/actions/runs/41279077691/artifacts/13071729961) (GitHub Actions attachment, retention-limited). Representative filenames below match the downloadable archive.

| Evidence screenshot | Observation | Recommended phase |
| --- | --- | --- |
| main-1280x800-dark-ping.png | The sidebar tool list extends below the fold; common workspace tools require sidebar scrolling. Primary Start and target entry are in different areas. | 3 |
| main-1280x800-dark-terminal.png | Before connecting, the Terminal presents several rows of controls above a good central "Start a terminal" empty state, including disabled session-only actions. Hide or defer irrelevant controls on first use. | 2, 5 |
| main-1280x800-dark-sftp.png | File-server setup, security credentials, shared file actions and activity logs compete for space. Dense controls and narrow left-hand settings need a clearer task sequence. | 2, 6 |
| dialog-1280x800-light-app-settings.png | App settings is one vertically scrolling modal mixing appearance, time format, report label, storage, vendor data, updates and app termination. No categories or search. | 4 |
| dialog-1280x800-light-ping-options.png | A generic advanced-settings dialog has little actionable content for Ping, while other diagnostics show numeric limits; inconsistent information density and extra interruption. | 2, 4 |
| dialog-1280x800-dark-data-deletion.png | App-data deletion has strong warnings, typed confirmation and a disabled destructive button until explicit input. This safety confirmation must remain. | 4, KEEP |
| main-1920x1080-light-overview.png | Large screens give useful room to cards and results, but information and action placement vary across tools; compact controls and low-emphasis hints merit consistency review. | 2, 5 |

Fixture-only limitations: the simulated host deliberately has no real adapters, sessions, captures or connected devices. Messages such as "No adapter information available", "Unavailable" and "No diagnostics yet" here are **expected fixture states**, not evidence of real-device failures. Screenshots do not demonstrate performance, Windows/UAC behavior or live connections.

The owner-approved minimum widths replace narrow pixel-specific tests. The historical 390px common-row-motion CI step has been removed from the audit branch; source integrity, editing behavior and other regression checks remain active.

## Open audit work before asking for approval

- Capture additional transfer-mode, report, terminal and session-confirmation views; the existing 132 screenshots already cover every navigation page and representative major dialogs.
- Finish comprehensive settings, confirmation and hidden-action inventories across separate JS/Go files rather than treating the lists above as exhaustive.
- Review what appears in both detached editor and terminal windows.
- Confirm startup/update timing behavior from existing code (the new six-hour schedule is for a later implementation phase).
- Record fixture/browser limitations and any uncovered controls accurately.

**Approval status: not requested.** No Phase 2 code may be written until the audit is complete and the owner approves the proposed changes.
