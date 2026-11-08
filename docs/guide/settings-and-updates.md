# Settings and Updates

Settings currently appear in the main application and in individual tools. The Phase 1 audit will identify their exact locations and recommend a unified Settings experience.

## Saved data
Network Toolkit retains preferences, saved profiles, credentials in the credential vault and macros. Updates must preserve all of these and migrate older settings without silently resetting them.

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
