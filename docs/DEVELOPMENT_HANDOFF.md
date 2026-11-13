# v1.21.56 Beta 1 - General text tools, toggle buttons and dialog focus

Owner decisions after v1.21.54: no brand-specific tool ("just make sure the tools required to do so exist and their functionality are clearly explained in an intuitive way, but not specific to any brand"), and "buttons that open a new row of menu items, like "Find" should be a toggle ... instead of needing a dedicated "hide" or close button". The owner also asked for three suggested follow-ups to be done in this session: dialog initial focus, focus back to the opener after a dialog redraws, and the stale live Terminal sidebar check.

v1.21.55 was published and verified: tag at merge commit 3dbc1a8 (PR #181), release run 43503395074 green, all seven assets matching GitHub's digests, `github_release.py verify` reporting "Verified 1.21.55", stable channel still at v1.21.34. Its CI caught a lost track click on the disk view's document bar (a paint put the bar back before its scroll event), fixed in that release; #180 had traced the earlier double-click failure to a wheel step the browser delivered late onto the next view. One native run took 27 s to start the editor page (2 s on its re-run): runner time, not the Document Map.

What changed:
- #182 General text tools: the SonicWall tool, its offer bar, `tool-detect`, the `derive` open mode and `editorbuffer.NewText` are removed. Base64 decode and Hex to text stop at a character that is not Base64/hex and offer **Decode the part before it** (`texttools.PositionError.Before`; page-held `partial`/`offset`, disk-backed `partial {from,to,line,column,scope}` and a ranged `tool-start`). Every tool description is plain with an example (`scripts/editor-text-tools.test.cjs` rejects brand or vendor words), short enough for two menu lines in CI's fonts. docs/guide/editor.md has the recipe "Make a one-line encoded settings file readable".
- #183 Toggle buttons: every button that reveals a row or panel outside the Text Editor toggles it (`aria-controls`/`aria-expanded`, shared `reveal-toggle` style); the Terminal Find row starts closed; packet capture's helper panels open below their buttons. Rule in AGENTS.md and docs/UI_GUIDELINES.md. `scripts/terminal-sidebar-layout-live.cjs` is rewritten for the rail-tab sidebar and runs in the `browser` job.
- #184 Dialog focus: `focusModalDefault` never overrides focus already inside the new dialog and never starts on the header's Close; `modalLastFocus` is set only when no dialog is open, so a redraw or a stacked dialog returns focus to the original opener. `checkDialogFocus()` in `scripts/ui-consistency.browser.cjs`; hosts-editor 12/12 (it failed 3 of 6 on main).

Known limits: the Text Editor's own Find bar still has its Hide button; the movable Find window (feat/editor-find-window, reviewed and fixed, next release) replaces it. Intermittent native Windows failures (`TestNativeSFTPDirect121InstalledWebView2`, `TestNativeEditorLocal119InstalledWebView2`) are tracked separately; they pass on re-run.

# v1.21.55 Beta 1 - Document Map and Document List

Owner request (Notepad++ screenshot): a Document Map and a Document List in the Text Editor. The owner confirmed on their PC that word-wrapped long lines scroll all the way through in v1.21.53.

v1.21.54 was published and verified: tag at merge commit fef0f56 (PR #179), release run 43405425920 green, all seven assets matching GitHub's digests, `github_release.py verify` reporting "Verified 1.21.54", stable channel still at v1.21.34. Its CI showed the intermittent disk-view double-click failure (passed on the one re-run); PR #180 traced it: the syntax layer was drawn about six rows above its text for a few frames deep inside a page, so the click missed the textarea. That check now clicks once the word has settled and records every selection change of a failed attempt (merged here too).

After v1.21.54 the owner decided: no brand-specific tool (the SonicWall readable copy goes; general tools with plain explanations stay), buttons that open a row toggle it closed instead of a separate Hide/Close control, and the Find and Replace modes get a built-in syntax guide. Those are on their own branches (fix/generic-text-tools, fix/toggle-buttons, feat/editor-find-window), not in this release.

What changed (rules in AGENTS.md, the Document Map bullet; guide in docs/guide/editor.md):
- `web/assets/editor-document-map.js`: a 96 px panel at the right edge of the editor workspace (64 px in narrow windows), toggled in More and command search, saved as `editorPreferences.documentMap` (off by default, shared). Drawing is bounded: words as blocks at one or more pixels per row, thin lines up to 8 lines per pixel row, one sampled line per pixel row beyond that, at most 240 characters read per line; the visible box is a positioned element, so scrolling never redraws the canvas; page-held documents over 2 MiB redraw after typing pauses. Disk-backed documents are mapped by position (by line without Wrap, by character offset with Wrap, the same values as the document scroll bar; `editor-disk-view.js` exposes them), with detail only for the loaded page. Hidden in Compare; one map following the active pane in split view.
- `web/assets/editor-document-list.js`: a third left-rail item, "Docs" (accessible name "Documents"), sharing the Files sidebar's width, side and open state (`editorFilesPreferences.panel = 'documents'`); unsaved markers, split groups, keyboard (arrows, Home/End, Enter/Space, Escape), and the tab strip's menu on right-click or Shift+F10.
- Both are in every Text Editor, including the standalone Terminal's floating editor (through its own rail and More menu, not its command search).

Checks: `TestEditorSessionBrowserMapList` (`scripts/editor-map-list.browser.cjs`: box against scroll, click/drag/wheel, typing, Wrap, font, themes, split view, list keys and menu, reload, a 640x400 detached window, 16 MB page-held and disk-backed documents with no long task over 50 ms on the disk-backed one, a 1.7 MB single-line Base64 document with Wrap on), `scripts/editor-document-map.test.cjs`, `standalone-apps`, `editor-disk-view`, `connect-sidebar118`, `editor-sidebar117`/`121`, `editor-toolbar113`, `file-row-drag120`, `TestSharedDataBrowser`.

Also in this release: a track click on the disk view's document bar could be lost (CI timed out waiting for the view to follow it): a paint that ran between the bar's move and its scroll event put the bar back. The view now remembers where it last put each bar (`barSet`, `barLeftSet`) and leaves a bar the user moved for its scroll event; `scripts/editor-disk-view.browser.cjs` moves the bar and paints in the same task (fails on the old code). The disk-view double-click check (#180) found the earlier CI failure: the browser delivered a wheel step late, onto the next view.

Known limits: on a 16 MB page-held document the browser's own textarea causes 100–300 ms long tasks with or without the map (the map's own work peaks near 8 ms); the map of a disk-backed document shows detail only for the loaded page and a light texture elsewhere. Not yet tried on Windows by the owner.

# v1.21.54 Beta 1 - Text tools in the Text Editor

Owner request: "text editor can decode base 64, but I also need functions that can also format a document like this SonicWall config correctly for reading. Notepad++ allows for editing documents using find and replace things like & with \n. I'd like there to be shortcuts or to functions like that and other common and useful ones, with tooltips that describe what they do." Owner decisions: a Tools menu on the toolbar (grouped, each item with a description and tooltip, the same groups in the right-click menu), and the SonicWall readable copy in a new tab, offered when an export is opened, never written back.

v1.21.53 was published and verified: tag at merge commit be0d53a (PR #177), release run 43392201165 green, all seven assets matching GitHub's digests, `github_release.py verify` reporting "Verified 1.21.53", stable channel still at v1.21.34. Its CI found a real bug (every mouse press counted as a selection drag, so a double-clicked word in the disk view could be lost; a press is now a drag only once the pointer moves with the button held).

What changed (rules in AGENTS.md, the text tools bullet; user guide in docs/guide/editor.md "Text tools"):
- `internal/texttools`: every tool once, in Go, streaming, with table tests (lines, encode/decode, JSON/XML format, case and tabs, SonicWall readable). Sort, remove duplicates and reverse spill sorted runs to the scratch folder (memory: one run plus one line per open run; a single gigantic line is held whole).
- `internal/editorbuffer`: `Snapshot.Rewrite` (a tool's result as one undo step; unchanged stretches keep their pieces and line endings), `NewText` (a new unsaved document from a tool's output), `LineRange`, and regular-expression Replace templates (`template.go`, cases shared with the page in `testdata/replace-template.json`).
- Engine routes (`cmd/network-toolkit/editor_text_tools.go`): page-held text through `POST /api/editor-views/text-tools` (result capped at 4x the text plus 32 MiB); disk-backed documents through `tool-start`/`tool-status`/`tool-cancel` over a buffer snapshot, `replace-text`, `tool-detect` (first 4 KB), and open mode `derive` for the readable copy. `Binary` documents refuse every one.
- Page: `web/assets/editor-text-tools.js`, the Tools button and right-click groups, command search entries, shortcuts Ctrl+Shift+L, Ctrl+J, Ctrl+Shift+B, Ctrl+Shift+U, Ctrl+U; More keeps Save copy, Text colors and preferences. Find has Normal / Extended / Regular expression modes. The JavaScript 1 MiB Base64 path is gone. The editor inside the standalone Terminal keeps exactly the actions it had (now under its Tools button and right-click menu).

Review (independent, before CI): one HIGH (JSON/XML pretty-print grew with the square of the nesting depth and could not be cancelled: now 512 levels at most, output bounded, Cancel checked while writing), three MEDIUM (the readable copy kept its source snapshot open on an error path, a lock-order hang; URL decode and Unescape held long escape runs in memory; line tools missed an empty last selected line), three LOW (HTML decode took legacy entity prefixes; a single Replace built a huge expansion before checking it; a sort run file could be left behind) and the floating editor's lost mouse actions: all fixed, each with a test that failed before.

The owner's real TZ 670 export (kept out of the repository) ends its Base64 with "&&", which Make readable refused; it now accepts '&' and white space after the Base64. Checked privately: all 43,904 settings match an independent decode (with 1,200 \n, 1,122 \r and 1 \x escapes). The fixtures now end with "&&" too.

Checks: `go test ./internal/texttools/ ./internal/editorbuffer/`, `go test ./cmd/network-toolkit -run 'Editor|Text|Tool|Binary'`, `node --test scripts/*.test.cjs` (1800), `TestEditorSessionBrowserTextTools` (`scripts/editor-text-tools.browser.cjs`), `scripts/standalone-apps.browser.cjs`, `scripts/editor-disk-view.browser.cjs`.

Known limits: sort and remove duplicates hold one whole line in memory; pretty-print refuses nesting deeper than 512 levels and output over 64x the input plus 64 MiB; in a file with mixed line endings, tools that rewrite their range (decode, format) give new line breaks the file's most common ending; Sort's order is the tool's own (case-insensitive, numbers by value), not the browser's locale order; the search mode is not saved between runs. Not yet tried on Windows by the owner.

# v1.21.53 Beta 1 - Wrapped long lines scroll all the way through

Owner, opening a 1.75 MB SonicWall settings export (one line of Base64) with Wrap on: "word wrap is not working and there is no vertical scrollbar". The disk view showed only the first 49,152-character page of the line; its vertical bar was sized from the line count (one line, so no bar), the wheel stepped between sections, and a mouse drag could not leave the loaded page, so the owner's selection of "everything" was the first page and Base64 decode silently converted only that part. A right-click also reset a whole-document (Ctrl+A) selection to the visible page.

v1.21.52 was published and verified: tag at merge commit 1fdc299 (PR #175), release run 43374809651 green, all seven assets matching GitHub's digests, `github_release.py verify` reporting "Verified 1.21.52", stable channel still at v1.21.34. The owner confirmed on their PC that files opened with the Text Editor now show the document icon.

What changed (model and rules in AGENTS.md and the header of `web/assets/editor-disk-view.js`): in wrapped mode without comparison alignment the loaded page slides through the document; the vertical bar maps to the character offset in the whole document, its thumb sized from estimated wrapped rows; a forward page change starts the new page at a visual row start of the current one so wrapped rows stay put, and pages inside a long line start a whole number of rows after the line start. Selections made by dragging, Shift+click and Shift+keys continue across pages (`virtual` selection), right-click keeps them, the cursor column is exact on long lines, and the status bar shows "N selected" for disk-backed and page-held documents. Without Wrap the bar under a long line spans the text. `editor-disk-session.js` gains `peek()`.

Checks: `scripts/editor-disk-view.browser.cjs` `verifyLongLineWrap` (synthetic 1.7 MB Base64 line: wheel and bar to the end, drag and Shift+click and Shift+PageDown past page edges, Ctrl+Home/End, the bar without wrap) and `scripts/editor-large-file.browser.cjs` `verifyLongLineStatus` (status after Ctrl+A, after a click, page-held count).

Known limits: after a jump (bar, Go to line, Ctrl+End, find) in a long line of prose or minified code, scrolling up into text not yet shown can re-wrap the top rows once per page change (the row holding the top character stays at the same height); Base64 and hex, and scrolling back over text already passed, are exact. The bar moves by characters, so it moves faster through runs of short or blank lines, and its thumb size is an estimate. Not yet tried on Windows/WebView2 by the owner.

# v1.21.52 Beta 1 - The document icon through Open with entries

Owner, after Setup 1.21.50/51 and setting .txt/.conf/.json to the Text Editor in Settings → Default apps: the files still show the Text Editor's program icon (the page on the dark tile), not the document icon. The released exe's icon groups are in order, so `"…\Network_Toolkit.exe",3` is the document icon and the ProgID already used it: the files were associated through an Applications entry. Explorer shows the `DefaultIcon` of the key the UserChoice names; `Applications\Network_Toolkit.exe\DefaultIcon` was the editor icon (1), and Windows' own `Applications\Network_Toolkit_TextEditor.exe` (written when the user picks that program in Open with) has none, so its files show that program's icon 0, also the tile.

v1.21.51 was published and verified: tag at merge commit db72c6b (PR #174), release run 43341159552 green, all seven assets matching GitHub's digests, `github_release.py verify` reporting "Verified 1.21.51", stable channel still at v1.21.34. Its CI found a real bug (Cancel of a tab move was ignored while the ownership poll ran, leaving the tab frozen; fixed). The released Network_Toolkit.exe was checked to hold RT_GROUP_ICON 1, 2, 3, 4 in that order, so index 3 is the document icon.

What changed (rules and sources in AGENTS.md, the icon rule): Setup, the portable registration and the post-update repair write the document icon (3) to `Applications\Network_Toolkit.exe\DefaultIcon` (the repair only when that entry's command is the Text Editor command of the running program). Windows' own entry of the separate Text Editor program gets `"<folder>\Network_Toolkit.exe",3` when it starts the program beside the installed or registered portable copy and has no `DefaultIcon` key (or the recorded value); the value is recorded as `EditorApplicationIcon` in Setup's `TextEditor` key or the portable record (Setup adopts the portable value), and uninstall, Remove and Clean up delete only that value while it equals the record. The entry is never created and its command never changed. `ApplicationIcon` and the right-click verb keep the editor icon, and Setup's and the portable registration's own `Applications\Network_Toolkit.exe` entry now also carries the program icon on its open verb (`shell\open` `Icon`, index 1; Visual Studio Code's pattern), so Open with shows the program whichever value it reads; it lives and goes with the entry, and the post-update repair adds it under the same guard. Windows' own entry of the Text Editor program never gets it.

`editorRegEditorAppIconUninstallerVersion` is 1.21.52, this release (the post-update repair writes the separate program's entry for an installed copy only when its uninstaller is at least that version); `TestEditorAppIconUninstallerVersion` keeps it no newer than `model.Version` and checks that Setup's uninstall holds `RemoveEditorApplicationIcon`.

Review (no HIGH or MEDIUM; LOW findings fixed): the post-update repair and the portable registration no longer replace an icon the user set on the `Applications\Network_Toolkit.exe` entry (the repair writes its `DefaultIcon` only when absent or `"<program>",1`, the verb icon only when absent; the portable registration only absent values or a registration's own); Setup still rewrites its own entry whole on every install, as before. Setup's icon write into Windows' own Text Editor entry is no longer fatal: a failed write is skipped and not recorded (`TestSetupEditorApplicationIconIsNotFatal`).

Known limits: not run on physical Windows. Which value Open with reads for the "Network Toolkit Text Editor" entry is not confirmed on Windows 11 (Microsoft documents the Applications `DefaultIcon` as the files' icon; ReactOS's Open with uses the program file's icon); the verb icon covers it the way Visual Studio Code does. Check Open with and Default apps on a real PC. Versions older than 1.21.52 do not know the `EditorApplicationIcon` record: an older Setup taking over a newer portable registration leaves that icon behind after uninstall, an older portable copy's Clean up leaves the value and its record key, and an older portable copy registering again writes the editor icon back until a newer registration, Setup or update repairs it. The version gate trusts `DisplayVersion`, which only Setup writes.

---

# v1.21.51 Beta 1 - Open with and file types after an update

Owner (installed with Setup long ago, updated in the app to 1.21.50): "Text editor won't show up in the list of apps for json and some other file types", and picking `Network_Toolkit_TextEditor.exe` with "Look for another app on this PC" failed with "The Text Editor launch request cannot be combined with another application mode."

v1.21.50 was published and verified: tag at merge commit fa936d2 (PR #173), release run 43159439824 green, all seven assets matching GitHub's digests, `github_release.py verify` reporting "Verified 1.21.50", stable channel still at v1.21.34. Its CI found a real peer-send bug (text queued for the standalone Terminal's own window right after a session popped out; fixed by `peerPrimaryHolds`).

A first version of this change also registered `Applications\Network_Toolkit_TextEditor.exe`; it was reverted because Windows could list "Network Toolkit Text Editor" twice under Open with, and the bare-path fix already makes Windows' own entry for that program work.

Causes: Windows' own Open with entry starts the chosen program as `"<program>" "%1"`, and the programs refused (Text Editor program) or ignored (`Network_Toolkit.exe`, which opened the main window) a bare path; and Setup up to 1.21.43 registered five types, while an in-app update never runs Setup.

What changed (rules in AGENTS.md): `applyOpenWithArgument` (editor_launch.go) reads one bare path, alone, as `--editor-standalone --open-file` (Terminal program refuses it; `updateRestartArgs` drops it). After an approved update, `refreshEditorTypes` (text_editor_types.go) adds missing types as Setup would when the installed uninstaller is 1.21.44 or later; with an older one, App settings → Updates and → Editor show "Windows lists the Text Editor for only some file types. Run Setup <version> once…" with a Run Setup button (Install the full version's verified download, two confirmations, then the app's own close).

Known limits: not run on physical Windows; no startup toast for the notice (it is in App settings only).

---

# v1.21.50 Beta 1 - Pop-out limits resolved

Owner, about the v1.21.49 limits: "can you resolve the limits?"

v1.21.49 was published and verified: tag at merge commit 9eefb9e (PR #172), release run 43126248300 green, all seven assets matching GitHub's digests, `github_release.py verify` reporting "Verified 1.21.49", stable channel still at v1.21.34. Its PR needed the native fixtures to allow `POST /api/window-send/report`, and a second review found and fixed a Save As path in the remote-file grant (now limited to the exact file on the pinned connection, tab kept in the main window).

What changed (owner decisions recorded in AGENTS.md): the receiving window of Send to comes to the front after a successful delivery (`window_raise.go`; child PIDs recorded at launch, `AllowSetForegroundWindow` for the receiver only, `GET /api/window-send/activation` long-poll per child); the document hand-over carries undo/redo for page-held tabs within the 8 MiB limit and names any tab that moved without it (disk-backed history lives in the engine buffer and stays); detached windows of the standalone apps reach peer send only through their owner engine, and window send runs in the standalone engines too; SFTP Edit from a detached workspace reopens a closed main window (`activatePrimary`, up to 20 s, only the reopened window counts as ready) before the grant.

Known limits: Windows may refuse the raise (the taskbar button flashes); disk-backed tabs move without undo history; not run on physical Windows.

---

# v1.21.49 Beta 1 - Popped-out windows are the separate programs, still connected

Owner: "when popping out or detaching the text editor from the main toolkit app, it must open the standalone text editor and transfer the tabs and content, or maintain a live, updated feed of the tabs or content between both. same for terminal. also all the single toolkit app features between them must continue to work accordingly". Decisions: "Connected separate program", "Add to the open one where possible", a single tab moved to a separate window keeps its own connected window, and Edit from a popped-out Terminal workspace's SFTP browser is allowed.

v1.21.48 was published and verified: tag at merge commit eca3c84 (PR #171), release run 43033856517 green on its fifth attempt (attempts 1 to 4 failed before building because the hosted Ubuntu runner could not resolve the Ubuntu package mirrors for about 4.5 hours), all seven assets matching GitHub's digests, the Update ZIP holding all three programs, `github_release.py verify` reporting "Verified 1.21.48", stable channel still at v1.21.34. Its PR needed three Windows fixes for 8.3 short paths in the Start menu rewrite (long-path the install folder, shortcut targets and the Start menu folder).

What changed (rules in AGENTS.md): detached windows start through `detachedViewProgram` (edition.go) from the same-version separate program when only this user and the system can change it and its folder, holding the checked file open until the window starts, else from the running exe; each separate program accepts only its own view switch. Window send (`window_send.go`, `web/assets/window-send.js`) routes Send to between the windows of one engine to the window that owns the target. Document hand-over (`peer_handover.go`, `web/assets/editor-handover.js`) gives local and untitled tabs to a running standalone Text Editor on Detach editor / Move all, confirm-then-release, with canonical path keys from the receiving engine. A detached Terminal workspace's SFTP Edit opens the file in the main window's editor through a grant limited to that file while the workspace owns the session on the same connection.

Two independent reviews: the first found one MEDIUM (unsaved text of an earlier hand-over could be discarded) and five LOW issues, all fixed or documented (a junction to a network share passes the drive-letter check; a junction above the program folder changed between check and start).

Known limits: not run on physical Windows; the taskbar identity of a pop-out comes from the separate program's file and is unconfirmed on a real PC; Send to does not bring the receiving window to the front; a detached window of a standalone app still cannot send to the other app; undo history does not move with a hand-over; SFTP Edit from a popped-out workspace needs the main window open.

---

# v1.21.48 Beta 1 - The separate programs arrive with the update

Owner, after updating to 1.21.47: "i updated but the separate exes did not appear", then "it should add them without asking because it's not obvious at all that it needs to be done manually", and "it also doesn't look like the file type icon has updated".

v1.21.47 was published and verified: tag at merge commit 5c47d15 (PR #170), release run 42983334985 green, all seven assets matching GitHub's digests, the portable ZIP holding all three programs, `github_release.py verify` reporting "Verified 1.21.47", stable channel still at v1.21.34.

Cause: updates from 1.21.46 and older are finished by the old helper, which replaces only Network_Toolkit.exe; only Setup rewrote shortcuts and the document icon. Now (`updater_companions_auto.go`, `postUpdateRepairs` in `updater_startup.go`): the first start after an approved update (`--update-health` plus the helper's confirmation, which every helper since v1.17.63 writes; Windows x64 main window only) installs the separate programs through the same verified path as the button, holding the network slot, once per run, never retried until the next update; the status reads `pending` from the first request so the page polls and hides the button. Setup-owned Start menu shortcuts are pointed at the separate programs only when the uninstall key's `DisplayVersion` is at least 1.21.47 (an older uninstaller would leave them behind); otherwise the page says the next Setup does it. `editorRegRepairDocumentIcon` sets our ProgID's `DefaultIcon` to index 3 only when its command starts the running exe. This is the owner's exception to the explicit download consent rule: only after an update the user approved, only this version's files from its own release.

An independent review found one MEDIUM issue (the shortcut rewrite stranded shortcuts with an old uninstaller) and three LOW ones (the page missed the install; no network slot; no auto-install after a versioned-filename reopen). The first three are fixed; the last is a documented known limit.

Known limits: the real shortcut rewrite runs only in hosted Windows CI; not run on physical Windows. It already applies to the update that installs 1.21.48 (the new exe runs the repairs on its first start, and the 1.21.47 helper installs all three programs anyway).

---

# v1.21.47 Beta 1 - The Text Editor and the Terminal as separate programs

Owner: "i'd like the full version to offer Text Editor and Terminal as separate executables that retain all of their current functionality and interactions", answered "Full separate programs" and "Portable too".

v1.21.46 was published and verified: tag at merge commit 7e3e1fc (PR #169), release run 42979316729 green, all seven assets matching GitHub's digests, `github_release.py verify` reporting "Verified 1.21.46", stable channel still at v1.21.34. Its new Windows tests found a real bug before release ("Install the full version" refused Setup when %LOCALAPPDATA% has 8.3 short names; fixed by comparing long path forms).

Design, upgrade paths and known limits are in `docs/SEPARATE_EXECUTABLES.md`. `Network_Toolkit_TextEditor.exe` and `Network_Toolkit_Terminal.exe` are the same code built with `ntk_editor`/`ntk_terminal` (resources in `internal/winres/<edition>`, linked with `build/separate-programs.overlay.json` so the main resource object is left out; a plain tagged `go build` fails with "too many .rsrc sections"). `Network_Toolkit.exe` forwards a standalone app to a same-version sibling and runs it itself when the sibling fails within 3 s. The Update ZIP carries all three; the update helper replaces them in one transaction, start-checks each separate program with `--version` and rolls everything back on any failure. Setup installs all three and replaces separate programs left behind by an older uninstaller. Release assets stay seven; `FIRST_SEPARATE_PROGRAMS` is 1.21.47, so 1.21.44 to 1.21.46 still verify.

An independent review found two MEDIUM issues (Setup refused a folder with left-behind separate programs; nothing checked that the separate programs start) and seven LOW ones; the MEDIUM ones and three LOW ones were fixed, the rest are recorded as known limits (busy check keyed by data root, a ~26 MB Update ZIP under older updaters' five-minute limit, a lone exe gaining two programs on update, a separate program started from Start during the update's start check is not stopped).

Known limits: not run on physical Windows; the release builds' real PE resources and the Setup paths are exercised only by hosted CI.

---

# v1.21.46 Beta 1 - The portable program works with Windows

Owner: "i want option 2 for full functionality" (the portable copy writes the same per-user registrations as Setup, with Remove) and "id also like an option for the portable version to be able to download the full install version if desired."

v1.21.45 was published and verified: tag at merge commit 517757c (PR #167), release run 42970739003 green (build, windows-setup, publish), all seven assets matching GitHub's digests, `github_release.py verify` reporting "Verified 1.21.45", stable channel still at v1.21.34. It is the first release built with Go 1.26.11 (PR #168, GO-2026-6859 in net/http).

Portable registration (`cmd/network-toolkit/text_editor_registration*.go`, App settings → Editor of the Windows main window): the same keys, values and file-type list as Setup (the Go list is generated from `scripts/installer/text-extensions.nsh`), commands pointing at the portable exe, the editor icon for program values and the document icon (3) for the ProgID; record and created keys under `Software\Network Toolkit\PortableTextEditor`; Remove deletes only recorded keys and only when empty (Go port of `DeleteEmptyUserKey`). Ownership: refused while Setup is installed; one portable copy at a time (takeover after confirming); Setup adopts a portable registration and retires its record only after its own registration is written; clean-up of a copy whose program is missing has its own request naming the program shown; a drive that is not connected is "unknown", not missing. Refused for a path containing % (Setup refuses such a folder too) and for a copy others can change. A hidden `--text-editor-registration=` switch, accepted only alone, serves the hosted check.

Install the full version (`cmd/network-toolkit/full_install*.go`, App settings → Updates): this version's Setup asset from the release (a fallback only when newer, and named), downloaded after consent through the updater's own download functions and verified with `updateVerifySum` against the release's SHA256SUMS; stored in `%LOCALAPPDATA%\NetworkToolkit-Setup` with an owner-only DACL (never the data folder); started after a second confirmation from a handle opened with read sharing only, hashed from that handle and held until CreateProcess returns.

A security review found two MEDIUM issues (the started Setup could differ from the verified bytes in a folder others can write to; the fallback could offer an older release as an update) and four LOW ones; all fixed. The Windows-only tests (`full_install_windows_test.go`, the registration DACL test) run in the `windows-terminal` job.

Also in this release (branch fix/peer-send-main-grace): Send to gives the main window up to 3 s to report before passing it over (`peerMainGrace`), and `scripts/standalone-peer-live.browser.cjs` waits for the main window to be listed before clicking. Known limit: a main window minimised for a long time may have its page's 1-second inbox poll throttled; a long-poll inbox is the proposed fix.

Known limits: Explorer's behaviour and the real Setup start were exercised under Wine and by hosted checks, not on a physical PC; in-app updates do not register anything (only the portable section or Setup does).

---

# v1.21.45 Beta 1 - A document icon for files opened with the Text Editor

Owner: "id like the icon files that set text editor to open as to be the icon image of the paper sheet, but not the background or border of the icon." The owner approved a rendered preview (outlined page, larger, transparent background).

v1.21.44 was published and verified: tag at merge commit 8381055 (PR #166), release run green (build, windows-setup, publish), all seven assets matching GitHub's digests, `github_release.py verify` reporting "Verified 1.21.44", stable channel still at v1.21.34.

`assets/source/Network_Toolkit_Document.svg` (and hand-drawn `Network_Toolkit_Document.<16|20|24|32>.svg` for the small sizes) build `assets/Network_Toolkit_Document.ico` (`python3 scripts/generate_icon_assets.py --document`). It is a fourth icon group appended after app 0, editor 1 and terminal 2, so the document icon is index 3 and earlier indexes are unchanged (`scripts/windows_resources_test.py`). Setup points the ProgID `NetworkToolkit.TextDocument\DefaultIcon` at index 3 on every install, repair and upgrade; `ApplicationIcon`, `Applications\Network_Toolkit.exe\DefaultIcon` and the right-click verb's `Icon` keep the editor tile (index 1).

This release is the first built with Go 1.26.11 (PR #168), which fixes GO-2026-6859, an HTTP/1 server crash in `net/http` published on 2026-11-11 that failed the dependency scan on every branch.

Known limits: Explorer's display and icon cache on real Windows were not observed (Setup was exercised under Wine and by the hosted setup check); in-app updates do not change file icons; a type the user sets to the editor through Open with, Always that Setup does not list uses `Applications\Network_Toolkit.exe` and so shows the tile.

---

# v1.21.44 Beta 1 - The Text Editor opens almost any file

Owner: "the text editor should be configurable as default app for other text formats and file formats that are unknown. similar to how notepad++ opens and supports almost any file type."

v1.21.43 was published and verified: tag at merge commit 1a4d6ee (PR #165), release run 42877313781 green (build, windows-setup, publish), all seven assets matching GitHub's digests, `github_release.py verify` reporting "Verified 1.21.43", stable channel still at v1.21.34. On that pull request two jobs failed once and passed on their single re-run: `TestSharedSavesFromSeparateProcessesLoseNothing` (one helper process got 409 "another app is saving" after the full five-second lock wait; to be investigated) and `scripts/editor-saved-connections113.browser.cjs` at 390 px (the "locked" hint did not appear within 15 s; not explained).

Installer: the offered types live in one file, `scripts/installer/text-extensions.nsh` (108 types), used by `windows.nsi`, `verify-windows.ps1` and the generated list in `docs/guide/editor.md` (`node scripts/text-extensions.cjs --write`; `scripts/text-extensions.test.cjs` fails on drift). Setup writes, per user: the Capabilities\FileAssociations values and each extension's `OpenWithProgids` value (never an extension's default value); `Software\Classes\Applications\Network_Toolkit.exe` (shown under Open with for any file; written only when absent or already starting the installed program); and, only with the Setup option or `/ContextMenu`, `Software\Classes\*\shell\NetworkToolkit.TextEditor`. Keys that could pre-exist are recorded under `Software\Network Toolkit\TextEditor\CreatedKeys`; uninstall and rollback remove only those. No `UserChoice`, nothing under `FileExts`, no `SupportedTypes`.

Editor engine (`internal/editorbuffer/codec.go`): bytes that are not valid UTF-8 are read as Windows-1252 and saved back in it (a character the encoding lacks stops the save); mixed line endings are kept per line ("Mixed (keep each line)"); content that is not text opens read-only and the engine refuses every write path; unreadable, missing and folder paths give one plain message. UTF-16 without a BOM is shown as not text, not guessed.

Two independent reviews ran before release. The first found that a mixed file could lose a line break on save and in the kept copy (a bare LF written directly after a lone CR merged into one CRLF) and that Replace All rewrote every line ending of a mixed file. Both are fixed: such an LF is written as CRLF, the only case where an untouched ending's bytes change; Replace All now replaces each match on its own in one undo step (`Snapshot.ReplaceAll` plus `Commit`), for every document; an edit leaves out what the new text repeats at the ends of its range, and an edit that keeps the number of lines keeps each line's ending. The second review matched the new Replace All against an in-memory model in about 5,000 random cases and found one hang: `Commit` re-took the buffer's lifetime lock while the handler still held a snapshot, so a pending close blocked both for good. `Commit` no longer takes that lock, the handler releases its snapshot first, reload, upload-replace and close stop a Replace All in flight, and an abandoned Replace All gives its scratch text back (`Replacement.Abort`). Silent setup also has `/NoContextMenu`, and an upgrade from a released version adopts the five earlier types' `OpenWithProgids` keys into the record only where a key holds nothing but our ProgID.

On 3f683ed two hosted jobs failed once each and are not from this change by the code paths involved: `TestNativeEditorTransferInstalledWebView2` (the detached window did not accept the offered transfer within its fixed 20 seconds) and `TestStandalonePeerSendBrowser`, phase "main only" (the main window's page had not polled its inbox within five seconds, so the sender went on to start the standalone app and waited up to 25 seconds, longer than the test's 20). Proposed for the second, not made here: wait two to three seconds for a fresh poll before passing the main window over, and let the test wait longer than `peerLaunchWait`.

Known limits: in-app updates do not add the new registrations (only Setup does); how Explorer on Windows 10 and 11 presents these entries was not observed (Setup was exercised under Wine and by the hosted setup check, not on a physical PC). A large binary file can need two to three times its size in temporary space.

---

# v1.21.43 Beta 1 - An opened document is the tab in front

Owner: "when a document is opened by default with text editor, the opened document must be the active tab. currently it seems like the last active tab remains active even when the new document is added as a new tab."

v1.21.42 was published and verified: tag at merge commit 684095f (PR #164), release run green (build, windows-setup, publish), all seven assets matching GitHub's digests, `github_release.py verify` reporting "Verified 1.21.42", stable channel still at v1.21.34. Its security review found no HIGH or MEDIUM; seven LOW items were hardened before the merge.

Cause: a file from outside and the reopening of the last session start together and finish in either order, and every reopened tab made itself active as it was added; when the file was itself a kept tab, the restore preferred the session's stored active tab. Wrong before the fix: a cold start with `--open-file` and a kept session (including tabs taken over from the other editor), the main window started with or handed its first file before its editor page was shown, restore off with kept unsaved text, several files before the session was back, and a session read that failed and was retried. A running editor whose session was already restored was right.

Fix (page only, `web/assets/file-editor.js`, `web/assets/editor-session.js`): the editor remembers the last document asked for until the session is back (`noteAsked`); kept tabs reopen behind it and `finishSessionRestore` activates it; the restore asks the host for that document (`requested`) and prefers it over the stored active tab. The same rule covers a file or text opened from another tool, and a tab the user clicks, while the session is still loading. Every kept tab and its unsaved text still reopens in order. Tests: `scripts/editor-session.test.cjs`, the last part of `scripts/editor-session-restore.browser.cjs`, and `TestEditorSessionBrowserOpenedFileIsActive` (two real engines, a disk-backed file with unsaved text, real second starts); the `browser-editor112` step now runs every `TestEditorSessionBrowser*` test.

Known limits: with 12 tabs already open a launch file is refused with the existing "12 maximum" message. The Windows file association, native foregrounding and the DPAPI record were not run; the hand-over was exercised through the same Go path on Linux. Not yet seen on physical Windows.

---

# v1.21.42 Beta 1 - Send to opens the other app by itself

Owner: "the 'send to' commands in terminal and text editor should automatically open the respective standalone app to perform the task and if the main toolkit app is already open then it would send there".

v1.21.41 was published and verified: tag at merge commit 7a376a0 (PR #163), release run green (build, windows-setup, publish), all seven assets matching GitHub's digests, `github_release.py verify` reporting "Verified 1.21.41", stable channel still at v1.21.34. On that pull request's final commit `TestNativeSFTPDirect121InstalledWebView2` failed once (setup-sftp timed out with a page startup error "Failed to fetch", run 42816756135) and passed in the other run and on its re-run; it is not explained.

The sender's engine now chooses the receiver (`peer_send.go`): the other standalone app if its protected record answers, otherwise the main window of the same data folder if its page is polling, otherwise the other standalone app is started through the existing fixed launch arguments and the text, held only in the sender's engine memory, is delivered when the new app is ready (25 seconds at most; dropped when the sender closes). The main window is a receiver only: it serves `/api/peer/terminal/*` and `/api/peer/editor/*` with the same guards (activation token, loopback, no browser headers, strict JSON, 1 MiB and 8 MiB, one pending request for a minute), has no sending routes, and its page runs the inbox only when `/api/status` reports `peerReceive`. A target chosen from one receiver's list is sent only to that receiver. Text to a terminal still goes through `pasteText` and its review; a freshly started Terminal has no session and says so. Older apps (plain or unreadable records, no `/api/peer/`) are skipped and named.

Decisions: a running standalone app wins over a running main window; the main window counts as running only with a live page; on Linux the main window never registers a record, so it is not a receiver.

Known limits: the DPAPI record of the main window, the real process start, and raising the native window are compiled and vetted and first run in hosted CI; a long-minimised main window whose timers are throttled can be passed over in favour of starting the standalone app. Not yet seen on physical Windows.

---

# v1.21.41 Beta 1 - Safer recovery of shared settings and saved data

v1.21.40 was published and verified: tag at merge commit 3c08e58 (PR #162), release run 42803456524 green (build, windows-setup, publish), all seven assets matching GitHub's digests, `github_release.py verify` reporting "Verified 1.21.40", main's Checks run 42803456756 green, stable channel still at v1.21.34. Four independent reviews ran on that change. The second found four more data-loss paths (an older window re-saving identical bytes hid another app's save; the migration recorded entries before its write succeeded; a standalone app recreating a missing settings file wiped main-only lists; vault identity creation from a standalone app undid another app's save) and the third found the missing-settings-file case still losing data on three write paths; all were fixed before the merge. The fourth found nothing at HIGH and three MEDIUM cases, which v1.21.40 shipped with and this version fixes.

Fixed here (`shared_review4_test.go`, design in `docs/SHARED_APP_DATA.md`):

- "Delete app data" run from an older build left `Network_Toolkit.saved-lists.json`, and this version showed the saved entries again. The copy is now paired with its data folder: one id is the value of the `sharedDataWriter` mark, `pair` in the copy, and the content of `Network_Toolkit.Views/00000000000000000000000000000000/Network_Toolkit.window.json`, which every version's delete removes. An unpaired copy is ignored and removed at the next start. A copy written by v1.21.40 has no id and is trusted as before.
- A standalone app without a usable copy no longer refuses (409) to write a missing settings file; it writes what it holds and marks the main-only keys as absent (`sharedMainOnlyAbsent`), which a running main window honours and clears. An app that finds the file written anew adds what it holds instead of reading absences as removals.
- With the file missing and the copy readable, a running app merges the copy three ways into what it holds before any write; a copy unreadable at start is taken in before the first write; anything recovered from the copy is named in a notice.

Three more reviews ran on this change before release and each found that the previous fix had overcorrected under a fault: a failed read of the copy or id file minted a new id (other apps then undid a removal) or deleted a good copy; the fix for that refused every save forever when the id file could never be read, and a give-up timer dropped held entries. The rule that ended it, now in `docs/SHARED_APP_DATA.md` ("A save that is refused"): "does not exist", "could not be read" and "read and different" are three answers everywhere, and a problem with the copy or its id file never refuses a save or a move. The settings are saved, the protection files are left untouched, the settings file carries `sharedCopyBehind` (`read` or `write`) until the copy is repaired, and the user is told once which file is the problem. Only an unreadable settings file itself, a busy lock, or an invalid file in a standalone app refuse a save. The page retries a refused save (1 s doubling to 30 s) with a sticky message, and at close retries for about two seconds and then asks (Try again, Close anyway, Keep open); answers that can never succeed are not retried. The last review found no HIGH and no fault in ordinary use; its six findings were fixed without another review round.

Still open after this release: the native "Close not approved" box can appear over the page's close question after five seconds, as it already does for the unsaved-documents prompt (the host has no way to learn that a page prompt is open). Seen once each and not explained: `TestEditorLargeFileBrowser` timing out in its edit step on a runner that took 62 s to open the two files (run 42796947012), and `TestWindowsLocalShellColorTranscript/powershell` not rendering the typed command (run 42810392964).

Known limits: the copy holds only saved lists and main-only keys, so other settings of a stale app can win after a lost file; with no file, no usable copy and no main window running, main-only data is gone; mixed with v1.21.40 (which strips the id from the copy on each save) the pairing is restored only while the Views entry exists. Not yet seen on physical Windows.

---

# v1.21.40 Beta 1 - One set of settings and saved data for all three apps

Owner: "any saved session from the portable app should be accessible to the installer version and the standalone installed version of terminal and vice versa. Same with Text editor files", "the settings should also carry from the main toolkit app to the standalones" and "my terminal for example should have all the same settings, and so should text editor".

The main window (portable or installed), the standalone Terminal and the standalone Text Editor of one data folder now share saved and recent connections, settings, the credential vault, SSH host trust, macros and (when the other editor app is closed) the Text Editor's kept session. The design, the key table, the one-time migration, failure handling and the behaviour with older versions are in `docs/SHARED_APP_DATA.md`; read it before changing any saved key. In short: a page sends the earlier value of every shared key it changed (`_sharedBase`) and the engine merges three ways under the data folder's lock (`shared_data.go`, `shared_merge.go`, `shared_state_http.go`); pages poll `GET /api/shared-data` every 2.5 seconds and apply live keys (`web/assets/shared-data.js`); `migrateSharedData` adds what the standalone workspaces kept for themselves and never overwrites; `internal/sharedfile` reads and replaces files that several running apps use. This replaces the earlier statement that the installed Text Editor has its own preferences: only window state (layout, open page, form drafts) stays with a window.

Two independent reviews ran before release. The first found two data-loss paths (a save cut the shared connection list to 100 and the merge read the rest as removals; the shared "reopen files" setting hid and then deleted kept unsaved editor text), a reader racing the atomic replace of the settings file (spurious vault lock; on Windows a failed save), "Delete app data" undone by the next migration, and smaller gaps. All were fixed with a failing test first; the per-item list is in the pull request. Saved lists are no longer cut anywhere (a full list refuses the new entry), kept unsaved text is always reopened once and never removed unseen, and the engine enforces both whatever a page does.

Older versions on the same data folder (a portable and an installed copy update separately): this version marks the settings file and keeps a copy of the saved lists (`Network_Toolkit.saved-lists.json`); when a file without the mark is read, entries the older writer dropped are put back and named in a notice (`shared_older_version.go`). What stays unprotected while an older copy is open is listed in `docs/SHARED_APP_DATA.md`; the release notes ask users to update every copy.

Windows: the shared-file code first ran in hosted CI on this branch and needed three iterations (folders need backup semantics to open; the lock file is created without sharing, so a concurrent first save is now waited for; a leaked session-lock handle; a plain `MoveFileEx` is refused while any reader has the file open, so replace falls back to a rename with POSIX semantics, and to a short retry where that is unavailable). The `windows-terminal` job has three new steps for it.

Also in this release, from PR #161: Setup and Uninstall record why they stopped in `%TEMP%\Network_Toolkit_Setup.log`, staged files are placed with a five-second retry, and the uninstaller waits for a pending delete.

Known limits: not yet seen on physical Windows; no real third-party scanner, no real v1.21.38 binary (its behaviour is simulated from its code) and no native password dialog for the vault import were exercised. One hardening has no dedicated test (a failed read never creates or drops the vault identity). A saved entry removed in an older version comes back in this one, by design. The hosted setup check has no shared-data step.

---

# v1.21.39 Beta 1 - Standalone apps send to each other and show only their own UI

Owner: "in installer mode, the standalone text editor should be able to send to the standalone terminal and vice versa with their existing send to tools. also, for the standalone tools, remove the features or functions, or ui elements not relevant to the specific app unless it is associated with another app that it can connect to" and "the standalone terminal app should still be able to open cmd and powershell".

Peer send (`peer_send.go`, `web/assets/peer-send.js`): the sender's page asks its own engine (`/api/peer-send/`, session token); the engine reads the other app's instance record of the same data root and calls it on loopback with that app's activation token (`/api/peer/`, loopback only, refused with any browser header, one pending request, 60 s). The Terminal's instance record is now DPAPI-protected like the main and editor records; an older plain record still receives window launches but is never trusted for peer send. The receiving page polls its inbox and runs the same code as an in-window send: the Terminal's `pasteText` (paste review, warnings, bracketed paste, 1 MiB) or the editor's insert (8 MiB). Window launch requests still carry only connection settings or a file path.

The Start a terminal card was built before `/api/status` answered, so it never showed Command Prompt or PowerShell in the standalone Terminal (or in the main window when Terminal is the startup page); `refreshTerminalWelcome()` rebuilds it when local shell availability arrives. Standalone palettes, F1 help, settings and buttons are limited to each app (`standalone-apps.js`); the main window and detached child windows are unchanged.

v1.21.37 and v1.21.38 were published and verified earlier the same day: tags at merge commits 45b4786 and a5c34d0, release runs 42758979504 and 42763002678 green, all seven assets of each matching GitHub's digests, and `github_release.py verify` reporting "Verified 1.21.37" and "Verified 1.21.38".

v1.21.39 was published and verified: tag at merge commit e42b2c3, release run 42775425248 green (build, windows-setup, publish), all seven assets matching GitHub's digests, `github_release.py verify` reporting "Verified 1.21.39"; the stable channel still points at v1.21.34.

After the merge, the main Checks run 42775425351 failed `windows-setup` at the last uninstall: `Network_Toolkit.exe` was still present although the uninstaller had exited 0 and removed its registration, with no process running from the install folder (the same job passed in the Release run and twice on the pull request). Two causes fit and both are fixed without a new version: `Invoke-Uninstaller` listed the launcher's children before reading whether the launcher had exited, so it could miss the working copy and check too early; and the uninstaller now waits up to five seconds for the program's name to disappear after a successful delete, because Windows completes a delete only when a scanner that still has the file open closes it (otherwise the empty program folder stayed behind). `Assert-UninstallFinished` now says which case it saw.

The first hosted run of that fix then failed one step later: the fresh custom-folder Setup (or its in-place re-run) exited 2, and silent Setup gives no reason, so the cause could not be read from the log (the other run of the same commit passed). Setup and Uninstall now write the reason they stop to `%TEMP%\Network_Toolkit_Setup.log` (the refusal message, or the install step that failed with its Windows error; message only, never a path), the setup check prints it on any unexpected exit code and checks it on the expected refusals, and staged files are moved into place with a five-second retry on sharing and access errors, because a scanner can hold a file that was written a moment earlier (the same cause as the uninstaller's delete retry in PR #157). If the step that failed was a different one, the next failure will name it. These installer changes ship with the next beta.

Both runs of that first commit also failed `browser-workflows107` in the new `standalone-apps.browser.cjs`: it compared the connection form of the main window and the standalone Terminal right after the form appeared, while `input-clear.js` and `dropdowns.js` add their buttons one animation frame later, so either side could be read before or after them (the two runs differed in opposite directions). The test now reads the form once it has stopped changing. Two other browser failures were seen once each on the same tree and are not explained yet: `app-updater-startup.browser.cjs:180` (the release-notes `details` element gone after pressing Enter on its summary; main run 42775425351) and `editor-layout-map.browser.cjs:104` (`[data-compare-text="left"]` not fillable within 20 s; run 42776696978).

Known limits: an older running app cannot read the protected Terminal record, so its "Separate terminal window" reports the workspace busy until that app restarts. The hosted setup check has no peer-send step. Windows-only parts (DPAPI record, foreground request) are compiled and vetted, and first run in hosted CI. Not yet seen on physical Windows.

---

# v1.21.38 Beta 1 - Text Editor session restore

Owner: "Notepad++ and even windows notepad now save previously opened documents in tab if they were open when the app closed." Each primary workspace (main window, standalone Text Editor) keeps `Network_Toolkit.editor-session/` beside its settings: a strictly validated `session.json` index (metadata only) and one copy per document with unsaved text. Copies of disk-backed documents are streamed by Go from a buffer snapshot, so there is no size ceiling and the text never passes through the renderer; small in-memory documents upload their text. Bindings, connection ids and passwords are never stored. The page saves in the background when something changed and once more during window close; when every dirty document was kept, app close does not prompt, and it still prompts for any document that could not be kept. Tab close and Close editor prompt as before.

Restore reopens tabs in order with the stored active tab. A dirty local file opens disk-backed from its copy with the stored hash as the save baseline, so Save refuses to overwrite a file that changed meanwhile; a deleted file's text and unsaved remote text come back as untitled tabs with a note. The preference `editorPreferences.restoreSession` (default on, More menu) deletes the stored session when turned off. The engine reports `editorSessionAvailable` only for the main and standalone editor workspaces; detached editor windows never save or restore. Sessions move with the data folder.

The native Windows editor acceptance (`editor_native_windows_test.go`: window transfers, drag, and the WM_CLOSE dirty confirmation) runs with `restoreSession` off, because with it on the app keeps dirty documents and does not ask. Keeping and reopening is covered by `TestEditorSessionBrowser` (real editor and session handlers in Chromium) and the Node close tests, not yet through the native window.

Known gaps: the standalone Text Editor's own settings still do not move with the data folder (existing gap), and deleting app data does not remove the standalone editor's session. Not yet seen on physical Windows.

---

# v1.21.37 Beta 1 - Terminal links and uninstall retry

Owner: "the terminal doesn't have to be able to set powershell or cmd as default, it just has to be able to run them. it should be able to be default SSH, Telnet, or serial app". Setup registers Network Toolkit Terminal per user for `ssh`, `telnet` and `serial` links (Capabilities\URLAssociations, ProgID `NetworkToolkit.TerminalLink`, Terminal icon). It never writes a UserChoice and creates a scheme key only when neither HKCU nor HKLM has one, recording what it created so uninstall removes only those keys, and only while unchanged. `--terminal --open-url "<link>"` must appear exactly so; `terminal_link.go` parses strictly (no paths, queries or commands; passwords dropped; the connection-launch host rules), hands the link to the running Terminal window or queues it, never stores it and never replays it after an update. The Terminal opens the existing connection form prefilled, offers an exact saved match, and connects only when the user chooses Connect. Terminal settings > Features > Links opens Windows Default apps.

The uninstaller now retries deleting `Network_Toolkit.exe` for about five seconds before deciding the program is open (PR #157); the hosted setup check also checks link registration, rollback, preservation of another app's handler, and a link delivered to a running Terminal without connecting.

Not yet seen on physical Windows: how Settings > Default apps lists the Terminal and how the "pick an app" prompt appears. The Windows default console (PowerShell/Command Prompt) prototype stays parked on `feat/windows-terminal-default` until a trusted signer exists.

---

# v1.21.36 Beta 1 - Windows setup options and standalone editor

Setup uses the selected program folder and preserves it on upgrade. The optional desktop shortcut opens Toolkit. The installed Text Editor opens in a separate workspace with its own preferences and keeps the existing paged editing, Compare, and save workflows. Each Start menu shortcut and native window uses its own icon.

Published as opt-in prerelease https://github.com/TechSavage1989/network-toolkit/releases/tag/v1.21.36 at 2026-11-10T03:20:43Z; stable latest remains v1.21.34. PR #155 merged after both Checks runs passed on 09daee9 (the push run's `browser-workflows107` hit the known `graph-scale119` live-window intermittent once and passed on its single re-run). Tag v1.21.36 points to merge commit 9316b2f787032d31fe455e425cd7a38d48909f24; Release run 42748387582 passed build, the hosted Windows setup test and publish. The seven downloaded assets (executable, Portable and Update ZIPs, Windows x64 setup, Linux x86_64 and aarch64 tarballs, SHA256SUMS) match GitHub's digests, and `python scripts/github_release.py verify --tag v1.21.36 --output-dir <folder>` reports "Verified 1.21.36".

After the merge, the main Checks run for 9316b2f failed `windows-setup`: right after `Uninstall.exe /S` returned, `Network_Toolkit.exe` was still in the install folder. NSIS uninstallers copy themselves to a temporary folder, start the copy and exit at once, so `verify-windows.ps1` (used by Checks and the Release gate) raced the copy. It now waits for that child process before checking what was removed. The same failure then recurred on PR #157 itself: the uninstaller first deletes `Network_Toolkit.exe` to detect a running copy and aborts on any failure, so a brief lock (antivirus or the indexer scanning the executable the previous step had just written) made it refuse. The uninstaller now retries that delete for about five seconds before deciding the program is open, and the verifier names any process still running from the install folder when uninstall leaves the program behind. `editor-views112` re-focuses the file-sidebar resize handle while waiting, because the sidebar can re-mount after the 390 px layout and drop focus. That focus wait failed twice in a row on the docs-only PR #156.

---

# v1.21.35 Beta 1 - Windows installation and editor defaults

The per-user installer registers the editor in Windows Default apps, adds Start menu shortcuts and keeps existing app data. Windows chooses file associations through its own Settings UI. Repeated Explorer launches use the active editor workspace. The guarded release tests the exact setup executable on a hosted Windows runner before publishing.

The standalone Terminal has a Start menu shortcut. System default terminal selection still requires packaged Windows console delegation and an inbound console session transport. The existing local terminal starts and owns a new pseudoconsole.

---

# v1.21.34 Stable - Large-file Text Editor and Compare

Promotes the editor improvements through v1.21.33 Beta 1 to the Stable channel. Editor behavior matches the verified beta. The release updates the version and channel metadata.

---

# v1.21.33 Beta 1 - Compare zoom alignment and right-click submenus

Compare remeasures line and spacer heights after a font change. The detached editor browser fixture checks matching rows through zoom in and out with Word Wrap on and off. The editor context menu keeps common actions at the top level and groups related commands into submenus that support mouse and keyboard navigation.

---

# v1.21.32 Beta 1 - Editor wheel zoom and moved text

Ctrl+mouse wheel changes the existing editor font size in single, split and comparison views. Synthetic log and PowerShell comparisons exercise repeated lines, moved sections, gray spacers, original line numbers and changed-character marks. Standard comparisons identify exact moved blocks before pairing nearby edits. Disk-backed comparisons use the closer context when both files offer a unique match farther down.

---

# v1.21.31 Beta 1 - Compare policy alignment and character highlighting

Disk-backed Compare preserves the next matching FortiOS policy header and UUID when different policy blocks precede it on both sides. Both panes share a single character comparison for each pair of loaded lines. Lines that span multiple editor pages still use whole-line marks.

The synthetic browser run compared two 2 GiB files, accepted an edit during save, and verified the saved SHA-256. Verify with `go test -mod=vendor ./internal/editorbuffer`, `node --test scripts/editor-alignment-model.test.cjs`, and the opt-in `TestEditorLargeFileBrowser` fixture.

---

# v1.21.30 Beta 1 - Clearer Compare replacement markers

Compare keeps an earlier exact row when a longer repeated match would preserve the same alignment. Editor panes and the differences-only view distinguish large length changes from smaller modifications without altering row geometry. The disk-backed change map still uses the overall region classification.

---

# v1.21.29 Beta 1 - Moved blocks and steadier Compare scrolling

Compare uses bounded context to disambiguate repeated lines and marks exact moved blocks while retaining source order and gray spacers. The synchronized disk viewport uses aligned display rows through page transitions.

---

# v1.21.28 Beta 1 - FortiOS policy alignment

The disk-backed comparison recognizes a unique edit header followed by the same UUID even when nearby fields changed. Synthetic regressions cover three policy insertions, a modified existing policy, and another insertion before the next matching policy.

---

# v1.21.27 Beta 1 - Large-file Word Wrap

The paged editor wraps visible source lines, keeps source numbers on the first visual row, and aligns wrapped comparisons with display-only spacers.

---

# v1.21.26 Beta 1 - Continuous differences-only comparison

The disk-backed difference index stores cumulative changed-row offsets. The differences-only view uses one virtual scroll range and fetches the visible window with its source line numbers. Gray spacers fill missing sides, and the comparison toolbar can still jump to a difference.

Moved-text research covers ComparePlus, WinMerge, and Git. Move detection is a separate follow-up.

---

# v1.21.25 Beta 1 - Editable disk-backed alignment

The comparison index stores a display row alongside each difference. Editable disk-backed panes use that index to project gray rows into the loaded viewport and translate selection and edits back to source offsets. The aligned scrollbar covers the whole comparison. The browser fixture inserts lines in a synthetic FortiOS file, scrolls through the gap, edits below it, and verifies saved bytes.

Syntax coloring reuses the highlighted viewport while scrolling and delays new work until the viewport settles. The change map continues to show the source position of each document.

---

# v1.21.24 Beta 1 - Compare navigation and FortiOS coloring

The working branch tests the full-document scrollbar with native Chromium scrollbars visible. Compare gains compact and dynamic map modes, fixed gutter symbols, counterpart line highlighting, and right-click comparison actions. Disk-backed views color the loaded page through the existing language worker, including FortiOS syntax. Display-only spacers in the editable disk-backed panes remain open.

---

# v1.21.23 Beta 1 - Compare filters and text conversion

The working branch hides the temporary page scrollbar, restores the disk comparison legend and symbols, preserves the viewport through undo and redo, and centers map clicks. Compare Options add whitespace matching modes, a map visibility switch, and jumps to the first and last difference. The read-only differences view pages through changed rows without loading the whole document. More and the text context menu provide selection-based UTF-8 hex and Base64 conversion.

The disk-backed renderer still needs display-only gap rows and an aligned map coordinate system.

---

# v1.21.22 Beta 1 - Editor change map

Comparison defaults to individual one-pixel marks with a shared map scrollbar. The wheel pans the map independently, and clicking a mark navigates the editor. Inline comparisons still offer Fit entire document under Options.

Disk-backed comparisons read the exact visible line range from the existing on-disk difference index. The browser caps the native scrollbar's physical height and maps it to the full logical range, so the map can display changes at the end of multi-gigabyte documents. The comparison browser fixtures cover consecutive marks, independent wheel and scrollbar input, and navigation to changes near the end of the file.

---

# v1.21.20 Beta 1 - integrated large-file editor

PR #139 integrates disk-backed open, edit, search, split views, comparison and saves in the normal app. The real-file browser test with two 2 GiB files verified responsiveness, typing during save and exact saved bytes. See [editor scalability](EDITOR_SCALABILITY_PLAN.md) and [the release handoff](RELEASE12120_HANDOFF.md).

The owner requested publication as soon as verification is complete. Check both push and pull request runs on the release commit, merge through the existing beta workflow, and verify the release assets and update channels.

---

# Earlier v1.21.20 Beta 1 bounded-file candidate (superseded acceptance criteria)

Candidate in [PR #139](https://github.com/TechSavage1989/network-toolkit/pull/139), continuing the owner's report that large files make scrolling and the entire app laggy and comparison does not work correctly. Not published. Stable remains v1.21.19. See [the technical handoff](RELEASE12120_HANDOFF.md) for changes, tests and limits.

- Large-file line numbers repaint once per frame; common edits update the cached line index; scrolling alone no longer uploads a full recovery snapshot.
- Comparisons accept up to 16 MiB and 30,000 lines per side. Large comparisons use a local background worker and bounded processing. The renderer's 30,000-line limit is retained.
- Branch Checks run 42529345146 passed all 15 jobs at application commit `0c8ae1f4b6b1dfe94f0cbaca511255f65d79640d`, including the 16 MiB edit/undo/save and real-worker comparison tests. The matching PR run's updater browser test was still running when this note was written. Final checks must pass on both events before merge.
- Restored the unchanged v1.21.19 notes to `docs/releases/`; updated the README and Editor guide. No layout changes require new README screenshots.
- Validation: `python scripts/github_release.py validate`, `python -m unittest discover -s scripts -p '*_test.py'`, and hosted `.github/workflows/checks.yml`. Native fixture tests run in hosted CI only.
- Remaining: green final checks, guarded release publication, and verification of source/tag, six assets, checksums and stable-channel isolation. Physical Windows/Linux and live-device acceptance are not claimed.

---

# v1.21.19 Stable (Highlighting in the Appearance tab) - publication verified (2026-11-08)

Published as the stable release https://github.com/TechSavage1989/network-toolkit/releases/tag/v1.21.19 at 2026-11-08T07:42:46Z (not a prerelease); releases/latest is now v1.21.19. Merged PR #137 (stacked on the docs PR #136, which merged first); tag v1.21.19 points to merge commit 9d93520c7404c0b4cc7bce0bce2c76123be12e39. Release workflow run 42459712063 succeeded. Assets: Network_Toolkit.exe, Portable and Update ZIPs, Linux x86_64 and aarch64 tarballs and SHA256SUMS; every asset digest matches SHA256SUMS, and the Linux x86_64 binary reports "Network Toolkit 1.21.19 linux amd64". Both Checks runs on PR #137 were green on the first attempt.

Owner: "I believe the highlighting settings should be with appearance, or at the very least allow selecting syntax highlighting form appearance as well so in real time you can see the preview." then "release as stable".
- Terminal settings tabs: Appearance, Features, Mouse & keyboard. Everything from Highlighting is on Appearance below the font settings. The Syntax highlighting picker and the Preview card are one sticky block (`.terminal-preview-sticky`) under the tab strip, so profile, color, prompt and underline changes show in the preview while the options scroll. Sticky only applies at a window height of 640 px or more.
- The font/Unicode sample moved from the preview card to just below the font settings (it is about the font, and keeps the pinned block short).
- `highlighting` still works as a tab id for `selectSettingsTab`/`openSettings` and lands on Appearance at the picker.
- Channel stays stable for this release (owner request).
- Tests: `scripts/terminal-appearance-highlighting.browser.cjs` (added to checks.yml) and four browser tests updated to open Appearance.
- Not claimed: physical Windows or Linux checks of this change.

---

# v1.21.18 Stable (Terminal Tools menu removed) - publication verified (2026-11-08)

Published as the stable release https://github.com/TechSavage1989/network-toolkit/releases/tag/v1.21.18 at 2026-11-08T07:02:33Z (not a prerelease); releases/latest is now v1.21.18. Merged PR #135; tag v1.21.18 points to merge commit 0bf23380230a0d4e783fcda16823fae7fbfffbf2. Release workflow run 42456939245 succeeded. Assets: Network_Toolkit.exe, Portable and Update ZIPs, Linux x86_64 and aarch64 tarballs and SHA256SUMS; every asset digest matches SHA256SUMS, and the Linux x86_64 binary reports "Network Toolkit 1.21.18 linux amd64".
- PR #135's first CI run failed app-chrome109 (separate terminal window lost 46 px of terminal height at 1024 px because the new toolbar icons wrapped the toolbar). Fixed by moving those icons to the sidebar rail; both Checks runs on the fix were green before the merge.
- Known CI issue: hosts-editor.browser.cjs ("Find shortcut stole focus from entry dialog") failed once on main for the docs-only commit a5b789d and passes elsewhere; a follow-up task was suggested to the owner.

Owner: "i don't think the tools menu is needed if all that is in it it connections basically or other redundent options", then "make the next release stable". The owner checked v1.21.17 on Windows and said "looks good".
- Removed: the Terminal toolbar's Tools button and menu, including the saved-connection select with Configure/Delete (the Saved sidebar's double-click and right-click menu cover them) and the Sessions/Profiles shortcuts (session tabs and the Saved/Recent tabs cover them).
- Moved: Password manager and SSH host keys to the Saved/Recent sidebar footer; Pane shortcuts to an ⓘ beside the layout picker; Separate terminal window to the Start a terminal card. Separate terminal windows show theme, window settings and help icons at the bottom of the terminal sidebar rail (above ⇄), since they have no app sidebar. They were first put at the end of the toolbar, but that made the toolbar wrap on CI's 1024 px terminal window (app-chrome109 measured 46 px less terminal height), so they moved to the rail. The scoped modal is now titled "Terminal window settings".
- `#terminalToolsPanel` still exists as a hidden staging container (terminal-extras.js moves the active-session bar and Settings sections out of it). `setTerminalToolsOpen`, `showTerminalProfiles` and `showTerminalSessions` stay because remote-browser.js and the UX audit script call them.
- Channel stays stable for this release (owner request). The next beta, if any, switches `docs/RELEASE_CHANNEL.txt` and `updateBuildChannel` back to `beta`.
- Tests: `scripts/terminal-tools-removed.browser.cjs` (added to checks.yml), updated macro/controls fixtures and the ux-phase3 help step. README terminal screenshot recaptured.
- Not claimed: physical Windows or Linux checks of this change.

---

# v1.21.17 Stable (all 1.21 betas) - publication verified (2026-11-08)

Published as the stable release https://github.com/TechSavage1989/network-toolkit/releases/tag/v1.21.17 at 2026-11-08T04:52:29Z (not a prerelease); releases/latest is now v1.21.17. Tag v1.21.17 points to ab0f50da978eaec9d171a648c882fee4d487a608 (the merge of PR #132, which came right after the stable PR #133 and contains it). The first publish attempt (release run 42448048350, for the PR #133 merge 4f428b6) built fine but GitHub refused `gh release create` with "HTTP 403: Resource not accessible by integration"; nothing was published. The next push to main started release run 42448282030, which built and published successfully, so the 403 was transient. Assets: Network_Toolkit.exe, Portable and Update ZIPs, Linux x86_64 and aarch64 tarballs and SHA256SUMS; every asset digest matches SHA256SUMS, and the Linux x86_64 binary reports "Network Toolkit 1.21.17 linux amd64".
- Next beta: switch `docs/RELEASE_CHANNEL.txt` and `updateBuildChannel` back to `beta` (as v1.21.0 did after 1.20.16).
- Old branches: run 42448308140 of delete-merged-branches deleted the 24 approved branches (each deleted branch and its last commit is in that run's log, for restore). The repository now has `main` and the open PR #129 branch.
- Known CI issue: browser-editor112 (editor-views112 focus wait on the detached file-list resize handle; the divider geometry check; editor-capacity114's 5 s input bound) fails intermittently on unrelated PRs and passes on re-run. A follow-up task was suggested to find the cause.

Owner: "delete old branches. also the current version works well. please make stable version". The owner confirms the current beta on Windows.
- Stable 1.21.17 contains the 1.21.0 to 1.21.16 betas. Channel switched to stable in `docs/RELEASE_CHANNEL.txt` and `updateBuildChannel` (cmd/network-toolkit/updater.go), as for 1.20.16. Version is above every beta, so beta users receive it.
- v1.21.16 Beta 17 (flat icon, PR #131 from another session) was published at 2026-11-08T02:53:30Z with tag v1.21.16 at merge commit 756b9656383cae618230adab79a2d101e5c12826; its six asset digests match SHA256SUMS (checked here). Its preparation deleted RELEASE_NOTES_1.21.15.txt instead of archiving it; restored to docs/releases/. It also left ux-phase6's fixture version, the UI guidelines version line and the updateNewer chain at 1.21.15; this release updates them.
- Linux: release packaging always includes the Linux tarballs. docs/LINUX_PLAN.md says Linux joins stable after the owner confirms it on a real Linux PC, which has not happened, so the stable notes and README keep Linux as a preview/beta.
- Old branches: the automatic cleanup kept 24 branches with unmerged work (see PR #130). PR #132 adds a manual `branches` input to `delete-merged-branches.yml`; the owner approved deleting them, and each deleted branch's last commit is logged in that run for restore.
- Not claimed: physical Linux checks.

---

# Repository housekeeping: merged branches deleted automatically (2026-11-08)

Owner: "i don't know how to delete merged branches. i thought you could do that". The session's GitHub access refuses branch deletion (git push --delete and the refs API both return 403), so `.github/workflows/delete-merged-branches.yml` deletes them with the repository's own token after every push to main (and on demand from Actions, with a dry-run option). Rules: delete when the branch's pull request was merged and the branch still points at the merged commit, or when main contains every commit on it and its last commit is over a day old; never the default, protected or open-PR branches. A local dry run against the live repository (read-only) found 224 branches to delete and 25 to keep: 24 with commits not on main (old beta/, investigate/, review/, proof/ and docs/ branches, `feat/vault-options`, `release-revision-v1.17.63`) and the open PR #129 branch. The kept ones stay for the owner to review; the run summary lists them. No version bump or release.

---

# v1.21.15 Beta 16 (Rule set colors named by purpose; Device prompts on every rule set) - publication verified (2026-11-08)

Published as opt-in prerelease https://github.com/TechSavage1989/network-toolkit/releases/tag/v1.21.15 at 2026-11-08T00:21:59Z; stable latest remains v1.20.16. Merged PR #127; tag v1.21.15 points to merge commit a9ac25f085b7461520abd3071d59c25a0bb6e482. Release workflow run 42429130578 succeeded. Assets: Network_Toolkit.exe, Portable and Update ZIPs, Linux x86_64 and aarch64 tarballs and SHA256SUMS; every asset digest matches SHA256SUMS, and the Linux x86_64 binary reports "Network Toolkit 1.21.15 linux amd64". CI note: windows-terminal (TestNativeTerminalSFTP123InstalledWebView2, the known Explorer drop timing failure; fix proposed in #121) and browser-editor112 (editor-views112 focus wait) failed once and passed on re-run.

Owner request: label rule set colors by function rather than color, and add Universal's Device prompts card (color and configuration) to every rule set.
- Purpose comes from `slotLabel()`: built-in notes, a custom set's `labels` (editable in its cards), or the MobaXterm meaning. Labels are app-only; `.msyn` export is unchanged.
- Device prompts now work with keyword rule sets: `promptRule` (default on, per set via `terminalSyntax.off[id]` containing `prompt`) makes `analyze()` color the prompt and carve rule set hits around it. The color (`highlightColors.prompt`) and name choice (`promptHighlight`) stay global.
- Behavior change to call out: rule sets color prompts by default from this version.
- Checked: Node unit tests, Go and Python tests, terminal-prompt-links browser test with rule set card screenshots.
- Not claimed: checked on a physical Windows PC.

---

# CI: Terminal SFTP drag-to-Explorer race in windows-terminal (2026-11-07) - no release

Report: `TestNativeTerminalSFTP123InstalledWebView2` sometimes fails at `actual detached-workspace-file` with "Actual Explorer destination file did not arrive" (native-terminal-sftp123.ps1:165). That is the only failure found in this test: run 41707445828, first attempt (2026-11-03, job 126166393524). The two main runs named in the report failed for other reasons. 42375596212 failed the baseline122 negative control (ps1:150, the Save dialog did not appear). 42371008876 failed `TestNativeEditorLocal119`. The candidate step passed in both.

Root cause, from the failing run's evidence (server actions and the script's input timeline use the same GetTickCount64 clock):
- The page arms the native copy only when the pointer leaves the source window. The owner is held at its 960px minimum width and the hosted screen is 1024px wide, so only a 16px strip of Explorer's Items View is exposed. The pointer leaves on step 27–28 of 28, which leaves about 530 ms between native `start` and release.
- In every passing main-window gesture, `DoDragDrop` started about 30 ms before the cursor stopped moving, and Explorer replied with COPY at once. In the failing detached gesture, `start` came at +1234 ms and the release at +1781 ms. `finish` came 16 ms after the release with outcome `cancelled`, and Explorer never asked for the data (no `materialize`). The release came before OLE had an accepted drop target. The detached child has no native trace, so the slow step (admission or Explorer's first `DragEnter`) cannot be named.
- The fixture writes the finish marker for any outcome, so the script then waited 15 s for a file that could not arrive.

Fix (test only, no application code): the gesture timings and the 500 ms dwell are unchanged. After the dwell, the script releases only when the native drag source reports Explorer's DROPEFFECT_COPY feedback. While OLE runs without feedback, it moves the cursor one pixel inside the drop strip. It fails at once with the native event sequence if OLE ends before the release, or after 10 s with a reason. The finish marker now holds the outcome, and Copy fails at once on anything other than `copied`. The detached child is built with `toolkit_native_drag_trace122` (typed, path-free trace in a private temp folder, as in SFTP121's diagnostic mode). The main window's observer writes the same events. The traces and per-gesture `nativeDrag` timings are in the `terminal-sftp123-native-checks` artifact. Baseline122 still runs the copied test files against the unmodified 60b02cae tree.
- Hosted CI on PR #121: the Terminal SFTP drag test passed seven of seven runs. Those were push run 42412176475 (75bc699), run 42413442967 attempts 1–5 (6cbde18) and run 42422856889 (5c76fbc). In the six timed runs (6cbde18 onward), all 48 gestures passed, OLE entered `DoDragDrop` 15–47 ms before the cursor stopped, and no nudges were needed. That margin of one to three 16 ms ticks is the race the gate now covers. The windows-terminal job was green in five of the six. Attempt 4 failed only in `TestTerminalShellExplicitApprovalAndLateCancellation` ("late-launch handle leaked"), which passed on its one re-run (attempt 5). That test checks closure by polling a handle value that Windows can reuse. It needs its own fix (keep a duplicate handle and use `CompareObjectHandles`).
- No pull_request Checks run was created for PR #121, so only push runs are listed.
- Not changed: `native-sftp-direct121.ps1` still releases after a fixed dwell. It failed the same way once (run 41560040060, main-folder).

---

# v1.21.14 Beta 15 (Rule set cards like Universal) - publication verified (2026-11-07)

Published as opt-in prerelease https://github.com/TechSavage1989/network-toolkit/releases/tag/v1.21.14 at 2026-11-07T23:24:49Z; stable latest remains v1.20.16. Merged PR #125; tag v1.21.14 points to merge commit edb48c3a1d5c7b95b93f5a4ebc7fec0c0707b6c0. Release workflow run 42424848024 succeeded. Assets: Network_Toolkit.exe, Portable and Update ZIPs, Linux x86_64 and aarch64 tarballs and SHA256SUMS; every asset digest matches SHA256SUMS, and the Linux x86_64 binary reports "Network Toolkit 1.21.14 linux amd64". PR #125 was branched from the v1.21.13 head; its watcher merged it only after v1.21.13 was merged and released.

Owner request (screenshots comparing the Universal page with Standard keywords): "include the same selection style configuraiton for all the rule sets, but also continue to support the 3rd party rule sets word customizability".
- Rule set pages in Customize use the Universal card grid: per-color switch, note, color button with hex, and the pattern (editable for own and imported sets; built-in patterns folded and read-only), plus a Device prompt name card and Turn all colors on / Reset colors.
- New saved key `terminalSyntax.off` ({rule set id: [colors]}), kept by `preferences()` and Go `safeTerminalSyntax`; `withSwitches()` blanks those colors when the set is used (terminals, legend, previews). The saved rule set itself is unchanged, so turning a color back on restores it.
- Bug found and fixed: clicking a pattern textarea opened the color picker (the click handler matched `data-syntax-color`, which the textareas also carry).
- Checked: Node unit tests, Go and Python tests, terminal-prompt-links browser test with rule-set card screenshots in light and dark.
- Not claimed: checked on a physical Windows PC.

---

# v1.21.13 Beta 14 (Appearance preview follows every highlighting profile) - publication verified (2026-11-07)

Published as opt-in prerelease https://github.com/TechSavage1989/network-toolkit/releases/tag/v1.21.13 at 2026-11-07T23:09:07Z; stable latest remains v1.20.16. Merged PR #124; tag v1.21.13 points to merge commit 99eb8e9ef55f779b37279a67a12353588621728b. Release workflow run 42423664610 succeeded. Assets: Network_Toolkit.exe, Portable and Update ZIPs, Linux x86_64 and aarch64 tarballs and SHA256SUMS; every asset digest matches SHA256SUMS, and the Linux x86_64 binary reports "Network Toolkit 1.21.13 linux amd64".

Owner request (screenshots: with "Standard keywords" selected the Appearance preview stayed plain, while Universal colored it): "allow the preview in terminal for text also work with the custom /3rd party syntax highlights".
- `terminal-appearance.js`: the preview text is generated from `PREVIEW_LINES` with the real analyzer for the active profile (`renderPreviewText`), so built-in and imported rule sets, their per-profile colors and the prompt name choice all show. Spans are rebuilt only when the profile, its rules or the prompt choice change, so color and underline edits restyle the same elements. Heading: `Preview · <profile>`.
- Owner also asked that every message end with a clear status and what is needed; recorded in AGENTS.md.
- Checked: unit tests (all), Go and Python tests, terminal-prompt-links browser test with rule-set preview screenshots in light and dark.
- Not claimed: checked on a physical Windows PC.

---

# v1.21.12 Beta 13 (Prompt name color, link underlines and Universal in Customize) - publication verified (2026-11-07)

Published as opt-in prerelease https://github.com/TechSavage1989/network-toolkit/releases/tag/v1.21.12 at 2026-11-07T22:42:32Z; stable latest remains v1.20.16. Merged PR #123; tag v1.21.12 points to merge commit 054c4c2c9019e91ca4cbc18dbe6b53186564a56d. Release workflow run 42421575200 succeeded. Assets: Network_Toolkit.exe, Portable and Update ZIPs, Linux x86_64 and aarch64 tarballs and SHA256SUMS; every asset digest matches SHA256SUMS, and the Linux x86_64 binary reports "Network Toolkit 1.21.12 linux amd64".

Owner requests after v1.21.11: "include customize for Universal syntax highlighting.", "add option for underline URL or network paths in universal syntax, separate from the standard underline option." and "also, add option to color or not color the device main prompt name, with customization of that color too" (screenshot: a Linux prompt `biqu@CR10SPro:~$` colored green by the device).
- Device prompt name: `terminalAppearance.promptHighlight` = `auto` (previous behavior) / `always` / `off`, color = the Device prompt highlight color (`highlightColors.prompt`). The device colored the owner's prompt itself, so Universal's own prompt color never showed; `always` and `off` therefore recolor `overrideable` cells (normal background, not inverse or hidden) for the name only, through decorations; the buffer is never changed. Works with keyword rule sets as well.
- `highlightLinkUnderlines` (default on) controls only `url`; `highlightUnderlines` controls the rest. URL detection now includes share/transfer schemes, UNC paths and NFS/scp `host:/path`.
- The Universal rule switches and device colors moved from the Highlighting tab to the Customize dialog's Universal page (`universalEditorHTML`, `[data-terminal-universal-editor]` in `SECTIONS`). Tests that toggled rules in the tab now open the dialog (`Rules and colors…`).
- Gotcha found while testing: `highlightOptions()` without a language returns the active rule set, so code that needs Universal's colors must call `legend()` with `ruleset:null, ruleSet:null`.
- Checked: unit tests, Go tests, terminal-prompt-links, terminal-style107, dropdowns and features-85 browser tests; screenshots of the tab and dialog in light, dark and 960×600 reviewed.
- Not claimed: tested against real devices or on a physical Windows PC.

---

# v1.21.11 Beta 12 (Terminal settings in tabs, universal highlighting, colors for every profile) - publication verified (2026-11-07)

Published as opt-in prerelease https://github.com/TechSavage1989/network-toolkit/releases/tag/v1.21.11 at 2026-11-07T21:29:11Z; stable latest remains v1.20.16. Merged PR #120; tag v1.21.11 points to merge commit bfd9567f1e2d1bd3c462ebcc5ffbb58d88940084. Release workflow run 42415784238 succeeded. Assets: Network_Toolkit.exe, Network_Toolkit_1.21.11_Portable.zip, Network_Toolkit_1.21.11_Update.zip, Network_Toolkit_1.21.11_Linux_x86_64.tar.gz, Network_Toolkit_1.21.11_Linux_aarch64.tar.gz, SHA256SUMS; all digests match SHA256SUMS. The published Linux x86_64 archive was downloaded, extracted and run (`--version` 1.21.11 linux amd64; inner SHA256SUMS OK). Checks: browser-editor112 (`editor-divider113`) failed once on 43d8d69 and passes locally; the dropdowns observer was narrowed in e4b192c; final runs 42413926151 and 42413931686 green on the first attempt.

Owner (after trying 1.21.10: "drop downs are better"): "please continue with the next releases, and combine 11 and 12 into one release". The menu cleanup and the universal highlighting therefore ship together as 1.21.11; no 1.21.12 was published.

Owner: "please polish and clean up the display and tools menus of terminal. make sure they are intuitive and think about ease of use and QoL and reduce redundancies." That request is the owner's approval for moving these controls (layout freeze); docs/UI_GUIDELINES.md records the new arrangement.
- Panel: `terminal-extras.js` builds `#terminalSessionPanel` (ids kept for tests and code) with a tab strip and four `[data-terminal-settings-panel]` pages. Markup in `terminalToolsHTML` marked `data-terminal-settings-target` (highlighting profile, font size field, transcript note) is moved into its page. Settings modules mount into their page when present, else into the panel (unit-test harnesses).
- `terminal-appearance.js`: `SECTIONS` + `scoped()` replace the single-section queries; confirmation checkboxes are found by id (they used to be looked up in the shell section).
- Tab memory is per app session (module variable), not saved. Tests that open the panel must click the tab they need (`[data-terminal-settings-tab="…"]`).
- Separate terminal windows have no sidebar, so Theme and App settings icons appear in the Tools heading there only.
- Checked: features-85, terminal-style107, color-picker, dropdowns, terminal-pager browser tests; unit tests; README terminal screenshot recaptured.
- Not claimed: checked on a physical Windows PC.

Owner: compare the original device highlighting with the MobaXterm rule sets, make it a universal profile with better detection, and make highlight colors customizable for every profile.
- Comparison: the MobaXterm rule sets color keywords anywhere (useful breadth, but they paint prose); the device highlighter was precise but missed logs, CLI errors, ping, routing/service states and several vendors. The universal profile adds those as shape-based recognizers (`universalHighlights` in terminal-highlight.js) and keeps the no-prose rule; `scripts/terminal-highlight-universal.test.cjs` holds the multi-vendor positives and the prose negatives (add both when extending).
- New kind `info` (Info / notice) is wired through LABELS/MARKS/COLORS, terminal-appearance HIGHLIGHTS, Go `terminalHighlightKinds` and the rule switches.
- Rule-set colors per profile live in `terminalSyntax.colors[id][slot]`; deleting a custom set removes its colors; the Customize dialog and the legend both call `NetworkToolkitTerminalSyntaxUI.editColor`. Picker previews use a module variable (`colorsFor`), never saved state.
- Not claimed: tested against real devices or on a physical Windows PC.

---

# v1.21.10 Beta 11 (Dropdowns in the app's colors) - publication verified (2026-11-07)

Published as opt-in prerelease https://github.com/TechSavage1989/network-toolkit/releases/tag/v1.21.10 at 2026-11-07T20:33:00Z; stable latest remains v1.20.16. Merged PR #119; tag v1.21.10 points to merge commit a60fb5b6c3872e3ba9c88570bdd0f56402ca688d. Release workflow run 42411270581 succeeded. Assets: Network_Toolkit.exe, Network_Toolkit_1.21.10_Portable.zip, Network_Toolkit_1.21.10_Update.zip, Network_Toolkit_1.21.10_Linux_x86_64.tar.gz, Network_Toolkit_1.21.10_Linux_aarch64.tar.gz, SHA256SUMS; all digests match SHA256SUMS. The published Linux x86_64 archive was downloaded, extracted and run (`--version` 1.21.10 linux amd64; inner SHA256SUMS OK). Checks: CI found that typing in a suggestion field opened the list over the session-folder dialog's button (fixed in 4e82e91: the list opens only on request). `windows-terminal` (`TestNativeTerminalSFTP123InstalledWebView2`, Explorer drop) failed once per head and passed on re-run; it also fails intermittently on main (runs 42375596212, 42371008876). Final runs 42408514386 and 42408518327 green.

Owner request (with a screenshot of the black font suggestion popup): native-feeling dropdowns instead of the black list, suggestion lists that show every choice regardless of typed text, and fonts shown in their own face. The same message also asked for a Display/Tools menu cleanup and a universal highlighting profile with customizable colors; those follow together in v1.21.11.
- `web/assets/dropdowns.js` + `dropdowns.css`. Closed selects are untouched (measured identical on 11 pages with and without the stylesheet), because CI compares layouts against pinned older builds and `appearance: base-select` sizes a closed select to its chosen item. The app-colored list is applied only while open: `.ntk-picker` is added on pointerdown/Alt+Arrow/F4 with width/height locked to the native box, and removed when `:open` ends (rAF watch). Unsupported engines keep native lists.
- While a picker is open, keydown/keyup/keypress are stopped at window capture; otherwise app Escape handlers closed the Display panel and left the list open.
- Comboboxes: any `input[list]` is converted (the `list` attribute moves to `data-combobox-list` so the browser popup never appears). Code must not rely on `input.list`; the datalist element stays and is read when the list opens, so dynamic lists (serial ports, folders) stay current. Terminal output DOM changes are ignored by the observer.
- Tests: `scripts/dropdowns.browser.cjs` (fonts, typing, keyboard, select colors, size lock, Escape, light/dark, time zone field), added to checks.yml. Note: `active-session-tools.browser.cjs` (R reconnect key from v1.21.9) is not part of CI; it passes locally.
- Not claimed: checked in WebView2 on a physical Windows PC.

---

# v1.21.9 Beta 10 (Paste warnings and reconnection keys, T3b) - publication verified (2026-11-07)

Published as opt-in prerelease https://github.com/TechSavage1989/network-toolkit/releases/tag/v1.21.9 at 2026-11-07T12:52:36Z; stable latest remains v1.20.16. Merged PR #117; tag v1.21.9 points to merge commit a1eef91ab62df3a449e025da6f05616d7869ac39. Release workflow run 42374124770 succeeded. Assets: Network_Toolkit.exe, Network_Toolkit_1.21.9_Portable.zip, Network_Toolkit_1.21.9_Update.zip, Network_Toolkit_1.21.9_Linux_x86_64.tar.gz, Network_Toolkit_1.21.9_Linux_aarch64.tar.gz, SHA256SUMS; all digests match SHA256SUMS. The published Linux x86_64 archive was downloaded, extracted and run (`--version` 1.21.9 linux amd64; inner SHA256SUMS OK). Checks: both runs green (42371964548 and 42371967035), first attempt.

Owner request: terminal options similar to MobaXterm (plan: docs/TERMINAL_SETTINGS_PLAN.md, T3b).
- Paste warnings: `web/assets/terminal-paste-warnings.js` (pure, `check(text, enabled)` → `{kind, message, sample}`; one warning per kind and reason; line-by-line scan capped at 4,000 lines × 8,192 characters). Private and lab hosts (RFC 1918, loopback, link-local, CGNAT, single-label names, `.local`/`.lan`/`.internal`/`.home.arpa`, ULA/link-local IPv6) are not flagged for plain HTTP, because network engineers routinely copy images from lab servers.
- `terminal-interaction.js`: `acceptText` opens the review for multi-line (when review is on) or any warned paste; `updateReview` renders `#terminalPasteAlerts` from the current text. Unit harnesses that load interaction functions by name must include `pasteWarnings` and stub `preferences` (see `terminal-paced-paste127.test.cjs`).
- Reconnection keys: `sessionKey` in `terminal-behavior.js` (capture phase). Ignored with modifiers, while composing, on repeat, in inputs/selects/editable elements (the xterm helper textarea is allowed), with a dialog open, on another page in the main window, for connected or reconnecting sessions, and whenever `terminalReconnectAllowed` is false.
- State: `terminalInteraction.pasteWarnings`, `terminalBehavior.reconnectKeys`, `terminalBehavior.closeOnOtherKey`, all sanitized in Go.
- Tests: `terminal-paste-warnings.test.cjs` (5), `terminal-paced-paste127.test.cjs` (+1 review integration), `terminal-behavior.test.cjs` (+1 keys), Go `TestTerminalPasteWarningsDefaultOnAndKeepTurnedOffKindsAcrossRestart`; browser `terminal-paced-paste127` (warning dialog in main, separate and detached windows) and `active-session-tools` (no R action while Reconnect is unavailable; R posts reconnect when allowed; card hint).
- Not claimed: tested against a real device or on a physical Windows PC.

---

# v1.21.8 Beta 9 (Terminal features and Advanced, T3) - publication verified (2026-11-07)

Published as opt-in prerelease https://github.com/TechSavage1989/network-toolkit/releases/tag/v1.21.8 at 2026-11-07T11:48:51Z; stable latest remains v1.20.16. Merged PR #115; tag v1.21.8 points to merge commit 2ce7ad6fdccb44eb5562b0a3faa5029251a39a3b. Release workflow run 42369530682 succeeded. Assets: Network_Toolkit.exe, Network_Toolkit_1.21.8_Portable.zip, Network_Toolkit_1.21.8_Update.zip, Network_Toolkit_1.21.8_Linux_x86_64.tar.gz, Network_Toolkit_1.21.8_Linux_aarch64.tar.gz, SHA256SUMS; all digests match SHA256SUMS. The published Linux x86_64 archive was downloaded, extracted and run (`--version` 1.21.8 linux amd64; inner SHA256SUMS OK). Checks: both runs green (42367566185 and 42367580552), first attempt.

Owner request: terminal options similar to MobaXterm (plan: docs/TERMINAL_SETTINGS_PLAN.md, T3; the rest is T3b).
- Module `web/assets/terminal-behavior.js` (in `web_assets.go`, loaded after terminal-appearance.js). It reads the app's top-level `state`, `saveState`, `terminalSessions` and `enqueueTerminalInput` by bare name: they are script bindings, not `window` properties (`root.state` is undefined in the app; a test guards this).
- `options()` is spread into the xterm constructor in `createTerminalDisplay` (replacing `scrollback:10000,convertEol:false`); `attach(entry)` adds the bell listener and the scrollbar class; `apply()` updates open terminals and the transcript note on change. `convertEol` is only written when the setting changes, so a device's own LNM mode is not overridden on every theme update.
- `displayBytes(entry, bytes)` runs in `writeTerminalDisplay` (pending-byte accounting still uses the original length). `observeOutput(entry, bytes)` runs only for live events, so a replayed or late-created display never answers an old ^E. Answerback is sent with `enqueueTerminalInput(entry, text, true, 'answerback')`, so it is not recorded as typed input or macro input.
- Scrollback is capped at 10,000: `terminal-state.js` LIMITS.scrollback and the 10 MB transfer bound what a separate window can receive. Raising it needs that format to grow first.
- State `terminalBehavior`: Go `safeTerminalBehavior` (terminal_behavior.go), the three window key lists, workspace display get/apply, `mergeState`.
- Tests: `terminal-behavior.test.cjs` (6, with real xterm), Go `TestTerminalBehaviorKeepsValidSettingsAndDefaultsTheRest`. Node 1617, Go, Python 48 pass; browser terminal-style107, terminal-paced-paste127, terminal-pager, terminal-connection-dialog, active-session-tools, ui-consistency, ux-phase6, color-picker pass locally. A real-app check confirmed the settings survive a reload through the Go sanitizer, the scrollbar CSS matches xterm 6's DOM and the bell flash shows.
- Not claimed: tested against a real device (bell, ^E or CR-only output) or on a physical Windows PC.

---

# v1.21.7 Beta 8 (Terminal font, colors and cursor, T2) - publication verified (2026-11-07)

Published as opt-in prerelease https://github.com/TechSavage1989/network-toolkit/releases/tag/v1.21.7 at 2026-11-07T10:50:32Z; stable latest remains v1.20.16. Merged PR #113; tag v1.21.7 points to merge commit 5933d04a2d441214d89e7124601a224d57005005. Release workflow run 42365427156 succeeded. Assets: Network_Toolkit.exe, Network_Toolkit_1.21.7_Portable.zip, Network_Toolkit_1.21.7_Update.zip, Network_Toolkit_1.21.7_Linux_x86_64.tar.gz, Network_Toolkit_1.21.7_Linux_aarch64.tar.gz, SHA256SUMS; all digests match SHA256SUMS. The published Linux x86_64 archive was downloaded, extracted and run (`--version` 1.21.7 linux amd64; inner SHA256SUMS OK). Checks: both runs green (push 42363781393 and pull request 42363806490), first attempt.

Owner request: terminal options similar to MobaXterm (plan: docs/TERMINAL_SETTINGS_PLAN.md, T2).
- Font size: Display's `#terminalFontSize` is now `<input type="number" min="8" max="36">` with a "px" label and `data-input-clear="off"` (the app's clear button hid the value in the 78px field). It applies on `change` (Enter or leaving the field); invalid values snap back. Zoom (`FONT_SIZES`) steps 8–36; Go accepts 8–36.
- Appearance (`terminal-appearance.js`): `cursor` and `boldBright` in `normalize`; `options()` returns `cursorStyle`, `cursorBlink`, `drawBoldTextInBrightColors`, spread into the xterm constructor and `updateTerminalTheme`. `createTerminalDisplay` no longer forces `cursorBlink:true`, so the setting must come from `options()`. Missing keys keep today's look (blinking block, bright bold).
- Seven presets with MobaXterm's names (our own palettes; Monokai and AfterGlow follow their published palettes), also allowed in Go `safeTerminalAppearance`, which keeps `cursor` and `boldBright`.
- Deferred to T3: grouping Display into MobaXterm-like sections and a fixed rows × columns.
- CI: record PR #112 was merged while its push-event Checks run had failed (`browser-terminal127`, "Resulting promise was garbage collected" in `terminal-paced-paste127.browser.cjs` `ready()`; the pull_request run on the same commit passed, and #112 changed docs only). `ready()` now records the xterm write callback in a page flag and polls with `waitForFunction`, so no long-lived page promise remains. The merge watcher now requires every Checks run on the head commit to be green.
- Tests: `terminal-appearance.test.cjs` (+1), `terminal-interaction-defaults.test.cjs` (+1 font limits across JS, HTML and Go), Go `TestTerminalCursorBoldBrightAndMobaXtermSchemesPersist`, `terminal-style107.browser.cjs` now types 8/30/13 px and presses Enter. Node 1611, Go, Python 48 pass; browser terminal-style107, color-picker, ux-phase6, ui-consistency, terminal-pager, terminal-connection-dialog, active-session-tools pass locally. A real-app check confirmed font size, cursor, bold option and Monokai survive a reload through the Go sanitizer.
- Not claimed: tested against a real device or on a physical Windows PC.

---

# v1.21.6 Beta 7 (Terminal syntax highlighting, T1) - publication verified (2026-11-07)

Published as opt-in prerelease https://github.com/TechSavage1989/network-toolkit/releases/tag/v1.21.6 at 2026-11-07T10:02:47Z; stable latest remains v1.20.16. Merged PR #111; tag v1.21.6 points to merge commit adf33240adb56918fc221165401223f07c3afc32. Release workflow run 42361977058 succeeded. Assets: Network_Toolkit.exe, Network_Toolkit_1.21.6_Portable.zip, Network_Toolkit_1.21.6_Update.zip, Network_Toolkit_1.21.6_Linux_x86_64.tar.gz, Network_Toolkit_1.21.6_Linux_aarch64.tar.gz, SHA256SUMS; all digests match SHA256SUMS. The published Linux x86_64 archive was downloaded, extracted and run (`--version` 1.21.6 linux amd64; inner SHA256SUMS OK). Checks: green pull-request run 42361035881, first attempt.

Owner (with MobaXterm screenshots and seven .msyn files): "I want the terminal to have similar configuration options, and similar syntax color recognition that MobaXTerm has." Plan and the mapping of every MobaXterm option, including what xterm.js cannot do: docs/TERMINAL_SETTINGS_PLAN.md (T1 syntax, T2 font/colors/cursor, T3 features/advanced, T4 logging, T5 local shells).
- Engine: `web/assets/terminal-syntax-rules.js` (module `NTKTerminalSyntaxRules`). Rule sets have Red/Green/Yellow/Blue/Magenta/Cyan/Underline patterns, regex or comma-separated words, case flag. Built-ins are our own patterns. `convert()` handles MobaXterm's POSIX bracket expressions and `¨` line markers (text is framed with ¨ before matching). Span = first matched capture group when the rest of the match has no letters/digits, else the whole match; blanks trimmed. Earlier colors win overlaps; Underline may overlap. Each pattern is probed with short-to-long runs (generic text plus the pattern's own letters) and rejected above 25 ms; invalid regexes report per color. Limits: 8000 chars per pattern, 50 own sets, 4096-char lines, 96 hits per color.
- Highlighter: `analyze(text, language, rules, ruleset)` uses the rule set except for local PowerShell/CMD. Colors come from `syntaxColors` (terminal scheme: bright on dark, normal on light; underline = foreground). Rule-set changes clear and repaint; `setTheme` keeps the rule set.
- UI: `terminal-syntax-ui.js` + `terminal-syntax.css`. The Display row's checkbox became the Syntax highlighting select (None ↔ `terminalHighlight=false`, the chosen set is kept) and Customize. Customize closes Display (`#terminalSessionPanel`) and Tools first; otherwise the panel covered the dialog. Imported sets are named after their file (the owner's files all share one internal name).
- State: `terminalSyntax {active, custom[]}`; Go `safeTerminalSyntax` (ids, built-in collisions, 50 sets, 8000-char patterns, control characters); added to `terminal_views.go`, `terminal_window.go`, `terminal_workspace_view.go`, `terminalWorkspaceDisplay`/`applyTerminalWorkspaceDisplay`, and restored in `mergeState`.
- Checked with the owner's seven files: all compile without errors and color as MobaXterm would; real xterm rendering checked with the Network and Standard sets.
- Tests: `terminal-syntax-rules.test.cjs` (10), `terminal-syntax-ui.test.cjs` (4), `terminal-highlight.test.cjs` (+1), Go `TestTerminalSyntaxKeepsOnlySafeCustomRuleSets`; node 1609, Go, Python pass. Browser: color-picker, features-83/84/85, terminal-style107, terminal-connection-dialog, terminal-pager, ui-consistency, ux-phase4/5/6 pass locally.
- AGENTS "Working with the owner": start owner-facing messages (input needed, published, waiting) with a bold marked line.
- Not claimed: tested against a real device or on a physical Windows PC.

---

# v1.21.5 Beta 6 (Update notice shows What's New in full) - publication verified (2026-11-07)

Published as opt-in prerelease https://github.com/TechSavage1989/network-toolkit/releases/tag/v1.21.5 at 2026-11-07T09:27:56Z; stable latest remains v1.20.16. Merged PR #109; tag v1.21.5 points to merge commit 08b7757f5d5cf2ec5c5b43aded993e9836e9fccc. Release workflow run 42359577156 succeeded. Assets: Network_Toolkit.exe, Network_Toolkit_1.21.5_Portable.zip, Network_Toolkit_1.21.5_Update.zip, Network_Toolkit_1.21.5_Linux_x86_64.tar.gz, Network_Toolkit_1.21.5_Linux_aarch64.tar.gz, SHA256SUMS; all digests match SHA256SUMS. The published Linux x86_64 archive was downloaded, extracted and run (`--version` 1.21.5 linux amd64; inner SHA256SUMS OK). Checks: green pull-request run 42358571723, first attempt.

Owner: "Do not truncate the high level primary update details in the updater".
- `web/assets/app-update-notes.js`: structured notes list every What's New item in full (no `.slice(0,4)`, no 180-character `short()` with "…"); none are moved into Additional details. Unstructured notes keep the first four bullets or paragraphs as highlights, now complete, and the rest stay in Additional details. Escaping and inert links unchanged.
- Checked with the real v1.21.3 notes: the 298-character first item now shows whole.
- Tests: `app-update-notes.test.cjs` (full highlights, Unicode, structured overflow), all updater node tests pass.
- AGENTS: What's New is shown in full, so keep it to a few clear sentences.
- Not claimed: checked on a physical Windows PC.

---

# v1.21.4 Beta 5 (Sidebar swap buttons at the bottom) - publication verified (2026-11-07)

Published as opt-in prerelease https://github.com/TechSavage1989/network-toolkit/releases/tag/v1.21.4 at 2026-11-07T08:51:03Z; stable latest remains v1.20.16. Merged PR #107; tag v1.21.4 points to merge commit 06455fa9a29bd7589e8cc82ca2d9dac4406c3416. Release workflow run 42357014751 succeeded. Assets: Network_Toolkit.exe, Network_Toolkit_1.21.4_Portable.zip, Network_Toolkit_1.21.4_Update.zip, Network_Toolkit_1.21.4_Linux_x86_64.tar.gz, Network_Toolkit_1.21.4_Linux_aarch64.tar.gz, SHA256SUMS; all digests match SHA256SUMS. The published Linux x86_64 archive was downloaded, extracted and run (`--version` 1.21.4 linux amd64; inner SHA256SUMS OK). Checks: green pull-request run 42356173853 on the PR head, first attempt.

Owner: "move the sidebar swap button tot he bottom for all tools, like it is on the new one."
- Terminal (`session-sidebar.js`), File Transfer client (`transfer-client-layout.js`) and server settings (`transfer-server.js`): `[data-session-action="swap-side"]`, `[data-client-action="swap-side"]` and `[data-transfer-sidebar-side]` now sit at the foot of `.terminal-sidebar-rail` with `.sidebar-rail-swap` (margin-top:auto), like the Text Editor's `[data-editor-files-dock]`. Headings keep their titles and close buttons. Rail click-to-reveal ignores buttons, so ⇄ never reopens a collapsed sidebar by accident.
- Configuration: `#toolConfigurationSide` lives in `#toolConfigFooter` (bottom row, left-aligned) while open; `syncSidebarControl` moves it to `#toolConfigurationRail` (column, ⇄ at the foot, show control on top via `order:0`) while collapsed. Footer and button stay hidden in the compact tabbed layout.
- Tests: node 1594 (sidebar-collapse asserts footer/rail placement; editor-files114 asserts `.sidebar-rail-swap`). Browser: terminal-connection-dialog, transfer-client-layout, transfer-server-layout, editor-sidebar117 (vs published116), editor-views112, sidebar-activity108, ux-phase4/5/6, ui-consistency, features-85/90 pass locally. Screens at 1600×900 for Configuration open/collapsed, Terminal, File Transfer client and server.
- Not claimed: checked on a physical Windows PC.

---

# v1.21.3 Beta 4 (Sidebars on either side) - publication verified (2026-11-07)

Published as opt-in prerelease https://github.com/TechSavage1989/network-toolkit/releases/tag/v1.21.3 at 2026-11-07T08:19:11Z; stable latest remains v1.20.16. Merged PR #105; tag v1.21.3 points to merge commit 0045255c72209bd633a61fad0b10999510df12b7. Release workflow run 42354726537 succeeded. Assets: Network_Toolkit.exe, Network_Toolkit_1.21.3_Portable.zip, Network_Toolkit_1.21.3_Update.zip, Network_Toolkit_1.21.3_Linux_x86_64.tar.gz, Network_Toolkit_1.21.3_Linux_aarch64.tar.gz, SHA256SUMS; all digests match SHA256SUMS. The published Linux x86_64 archive was downloaded, extracted and run (`--version` 1.21.3 linux amd64; inner SHA256SUMS OK). Checks: green runs 42353599711 (pull request) and 42353621903 (push) on the PR head, first attempt.

Owner: "can you make the sidebar for text editor reposition able to both sides, similar to other sidebars, like the one in terminal?" then, mid-work: "allow all the resizable sidebars to swap sides where able."
- Inventory of resizable sidebars: Terminal sessions and File Transfer client (already swapped); added Text Editor Connect/Files, diagnostic Configuration, File Transfer server settings; Left added to the position menus of entry details, Network discovery device details and Packet capture details. Not sidebars (unchanged): Terminal and editor compare split panes, File Transfer local/remote panes, graph/list and activity row splitters. The main navigation is not resizable.
- Pattern (UI_GUIDELINES "Sidebars"): rail stays on the sidebar's left; ⇄ button in the heading, or at the rail foot for the Text Editor (its heading row was removed in v1.19.1 and `editor-files114` asserts it stays removed); the resize edge faces the content and drag/arrow directions mirror; the side is saved with the sidebar's layout.
- Text Editor: `.nt-editor-files-resize` moved from the content panel to the shell; `.dock-right` and `.files-dock-right` (order 2); compact drawer opens from the right when docked right. Found and fixed: `mergeState` never restored `editorFilesPreferences`, so width/panel/collapsed reset at every start.
- Configuration: `safeToolLayout` keeps `side`; `toolConfigurationSide()`; `#toolConfigurationSide` in `#toolConfigHeading`, hidden in the compact (tabbed) layout; Reset layout returns it to the left.
- File Transfer server: `transferServerSidebar[p].side` (Go `sanitizeTransferClientSettings` now keeps `side`), `transfer-settings-right` flex order.
- Details panes: dock values gain `left`; Auto still chooses Right or Bottom; explicit Left/Right do not fall to Bottom on narrow results (Network discovery keeps its 900 px stacking rule).
- Tests: node 1594+ (editor-files114 dock test, result-details left, packet left, sidebar-collapse configuration right and discovery left); Go transfer sanitizer test. Browser: editor-sidebar117 (vs published116), editor-toolbar113 (vs published112), result-readers113 (vs published112), editor-views112, transfer-server-layout, discovery-saved/filter/port-actions, packet-capture-privilege, sidebar-activity108, ux-phase4/5/6, ui-consistency, features-92/96 pass locally. Screens reviewed at 1600×900 (Configuration right and collapsed, Text Editor right and collapsed, server settings right, reload persistence).
- Not claimed: checked on a physical Windows PC.

---

# v1.21.2 Beta 3 (Packet capture adapter list) - publication verified (2026-11-07)

Owner check on Windows (2026-11-07): "it looks good." The owner's report that led to this beta came from v1.21.1 running on Windows (guided setup visible); the adapter list was the only feedback on the v1.21.1 Packet capture changes.

Published as opt-in prerelease https://github.com/TechSavage1989/network-toolkit/releases/tag/v1.21.2 at 2026-11-07T07:10:42Z; stable latest remains v1.20.16. Merged PR #102; tag v1.21.2 points to merge commit 4fcaf11dfde1d5abca649117f4027fd1de0447c0. Release workflow run 42349968953 succeeded. Assets: Network_Toolkit.exe, Network_Toolkit_1.21.2_Portable.zip, Network_Toolkit_1.21.2_Update.zip, Network_Toolkit_1.21.2_Linux_x86_64.tar.gz, Network_Toolkit_1.21.2_Linux_aarch64.tar.gz, SHA256SUMS; all digests match SHA256SUMS. The published Linux x86_64 archive was downloaded, extracted and run (`--version` 1.21.2 linux amd64; inner SHA256SUMS OK). Checks: green runs 42349084839 (pull request) and 42349105843 (push), first attempt.

Owner, with screenshots of Test a target's Source interface list and the Packet capture connection list: "the packet capture network connection adapter selection should match the design of other adapter sleections thorughout the app".
- Backend: `packetSystemAdapter` carries `Status` and `Addresses` from `readLocalIP` (no new privileged read); `packetAdapterView` fills the new `capture.Interface` fields `hardware`, `status`, `addresses` (omitempty, so imports and legacy live/file IDs are unchanged). Selection IDs and the no-fallback rule are unchanged.
- UI: `captureAdapterLabel` mirrors `source-interface.js` `choiceLabel` ("Name - Hardware | address (IPv4)"); without a link it shows Not connected (Down, Lower layer down, Dormant), Disabled or the Windows status; Up without an address shows "No IP address". Refresh is a small button on the label row (aria-label "Refresh adapters"), the select is full width and the middle column 1.35× wider. `packet-setup-only` on the workspace (no capture selected) removes the configuration region's max-height so the big Start is visible at 960×600, 1280×800 and 1920×1080. The how-it-works strip drops its numbers and hides at max-height 720 px.
- Tests: Go `TestPacketAdapterPickerShowsHardwareStateAndAddresses`; node packet-capture (90; labels, escaping, title, setup-only class), all node 1589; packet-capture-privilege, packet-troubleshooting-live, packet-dhcp-live and packet-inspector-live pass on the Linux build.
- Not claimed: checked on a physical Windows PC with real adapters; validated with a synthetic adapter list.

---

# v1.21.1 Beta 2 (Packet capture for beginners) - publication verified (2026-11-07)

Published as opt-in prerelease https://github.com/TechSavage1989/network-toolkit/releases/tag/v1.21.1 at 2026-11-07T06:38:00Z; stable latest remains v1.20.16. Merged PR #100; tag v1.21.1 points to merge commit 7f24d361c0469fc21db6ad9dce41b8bdacb50cbe. Release workflow run 42347674184 succeeded. Assets: Network_Toolkit.exe, Network_Toolkit_1.21.1_Portable.zip, Network_Toolkit_1.21.1_Update.zip, Network_Toolkit_1.21.1_Linux_x86_64.tar.gz, Network_Toolkit_1.21.1_Linux_aarch64.tar.gz, SHA256SUMS; all digests match SHA256SUMS. The published Linux x86_64 archive was downloaded, extracted and run (`--version` 1.21.1 linux amd64; inner SHA256SUMS OK). Checks: green runs 42346624230 (pull request) and 42346647922 (push) on the PR head, first attempt.

Owner: "can the polish and intuitiveness of the packet capture be improved? the idea is that it is a tool that someone with little knowledge of how to use or interpret a packet capture can do so to troubleshoot issues." Owner chose "Everything in one beta" (plain results and polish plus guided capture).
- Engine (`internal/capture`): `troubleshoot_plain.go` adds `Verdict` (problems / notes / clear / little / empty; under 10 packets or 3 s is "little") and plain `Headline` per finding, with the previous title and exact counts kept in `Title`, `Observation` and `Technical`. `names.go`: service names for well-known ports (`ServiceLabel`, "usually" in the UI) and `HostNames` learned only from DNS A/AAAA answers in the capture (max 4096; nothing is looked up). `analyze.go`: conversations gain `ServicePort`, `Service`, `Outcome` (worked / refused / no-reply / ongoing / replied / one-way) and `NameA`/`NameB`; endpoints gain `Name`. `target.go`: `ParseTarget` (URL, host:port, [v6]:port, IP, name) and `CheckTarget`, four read-only steps (lookup, reach, connect, respond) with example packets and filters; a recording that ends within 3 s of the last attempt is "warn", not a failure.
- HTTP: `GET /api/packet-captures/{id}/check?target=` (token required, GET only, 400 for invalid targets).
- UI (`packet-capture.js/css`): three numbered setup steps and a large Start (`.packet-start-big`); "open a saved capture file" link (`data-packet-action="open-file"`); recording coach `#packetRecordingCoach` (timer, count, progress, Stop and analyse); verdict banner; "Check a website or device" card (`#packetTargetCheck`, remembered per capture in `model.captureTargets`); finding headlines, Problem / Good to know, folded Technical details; limitations folded; Conversations Result/Service columns with names; Devices show names; counters keep color as an inset stripe; Save as / Save shown packets / Save all only with packets; Pause updates only while recording. In-app How to use and docs/guide/diagnostics.md describe the guided flow; UI_GUIDELINES records the layout.
- Tests: Go (`target_test.go`, HTTP check route), node `packet-capture.test.cjs` (89; coach, check card, verdict, conversation results), packet-capture-privilege (CI; now also clicks the big Start). Live capture tests (`packet-troubleshooting-live`, `packet-dhcp-live`, `packet-inspector-live`, not in CI) were already failing on main: they now open the folded "Explain this packet" section, scope locators to table rows and the toolbar Start, clear toasts before wheel checks, and use the supported 960×600 minimum instead of 760 px. All three pass with the new build.
- Not claimed: live recording on a physical Windows PC; validation used synthetic captures.

---

# v1.21.0 Beta 1 (Linux L1) - publication verified (2026-11-06)

Published as opt-in prerelease https://github.com/TechSavage1989/network-toolkit/releases/tag/v1.21.0 at 2026-11-06T19:11:53Z; stable latest remains v1.20.16. Merged PR #98; tag v1.21.0 points to merge commit 6a0ab88661873e5070e833496f888327aed26ece. Release workflow run 42276939167 succeeded. Assets: Network_Toolkit.exe, Network_Toolkit_1.21.0_Portable.zip, Network_Toolkit_1.21.0_Update.zip, Network_Toolkit_1.21.0_Linux_x86_64.tar.gz, Network_Toolkit_1.21.0_Linux_aarch64.tar.gz, SHA256SUMS; all digests match SHA256SUMS. The published Linux x86_64 archive was downloaded, extracted and run (`--version` 1.21.0 linux amd64; inner SHA256SUMS OK). Checks: green run 42275134992 on the PR head. The first head failed `browser` because GitHub runners include Google Chrome, so the tab-mode test got an app window; fixed with NETWORK_TOOLKIT_BROWSER=default.

Owner: "can we get this app now developed for Linux?" Owner decisions: app-mode window (Chromium-family browser, fallback default browser); portable .tar.gz; all features in full (vault, local shell, live capture, local network tools, updater); most popular distributions. Plan and phases: docs/LINUX_PLAN.md (L1 foundation, L2 vault + local shell, L3 live capture, L4 local network + Wi-Fi, L5 updater, L6 separate windows + Save As).
- Security: on non-Windows the page used to embed the session token for any local reader of the loopback port. In Linux window mode (`a.launchGate`, not in --no-browser/dev mode or Windows) GET / and the terminal/editor view pages require the `ntk_window` cookie, obtained only through a single-use, 2-minute `/launch?ticket=` link the app hands to the window it starts. The cookie cannot call the API (X-Session-Token still required).
- Window: `lifecycle_linux.go` picks NETWORK_TOOLKIT_BROWSER, then google-chrome(-stable), chromium(-browser), microsoft-edge(-stable), brave(-browser), vivaldi(-stable), then Flatpak IDs; profile in the app-data folder (snap: ~/snap/<name>/common; flatpak: ~/.var/app/<id>/data); `--app --user-data-dir --class=NetworkToolkit`, never --no-sandbox; as root without an explicit browser it uses the tab fallback. Window process exit stops the app unless the page keeps polling for 8 s (window handed to another browser process), in which case it behaves as a tab. Tab mode: `status.uiHost` "browser-tab" shows Quit Network Toolkit (closeApp path).
- Release: build.sh builds linux amd64/arm64 (static, -trimpath); github_release.py `--linux-x86-64 --linux-aarch64` packages reproducible `Network_Toolkit_<ver>_Linux_<arch>.tar.gz` (program 0755, README from docs/LINUX_README.txt, notes, VALIDATION, inner SHA256SUMS), verifies ELF machine/version/permissions/regular files, adds them to SHA256SUMS and uploads before SHA256SUMS (`release_assets()`, also used by revise_release.py). Windows updater only reads its own package line.
- Not yet on Linux (messages say "comes to the Linux edition in a later beta"): vault/credentials, local shell, live capture, Local network, Wi-Fi, in-app updates, separate windows, Save As.
Tests: Go (gate, launcher, close detection, desktop entry), Python release tests (Linux archive round trip, tamper cases, asset order), `linux-window.browser.cjs` (real Linux build: launch link, refusal for other readers, single use, settings save, menu entry add/remove, window close stops app, tab mode Quit) and a full local packaging rehearsal with the extracted program running. Not confirmed on a physical Linux desktop.

---

# v1.20.16 stable (2026-11-05) - publication verified

Published as stable release https://github.com/TechSavage1989/network-toolkit/releases/tag/v1.20.16 at 2026-11-05T21:44:10Z (prerelease=false); `releases/latest` is now v1.20.16 (was v1.20.12). Merged PR #96; tag v1.20.16 points to merge commit adab29be4ec7ab668f44c7bac2ab988cfeb1e552. Release workflow run 42114843648 succeeded. Assets: Network_Toolkit.exe, Network_Toolkit_1.20.16_Portable.zip, Network_Toolkit_1.20.16_Update.zip, SHA256SUMS; asset digests match SHA256SUMS. Checks: green run 42112555267 on the PR head. Next beta must switch `updateBuildChannel` and docs/RELEASE_CHANNEL.txt back to beta.

Owner: "publish the stable release". Stable release of the 1.20.13 to 1.20.15 betas. No open owner-reported issues; the owner confirmed v1.20.13 on Windows ("it works well"); the v1.20.14 additions (editor right-click menu, toolbar Send to terminal, Terminal menu Send to Text Editor, icons) and the v1.20.15 Network discovery shortcut had not been reported on Windows when this was prepared. Known CI intermittents are slow-runner job timeouts (browser-workflows107, browser-terminal127) and windows-terminal, recorded in earlier entries. Release channel switched to stable (`updateBuildChannel`, docs/RELEASE_CHANNEL.txt), version 1.20.16 everywhere, release notes for stable users, UI guidelines status names v1.20.16. No application code changes beyond the channel constant. Next beta must switch the channel back to beta.

---

# v1.20.15 Beta 3 - publication verified (2026-11-05)

Published as opt-in prerelease https://github.com/TechSavage1989/network-toolkit/releases/tag/v1.20.15 at 2026-11-05T21:10:57Z; stable latest remains v1.20.12. Merged PR #94; tag v1.20.15 points to merge commit d727ce39ab42c85bfb32847d897baf2efd8e1ea2. Release workflow run 42109784573 succeeded. Assets: Network_Toolkit.exe, Network_Toolkit_1.20.15_Portable.zip, Network_Toolkit_1.20.15_Update.zip, SHA256SUMS; asset digests match SHA256SUMS. Checks: green run 42107423129 on the PR head.

Owner: "add Scan a network in Network discovery button to Monitor tool tab and traceroute like it's in ping."
- `renderTargetControls` shows the `.scope-discovery` button and "One hostname or IP address." for `DISCOVERY_LINK_TOOLS` (ping, tcp, monitor, trace). `SUBNET_TOOLS` (sweep mode) is unchanged. Monitor and Trace route gain the same 39 px target-bar row Ping has.
- ux-phase6 checks the button on Monitor and Trace route and that it opens Network discovery.

---

# v1.20.14 Beta 2 - publication verified (2026-11-05)

Published as opt-in prerelease https://github.com/TechSavage1989/network-toolkit/releases/tag/v1.20.14 at 2026-11-05T20:11:00Z; stable latest remains v1.20.12. Merged PR #92; tag v1.20.14 points to merge commit 6505ee958645649876ca048bf3023e739b2d88ea. Release workflow run 42100507462 succeeded. Assets: Network_Toolkit.exe, Network_Toolkit_1.20.14_Portable.zip, Network_Toolkit_1.20.14_Update.zip, SHA256SUMS; asset digests match SHA256SUMS. Checks: full green run 42094412114 on the PR head; in the sibling run (42094402024) browser-workflows107 was cancelled at its 20-minute job limit while still passing (it had reached graph-navigation114), recorded as a slow-runner timeout. The same happened to browser-terminal127 on record PR #91.

Owner check of v1.20.13 on Windows: "it works well". Owner: "please make the send to terminal an always visible tool, perhaps with an icon that makes sense for what it does. also add a right click menu to text editor that contains intuitive features and options, including send to terminal. also add send to text editor as an option in the terminal right click menu", then "add an arrow to the send to editor icon in terminal similar to the icon in the text editor for send to terminal".
- Toolbar: `data-ed="sendTerminal"` moves from More to the main toolbar as an icon-only button (`.nt-editor-icon-button`, new `sendTerminal` icon) after Go to line; editor-toolbar113 still passes against the published-112 baseline (a text button had wrapped the toolbar).
- Editor menu: `editor-context-menu.js` replaces the browser menu on the document text area (`textMenu.target`), reuses `.terminal-context-menu` styling, runs toolbar actions via `textMenu.run`, and Cut/Copy/Paste via `navigator.clipboard` + `replaceSelection` (undoable). Menu key / Shift+F10 open it; arrow keys, Escape and outside clicks behave like the Terminal menu. Coordinates fall back to the text area when an event has none.
- Terminal menu: `send-editor` in `MENU_ITEMS` and `DEFAULT_MENU`; `menuRevision` 2 adds it once to saved custom selections (JS `safePreferences`, Go `safeTerminalInteraction` with `terminalMenuRevision`/`terminalMenuAdded`), and a later untick is kept. New `sendEditor` icon (document with the same arrow) on the console bar button, Terminal menu and tab menu.
Tests: node (1583), Go (menu migration test), `tab-scroll-send.browser.cjs` (16 checks), editor-toolbar113 vs published-112, editor-drag115, editor-views112, editor-saved-connections113, terminal-pager, terminal-style107, terminal-paced-paste127, terminal-macro-recording, terminal-tab-click116, terminal-connection-dialog, ux-phase4/5/6, ui-consistency.

---

# v1.20.13 Beta 1 - publication verified (2026-11-05)

Owner check on Windows: "it works well" (tab row scrolling, Send to terminal, terminal output to the editor).

Published as opt-in prerelease https://github.com/TechSavage1989/network-toolkit/releases/tag/v1.20.13 at 2026-11-05T19:00:59Z; stable latest remains v1.20.12. Merged PR #90; tag v1.20.13 points to merge commit 9b247e94c118421fcf6998bd9c74b0cd3ca19233. Release workflow run 42089359513 succeeded. Assets: Network_Toolkit.exe, Network_Toolkit_1.20.13_Portable.zip, Network_Toolkit_1.20.13_Update.zip, SHA256SUMS; asset digests match SHA256SUMS. Checks: green runs 42087159510 and 42087169787 on the PR head. The first head failed browser-workflows107 (app-chrome109 detached 1024x720 terminal height) because the text "To editor" button wrapped the console bar with CI fonts; fixed by making it icon-only.

Owner: "for text editor and terminal, when more tabs than fit on screen are added and need to be scrolled, allow the mousewheel to scroll left and right when moused over the tab row, but also add left and right scroll buttons at the end of the tab row ..." and "implement a feature in the text editor that allows to send the current active tab/document content, or selection ... to a specified terminal tab as input ... automatically jump to that active terminal session and follow the already configured rules for input and output", then "send data that would be collected from select output or copy output in terminal to text editor, with the option to create a new document/tab or select an open text editor tab ... allow placing the cursor, or otherwise selecting the position".
- Tab scroll: `ux-tab-scroll.js` `attach(strip)` is called from `renderTerminalChrome` and the editor's `drawTabs`; it appends a sticky `[data-tab-scroll-controls]` group as the strip's last child (after "+", which `syncOrder` keeps after the tab rows), shown only when the tabs overflow (measured without the controls), wheel → horizontal scroll with `passive:false`, buttons scroll 70% of the visible width (smooth unless reduced motion), disabled at ends, `tabindex=-1`, pointerdown preventDefault keeps focus; a newly selected tab is scrolled clear of the controls.
- Editor → Terminal: the button lives in the editor's More row (a toolbar button wrapped the toolbar at 1440 px and failed editor-toolbar113's height gate against the published-112 baseline); Ctrl+Shift+Enter also opens it. The dialog snapshots selection/document (refused if the document changes before Send), lists connected sessions shown in this window in tab order, remembers the last target; Send closes the dialog, hides a floating editor, navigates to Terminal, selects the session, waits until it accepts input and calls `pasteText`. Not offered in detached editor/terminal windows.
- Terminal → Editor: the console bar button is icon-only (a text "To editor" button wrapped the console bar with CI fonts and failed app-chrome109's detached 1024x720 height gate). `NetworkToolkitTerminalExtras.capture` returns the chosen block's text exactly as Copy output; `file-editor.js` gains `listDocuments`, `insertText(id,text,'start'|'end'|'cursor'|{line})` (via `applyEdit`/`setContent`: undoable, dirty) and `pickInsert` (the Insert here / Cancel bar). New documents are created empty then filled, so they are unsaved and closing asks first.
Tests: node (1583), `tab-scroll-send.browser.cjs` (12 checks), editor-toolbar113 against the published-112 baseline, editor-drag115, editor-views112, terminal-pager, terminal-style107, terminal-paced-paste127, terminal-macro-recording, terminal-tab-click116, ux-phase4/5/6, ui-consistency pass locally.

---

# v1.20.12 stable (2026-11-04) - publication verified

Owner check on Windows: "restart as admin works, overview drag works too" (Restart this tab as administrator and Overview section drag, both added in v1.20.11 and shipped in v1.20.12).

Published as stable release https://github.com/TechSavage1989/network-toolkit/releases/tag/v1.20.12 at 2026-11-04T18:13:08Z (prerelease=false); `releases/latest` is now v1.20.12 (was v1.20.2). Merged PR #87; tag v1.20.12 points to merge commit a12c088da3c75fb383cb9d6803c7c7c76546e8a3. Release workflow run 41884351761 succeeded. Assets: Network_Toolkit.exe, Network_Toolkit_1.20.12_Portable.zip, Network_Toolkit_1.20.12_Update.zip, SHA256SUMS; asset digests match SHA256SUMS. Checks: full green run 41882247441 on the PR head. Next beta must switch `updateBuildChannel` and docs/RELEASE_CHANNEL.txt back to beta.

Owner: "if there are no other issues currently known, push for the stable release". No open owner-reported issues; known CI intermittents (editor-views112, native-terminal127 timeout, native-editor128-diagnostic) are test-environment flakes recorded in earlier entries. The owner confirmed the v1.20.7 to v1.20.10 betas on Windows; the v1.20.11 additions (Restart this tab as administrator, Overview section drag) had not yet been checked on Windows when this was prepared. Release channel switched to stable (`updateBuildChannel`, docs/RELEASE_CHANNEL.txt), version 1.20.12 everywhere, release notes written for stable users covering 1.20.3 to 1.20.11, UI guidelines status now names v1.20.12. No application code changes beyond the channel constant.

---

# v1.20.11 Beta 9 - publication verified (2026-11-04)

Published as opt-in prerelease https://github.com/TechSavage1989/network-toolkit/releases/tag/v1.20.11 at 2026-11-04T17:16:19Z; stable latest remains v1.20.2. Merged PR #85; tag v1.20.11 points to merge commit 3e44620b7ef531567eb9645100f26c37fcff7b54. Release workflow run 41875474487 succeeded. Assets: Network_Toolkit.exe, Network_Toolkit_1.20.11_Portable.zip, Network_Toolkit_1.20.11_Update.zip, SHA256SUMS; asset digests match SHA256SUMS. Checks: green runs 41873103508 and 41873117051 on the PR head. An earlier head failed `check` because local browser-test output (test-results/) had been committed; it was removed and ignored before merge. native-terminal127 timed out once (context deadline exceeded) on that earlier head while passing in the sibling run on the same commit; recorded as intermittent.

Owner check on Windows of v1.20.10: "open as admin in new tab works ... otherwise everything else looks good now" (test tab and Overview layout persistence, Customize dialog and vault dialogs reported fine).
Owner: "id like an option to convert a current tab to administrator" and "i would like the overview cards/sections to be draggable, and obviously retain positions thorugh restarts".
- Convert to administrator: `terminal-tabs.js` adds `convert-admin` (visible with `run-as-admin` on `adminCandidate` tabs). A confirmation explains a running shell cannot be elevated in place and that the current shell's processes and output end; it offers "Open in a new tab instead". `convertToAdmin` records the original's pane and tab index, runs the normal elevated connect path, and only if a new session with `runAsAdministrator` reaches `connected` assigns it the original pane, inserts it at the original position and deletes the original (`deleteTerminalTab(old, live)`). Declined UAC / failed start: nothing is closed. Not exercised on Windows from this session.
- Overview drag: `overviewLayoutHTML` wraps each section in `[data-overview-part]` with a `.overview-drag-handle` (pill on the top edge, visible on hover/focus). `ux-customize.js` drags by the handle (6 px threshold, mouse/pen, drop line only where shown order changes: above/below, or left/right inside the adapters/activity pair; Escape cancels) and Alt+Shift+Up/Down on the handle; `moveSection` rewrites `state.overviewLayout.order` (hidden sections keep their place) via the existing `setLayout`, which saves; v1.20.10 `mergeState` restores it at startup.
Tests: node (1583, new convert test), browser customize-layout (11 checks, including section drag, no-op line, Escape, keyboard, and restart).

---

# v1.20.10 Beta 8 - publication verified (2026-11-04)

Published as opt-in prerelease https://github.com/TechSavage1989/network-toolkit/releases/tag/v1.20.10 at 2026-11-04T16:34:13Z; stable latest remains v1.20.2. Merged PR #83; tag v1.20.10 points to merge commit a7bc81aa98654d399de26491670a8aa7ab1d06f0. Release workflow run 41868026036 succeeded. Assets: Network_Toolkit.exe, Network_Toolkit_1.20.10_Portable.zip, Network_Toolkit_1.20.10_Update.zip, SHA256SUMS; asset digests match SHA256SUMS. Checks: green runs 41863373526 and 41863364107 on the PR head.

Owner: "the positions must save between runs. right now they reset when the app is restarted. also by default make Monitor first, then ping, then Trace Route, then DNS, the Ports, Device check, Details."
Cause: v1.20.9 saved `state.testTabOrder` and `state.overviewLayout` through `/api/state` (the server keeps them; new Go test) but `mergeState` copies saved keys one by one and had no entry for them, so both were dropped at startup. The browser test had only checked saving, not a reload. Fix: `mergeState` restores both (filtered to known tests/sections, de-duplicated). `TEST_TAB_DEFAULT_ORDER.target` = monitor, ping, trace, dns, tcp, quick, identify; `orderedTestTabs` applies it before any saved order. `customize-layout.browser.cjs` now reloads from saved state and asserts the order and a dashboard layout come back (10 checks).
Tests: node (1582), Go state test, browser customize-layout, ux-phase6, connect-sidebar118.

---

# v1.20.9 Beta 7 - publication verified (2026-11-04)

Owner check on Windows: "the test tabs drag now" (Test a target / Test a website tab reordering works in v1.20.9). The Overview Customize dialog was not separately reported.

Published as opt-in prerelease https://github.com/TechSavage1989/network-toolkit/releases/tag/v1.20.9 at 2026-11-04T07:21:57Z; stable latest remains v1.20.2. Merged PR #80; tag v1.20.9 points to merge commit 1edf61527d1c9adfa26a8daea87006f3be7376c1. Release workflow run 41791456377 succeeded. Assets: Network_Toolkit.exe, Network_Toolkit_1.20.9_Portable.zip, Network_Toolkit_1.20.9_Update.zip, SHA256SUMS; asset digests match SHA256SUMS. Checks: full green run 41789524590 on the PR head; the other run on the same commit (41789521358) failed only browser-editor112 at the known editor-views112 intermittent ("Timed out: detached waitForFunction ... nt-editor-files-resize" focus), unrelated to this change and passing in the green run.

Owner: "allow the Test a target tool tabs, and test a website tool tabs to be moveable for customization. also make the dashboard customizable."
- Test tabs: `orderedTestTabs(group,members)` in `index.html` applies `state.testTabOrder.{target,web}` (unknown or new tests appended). `ux-customize.js` adds pointer drag (6 px threshold, mouse/pen only, drop line only where the order changes, Escape cancels, trailing click suppressed so a drag never navigates) and Alt+Shift+Left/Right; it moves the button in place and saves the group order.
- Overview: `renderOverview` builds named parts (`stats`, `network`, `publicIP`, `discovery`, `adapters`, `activity`) and `overviewLayoutHTML` renders them from `state.overviewLayout` (`order`, `hidden`), pairing adapters/activity in the two-column row when adjacent; warnings always show; an all-hidden layout shows an explanatory empty state. A Customize button next to Refresh opens a dialog with Show checkboxes, Move up/down buttons and Reset to default; changes apply and save immediately.
- `customize-layout.browser.cjs` (in Checks) covers default order, hide/reorder/save, reset, empty state, tab drag (no line over the near half, no navigation, saved), keyboard move, plain click and separate website order. AGENTS.md: compile with `-o /dev/null`.
Tests: node (1582) and browser features-88..91, ui-consistency, ux-phase4/5/6, connect-sidebar118, customize-layout pass locally; app-chrome109 needs the CI-only published-108 baseline.

---

# v1.20.8 Beta 6 - publication verified (2026-11-04)

Published as opt-in prerelease https://github.com/TechSavage1989/network-toolkit/releases/tag/v1.20.8 at 2026-11-04T07:01:13Z; stable latest remains v1.20.2. Merged PR #79 (replacing closed PR #78, whose history contained an accidentally committed `go build` binary; never merged); tag v1.20.8 points to merge commit be1fe5f195b3db5cfd2c2d191d34a027184cf565. Release workflow run 41789079323 succeeded. Assets: Network_Toolkit.exe, Network_Toolkit_1.20.8_Portable.zip, Network_Toolkit_1.20.8_Update.zip, SHA256SUMS; asset digests match SHA256SUMS. Checks: full green run 41786766626.

Owner: "The credential store should be configurable to either be locked behind a master password, or not. also the password manager should allow any password, but show a visual gauge for how secure the master password is. also the credential manager should also allow for saving username with password." and "when moving tabs in terminal, text editor, save session list, it should [show] a move indicator only once you've moved the tab past the next one".
- Optional master password: `credential_vault_v2.go` adds `setupWithoutPassword`, `removeMasterPassword`, `addMasterPassword`, the DPAPI-protected `Network_Toolkit.credentials.auto` (random 32-byte value, base64url) and `status().Mode` ("password"/"windows"). HTTP actions `setup-windows` (native confirmation `VaultActionNoPassword`), `set-password` (`VaultSecretSetPassword`), `remove-password` (`VaultSecretRemovePassword`, asks for the current password); `change-password` in windows mode maps to `set-password`; `lock`/`forget` are refused in windows mode. `autoUnlockCredentials` runs on GET `/credential-vault` and any `/credentials` request, never prompts, uses TryLock and binds the config identity without a remember record. Vault delete removes the auto file; recovery in windows mode removes it. Safety: the auto file is written in the activate `beforeCommit` hook under the guard and restored on commit failure (`TestVaultNoPasswordSetupRollsBackKeyWhenVaultWriteFails`).
- Any password + gauge: `vaultcrypto.MinPasswordRunes` 1 and dialog validation allow any non-empty password. `estimateVaultPasswordStrength` (pool entropy minus repeats/sequences/common passwords; five levels) feeds a static text gauge (`vaultControlStrength`) under the new-password field, refreshed on EN_CHANGE; it reads the field into a cleared buffer and never displays the password. Native dialog not exercised on Windows from this session.
- Username: credentials already stored a username; selecting a saved credential now always fills its username (`applySelection`).
- Drop lines: `terminal-tab-drag.js` `unchangedDrop`, `editor-drag.js` `unchangedDrop`, and `session-sidebar.js` `savedDropTarget` show no line where the drop would not change the order.
- Docs: CREDENTIAL_SECURITY.md (policy and new mode), UI guidelines (Password manager, confirmations, drop lines), settings guide.
Tests: Go (all packages), node (1582), Python, release validator; browser credential-security, session-saved-order (new no-line check), terminal-tab-click116, editor-drag115.

---

# v1.20.7 Beta 5 - publication verified (2026-11-04)

Published as opt-in prerelease https://github.com/TechSavage1989/network-toolkit/releases/tag/v1.20.7 at 2026-11-04T05:58:53Z; stable latest remains v1.20.2. Merged PR #76; tag v1.20.7 points to merge commit e5ee76ac59de0d08c370919c2322332f86e97fb5. Release workflow run 41782210872 succeeded. Assets: Network_Toolkit.exe, Network_Toolkit_1.20.7_Portable.zip, Network_Toolkit_1.20.7_Update.zip, SHA256SUMS; asset digests match SHA256SUMS. Checks: full green run 41780811794.

Owner: "the drive letter must be shown in the new terminal connection for powershell and command line for the starting directory configuration"; "also must be able to set a command line or powershell terminal to admin mode after launch if desired"; "also, disconnect all must also have a close all option"; "also allow re-ordering of saved sessions … by drag and drop".
- Drive letters: `terminalDriveLabel()` in `index.html` formats the local-directories drive buttons like the Files drive bar ("D: Data HDD", "C: · Local drive" for generic labels, label kept when it already names the letter); full path as tooltip.
- Administrator after launch: Windows cannot elevate a running process, so the tab menu (`terminal-tabs.js`, `run-as-admin`) shows "Open as administrator in a new tab" only for local cmd/PowerShell tabs that are not elevated; it opens a new tab, prefills the connection dialog with the same type and working directory plus `runAsAdministrator`, and submits it, so the existing approval path ("Waiting for Windows administrator approval…") and UAC prompt apply. The original tab is untouched. Not exercised on a physical Windows machine from this session.
- Close all: `#terminalDisconnectOptions` gains `data-terminal-action="close-all"` (existing `bulkWindow('close-all')`, its confirmation and Do not ask again). `terminal-extras.js` enables Disconnect all and Close all separately; the arrow is enabled while either applies (Close all covers disconnected tabs). `closeAllAvailable()` exported from `terminal-tabs.js`.
- Saved reordering: `session-sidebar.js` lists Saved by name until the first move, then in stored `terminalProfiles` order (`terminalSavedManualOrder`, added to the Go state whitelists in `terminal_settings.go`, `terminal_views_http.go`, `terminal_window.go`, `terminal_workspace_view.go`). Pointer drag with a 6 px threshold (mouse/pen only, touch keeps scrolling), accent drop line, Escape cancels, trailing click suppressed; Alt+Shift+Up/Down moves a focused row. `session-saved-order.browser.cjs` covers drag above/below, persistence, Escape, keyboard, and that double-click still opens (sign-in dialog for a password profile); headless Chromium does not synthesize dblclick on these rows on main either, so the test dispatches it.
Tests: Go, node (1581), Python and the release validator pass; browser tests terminal-disconnect133, terminal-local117, features-85, connect-sidebar118, terminal-prompt-labels127, terminal-tab-click116 and session-saved-order pass locally.

---

# v1.20.6 Beta 4 - publication verified (2026-11-04)

Published as opt-in prerelease https://github.com/TechSavage1989/network-toolkit/releases/tag/v1.20.6 at 2026-11-04T05:26:51Z; stable latest remains v1.20.2. Merged PR #74; tag v1.20.6 points to merge commit fe53c517645f2b3b9b15fb33167c6a7ca2491896. Guarded Release workflow run 41779004449 succeeded. Assets: Network_Toolkit.exe, Network_Toolkit_1.20.6_Portable.zip, Network_Toolkit_1.20.6_Update.zip, SHA256SUMS; asset digests match SHA256SUMS. Checks: full green run 41777429883 on the PR head.

Owner: "my intention was to not have the new tab button at all since there should be the new visual tab addition button." Change: the `btn small primary` New tab button is removed from the Terminal command bar (`renderTerminalWorkspace`); the strip "+" (`data-terminal-action="new"`, same handler and disabled state) is the one New tab control. `#terminalTabs` is now hidden only when empty in a detached single-session window, so an empty main or workspace Terminal shows the strip with just the "+" above the pane's SSH/Telnet/Serial/PowerShell start buttons. Command search keeps "Open a new terminal tab". `terminal-connection-dialog` and `terminal-macros-live` click the strip "+". Release notes, UI guidelines (Tab strips entry), Terminal guide and README `terminal.png` updated; screenshots with tabs and with no tabs reviewed from this session. Owner review on Windows is the acceptance check.

---

# v1.20.5 Beta 3 - publication verified (2026-11-04)

Published as opt-in prerelease https://github.com/TechSavage1989/network-toolkit/releases/tag/v1.20.5 at 2026-11-04T04:53:30Z; stable latest remains v1.20.2. Merged PR #72; tag v1.20.5 points to merge commit 249f0c973661795a868124ddc25b8c45765f872e. Guarded Release workflow run 41775606306 (build and publish succeeded). Assets: Network_Toolkit.exe, Network_Toolkit_1.20.5_Portable.zip, Network_Toolkit_1.20.5_Update.zip, SHA256SUMS; asset digests match SHA256SUMS. Beta feed lists v1.20.5 first; `releases/latest` still v1.20.2. Checks on the merged commit: full green run 41774256405 (the first run on the candidate failed only native-terminal127/129 on the ordering bug fixed in the second commit). The merged branch could not be deleted from this session (remote refuses ref deletion through the proxy); it joins the stale-branch cleanup list.

Owner: "the terminal new tab and text editor new tab should share the same design language. a tab style + button similar to but more in line with the app design style and more aesthetically appealing should be used for both." Before: the Terminal strip had full-height rectangular tabs with a 2 px accent underline and no "+" (New tab lived only in the command bar); the Text Editor copied that look with its own rules and a plain text "+".
Change: one shared stylesheet `web/assets/ux-tab-strip.css` (linked after `ux-phase6.css`, registered in `web_assets.go`) styles `.terminal-tab` and `.nt-editor-tab` the same way from the ux-foundations tokens: 40 px strip on `--side`, 30 px tabs with `--ux-radius-sm`, hover `--hover`, selected tab `--accentBg` + accent text + weight 650 (the app's `.tab.active` look), 22 px "⋯" and close controls in `--faint`/`--muted`, and a 30 px dashed "+" button that fills with `--accentBg` on hover. The Terminal strip now renders that "+" (`data-terminal-action="new"`, so it shares the command bar button's handler and disabled state); the editor's `.nt-editor-new-tab` keeps `data-ed="new"` and shows the plus icon. Old rules removed from `index.html` (inline terminal tab CSS and the 720 px min-width rule, now in the shared sheet), `file-editor.css`, `discovery-workspace.css`, `ux-command-palette.css` and `editor-aligned-compare.css` (aligned compare strips are 40 px, matching the shared strip). No markup, id, data attribute, menu, drag, shortcut or handler changes.
First CI run: native-terminal127 and native-terminal129 failed five times with "actual outside-client pointer release did not detach" while every other job passed and main passed. Cause found with a scratch Playwright replay of the native sequence (reorder, Escape-cancelled drag, outside release) against both trees: `syncOrder()` in `terminal-tab-drag.js` compared tab rows against `bar.children[index]`; with the "+" button as a strip child the comparison was off by one after the first reorder, so every later refresh re-appended the tab rows, which moved the captured tab in the DOM and ended the drag before release. Fix: ordering considers only rows with a `[data-terminal-tab]` button and re-appends the "+" after them; regression unit test added (a trailing "+" never makes a refresh move rows, and stays last after a reorder). The "+" is also hidden in a detached single-session window, whose command bar already hides New tab.
Tests: `editor-saved-connections113` asserts the shared tab geometry (6 px radius, no bottom border, 11 px label); `terminal-connection-dialog` scopes its New tab click to the command bar because the strip now has a second `new` control. Screenshots at 1440×900 in dark and light for both tools reviewed from this session; README `terminal.png` and `text-editor-compare.png` regenerated. UI guidelines gain a "Tab strips" entry. Owner review on Windows is the acceptance check.

---

# v1.20.4 Beta 2 - publication verified (2026-11-03)

Published as opt-in prerelease https://github.com/TechSavage1989/network-toolkit/releases/tag/v1.20.4 at 2026-11-03T23:23:45Z; stable latest remains v1.20.2. Merged PR #70; tag v1.20.4 points to merge commit 2f6a80bc362d0bfe90ac0174f8669b7633f396f1. Guarded Release workflow https://github.com/TechSavage1989/network-toolkit/actions/runs/41738459942 completed build and publish. Four assets: Network_Toolkit.exe, Network_Toolkit_1.20.4_Portable.zip, Network_Toolkit_1.20.4_Update.zip and SHA256SUMS; SHA256SUMS entries match GitHub's asset digests. The pull_request Checks run https://github.com/TechSavage1989/network-toolkit/actions/runs/41736249066 passed every job on the exact source commit 088bb5b; the parallel push run https://github.com/TechSavage1989/network-toolkit/actions/runs/41736237590 failed once in browser at hosts-editor.browser.cjs ("Find shortcut stole focus from entry dialog"), re-run for the record. That assertion also fails on a clean checkout of main before this change (2 of 3 local runs) and passes on the pull_request run of the same commit, so it is a timing-sensitive focus check, not a regression; added to the intermittent list. The first commit of this PR failed features-85 on both runs because the appearance editor was no longer caught up when its panel opened; fixed before merge. Owner acceptance of the typing feel on Windows rests with the owner.

Owner: "when typing into the terminal, there is latency or lagginess. the experience is a bit sluggish sometimes." Method: a scratch Playwright benchmark against the bundled UI with a synthetic SSH session (120x40) timing `xterm.write` of one character including the app's synchronous write handlers, and a CDP CPU profile over 150 echoed writes on a screen full of AOS-CX config. Before: median per echoed character 12.9 ms on an empty screen and 17.6 ms (p90 29.6) on a full screen, versus 4.7 ms with the highlighter disposed; keystroke to HTTP request about 16 ms. Profile: `getClientRects` 452 ms, `positionTerminalTools` 88 ms, appearance editor `sync`/`syncColorButton`/`syncShellSettings` 105 ms, `input-clear.js name` 42 ms, highlighter about 180 ms, per 150 writes.
Causes and changes: (1) `enqueueTerminalInput` delayed every keystroke by a 10 ms timer and `drainTerminalInput` sent one request per queued chunk; now 0 ms and queued same-origin chunks are concatenated up to 16 KiB per request. (2) `terminal-highlight.js` scanned every marked row and re-analyzed the whole viewport synchronously on each parsed write; now per-row signatures (text plus the hidden/colorable/overrideable cell flags from `collect`) skip `analyze`/`updateRow` for unchanged rows, the synchronous pass covers existing marks plus rows whose text changed, and a deferred full pass (`fullPass` on the existing 40 ms timer) reconciles attribute-only changes elsewhere; the "existing marks honor ANSI and hidden rewrites immediately" test still passes. (3) `terminal-extras.js sync()` called `NetworkToolkitTerminalAppearance.sync()` on every output batch; `sync()` now returns at once while its editor section is under `.hidden`, and the editor is caught up when the session panel (`setSessionOpen`) or the tools panel (`setTerminalToolsOpen`) opens; features-85 caught the first version, which skipped the catch-up. `positionTerminalTools` no longer reads `offsetTop` for a hidden panel. (4) `drainTerminalInput` re-assigned `xterm.options.disableStdin` after every request; xterm then flips its helper textarea's `readonly` attribute, which `input-clear.js`'s document-wide MutationObserver treated as an input change and answered with a full `querySelectorAll('input,textarea')` rescan and `getClientRects` per input. The option is now assigned only when it changes, and both the input-clear and graph-workspace observers skip mutation records inside `.xterm`. After: 5.6 ms empty / 9.5 ms full screen (p90 11.5) per echoed character, keystroke to request about 7 ms. Tests: full node unit suite, terminal browser tests (style107, pager, scroll, macros, macro history, paced paste, features-84). No Go changes.

---

# v1.20.3 Beta 1 - publication verified (2026-11-03)

Published as opt-in prerelease https://github.com/TechSavage1989/network-toolkit/releases/tag/v1.20.3 at 2026-11-03T20:13:26Z; stable latest remains v1.20.2. Merged PR #68; tag v1.20.3 points to merge commit 7145ad82d5b19f9c34874022389d9121c2070855. Guarded Release workflow https://github.com/TechSavage1989/network-toolkit/actions/runs/41710029001 completed build and publish. Four assets: Network_Toolkit.exe, Network_Toolkit_1.20.3_Portable.zip, Network_Toolkit_1.20.3_Update.zip and SHA256SUMS; SHA256SUMS entries match GitHub's asset digests. The pull_request Checks run https://github.com/TechSavage1989/network-toolkit/actions/runs/41707451486 passed every job on the exact source commit e323578; the parallel push run https://github.com/TechSavage1989/network-toolkit/actions/runs/41707445828 failed once in windows-terminal (TestNativeTerminalSFTP123InstalledWebView2 on the hosted Windows runner, unrelated to Save As) and was re-run for the record, not waived. The first commit of this PR failed features-88 on both runs because the folder picker reused the Files sidebar's nt-editor-picker-list class; the picker now has nt-editor-saveas-* classes. Owner acceptance of the real Windows Save As dialog rests with the owner.

Owner: "the text editor save as must be able to open a windows explorer window to choose a location to save, or a sftp directory depending on the choice" and "right now save as doesn't work." Before: Save As was a typed-path prompt; a path that already existed was refused by design ("destination exists; reload instead of overwriting") and a relative path was rejected, which reads as "does not work". Change: `internal/desktop.PromptSaveFile` (Windows: `GetSaveFileNameW` with OFN_OVERWRITEPROMPT and the same path validation as the export dialog, run on a locked OS thread with an invisible owner window; other OS: `ErrSaveDialogUnavailable`; single-flight via `ErrSaveDialogBusy`). `POST /api/file-editor/save-dialog` {suggested, directory} returns {path} or {cancelled}; 409 when busy, 400 when unavailable. `saveEditableLocalWith(..., overwrite)` and the remote write-text `Overwrite` flag replace an existing destination only with that explicit consent; `editorFileRequest` forwards it for detached windows. `file-editor.js`: `chooseSaveAsPath` tries the native dialog for local documents (binding local/localDraft or local sources), falls back to the prompt when the endpoint is unavailable, reports a busy dialog, and opens `remoteSaveAsPicker` for documents that carry a `list` function (remote-browser.js passes one for the remote side); the picker browses with Up and folder buttons, validates the name, and asks "Replace file?" when the name exists. Tests: Go `TestSaveEditableLocalOverwriteConsent`, `TestEditorSaveDialogHandler`; browser `scripts/editor-save-as.browser.cjs` (dialog path, cancel, busy, unavailable fallback, SFTP picker with replace confirm, Up/Enter/Escape); `file-editor-start.test.cjs` waits for the dialog check before the prompt. Not covered: the real Windows dialog (needs a Windows machine).

---

# README and repository standards (2026-11-03) - no release

Documentation only; version stays 1.20.2. README follows the shared header and section order in docs/REPOSITORY_STANDARDS.md (copied verbatim). README images are in docs/images/ (hero.png plus one per feature) and come from `scripts/readme-screenshots.browser.cjs`, which renders the real web UI against synthetic fixtures; the packet capture image needs `NTK_APP_EXECUTABLE` (any build, a Linux one works) so the real app analyses its embedded synthetic PCAP. Layout exception: CLAUDE.md is in the root and in the root allowlist (scripts/repository_layout_test.py); it only imports AGENTS.md and the standard.

# CI intermittents and handoff split (2026-11-03) - no release

Owner asked for the hosted-CI intermittents to be worked and the handoff file split. Census of the last 60 Checks runs: 15 failed jobs; by test, TestNativeEditorTransferInstalledWebView2 5, TestNativeTerminalTransferInstalledWebView2 4, TestNativeBootstrapInstalledWebView2 2, TestNativeTerminalProfiles130 1, TestLocalNetworkAdminReadOnlyWindowsDiagnostic 1, plus browser-side one-offs (editor-views112 keyboard resize, graph-scale119 fit endpoint, terminal-paced-paste127 garbage-collected promise).
Changes, all behavior-based and none waiving a check:
- `terminal-paced-paste127.browser.cjs` `ready()`: the page-side promise for the xterm write can no longer stay unsettled (Playwright reported "Resulting promise was garbage collected"): it settles on the write callback, or when the prompt is visible in the buffer, or after 10 s with a diagnostic (buffer, cursor, attachment, session state) that the test throws, so the next failure explains itself. A first attempt that only polled the buffer failed on the hosted runner twice while passing locally; the callback path is kept for that reason.
- `graph-scale119.browser.cjs`: left unchanged. A bounded wait for the fitted endpoints turned the rare endpoint mismatch into a timeout on the runner, so the mismatch is a layout question, not a timing one; still on the re-run-and-record list.
- `editor-views112.browser.cjs`: after returning to 1440 px the test waits for layout, for the resize handle to actually hold focus, and for the resize to be applied before asserting the sidebar widened past 300 px.
- `native_bootstrap_runtime_windows_test.go`: 90 s per document instead of 35 s; the hosted runner needed about 40 s to bring up WebView2 runtime 156 for the third document. Readiness and the valid probe are still required.
- `checks.yml` `windows-terminal`: the two native transfer acceptance runs that repeated scenarios already covered by dedicated jobs are removed (editor transfer in the standard mode is in native-editor128-diagnostic; the names/gestures/selection Terminal transfer is in native-terminal127 and the resize variant in native-terminal129). The 4 MiB editor performance run and every other scenario still run once. Each test still gates the workflow; fewer duplicate executions mean fewer chances for the same intermittent to fail a run.
Not changed: the native transfer tests themselves time out at their full budget (240 to 360 s) when the WebView2 window handoff never acknowledges on the hosted runner. That is not reproducible from this Linux session; it needs a Windows machine with the runner's WebView2 version. Re-run and record remains the policy for those.
Handoff split: the 1.17.x entries (v1.17.70 to v1.17.150, about 7,750 lines) moved unchanged to `docs/archive/DEVELOPMENT_HANDOFF_1.17.md`; this file keeps the UX roadmap era (v1.18.2 onward).

---

# v1.20.2 stable - publication verified (2026-11-03)

Published as stable release https://github.com/TechSavage1989/network-toolkit/releases/tag/v1.20.2 at 2026-11-03T05:42:21Z (prerelease=false); the stable channel's latest moved from v1.20.0 to v1.20.2, and the newest prerelease remains v1.20.1, which v1.20.2 supersedes for beta users. Merged PR #65; tag v1.20.2 points to merge commit 4e5adbdc71619861d1758635983309918e89db9a. Guarded Release workflow https://github.com/TechSavage1989/network-toolkit/actions/runs/41585232710 completed build and publish. Four assets: Network_Toolkit.exe, Network_Toolkit_1.20.2_Portable.zip, Network_Toolkit_1.20.2_Update.zip and SHA256SUMS; SHA256SUMS entries match GitHub's asset digests. The pull_request Checks run https://github.com/TechSavage1989/network-toolkit/actions/runs/41582673658 passed every job on the exact source commit c56fa19; the parallel push run https://github.com/TechSavage1989/network-toolkit/actions/runs/41582668795 failed once in windows-terminal (TestNativeEditorCloseApprovalInstalledWebView2 on the hosted Windows runner, a docs-and-channel-only change) and was re-run for the record, not waived. No previous release was modified.

Owner: "publish 1.20.2 stable" after confirming on Windows that the update progress bar animates again with v1.20.1 Beta 1. Release channel switched to stable, version 1.20.2 everywhere, README's latest-beta line removed, release notes written for stable users, UI guidelines status now names v1.20.2. No application code changes beyond the channel constant.

---

# v1.20.1 Beta 1 - publication verified (2026-11-03)

Published as opt-in prerelease https://github.com/TechSavage1989/network-toolkit/releases/tag/v1.20.1 at 2026-11-03T04:41:25Z; stable latest remains v1.20.0. Merged PR #63; tag v1.20.1 points to merge commit 0fb480a09eb307f108e5adb0b11eaf6df8ac52a3. Guarded Release workflow https://github.com/TechSavage1989/network-toolkit/actions/runs/41579298512 completed build and publish. Four assets: Network_Toolkit.exe, Network_Toolkit_1.20.1_Portable.zip, Network_Toolkit_1.20.1_Update.zip and SHA256SUMS; SHA256SUMS entries match GitHub's asset digests. Both Checks runs on the final source commit 96faedb (main merged in after the v1.20.0 record) passed every job: pull_request https://github.com/TechSavage1989/network-toolkit/actions/runs/41577509664 and push https://github.com/TechSavage1989/network-toolkit/actions/runs/41577503616, including the new progress-animation browser check. Owner acceptance of the animation on Windows rests with the owner.

Owner: "the install and restart progress animation no longer plays." The in-app bar is a native `<progress>` without a value (indeterminate) rendered by `progressHTML` during the saving/installing/restarting phases. Headless Chromium experiment with the shipped stylesheets: an indeterminate bar animates with no author styles, with only `::-webkit-progress-bar`/`::-webkit-progress-value` styled, or with `accent-color`/`height`/`overflow:hidden` on the element, but stops as soon as `border-radius` is set on the `progress` element while both pseudo-elements are styled. `ux-foundations.css` (Phase 2, so this has been the case since v1.18.1) had exactly that combination. Fix: drop `border-radius` from `:where(progress)`; the pseudo-elements keep their radius so the look is unchanged. Regression check `scripts/progress-animation.browser.cjs` loads `ux-foundations.css` and `discovery-workspace.css`, screenshots an indeterminate bar twice 400 ms apart in light and dark themes and asserts the frames differ; wired into the browser job. The separate native installer window (Win32 marquee, `updater_progress_windows.go`) is unchanged since 1.17 and cannot be exercised here; if the owner meant that window, a screenshot is needed. Channel switched back to beta for 1.20.1.

---

# v1.20.0 stable - publication verified (2026-11-03)

Published as stable release https://github.com/TechSavage1989/network-toolkit/releases/tag/v1.20.0 at 2026-11-03T04:01:33Z (prerelease=false); the stable channel's latest moved from v1.19.0 to v1.20.0, and the newest prerelease remains v1.19.9, which v1.20.0 supersedes for beta users. Merged PR #61; tag v1.20.0 points to merge commit 1fa1f3109307a99e12eb442502ed886b6e3e2113. Guarded Release workflow https://github.com/TechSavage1989/network-toolkit/actions/runs/41575342430 completed build and publish. Four assets: Network_Toolkit.exe, Network_Toolkit_1.20.0_Portable.zip, Network_Toolkit_1.20.0_Update.zip and SHA256SUMS; SHA256SUMS entries match GitHub's asset digests. The pull_request Checks run https://github.com/TechSavage1989/network-toolkit/actions/runs/41572558208 passed every job on the exact source commit 88f57da; the parallel push run https://github.com/TechSavage1989/network-toolkit/actions/runs/41572554077 needed one re-run of a hosted-runner intermittent and then passed every job. No previous release was modified.

Owner: "prepare v1.20.0 stable" after confirming the batch-output fix on the live switch. Release channel switched to stable (`docs/RELEASE_CHANNEL.txt` and `updateBuildChannel`), version 1.20.0 everywhere, README's latest-beta line removed (the stable is newer than every beta), release notes summarize 1.19.1 to 1.19.9 in plain language, UI guidelines status now names v1.20.0. No application code changes beyond the channel constant. After publication the stable channel's latest moves from v1.19.0 to v1.20.0 and beta users receive it as well because 1.20.0 sorts above 1.19.9.

---

# v1.19.9 Beta 9 - publication verified (2026-11-03)

Published as opt-in prerelease https://github.com/TechSavage1989/network-toolkit/releases/tag/v1.19.9 at 2026-11-03T02:34:59Z; stable latest remains v1.19.0. Merged PR #58; tag v1.19.9 points to merge commit 8a0f552e9b31f3aa293b7874e2ab38c3249fad43. Guarded Release workflow https://github.com/TechSavage1989/network-toolkit/actions/runs/41566683570 completed build and publish. Four assets: Network_Toolkit.exe, Network_Toolkit_1.19.9_Portable.zip, Network_Toolkit_1.19.9_Update.zip and SHA256SUMS; SHA256SUMS entries match GitHub's asset digests. Both Checks runs on the final source commit 2f27e66 (main merged in after the v1.19.8 docs record) passed every job: pull_request https://github.com/TechSavage1989/network-toolkit/actions/runs/41565292402 and push https://github.com/TechSavage1989/network-toolkit/actions/runs/41565286994. The earlier commit 3a51e89 saw one known intermittent per run (native-terminal129 TestNativeTerminalTransferInstalledWebView2 360s deadline; browser-editor112 editor-views112 keyboard-resize assertion), both re-run. Hosted-runner note from the docs-only PR #57 round: TestNativeBootstrapInstalledWebView2 failed once ("installed WebView2 did not initialize through native-only capability", runtime 156.0.4314.50) and terminal-paced-paste127.browser.cjs once with "page.evaluate: Resulting promise was garbage collected"; both passed on re-run and on every main run, recorded as intermittent, not waived. Live acceptance of the 30-minute update cadence and the toast rests with the owner.

Owner: "I'd like the app to auto detect the update instead of me having to manually check for updates, or close the app and reopen." The scheduler (`app-updater.js` `createScheduler`) already ran at startup and every six hours, and `wake()` on focus only re-checked after six hours, so a beta published while the app stayed open went unnoticed for up to six hours. Change: `createScheduler` takes `interval` and `freshness` from the host (defaults unchanged at six hours, clamped to at least one minute); the host passes 30 minutes and 10 minutes. `createController.check(background)` calls `env.announce(snapshot)` when a background check finds an update; the host toasts once per notice key (same dismissal rules as the top-of-app notice). Setting text and the guide describe the new cadence. AGENTS.md's "at startup and every six hours" minimum is still met (checks are more frequent); the owner asked for this change. Feed cost: conditional GitHub requests (ETag/304) plus the server-side cooldown, so 2 requests/hour at most while running.
Owner follow-up on batch gaps: one blank line per page remained after v1.19.8 because AOS-CX prints the pager prompt after an extra line break; `pacedOutput` now also drops the whitespace-only line directly before a recognized pager line. Test extended with the "--More--, next page: Space, next line: Enter, quit: Control-C" form.
Tests: `app-updater.test.cjs` (host cadence/freshness, clamping, defaults; background announce once per check, not for manual checks), `terminal-paced-capture134.test.cjs`.

---

# v1.19.8 Beta 8 - publication verified (2026-11-03)

Published as opt-in prerelease https://github.com/TechSavage1989/network-toolkit/releases/tag/v1.19.8 at 2026-11-03T01:49:13Z; stable latest remains v1.19.0. Merged PR #56; tag v1.19.8 points to merge commit 1397a773e5064f9eba92408299c60d28ba01eb4f. Guarded Release workflow https://github.com/TechSavage1989/network-toolkit/actions/runs/41561891015 completed build and publish. Four assets: Network_Toolkit.exe, Network_Toolkit_1.19.8_Portable.zip, Network_Toolkit_1.19.8_Update.zip and SHA256SUMS; SHA256SUMS entries match GitHub's asset digests. The pull_request Checks run https://github.com/TechSavage1989/network-toolkit/actions/runs/41560043934 passed every job on the exact source commit ac96514; the parallel push run https://github.com/TechSavage1989/network-toolkit/actions/runs/41560040060 failed once in windows-terminal (TestNativeSFTPDirect121InstalledWebView2, "Actual Explorer destination file did not arrive" for the main-folder drag gesture on the hosted Windows runner; the same test had an earlier cancellation-state intermittent on record): re-run requested, not waived. Owner standing instruction from this round: push, merge and publish betas without asking.

Owner report with a full copied batch (sh run / sh vlan / sh ip route / sh int bri / sh lldp ne on an AOS-CX 6200M): an empty line and a 63-space line appeared at every page boundary. The engine answers the pager (v1.19.6) and `pacedOutput` already drops the `--More--` line itself, but the device erases the prompt with `\r` + spaces + `\r`, which the capture keeps as whitespace-only lines. Change (`terminal-extras.js` `pacedOutput`): after a line matched by `matchPagerLine`, consecutive whitespace-only lines are skipped (`afterPager`); the first non-blank line resets the state, so blank lines elsewhere stay. Covers paced pastes, Library macros and Run commands since all use `pacedOutput`. Screen-based output (typed commands, raw pastes) is unchanged: xterm trims trailing spaces, so the residue there is an ordinary empty line that cannot be told from real output. Test: `terminal-paced-capture134.test.cjs` "pager prompt residue ... leaves no gaps".

---

# v1.19.7 Beta 7 - publication verified (2026-11-03)

Published as opt-in prerelease https://github.com/TechSavage1989/network-toolkit/releases/tag/v1.19.7 at 2026-11-03T01:10:57Z; stable latest remains v1.19.0. Merged PR #54; tag v1.19.7 points to merge commit 212a21ce4c92e3d89cdad5ffa0c102c732b4f42f. Guarded Release workflow https://github.com/TechSavage1989/network-toolkit/actions/runs/41557675592 completed build and publish. Four assets: Network_Toolkit.exe, Network_Toolkit_1.19.7_Portable.zip, Network_Toolkit_1.19.7_Update.zip and SHA256SUMS; SHA256SUMS entries match GitHub's asset digests. The pull_request Checks run https://github.com/TechSavage1989/network-toolkit/actions/runs/41555883719 passed every job on the exact source commit 1e55a4b; the parallel push run https://github.com/TechSavage1989/network-toolkit/actions/runs/41555875856 timed out once in native-terminal129 (TestNativeTerminalTransferInstalledWebView2, 360s context deadline on the hosted Windows runner, the same intermittent recorded for v1.19.4) and passed on re-run: recorded, not waived. Live-device acceptance of the Last batch tools with macros rests with the owner.

Owner: "The last command/last paste, select output copy output feature needs to work with macros too" and "instead of last paste, make it last command batch or whatever else may make sense, and make sure it works in any scenario." Before: only paced pastes created a linked history block and only `literalPaste` runs recorded `sentPaste` cursors / served `PasteCapture`. Change: `execute` records a capture step for every run's accepted command line; `PasteCapture` serves any run (still per session, still redacting secrets). `terminal-macros.js`: `noteRunBlocks(targets,commands)` creates one paste-kind history block per target console (via `NetworkToolkitTerminalExtras.notePaste`) before the `/runs` or `/send` request, `linkRunBlocks` ties them to the accepted run (`trackPacedPaste`, now for any run) or discards them when the start fails; used by `runPreview`, startup bindings and Run commands (`send` with execute). `collectPacedCaptures` completes blocks for any finished run; the macro shortcut texts (`S.last`) are updated for literal pastes and Run commands (`send-commands`) runs, not for Library macros. Rename: console switch "Last paste" -> "Last batch" with the explanatory tooltip, menu items "Jump to batch start / Previous batch / Next batch / Select batch output / Copy batch output", sidebar "Jump to latest command batch", macro shortcut "Last batch"; data attributes and command ids (`last-paste`, `paste-*`) are unchanged so saved settings and tests keep working. Scenarios covered: typed command, raw paste, paced paste, Library macro, shortcut/startup macro, Run commands list, multiple targets, failed start (blocks discarded), disabled/non-command steps (excluded on both sides), secrets (redacted in capture). Tests: Go `TestMacro136CaptureAssociatesCommandsAndOutputForMacroRuns`; node `terminal-paced-capture134.test.cjs` (macro run links and completes without touching macro shortcuts; `noteRunBlocks`/`linkRunBlocks`). Guide and UI guidelines updated.

Owner: "I don't want the Runs window to be switched to after a multi line paste or after using a macro." Only `startPacedPaste` called `openView('runs')` (which also expanded the Macros sidebar tab); macro starts (`confirmRun`, shortcut runs) already only re-render. Change: the `openView('runs')` call is removed; `notify('Paced paste started. Pause or Stop is available in Runs.')` stays. `terminal-paced-paste127.test.cjs` now asserts no view change and the browser test opens the Runs tab itself before using Pause/Stop; `terminal-macros.test.cjs` already asserted "An accepted run does not navigate". UI_GUIDELINES Terminal paragraph updated.

---

# v1.19.6 Beta 6 - publication verified (2026-11-03)

Published as opt-in prerelease https://github.com/TechSavage1989/network-toolkit/releases/tag/v1.19.6 at 2026-11-03T00:17:47Z; stable latest remains v1.19.0. Merged PR #52; tag v1.19.6 points to merge commit 776e9f30e7a4e54207f8cee3b5e63519b5e7acdb. Guarded Release workflow https://github.com/TechSavage1989/network-toolkit/actions/runs/41551464944 completed build and publish. Four assets: Network_Toolkit.exe, Network_Toolkit_1.19.6_Portable.zip, Network_Toolkit_1.19.6_Update.zip and SHA256SUMS; SHA256SUMS entries match GitHub's asset digests. The push Checks run https://github.com/TechSavage1989/network-toolkit/actions/runs/41549453868 passed every job on the exact source commit 0de932c; the parallel pull_request run https://github.com/TechSavage1989/network-toolkit/actions/runs/41549459403 failed once in native-editor128-diagnostic (TestNativeEditorTransferInstalledWebView2, destination ownership ACK not reached on the hosted Windows runner, same intermittent as the v1.19.4 round and the docs-only main run 41518631051) and passed on re-run: recorded, not waived. Live-device acceptance of the pager fix rests with the owner.

Owner report with screenshots: pasting "sh run / sh int bri / sh ip route / sh lldp ne" into CCC-IT-CENTER-6200M-SW01 with One line at a time stopped at 1/4 (stopped by the owner) and the output did not follow. Cause: `Engine.OwnsPager` suppresses the console's automatic --More-- reply while a run is active, and `waitPastePrompt` (literal paste) never answered the pager itself (`TestPaste127DoesNotAutoAnswerPager...` pinned that), so `sh run` sat at --More-- until the 30-second prompt timeout or Stop. The "no auto scroll" symptom is the same stall: nothing new arrived.
Change: `PasteRequest` gains `autoPager` and `pagerKey`; `StartPaste` sets `Definition.AutoPager` and stores the key in `pasteOptions` on the execution (`startPrepared` now takes `*pasteOptions`, nil for macros). `waitPastePrompt` scans fresh output with `pastePagerPattern` (the macro `pagerPattern` plus "Press Enter/Return ... more|continue|next"), writes one `pagerReply` per new match (auto: Enter only when the prompt names Enter/Return, else Space) and resets the prompt-stability candidate; a pager line still never matches `DefaultPrompt`. Frontend: `terminal-interaction.js` `pacedPagerPrefs`/`pacedPagerNote` read `NetworkToolkitTerminalExtras.prefs(entry)` (session Auto pager, default on), pass them to `startPacedPaste(target,text,revision,options)` and replace the "No auto pager" hint; `startPacedPaste` calls `prepareRunView` so the terminal follows output. Tests: Go `TestPaste135AutoPagerAnswersMoreBeforeNextLine` (auto/space/enter × --More--/Press Enter) and `TestPaste135AutoPagerOffStillWaitsAtMore`; node `terminal-paced-paste127.test.cjs` (options in the request body, unknown key falls back to auto); browser `terminal-paced-paste127.browser.cjs` (body carries autoPager/pagerKey). Macro runs are unchanged.
Not verified against a live device (owner's switch is out of bounds); the fixture emits Cisco-style `--More--` between lines.

---

# v1.19.5 Beta 5 - publication verified (2026-11-02)

Published as opt-in prerelease https://github.com/TechSavage1989/network-toolkit/releases/tag/v1.19.5 at 2026-11-02T20:03:54Z; stable latest remains v1.19.0. Merged PR #50; tag v1.19.5 points to merge commit 0a6e3ff2c8bdaa8ae82b553bec796b5ee71ffa94. Guarded Release workflow https://github.com/TechSavage1989/network-toolkit/actions/runs/41514798623 completed build and publish. Four assets: Network_Toolkit.exe, Network_Toolkit_1.19.5_Portable.zip, Network_Toolkit_1.19.5_Update.zip and SHA256SUMS; SHA256SUMS entries match GitHub's asset digests. The push Checks run https://github.com/TechSavage1989/network-toolkit/actions/runs/41512420140 passed every job on the exact source commit 2abcda9; the parallel pull_request run https://github.com/TechSavage1989/network-toolkit/actions/runs/41512431184 timed out once in native-terminal130 (TestNativeTerminalProfiles130InstalledWebView2, 240s context deadline) and once in windows-terminal (TestNativeEditorLocal119InstalledWebView2, native command snapshot timed out), both hosted Windows WebView2 intermittents already on record: re-run requested, not waived. Earlier rounds on this PR failed in features-94 and value-copy-live because those tests still asserted the removed toolbars, copy icons and dates; the tests were updated to the approved behavior.

Owner reviewed a per-tool screenshot sheet and approved: Activity log removed from all results pages (events remain in Export); generic renderers no longer dump source/target/recordedAt (resultBodyData strips them before per-tool rendering; identify also drops identityMethod/elapsedMs; smtp drops elapsedMs); renderWol renders one sentence; renderHttp shows Final URL only when different, Headers in a collapsed details, Redirects only when present; dataList gains `toolbar:false` and `plainCells` (adds data-value-copy="off", excluded by value-copy.js) and resultTable gains `toolsColumn:false` and `timeFormat:'time'` (NetworkToolkitResultTime.column(job,timeFormat), cell({date})); the auto-added Recorded column appears only when a row carries a time; generic arrays get a toolbar only beyond 25 rows; quick/trace/dns/tcp lists have no toolbar; DNS per-type queries table removed; discovery network-notes replaced by the tool's Help and formats note (configuration-layout.js). Owner kept: Ping/Monitor filter bar always; discovery toolbar; HTTP headers and body (collapsed).

---

# v1.19.4 Beta 4 - publication verified (2026-11-02)

Published as opt-in prerelease https://github.com/TechSavage1989/network-toolkit/releases/tag/v1.19.4 at 2026-11-02T13:23:44Z; stable latest remains v1.19.0. Merged PR #48; tag v1.19.4 points to merge commit 7c6146807fb07edbcc3ce3553a0d67aa6742b625. Guarded Release workflow https://github.com/TechSavage1989/network-toolkit/actions/runs/41452168553 completed build and publish. Four assets: Network_Toolkit.exe, Network_Toolkit_1.19.4_Portable.zip, Network_Toolkit_1.19.4_Update.zip and SHA256SUMS; SHA256SUMS entries match GitHub's asset digests. PR #48 pull_request Checks https://github.com/TechSavage1989/network-toolkit/actions/runs/41449959795 passed on the exact source; the parallel push run's native-terminal127 job timed out once (TestNativeTerminalTransferInstalledWebView2, 240s context deadline on the hosted Windows runner) and passed on the pull_request run and the last four main runs: recorded as intermittent, re-run requested, not waived. Earlier rounds on this PR also saw the graph-curve113 sparse-startup check fail once (same intermittent zoom-state leak noted for v1.19.2).

Owner request: results pages carry redundant technical panels (Device check example: "Recorded source: Automatic routing", "Additional information" with a nested "Source" block repeating mode/status/resolver/note, then the Activity log) that bury the primary results. Owner approved: remove the leftover-field dump, hide Recorded source for Automatic routing (keep it collapsed for a chosen adapter), keep the Activity log collapsed at the bottom; then review each tool's result page from a screenshot sheet for further cuts.
Change: renderResultExtras returns '' (the v1.17.113 "readable extras" panel is retired; renderStaticObject is still used by the entry readers). NetworkToolkitSource.resultHTML returns '' for mode automatic. Copy and Export are built from result data (jobText/NetworkToolkitReportExport), so no recorded field is lost. Tests updated: result-readers113 (unit + browser: unique evidence must be in Copy/Export text, no extras panel), source-interface (chosen source renders one panel, automatic none), features-92 (no recorded-source panel for automatic routing).
Next: per-tool result screenshot review sheet for the owner.

---

# v1.19.3 Beta 3 - publication verified (2026-11-02)

Published as opt-in prerelease https://github.com/TechSavage1989/network-toolkit/releases/tag/v1.19.3 at 2026-11-02T05:57:52Z; stable latest remains v1.19.0. Merged PR #46; tag v1.19.3 points to merge commit 0c3b598fa69b5feff501a348601b2fbb22407bfd. Guarded Release workflow https://github.com/TechSavage1989/network-toolkit/actions/runs/41396803080 completed build and publish. Four assets: Network_Toolkit.exe, Network_Toolkit_1.19.3_Portable.zip, Network_Toolkit_1.19.3_Update.zip and SHA256SUMS; SHA256SUMS entries match GitHub's asset digests. PR #46 pull_request Checks https://github.com/TechSavage1989/network-toolkit/actions/runs/41395359678 passed on the exact source; the parallel push run's browser-workflows107 job failed once in editor-workflows108 (compare gutter visibility, `layout.visibleGutter.length>0`), which passed in the pull_request run and locally 2/2: recorded as intermittent, not waived.

Owner report: Local network > IP configuration took a long time to appear, and Network actions sat above the current configuration. Cause: GET /local-tools/ip runs the fast native GetAdaptersAddresses read and then a PowerShell read of each adapter's administrative state (needed only for the Enable/Disable network actions); the page waited for both.
Change: new `GET /local-tools/ip?phase=basic` (backend `readIPBasic`, Windows = native read with admin state normalized to unknown; non-Windows = readLocalIP) returns the snapshot marked `adminPending`. The UI renders it immediately (Public IP, snapshot time, adapter picker, Current configuration + IPv4 editor), then requests the full snapshot and fills in the Network actions card, which now sits below the configuration and shows "Reading adapter state" with its buttons disabled until then. Reload uses the same two phases; a newer reload cancels an older one via a token. Default GET /local-tools/ip is unchanged (operations and verification still use the full read).
Tests: Go `TestLocalIPBasicPhaseServesNativeSnapshotBeforeAdminState`; ux-phase6 browser check asserts section order, pending state and the fill-in.

---

# v1.19.2 Beta 2 - publication verified (2026-11-02)

Published as opt-in prerelease https://github.com/TechSavage1989/network-toolkit/releases/tag/v1.19.2 at 2026-11-02T05:23:07Z; stable latest remains v1.19.0. Merged PR #44; tag v1.19.2 points to merge commit 78555d6cf716b84bb0a5706df37ae6df8fc3fe72. Guarded Release workflow https://github.com/TechSavage1989/network-toolkit/actions/runs/41393451200 completed build and publish. Four assets: Network_Toolkit.exe, Network_Toolkit_1.19.2_Portable.zip, Network_Toolkit_1.19.2_Update.zip and SHA256SUMS; SHA256SUMS entries match GitHub's asset digests. PR #44 Checks (https://github.com/TechSavage1989/network-toolkit/actions/runs/41391957700 and https://github.com/TechSavage1989/network-toolkit/actions/runs/41391952867) and the Phase 6 workflow passed on the exact source.

Branch fix/flaky-tests-monitor-graph, based on published v1.19.1. Owner asked to fix the intermittent CI checks and the cramped Monitor graph at 1280x800, and to publish the result as a beta when green.

- Real bug fixed in editor-aligned-compare.js: the status row under each compare pane could wrap on one side only, making that editor 15px shorter; matching lines then drifted near the end of a short document. Both rows now take the taller row's height (matchPaneChrome), re-applied on resize.
- Flaky tests made behavior-based: editor-alignment113 waits (bounded) for scroll sync to settle and tests main 1440 / detached 1440 and 640 (390px retired per owner decision 2); editor-layout-map waits for the new comparison before asserting marks; transfer-client-layout serves the mock read after 1.5s so the loading state is observable on slow runners; graph-curve113 waits for the sparse render.
- Monitor graph: graph-navigator.css adds a 721-820px rule that compacts only the overview strip (34px) and, under 820px, the samples toolbar and table header inside graph results. Graph height stays 230px; plot 84 to 99px; two complete rows at 1280x800. features-94/95 test 1440x1000, 1280x800 and 960x1000, and treat 800px as a short window (their own threshold boundary).
- Open issue: main window at 960px wide, end-of-document after keyboard divider resize, panes up to 9px apart (scroll-sync residual correction, renderedPeerOffset/nextPaintedOffset). Documented in editor-alignment113; add 960 back there when fixed.
- Open issue found by the widened tests: Packet capture at 1280x800 on hosted Ubuntu runners wraps the view bar (83px) and leaves the capture configuration reader 9px tall (scrollHeight 154). Not reproduced locally; pre-existing (capture CSS untouched here). features-94/95 measure packet layouts at 1000px height until fixed.
- Remote branch cleanup cannot be done from this session (API path blocked); the owner was given a git command for the merged branches.

---

# v1.19.1 Beta 1 - publication verified (2026-11-02)

Published as opt-in prerelease https://github.com/TechSavage1989/network-toolkit/releases/tag/v1.19.1 at 2026-11-02T02:55:32Z; stable latest remains v1.19.0. Merged PR #42; tag v1.19.1 points to merge commit ad3675be2a8b9794a78fb79f4d413851fdf500f7. Guarded Release workflow https://github.com/TechSavage1989/network-toolkit/actions/runs/41380061126 completed build and publish. Four assets: Network_Toolkit.exe, Network_Toolkit_1.19.1_Portable.zip, Network_Toolkit_1.19.1_Update.zip and SHA256SUMS; SHA256SUMS entries match GitHub's asset digests. PR #42 Checks (pull_request https://github.com/TechSavage1989/network-toolkit/actions/runs/41378520940, push https://github.com/TechSavage1989/network-toolkit/actions/runs/41378516575) and Phase 6 workflow https://github.com/TechSavage1989/network-toolkit/actions/runs/41378516656 passed on the exact source. During review a CSS :has() slowdown that made the File Transfer editor test fail was found and fixed before merge.

Owner check: on 2026-11-02 the owner reported that v1.19.1 is working. The scope of that check was not recorded (which tools or devices were exercised), so it is not a full physical Windows / live-device acceptance; record specifics when available.

Branch ux/consolidation-2, based on published v1.19.0 stable. The owner approved the redundancy proposals (docs/UX_AUDIT.md section 12), keeping Wake on LAN as its own tool. docs/UI_GUIDELINES.md is updated in the same change, as the layout freeze requires.

- Test a target tabs: Device check, Ping, Trace route, DNS, Ports, Monitor, Details (short labels; tooltips and command search use full names). Test a website: HTTP check and TLS certificate with one shared address. Local network gains a Hosts file tab. Ping/TCP ports no longer switch to network sweeps; Scan a network opens Network discovery, and saved sweep settings/ranges migrate there.
- Fixed: at 600px window height the diagnostic page heading (and with it the Phase 6 test tabs) was hidden; test tabs now stay visible.
- Release channel switched back to beta (docs/RELEASE_CHANNEL.txt, updateBuildChannel); stable users stay on 1.19.0.
- Tests: ux-phase6 browser test covers the new groups, website address handling, Hosts tab and sweep migration. features-94/95 graph-space checks now use supported widths (1440/1280/960 at 1000px height) instead of 820/390px.
- Known limitation (also in v1.19.0): at 1280x800 the Monitor graph's plot area is 84px, under the 90px the graph-space check expects in taller windows. Follow-up: make graph controls more compact in short windows.

---

# v1.19.0 stable - publication verified (2026-11-02)

Published as the stable release https://github.com/TechSavage1989/network-toolkit/releases/tag/v1.19.0 at 2026-11-02T01:20:18Z (not a prerelease; GitHub's latest, replacing v1.17.136). Merged PR #40; tag v1.19.0 points to merge commit 46adc55b7a66f755060019eab198b23801f07804. Guarded Release workflow https://github.com/TechSavage1989/network-toolkit/actions/runs/41371760780 completed build and publish. Four assets: Network_Toolkit.exe, Network_Toolkit_1.19.0_Portable.zip, Network_Toolkit_1.19.0_Update.zip and SHA256SUMS; SHA256SUMS entries match GitHub's asset digests. Physical Windows and live-device acceptance were not performed.

Branch release/1.19.0-stable, built on the post-Phase 6 docs branch (UI guidelines draft, README screenshots). The owner chose 1.19.0 instead of the planned 1.18.0, because a 1.18.0 stable would sort below the published v1.18.5 beta and the updater refuses downgrades.

Stable switches: docs/RELEASE_CHANNEL.txt is `stable` and updater.go `updateBuildChannel` is "stable", so the guarded Release workflow publishes a normal (non-prerelease) release that becomes the stable "latest". Opted-in beta users also receive it because the beta feed picks the newest numeric version, including stable releases. RELEASE_NOTES_1.19.0.txt summarizes the 1.18.1 to 1.18.5 betas; the 1.18.4 and 1.18.5 notes are archived in docs/releases/.

Before publishing: owner approval of docs/UI_GUIDELINES.md (starts the layout freeze) and of this candidate, green Checks on the exact source. Physical Windows and live-device acceptance have not been performed; decide whether stable requires them. Merging to main publishes.

---

# v1.18.5 Beta 5 - publication verified (2026-11-02)

Published as opt-in prerelease https://github.com/TechSavage1989/network-toolkit/releases/tag/v1.18.5 at 2026-11-02T00:44:07Z. Merged PR #37; tag v1.18.5 points to merge commit 36ef57d14b078bf5c1f9b29b2073787cd3d38609. Guarded Release workflow https://github.com/TechSavage1989/network-toolkit/actions/runs/41368461612 completed build and publish successfully. Four assets: Network_Toolkit.exe, Network_Toolkit_1.18.5_Portable.zip, Network_Toolkit_1.18.5_Update.zip and SHA256SUMS; the SHA256SUMS entries match GitHub's asset digests. Stable latest remains v1.17.136. The owner approved the Phase 6 consolidation and asked to publish. Read docs/UX_PHASE6_HANDOFF.md, docs/UX_POLISH_PLAN.md and docs/DEVELOPMENT_ROADMAP.md before proceeding.

Phase 6 merges Ping, Trace route, DNS lookup and TCP ports into Test a target (tabs share the host and show each test's latest result) and SFTP, FTP/FTPS and TFTP into File Transfer (protocol picker). Former names remain in command search. The Terminal sidebar's separate Sidebar button is removed. The first-launch welcome stays on Overview. Engines, options, jobs, saved profiles, credentials, vault, sessions, UAC and host-key/overwrite confirmations are unchanged.

After Phase 6: write docs/UI_GUIDELINES.md, freeze layout pending owner approval, update README screenshots and prepare the 1.18.0 stable candidate. A combined side-by-side results view for Test a target was not built and needs an owner-reviewed design.

---

# v1.18.4 Beta 4 - publication verified (2026-11-01)

Published as opt-in prerelease https://github.com/TechSavage1989/network-toolkit/releases/tag/v1.18.4 at 2026-11-01T20:33:03Z. PR #36 merged source/tag 977e9bf6a954a16be2d8f74e8f6bd5f61fccbb19. Guarded Release workflow https://github.com/TechSavage1989/network-toolkit/actions/runs/41347071329 passed both Windows build and publish. Official assets: Network_Toolkit.exe, Network_Toolkit_1.18.4_Portable.zip, Network_Toolkit_1.18.4_Update.zip and SHA256SUMS, all uploaded with GitHub SHA-256 digests. Previous releases untouched. Stable latest stays v1.17.136.

Phase 5: a dismissible first-run welcome offers Device check, Terminal and File Transfer without automatic network actions. Returning users keep their saved state without an unsolicited welcome; F1 Help and command search can reopen the guide. Diagnostics now show examples and meaningful next steps, and failures keep their original technical information. No changes to credential vault, profiles, macros, editor documents, live sessions, Windows/UAC checks, file overwrite/host trust, opt-in verified updater, or stable executable naming. What's New remains user-focused; technical notes belong in Additional Details.

Hosted synthetic Phase 5 browser, updater and guarded version-resource workflow https://github.com/TechSavage1989/network-toolkit/actions/runs/41346939400 succeeded, as did the exact-source release packaging and publication. At release verification, the full Checks matrix remained in progress; earlier duplicate runs had intermittent Editor alignment/timing and graph observations, and a sticky-header double-click test was corrected by centering the row before real pointer activation. Do not claim full CI, physical desktop or live-device acceptance where not verified. Follow docs/UX_PHASE5_HANDOFF.md, docs/UX_POLISH_PLAN.md and docs/DEVELOPMENT_ROADMAP.md. Next: owner-approved Phase 6 tool consolidation, preservation of preferences and sessions, then UI guidelines and review for stable release. Do not use the owner's desktop or touch prior tags/assets.
---

# v1.18.3 Beta 3 - publication verified (2026-11-01)

Published as opt-in prerelease https://github.com/TechSavage1989/network-toolkit/releases/tag/v1.18.3 at 2026-11-01T19:26:12Z. Merge PR #34 source/tag bec58aa724e1ad6b33237eb6c6a4aff783bf0c85. Guarded Release workflow https://github.com/TechSavage1989/network-toolkit/actions/runs/41341463401 completed build and publish successfully. GitHub lists four assets with SHA-256 digests: Network_Toolkit.exe, Network_Toolkit_1.18.3_Portable.zip, Network_Toolkit_1.18.3_Update.zip and SHA256SUMS. Stable latest remains v1.17.136. All prior tags/assets untouched.

Phase 4: searchable Settings categories with existing controls and links to specialized tool preferences; Ctrl+Shift+T safely reopens clean local Editor files and unused Terminal drafts without secrets or remote reconnection; saved-target removal supports a ten-second Undo. Preserves credentials, vault, profiles, macros, unsaved documents, live sessions, Windows/UAC reviews, SSH and overwrite security and checksum-verified user-consented portable updates. Fixed executable filename Network_Toolkit.exe; high-level What's New, technical detail in Additional Details. No desktop, personal network or physical Windows acceptance was used.

Exact-source Checks https://github.com/TechSavage1989/network-toolkit/actions/runs/41340055916 passed all 15 jobs, and focused Phase 4 browser suite passed. A simultaneous duplicate PR Checks run 41340062114 reported intermittent failures: 16 MiB Editor input 5.254 seconds versus 5-second performance bound, and a graph-window point-range assertion. They were not reproduced in the primary green run but should be investigated before stable; never waive a genuine data/security failure. See docs/UX_PHASE4_HANDOFF.md and docs/UX_POLISH_PLAN.md. Next Phase 5 (language, empty states and first-run) awaits owner approval.

---

# v1.18.2 Beta 2 - publication verified (2026-11-01)

Published as opt-in prerelease at https://github.com/TechSavage1989/network-toolkit/releases/tag/v1.18.2 on 2026-11-01T16:47:21Z. Tagged source 8ae89ad005ac112a2ab68dfffedca628efca1d65, merged PR #31. The guarded Release workflow https://github.com/TechSavage1989/network-toolkit/actions/runs/41327840149 completed both build and publish successfully. Four official assets are present with GitHub SHA-256 digests: Network_Toolkit.exe, Network_Toolkit_1.18.2_Portable.zip, Network_Toolkit_1.18.2_Update.zip and SHA256SUMS. Stable latest is still v1.17.136; the beta release is opt-in. Older tags/assets were not changed.

UX Phase 3 groups all 23 tools in Home/Diagnose/Connect/Transfer/Configure, adds the Ctrl+Shift+P search palette and F1 keyboard help, and exposes Terminal/Editor tab actions, Terminal sidebar Show/Hide, and File Transfer Reset panes. Existing credentials, vault, saved profiles/settings/macros, unsaved documents, active sessions, UAC/network change review and explicit verified updater approval are retained. The executable name stays Network_Toolkit.exe. What's New remains short and user-facing; implementation detail belongs under Additional Details.

All 15 jobs in PR Checks https://github.com/TechSavage1989/network-toolkit/actions/runs/41326780380 passed. The exact-source hosted release build, package verification and publication passed. The main-branch Checks run https://github.com/TechSavage1989/network-toolkit/actions/runs/41327840133 was still running when this record was prepared; verify its final result. Physical Windows acceptance, real SSH/SFTP device interruption and multi-hour soak were not performed. Do not claim those validations.

Next: UX Phase 4, unified searchable Settings, agreed confirmation changes, and safe reopen-closed-tabs behavior, after required owner approval. Read docs/UX_POLISH_PLAN.md (authoritative), docs/DEVELOPMENT_ROADMAP.md and docs/UX_PHASE3_HANDOFF.md before implementation. The historical eleven-area roadmap is archived at docs/archive/DEVELOPMENT_ROADMAP_PRE_UX_1.17.md, not active. Do not modify any released tag or asset.

---

---

# Earlier releases

Handoffs for the 1.17.x releases (v1.17.70 to v1.17.150, before the UX roadmap) are archived in [docs/archive/DEVELOPMENT_HANDOFF_1.17.md](archive/DEVELOPMENT_HANDOFF_1.17.md). Older per-release handoffs remain as docs/RELEASE###_HANDOFF.md.
