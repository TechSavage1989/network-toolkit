# Network Toolkit UI Guidelines

**Status:** Adopted with the v1.19.0 stable release (2026-11-02) and updated for the owner-approved v1.19.1 consolidation. It describes the layout, components and wording as they ship in v1.19.1. The layout is frozen (see [Layout freeze](#layout-freeze)).

**Authority:** [UX Polish Plan](UX_POLISH_PLAN.md). Sources of truth in code: `web/assets/ux-foundations.css` (tokens), the `:root` and `[data-theme="light"]` blocks in `web/index.html` (colors), `renderNavigation()` in `web/index.html` (navigation), and `web/assets/ux-command-palette.js` (shortcuts).

## Layout freeze

Now that this document is adopted:
- Moving, renaming, merging, removing or regrouping an existing control, navigation entry, tab or panel needs the owner's explicit approval first.
- Allowed without approval: bug fixes that restore the documented layout, wording fixes that follow the [wording rules](#wording), new options inside an existing panel, and accessibility fixes that do not move controls.
- Every approved layout change updates this document in the same pull request.

## Windows and sizes

- Minimum native sizes: main window 960×600; detached Terminal and Editor windows 640×400. Narrower widths are unsupported.
- Screenshots for review are taken at 1280×800 and 1920×1080, in light and dark themes.
- Layout tests check behavior and data (what is visible, reachable and preserved), not pixel positions. Cosmetic tolerances must not block a release.

## Navigation

The left sidebar has five groups. Order is fixed.

| Group | Entries |
| --- | --- |
| Home | Overview · Saved targets · Reports |
| Diagnose | **Test a target** (Device check, Ping, Trace route, DNS lookup, TCP ports, Connection monitor, Device details) · **Test a website** (HTTP check, TLS certificate) · SMTP test · Network discovery · Packet capture |
| Connect | Terminal · Wake on LAN |
| Transfer | **File Transfer** (SFTP, FTP / FTPS, TFTP) |
| Configure | **Local network** (IP configuration, Routing table, ARP / neighbors, Connections, Wi-Fi, Hosts file) · Network utilities · Text Editor |

Rules:
- A merged entry (Test a target, Test a website, File Transfer) reopens the member used last; Local network opens its last view. Code that opens a specific member (for example a device's Open SFTP) opens exactly that member.
- Every former or member tool name stays findable in command search (Ctrl+Shift+P).
- Activity dots on a merged entry combine all its members (for example, a running TFTP server marks File Transfer).
- Ping and TCP ports check one target. Scanning a network or address range always happens in Network discovery; Ping and TCP ports link to it.
- Test a website uses one address for both tabs: HTTP check adds `https://` to a bare host, and TLS certificate uses the host from a URL.
- The sidebar can be collapsed to an icon rail; single clicks navigate, the arrow, divider or a double-click expands it.

## Page anatomy

Top to bottom, a tool page has:
1. **Notices:** offline/disconnected banner, then the first-launch welcome (Overview only, until dismissed).
2. **Page actions row:** member tabs for merged entries (test tabs or protocol picker) first, then tool-level buttons such as Status colors and Reset layout.
3. **Command bar:** the one primary action (for example Look up DNS), the current target, and the Configuration/Results switch.
4. **Workspace:** Configuration (target, source interface, options; advanced options behind Expand options) and Results.

Member tabs:
- Test a target and Test a website tabs show a status dot for the current target: green finished, yellow running, red failed, faint not yet run. Switching tabs never starts anything.
- Tab labels may be short (DNS, Ports, Monitor, Details); the tooltip and command search use the full tool name.
- Test tabs stay on one row. When space runs out they scroll sideways, and they stay visible in short windows where the rest of the page heading is hidden.
- File Transfer protocol buttons carry a tooltip with the protocol's security (SFTP encrypted over SSH; FTPS encrypted, FTP not; TFTP has no encryption or login).

Terminal: a session sidebar (Saved, Recent, Files, Macros) docks left or right. Clicking the selected tab hides it; clicking any tab shows it. There is no separate Sidebar button.

## Components and tokens

Use the shared tokens in `ux-foundations.css`; do not hard-code new sizes or colors.

| Role | Token / rule |
| --- | --- |
| Spacing | `--ux-space-1`…`--ux-space-6` = 4, 8, 12, 16, 20, 24 px |
| Text | `--ux-text-xs` 11 · `sm` 12 · `base` 13 · `md` 15 · `lg` 19 px; UI font `--sans`, code/addresses `--mono` |
| Radius | `--ux-radius-sm` 6 (buttons, inputs, tabs) · `md` 9 (notices, toasts) · `lg` 12 (panels, dialogs) |
| Control height | 36 px (`--ux-control-height`), small 30 px |
| Focus | 2 px outline in `--accent`, visible on every interactive element |
| Disabled | `opacity:.65`, `cursor:not-allowed` |
| Motion | short color transitions only; none under `prefers-reduced-motion` |

Buttons:
- One **primary** button per view, for the main action. Other actions use the default or `ghost` style; `small` inside toolbars and tab rows.
- Icon-only buttons need an `aria-label` and a tooltip; show the shortcut in the tooltip when one exists.

States every tool provides:
- **Empty:** what the tool does, what to enter, and an example (see `web/assets/ux-phase5.js`).
- **Loading/progress:** the shared `progress` style or a running status; never a blank area.
- **Error:** the original error text plus a next step. Never suggest bypassing a certificate, host-key or permission warning.

## Color roles

Dark is the default theme; light is equivalent. Use roles, not hex values.

| Role | Use | Dark | Light |
| --- | --- | --- | --- |
| `--accent` | primary actions, focus, selection | #6eabff | #1765c5 |
| `--good` | finished / healthy | #5dd6a0 | #147c52 |
| `--warn` | running / attention | #f4c56b | #936000 |
| `--bad` | failed / destructive | #ff8790 | #c33244 |
| `--text` / `--muted` / `--faint` | primary / secondary / tertiary text | | |
| `--panel` / `--panel2` / `--border` | surfaces and outlines | | |

Never rely on color alone: status dots also have text (tooltip and screen-reader label).

## Confirmations

Keep a confirmation only when an action can lose data, interrupt live sessions or change system or security state. Owner-approved policy (from [UX_AUDIT.md](UX_AUDIT.md) section 10):

- **Keep:** closing unsaved Editor work; closing or disconnecting live Terminal sessions (including Reconnect all and Disconnect all); macros sent to multiple targets; reviewed terminal paste; file overwrite; starting FTP/TFTP servers (plain-text warning); adapter, IP and Hosts changes (with Windows/UAC approval and preview); deleting app data (typed confirmation); deleting vault or SSH host-trust entries; downloading or installing updates; quitting with running diagnostics.
- **Undo toast instead:** deleting a saved target (exact restore).
- **No confirmation:** hiding or collapsing a sidebar or panel, theme and color toggles, tab selection, and warnings about turning off other confirmations.

## Wording

- Plain language first. Say what the user can do ("Look up DNS records"), not how the engine works.
- Errors say what happened and what to do next; keep the original technical error visible beside the advice.
- Use sentence case for labels and buttons; tool names in the navigation are proper names and keep their capitals. Name buttons with a verb and object (Start ping, Open terminal).
- Use the same name for the same thing everywhere: "Test a target", "File Transfer", "Saved targets", "Terminal", "Text Editor".
- Examples use documentation addresses only: `example.com`, `192.0.2.0/24`, `198.51.100.0/24`, `203.0.113.0/24`.
- Security notes are short and factual (TFTP: "no encryption or login; use only on trusted networks").
- What's New in release notes stays short and user-focused; technical detail goes under Additional Details.

## Keyboard

| Area | Shortcut | Action |
| --- | --- | --- |
| Application | Ctrl+Shift+P | Search tools and commands |
| Application | F1 | Keyboard shortcuts and help |
| Application | Ctrl+Shift+T | Reopen an eligible recently closed tab |
| Text Editor | Ctrl+N | New document |
| Text Editor / Terminal | Alt+Shift+Left / Right | Reorder a tab |
| Text Editor / Terminal | Shift+F10 | Open the tab menu |
| Terminal | Ctrl+Shift+F | Find in terminal output |
| File browser | F2 · Ctrl/Shift+click | Rename · select multiple |
| Panels | Enter or Space / arrow keys on a divider | Collapse or restore / resize |

Rules: application shortcuts never intercept keys a focused terminal session should send to a device. Every gesture-only action has a visible alternative (button, menu or command search entry).

## Accessibility

- Every interactive element is reachable by keyboard with a visible focus outline.
- Tabs use `role="tab"` with `aria-selected`; dividers use `role="separator"` with value attributes.
- Icon-only controls have accessible names; status changes have text equivalents.

## Proposing a change after the freeze

1. Describe the change with before/after screenshots at 1280×800 and 1920×1080, light and dark.
2. State which saved data, sessions and confirmations are affected (normally none).
3. Get the owner's approval, then update this document in the same pull request.
