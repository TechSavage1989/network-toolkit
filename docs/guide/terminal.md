# Terminal

Open Terminal to connect to a permitted device using SSH, Telnet or serial access where supported. Save connection profiles and choose the appropriate connection type. Use approved credentials and validate host identity before trusting SSH connections.

Saved and recent connections, saved credentials, trusted SSH host keys, macros and all Terminal settings are the same in the main window's Terminal page and in the separate **Network Toolkit Terminal** window, and a change in one appears in the other within a few seconds. See [One set of settings and saved data for all apps](settings-and-updates.md#one-set-of-settings-and-saved-data-for-all-apps).

## Sessions and panes
You can work with multiple tabs and split panes. Open another tab with the **+** at the end of the tab strip (or "Open a new terminal tab" in command search). The sidebar contains Saved, Recent, Files and Macros tabs; click the selected tab to hide the sidebar and any tab to show it again, and use ⇄ at the bottom of its icon strip to move it to the other side; tabs and panes may have context menus and drag interactions. Keep sessions open while you navigate other tools.

Buttons that show a row are toggles: press them again to hide it. **Find** (magnifier, or Ctrl+Shift+F) shows the find row for terminal output and hides it again (Escape also hides it), and **Command bar** shows or hides the Send to row. Each looks pressed while its row is shown.

## Terminal settings
**Settings** in the terminal toolbar opens Terminal settings, with three tabs: **Appearance** (font, size, cursor, syntax highlighting, colors and a live preview), **Features** (scrollback, bell, selection, line endings, confirmations, and making the Terminal the default for SSH, Telnet and serial links) and **Mouse & keyboard** (copy, paste, right-click, Ctrl+C, pager and paste warnings). It reopens on the tab you used last. There is no separate Tools menu: open, edit or delete saved connections from the **Saved** sidebar (right-click for more), use **Password manager** and **SSH host keys** at the bottom of the Saved and Recent sidebar, the ⓘ button beside the pane layout for pane shortcuts, and **Separate terminal window** on the Start a terminal screen. Help is F1 or the ? button in the app sidebar; separate terminal windows show theme, window settings and help (?) at the bottom of the sidebar's icon strip.

## Syntax highlighting
Settings → Appearance → **Syntax highlighting** chooses how terminal output is colored. The picker sits right above the preview, and both stay in view while you scroll through the colors and options below them, so every change shows at once. The colors the chosen profile uses are listed under the preview (click one to change it):
- **Universal (devices, servers and logs)** (default) recognizes addresses, MAC addresses, URLs and network paths (`https://…`, `smb://…`, `\\server\share`, `192.0.2.10:/export`), interfaces from many vendors (Cisco, Juniper, Aruba, Arista, Huawei/H3C, FortiGate, Palo Alto, MikroTik, Linux), prompts (including Windows, macOS and Huawei), commands and settings, states, log severities (`ERROR`, `[WARN]`, `level=info`, `%LINK-3-UPDOWN`), CLI errors (`% Invalid input`, `command not found`), ping loss, BGP/OSPF neighbor states, STP states and systemd service states. It colors only recognizable shapes, never ordinary sentences.
- **Keyword rule sets** work like MobaXterm's: Standard keywords (errors red, success green, warnings yellow, addresses magenta, information cyan, links underlined), Unix shell script, Network configuration (Cisco and others), Perl syntax and SQL syntax.
- **None** turns highlighting off.

**Colors:** click any color in the legend under the menu to change it for that profile; **Reset colors** goes back to the scheme's colors. In Customize, the color square beside each rule does the same for any rule set.

**Device prompt name:** choose how the device's name in its prompt (`user@host` in `user@host:~$`, `switch` in `switch#`) is colored, with its color beside the choice. *Color it unless the device does* is the default; *Always my color* uses your color even when the device colors its own prompt (such as the green Linux prompt); *No color* shows the name as plain text. It works with every profile.

**Underlines:** *Underline highlighted text* and *Underline URLs and network paths* are separate switches, so links can stay underlined when other highlights are color only.

**Preview:** the Preview in Settings → Appearance colors its sample output with the profile you use, including keyword rule sets and your own imported sets, and names that profile in its heading.

**Universal rules and colors:** choose **Rules and colors…** (or **Customize** while Universal is selected) to turn each kind of highlight on or off, change its color and try it on your own text.

**Customize** shows each color of a rule set as a card named by what it marks (such as Errors and failures): switch it on or off, click its color to change it, and (for your own and imported sets) rename it and edit its words or pattern. Every rule set also has a **Device prompts** card, like Universal, to color the device prompt and choose how its name is shown. Built-in sets can't be edited, but their colors can be switched and recolored; Duplicate makes an editable copy with the same colors and switches. Each pattern is a regular expression or a comma-separated word list; try it on your own text, and import or export MobaXterm `.msyn` files. Patterns from MobaXterm work as they are. Colors follow the terminal color scheme, and text already colored by the device keeps its color. Local PowerShell and Command Prompt tabs keep their own shell colors.

## Font, colors and cursor
Settings → **Appearance**: pick a font (each is shown in its own typeface, with a note saying whether it is installed), type any size from 8 to 36 and press Enter (Ctrl+wheel over the terminal also changes it), and set the weight, line spacing and:
- **Cursor:** block, underline or line, each steady or blinking (blinking block is the default).
- **Color scheme:** besides the existing schemes, MobaXterm's Dark background / Light text, Light background / Dark text, Soft pastel, Bright, Monokai, AfterGlow and Monochrome. Customize colors changes any color, including the cursor's.
- **Show bold text in brighter colors:** on by default; turn it off to keep bold text in its normal color.

Changes apply to open terminals at once, are kept after a restart, and are shared with the separate Terminal window and the other Network Toolkit apps.

## Terminal features
Settings → **Features** sets:
- **Scrollback lines:** 500 to 10,000 lines kept above the screen (10,000 by default).
- **Bell:** none, flash the terminal, or play a sound when a device sends a bell.
- **Word delimiters:** the characters that end a word when you double-click to select.
- **Show scrollbar** and **Bracketed paste** (programs that ask for it can tell pasted text from typing).
- **Advanced:** *Implicit CR in every LF* and *Implicit LF in every CR* fix devices whose lines overlap or run together; they change only what is shown, not transcripts or saved output. *Answerback to ^E* is sent automatically whenever a device asks for it, so never put a password there.
- **After a session ends:** press R to reconnect or S to save the session. Optionally, any other key closes the ended session's tab.

## Paste warnings
Before text is pasted, the terminal checks it for commands that download and run a script (`curl … | sh`), changes to shell startup files, archives unpacked into system folders, look-alike, hidden or typographic characters, and plain-HTTP or shortened links. If any are found, the paste review opens, even for one line, and lists them. You can edit the text or send it anyway. Turn each kind off in Settings → Mouse & keyboard → **Warn before pasting**.

## Open SSH, Telnet and serial links
With the Windows setup installed, Network Toolkit Terminal can open `ssh://`, `telnet://` and `serial:` links, for example from a browser, a wiki or an inventory page. Windows asks you to choose the app for each kind of link; setup never changes your choice for you.

**Make it the default:** in Terminal settings → **Features** → **Links**, choose **Make Network Toolkit the default for SSH, Telnet and serial links**. Windows Settings → Default apps opens; choose **Network Toolkit Terminal** for SSH, TELNET and SERIAL. (You can also go to Settings → Apps → Default apps → **Choose defaults by link type**.) The portable edition is not registered with Windows, so it cannot be chosen; install Network Toolkit with its setup to use it for links.

**What a link does:** it opens the Terminal (or brings the open Terminal window forward) with the **New terminal connection** form filled in. Nothing connects until you choose **Connect**. If a saved connection has the same type, host, port and user, the form offers **Use saved connection <name>** so its saved credential and settings are used. SSH host key checks, the credential vault and every other protection work as they do for a connection you type in. Links that arrive while a form is open wait and open one after another.

Supported links:
- `ssh://host`, `ssh://user@host:2222` (an IPv6 address goes in brackets: `ssh://user@[2001:db8::1]`). PuTTY-style extras such as `;fingerprint=…` are ignored; the Terminal checks host keys itself.
- `telnet://host` or `telnet://host:2323`. Telnet sign-in happens in the session, so a user name in a Telnet link is not used.
- `serial:COM3` or `serial://COM3`, optionally with line settings: `serial:COM3?baud=115200&data=8&parity=none&stop=1&flow=none`. `parity` is none, odd, even, mark or space (or n, o, e, m, s); `stop` is 1, 1.5 or 2; `flow` is none, xonxoff or rtscts.

**Safety:** **passwords in links are ignored**; the form says so when a link contains one. A link cannot run a command, choose a saved credential, a local shell or administrator mode, or carry anything but a destination. Links with paths, commands, other options, spaces or control characters are refused. A link is never saved in settings, and an update restart does not reopen it.

## Connection controls
Drag a Saved session to reorder the list (or focus it and press Alt+Shift+Up or Down); the order is kept. To run a Command Prompt or PowerShell tab as administrator after it started, choose **Open as administrator in a new tab** from the tab's More actions menu; Windows asks for approval and the original tab stays open. To replace the tab instead, choose **Restart this tab as administrator**: after you confirm and Windows approves, the elevated shell takes the tab's place and the old shell closes (anything running in it stops). If you decline the prompt, the tab is left as it is. The Disconnect button's menu offers **Disconnect all** and **Close all tabs**. Reconnect an eligible session from its controls. The adjacent reconnect menu may also offer Reconnect all. Some tabs can be detached by dragging outside the window. Terminal tab actions are available from each tab's More actions button. The Terminal sidebar includes a visible show/hide control. Ctrl+Shift+P opens application command search when the console is not focused; F1 opens help outside focused terminal input.

## Console output tools
The console bar under a session offers Last command and Last batch. Last command is the output of the most recent single command; Last batch is the output of the most recent command batch, whether it came from a paste, a macro, a startup macro or the Run commands box. Select output, Copy output and **Send to Text Editor** (the document-with-arrow button) use the chosen block, Include commands adds the command lines, and the arrows jump to the block's start or end. Macro output is filled in from the run's verified capture when the run finishes, so it contains only lines the device actually received. **Send to Text Editor** (also in the right-click menu) sends that output, or the text you selected in the terminal, to the Text Editor: pick a new document or an open one, and for an open one choose where it goes (by default you click the spot in the document and choose **Insert here**). When many tabs are open, scroll the tab row with the mouse wheel or the arrow buttons at its right end.

## Separate terminal windows
Detaching a session or the whole Terminal workspace (dragging a tab out, or **Detach / reattach session** and **Detach / reattach workspace** in a tab's menu) keeps every session connected: nothing is reconnected or signed in again. When **Network_Toolkit_Terminal.exe** of the same version is beside `Network_Toolkit.exe` (setup and the portable folder include it), that window runs as the Terminal program: it has the Terminal icon and groups with **Network Toolkit Terminal** on the taskbar. It stays connected to the Toolkit window it came from, and **Detach / reattach** docks the sessions back. Without that program the window opens as before.

**Send to Text Editor** works in such a window: it lists the open documents of the main window and of every separate Text Editor window of the same Toolkit, grouped by window, and **New document** goes to the main window's Text Editor (while the main window is closed, to a separate Text Editor window). The window that holds the document adds the text as an ordinary, undoable edit; if a dialog is open there, nothing is added and you are told why. In the main window the same dialog also lists the documents of its separate Text Editor windows, and **Send to terminal** in the main window or a separate editor window lists the sessions of separate Terminal windows. The window that receives the text comes to the front (Windows may instead flash its taskbar button if another program is in use).

A window popped out of the **Terminal app** works like the Terminal app itself: its **Send to Text Editor** reaches Network Toolkit Text Editor (or the main window) exactly as described in [Send output to the Text Editor app](#send-output-to-the-text-editor-app), and text that the Text Editor app sends to a session shown in that window arrives there.

The SFTP browser of a separate Terminal window is read-only there, with one exception in a detached **workspace** (not a single-session window): **Edit** (button, right-click menu) opens the selected file in the main window's Text Editor, on the same SSH connection, where you edit and save it as if you had opened it in the main window (the same unsaved-changes and changed-on-the-server checks). Only that file becomes available to the main window, and only while that tab is open, the session stays in that workspace window and stays connected. It can be saved only to the same file: to keep it elsewhere, save a copy locally or open the file from the main window's own connection. The tab stays in the main window (it cannot be moved to another window), and if the workspace window gives the session back or closes, saving says so: reopen the file from the main window to save it. If the main window is closed, **Edit** opens it again and the file opens there once it is ready (if it does not open within about 20 seconds, you are told and nothing is opened).

## The Terminal app
With the Windows setup installed, **Network Toolkit Terminal** in the Start menu opens the Terminal in its own window, without the rest of the Toolkit. It is its own program, **Network_Toolkit_Terminal.exe** (also in the portable folder), with its own taskbar button and name in Task Manager; starting `Network_Toolkit.exe --terminal`, an older shortcut or a link opens the same Terminal window. It is also the window that **Separate terminal window** opens. It contains what belongs to the Terminal: SSH, Telnet and serial sessions, **Command Prompt and PowerShell** tabs (with a starting folder and **Run as administrator**, exactly as in the main Toolkit window), tabs and panes, saved and recent connections, the password manager, SSH host keys, macros, Terminal settings, the SFTP sidebar for SSH sessions with its floating editor, and **Send to Text Editor**.

Actions that open another Toolkit tool are not shown there: **Open Text Editor** in a tab's menu, and **Editor** and **Expand** in the SFTP sidebar. Remote files still open in the Terminal's own floating editor with **Edit**. Theme, **Window settings** and help (?) are at the bottom of the sidebar's icon strip. Command search (Ctrl+Shift+P) and the F1 help list only Terminal commands.

### Send output to the Text Editor app
In the Terminal app, **Send to Text Editor** (the document-with-arrow button under the console, **Send output to Text Editor…** in a tab's menu, or **Send to Text Editor…** in the right-click menu) sends to **Network Toolkit Text Editor**, the separate Text Editor app. The dialog is the same: choose the last command or batch output or the selected text, then a new document or one of the documents open in the Text Editor app, and where it goes.

You are never asked to open anything first, and you choose **Send to editor** once. The Terminal app looks for an editor in this order:

1. **Network Toolkit Text Editor**, if it is open. Documents in its popped-out windows are listed too, and the text goes to the window that holds the document, which comes to the front.
2. Otherwise the main **Network Toolkit** window, if it is open: its open documents are listed, and the text goes to its Text Editor, which comes to the front. A window that has only just opened gets up to about 3 seconds to get ready before it is passed over. Documents in its separate Text Editor windows are listed too, also while the main window itself is closed; the text then goes to that window.
3. Otherwise Network Toolkit Text Editor is started for you and the text you sent goes into a new document as soon as it is ready. If it does not come up within about 25 seconds, the Terminal says so and nothing is sent; your terminal output is still there to send again.

- The editor comes forward with the text added as an ordinary edit: the document is marked unsaved and Undo removes it. For **A position I choose in the editor**, the Text Editor shows its **Insert here** bar and waits for you.
- An app that is still open from an older version cannot receive text. It is skipped, and the dialog names it so you can close it and open it again.
- Up to 8 MiB of text can be sent at a time.
- If a dialog is open in the Text Editor, the text is refused instead of replacing it. Close the dialog and send again.

Text sent from the Text Editor app with **Send to terminal** arrives here through the normal paste path: it is entered in the session you chose there, and text with line breaks opens the paste review first when **Review multi-line paste** is on. The same is true in the main Toolkit window when the Text Editor app sends to it. Only the text you send travels between the apps. Passwords, saved connections, host keys and terminal output you did not send are never shared, and nothing leaves this computer. The apps must use the same data folder, which they do when installed with setup. In the main Toolkit window **Send to Text Editor** works inside that window and its separate windows (see [Separate terminal windows](#separate-terminal-windows)); it does not send to the Text Editor app.

## Macros
Use the Macros library only for commands you are authorized to send. Review command targets and content before running macros in multiple sessions. Preferences, profiles, histories and active sessions must be preserved through upgrades.

For file browsing over SSH see [Transfers](transfers.md).
