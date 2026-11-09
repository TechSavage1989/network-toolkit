# Text Editor

The built-in Editor works with local and supported remote text documents. It provides multiple tabs, comparison, search, syntax highlighting and document navigation.

## Open and edit
Open or create a document, review its source/connection, then edit. Save explicitly. Save As opens the Windows Save As dialog for files on your computer and a folder picker for files on an SFTP server; an existing file is replaced only after you confirm. Keep unsaved changes and undo history when switching views or during update workflows.

## Compare
Choose two documents in the comparison view. The change map shows individual changed lines on the left and right tracks. Use the wheel or the map's scrollbar to browse it independently, then click a mark to jump to that line. Sync scroll keeps the comparison panes together as you navigate.

Choose **Show only differences** in **Options** to browse additions, removals, and modifications with their original line numbers. Use **Previous** and **Next** to switch blocks. **Exact** counts every space and tab. **Ignore changed spaces** matches `set  status` with `set status`. **Ignore all spaces** also matches `set status` with `setstatus`. These settings affect the comparison and leave the text unchanged.

**Dynamic fit** shows the entire document with one-pixel marks and is the default. **Compact** shows up to three lines per map pixel. The editor remembers the last map zoom you choose. Right-click either pane for difference navigation, map visibility, whitespace, scroll sync, and highlighting controls. Clicking or moving the caret to a line highlights its counterpart when one exists.

When both comparison documents use disk-backed editing, gray, unnumbered rows fill the space opposite insertions and deletions. The panes scroll on the same row positions when **Sync scroll** is enabled. Copying, editing, and saving operate on the original text.

Select text, then open **More** or right-click the selection to convert between UTF-8 text and hex or Base64. The result replaces the selection and can be undone. Each conversion accepts a selection of about one million characters.

Detached Editor windows have a minimum width of 640 pixels.

Starting with v1.21.20 Beta 1, large text files use temporary disk storage and display only the section you are working on. Open, edit, save and compare files without a fixed file-size or line-count limit. Opening, searching, comparing and saving still take time to read or write the data, and large files need space for temporary working copies.

Large comparisons run in the background and show changes as they are found. Use **Next** and **Previous** to move through differences. Large-file views use plain text and unwrapped lines. Comparisons highlight changed lines and may group distant rearrangements into one change. Select the commands you intend to send when using **Send to terminal** from a large file.

## Files and Connect sidebar
The sidebar's Connect and Files buttons open saved connections and the file browser. Drag its edge to resize it, or double-click the edge to hide it. The ⇄ button at the bottom of the sidebar's icon strip moves it to the other side of the editor; the side, width and open panel are kept when you restart.

## Tabs and splitters
Open a new document with the **+** at the end of the tab strip or Ctrl+N. When the tabs do not fit, scroll the tab row with the mouse wheel or the arrow buttons at its right end. Right-click the text (or press the Menu key or Shift+F10) for editing commands: Undo, Redo, Cut, Copy, Paste, Select all, Find, Replace, Go to line, comment, duplicate, case changes and Format document. To run text on a device, choose **Send to terminal** (the terminal-with-arrow button on the toolbar, the right-click menu, or Ctrl+Shift+Enter): pick the selection or the whole document and a connected Terminal session; the Terminal opens on that session and the text is entered like a paste, with your paste review settings. Each tab has a visible **More actions** menu with Move left, Move right and Move to separate window. Existing Alt+drag and Alt+Shift+Arrow shortcuts remain supported, as does tab dragging. Dividers can be resized and may support double-click reset or collapse.

## Configuration text
For network configuration syntax, choose Auto, the network language or plain text. This affects highlighting only; review actual device commands independently before applying configuration.
