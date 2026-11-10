# Text Editor

The built-in Editor works with local and supported remote text documents. It provides multiple tabs, comparison, search, syntax highlighting and document navigation.

## Open and edit
Open or create a document, review its source/connection, then edit. Save explicitly. Save As opens the Windows Save As dialog for files on your computer and a folder picker for files on an SFTP server; an existing file is replaced only after you confirm. Keep unsaved changes and undo history when switching views or during update workflows.

Hold **Ctrl** and roll the mouse wheel over editor text to change the font size. Wheel up enlarges the text and wheel down shrinks it. In Compare and Split View, both panes use the same size shown in the **Font size** control.

## Reopen files from the last session
Like Notepad and Notepad++, the Text Editor reopens the tabs that were open when it last closed, in the same order, with the same tab selected. Unsaved changes come back too, still marked as unsaved. The main window reopens its tabs the first time the Text Editor opens there; the Text Editor app reopens its own tabs when it starts. Detached editor windows do not keep a session.

- Files on your computer reopen from disk. A file with unsaved changes shows your unsaved version. If the file changed on disk since then, the editor says so, and **Save** refuses to replace the newer file; use **Save As** to keep your version. If the file was deleted, your text opens in a new untitled tab.
- New, unsaved documents reopen with their text. Blank, untouched documents are not kept.
- Unsaved text from an SFTP, FTP, SSH terminal or browser file opens as a new untitled tab that names where it came from. Reconnect and use **Save As** to save it there. Remote files without unsaved changes are not reopened, and connection details and passwords are never stored.

While this is on, closing the app or the Text Editor does not ask about unsaved changes that were kept. It still asks about any document whose text could not be kept. Closing a single tab, or **Close editor**, asks as before.

To turn this off, open **More** and clear **Reopen files and unsaved changes from the last session**. Turning it off removes the kept session and its unsaved text. Unsaved text is kept in a `Network_Toolkit.editor-session` folder in this app's data folder until it is saved or its tab is closed; large files are copied without a size limit, so the folder needs room for them. Moving the data folder moves the session too, and deleting stored app data removes it.

## Compare
Choose two documents in the comparison view. The change map shows individual changed lines on the left and right tracks. Use the wheel or the map's scrollbar to browse it independently, then click a mark to jump to that line. Sync scroll keeps the comparison panes together as you navigate.

Choose **Show only differences** in **Options** to scroll through all additions, removals, and modifications in one view. Original line numbers and gray alignment spacers stay visible. Use **Previous** and **Next** to jump to a change. **Exact** counts every space and tab. **Ignore changed spaces** matches `set  status` with `set status`. **Ignore all spaces** also matches `set status` with `setstatus`. These settings affect the comparison and leave the text unchanged.

**Dynamic fit** shows the entire document with one-pixel marks and is the default. **Compact** shows up to three lines per map pixel. The editor remembers the last map zoom you choose. Right-click either pane and open **Compare** for difference navigation, map visibility, whitespace, scroll sync, and highlighting controls. Clicking or moving the caret to a line highlights its counterpart when one exists.

When both comparison documents use disk-backed editing, gray, unnumbered rows fill the space opposite insertions and deletions. The panes scroll on the same row positions when **Sync scroll** is enabled. Copying, editing, and saving operate on the original text.

**Highlight changed characters** emphasizes the edited parts of paired lines. A replacement can highlight a whole value when only a few letters happen to match. A line marked **Added** or **Removed** uses a whole-line color.

Select text, then open **More** or right-click and choose **Convert text** to convert between UTF-8 text and hex or Base64. The result replaces the selection and can be undone. Each conversion accepts a selection of about one million characters.

Detached Editor windows have a minimum width of 640 pixels.

Starting with v1.21.20 Beta 1, large text files use temporary disk storage and display only the section you are working on. Open, edit, save and compare files without a fixed file-size or line-count limit. Opening, searching, comparing and saving still take time to read or write the data, and large files need space for temporary working copies.

Large comparisons run in the background and show changes as they are found. Use **Next** and **Previous** to move through differences. Use **Wrap** to fit long lines to the window. The first row shows the source line number and continuation rows leave the gutter blank. Compare aligns wrapped lines that fit in a loaded page when both files use the same editor mode. Very long lines can continue across pages, and their sections remain accessible. Comparisons highlight changed lines and may group distant rearrangements into one change. Select the commands you intend to send when using **Send to terminal** from a large file.

## Files and Connect sidebar
The sidebar's Connect and Files buttons open saved connections and the file browser. Drag its edge to resize it, or double-click the edge to hide it. The ⇄ button at the bottom of the sidebar's icon strip moves it to the other side of the editor; the side, width and open panel are kept when you restart.

## Tabs and splitters
Open a new document with the **+** at the end of the tab strip or Ctrl+N. When the tabs do not fit, scroll the tab row with the mouse wheel or the arrow buttons at its right end. Right-click editor text or press Shift+F10 for common editing and navigation commands. **Edit text** groups comment, duplicate, case, and format commands. **Convert text** groups hex and Base64 commands, and **Compare** groups comparison commands. Hover or click to open a submenu. With the keyboard, use Right Arrow to open it and Left Arrow to return to the main menu. To run text on a device, choose **Send to terminal** (the terminal-with-arrow button on the toolbar, the right-click menu, or Ctrl+Shift+Enter): pick the selection or the whole document and a connected Terminal session; the Terminal opens on that session and the text is entered like a paste, with your paste review settings. Each tab has a visible **More actions** menu with Move left, Move right and Move to separate window. Existing Alt+drag and Alt+Shift+Arrow shortcuts remain supported, as does tab dragging. Dividers can be resized and may support double-click reset or collapse.

## The Text Editor app
With the Windows setup installed, **Network Toolkit Text Editor** in the Start menu opens the editor in its own window, without the rest of the Toolkit. It contains what belongs to the editor: documents and tabs, compare and split view, the Files and Connect sidebar (files on your computer and on SFTP, FTP and SSH connections), text colors and **Send to terminal**. Diagnostics, the Toolkit's App settings and update controls stay in the main Toolkit window. **Open terminal** on a connected SFTP session is offered only in the main Toolkit window, because a signed-in connection cannot be handed to another app.

The three small buttons at the right end of the status bar switch the color theme, open **Window settings** (theme and accent color) and open help (F1). Command search (Ctrl+Shift+P) and the F1 help list only editor commands.

### Send to the Terminal app
In the Text Editor app, **Send to terminal** (toolbar button, right-click menu or Ctrl+Shift+Enter) lists the connected sessions of **Network Toolkit Terminal**, the separate Terminal app. Pick the selection or the whole document and a session, then choose **Send to terminal**.

- The Terminal app comes forward on that session and the text is entered like a paste, with the Terminal's own paste settings. Text with line breaks is shown there for review first when **Review multi-line paste** is on, and paste warnings apply. The Text Editor then tells you whether the text was sent or is waiting for your review in the Terminal.
- If the Terminal app is not open, or has no connected session, the dialog says so and **Open Terminal** starts it. Connect a session there, then send again.
- Up to 1 MiB of text can be sent at a time. Your document stays open and unchanged.
- If a dialog is open in the Terminal (a connection form, a sign-in or host key question, or another paste review), the text is refused instead of replacing it. Close the dialog and send again.

Text sent from the Terminal app with **Send to Text Editor** arrives here as a new document or in the open document you chose there; see [Terminal](terminal.md#the-terminal-app). Only the text you send travels between the two apps. Passwords, saved connections, host keys and file contents you did not send are never shared, and nothing leaves this computer. Both apps must use the same data folder, which they do when installed with setup. In the main Toolkit window **Send to terminal** works as before, inside that window.

## Configuration text
For network configuration syntax, choose Auto, the network language or plain text. This affects highlighting only; review actual device commands independently before applying configuration.
