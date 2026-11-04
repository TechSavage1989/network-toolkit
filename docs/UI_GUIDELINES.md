# Network Toolkit UI Guidelines

**Status:** Adopted with the v1.19.0 stable release (2026-11-02), updated for the owner-approved v1.19.1 consolidation and the owner-requested 1.19.x changes. It describes the layout, components and wording as they ship in v1.20.12. The layout is frozen (see [Layout freeze](#layout-freeze)).

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

## Overview dashboard

The Overview's **Customize** button (next to Refresh) opens a dialog listing its sections: Summary cards; Gateways, DNS and DHCP; Public IP; Scan a local network; Network adapters; Recent activity. Each has a Show checkbox and Move up / Move down buttons; Reset to default restores all sections in the original order. Changes apply immediately and are saved. Network adapters and Recent activity share a two-column row when adjacent. Adapter warnings always show. With every section hidden the page says so instead of going blank. Each shown section also has a drag handle, a small pill centred on its top edge that appears on hover or keyboard focus: dragging it reorders sections (drop line above/below, or left/right inside the adapters/activity pair, only where the order would change; Escape cancels), and Alt+Shift+Up/Down on the focused handle moves it. The order is the same saved layout and is kept across restarts.

## Page anatomy

Top to bottom, a tool page has:
1. **Notices:** offline/disconnected banner, then the first-launch welcome (Overview only, until dismissed).
2. **Page actions row:** member tabs for merged entries (test tabs or protocol picker) first, then tool-level buttons such as Status colors and Reset layout.
3. **Command bar:** the one primary action (for example Look up DNS), the current target, and the Configuration/Results switch.
4. **Workspace:** Configuration (target, source interface, options; advanced options behind Expand options) and Results.

Member tabs:
- Test a target and Test a website tabs show a status dot for the current target: green finished, yellow running, red failed, faint not yet run. Switching tabs never starts anything.
- Tab labels may be short (DNS, Ports, Monitor, Details); the tooltip and command search use the full tool name.
- Test a target tabs default to Monitor, Ping, Trace route, DNS, Ports, Device check, Details (owner order, v1.20.10).
- Test a target and Test a website tabs can be reordered by dragging (drop line only where the order would change) or Alt+Shift+Left/Right; each group keeps its own saved order, restored when the app restarts.
- Test tabs stay on one row. When space runs out they scroll sideways, and they stay visible in short windows where the rest of the page heading is hidden.
- File Transfer protocol buttons carry a tooltip with the protocol's security (SFTP encrypted over SSH; FTPS encrypted, FTP not; TFTP has no encryption or login).

Local network > IP configuration, top to bottom: Public IP, snapshot time, adapter picker, Current configuration beside the IPv4 editor, then Network actions last. The page renders as soon as the adapter settings are read; Network actions fill in after the slower adapter-state read and show "Reading adapter state" until then.

Terminal: a session sidebar (Saved, Recent, Files, Macros) docks left or right. Saved is listed by name until the user drags a row (or uses Alt+Shift+Up/Down on a focused row); from then on the user's order is kept. The Disconnect split button's menu holds Disconnect all and Close all tabs. A standard (not elevated) Command Prompt or PowerShell tab's menu offers "Open as administrator in a new tab", which starts an elevated copy through the Windows prompt and keeps the original tab, and "Restart this tab as administrator…", which confirms first and replaces the tab (same pane and position) only after the elevated shell is running. Starting directory drive buttons show the drive letter and volume name ("D: Data HDD"). When dragging a tab (Terminal or Text Editor) or a Saved session, the drop line appears only where releasing would change the order: over the item itself or the near half of its neighbour there is no line.

Password manager: vault setup offers "Set up with master password" and "Set up without master password". Any master password is accepted; the native Windows dialog shows a live strength gauge (text bar, label and hint). An unlocked vault offers Change, Remove or Add master password as applicable; Lock and forget is offered only when a master password exists. Choosing a saved credential fills in its username. Clicking the selected tab hides it; clicking any tab shows it. There is no separate Sidebar button. Starting a macro or a One line at a time paste never switches the Macros panel to its Runs tab; a toast says where Pause and Stop are. The console bar's output tools (Last command / Last batch, Jump to start/end, Select output, Copy output, Include commands) cover typed commands and every command batch: a raw or paced paste, a macro run from the Library, a shortcut or a startup binding, and a Run commands list. "Last batch" is the name for all of these; do not call it "Last paste".

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

Tab strips (Terminal sessions, Text Editor documents; `web/assets/ux-tab-strip.css`):
- Both tools use the same strip: 40 px tall on `--side`, tabs 30 px with `--ux-radius-sm`, 150 to 255 px wide (135 px under 720 px), label text `--ux-text-xs`.
- Inactive tabs are transparent in `--muted`; hover `--hover`; the selected tab is `--accentBg` with accent text at weight 650, the same look as the page tabs. No underline or side borders.
- Each tab keeps its status dot (Terminal) or unsaved marker (Text Editor), the "⋯" More actions button and the close button as 22 px quiet controls (`--faint`, `--muted` on the selected tab, `--hover` when hovered).
- Every strip that can open a tab ends with a 30 px tab-shaped "+" button: dashed `--border`, accent plus icon, filling with `--accentBg` on hover. It is the only New tab control in each tool (the Terminal command bar's New tab button was removed at the owner's request in v1.20.6; command search keeps "Open a new terminal tab" and the Text Editor keeps Ctrl+N). The Terminal strip stays visible with just the "+" when no tab is open; the editor shows it in the single view and the left view only.

Buttons:
- One **primary** button per view, for the main action. Other actions use the default or `ghost` style; `small` inside toolbars and tab rows.
- Icon-only buttons need an `aria-label` and a tooltip; show the shortcut in the tooltip when one exists.

States every tool provides:
- **Empty:** what the tool does, what to enter, and an example (see `web/assets/ux-phase5.js`).
- **Loading/progress:** the shared `progress` style or a running status; never a blank area.
- **Error:** the original error text plus a next step. Never suggest bypassing a certificate, host-key or permission warning.

Results pages show the result header (title, status, summary, target, recorded time) and then the tool's primary results only. There is no Activity log on the page (events remain in Export), and no recorded-source, target or recorded-time fields in the body. A collapsed "Recorded source" panel appears only when a specific adapter was chosen. Filter/sort toolbars appear on lists that can grow long (Ping and Monitor samples, Network discovery, generic tables beyond 25 rows), not on short fixed lists (Device check, Trace route, DNS records, TCP ports, SMTP responses). Row actions open from the row itself (right-click or Enter); sample tables carry no per-row Tools button or copy icons. Timestamps inside a results table show the clock only; the date is in the header. Explanatory text about a tool's results belongs in its Help and formats section, not on the results page.

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

- **Keep:** closing unsaved Editor work; closing or disconnecting live Terminal sessions (including Reconnect all and Disconnect all); macros sent to multiple targets; reviewed terminal paste; file overwrite (the Windows Save As prompt for local files, the editor's Replace confirmation for SFTP files); starting FTP/TFTP servers (plain-text warning); adapter, IP and Hosts changes (with Windows/UAC approval and preview); deleting app data (typed confirmation); deleting vault or SSH host-trust entries; setting up the vault without a master password or removing it (native Windows confirmation or current password); downloading or installing updates; quitting with running diagnostics.
- **Undo toast instead:** deleting a saved target (exact restore).
- **No confirmation:** hiding or collapsing a sidebar or panel, theme and color toggles, tab selection, and warnings about turning off other confirmations.

## Wording

- Plain language first. Say what the user can do ("Look up DNS records"), not how the engine works.
- Errors say what happened and what to do next; keep the original technical error visible beside the advice.
- Use sentence case for labels and buttons; tool names in the navigation are proper names and keep their capitals. Name buttons with a verb and object (Start ping, Open terminal).
- Use the same name for the same thing everywhere: "Test a target", "File Transfer", "Saved targets", "Terminal", "Text Editor", "Last batch" (a pasted, macro or Run commands batch).
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
| Terminal Saved list | Alt+Shift+Up / Down | Move a focused Saved session |
| Test a target / Test a website | Alt+Shift+Left / Right | Reorder a focused test tab |
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
