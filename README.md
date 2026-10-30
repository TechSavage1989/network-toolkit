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

## Current candidate — 1.17.127

Safer paced multiline paste adds an explicit one-line-at-a-time option in paste
review, waiting for a fresh prompt before continuing. Raw paste remains available.
Tabs, Saved and Recent show a learned CLI name above a smaller endpoint, or just
the endpoint when no name is recognized. Existing connection identity is retained.
Validation and publication are pending. See [127 handoff](docs/RELEASE127_HANDOFF.md)
for exact evidence and limits; this is not a claim that every rendering issue is fixed.

## Previous 1.17.121 release record (retained)

Published 2026-10-30T05:45:41Z; actual public verification 05:48:05.062831Z.
Release 428692620: https://github.com/TechSavage1989/network-toolkit/releases/tag/v1.17.121
Compact Files/Connect controls reclaim file-list height. Local and already-authorized
SFTP files/folders can be dragged directly to Windows Explorer; file drops inside
the editor preserve dirty documents. Copy-only transfer, cancellation and existing
approval/size bounds remain. Actual Explorer folders were tested; separate Desktop
Shell/cross-integrity/arbitrary-app delivery and visible UAC transitions were not.
FTP/FTPS native export is not included.

**Known reliability issue:** the same-source duplicate Windows run failed its
first detached-child SFTP file drag after all five main-window cases passed.
No data was requested/materialized and no destination was written. The cause is
unknown; matching geometry/approved metadata does not establish a fixture-only
or premature-release explanation. Original ten-case success is retained, not a
guarantee of reliable detached delivery. No rerun, rebuild or repair is claimed.

The original review passed all 22 new rendered/native cases and core/security/
dependency/Windows checks; the later duplicate failure above remains unresolved.
Review 41009283786: six SUCCESS/one FAILURE (comparison motion), last job completed 05:54:57Z.
Duplicate 41010025845: five SUCCESS/one Windows FAILURE/one browser CANCELLED, last job completed 06:04:03Z.
The duplicate real-five-minute case did not log PASS before cancellation;
its timer stage is not a pass, and cancellation cause/actor are unestablished.
All four actual public downloads match the reviewed bytes; 1,170 exact source
files/1,177 portable members, manifests, PE 121/nine canonical icons/12 resources,
update four-file allowlist and latest feed/tag/main/notes are verified. Prior 120
release/tag/body/original four assets remain unchanged.
See [release 121 handoff](docs/RELEASE121_HANDOFF.md) for exact evidence, hashes,
known comparison/Hosts/Alt-transfer caveats and retained earlier failed checks.

## Features

The text editor supports independent left/right tab groups, comparison, tab context
menus and a resizable Windows/SFTP file sidebar. Use **Compare to** from a tab menu,
drag either split divider, and use the sidebar’s **Connect** tab for active or saved
connections and **Files** for Windows/SFTP browsing. Drag a supported file onto
the editor to open it without replacing dirty documents. See
[file drops](docs/EDITOR_FILE_DROP121.md),
[saved connections](docs/EDITOR_SAVED_CONNECTIONS113.md) and
[detached local access](docs/EDITOR_LOCAL119.md) for behavior and permission limits.

Compact sidebar controls and optional editor opening panels leave more room for
the active tool and document. See [layout guide](docs/APP_CHROME109.md).

- **Investigate connectivity:** inspect local adapters, run Ping, Trace route, DNS and TCP-port checks, discover IPv4 hosts, and monitor connections.
- **Work with devices:** open SSH, Telnet and serial terminals; organize saved sessions and macros; detach terminals into their own windows.
- **Transfer files:** use SFTP, FTP/FTPS and TFTP client/server tools, with a built-in text editor.
- **Review and export results:** inspect individual records, copy useful values and save diagnostic reports.
- **Explore history:** keep graph endpoints and timestamps aligned, and shape older
  history with the compact [compression curve controls](docs/GRAPH_CURVE113.md).
- **Keep routine work consistent:** use saved-session indicators, named filters, installed terminal fonts, shared color palettes and reviewed file drops. See [workflow guide](docs/WORKFLOWS107.md) for behavior and protocol limits.
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
| Loaded editor documents and close confirmation | [Editor activity and close policy](docs/EDITOR_ACTIVITY_CLOSE113.md) |
| Concise result details and retained exports | [Result readers](docs/RESULT_READERS113.md) |

## Updates and settings

Open **App settings → Network Toolkit updates** to check for updates. Choose **Download update**, review the release notes and save your work, then confirm **Install and restart**. Downloading alone does not install anything.

Automatic checks run at startup and periodically while the app is visible and online. You can disable them in App settings; **Later** hides a reminder for 24 hours. Active jobs, terminal sessions, transfers and configuration changes block installation. Close other app windows first.

Editor **More → Ask to save unsaved changes before closing** is enabled by default.
Disabling it requires a warning confirmation and allows unsaved document content
to be discarded; active saves, failed-save warnings and ownership transfers remain
protected. Detached windows follow the main window's setting. Moving or merging
documents always preserves their contents.

- Updates use this repository's stable releases and verify the binary-only **`Update.zip`** and its checksums. **Do not import the source-containing `Portable.zip` as an update.**
- Settings and saved profiles are retained. App settings lets you choose the data folder; `--data-dir` is an explicit per-run override. Keep the same data folder and launch options when replacing the executable manually.
- A previous executable is retained for recovery. See [UPDATING.txt](docs/UPDATING.txt) before restoring it or handling an older update-feed migration.
- Builds are not Authenticode signed. Windows may show an unknown-publisher warning. Use the official release assets; SHA-256 checks detect corruption, not a compromised publisher.

Published and verified: [1.17.112 editor views and file sidebar](docs/RELEASE112_HANDOFF.md). All seven hosted review/main checks, automatic Release and four public assets passed verification.

## Development

<details>
<summary>Build, checks and release safeguards</summary>

Read [AGENTS.md](AGENTS.md) and [DEVELOPMENT_HANDOFF.md](docs/DEVELOPMENT_HANDOFF.md) before editing or building. Start from current `main`; the handoff records the released source, required checks and known validation limits.

Application source and Go tests live in [`cmd/network-toolkit/`](cmd/network-toolkit); supporting packages are in `internal/`, interface assets in `web/`, and guides in [`docs/`](docs/DEVELOPMENT.md).

The application uses Go with an embedded web interface. Use the toolchain specified in [go.mod](go.mod). Windows build entry points are [BUILD_WINDOWS.cmd](BUILD_WINDOWS.cmd) and [build.sh](build.sh); the [Checks workflow](.github/workflows/checks.yml) is the reference for Go, JavaScript, release-guard and browser validation.

**Publication is a separate action:** main pushes can publish new application versions. Do not change versions, move released tags, replace existing assets or change repository access as part of documentation work. This repository is the canonical source and update feed.

See [VALIDATION.txt](docs/VALIDATION.txt) for recorded results and [THIRD_PARTY_NOTICES.txt](docs/THIRD_PARTY_NOTICES.txt) for dependency notices. Cross-compilation, browser fixtures and update staging are not proof of native Windows/WebView2 interaction or an actual install/restart.

</details>

Current published version: [1.17.108 — activity indicators, inline bookmarks, graphs and editor workflows](https://github.com/TechSavage1989/network-toolkit/releases/tag/v1.17.108). See the [verified release handoff](docs/RELEASE108_HANDOFF.md) for evidence and compatibility limits.

### Editor documents and windows

See [editor tabs, comparison and change map](docs/EDITOR_WORKFLOWS108.md),
[acknowledged editor window moves and limits](docs/EDITOR_WINDOWS.md), and
[offline network configuration syntax support](docs/NETWORK_CONFIG_EDITOR.md).

Published and verified: [1.17.109 vertical workspace improvements](docs/RELEASE109_HANDOFF.md). All six exact-source gates passed; public assets, source, version/icons and updater feed are verified.

Published and verified: [1.17.111 compact controls and comparison maps](docs/RELEASE111_HANDOFF.md). Hosted checks, isolated visual/native tests and all four public downloads passed verification.

Previous published and verified: [1.17.110 graph and editor corrections](docs/RELEASE110_HANDOFF.md). All six exact-source gates, temporal/native evidence, public assets and updater feed passed verification.
