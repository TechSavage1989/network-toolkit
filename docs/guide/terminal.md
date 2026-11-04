# Terminal

Open Terminal to connect to a permitted device using SSH, Telnet or serial access where supported. Save connection profiles and choose the appropriate connection type. Use approved credentials and validate host identity before trusting SSH connections.

## Sessions and panes
You can work with multiple tabs and split panes. Open another tab with the **+** at the end of the tab strip (or "Open a new terminal tab" in command search). The sidebar contains Saved, Recent, Files and Macros tabs; click the selected tab to hide the sidebar and any tab to show it again; tabs and panes may have context menus and drag interactions. Keep sessions open while you navigate other tools.

## Connection controls
Drag a Saved session to reorder the list (or focus it and press Alt+Shift+Up or Down); the order is kept. To run a Command Prompt or PowerShell tab as administrator after it started, choose **Open as administrator in a new tab** from the tab's More actions menu; Windows asks for approval and the original tab stays open. The Disconnect button's menu offers **Disconnect all** and **Close all tabs**. Reconnect an eligible session from its controls. The adjacent reconnect menu may also offer Reconnect all. Some tabs can be detached by dragging outside the window. Terminal tab actions are available from each tab's More actions button. The Terminal sidebar includes a visible show/hide control. Ctrl+Shift+P opens application command search when the console is not focused; F1 opens help outside focused terminal input.

## Console output tools
The console bar under a session offers Last command and Last batch. Last command is the output of the most recent single command; Last batch is the output of the most recent command batch, whether it came from a paste, a macro, a startup macro or the Run commands box. Select output and Copy output use the chosen block, Include commands adds the command lines, and the arrows jump to the block's start or end. Macro output is filled in from the run's verified capture when the run finishes, so it contains only lines the device actually received.

## Macros
Use the Macros library only for commands you are authorized to send. Review command targets and content before running macros in multiple sessions. Preferences, profiles, histories and active sessions must be preserved through upgrades.

For file browsing over SSH see [Transfers](transfers.md).
