# Text Editor

The built-in Editor works with local and supported remote text documents. It provides multiple tabs, comparison, search, syntax highlighting and document navigation.

## Open and edit
Open or create a document, review its source/connection, then edit. Save explicitly. Save As opens the Windows Save As dialog for files on your computer and a folder picker for files on an SFTP server; an existing file is replaced only after you confirm. Keep unsaved changes and undo history when switching views or during update workflows.

Hold **Ctrl** and roll the mouse wheel over editor text to change the font size. Wheel up enlarges the text and wheel down shrinks it. In Compare and Split View, both panes use the same size shown in the **Font size** control.

Editor preferences (word wrap, highlighting, indent, font size, colors, compare options, and whether files are reopened) and the saved connections in the Files sidebar are the same in the main window's editor and in the separate **Network Toolkit Text Editor** window. See [One set of settings and saved data for all apps](settings-and-updates.md#one-set-of-settings-and-saved-data-for-all-apps).

## Open any file
The Text Editor opens whatever you give it, like Notepad++: a file of a type it knows, a type nobody knows, or a file with no extension at all. It never changes a file just by opening it, and a save changes only what you edited.

- **Text is recognized by its content, not its name.** UTF-8 (with or without a byte order mark) and UTF-16 with a byte order mark are read as such. Text that is not valid UTF-8 is read as **Windows-1252** (Western European, which also covers Latin-1), so every byte shows as a character and none is lost.
- **Saving keeps the file's own format.** The encoding, the byte order mark and the line endings stay as they were. If you save without editing, or with **Save As**, the file is written back byte for byte.
- **Mixed line endings are kept line by line.** When a file's lines end in different ways (some CRLF, some LF or CR), **Line endings** shows **Mixed (keep each line)** and the status bar says **Mixed line endings**. Every line you did not retype keeps its own ending; lines you add get the ending most of the file uses. Commands that rewrite lines keep the endings as well: **Replace All** changes only its matches, and changing case, indenting, commenting, trimming or sorting a selection (or pasting as many lines as you selected) leaves each line's ending where it was. There is one exception, so that the file always reopens as the text you saw: a line break that would be saved as a bare LF directly after a line ending in a lone CR is saved as CRLF, because CR followed by LF reads back as a single line break. Choose **LF**, **CRLF** or **CR** to convert all lines when you save.
- **Windows-1252 files stay Windows-1252.** If you type a character that encoding does not have (for example `→`), Save stops and tells you; choose **UTF-8** under **Encoding** to save every character. Nothing is written until then.
- **A file that is not text is shown read-only.** A program, an archive, an image, or any file with NUL bytes (also UTF-16 text without a byte order mark) opens so you can look at it and search it: its bytes are shown as characters and control symbols such as `␀`. The editor says that it is not a text file, **Save**, **Save As**, **Line endings** and **Encoding** are off, and nothing you type, paste or replace reaches it. The editor cannot save such a file, so it can never damage one. **Reload** reads it again.
- **Size does not matter.** Large files of any kind use temporary disk storage as described below. Showing a large file that is not text can need two to three times its size in temporary space.
- **A folder, a missing file, or a file another program has locked** gives a short message and opens nothing.
- **Coloring** follows the file name when the editor has a language for it, and the content for network device configurations. Everything else opens as plain text; choose a language under **Language** if you want one.

### Make the Text Editor a default app in Windows
With the Windows setup installed, Windows offers **Network Toolkit Text Editor** for files. Setup only makes it available: it never changes which app opens a file, and it never replaces another app's registration. You choose in Windows. The portable program registers nothing unless you ask it to; see [Portable copy: use its Text Editor in Windows](#portable-copy-use-its-text-editor-in-windows).

- **One file type:** right-click a file, choose **Open with → Choose another app**, pick **Network Toolkit Text Editor** and choose **Always**.
- **Several types at once:** open **Settings → Apps → Default apps**, search for **Network Toolkit Text Editor** and open it. Windows lists every type below; choose the editor for each one you want. (On Windows 10: **Default apps → Set defaults by app → Network Toolkit Text Editor → Manage**.) The last page of Setup has a box, **Open Windows Default Apps to choose a text editor**, that takes you there.
- **Unknown types and files without an extension:** the editor is in **Open with → Choose another app** for every file (under **More apps** if it is not at the top). For a type Windows has not seen before, **Always** makes it the default for that type.
- **Right-click entry (optional):** in Setup, tick **Right-click menu: Edit with Network Toolkit Text Editor** to get that entry for every file, like Notepad++'s. On Windows 11 it is under **Show more options**. For a silent setup add `/ContextMenu`. The choice is remembered when you upgrade; run Setup again and clear the box, or run a silent setup with `/NoContextMenu`, to remove it (the two switches cannot be used together). Uninstall removes it.

Setup lists the editor in Default apps for these types:

<!-- text-extensions:start (generated from scripts/installer/text-extensions.nsh; run node scripts/text-extensions.cjs --write) -->
- **Plain text and logs:** `.txt` `.text` `.log` `.out` `.err` `.nfo` `.list`
- **Configuration:** `.ini` `.cfg` `.conf` `.cnf` `.config` `.properties` `.env` `.toml` `.yaml` `.yml` `.editorconfig` `.rules` `.inf` `.reg`
- **Network devices:** `.rsc` `.cisco` `.ios` `.junos` `.cli` `.zone`
- **Certificates in text form:** `.pem` `.csr`
- **Data:** `.json` `.jsonl` `.ndjson` `.jsonc` `.json5` `.xml` `.xsd` `.xsl` `.xslt` `.csv` `.tsv` `.sql`
- **Documentation and web:** `.md` `.markdown` `.rst` `.adoc` `.tex` `.html` `.htm` `.xhtml` `.css` `.scss` `.less`
- **Scripts:** `.sh` `.bash` `.zsh` `.ksh` `.fish` `.ps1` `.psm1` `.psd1` `.bat` `.cmd` `.vbs` `.awk` `.tcl`
- **Source code:** `.py` `.pyw` `.js` `.mjs` `.cjs` `.jsx` `.ts` `.tsx` `.go` `.c` `.h` `.cpp` `.cc` `.cxx` `.hpp` `.hh` `.cs` `.java` `.kt` `.rs` `.rb` `.pl` `.pm` `.php` `.lua` `.swift` `.vb`
- **Build and infrastructure:** `.mk` `.mak` `.cmake` `.gradle` `.proto` `.tf` `.tfvars` `.hcl` `.dockerfile`
- **Version control:** `.diff` `.patch` `.gitignore` `.gitattributes` `.gitconfig` `.gitmodules` `.dockerignore`
<!-- text-extensions:end -->

If Windows already has an **Open with** entry for a `Network_Toolkit.exe` in another folder (for example the portable program, picked there earlier with **Choose an app on your PC**), Setup leaves that entry exactly as it is, and uninstall does not remove it. **Open with** for unknown file types then still starts that other copy; the types listed above, Default apps and the right-click entry use the installed editor. A registration the portable program made itself (next section) is different: Setup takes it over.

Documents, images, media, archives and programs are not in the list, because the editor is not the right default for them; they still open through **Open with**. The file you open from Windows becomes the selected tab, as described next.

**File icons:** a file whose type opens in the Text Editor shows a plain sheet of paper: the page from the editor's icon, without the blue tile behind it. The editor itself keeps its full icon in the Start menu, on the taskbar, in Default apps, in **Open with** and on the right-click entry. A type outside the list above that you set to the editor through **Open with** may show the editor's icon instead. To get the sheet after an upgrade, run the new Setup; an update from inside the app does not change what Windows has registered. Windows keeps a cache of icons, so Explorer can show the old icon for a while; signing out and back in, or restarting Explorer, refreshes it.

### Portable copy: use its Text Editor in Windows
Without installing, the portable program can offer its own Text Editor to Windows the way Setup does. In the portable copy open **App settings → Editor → Use this copy's Text Editor in Windows** and choose **Register**. Tick **Right-click menu: Edit with Network Toolkit Text Editor** first if you want that entry. The portable copy then appears under **Open with** for every file and in **Settings → Apps → Default apps** for the types listed above, exactly as the installed program would, and you choose the default in Windows as described above. Opening a file this way works like the installed program: the file goes to a Text Editor window that is already open, as the selected tab.

- **Only your account is changed** (no administrator approval), and only when you click. Nothing is written when the app starts. Windows never makes it the default by itself, no other program's registration is changed, and the default value of a file type is never touched.
- **Remove** takes back exactly what **Register** added: the copy keeps a record of each key it created and removes a key only when nothing else is left in it. **Register again** repairs the registration or changes the right-click choice.
- **The program file must be named `Network_Toolkit.exe`**, because Windows lists the editor under that name.
- **The copy must be in a folder only you can change.** Once registered, Windows runs this program for every text file you double-click, so a copy that other users can change (for example in a shared folder or on a drive where everyone may write) is refused, and the section says who can change it. Move the folder under your user folder, such as Documents or `%LOCALAPPDATA%`, and register from there. A folder whose path contains a `%` sign is refused too, because Windows would read it as part of the command; Setup refuses such a program folder as well.
- **One copy at a time.** If another portable copy is registered, the section names its folder; **Use this copy instead…** replaces it only after you confirm, and the other copy then shows that it is no longer registered.
- **The installed program comes first.** While Network Toolkit is installed with Setup, a portable copy shows that the installed Text Editor is registered and offers nothing that would change that. If you run Setup after registering a portable copy, Setup takes the registration over for the installed program (your right-click choice is carried over as the preselected box in Setup), and the portable copy shows that it is no longer registered. Uninstalling the installed program then removes the registration completely; register the portable copy again if you want it back.
- **Before you move or delete the portable folder, choose Remove.** Otherwise Windows keeps pointing at the missing program and says it cannot find the app when you open a file. Another portable copy shows **Clean up** for such a registration, and offers it once when it starts. Clean up removes only the registration it showed you, and only while that program is still missing. If the program was on a drive or network share that is not connected right now, nothing is offered: the section says the drive is not connected, so connect it, or remove the registration from that copy.

The Text Editor app, the Terminal app and the Linux edition do not show this section; the main window of the portable copy does.

## Reopen files from the last session
Like Notepad and Notepad++, the Text Editor reopens the tabs that were open when it last closed, in the same order, with the same tab selected. Unsaved changes come back too, still marked as unsaved. The main window reopens its tabs the first time the Text Editor opens there; the Text Editor app reopens its tabs when it starts. Each of the two also reopens what the other one had kept, if that other app is not running at the time: tabs you left open in the Text Editor app come back in the main window, and the other way round. They are moved, not copied, so nothing appears twice. If both are running, each keeps its own tabs. A file that was open in both with unsaved changes comes back twice, once as the file and once as a copy that names it, so neither version is lost. Detached editor windows do not keep a session.

A file you open from outside, for example by double-clicking it in Windows Explorer when the Text Editor is your default app, always becomes the selected tab. The tabs from the last session still reopen beside it, in their order and with their unsaved changes. If the file is already open in a tab, that tab is selected instead of opening it twice. The tab that was selected last time is selected only when you start the editor without a file.

- Files on your computer reopen from disk. A file with unsaved changes shows your unsaved version. If the file changed on disk since then, the editor says so, and **Save** refuses to replace the newer file; use **Save As** to keep your version. If the file was deleted, your text opens in a new untitled tab.
- New, unsaved documents reopen with their text. Blank, untouched documents are not kept.
- Unsaved text from an SFTP, FTP, SSH terminal or browser file opens as a new untitled tab that names where it came from. Reconnect and use **Save As** to save it there. Remote files without unsaved changes are not reopened, and connection details and passwords are never stored.

While this is on, closing the app or the Text Editor does not ask about unsaved changes that were kept. It still asks about any document whose text could not be kept. Closing a single tab, or **Close editor**, asks as before.

To turn this off, open **More** and clear **Reopen files and unsaved changes from the last session**. The setting is shared with the main window and the Text Editor app. Turning it off removes the kept copies of the documents that are open (their text stays in the open tabs, and closing asks about it as usual). Unsaved text that an app kept earlier is never left hidden: if the setting is off when that app starts, its unsaved documents are reopened once so you can save them. Unsaved text is kept in a `Network_Toolkit.editor-session` folder in this app's data folder until it is saved or its tab is closed; large files are copied without a size limit, so the folder needs room for them. Moving the data folder moves the session too, and deleting stored app data removes it.

## Compare
Choose two documents in the comparison view. The change map shows individual changed lines on the left and right tracks. Use the wheel or the map's scrollbar to browse it independently, then click a mark to jump to that line. Sync scroll keeps the comparison panes together as you navigate.

Choose **Show only differences** in **Options** to scroll through all additions, removals, and modifications in one view. Original line numbers and gray alignment spacers stay visible. Use **Previous** and **Next** to jump to a change. **Exact** counts every space and tab. **Ignore changed spaces** matches `set  status` with `set status`. **Ignore all spaces** also matches `set status` with `setstatus`. These settings affect the comparison and leave the text unchanged.

**Dynamic fit** shows the entire document with one-pixel marks and is the default. **Compact** shows up to three lines per map pixel. The editor remembers the last map zoom you choose. Right-click either pane and open **Compare** for difference navigation, map visibility, whitespace, scroll sync, and highlighting controls. Clicking or moving the caret to a line highlights its counterpart when one exists.

When both comparison documents use disk-backed editing, gray, unnumbered rows fill the space opposite insertions and deletions. The panes scroll on the same row positions when **Sync scroll** is enabled. Copying, editing, and saving operate on the original text.

**Highlight changed characters** emphasizes the edited parts of paired lines. A replacement can highlight a whole value when only a few letters happen to match. A line marked **Added** or **Removed** uses a whole-line color.

Select text, then open **More** or right-click and choose **Convert text** to convert between UTF-8 text and hex or Base64. The result replaces the selection and can be undone. Each conversion accepts a selection of about one million characters.

Detached Editor windows have a minimum width of 640 pixels.

### Separate editor windows
**Move to separate window** (a tab's More actions menu, or dragging a tab out) opens the documents in their own window. When **Network_Toolkit_TextEditor.exe** of the same version is beside `Network_Toolkit.exe` (setup and the portable folder include it), that window runs as the Text Editor program: it has the Text Editor icon and groups with **Network Toolkit Text Editor** on the taskbar. It stays connected to the Toolkit window it came from: tabs, unsaved changes and disk-backed documents move without being reopened, nothing is saved or copied on the way, and **Move / merge** with **Main workspace** brings them back. Without that program the window opens as before.

**Send to terminal** in such a window lists the connected sessions of the main window and of every separate Terminal window of the same Toolkit, grouped by window. The window that shows the chosen session enters the text exactly as a send inside one window does (paste review, paste warnings, no Enter added); if a dialog is open there, nothing is sent and you are told why. In the main window the same dialog also lists the sessions of its separate Terminal windows.

Starting with v1.21.20 Beta 1, large text files use temporary disk storage and display only the section you are working on. Open, edit, save and compare files without a fixed file-size or line-count limit. Opening, searching, comparing and saving still take time to read or write the data, and large files need space for temporary working copies.

Large comparisons run in the background and show changes as they are found. Use **Next** and **Previous** to move through differences. Use **Wrap** to fit long lines to the window. The first row shows the source line number and continuation rows leave the gutter blank. Compare aligns wrapped lines that fit in a loaded page when both files use the same editor mode. Very long lines can continue across pages, and their sections remain accessible. Comparisons highlight changed lines and may group distant rearrangements into one change. Select the commands you intend to send when using **Send to terminal** from a large file.

## Files and Connect sidebar
The sidebar's Connect and Files buttons open saved connections and the file browser. Drag its edge to resize it, or double-click the edge to hide it. The ⇄ button at the bottom of the sidebar's icon strip moves it to the other side of the editor; the side, width and open panel are kept when you restart.

## Tabs and splitters
Open a new document with the **+** at the end of the tab strip or Ctrl+N. When the tabs do not fit, scroll the tab row with the mouse wheel or the arrow buttons at its right end. Right-click editor text or press Shift+F10 for common editing and navigation commands. **Edit text** groups comment, duplicate, case, and format commands. **Convert text** groups hex and Base64 commands, and **Compare** groups comparison commands. Hover or click to open a submenu. With the keyboard, use Right Arrow to open it and Left Arrow to return to the main menu. To run text on a device, choose **Send to terminal** (the terminal-with-arrow button on the toolbar, the right-click menu, or Ctrl+Shift+Enter): pick the selection or the whole document and a connected Terminal session; the Terminal opens on that session and the text is entered like a paste, with your paste review settings. Each tab has a visible **More actions** menu with Move left, Move right and Move to separate window. Existing Alt+drag and Alt+Shift+Arrow shortcuts remain supported, as does tab dragging. Dividers can be resized and may support double-click reset or collapse.

## The Text Editor app
With the Windows setup installed, **Network Toolkit Text Editor** in the Start menu opens the editor in its own window, without the rest of the Toolkit. It is its own program, **Network_Toolkit_TextEditor.exe** (also in the portable folder), with its own taskbar button and name in Task Manager; a file opened from Explorer or `Network_Toolkit.exe --editor-standalone` opens in the same window. It contains what belongs to the editor: documents and tabs, compare and split view, the Files and Connect sidebar (files on your computer and on SFTP, FTP and SSH connections), text colors and **Send to terminal**. Diagnostics, the Toolkit's App settings and update controls stay in the main Toolkit window. **Open terminal** on a connected SFTP session is offered only in the main Toolkit window, because a signed-in connection cannot be handed to another app.

The three small buttons at the right end of the status bar switch the color theme, open **Window settings** (theme and accent color) and open help (F1). Command search (Ctrl+Shift+P) and the F1 help list only editor commands.

### Send to the Terminal app
In the Text Editor app, **Send to terminal** (toolbar button, right-click menu or Ctrl+Shift+Enter) lists the connected sessions of **Network Toolkit Terminal**, the separate Terminal app. Pick the selection or the whole document and a session, then choose **Send to terminal**.

You are never asked to open anything first. The Text Editor app looks for a terminal in this order:

1. **Network Toolkit Terminal**, if it is open.
2. Otherwise the main **Network Toolkit** window, if it is open: its Terminal sessions are listed, and the text goes to its Terminal page, which comes to the front. A window that has only just opened gets up to about 3 seconds to get ready before it is passed over.
3. Otherwise Network Toolkit Terminal is started for you. A newly started Terminal has no session yet, and nothing is connected for you: connect one there, then choose **Send to terminal** again. If the Terminal does not come up within about 25 seconds, the Text Editor says so and your document is unchanged.

- The Terminal comes forward on that session and the text is entered like a paste, with the Terminal's own paste settings. Text with line breaks is shown there for review first when **Review multi-line paste** is on, and paste warnings apply. The Text Editor then tells you where the text went and whether it was sent or is waiting for your review.
- If the terminal that was found has no connected session, the dialog says so. For the Terminal app, **Show Terminal** brings it to the front. Connect a session there, then send again.
- An app that is still open from an older version cannot receive text. It is skipped, and the Text Editor names it so you can close it and open it again.
- Up to 1 MiB of text can be sent at a time. Your document stays open and unchanged.
- If a dialog is open in the Terminal (a connection form, a sign-in or host key question, or another paste review), the text is refused instead of replacing it. Close the dialog and send again.

Text sent from the Terminal app with **Send to Text Editor** arrives here as a new document or in the open document you chose there; see [Terminal](terminal.md#the-terminal-app). Only the text you send travels between the apps. Passwords, saved connections, host keys and file contents you did not send are never shared, and nothing leaves this computer. The apps must use the same data folder, which they do when installed with setup. In the main Toolkit window **Send to terminal** works inside that window and its separate windows (see [Separate editor windows](#separate-editor-windows)); it does not send to the Terminal app.

## Configuration text
For network configuration syntax, choose Auto, the network language or plain text. This affects highlighting only; review actual device commands independently before applying configuration.
