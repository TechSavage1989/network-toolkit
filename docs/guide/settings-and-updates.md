# Settings and Updates

Settings currently appear in the main application and in individual tools. The Phase 1 audit will identify their exact locations and recommend a unified Settings experience.

## Saved data
Network Toolkit retains preferences, saved profiles, credentials in the credential vault and macros. Updates must preserve all of these and migrate older settings without silently resetting them.

## One set of settings and saved data for all apps
Network Toolkit, the **Terminal** window and the **Text Editor** window share their settings and saved data. It does not matter whether you start them from the portable folder or from the installed shortcuts: for the same Windows account they all use the same data folder.

- **Saved data**: saved and recent connections with their folders and order, saved credentials, trusted SSH host keys, and macros.
- **Settings**: the light or dark theme and accent color, Terminal settings (appearance, syntax highlighting and your own rule sets, features, mouse and keyboard, confirmations), and Text Editor preferences (word wrap, font size, colors, reopening files).

A change made in one app shows up in the others within a few seconds, without a restart. If you have a dialog open or are typing, the change waits until you are done. Two apps can be open and changed at the same time; neither undoes what the other saved.

Each window still keeps what is only about that window: its size, which sidebar tab and panels are open, split panes, and what you have typed into forms.

The saved credentials are in one vault. When you unlock it in one app, the other apps ask for the master password themselves unless you let Windows remember the unlock; **Lock and forget** locks it in all of them.

### The first start after updating
Earlier versions gave the Terminal window and the Text Editor window their own copies of connections, settings, trusted SSH keys and macros. The first time you start a version that shares them, Network Toolkit brings those copies together:

- Connections and macros that only one app had are added for all of them. If both had changed the same one, both versions are kept; the second is named with the app it came from, for example **Core switch (Terminal)**. Remove the one you do not need.
- Where a setting differed, the main window's setting is used. Two Text Editor settings are the exception: **Reopen files and unsaved changes from the last session** and **Ask before closing unsaved documents** stay on if either app had them on, so unsaved text is neither hidden nor discarded by the change.
- Trusted SSH host keys are combined. A message lists the hosts that only the Terminal or Text Editor window had trusted, because they are trusted by every app now; to stop trusting one, open the Terminal's **Saved** sidebar, choose **SSH host keys** and remove it. If the Terminal window had remembered a different key for a host than the main window, the main window's key is kept and the message lists those hosts too. Compare the fingerprint the next time you connect before you accept a changed key.

Nothing is deleted: the earlier files stay in the data folder, and a copy of each file is kept as it was before the change (`Network_Toolkit.before-sharing…`).

**Saved credentials of the earlier Terminal window** are not moved automatically, because that vault may have its own master password. Open **Password manager** and choose **Import saved credentials from the previous Terminal vault**. If that vault has a master password, Windows asks for it. The earlier vault is not changed. Until you import, a connection that uses one of those credentials tells you so. If the shared vault is full, the message names the credentials that did not fit; remove some you no longer need and import again.

### Lists that are full
You can save up to 200 sessions, 200 FTP connections, 200 targets and 100 TFTP profiles. When a list is full, saving another entry is refused with a message and nothing is changed; an existing entry can still be edited. If two apps add an entry at the same moment and only one fits, the other app tells you which entry was not saved.

### Using an older copy at the same time
Use the same version of the portable and the installed program. A copy from before this change keeps working, but it does not merge its changes with what the other apps saved:

- While an older main window is open, each of its saves puts back the settings and lists it holds, so a change you made in an updated app can be replaced until that window is closed. An updated app that notices this tells you once.
- Saved sessions, FTP connections, targets and TFTP profiles are never lost this way. An older copy can show at most 100 saved sessions; whatever it drops is put back by the updated apps, and a message names what was put back. For the same reason an entry you removed in the older copy comes back: remove it again in an updated app.
- What you add in an older Terminal or Text Editor app (saved sessions, macros, trusted hosts) appears in the updated apps the next time one of them starts.

Updating every copy ends all of this.

## Windows installation and defaults

Run `Network_Toolkit_<version>_Windows_x64_Setup.exe` from the project's releases page. Choose a program folder on the first install and select **Create a desktop shortcut** if you want to launch Toolkit from the desktop. Select **Right-click menu: Edit with Network Toolkit Text Editor** if you want that entry for every file in Explorer. Setup installs for your Windows account and uses the same folder on upgrades.

Open **Network Toolkit**, **Network Toolkit Text Editor**, or **Network Toolkit Terminal** from Start. Each shortcut has its own icon and opens its own window. The editor and terminal can run while the main Toolkit is closed, show only their own tools, and can send text to each other ([Text Editor app](editor.md#the-text-editor-app), [Terminal app](terminal.md#the-terminal-app)). Close the app windows before running setup again or uninstalling. Uninstall from **Settings → Apps → Installed apps**. Setup and uninstall leave your saved profiles, credential vault, settings and documents in place.

To use the editor when you double-click a file, open **Settings → Apps → Default apps**, search for **Network Toolkit Text Editor**, and choose it for each extension you want. The file you double-click opens as the selected tab, next to the tabs the editor reopens from last time. Setup lists the editor for text, configuration, script, data and source code types and offers it under **Open with** for every other file, including files without an extension; see [Make the Text Editor a default app in Windows](editor.md#make-the-text-editor-a-default-app-in-windows) for the list and the steps. Setup never picks a default for you. The Terminal shortcut opens a standalone Network Toolkit Terminal window. Without installing, the portable program can register its own Text Editor the same way from **App settings → Editor → Use this copy's Text Editor in Windows**; while the installed program is registered, a portable copy leaves that registration alone, and Setup takes over a portable copy's registration ([details](editor.md#portable-copy-use-its-text-editor-in-windows)).

Setup also lists **Network Toolkit Terminal** in Default apps for SSH, Telnet and serial links (`ssh://`, `telnet://`, `serial:`). To use it, choose **Make Network Toolkit the default for SSH, Telnet and serial links** in Terminal settings → Features → Links, or search for Network Toolkit Terminal in **Settings → Apps → Default apps**. Setup never picks a default for you and does not replace another app's link handler; uninstall removes only what setup added. A link always opens the connection form for you to confirm, and passwords in links are ignored. See [Open SSH, Telnet and serial links](terminal.md#open-ssh-telnet-and-serial-links). The Terminal runs Command Prompt and PowerShell in its own tabs; it does not replace the Windows default terminal.

## Linux: applications menu
In the Linux edition, **App settings → General → Applications menu** adds Network Toolkit to your desktop's applications menu (only files in your own account are written: a menu entry and the icon). If you move the program folder, choose **Update menu entry**. **Remove from menu** takes it out again.

## Customize the Overview
On the Overview, choose **Customize** to show or hide its sections (summary cards; gateways, DNS and DHCP; public IP; network scan; adapters; recent activity) and move them up or down. **Reset to default** restores the original layout. You can also drag a section: hover it and drag the small handle on its top edge (or focus the handle and press Alt+Shift+Up or Down). Your layout is kept when you restart the app.

## Password manager and the credential vault
Open **Password manager** (from a connection's Saved credential field, or at the bottom of the Terminal's Saved sidebar) to store a username together with its password or SSH key. Choosing a saved credential in a connection fills in its username.

When you set up the vault, choose one of two protections:
- **With a master password.** Any password is allowed. A strength gauge in the Windows dialog shows how hard it is to guess. You can also create an offline recovery key and let Windows remember the unlock.
- **Without a master password.** The vault opens automatically for your Windows account. Other software running as you, or an administrator, may be able to read saved credentials.

You can switch later: **Add master password** or **Remove master password** in Password manager. Your saved credentials are kept either way.

## Updates
Use **Check for updates** for an on-demand check. Stable updates and opt-in beta updates use separate channels. A beta must be explicitly selected. Downloading and installing updates requires user consent and package checksum verification.

Automatic **checks** run at startup, every 30 minutes while the app is running, and when you return to the app after 10 minutes away, while enabled. When a check finds a newer version, a notice appears at the top of the app and a short message tells you once per version. The manual Check for updates remains available, and downloading and installing still require explicit approval.

An update installs `Network_Toolkit.exe` and, beside it, the separate Text Editor and Terminal programs (`Network_Toolkit_TextEditor.exe`, `Network_Toolkit_Terminal.exe`) together: all three are checked first, and if one cannot be replaced (for example because it is open) none is changed. Close the separate Text Editor and Terminal windows before installing; the updater lists them if they are open. A copy updated from version 1.21.46 or older gets the new `Network_Toolkit.exe` only, which keeps opening the Text Editor and the Terminal itself. Right after that update, the new version adds the separate programs by itself in the background: it downloads them from the same version's release and checks them before adding them, and the **Updates** page shows that they are being added, then "The separate Text Editor and Terminal programs were added." In an installed copy, the Start menu shortcuts that Setup created for the Text Editor and the Terminal then open the separate programs; shortcuts you changed are left as they are. If the copy was installed with Setup 1.21.46 or older, the shortcuts keep opening the apps through `Network_Toolkit.exe` (which hands them to the separate programs) until you next run Setup, so that uninstalling still removes them. This happens only after an update you approved, never when you simply start the app. If it cannot finish (for example without a connection), nothing changes, the app works as before, and the **Updates** page offers **Install the separate programs** with the reason; it is not tried again automatically. The next update installs them in any case. The same first start also gives text files associated with the Text Editor their document icon again, if the registration still showed the editor icon.

### Install the full version from the portable copy
The portable program can install the full version for you. Open **App settings → Updates → Install the full version** (shown in the portable copy on Windows only). The installed version adds Start menu entries for Network Toolkit, its Text Editor and its Terminal, lets Windows offer the Text Editor for files and the Terminal for SSH, Telnet and serial links, and comes with an uninstaller. It uses the same data folder, so your settings and saved data stay as they are, and the portable copy is left where it is.

1. **Find Setup for version …** looks on the official release page for `Network_Toolkit_<version>_Windows_x64_Setup.exe` of the version you are running. If that release has no Setup, the latest release of your update channel (stable, or beta when you opted in) is offered instead, but only when it is newer than your version; the section names both versions and says that installing it also updates the program. An older release is never offered, because it could save over settings the newer version wrote. Otherwise the section says that no installer was found for your version.
2. **Download Setup…** asks first. Only after you choose **Download and verify** is Setup downloaded, with its size and progress shown, and checked against that release's `SHA256SUMS` exactly as updates are. If the check fails, nothing is kept. Only GitHub's release addresses that the updater uses are contacted.
3. **Start Setup…** asks again. Only after you choose **Start Setup** does Setup open, in its own window, as the normal interactive Setup: it asks for a folder and options before it installs anything. The file is checked once more just before it starts; a file that changed is deleted instead.
4. When Setup has finished, open Network Toolkit from the Start menu. You can then close the portable copy and delete its folder. If you had registered the portable copy's Text Editor with Windows, Setup takes that registration over (see [Text Editor](editor.md#portable-copy-use-its-text-editor-in-windows)).

**Already installed?** The section says where, and **Open the installed program** starts it. **Reinstall or repair anyway** still offers Setup.

**Data folder.** The portable and the installed program use the same data folder: `%LocalAppData%\NetworkToolkit`, or the folder you chose under **App settings → General → App data location**, because that choice is saved in `%LocalAppData%\NetworkToolkit` where both read it. Only a portable copy started with `--data-dir <folder>` uses a folder the installed program does not know; the section says so. After installing, open **App settings → General** in the installed program, enter that folder as the data folder (clear **Copy current saved settings**, because the folder already holds your data) and restart it, or start the installed program with the same `--data-dir`.

A downloaded Setup is kept in `%LOCALAPPDATA%\NetworkToolkit-Setup`, in a folder only your Windows account can open, never in the data folder (which you may have put where others can write). **Delete the download** removes it, and the next download removes earlier ones. Before Setup starts, the folder is checked again and Setup is read once more, from a handle that stops anyone from changing or replacing the file until Setup is running; if anything changed, Setup is not started.

## Safety
Before installing and restarting, save unsaved documents and review active sessions. Updates must not silently close or discard work.

## Settings categories (Beta 3)

Open **App settings** to search General, Appearance, Terminal, Editor, Graphs, Transfers, Network, Updates and Confirmations. Existing appearance, timestamp and updater controls are shown in the window. Other categories link to the tool-specific controls, preserving the settings you have already saved.

## Recover eligible recently closed tabs

Press **Ctrl+Shift+T** or use **Search tools and commands** to reopen a recently closed eligible tab. Clean local Text Editor files are reloaded from disk, and unused Terminal draft tabs can be reopened. Active SSH, Telnet and Serial connections, credentials, unsaved document content, and deliberately discarded edits are not stored in tab history. The shortcut does not intercept keystrokes while the Terminal console has focus.

## Undo a saved target removal

Removing a saved diagnostic target offers an **Undo** action for ten seconds. Undo restores the exact entry at its original position unless another entry has reused that ID. Removing credentials, trusted SSH keys and app data still follows existing security requirements.
