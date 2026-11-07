# Network Toolkit UI Guidelines

**Status:** Adopted with the v1.19.0 stable release (2026-11-02), updated for the owner-approved v1.19.1 consolidation and the owner-requested 1.19.x changes. It describes the layout, components and wording as they ship in v1.21.11. The layout is frozen (see [Layout freeze](#layout-freeze)).

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

Sidebars (owner request, v1.21.3): every resizable sidebar can sit on either side where the layout allows. Terminal sessions, File Transfer client connections, File Transfer server settings, the Text Editor's Connect/Files sidebar and the diagnostic Configuration panel swap with an ⇄ icon button (`arrowSwap`, "Move … sidebar to left/right") at the bottom of the sidebar (owner request, v1.21.4): at the foot of the icon rail (`.sidebar-rail-swap`, so it stays reachable while collapsed), and for the Configuration panel, which has no rail while open, in a slim bottom row (`#toolConfigFooter`) that moves to the foot of its collapsed rail. Their icon rail stays on the sidebar's left; the resize edge faces the main content and dragging it away from the sidebar widens it. Details panes that already have a position menu (entry details, Network discovery device details, Packet capture packet details) offer Auto, Right, Left and Bottom; Auto never picks Left, and an explicit Left or Right is not moved below on narrow results (Network discovery still stacks below 900 px). The side is saved with the sidebar's other layout settings and restored at startup; Reset layout returns the Configuration panel to the left. Compact layouts that show one panel at a time ignore the side.

Terminal syntax highlighting (owner request, v1.21.6; plan in TERMINAL_SETTINGS_PLAN.md): Display's first row holds "Syntax highlighting" (None, Network Toolkit device highlighting [default], Standard keywords, Unix shell script, Network configuration, Perl syntax, SQL syntax, then the user's rule sets) and Customize. Customize opens the Syntax highlighting dialog (it closes Display first): Rule set picker, Duplicate, Delete (own sets, two-step), Import .msyn…, Export .msyn; Name, Regular expressions, Case sensitive; one field per color (Red, Green, Yellow, Blue, Magenta, Cyan, Underline) with its swatch and purpose; "Try it with your own text" with a live preview; Close and Use this rule set. Built-in rule sets are read-only; own sets save as you type. Local PowerShell and Command Prompt tabs keep their own shell colors.

Terminal font, colors and cursor (owner request, v1.21.7; T2 of TERMINAL_SETTINGS_PLAN.md): Display's font size is a number field (8 to 36, followed by "px", no clear button) confirmed with Enter or by leaving the field; Ctrl+wheel and Larger/Smaller text step through 8–36. Terminal appearance's grid is Font, Font weight, Line spacing, Cursor (Block, Underline, Line, Blinking block [default], Blinking underline, Blinking line), Color scheme; "Show bold text in brighter colors" (on by default) sits under the grid. Color scheme adds MobaXterm's names: Dark background / Light text, Light background / Dark text, Soft pastel, Bright, Monokai, AfterGlow and Monochrome. The style preview shows the chosen cursor shape, blinking unless the system asks for reduced motion.

Terminal features (owner request, v1.21.8; T3 of TERMINAL_SETTINGS_PLAN.md): a "Terminal features" section follows Terminal appearance in Display. Grid: Scrollback lines (number, 500–10,000, no clear button) and Bell (None [default], Flash the terminal, Play a sound); Word delimiters for double-click selection (full width, monospace). Checkboxes: Show scrollbar (on), Bracketed paste (on). "Advanced" sub-heading: Implicit CR in every LF, Implicit LF in every CR (both off) with a one-line hint, then Answerback to ^E (monospace, placeholder "No reply", hint warns against passwords), then Reset terminal features. The bell flash is a 3px accent inset outline for 200 ms. The Tools panel transcript note shows the chosen scrollback count.

Paste warnings and reconnection keys (owner request, v1.21.9; T3b): Mouse and keyboard defaults gains a "Warn before pasting" group of five checkboxes (all on) with a one-line hint. A paste that triggers a warning opens the paste review even for one line; under the status line a bordered "Check before sending" box lists each warning with a short monospace sample and the hint "Sending is still allowed…"; it updates as the text is edited and never disables Send. Terminal features adds "After a session ends, press R to reconnect or S to save it" (on) and an indented "Any other key closes the ended session's tab" (off, disabled while the keys are off). The Disconnected card shows "Press R to reconnect or S to save this session." under Reconnect only while Reconnect is available.

Dropdowns (owner request, v1.21.10): every single-choice `<select>` opens an app-colored list (panel background, 8px radius, accent check mark and accent text on the current choice, hover row) where the engine supports customizable selects; the closed control keeps its native look and size. Text fields with suggestions use the shared combobox: a chevron at the right edge (left of the clear button) opens a list of every choice; typed text moves the highlight to the best match and never filters. Font suggestions are shown in their own face with Installed / Not installed / System default notes. New dropdowns need no extra code: a `<select>` or an `<input list>` gets this behavior automatically.

Terminal settings and Tools (owner request, v1.21.11: "polish and clean up the display and tools menus of terminal … reduce redundancies"; supersedes the Display layout described for v1.21.6–v1.21.9 above). The toolbar's **Settings** (gear) opens Terminal settings: heading, sticky tab strip **Appearance · Highlighting · Features · Mouse & keyboard** (arrow keys move between tabs; the panel reopens on the last tab). Appearance: grid Font, Font size (px), Font weight, Line spacing, Cursor, Color scheme; Show bold text in brighter colors; font availability; one Preview card (style preview plus font/Unicode sample); Customize colors (console colors, device colors); Reset appearance. Highlighting: Syntax highlighting select, Customize, status, then an inline legend of the profile's colors on the terminal background; Underline highlighted text; "Network Toolkit device highlighting" with Recognized device-output rules and Device highlighting colors; Local shell syntax colors. Features: Terminal features grid and switches, Advanced, Reset terminal features, Session confirmations. Mouse & keyboard: a one-line scope note, then the former Tools section (copy, paste review, right-click, Ctrl+C, pager, Warn before pasting, context-menu tools). Removed from the panel: Toggle theme and App settings (in the app sidebar; separate terminal windows, which have no sidebar, show them as icons in the Tools heading), Highlight legend (now inline), Pane shortcuts (now in the Tools heading) and the Mouse and keyboard defaults link. **Tools** (no gear icon) holds Connections only, with Pane shortcuts and Help in its heading.

Terminal: a session sidebar (Saved, Recent, Files, Macros) docks left or right. Saved is listed by name until the user drags a row (or uses Alt+Shift+Up/Down on a focused row); from then on the user's order is kept. The Disconnect split button's menu holds Disconnect all and Close all tabs. A standard (not elevated) Command Prompt or PowerShell tab's menu offers "Open as administrator in a new tab", which starts an elevated copy through the Windows prompt and keeps the original tab, and "Restart this tab as administrator…", which confirms first and replaces the tab (same pane and position) only after the elevated shell is running. Starting directory drive buttons show the drive letter and volume name ("D: Data HDD"). When dragging a tab (Terminal or Text Editor) or a Saved session, the drop line appears only where releasing would change the order: over the item itself or the near half of its neighbour there is no line.

Password manager: vault setup offers "Set up with master password" and "Set up without master password". Any master password is accepted; the native Windows dialog shows a live strength gauge (text bar, label and hint). An unlocked vault offers Change, Remove or Add master password as applicable; Lock and forget is offered only when a master password exists. Choosing a saved credential fills in its username. Clicking the selected tab hides it; clicking any tab shows it.

Packet capture (owner request, v1.21.1: usable without packet-analysis knowledge): Set up a new capture lists three numbered steps (What is the problem?, Which network connection?, Website or device to check, optional), then a large Start, the duration and "open a saved capture file"; the unnumbered "Start capture · Reproduce the problem · Stop and review" strip above them hides in windows 720 px tall or less. While no capture is open the setup takes the height it needs, so Start stays in view. Which network connection? follows the Source interface pattern (owner request, v1.21.2): a small Refresh on the label row, a full-width picker, and options reading "Name - Hardware | 192.0.2.4 (IPv4)", or "| Not connected" / "| Disabled" when the adapter has no link. While recording, a coach panel under the status line shows the timer, packet count, what to try now and Stop and analyse. Troubleshooting leads with a verdict banner (problems / notes / clear / little traffic / empty) and Copy findings and Save troubleshooting report, then the "Check a website or device" card (four steps: Name lookup, Reach the device, Connect, Service reply, each with Show packets when it has evidence), then findings with plain headlines, "Problem" or "Good to know", what to check next and folded Technical details, then Traffic at a glance, What this capture cannot show (folded) and the terms guide. Conversations starts with Result (Worked, Refused, No reply, Already open, Answered, One way) and Service, and shows device names learned from name lookups in the capture. The view bar shows Pause updates only while recording and Save as with Save shown packets and Save all only when the capture has packets. There is no separate Sidebar button. Starting a macro or a One line at a time paste never switches the Macros panel to its Runs tab; a toast says where Pause and Stop are. The console bar's output tools (Last command / Last batch, Jump to start/end, Select output, Copy output, Include commands) cover typed commands and every command batch: a raw or paced paste, a macro run from the Library, a shortcut or a startup binding, and a Run commands list. "Last batch" is the name for all of these; do not call it "Last paste".

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
- When tabs do not fit, the strip scrolls sideways: the mouse wheel over it scrolls left and right, and two quiet chevron buttons (22 px, `--muted`, `--hover` when hovered, faded when disabled at an end) are pinned to the right edge over a short `--side` fade. They appear only while the tabs overflow, are not tab stops, never switch tabs, and a newly selected tab is scrolled clear of them (v1.20.13).

Linux edition (v1.21.0): the interface is the same. Its window is an app-mode browser window titled Network Toolkit; when no Chromium-family browser is installed, a power button (Quit Network Toolkit) joins the sidebar's utility buttons. App settings → General starts with an Applications menu block (Add / Update menu entry, Remove from menu). Features not yet on Linux keep their place and say they come in a later beta.

Test a target, Scan a network shortcut (v1.20.15, owner request): Ping, Ports, Monitor and Trace route show the "Scan a network in Network discovery" button and the "One hostname or IP address." note above the target field (`DISCOVERY_LINK_TOOLS`). Ping and Ports keep their network sweep mode; Monitor and Trace route only link to Network discovery.

Text Editor right-click menu (v1.20.14, owner request): right-click, the Menu key or Shift+F10 in the document text opens the app menu (same look as the Terminal menu, icons and shortcuts on the right) instead of the browser's: Undo, Redo | Cut, Copy, Paste, Select all | Send selection/document to terminal | Find, Replace, Go to line | Toggle comment, Duplicate line, UPPERCASE, lowercase, Format document. Unavailable items are disabled, never hidden.

Sending between the Text Editor and the Terminal (v1.20.13, owner request):
- Text Editor **Send to terminal** (an always-visible icon button after Go to line: terminal with an arrow, `sendTerminal`; Ctrl+Shift+Enter; also in the text right-click menu) sends the selection, or the whole document, to a connected Terminal session in the same window. The Terminal opens on that session and the text goes through the normal paste path, so the paste review, limits and destination checks apply; nothing is added (no Enter).
- The Terminal console bar's icon-only **Send to Text Editor** button (beside Copy output; icon only so the bar keeps its single row; `sendEditor`, a document with the same arrow), the Terminal right-click menu item and the tab menu's "Send output to Text Editor…" send the chosen Last command / Last batch output (the Copy output text) or the selected terminal text to a new document or an open one. For an open document, the default asks for the position: a bar above the document says what is ready and offers **Insert here** and **Cancel**; the user places the cursor or selects text to replace. Start, end, the document's current cursor and a line number are also offered. Inserts are ordinary undoable edits and mark the document unsaved.

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
