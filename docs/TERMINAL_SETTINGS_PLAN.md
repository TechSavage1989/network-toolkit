# Terminal settings plan (MobaXterm-style)

Owner request (2026-11-07), with MobaXterm screenshots and seven `.msyn` syntax files: "I want the terminal to have similar configuration options, and similar syntax color recognition that MobaXTerm has."

The owner's non-negotiables in AGENTS.md apply unchanged. Saved sessions, credentials, macros and existing display preferences are kept, and every new setting is restored at startup and passed to separate Terminal windows.

## Phases (one opt-in beta each)

**T1 · Syntax highlighting (v1.21.6)**
- **Syntax highlighting** menu in Display: None, Network Toolkit device highlighting (the existing default), Standard keywords (OK/warning/error/…), Unix shell script, Network configuration (Cisco and others), Perl syntax, SQL syntax, and your own rule sets.
- **Customize**:
  - One pattern per color (Red, Green, Yellow, Blue, Magenta, Cyan, Underline). Each pattern is a regular expression or a comma-separated word list, with a case-sensitive option.
  - Live preview with your own text.
  - Duplicate, delete, import and export MobaXterm `.msyn` files (Windows-1252). Imported sets are named after their file.
- MobaXterm patterns work as written: POSIX bracket expressions (`[]…]`, `[^\]`, `[[:digit:]]`) and the `¨` start- and end-of-line marker. The first capture group is colored when the rest of the match is only delimiters.
- Patterns that would freeze the terminal are rejected with an explanation.
- Colors come from the terminal color scheme: bright colors on dark backgrounds, normal colors on light ones. Text colored by the remote program keeps its own color.

**T2 · Font, colors and cursor (v1.21.7)**
- **Font:**
  - Any font size from 8 to 36 px, typed in Display (Ctrl+wheel steps through the same range).
  - Bold and line spacing (both exist today).
  - Bold text shown in brighter colors (on by default, as before).
- **Colors:**
  - Additional schemes with MobaXterm's names: Dark background / Light text, Light background / Dark text, Soft pastel, Bright, Monokai, AfterGlow, Monochrome. Solarized and Custom already exist.
  - Background, foreground and the 16 terminal colors (these exist).
- **Cursor:** color (Customize colors), plus type (block, underline or line), each optionally blinking. Blinking block stays the default.
- Moved to T3: grouping Display into MobaXterm-like sections (Look and feel, Terminal features, Advanced, Local shell), which needs the T3 settings to fill them, and a fixed number of rows × columns, which changes how panes fit and resize.

**T3 · Terminal features and Advanced**
- Group Display into Look and feel, Terminal features, Advanced and Local shell (moved from T2).
- **Fixed size:** an optional fixed number of rows × columns instead of fitting the pane (moved from T2).
- **Scrollback:** a configurable number of lines (today 10,000), and show or hide the scrollbar.
- **Line endings:** implicit CR in every LF; implicit LF in every CR.
- **Bell:** none, visual flash or sound.
- **Answerback:** a reply text for ^E (ENQ).
- **Bracketed paste:** on or off.
- **Selection and copy:**
  - Auto-copy selected text (exists).
  - Characters that count as word delimiters for double-click selection.
  - Paste with right-click (exists).
- **Paste warnings** (multi-line already exists): homograph characters, commands piped to interpreters (`curl … | sh`), changes to shell profiles, archives extracted to system folders, and short or plain-HTTP links.
- **Reconnection keys** after a session ends: R reconnects, S saves the session, and any other key closes the tab.
- **Close prompt:** exists (session confirmations).

**T4 · Session logging**
- Save terminal output to a chosen folder, with a file name pattern.
- Log types: printable output, all session output, or printable output with timestamps.
- Logs can contain sensitive device output, so the folder is chosen explicitly and nothing is logged by default.

**T5 · Local shells**
- Windows: Command Prompt and Windows PowerShell (exist), PowerShell 7 (`pwsh`), WSL distributions (`wsl -l -q`) and Git Bash, each detected when installed.
- Backspace sends ^H (option).
- Linux shells follow docs/LINUX_PLAN.md phase L2.

## Not planned, and why
These MobaXterm options have no equivalent in the browser terminal (xterm.js) or do not apply to Network Toolkit:
- **Text rendering** handled by the browser engine: antialiasing toggle, ligatures (xterm.js has no supported ligature addon for WebView2), Arabic text shaping, bidirectional text, and font or terminal character sets (the terminal is always UTF-8).
- **Terminal modes** the remote program controls: forced local echo and local line editing, and forced application cursor or keypad modes.
- **Escape-sequence behavior:** "destructive backspace", alternate-screen switching, "scrollback clearing" and xterm mouse reporting are standard escape sequences that xterm.js always honors. Mouse reporting may become an on/off option in T3 if the owner wants it.
- **MobaXterm's built-in Cygwin environment:** Use Windows PATH and Prompt type belong to MobaXterm's own Unix environment.

## Status
- T1: published in v1.21.6 Beta 7 on 2026-11-07 (syntax highlighting rule sets, Customize, .msyn import and export).
- T2: published in v1.21.7 Beta 8 on 2026-11-07 (font size 8 to 36, cursor type, bold in brighter colors, seven MobaXterm-named color schemes).
