<p align="center">
  <img src="assets/Network_Toolkit_Icon.svg" width="112" alt="Network Toolkit icon">
</p>

<h1 align="center">Network Toolkit</h1>
<p align="center">Network diagnostics, device terminals and file transfers in one portable Windows app.</p>

<p align="center">
  <a href="https://github.com/TechSavage1989/network-toolkit/releases/latest">Download</a> ·
  <a href="#getting-started">Get started</a> ·
  <a href="#features">Features</a> ·
  <a href="#updates-and-settings">Updates</a> ·
  <a href="docs/releases">Release history</a> ·
  <a href="#development">Development</a>
</p>

## Features

- **Investigate connectivity:** inspect local adapters, run Ping, Trace route, DNS and TCP-port checks, discover IPv4 hosts, and monitor connections.
- **Work with devices:** open SSH, Telnet and serial terminals; organize saved sessions and macros; detach terminals into their own windows.
- **Transfer files:** use SFTP, FTP/FTPS and TFTP client/server tools, with a built-in text editor.
- **Review and export results:** inspect individual records, copy useful values and save diagnostic reports.
- **Manage local network settings:** use IP configuration and the Hosts editor with explicit review and Windows approval for protected changes.

## Download

Download **`Network_Toolkit.exe`** from [the latest release](https://github.com/TechSavage1989/network-toolkit/releases/latest). The versioned **`Portable.zip`** includes the complete source package; **`Update.zip`** is for the in-app updater, not a first-run source package.

## Requirements

| Requirement | Details |
| --- | --- |
| Operating system | Windows 10 or 11, 64-bit x64 |
| Runtime | Microsoft Edge WebView2 Runtime; the Edge browser alone is not the same component |
| Installation | Portable executable; no separate Go, Python, Node.js, .NET, Nmap or Npcap installation |
| Network access | Needed for the selected remote target, online lookups and update checks; local tools can work offline |
| Permissions | Normal use runs as your Windows account. Protected configuration changes and native packet recording can require administrator approval. |

> **Use responsibly:** scan or connect only to systems you are authorized to manage. Telnet, plain FTP and TFTP do not provide encrypted transport. Review Hosts/IP changes before applying them.

## Getting started

1. Open [the latest release](https://github.com/TechSavage1989/network-toolkit/releases/latest) and download **`Network_Toolkit.exe`**. Choose the **`Portable.zip`** asset if you also want the complete source package; extract it before use.
2. Keep the executable in a local folder you can write to. Make sure **Microsoft Edge WebView2 Runtime** is installed, then launch the app.
3. Start with **Overview** to inspect adapters, or choose a diagnostic tool and enter a target you are authorized to test.
4. Review the results and export anything you want to keep. Diagnostic output is session-based; save editor changes and reports before closing or updating.

## How to use

| Topic | Reference |
| --- | --- |
| Tools, storage, reports and troubleshooting | [Detailed user guide](docs/README.txt) |
| Update installation, offline import and recovery | [Update guide](docs/UPDATING.txt) |
| Packet recording and analysis | [Packet troubleshooting](docs/PACKET_TROUBLESHOOTING.txt) |
| Reusable terminal commands | [Terminal macros](docs/TERMINAL_MACROS.txt) |
| Terminal appearance and matching rules | [Terminal highlighting](docs/TERMINAL_HIGHLIGHTING.txt) |

## Updates and settings

Open **App settings → Network Toolkit updates** to check for updates. Choose **Download update**, review the release notes and save your work, then confirm **Install and restart**. Downloading alone does not install anything.

Automatic checks run at startup and periodically while the app is visible and online. You can disable them in App settings; **Later** hides a reminder for 24 hours. Active jobs, terminal sessions, transfers and configuration changes block installation. Close other app windows first.

- Updates use this repository's stable releases and verify the binary-only **`Update.zip`** and its checksums. **Do not import the source-containing `Portable.zip` as an update.**
- Settings and saved profiles are retained. App settings lets you choose the data folder; `--data-dir` is an explicit per-run override. Keep the same data folder and launch options when replacing the executable manually.
- A previous executable is retained for recovery. See [UPDATING.txt](docs/UPDATING.txt) before restoring it or handling an older update-feed migration.
- Builds are not Authenticode signed. Windows may show an unknown-publisher warning. Use the official release assets; SHA-256 checks detect corruption, not a compromised publisher.

## Development

<details>
<summary>Build, checks and release safeguards</summary>

Read [AGENTS.md](AGENTS.md) and [DEVELOPMENT_HANDOFF.md](docs/DEVELOPMENT_HANDOFF.md) before editing or building. Start from current `main`; the handoff records the released source, required checks and known validation limits.

Application source and Go tests live in [`cmd/network-toolkit/`](cmd/network-toolkit); supporting packages are in `internal/`, interface assets in `web/`, and guides in [`docs/`](docs/DEVELOPMENT.md).

The application uses Go with an embedded web interface. Use the toolchain specified in [go.mod](go.mod). Windows build entry points are [BUILD_WINDOWS.cmd](BUILD_WINDOWS.cmd) and [build.sh](build.sh); the [Checks workflow](.github/workflows/checks.yml) is the reference for Go, JavaScript, release-guard and browser validation.

**Publication is a separate action:** main pushes can publish new application versions. Do not change versions, move released tags, replace existing assets or change repository access as part of documentation work. This repository is the canonical source and update feed.

See [VALIDATION.txt](docs/VALIDATION.txt) for recorded results and [THIRD_PARTY_NOTICES.txt](docs/THIRD_PARTY_NOTICES.txt) for dependency notices. Cross-compilation, browser fixtures and update staging are not proof of native Windows/WebView2 interaction or an actual install/restart.

</details>

Current version: [1.17.102 stable recent Condensed spacing](RELEASE_NOTES_1.17.102.txt).
