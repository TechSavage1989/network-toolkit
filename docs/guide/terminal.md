# Terminal

Open Terminal to connect to a permitted device using SSH, Telnet or serial access where supported. Save connection profiles and choose the appropriate connection type. Use approved credentials and validate host identity before trusting SSH connections.

## Sessions and panes
You can work with multiple tabs and split panes. Open another tab with the **+** at the end of the tab strip (or "Open a new terminal tab" in command search). The sidebar contains Saved, Recent, Files and Macros tabs; click the selected tab to hide the sidebar and any tab to show it again, and use ⇄ at the bottom of its icon strip to move it to the other side; tabs and panes may have context menus and drag interactions. Keep sessions open while you navigate other tools.

## Terminal settings
**Settings** in the terminal toolbar opens Terminal settings, with four tabs: **Appearance** (font, size, cursor, colors and a live preview), **Highlighting** (how output is colored), **Features** (scrollback, bell, selection, line endings, confirmations) and **Mouse & keyboard** (copy, paste, right-click, Ctrl+C, pager and paste warnings). It reopens on the tab you used last. **Tools** holds saved connections, the password manager, SSH host keys, the separate window and help.

## Syntax highlighting
Settings → Highlighting → **Syntax highlighting** chooses how terminal output is colored; the colors the chosen profile uses are shown right below it:
- **Universal (devices, servers and logs)** (default) recognizes addresses, MAC addresses, interfaces from many vendors (Cisco, Juniper, Aruba, Arista, Huawei/H3C, FortiGate, Palo Alto, MikroTik, Linux), prompts (including Windows and Huawei), commands and settings, states, log severities (`ERROR`, `[WARN]`, `level=info`, `%LINK-3-UPDOWN`), CLI errors (`% Invalid input`, `command not found`), ping loss, BGP/OSPF neighbor states, STP states and systemd service states. It colors only recognizable shapes, never ordinary sentences.
- **Keyword rule sets** work like MobaXterm's: Standard keywords (errors red, success green, warnings yellow, addresses magenta, information cyan, links underlined), Unix shell script, Network configuration (Cisco and others), Perl syntax and SQL syntax.
- **None** turns highlighting off.

**Colors:** click any color in the legend under the menu to change it for that profile; **Reset colors** goes back to the scheme's colors. In Customize, the color square beside each rule does the same for any rule set.

**Customize** lets you duplicate a rule set and edit one pattern per color (a regular expression or a comma-separated word list), try it on your own text, and import or export MobaXterm `.msyn` files. Patterns from MobaXterm work as they are. Colors follow the terminal color scheme, and text already colored by the device keeps its color. Local PowerShell and Command Prompt tabs keep their own shell colors.

## Font, colors and cursor
Settings → **Appearance**: pick a font (each is shown in its own typeface, with a note saying whether it is installed), type any size from 8 to 36 and press Enter (Ctrl+wheel over the terminal also changes it), and set the weight, line spacing and:
- **Cursor:** block, underline or line, each steady or blinking (blinking block is the default).
- **Color scheme:** besides the existing schemes, MobaXterm's Dark background / Light text, Light background / Dark text, Soft pastel, Bright, Monokai, AfterGlow and Monochrome. Customize colors changes any color, including the cursor's.
- **Show bold text in brighter colors:** on by default; turn it off to keep bold text in its normal color.

Changes apply to open terminals at once and are kept after a restart and in separate Terminal windows.

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

## Connection controls
Drag a Saved session to reorder the list (or focus it and press Alt+Shift+Up or Down); the order is kept. To run a Command Prompt or PowerShell tab as administrator after it started, choose **Open as administrator in a new tab** from the tab's More actions menu; Windows asks for approval and the original tab stays open. To replace the tab instead, choose **Restart this tab as administrator**: after you confirm and Windows approves, the elevated shell takes the tab's place and the old shell closes (anything running in it stops). If you decline the prompt, the tab is left as it is. The Disconnect button's menu offers **Disconnect all** and **Close all tabs**. Reconnect an eligible session from its controls. The adjacent reconnect menu may also offer Reconnect all. Some tabs can be detached by dragging outside the window. Terminal tab actions are available from each tab's More actions button. The Terminal sidebar includes a visible show/hide control. Ctrl+Shift+P opens application command search when the console is not focused; F1 opens help outside focused terminal input.

## Console output tools
The console bar under a session offers Last command and Last batch. Last command is the output of the most recent single command; Last batch is the output of the most recent command batch, whether it came from a paste, a macro, a startup macro or the Run commands box. Select output, Copy output and **Send to Text Editor** (the document-with-arrow button) use the chosen block, Include commands adds the command lines, and the arrows jump to the block's start or end. Macro output is filled in from the run's verified capture when the run finishes, so it contains only lines the device actually received. **Send to Text Editor** (also in the right-click menu) sends that output, or the text you selected in the terminal, to the Text Editor: pick a new document or an open one, and for an open one choose where it goes (by default you click the spot in the document and choose **Insert here**). When many tabs are open, scroll the tab row with the mouse wheel or the arrow buttons at its right end.

## Macros
Use the Macros library only for commands you are authorized to send. Review command targets and content before running macros in multiple sessions. Preferences, profiles, histories and active sessions must be preserved through upgrades.

For file browsing over SSH see [Transfers](transfers.md).
