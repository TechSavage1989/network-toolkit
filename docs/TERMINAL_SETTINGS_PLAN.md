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
- Moved to T3b: grouping Display into MobaXterm-like sections (Look and feel, Terminal features, Advanced, Local shell), and a fixed number of rows × columns, which changes how panes fit and resize.

**T3 · Terminal features and Advanced (v1.21.8)**
New "Terminal features" section in Display, below Terminal appearance:
- **Scrollback:** 500 to 10,000 lines (10,000 stays the default and the maximum, because a session moved to a separate window carries at most 10,000 lines), and show or hide the scrollbar.
- **Bell:** none (default, as today), flash the terminal, or play a sound.
- **Bracketed paste:** on (default) or off.
- **Word delimiters** for double-click selection (xterm.js's own set by default).
- **Advanced:** implicit CR in every LF; implicit LF in every CR (both off by default; they change only what is shown); answerback text for ^E (ENQ), empty by default, printable characters only, sent at most twice a second.
- Already present: auto-copy selected text, paste with right-click, multi-line paste review, close prompts (session confirmations).

**T3b · Paste safety, reconnection and layout**
- **Paste warnings** (multi-line review already exists): homograph characters, commands piped to interpreters (`curl … | sh`), changes to shell profiles, archives extracted to system folders, and short or plain-HTTP links.
- **Reconnection keys** after a session ends: R reconnects and S saves the session. Closing the tab on any other key needs care so a stray keypress does not close output you still want; it will be offered as an option.
- **Fixed size:** an optional fixed number of rows × columns instead of fitting the pane.
- **Grouping:** arrange Display into Look and feel, Terminal features, Advanced and Local shell. This moves existing controls, so it waits for the owner's approval under the layout freeze (docs/UI_GUIDELINES.md).

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
- **Escape-sequence behavior:** "destructive backspace", alternate-screen switching, "scrollback clearing" and xterm mouse reporting are standard escape sequences that xterm.js always honors. Mouse reporting may become an on/off option in T3b if the owner wants it.
- **MobaXterm's built-in Cygwin environment:** Use Windows PATH and Prompt type belong to MobaXterm's own Unix environment.

## Status
- T1: published in v1.21.6 Beta 7 on 2026-11-07 (syntax highlighting rule sets, Customize, .msyn import and export).
- T2: published in v1.21.7 Beta 8 on 2026-11-07 (font size 8 to 36, cursor type, bold in brighter colors, seven MobaXterm-named color schemes).
- T3: v1.21.8 Beta 9 candidate (scrollback and scrollbar, bell, bracketed paste, word delimiters, implicit CR/LF, answerback).
