# Text Editor

The built-in Editor works with local and supported remote text documents. It provides multiple tabs, comparison, Find, Replace and Mark with a search results panel, bookmarks, syntax highlighting and document navigation.

## Open and edit
Open or create a document, review its source/connection, then edit. Save explicitly. Save As opens the Windows Save As dialog for files on your computer and a folder picker for files on an SFTP server; an existing file is replaced only after you confirm. Keep unsaved changes and undo history when switching views or during update workflows.

Hold **Ctrl** and roll the mouse wheel over editor text to change the font size. Wheel up enlarges the text and wheel down shrinks it. In Compare and Split View, both panes use the same size shown in the **Font size** control.

Editor preferences (word wrap, highlighting, indent, font size, colors, compare options, the document map, icon or text toolbar buttons, and whether files are reopened) and the saved connections in the Files sidebar are the same in the main window's editor and in the separate **Network Toolkit Text Editor** window. See [One set of settings and saved data for all apps](settings-and-updates.md#one-set-of-settings-and-saved-data-for-all-apps).

## Toolbar
The toolbar's buttons are icons. Point to one to see its name and shortcut. To see words instead, open **More** and set **Toolbar buttons** (under **View**) to **Text**; **Icons** switches back. The choice is kept when you restart, and it is the same in every Text Editor of the app: the main window, the Text Editor app, separate editor windows and the editor that opens from the Terminal app. The buttons do the same in both:

| Icon | Button | Shortcut |
| --- | --- | --- |
| Open folder | **Open file** | Ctrl+O |
| Floppy disk (highlighted) | **Save** | Ctrl+S |
| Floppy disk with a pencil | **Save as** | Ctrl+Shift+S |
| Two circling arrows | **Reload** | |
| Two columns | **Compare** | |
| Box with an arrow out | **Detach** (**Open separately** when no document is open) | |
| Two windows with arrows between them | **Move / merge** | |
| Window split in two | **Split** | |
| Curved arrows | **Undo**, **Redo** | Ctrl+Z, Ctrl+Y |
| Magnifier | **Find** | Ctrl+F |
| Magnifier over an arrow | **Replace** | Ctrl+H |
| Arrow to a line | **Go to line** | Ctrl+G |
| Terminal with an arrow | **Send to terminal** | Ctrl+Shift+Enter |
| Wrench | **Tools** (the [text tools](#text-tools)) | |
| Four corners | **Full editor** (only in the editor that opens over another tool; it opens the Text Editor page) | |
| × | **Close editor** | |

**Find**, **Replace**, **Compare** and **Split** stay highlighted while what they opened is shown; press them again to close it. While a move to another window is pending, or a move needs your attention, **Move / merge** also says so in words beside its icon. **More** keeps its name, so the setting is always easy to find.

The toolbar has no **Files** button: the icon strip at the edge of the sidebar opens **Docs**, **Connect** and **Files** (see [Files and Connect sidebar](#files-and-connect-sidebar)).

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

A copy installed long ago and since updated from inside the app may offer the editor for only some of these types (Setup up to 1.21.43 registered five). **App settings → Updates** then offers to run this version's Setup once; see [File types after an update](settings-and-updates.md#updates).

Documents, images, media, archives and programs are not in the list, because the editor is not the right default for them; they still open through **Open with**. The file you open from Windows becomes the selected tab, as described next.

**File icons:** a file that opens in the Text Editor shows a plain sheet of paper: the page from the editor's icon, without the blue tile behind it. That holds however you chose the editor: in Default apps, in **Open with** (also for a type outside the list above), or by picking `Network_Toolkit_TextEditor.exe` itself with **Choose an app on your PC**. In that last case Windows makes its own entry for that program; Setup and **Use this copy's Text Editor in Windows** give it the sheet as well, never change anything else in it, and take back only that icon when you uninstall or remove the registration. An icon you set there yourself is left alone. The editor itself keeps its full icon in the Start menu, on the taskbar, in Default apps and on the right-click entry. After an update from inside the app, the first start gives the files the sheet; for Windows' own `Network_Toolkit_TextEditor.exe` entry of an installed copy whose Setup is older than 1.21.52, run the new Setup once. Windows keeps a cache of icons, so Explorer can show the old icon for a while; signing out and back in, or restarting Explorer, refreshes it.

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

To convert between text and hex, Base64 and other encodings, use the [text tools](#text-tools): they work on the selection or the whole document, of any size, and can be undone.

Detached Editor windows have a minimum width of 640 pixels.

### Separate editor windows
**Move to separate window** (a tab's More actions menu, or dragging a tab out) opens the documents in their own window. When **Network_Toolkit_TextEditor.exe** of the same version is beside `Network_Toolkit.exe` (setup and the portable folder include it), that window runs as the Text Editor program: it has the Text Editor icon and groups with **Network Toolkit Text Editor** on the taskbar. It stays connected to the Toolkit window it came from: tabs, unsaved changes and disk-backed documents move without being reopened, nothing is saved or copied on the way, and **Move / merge** with **Main workspace** brings them back. Without that program the window opens as before.

**Send to terminal** in such a window lists the connected sessions of the main window and of every separate Terminal window of the same Toolkit, grouped by window. The window that shows the chosen session enters the text exactly as a send inside one window does (paste review, paste warnings, no Enter added); if a dialog is open there, nothing is sent and you are told why. In the main window the same dialog also lists the sessions of its separate Terminal windows. The window that receives the text comes to the front (Windows may instead flash its taskbar button if another program is in use).

A window popped out of the **Text Editor app** works like the Text Editor app itself: its **Send to terminal** reaches Network Toolkit Terminal (or the main window) exactly as described in [Send to the Terminal app](#send-to-the-terminal-app). Text sent from the Terminal app to a document that such a window holds arrives in that window.

Starting with v1.21.20 Beta 1, large text files use temporary disk storage and display only the section you are working on. Open, edit, save and compare files without a fixed file-size or line-count limit. Opening, searching, comparing and saving still take time to read or write the data, and large files need space for temporary working copies.

Large comparisons run in the background and show changes as they are found. Use **Next** and **Previous** to move through differences. Use **Wrap** to fit long lines to the window. The first row shows the source line number and continuation rows leave the gutter blank. Compare aligns wrapped lines that fit in a loaded page when both files use the same editor mode. Very long lines can continue across pages, and their sections remain accessible. Comparisons highlight changed lines and may group distant rearrangements into one change. Select the commands you intend to send when using **Send to terminal** from a large file.

## Text tools
The **Tools** button on the toolbar (next to **Send to terminal**) opens the text tools in three groups: **Lines**, **Encode / decode** and **Format**. The same groups are in the right-click menu, and **Search tools and commands** (Ctrl+Shift+P) finds every tool by name. Each tool shows a short description with a tiny example, also as its tooltip, and its shortcut if it has one. They are general tools for any text, not for one kind of device or file.

- A tool works on the **selection**, or on the **whole document** when nothing is selected. The status bar says which: for example *Decoded the whole document from Base64* or *Sorted the selected lines A→Z (12 lines)*. Tools that work on lines take in the whole of every line the selection touches. After a tool runs, its result is selected.
- Every tool is **one undo step**: Ctrl+Z takes it back.
- A tool that would change nothing says so and changes nothing. A tool that cannot run (Base64 that is not valid, JSON or XML with an error, decoded bytes that are not UTF-8 text) changes nothing and says why, with the line and column of the problem.
- **Decode the part before it.** When **Base64 decode** or **Hex to text** meets a character that is not part of the encoding (for example the `&&` after the Base64 in `aGVsbG8=&&`), it stops and changes nothing, and the message names the character with its line and column. If the text before that character decodes on its own, a bar above the text offers **Decode the part before it**: it decodes from the start of the selection (or of the document) up to that character, as one undo step, and leaves that character and everything after it exactly as they were. The decoders never skip a character on their own. The offer goes away when you edit the document, switch tabs or run another tool.
- **Any size.** The tools run in the Network Toolkit engine, one implementation for every document. On a large document they run over its disk storage, show their progress above the text with **Cancel**, and the rest of the app stays responsive; sorting and removing duplicates spill to temporary files instead of holding the document in memory. Only a single line is ever held whole while lines are compared.
- Text that is not UTF-8 (Windows-1252), UTF-16 text and files whose lines end in different ways keep their format: a tool changes the text you see, lines it does not change keep their own endings, and saving still stops if the result has a character the file's encoding cannot hold. A file that is not text cannot be changed by any tool.
- The tools are in every Text Editor: the main window's, separate editor windows and the Text Editor app. The editor inside the Terminal app (for files of its SFTP sidebar) keeps the editing actions it always had in its **Tools** and right-click menus: Toggle comment, Duplicate lines, Sort A→Z, Trim trailing spaces, Base64 and hex (with **Decode the part before it**), Format document, UPPERCASE and lowercase.

| Group | Tool | What it does | Shortcut |
|---|---|---|---|
| Lines | Toggle comment | Adds or removes the comment mark of the document's language on the selected lines: `x = 1` → `# x = 1`. | Ctrl+/ |
| Lines | Duplicate lines | Copies the current line, or the selected lines, right below them: `a` → `a`, `a`. | Ctrl+D |
| Lines | Split at text… | Starts a new line at every place the text appears and removes it: `a&b&c` → three lines `a`, `b`, `c`. Tick **Keep the separator at the end of each line** to keep it. The text is matched exactly; it is not a regular expression. | Ctrl+Shift+L |
| Lines | Join lines… | Joins the selected lines into one, with the text you choose between them (a space unless you change it; it may be empty): lines `a`, `b`, `c` → `a b c`. | Ctrl+J |
| Lines | Sort A→Z / Sort Z→A | Sorts lines ignoring letter case, with numbers in number order: `b`, `a10`, `a2` → `a2`, `a10`, `b`. Other characters sort by their code, and equal lines keep their order. | |
| Lines | Remove duplicate lines | Removes lines that repeat an earlier line exactly; the first one stays where it is: `a`, `b`, `a` → `a`, `b`. | |
| Lines | Remove empty lines | Removes lines that are empty or hold only spaces and tabs: `a`, (empty), `b` → `a`, `b`. | |
| Lines | Trim trailing spaces | Removes spaces and tabs at the end of each line: `"a  "` → `"a"`. | |
| Lines | Trim leading and trailing spaces | Removes spaces and tabs at both ends of each line: `"  a  "` → `"a"`. | |
| Lines | Reverse line order | Puts the lines in the opposite order: `a`, `b`, `c` → `c`, `b`, `a`. | |
| Encode / decode | Base64 decode | `aGVsbG8=` → `hello`. Standard or URL-safe Base64, with or without padding; spaces and line breaks in it are ignored. Any other character stops it (see **Decode the part before it** above). | Ctrl+Shift+B |
| Encode / decode | Base64 encode | Writes the text (UTF-8) as Base64 on one line: `hello` → `aGVsbG8=`. | |
| Encode / decode | URL decode | `%20` → space, `%3A` → `:`; a `+` stays a plus sign. | |
| Encode / decode | URL decode form data | Like URL decode, but a `+` is a space, as in submitted web forms: `a+b%21` → `a b!`. | |
| Encode / decode | URL encode | Writes every character except letters, digits and `- _ . ~` as `%XX`: `a b` → `a%20b`. | |
| Encode / decode | Hex to text / Text to hex | `68 69` → `hi` (spaces, `:`, `-`, `,` and `0x` between the digit pairs are ignored; any other character stops it, as in Base64 decode), or the UTF-8 bytes of the text as hex: `hi` → `6869`. | |
| Encode / decode | HTML entities decode / encode | `&lt;b&gt;` → `<b>`, `&amp;` → `&`, `&#233;` → `é`; or `&`, `<`, `>`, `"` and `'` written as entities: `<b>` → `&lt;b&gt;`. | |
| Encode / decode | Unescape / Escape `\n \t \" \\` | `\n` → line break, `\t` → tab, `\"` → `"`, `\\` → `\`, `\x41` → `A` (also `\r` and `\uNNNN`); or line breaks, tabs, double quotes and backslashes written as escapes (other control characters as `\xNN`). | |
| Format | Format document | Lays out the document by the rules of its language, such as JSON, XML, HTML, CSS or JavaScript (**Indent document** for languages whose indentation only is adjusted). | Ctrl+Shift+F |
| Format | Pretty-print JSON / Minify JSON | `{"a":1}` → three lines, indented with the editor's **Indent** setting; or all white space removed: `{ "a": 1 }` → `{"a":1}`. Strings and numbers are copied exactly. | |
| Format | Pretty-print XML / Minify XML | `<a><b/></a>` → three indented lines (an element that holds only text stays on one line); or the white space between tags removed. Tags, text and comments are copied exactly. | |
| Format | Tabs to spaces / Leading spaces to tabs | With an **Indent** of 4: a tab → spaces up to the next tab stop, or 8 spaces at the start of a line → 2 tabs (4 when Indent is Tabs). | |
| Format | UPPERCASE / lowercase / Title Case | `Hello` → `HELLO` or `hello`; Title Case starts every word with a capital letter: `hello wORLD` → `Hello World`. | Ctrl+Shift+U / Ctrl+U |

### Make a one-line encoded settings file readable
Some programs save their settings as one long line of Base64 text, sometimes with a few extra characters after it, such as `&&`. Decoded, such a file is often one line of `name=value` pairs joined by `&`, with special characters written as `%XX` (`%20` is a space). Three general tools make it readable, on a file of any size:

1. **Base64 decode** (Ctrl+Shift+B), with nothing selected (press Ctrl+Home first). If the Base64 is followed by characters that are not Base64, such as `&&`, the tool stops and changes nothing; choose **Decode the part before it** in the bar above the text. The Base64 becomes `name=value&name=value&…`, the extra characters stay as they were, and the decoded part stays selected.
2. **Split at text…** (Ctrl+Shift+L) with `&`, which the box suggests: every `&` ends a line, so each `name=value` pair is on its own line.
3. **URL decode**: `%20` becomes a space, `%3A` a colon, `%2F` a slash, and so on; a `+` stays a plus sign (use **URL decode form data** where a `+` stands for a space).

For example, `bmFtZT1PZmZpY2UlMjAxJmFkZHJlc3M9MTkyLjAuMi4xMCY=&&` becomes `name=Office%201&address=192.0.2.10&&&` after step 1, three lines after step 2 (`name=Office%201`, `address=192.0.2.10` and the `&&` that was after the Base64), and `name=Office 1` on the first line after step 3. Steps 2 and 3 work on the text the step before left selected; with nothing selected they work on the whole document. Each step is one undo step, and the file on disk changes only when you save it: choose **Save as** to keep the original. **Remove empty lines** and **Sort A→Z** can tidy the result.

## Find, Replace and Mark
**Find** and **Replace** on the toolbar, Ctrl+F, Ctrl+H and Ctrl+M open the **Find window** on its **Find**, **Replace** or **Mark** tab, as in Notepad++. They are toggles: the button or shortcut of the tab that is showing closes the window again (the button looks pressed while its tab shows), and the other one switches the tab. Escape, or the × of the window, closes it too. The window floats over the editor: drag its tab row to move it; it opens again where you left it while the editor is open. Text selected in the document (one line) becomes **Find what**. The window works the same in the main window's editor, in separate editor windows (it fits a window of 640 × 400) and in the Text Editor app; the editor that opens SFTP files in the Terminal app has it too.

**Find what** and **Replace with** keep the last 20 searches and replacements: open the list with the arrow at the end of the field. The list is shared by the editor windows of one app and lasts until the app closes. It is never saved: search text can contain passwords.

### Options
- **Backward direction**: **Find Next** goes up instead of down (Find and Replace tabs).
- **Match whole word only** and **Match case**.
- **Wrap around** (on by default): at the end of the document, or of the selection with **In selection**, the search goes on from the other end, and the status line says so. Off, **Find Next** stops at the end.
- **In selection** (when text is selected): **Count**, **Find Next**, **Find All in Current Document**, **Replace**, **Replace All** and **Mark All** look only for matches that lie entirely inside the selected range (text just outside it still counts for `^`, `$` and whole words), also in a large (disk-backed) document. The range is kept while **Find Next** selects the matches inside it.
- **Search Mode** (the **?** beside it opens the syntax guide below):
  - **Normal** finds the text exactly as typed.
  - **Extended (\n, \r, \t, \0, \x…)** works like Normal, but `\n`, `\r`, `\t`, `\\`, `\xNN` and `\uNNNN` in **Find what** and **Replace with** stand for a line break, a tab and other characters (Notepad++'s Extended mode). Find `&`, replace with `\n` and **Replace All** starts a new line at every `&`. The editor holds every line break as one `\n`, so `\r` and `\r\n` mean a line break too; `\0` is refused, because no text in the editor can hold a NUL character.
  - **Regular expression**: **Find what** is a regular expression; `^` and `$` match at the start and end of each line. **Replace with** may use `$1` to `$99`, `${name}`, `$&` (or `$0`) for the text of groups, `$$` for a dollar sign, and `\n`, `\t` for a line break and a tab. A group that does not exist stays as written.
  - **. matches newline** (Regular expression only): `.` also matches a line break, so a pattern can span lines.

  The same rules apply to every document, whatever its size: every search runs in the app's engine. A regular expression does **not** support lookahead and lookbehind (`(?=`, `(?!`, `(?<=`, `(?<!`), backreferences such as `\1` in **Find what** (use `$1` in **Replace with**), `\uNNNN` (use `\x{NNNN}`, or Extended mode), possessive repeats such as `++` and atomic groups `(?>…)`; the search says so and where.
- **Transparency**: makes the window see-through **On losing focus** (while you work in the document) or **Always**; the slider sets how opaque it stays. This setting is kept when you restart; the other options last while the editor is open.

### Syntax guide
The **?** beside **Search Mode** opens a short guide to the current mode, inside the window; click **?** again to close it. For **Extended** it lists `\n`, `\r`, `\t`, `\\`, `\xNN` and `\uNNNN`; for **Regular expression** it lists `.`, `\d \w \s` and their capitals, `\b`, `^ $`, `* + ?` (and `?` after a repeat for as few as possible), `{n} {n,m}`, `[abc] [^abc] [a-z]`, `( )`, `(?: )`, `(?<name> )`, `|`, `\` and `(?i)`, and on the **Replace** tab also `$1`, `${name}`, `$&` / `$0`, `$$`, `\n` and `\t`. Each row says what the token means, with a small example; click a row to insert the token at the cursor of the field you last used (**Find what** or **Replace with**).

In Extended and Regular expression mode one quiet line under **Find what** explains the part at the cursor in plain words (for example `\d — a digit 0–9`). If the pattern cannot be used, the line names the problem and its position instead, such as `Problem at character 1: A ( has no closing ).`. Normal mode shows no line.

### Find
**Find Next** (F3; Shift+F3 finds the previous match) selects the next match; the match stays highlighted while you work in the window. In **Normal** mode the search runs as you type: after a short pause in typing **Find what**, the first match from the cursor is selected. A pattern that can match empty text, such as `^$` for an empty line, moves on to the next one at each **Find Next**.

While the Find window has the focus, the editor's shortcuts (Ctrl+S, Ctrl+N, Ctrl+O, Ctrl+G and the others) and the bookmark keys still work; the fields keep their own editing keys (Ctrl+A, Ctrl+C, Ctrl+V, Ctrl+X, Ctrl+Z, Ctrl+Y). **Count** shows the number of matches in the window's status line. **Find All in Current Document** and **Find All in All Opened Documents** list the matches in the **Search results** panel.

### Replace
**Replace** replaces the selected match and selects the next one. If you change the selection or the text while it works, it replaces nothing and finds the next match from where you are. **Replace All** replaces every match of the document (or of the selection) as one step that one Undo takes back. **Replace All in All Opened Documents** does the same in every open document that can change: a file that is not text, a document being saved, moved to another window, still opening or shown in a comparison is skipped and named. Nothing is saved: the changed documents show as unsaved, and Undo in each document takes its replacements back. The ⇅ button swaps **Find what** and **Replace with**; its menu also copies one to the other.

### Mark
**Mark All** highlights every match in the document with the mark colour of the editor colours (it follows the light and dark themes and your custom colours), and with **Bookmark line** also bookmarks each line with a match. **Purge for each search** clears the earlier marks (and, with **Bookmark line**, the earlier bookmarks) first; without it, marks of several searches add up. **Clear all marks** removes the marks, and **Copy Marked Text** copies the text of every mark in the order of the document, one per line; text that two marks share is copied once. Marks follow edits: they are the matches of the search wherever the text now is, and marks made **In selection** keep to the text that was selected. In a large document only the part on screen is searched for its marks.

### Bookmarks
Ctrl+F2 bookmarks the line of the cursor (again to remove it); a dot in the line-number gutter shows it. F2 and Shift+F2 go to the next and previous bookmark. Lines you add or remove above a bookmark move it with its line, and a change that keeps the number of lines (**Replace All** of a word, for example, or its Undo) leaves every bookmark where it is. In a large document, a change whose place is not known (Undo, Redo or a text tool) keeps bookmarks on their line numbers, and on the last line if the document becomes shorter. Marks and bookmarks last while their document is open in that window; they are never saved.

### Search results
The **Search results** panel is docked at the bottom of the editor; **Find All** opens it. The **Results** button in the status bar (with the number of hits) shows and hides it, as does **Show or hide search results** in the command palette. Drag its top edge to resize it (arrow keys on the edge resize it too; double-click resets it). Each **Find All** adds a block at the top: `Search "text" (N hits in M files of K searched)` with the search mode, then each document and its `Line N:` rows, with the matches highlighted. Very long lines are cut down around their matches (… marks the cut). Click ▸ / ▾ (or the Left and Right arrow keys) to collapse or expand a search or a document. Double-click a row, or select it and press Enter, to go to that match and select it. If the document was closed or has changed since the search, or the search's results are no longer available, the row says so instead of going to the wrong place: search again. Right-click (or Shift+F10) for **Copy** (the selected rows as shown), **Copy selected lines** (only the lines' text), **Collapse all**, **Expand all** and **Clear all**, which also closes the panel. Shift+Arrow keys or Shift+click select several rows. Earlier searches stay until you clear them.

A search with millions of matches works in any document: the app keeps the rows and the panel reads and draws only the rows on screen. Count, Find All, Mark All, Copy Marked Text and Replace All of a large document run in the background with their progress in the window's status line, where **Cancel** stops them. A file that is not text can be searched, counted and marked, but nothing in it is replaced.

## Files and Connect sidebar
The sidebar's icon strip has three buttons, in this order: **Docs** (the [Document List](#document-map-and-document-list)), **Connect** (saved and open connections) and **Files** (the file browser). A button opens its panel, and the button of the panel that is open hides the sidebar; with the keyboard, Up and Down (or Home and End) move between them and Enter or Space hides or shows the sidebar. **Search tools and commands** (Ctrl+Shift+P) has *Show the files sidebar*, *Show the connections sidebar* and *Show or hide the editor sidebar*. Drag the sidebar's edge to resize it, or double-click the edge to hide it. The ⇄ button at the bottom of the sidebar's icon strip moves it to the other side of the editor; the side, width and open panel are kept when you restart.

## Document Map and Document List
Like Notepad++, the Text Editor can show a **Document Map** and a **Document List**. Turn them on in **More** under **View**, or with **Search tools and commands** (Ctrl+Shift+P): *Show or hide the document map* and *Show the document list*. Both are kept when you restart, and they are the same in the main window, the Text Editor app and its separate windows. The editor that opens from the Terminal app has them too, in its **More** menu and sidebar.

The **Document Map** is a narrow strip at the right edge of the document that shows the whole document scaled down, with a light box over the part on screen. It is off until you turn it on.

- Click in the map to show that part of the document. Drag the box to scroll, or roll the mouse wheel over the map.
- The map follows typing, **Wrap**, the font size, the light and dark themes and the editor colors. In split view it shows the document of the pane you are working in. While comparing it is hidden, because the comparison has its own change map.
- The **Search results** panel opens below the text and the map, and the map grows back when you hide the panel. Marks and the match **Find Next** selected show in the text, not in the map.
- A short document is drawn word by word in its syntax colors. A longer one is drawn as thin lines, and a very long one with one line for each row of the map, so the map stays quick for a document of any size.
- A large file that the editor reads in sections is mapped by position, like its scroll bar: by line, or by character with **Wrap** on, so a file that is one long line (such as a Base64 export) is mapped from its first character to its last. The part you have read is drawn in detail; the rest is a light texture until you scroll there.

The **Document List** is the **Docs** button at the top of the sidebar's icon strip, above Connect and Files. It lists the open documents in tab order: an unsaved marker (●), the name, and the extension in a column of its own, with the selected document highlighted. In split view the documents of the left and right views are listed under their own headings.

- Click a document, or move with Up and Down and press Enter, to switch to it. Escape returns to the text.
- Right-click a document, or press Shift+F10, for the same menu as its tab: save, compare, move, copy the path or close.
- The list shares the sidebar's width, side and open or closed state with Files. In a narrow window the sidebar covers the text, and it closes when you choose a document.

## Tabs and splitters
Open a new document with the **+** at the end of the tab strip or Ctrl+N. When the tabs do not fit, scroll the tab row with the mouse wheel or the arrow buttons at its right end. Right-click editor text or press Shift+F10 for common editing and navigation commands. The [text tools](#text-tools) are grouped as **Lines**, **Encode / decode** and **Format**, the same groups as the toolbar's **Tools** menu, and **Compare** groups comparison commands. **More** on the toolbar keeps **Save copy**, **Text colors**, the **View** options (**Document map**, **Document list**, **Toolbar buttons**: icons or text) and the close and reopen preferences. Hover or click to open a submenu. With the keyboard, use Right Arrow to open it and Left Arrow to return to the main menu. To run text on a device, choose **Send to terminal** (the terminal-with-arrow button on the toolbar, the right-click menu, or Ctrl+Shift+Enter): pick the selection or the whole document and a connected Terminal session; the Terminal opens on that session and the text is entered like a paste, with your paste review settings. Each tab has a visible **More actions** menu with Move left, Move right and Move to separate window. Existing Alt+drag and Alt+Shift+Arrow shortcuts remain supported, as does tab dragging. Dividers can be resized and may support double-click reset or collapse.

## The Text Editor app
With the Windows setup installed, **Network Toolkit Text Editor** in the Start menu opens the editor in its own window, without the rest of the Toolkit. It is its own program, **Network_Toolkit_TextEditor.exe** (also in the portable folder), with its own taskbar button and name in Task Manager; a file opened from Explorer or `Network_Toolkit.exe --editor-standalone` opens in the same window. It contains what belongs to the editor: documents and tabs, compare and split view, the Files and Connect sidebar (files on your computer and on SFTP, FTP and SSH connections), text colors and **Send to terminal**. Diagnostics, the Toolkit's App settings and update controls stay in the main Toolkit window. **Open terminal** on a connected SFTP session is offered only in the main Toolkit window, because a signed-in connection cannot be handed to another app.

The three small buttons at the right end of the status bar switch the color theme, open **Window settings** (theme and accent color) and open help (F1). Command search (Ctrl+Shift+P) and the F1 help list only editor commands.

### Send to the Terminal app
In the Text Editor app, **Send to terminal** (toolbar button, right-click menu or Ctrl+Shift+Enter) lists the connected sessions of **Network Toolkit Terminal**, the separate Terminal app. Pick the selection or the whole document and a session, then choose **Send to terminal**.

You are never asked to open anything first. The Text Editor app looks for a terminal in this order:

1. **Network Toolkit Terminal**, if it is open. Sessions in its popped-out Terminal windows are listed too, and the text goes to the window that shows the session, which comes to the front.
2. Otherwise the main **Network Toolkit** window, if it is open: its Terminal sessions are listed, and the text goes to its Terminal page, which comes to the front. A window that has only just opened gets up to about 3 seconds to get ready before it is passed over. Sessions in its separate Terminal windows are listed too, also while the main window itself is closed; the text then goes to that Terminal window.
3. Otherwise Network Toolkit Terminal is started for you. A newly started Terminal has no session yet, and nothing is connected for you: connect one there, then choose **Send to terminal** again. If the Terminal does not come up within about 25 seconds, the Text Editor says so and your document is unchanged.

- The Terminal comes forward on that session and the text is entered like a paste, with the Terminal's own paste settings. Text with line breaks is shown there for review first when **Review multi-line paste** is on, and paste warnings apply. The Text Editor then tells you where the text went and whether it was sent or is waiting for your review.
- If the terminal that was found has no connected session, the dialog says so. For the Terminal app, **Show Terminal** brings it to the front. Connect a session there, then send again.
- An app that is still open from an older version cannot receive text. It is skipped, and the Text Editor names it so you can close it and open it again.
- Up to 1 MiB of text can be sent at a time. Your document stays open and unchanged.
- If a dialog is open in the Terminal (a connection form, a sign-in or host key question, or another paste review), the text is refused instead of replacing it. Close the dialog and send again.

### Pop out into the open Text Editor app
When the Text Editor app is already open with the same data folder, popping the editor out of the main **Network Toolkit** window with **Detach editor** or **Move all / detach editor** (destination **New editor window**) adds your tabs to the open Text Editor instead of a new window, and that window comes to the front:

- Untitled documents arrive as new unsaved documents with their title and text.
- Files on this computer arrive with their unsaved changes, encoding, line endings and unsaved marker as they were. The Text Editor opens the file itself and checks that it still holds what the main window read.
- Tabs that need the main window still open in a connected window, as before: files opened over SFTP or SSH, binary files, unsaved changes in large documents, encodings or line endings that cannot be carried exactly (Windows-1252, mixed line endings), files not saved yet, files on a network share, and files that changed on disk since they were opened.

**Move to separate window** on a single tab always opens that tab in its own connected window, even while the Text Editor app is open.

The main window tells you where the tabs went, for example "3 tabs moved to the open Network Toolkit Text Editor; 1 tab that uses an SFTP connection opens in a connected window." A tab leaves the main window only after the Text Editor confirms that it has it; with **Reopen files next time** on, that includes keeping its unsaved text for next time. If the Text Editor refuses (for example because a dialog is open there), does not answer within about a minute, or closes, the tabs stay in the main window unchanged and you are told why. Undo history moves with the tabs: **Undo** and **Redo** in the Text Editor continue where they were. Large documents (the ones kept on disk while you edit) move without their undo history, and so does a tab whose history would not fit in what one move carries (8 MiB together with the unsaved text); the main window's message names such tabs, and Undo there starts from the moved text. When the Text Editor app is not open, popping out works exactly as before. Terminal sessions never move into the Terminal app.

Text sent from the Terminal app with **Send to Text Editor** arrives here as a new document or in the open document you chose there; see [Terminal](terminal.md#the-terminal-app). Only the text you send travels between the apps. Passwords, saved connections, host keys and file contents you did not send are never shared, and nothing leaves this computer. The apps must use the same data folder, which they do when installed with setup. In the main Toolkit window **Send to terminal** works inside that window and its separate windows (see [Separate editor windows](#separate-editor-windows)); it does not send to the Terminal app.

## Configuration text
For network configuration syntax, choose Auto, the network language or plain text. This affects highlighting only; review actual device commands independently before applying configuration.
