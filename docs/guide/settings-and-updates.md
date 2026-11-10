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
- Where a setting differed, the main window's setting is used.
- Trusted SSH host keys are combined. If the Terminal window had remembered a different key for a host than the main window, the main window's key is kept and a message lists those hosts. Compare the fingerprint the next time you connect before you accept a changed key.

Nothing is deleted: the earlier files stay in the data folder, and a copy of each file is kept as it was before the change (`Network_Toolkit.before-sharing…`).

**Saved credentials of the earlier Terminal window** are not moved automatically, because that vault may have its own master password. Open **Password manager** and choose **Import saved credentials from the previous Terminal vault**. If that vault has a master password, Windows asks for it. The earlier vault is not changed. Until you import, a connection that uses one of those credentials tells you so.

Use the same version of the portable and the installed program. An older copy keeps working, but it does not share its changes and can overwrite a change another app made at the same moment.

## Windows installation and defaults

Run `Network_Toolkit_<version>_Windows_x64_Setup.exe` from the project's releases page. Choose a program folder on the first install and select **Create a desktop shortcut** if you want to launch Toolkit from the desktop. Setup installs for your Windows account and uses the same folder on upgrades.

Open **Network Toolkit**, **Network Toolkit Text Editor**, or **Network Toolkit Terminal** from Start. Each shortcut has its own icon and opens its own window. The editor and terminal can run while the main Toolkit is closed. Close the app windows before running setup again or uninstalling. Uninstall from **Settings → Apps → Installed apps**. Setup and uninstall leave your saved profiles, credential vault, settings and documents in place.

To use the editor when you double-click a text file, open **Settings → Apps → Default apps**, search for **Network Toolkit Text Editor**, and choose it for each extension you want. Setup registers `.txt`, `.log`, `.ini`, `.cfg` and `.conf`. The Terminal shortcut opens a standalone Network Toolkit Terminal window.

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

## Safety
Before installing and restarting, save unsaved documents and review active sessions. Updates must not silently close or discard work.

## Settings categories (Beta 3)

Open **App settings** to search General, Appearance, Terminal, Editor, Graphs, Transfers, Network, Updates and Confirmations. Existing appearance, timestamp and updater controls are shown in the window. Other categories link to the tool-specific controls, preserving the settings you have already saved.

## Recover eligible recently closed tabs

Press **Ctrl+Shift+T** or use **Search tools and commands** to reopen a recently closed eligible tab. Clean local Text Editor files are reloaded from disk, and unused Terminal draft tabs can be reopened. Active SSH, Telnet and Serial connections, credentials, unsaved document content, and deliberately discarded edits are not stored in tab history. The shortcut does not intercept keystrokes while the Terminal console has focus.

## Undo a saved target removal

Removing a saved diagnostic target offers an **Undo** action for ten seconds. Undo restores the exact entry at its original position unless another entry has reused that ID. Removing credentials, trusted SSH keys and app data still follows existing security requirements.
