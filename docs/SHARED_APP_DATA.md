# Shared app data

Settings and saved data are shared by every Network Toolkit app that uses the same data folder: the main window (the portable program and the installed one), the standalone Terminal and the standalone Text Editor. A connection saved in one app is there in the others; a setting changed in one takes effect in the others. This note describes where the data lives, how saves from several running apps are combined, the one-time migration, and what happens when something fails.

## What is shared

| Shared by every app | Where |
| --- | --- |
| Saved and recent connections (terminal, FTP/FTPS, SFTP), manual order, learned prompt names, verified SFTP fingerprints | `Network_Toolkit.settings.json` in the data folder |
| Settings: theme and accent, color picker and diagnostic colors, all Terminal settings (appearance, highlighting rule sets, features, mouse and keyboard, font size, confirmations, sidebar side), Text Editor preferences and its file sidebar, time format | the same file |
| Saved credentials | `Network_Toolkit.credentials.v2.vault` (one vault per data folder) |
| Trusted SSH host keys | `Network_Toolkit.ssh_known_hosts.json` |
| Macros and startup macros | `Network_Toolkit.macros.json` |
| Open editor tabs and kept copies of unsaved text | `Network_Toolkit.editor-session/` per editor workspace, taken over by the other editor when its app is not running |

The portable and the installed program already used the same data folder (`%LocalAppData%\NetworkToolkit`, or the folder chosen under **App data**; see `storage_location.go`). The standalone apps now use that folder's files too, instead of copies in `Network_Toolkit.Terminal/` and `Network_Toolkit.TextEditor/`. Those workspace folders hold only what is about one window.

**The key table.** `sharedDataKeys` in `cmd/network-toolkit/shared_data.go` lists every shared settings key with how it is merged; `perWindowStateKeys` lists every key that stays with one window. A key in neither table is treated as per-window, and `TestSharedKeyTableClassifiesEveryPageKey` and the browser check fail until it is classified. Classify a new key as shared if the user sets it on purpose, and as per-window if it is about one window or one run.

Per-window (each window keeps its own): the active page and selected tabs, which panels are open or collapsed, panel and sidebar sizes, the pane layout, form drafts and inputs (the connection form, diagnostic forms, calculators), current filters and searches, the first-launch marker, update reminders, and one-time upgrade markers. Keys marked `MainOnly` (saved diagnostic targets, saved result filters, TFTP profiles, file-server settings, chosen result columns, the Overview layout) are shared between the portable and the installed main window; a standalone app neither receives nor writes them.

## Saving from several apps

Each page keeps all its settings in memory and saves all of them. A save is therefore never taken as the new truth.

1. The page remembers the shared values it and the engine last agreed on. With a save it sends, in `_sharedBase`, the earlier value of every shared key it changed. Keys it does not list were not changed by it.
2. The engine takes the data folder's lock (`Network_Toolkit.credentials.lock`, the lock earlier versions already used for the settings file), reads the settings file as it is now, and for each changed key merges three ways: the page's value, the value the page started from, and the value on disk.
   - **Lists of entries with an id** (connections): entries the page added, removed or changed are added, removed or changed on top of the list on disk. An entry changed on both sides keeps the page's version. A change beats a removal, so an edit is never dropped. The page's order is used if the page reordered the list.
   - **Settings objects**: merged member by member, with the same rules. Two apps that change different options of one dialog both keep their change.
   - **Single values**: the page's value is taken only if the page changed it.
3. The result is written with an atomic replace. If it differs from what the page sent, the answer carries the merged values and the page applies them.

Every 2.5 seconds, while its window is visible, a page asks `GET /api/shared-data?revision=…`. If another app changed something, it gets the current values of the keys marked `Live` and applies them: lists are redrawn, the theme, Terminal settings and editor preferences take effect at once. A change waits while the user has unsaved changes, an open dialog or menu, a drag, or is typing in a field it would redraw. Keys not marked `Live` are read when a page starts.

A client that sends no `_sharedBase` is taken to have started from what the engine last served to it or accepted from it.

**Lists are never cut short.** Because a list is merged by entry, a page that sent a shorter list than it was given would be read as "the user removed these". The key table therefore gives every list one of two rules, and the page (`SAVED_LIMITS` and `ROLLING_LIMITS` in `web/assets/shared-data.js`) uses the same numbers; `scripts/shared-list-limits.test.cjs` compares them and fails on any page statement that shortens a saved list.

- A **saved list** (`MaxSaved`: saved sessions 200, saved FTP connections 200, saved targets 200, saved TFTP profiles 100) is loaded whole and saved whole. No sanitizer and no load path shortens it, even when it holds more than the number. When it is full, the page refuses another entry in plain words and changes nothing. If two apps add at the same moment and only one entry fits, the engine saves the first, leaves the other out, and tells that page which entry was not saved (`sharedData.refused` in the answer); the page removes it from its list and says so.
- A **rolling list** (`Limit`: recent connections 50, learned prompt names 200) keeps its newest entries; the oldest leave on their own.

The 2 MiB limit of a settings file is unchanged, as are all sanitizers; they run on the page's values, on the earlier values it sends, and on what is read from disk.

## Other shared stores

- **Credential vault.** One vault per data folder, bound to the configuration identity in the data folder's settings file. Unlock rules are unchanged: a master password, a remembered unlock for this Windows account, or the no-master-password mode. A vault file another app saved is taken over only if it is the same vault generation and authenticates with the key this app already holds; otherwise the app starts over from the files on disk, as at start (locked unless a valid remembered unlock exists). A save that finds a newer vault redoes its change on that vault.
- **SSH host trust.** Every check and every change reads the trust file again if it changed, and changes are made under `Network_Toolkit.shared-data.lock`. A decision made in another app while a prompt was open is never replaced. A trust file that cannot be read disables SSH, as at start.
- **Macros.** The library keeps its revision check; a save is read-compare-write under the same lock.
- **Editor sessions.** Each process that uses its editor session holds a shared lock on `Network_Toolkit.editor-session.lock` in its workspace. An editor that restores its session takes over the other workspace's tabs only if it gets that lock exclusively (and no older version's window answers). A tab's record leaves the other index only after this index holds it; a kept copy is removed there only after it exists here. A file open in both with unsaved text keeps both versions.

  "Reopen files next time" (`editorPreferences.restoreSession`) is a shared setting, so it can be off in an app whose last run kept unsaved text, and it can change while an app runs. Kept text is never left hidden and never removed unseen:
  - An app that starts with the setting off still reads its session and reopens every document with unsaved text, once (saved files are not reopened). After a document is open in the window, its kept copy leaves the session and closing asks as usual.
  - When the setting goes off while an app runs (here or in another app), the kept copies of the documents open in that window leave the session; records that could not be reopened stay. Switching it on again keeps those records too.
  - The engine enforces this whatever a page does: until a run has been given the session (`GET /api/editor-session`), it removes only kept copies that run wrote itself and refuses (409) a save or delete that would drop the record of any other kept copy. After an unreadable index was set aside, deleting the session keeps that index and the copies it may name.

## One-time migration

At start, `migrateSharedData` (`shared_migration.go`) adds what the standalone workspaces kept for themselves to the data folder:

- Connections and macros the data folder does not have are added. One with the same id but different content is added as a second entry named "… (Terminal)". Nothing in the data folder is overwritten.
- A setting that differs keeps the data folder's value; a setting only the workspace had is taken over. Highlighting rule sets written in the standalone window are kept next to the data folder's.
- Two Text Editor settings are the exception, because they decide what happens to unsaved text: for "Reopen files next time" and "Ask before closing unsaved documents" the safer value (on) wins when the data folder and a workspace differ. This is decided once per workspace, so a later choice of the user stays.
- Trusted SSH hosts are united. If the two had different keys for a host, the data folder's key stays, the other is not trusted, and the user is told which hosts to check.
- Credentials are not moved automatically (the previous vault may have another master password). The password manager offers **Import saved credentials from the previous Terminal vault**; see `credential_vault_import.go`.

The workspace files are only read and stay where they are. Each data-folder file is copied once, untouched, before its first change (`Network_Toolkit.before-sharing.*`). `Network_Toolkit.shared-data.json` records what was handled, so a repeated run does nothing and an entry the user later removes does not come back. If an older version of the app later changes a workspace file, only its new entries are added.

## When something fails

- **A crash during a save**: every file is replaced atomically; the previous file stays until the new one is complete.
- **A crash during the migration**: each part can be repeated. The next start completes it and adds nothing twice.
- **One part of the migration fails** (for example a damaged macro library): the other parts are done, the failure is logged, and the part is tried again at the next start.
- **The data folder's settings file cannot be read**: a standalone app saves no shared data over it and reports the problem; the file is not changed.
- **Another app holds the lock**: a save waits up to five seconds, then reports that another app is saving.
- **An older version runs at the same time**: it keeps working on its own copies as before. An older main window can overwrite a change another app made in the same moment, because it saves without merging. Update all copies of the app to avoid this.

Moving the data folder and deleting app data include the shared files, the migration record and the untouched copies. Both are refused while another app of the data folder is running.
