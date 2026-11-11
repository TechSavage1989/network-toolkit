# Separate Text Editor and Terminal programs

Owner request (2026-11-11): the full Windows version offers the Text Editor and
the Terminal as separate programs that keep all of their current functionality
and interactions. Chosen form: **full programs built from the same code**,
`Network_Toolkit_TextEditor.exe` and `Network_Toolkit_Terminal.exe`, each with
its own icon, version resource and name in Task Manager, the taskbar and Open
with, shipped in Setup and in the portable ZIP beside `Network_Toolkit.exe`. The
updater updates all three together.

This document records how the code works today and the design decisions, from
the code. Section numbers follow the questions asked before implementation.

## Today

One executable runs as three apps, each its own process with its own loopback
server and window:

| App | Switch | Workspace (under the data root) | Window mode |
|---|---|---|---|
| Main window | none | the data root itself | `main` |
| Text Editor | `--editor-standalone` (`--open-file "<path>"`) | `Network_Toolkit.TextEditor/` | `editor-standalone` |
| Terminal | `--terminal` (`--terminal-preset`, `--open-url "<link>"`) | `Network_Toolkit.Terminal/` | `terminal` |

The data root is per user (`%LocalAppData%\NetworkToolkit`, or the folder chosen
in the app; `resolveAppStorage` ignores the executable's folder). Every app of a
data root shares settings, saved lists, vault, host trust and macros
(docs/SHARED_APP_DATA.md). A standalone app is a single instance per data root:
the lock is `Network_Toolkit.window.lock` in its workspace, and its protected
instance record `Network_Toolkit.instance.json` (DPAPI on Windows) holds the
loopback endpoint, the activation token, an instance id and the PID. A second
start hands its request (connection preset, link, file) to the running instance
through `/api/terminal-window` or `/api/editor-launch` and exits
(`prepareTerminalWindowRequest`, `prepareEditorWorkspace`). Peer send finds the
receiver through the same records (`readPeerInstance`) and starts the other app
with `launchPeerApp`, which runs `updateCurrentExecutable()` with the fixed
`--terminal` / `--editor-standalone` arguments. Nothing in the lock, the record,
the activation, peer send's trust (`readPeerInstance`, `peerForegroundPID`) or
shared data depends on the executable's file name; all of it is keyed by the
data root.

## 1. How the mode is chosen: a build tag

Decision: **a build-time constant**, set by Go build tags, in the one
`cmd/network-toolkit` package:

| Build | Tags | `buildEdition` | Starts as |
|---|---|---|---|
| `Network_Toolkit.exe` | none | `""` | main window (as today) |
| `Network_Toolkit_TextEditor.exe` | `ntk_editor` | `editor-standalone` | Text Editor |
| `Network_Toolkit_Terminal.exe` | `ntk_terminal` | `terminal` | Terminal |

Why not the executable's file name: a renamed or copied file would silently
change what it runs (a `Network_Toolkit.exe` copied as `Terminal.exe` would
stop being the main window, a pinned or registered path could start another
app after a rename, and Windows keeps a renamed image's identity in its
taskbar). With a build constant the program is what it was built as, whatever
its name; the single-instance lock and the data rules are the same in any case
because they are keyed by the data root, not the name.

How a separate program treats switches (`applyBuildEdition` in `edition.go`):

- No switch: its own app (`--terminal` or `--editor-standalone` is implied).
- Its own switches keep working: the Terminal program accepts `--terminal`,
  `--terminal-preset`, `--open-url "<link>"` (as `--open-url` alone or with
  `--terminal`); the Text Editor program accepts `--editor-standalone`,
  `--editor` (treated as the standalone editor) and `--open-file "<path>"`.
  `--data-dir`, `--port` and `--no-browser` work as before.
- Private roles keep working on every build, unchanged: `--terminal-view` and
  `--editor-view` (detached windows of that process), the elevated helpers,
  `--apply-update`, `--update-reopen`, `--update-health`, `--version`.
- A switch for another app is refused with a message naming the program to
  use (the Terminal program never opens the Text Editor, and the reverse), so a
  program's name in Task Manager always matches what it shows.

`Network_Toolkit.exe` keeps every switch. Released shortcuts, registrations,
`--terminal --open-url` links, `Open_Terminal.cmd` and `updateRestartArgs`
keep working unchanged. When `Network_Toolkit.exe` is started as a standalone
app (`--terminal` or `--editor-standalone`) and the matching separate program
of the same version is beside it, it **forwards** the exact same arguments to
that program and exits (`forwardToSeparateProgram`), so the window gets the
separate program's identity wherever it was started from: an older shortcut, a
pinned item, Explorer's Open with or a link. If the separate program is
missing, is not the same version, or does not start, `Network_Toolkit.exe`
runs the app itself exactly as today. The forwarding process passes its
foreground right to the started program (`AllowSetForegroundWindow`) so the
window comes to the front as it did before.

Build mechanics: Go links every `.syso` of every package in a build and refuses
two resource sections ("too many .rsrc sections"). The main resource stays at
`cmd/network-toolkit/resource_windows_amd64.syso` (byte-for-byte unchanged).
Each separate program's resource lives in its own package,
`internal/winres/editor` and `internal/winres/terminal`, imported only by the
tagged edition file. The separate programs are built with an overlay that
removes the main resource from the build (`build.sh`, `BUILD_WINDOWS.cmd`
write it). A build that forgets the overlay fails at link time instead of
producing a program with two resource sets. Tags that name both editions do
not compile.

## 2. Identity on Windows

- **AppUserModelID.** Decision: no explicit AppUserModelID. Windows derives
  each process's default AppUserModelID from its executable path, so each
  separate program groups in its own taskbar button, pins as itself and keeps
  its own (empty) jump list with no code. An explicit ID would have to be
  written into every shortcut (Setup's, the user's pins, older Setup's
  shortcuts) or the running window and its pinned button split into two
  buttons. A standalone app started through `Network_Toolkit.exe` is
  forwarded (section 1), so it groups with the separate program too.
- **Window and class icons.** Every build carries the same three icon groups
  at the same resource IDs (1 Toolkit, 2 Text Editor, 3 Terminal), so the
  native window code (`desktop.IconToolkit/IconEditor/IconTerminal`) and the
  splash need no change: a Terminal window shows the Terminal icon, a detached
  editor window opened from the Terminal shows the editor icon.
- **Icon index 0** (Explorer, Task Manager, Start, a pin): a separate program
  adds one named icon group, `APP`, that refers to the same icon images as its
  own app group (no image is stored twice). Named groups come before numbered
  ones in a resource directory, so `Network_Toolkit_TextEditor.exe,0` is the
  editor tile and `Network_Toolkit_Terminal.exe,0` the Terminal tile. Indexes
  1, 2 and 3 of a separate program are the main program's 0, 1 and 2.
  `Network_Toolkit.exe`'s resource is unchanged, so every index that Setup's
  registrations and shortcuts point at (`Network_Toolkit.exe,1` for the
  editor's document type, Open with and Default apps; `,2` for terminal links)
  still holds the same icon. When the document icon of PR #167 lands it is
  added to the shared group list by `scripts/build_windows_resources.py`, so it
  exists in every build at the index the main program gives it, plus one in
  the separate programs.
- **Version resource** per program (`scripts/build_windows_resources.py`
  writes three): `FileDescription` "Network Toolkit" / "Network Toolkit Text
  Editor" / "Network Toolkit Terminal" (the name Task Manager shows),
  `OriginalFilename` `Network_Toolkit.exe` / `Network_Toolkit_TextEditor.exe` /
  `Network_Toolkit_Terminal.exe`, the same `ProductName` "Network Toolkit",
  `InternalName` "NetworkToolkit", version numbers and manifest. Every build
  therefore passes the updater's existing product/version check
  (`updateValidatePE`), and the new updater tells the three apart by
  `OriginalFilename` (a Terminal candidate can never be installed as the main
  program).
- **Shortcuts and registrations afterwards** (section 6): Setup's Text Editor
  and Terminal Start menu shortcuts point at the separate programs. File
  registrations, Open with, the right-click entry and terminal links keep their
  current commands in this change (they reach the separate program through
  forwarding).

## 3. Interop between the programs

- **Single instance across names.** The lock and the record live in the
  workspace of the data root, so `Network_Toolkit.exe --terminal` and
  `Network_Toolkit_Terminal.exe` compete for the same lock. Whichever starts
  second hands its request to the running one and exits, exactly as two starts
  of the same file do today. No file name is stored in a record.
- **Launch requests** (connection preset, link, file) are unchanged and go
  through the same activation token routes.
- **Starting another app** (`standaloneProgram` in `edition.go`): the main
  window's Separate terminal window (`launchIndependentTerminal`) and peer
  send's `launchPeerApp` start, in this order: the separate program for that
  app beside the running program, if it is a regular file (not a link) of the
  same version; otherwise `Network_Toolkit.exe` beside it with the existing
  fixed switch (the running program itself when it is `Network_Toolkit.exe`);
  otherwise the start fails and the user is told, as for any start failure.
  The arguments are the existing fixed ones (`--terminal --data-dir <root>
  [--terminal-preset ...]`, `--editor-standalone --data-dir <root>`): no text,
  credentials or file paths.
- **Detached windows** (`--terminal-view`, `--editor-view`) are "connected
  separate programs" (owner decision 2026-11-11: a popped-out window has the
  separate program's icon and taskbar name but stays connected to the engine
  that opened it). `launchDetachedTerminal` and `launchDetachedEditor` start
  `detachedViewProgram`: on Windows, the separate program of that app
  (`Network_Toolkit_Terminal.exe` for `--terminal-view`,
  `Network_Toolkit_TextEditor.exe` for `--editor-view`) beside the running
  program when `separateProgramMatches` accepts it (a regular file, not a
  link, whose version resource names this exact version and that program's
  `OriginalFilename`, the check `standaloneProgram` and forwarding use);
  otherwise the running program itself, as before. Linux always uses the
  running program. This applies to every engine: the main window's pop-outs,
  the standalone Terminal's (already the Terminal program) and an editor
  window opened from the Terminal program (the Text Editor program when it is
  there). The child receives the same private stdin bootstrap (owner endpoint,
  view id, scoped view token, profile folder); nothing is on its command line
  but `--terminal-view`/`--editor-view` and `--no-browser`. The page, the
  sessions, the documents and their ownership stay in the opening engine, so
  tabs, unsaved text and live sessions move with nothing reconnected, and
  dock back, SFTP, macros and credentials work as before. The separate
  programs accept the view switches (`applyBuildEdition`: a view never also
  opens the standalone app, and the other app's switches are still refused
  beside it), never forward (forwarding is main program only), and carry the
  same icon groups, so `desktop.IconTerminal`/`IconEditor` still give the
  window its icon. The taskbar button and grouping come from the program's
  path (no explicit AppUserModelID), so a popped-out window groups with the
  standalone app of that program. The same-folder check trusts the separate
  program exactly as much as `Network_Toolkit.exe` itself (it is replaced by
  the same updater and Setup).
- **Send between the windows of one engine** (`window_send.go`,
  `web/assets/window-send.js`): Send to terminal and Send to Text Editor reach
  the other windows of the same engine (main window, detached Terminal
  windows, detached editor windows). Every window's page polls
  `/api/window-send/inbox` under its own token and reports its connected,
  owned sessions and its open documents (names only); the engine keeps only
  sessions the window owns (`ownerLocked`). A send names the window and the
  target; the engine refuses it unless that window is alive (main page
  present, Terminal view `detached`, editor view `ready`), polled within
  `peerTargetsFresh`, lists the target, still owns the session (the ownership
  `withTerminalOwner` checks) or holds the registry's document without a
  pending move, and has no other send waiting (one per window, a minute at
  most). The receiving page runs peer-send.js's `receiveTerminal` /
  `receiveEditor`, which are the in-window paths (`pasteText` with paste
  review, warnings and bracketed paste; `newDocument`/`insertText`/
  `pickInsert`) behind `blocked()` before and immediately before the change;
  its paste is then checked by `withTerminalOwner` like any input. Text only,
  no Enter. Only the main Toolkit engine offers it (`windowSendAvailable`);
  standalone apps' windows keep their refusal, and child windows still have no
  peer routes (`/api/peer-send/` is refused to view tokens).
- **Peer send** trusts protected records only, by their protection, not by a
  file name; `peerForegroundPID` takes the PID from such a record whichever
  program wrote it. Older versions' records (plain Terminal records, engines
  without `/api/peer/`) are skipped and named exactly as today.
- **Document hand-over** (`peer_handover.go`): the main window's engine finds
  the standalone Text Editor through the same protected record, by data root,
  whichever program (`Network_Toolkit_TextEditor.exe` or
  `Network_Toolkit.exe --editor-standalone`) runs it.
- **Terminal link status** (`/api/terminal-links`): "this copy is the
  registered one" also accepts the registration of `Network_Toolkit.exe` beside
  the separate Terminal program, and the separate program's own command, so
  the Terminal program does not report a registration of its own folder as
  another copy.
- **Updates** stay in the main window only (as today). A running separate
  Text Editor or Terminal of the data root is reported as an activity to close
  before installing, because its file is being replaced.

## 4. Updater

### How it works today

1. The main window reads the release feed (`updateParseReleaseCandidate`):
   only `Network_Toolkit_<v>_Update.zip` and `SHA256SUMS` are used, with
   trusted URLs, sizes up to 128 MiB and 64 KiB.
2. Download (after the user clicks), verify the ZIP against the release
   `SHA256SUMS`, then `updateExtractPackage`: every member must be a clean path
   under `Network_Toolkit_<v>/` (at most 3000), and only two members are read:
   `Network_Toolkit_<v>_Windows_x64.exe` (up to 96 MiB) and the inner
   `SHA256SUMS`; the executable must match its inner checksum line and pass
   `updateValidatePE`. **Other members are ignored.** This has been so since
   v1.17.63 (checked in the v1.17.63 and v1.17.90 tags and current main).
3. The executable is staged as `candidate.exe`; installing (after the user
   confirms) copies the **running** program to `installer.exe`, writes
   `plan.json` and starts it with `--apply-update`. The helper is therefore
   the old version's code. It waits for the app to exit, checks hashes,
   replaces the one target file with `ReplaceFileW` (backup kept), starts the
   new program with the old arguments plus `--update-health`, waits up to 90 s
   for the readiness nonce, and rolls back on any failure.
4. The new program's retention keeps one `previous.exe` and cleans the stage.
   Setup's `DisplayVersion` is not rewritten; Setup reads the program's file
   version instead (`RefuseDowngrade`).

### What a released updater does with the new Update ZIP

The first version with separate programs ships an Update ZIP that **keeps the
existing contract** and adds two members:

```
Network_Toolkit_<v>/Network_Toolkit_<v>_Windows_x64.exe             (unchanged)
Network_Toolkit_<v>/Network_Toolkit_TextEditor_<v>_Windows_x64.exe  (new)
Network_Toolkit_<v>/Network_Toolkit_Terminal_<v>_Windows_x64.exe    (new)
Network_Toolkit_<v>/UPDATING.txt, THIRD_PARTY_NOTICES.txt
Network_Toolkit_<v>/SHA256SUMS   (covers all five)
```

v1.21.44 and every release since v1.17.63 accept it: the paths are safe, the
count is far below 3000, the size (about 30 MB) is below 128 MiB, the inner
`SHA256SUMS` lines are well-formed, and the extra members are ignored. The old
updater therefore installs and verifies `Network_Toolkit.exe` only, exactly as
before, and never half-applies anything: there is nothing else it could write.
The new names are not official versioned names
(`updateOfficialExecutableVersion` needs a version right after
`Network_Toolkit_`), so no naming or retention code of an older version treats
them as its own.

### Users on released versions

- **In-app update from v1.21.46 or older**: Network_Toolkit.exe is updated by
  the old helper. Nothing else changes: every shortcut and registration still
  starts `Network_Toolkit.exe`, which runs every app itself because the
  separate programs are not there. No function is lost and no step is needed.
  **Added without asking after the update (v1.21.48, owner decision
  2026-11-11: "it should add them without asking because it's not obvious at
  all that it needs to be done manually").** In the start that follows the
  update, the new version adds the separate programs in the background
  (`installCompanionsAfterUpdate`, `updater_companions_auto.go`): the user's
  approval of the update is the consent. It uses the manual path below
  unchanged: the **same version's** Update ZIP and `SHA256SUMS` from the
  version's own release (the trusted URL checks of the feed), the ZIP verified,
  the ZIP's `Network_Toolkit.exe` byte-identical to the running one (so the
  programs belong to exactly this build), each separate program verified
  against the inner checksums, `updateValidatePE` and its `OriginalFilename`,
  both or neither placed (`updateInstallCompanions`). No restart is needed.
  - **The signal** is what every released helper (v1.17.63 to v1.21.46) gives
    when it restarts the new program: `--update-health <stage folder>` on the
    command line (`updateRestartCommand`) and, once the new window reported
    its start, the confirmation "Installed <version> and confirmed the
    application interface started. ..." in `update-result.txt` of that folder,
    which `awaitUpdateCommit` already waits for. The programs are added only
    after that confirmation (an update that is still being put back adds
    nothing), only in the Windows x64 main window, and only when they are
    missing or not of this version. A plain start (no `--update-health`: a
    lone downloaded exe, a later start, a restart through
    `updateRestartArgs`, which drops the switch) never downloads, nor does
    Linux.
  - **Once per update.** The `--update-health` start happens once; the result
    is kept in that run's memory (`companionsInstall` in the status), never
    in saved settings. A failure (offline, a locked file, any check) leaves
    everything as it was, Network_Toolkit.exe keeps running the apps itself,
    and the updater page shows **Install the separate programs** with the
    reason. It is not retried automatically; the next update installs all
    three anyway.
  - From the first status read of such a start (`--update-health` given and
    the programs missing), the status reports `pending` and the Updates page
    says that they will be added, without the manual button (the engine
    refuses it while pending); the helper's confirmation can take up to 95 s.
    The page reads the status every 2 s while pending or running. A start
    whose update is not confirmed, or that does not qualify, drops `pending`.
    While it runs, the install holds the updater's network slot
    (`networkBusy`), like the manual button, so no check, download or
    "Install with Setup" runs beside it; the page's automatic checks wait and
    every other update action is refused with a plain message. Then the page
    shows "The separate Text Editor and Terminal programs were added." once.
  - **Start menu shortcuts of an installed copy.** After either path placed the
    programs, and only when Setup's uninstall key's `InstallLocation` is the
    running program's folder, the shortcuts **Network Toolkit Text Editor**
    and **Network Toolkit Terminal** in this user's Start menu folder
    "Network Toolkit" are pointed at the separate programs, without arguments,
    with their icon 0, as Setup does. Only Setup's own shortcuts change, by
    Setup's rule (`IsOwnedShortcut`): target `$INSTDIR\Network_Toolkit.exe`
    with `--editor-standalone`/`--editor` or `--terminal`. A shortcut that
    points elsewhere, has other arguments, or is a link is left as it is.
    **Only when the installed uninstaller removes the new form:** an in-app
    update never replaces `Uninstall.exe`, and the uninstaller of 1.21.46 or
    older recognises only shortcuts that start `Network_Toolkit.exe`, so it
    would leave retargeted shortcuts (and the folder) behind. The uninstall
    key's `DisplayVersion` is written only by Setup, so it is the version of
    the uninstaller; it must be a strict `x.y.z` of at least 1.21.47 (the
    first Setup with the separate-program form in `IsOwnedShortcut`).
    Otherwise, or when it cannot be read, the shortcuts are left for the next
    Setup and the Updates page says so (`startMenu: "next-setup"`); they keep
    working through forwarding. The
    change goes through the Shell's IShellLinkW (target, arguments and icon
    only), a copy is saved outside the Start menu and checked, and the
    original is overwritten only while it still holds the bytes that were read
    (`updateShortcutCommitNative`). A failure is reported on the Updates page
    and never undoes the programs: such a shortcut keeps working through
    forwarding.
  - **Document icon.** The same first start points the Text Editor ProgID's
    `DefaultIcon` at the document icon (`"<program>",3`) when its
    `shell\open\command` starts the running `Network_Toolkit.exe` (Setup's or
    the portable registration), as Setup does on every install; an in-app
    update from 1.21.44 or older left the editor icon (1). Only that value is
    written, then `SHChangeNotify(SHCNE_ASSOCCHANGED)`.

  The updater page offers **Install the separate Text Editor and Terminal
  programs** when they are still missing or not of this version (Windows main
  window): after a failure above, or for a copy that was not updated in-app.
  It is an explicit action like Download, with the same path and checks.
  Users who do not choose it get them with the next update, which installs
  all three (below).
- **Setup over v1.21.46 or older**: installs all three and points its owned
  Text Editor and Terminal shortcuts at the separate programs (section 6).

### New updater (the first version with separate programs and later)

- Extract: the main program as before, plus each separate program present in
  the ZIP (inner checksum, `updateValidatePE`, `OriginalFilename`). Both or
  neither must be present. A ZIP without them (only an older format) is
  refused when separate programs are installed beside the running program, so
  the three can never end up at different versions.
- Stage: `candidate.exe` plus `candidate-<name>` per separate program; the
  status shows each SHA-256.
- Plan: a new optional `companions` list: target name, the installed file's
  SHA-256 (empty when it is not installed yet) and the candidate's SHA-256.
  `plan.json` from older helpers has no such field and reads as before.
- Apply (the helper is this version's code): after the app exits, every
  installed file and every candidate is checked against the plan first; then
  the main program is replaced as today and each separate program is replaced
  (`ReplaceFileW`, backup kept) or created (no-replace move). If any step
  fails, every step already done is undone in reverse (backups restored,
  created files removed) before the previous version restarts. The same full
  undo runs when the new program does not confirm its start within 90 s.
  After a confirmed start the separate programs' backups are kept in the stage
  folder until retention cleans it, like `previous.exe`.
- Restart: the helper restarts the program the user installed from with that
  program's own arguments (the plan's target and args, as today). Updates are
  installed from the main window, so that is the main window.
- Portable and installed copies use the same updater; the installed copy's
  uninstall key is not touched (Setup reads the file version, which the new
  program carries).

Conclusion: the transition is safe for users of released versions without any
step by the owner.

## 5. Release assets and tooling

- `build.sh` (and `BUILD_WINDOWS.cmd`) build `build/Network_Toolkit.exe`,
  `build/Network_Toolkit_TextEditor.exe` and `build/Network_Toolkit_Terminal.exe`.
- `scripts/github_release.py package` takes `--editor-exe` and
  `--terminal-exe`. The **portable ZIP** holds the three programs at its top
  level (its `Open_Terminal.cmd` starts the Terminal program); verify requires
  exactly those three executables. The **Update ZIP** is as in section 4
  (verify: exactly six members). `validate` checks all three resource objects
  and their names; `verify` checks each program's version and
  `OriginalFilename`.
- **Release assets stay the same seven** (`Network_Toolkit.exe`, Portable
  ZIP, Update ZIP, Setup, two Linux archives, `SHA256SUMS`): the separate
  programs are delivered inside the Portable ZIP, the Update ZIP and Setup,
  so the asset list, the release workflow's upload list and the "seven assets"
  check in the handoff do not change.
- **Linux**: no separate binaries. The Linux edition opens its apps as browser
  windows, so a separate binary gives no taskbar or Task Manager identity, it
  has no standalone Terminal menu entry and no file registrations, and the
  archives would double in size for nothing visible. The code builds with the
  tags on Linux (the tests use that), and every launch falls back to the
  switch when the separate program is absent, so nothing changes for Linux
  users.
- **Size**: each Windows program is about 21 MB. The portable ZIP and the
  Update ZIP grow by about two compressed programs; Setup grows less because
  it compresses the three nearly identical programs together with a larger
  LZMA dictionary. Measured numbers are recorded in the appendix.

## 6. Setup

- Installs the three programs: all are extracted to staged temporary files
  first; before any is placed, each existing program file is checked to be
  closed (opened for exclusive write, retried about five seconds like
  `PlaceStagedFile`); then the separate programs are placed before
  `Network_Toolkit.exe`, each through `PlaceStagedFile` with its own
  `$FailedStep`. A failure therefore leaves `Network_Toolkit.exe` at the old
  version, so `RefuseDowngrade` lets the same Setup run again. A fresh setup
  rolls back every file it placed.
- A fresh setup refuses a folder that already holds any of the three programs.
- Start menu: **Network Toolkit Text Editor** and **Network Toolkit Terminal**
  point at the separate programs, without arguments, with their icon 0. An
  upgrade replaces those shortcuts only when they are Setup's own: the
  ownership check (`IsOwnedShortcut`) accepts the released form
  (`Network_Toolkit.exe` with `--editor-standalone`/`--editor` or
  `--terminal`) and the new form (the separate program without arguments). A
  shortcut that points elsewhere is kept. Uninstall removes owned shortcuts of
  either form.
- Uninstall: checks that none of the three programs is open before removing
  anything ("Close Network Toolkit and any separate Editor or Terminal windows
  before uninstalling."), then removes all three with the existing retry and
  wait.
- **Registrations: keep the current commands in this change.** File types,
  Open with (`Applications\Network_Toolkit.exe`), the right-click entry, Default
  apps and terminal links keep `"Network_Toolkit.exe" --editor-standalone
  --open-file "%1"` and `"Network_Toolkit.exe" --terminal --open-url "%1"`.
  Forwarding (section 1) already opens them in the separate programs. Changing
  them now would collide with `feat/portable-windows-integration`, would move
  Open with to a new `Applications\Network_Toolkit_TextEditor.exe` key that the
  created-key records and the upgrade adoption would have to cover, and would
  break links for users who updated in-app (they have no separate programs
  until they install them). Recommendation for that later PR: switch the
  commands to `"Network_Toolkit_TextEditor.exe" --open-file "%1"` and
  `"Network_Toolkit_Terminal.exe" --open-url "%1"` with icon index 0 of those
  programs, only in Setup (which always installs them), keep recognizing and
  removing the old commands, and keep `Network_Toolkit.exe` accepting both
  forms.

## 7. Data and settings

Nothing changes: the data root, the two workspaces, every settings key, the
vault, host trust, macros, the editor session and the instance records are the
same whichever program runs an app, because none of them depends on the
executable. Switching between `Network_Toolkit.exe --terminal` and
`Network_Toolkit_Terminal.exe` (in either direction, or between versions) opens
the same Terminal workspace with the same data, and both cannot run at once
for one data root. The update rollback slot (`updates/<hash of the program
folder>`) is per folder, so all three programs of a folder share it.

## Review follow-ups

- **Start check.** The update helper runs each separate program it placed
  with `--version` (it must exit 0 and name the installed version within
  30 s) before it starts the new main program; if one does not start, every
  file is put back. `Network_Toolkit.exe` that forwards a standalone app
  watches the separate program for 3 s and runs the app itself when that
  program exits with a failure in that time (a successful exit is a
  hand-over, a running program owns the window; neither is started twice).
- **Unconfirmed update.** Until the helper confirms the update, the restarted
  main window opens no separate terminal window, so undoing the update does
  not meet a separate program it started.
- **Setup over left-behind programs.** An uninstaller of 1.21.46 or older
  leaves the separate programs behind after an in-app update added them. Only
  `Network_Toolkit.exe` or `Uninstall.exe` make a folder another installation;
  left-behind separate programs are replaced once they are closed.
- **Release gate.** `FIRST_SEPARATE_PROGRAMS` is 1.21.47: the published
  1.21.44 to 1.21.46 still verify with one program.

## Known limits

- **Open windows are found by data root.** The updater's "close these first"
  check looks for the separate Text Editor and Terminal windows of the data
  root, not of the program folder. A Terminal of the same data root opened
  from another portable copy is listed too (close it or ignore the list and
  close it later); one that uses another data root is not, and if it runs
  from this folder its file stays in use, so the update fails and puts every
  file back.
- **Download time under released updaters.** The Update ZIP is about 26 MB.
  Released updaters download it with their existing 5-minute limit, as they
  did the 8.8 MB package: on a connection slower than about 90 KB/s the
  download stops with the existing message and nothing changes; the portable
  ZIP or Setup remain available.
- **A lone program gets company.** A copy of `Network_Toolkit.exe` kept on its
  own (for example the single-file release asset) is offered **Install the
  separate programs**, and the next in-app update adds the two separate
  programs beside it (in the update or, from a helper of 1.21.46 or older, in
  the start after it), because releases ship all three together.
- **Legacy versioned file names.** When the update was installed under an
  old versioned file name and the new program reopens itself as
  `Network_Toolkit.exe` (`reopenPromotedUpdate`), the reopened run has no
  `--update-health`, so the separate programs are not added automatically;
  the Updates page offers the button.
- **Pinned and other shortcuts.** Only Setup's two Start menu shortcuts are
  changed. Taskbar pins, desktop or user-made shortcuts that start
  `Network_Toolkit.exe --terminal` or `--editor-standalone` keep working
  through forwarding.
- **Programs started outside the main window during an unconfirmed update.**
  A separate program started from the Start menu, or through a link, while the
  helper waits for the new version is not stopped by it; if the update is then
  put back, that file is in use and the helper reports which file to restore
  (as it does for the main program).

## Tests

- Go: edition rules for each build, forwarding and launch selection
  (separate program present, missing, other version, a link), single instance
  and hand-over across program names with real binaries, peer send starting
  the separate program and falling back, update extraction of the old and new
  ZIP formats, the install transaction with injected failures at every step
  and rollback of all three, restart arguments, and the separate-program
  installation for users who updated from a released version (local fixture
  server with synthetic release assets).
- Python: release packaging (assets, ZIP members, `SHA256SUMS`,
  `validate`/`verify`) and the per-program resources (icon groups, index 0,
  version strings, the main resource unchanged).
- `scripts/installer/verify-windows.ps1` (hosted Windows only): three programs
  installed, shortcuts, upgrade from a released Setup, uninstall, and a running
  separate program blocking uninstall.

## Appendix: measured sizes

Builds of this change (Go 1.26.11, `sh ./build.sh`, `github_release.py package`):

| Item | Before | With the separate programs |
|---|---|---|
| Each Windows program | 22.9 MB | 22.9 MB (three programs) |
| One program compressed in a ZIP | 8.8 MB | 8.8 MB each |
| Update ZIP | about 8.8 MB | 26.3 MB |
| Portable ZIP | about 19 MB | 37.2 MB |
| Setup | 7.08 MB | 7.66 MB (64 MB LZMA dictionary: the second and third program cost 0.6 MB) |
| Linux archives | unchanged | unchanged |

Setup was also tried without the larger dictionary: 20.9 MB.

Wine (not Windows) was used to run the Windows builds: `--version` of each
program; `Network_Toolkit.exe --terminal` forwarding to
`Network_Toolkit_Terminal.exe` (the PE version check passed) with a DPAPI
record written, and a second start handing over; a fresh silent Setup, an
upgrade and an uninstall (three programs, shortcuts at the separate programs
without arguments, byte-identical files). Wine does not lock a running
program file as Windows does, so the open-program refusals are checked only by
`scripts/installer/verify-windows.ps1` on hosted Windows.
