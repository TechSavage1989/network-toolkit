# Settings and Updates

Settings currently appear in the main application and in individual tools. The Phase 1 audit will identify their exact locations and recommend a unified Settings experience.

## Saved data
Network Toolkit retains preferences, saved profiles, credentials in the credential vault and macros. Updates must preserve all of these and migrate older settings without silently resetting them.

## Updates
Use **Check for updates** for an on-demand check. Stable updates and opt-in beta updates use separate channels. A beta must be explicitly selected. Downloading and installing updates requires user consent and package checksum verification.

Automatic **checks** run at startup and every six hours while enabled. The manual Check for updates remains available, and downloading and installing still require explicit approval.

## Safety
Before installing and restarting, save unsaved documents and review active sessions. Updates must not silently close or discard work.

## Settings categories (Beta 3)

Open **App settings** to search General, Appearance, Terminal, Editor, Graphs, Transfers, Network, Updates and Confirmations. Existing appearance, timestamp and updater controls are shown in the window. Other categories link to the tool-specific controls, preserving the settings you have already saved.

## Recover eligible recently closed tabs

Press **Ctrl+Shift+T** or use **Search tools and commands** to reopen a recently closed eligible tab. Clean local Text Editor files are reloaded from disk, and unused Terminal draft tabs can be reopened. Active SSH, Telnet and Serial connections, credentials, unsaved document content, and deliberately discarded edits are not stored in tab history. The shortcut does not intercept keystrokes while the Terminal console has focus.

## Undo a saved target removal

Removing a saved diagnostic target offers an **Undo** action for ten seconds. Undo restores the exact entry at its original position unless another entry has reused that ID. Removing credentials, trusted SSH keys and app data still follows existing security requirements.
