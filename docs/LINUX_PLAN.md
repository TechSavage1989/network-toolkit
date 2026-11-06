# Linux edition plan

Owner request (2026-11-06): "can we get this app now developed for Linux?"

Owner decisions:
- **Window:** an app-mode window from an installed Chromium-family browser (Chrome, Chromium, Edge, Brave), falling back to the default browser.
- **Download:** a portable `.tar.gz`, like the Windows portable ZIP.
- **Scope:** all features, in full quality, capacity and functionality: vault, local shell, live packet capture, local network tools, Wi-Fi and the built-in updater.
- **Distributions:** the most popular and common ones (Ubuntu and Debian, including Mint; Fedora; Arch). The program is a self-contained static build, so it has no distribution-specific libraries.

The owner's non-negotiables in AGENTS.md apply unchanged. Windows behaviour, file names, update feed and saved data must not change. Linux is added alongside.

## Where the app stands today

The Go engine already builds for Linux (`CGO_ENABLED=0 GOOS=linux`); CI scans it for vulnerabilities. Started on Linux, it serves the UI locally and opens a browser tab.

These already work on Linux:
- Test a target and Test a website: Ping, Trace route, DNS, Ports, Device check, Details, Monitor, HTTP, TLS and SMTP.
- Network discovery and ARP neighbours.
- SSH, Telnet and serial terminals.
- SFTP/FTP/TFTP transfers.
- The Text Editor.
- PCAP import and inspection.
- Reports and settings.

These are Windows-only today, and each phase below replaces them:

| Area | Windows today | Linux equivalent |
|---|---|---|
| App window | WebView2 host (`internal/desktop`) | App-mode window from an installed Chromium-family browser, with a fallback to `xdg-open` |
| Secret entry and approvals | Native Win32 dialogs (vault, credential confirmation) | Native dialogs: `zenity`, `kdialog` or `yad`, whichever the desktop has. There is no fallback that types secrets into the web page |
| Vault without master password | DPAPI (`.credentials.auto`) | Secret Service keyring over D-Bus: GNOME Keyring, KWallet or KeePassXC |
| Local shell | Command Prompt and PowerShell through ConPTY; elevation through UAC | bash, zsh, fish and sh through a pseudo-terminal; elevation through `pkexec` (polkit), else `sudo` |
| Save As | Windows Save As dialog | Desktop portal / `zenity --file-selection --save` / `kdialog` |
| Local network | Hosts file and IP configuration through an elevated helper | `/etc/hosts` and NetworkManager (`nmcli`) through a `pkexec` helper |
| Wi-Fi | Profiles and keys through `netsh` | NetworkManager profiles; viewing a key needs administrator approval |
| Live capture | Elevated helper | `AF_PACKET` capture in a `pkexec` helper (or a binary with `cap_net_raw`) |
| Updater | Update ZIP, executable swap, restart | Linux update archive, binary swap in the portable folder, restart; same consent and checksum checks |
| Drives | Windows drive letters | Root, home and mounted filesystems from `/proc/self/mounts` |

## Phases (one opt-in beta each, then stable)

**L1 · Foundation**
- Linux amd64 and arm64 builds.
- `Network_Toolkit_<ver>_Linux_x86_64.tar.gz` and `_Linux_aarch64.tar.gz` added to every release and to `SHA256SUMS`.
- App-mode window and single-instance handoff.
- Data stays in the existing per-user folder `~/.config/NetworkToolkit` (movable from App settings, like Windows).
- Optional menu entry: "Add to applications menu" writes a `.desktop` file and icon under `~/.local/share`.
- Platform-aware wording, so Windows-only controls explain what is coming instead of failing.
- Linux CI build and browser smoke test.

**L2 · Vault and local shell**
- Native secret and approval dialogs.
- Master-password vault.
- No-master-password mode in the Secret Service keyring.
- Local shell tabs (detected shells, starting directory, drives).
- Elevated shell tabs, and "Restart this tab as administrator", via `pkexec`.

**L3 · Live packet capture**
- `AF_PACKET` capture helper started through `pkexec` (or capabilities).
- Interface list, filters, ring buffer and limits as on Windows.

**L4 · Local network and Wi-Fi**
- Hosts file review and edit with the existing review and confirmation steps.
- IP, DNS and adapter enable/disable through NetworkManager.
- Wi-Fi profiles with key reveal after approval.

**L5 · Updater**
- Linux update archive per architecture.
- Download, SHA256SUMS check, explicit install consent, binary swap with backup and rollback, then restart.

**L6 · Separate windows and Save As**
- Detached Terminal and Text Editor windows as additional app-mode windows.
- Native Save As (desktop portal, `zenity` or `kdialog`).

**Stable**
- Linux joins the stable release once the owner confirms each phase on a real Linux PC.

## Release and update rules

- Windows assets and names are unchanged. The Windows updater only reads `Network_Toolkit_<ver>_Update.zip` and `SHA256SUMS`, and ignores other assets.
- Linux assets are listed in the same `SHA256SUMS`.
- The Linux updater (L5) reads only its own architecture's update archive and `SHA256SUMS`.

## Testing

- Hosted CI builds and tests every Linux phase: unit tests, browser tests with the Linux build, and fixtures for keyring, pseudo-terminal and helper protocols.
- CI gives no claim of physical Linux or live-device acceptance. The owner confirms each phase on a real Linux PC.

## Status

**L1 (v1.21.0 Beta 1, published 2026-11-06)**
- App-mode window: a dedicated profile; the app stops when the window closes.
- Browser-tab fallback with Quit Network Toolkit.
- Private one-time launch link (`browser_launch_gate.go`), so the session token is never served to other local processes or users.
- Add to applications menu (user `.desktop` entry and SVG icon).
- Linux x86_64 and aarch64 `.tar.gz` release assets; CI rehearses packaging on every run.
- Features that arrive in later phases say so in the app.
