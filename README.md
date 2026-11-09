<p align="center">
  <img src="assets/Network_Toolkit_Icon.png" width="112" alt="Network Toolkit icon">
</p>

<h1 align="center">Network Toolkit</h1>
<p align="center">A portable toolkit for network diagnostics, device connections, file transfers and network administration, for Windows and (in beta) Linux.</p>

<p align="center">
  <a href="https://github.com/TechSavage1989/network-toolkit/releases/latest">Download</a> ·
  <a href="#getting-started">Get started</a> ·
  <a href="#features">Features</a> ·
  <a href="#screenshots">Screenshots</a> ·
  <a href="#updates-and-settings">Updates</a> ·
  <a href="#development">Development</a>
</p>

<p align="center">
  <img src="docs/images/hero.png" alt="Test a target showing a completed Ping with response-time graph, statistics and samples" width="900">
</p>

## Features

- **Test a target:** enter a host once and run a device check, Ping, Trace route, DNS lookup, TCP port checks, a continuous monitor or device details from one page. Drag the tabs into your preferred order.
- **Test a website:** HTTP and TLS certificate checks for one address; SMTP tests for mail servers.
- **Discover network devices** and inspect connection details.
- **Watch targets** with live response-time graphs.
- **Packet capture for everyone:** record while a problem happens, then get a plain answer ("2 problems found") with what each problem means and what to check next. Check a website or device step by step to see where its connection stopped. Experts keep the packet list, filters, technical details and PCAPNG export.
- **Connect** with SSH, Telnet and supported serial terminals, using saved connections, multiple tabs and macros. Color terminal output with universal highlighting (devices, servers and logs) or MobaXterm-style keyword rule sets, each with its own colors, including your own `.msyn` files; pick a color (or none) for the device's name in its prompt, and choose the font size, cursor, color scheme, scrollback, bell and line-ending options. Risky pastes are flagged before they are sent.
- **Password manager:** save usernames with passwords or SSH keys in an encrypted vault, with or without a master password; a strength gauge helps you choose one.
- **File Transfer:** browse, transfer and host files over SFTP, FTP/FTPS and TFTP from one page.
- **Text Editor:** open, edit and compare text files, including FortiOS and other network device configurations with syntax coloring. Convert selected text to or from hex and Base64. Send selected commands to a Terminal session and bring terminal output back into a document. The v1.21.25 beta processes large files in the background without fixed file-size or line-count limits.
- **Local network:** view local network information and review authorized configuration changes.
- **Network utilities:** Wake on LAN, an IPv4 subnet calculator, a transfer-time estimate and a MAC address formatter.
- **Customizable Overview:** choose which dashboard sections to show, and drag them into the order you want; the layout is kept between runs.
- **Settings and updates:** save preferences and use the built-in updater.

## Screenshots

<sub>All screenshots show the real interface with fictional example data (documentation addresses such as 192.0.2.x and example.com). Regenerate them with `node scripts/readme-screenshots.browser.cjs`.</sub>

### Test a target

<table>
  <tr><td width="50%"><b>Trace route with hop hostnames and probe times</b><br><img src="docs/images/trace-route.png" alt="Trace route results listing eight hops with hostnames, round-trip times and one hop without a reply" width="440"></td><td width="50%"><b>DNS lookup across common record types</b><br><img src="docs/images/dns-lookup.png" alt="DNS lookup results with A, AAAA, CAA, MX and NS records and their TTLs" width="440"></td></tr>
  <tr><td width="50%"><b>TCP port checks: open, refused and timed out</b><br><img src="docs/images/tcp-ports.png" alt="TCP ports results showing open, closed and timed-out ports with connect times" width="440"></td><td width="50%"><b>Connection monitor with a ten-minute response-time graph</b><br><img src="docs/images/connection-monitor.png" alt="Connection monitor graph of 600 TCP connection samples with loss and timing statistics" width="440"></td></tr>
</table>

### Test a website and mail server

<table>
  <tr><td width="50%"><b>HTTP check with redirects, TLS and timing (light theme)</b><br><img src="docs/images/website-http.png" alt="HTTP check results with status 200, total time, TLS details and a redirect from HTTP to HTTPS" width="440"></td><td width="50%"><b>TLS certificate validation and chain</b><br><img src="docs/images/website-tls.png" alt="TLS certificate results showing a passed validation, TLS 1.3 and the certificate chain" width="440"></td></tr>
  <tr><td width="50%"><b>SMTP test from connection to test message</b><br><img src="docs/images/smtp-test.png" alt="SMTP test results listing each stage, including STARTTLS and authentication, as passed" width="440"></td><td width="50%"></td></tr>
</table>

### Network discovery and packet capture

<table>
  <tr><td width="50%"><b>Network discovery with hostnames, vendors and open ports</b><br><img src="docs/images/network-discovery.png" alt="Network discovery of a /25 network listing responding devices with MAC addresses, vendors and open TCP ports" width="440"></td><td width="50%"><b>Packet capture explained in plain words</b><br><img src="docs/images/packet-capture.png" alt="Packet capture result saying 2 problems found, a step-by-step check showing that Remote Desktop on 192.0.2.12 refused the connection, and plain explanations of a missing DNS name and the refused connection" width="440"></td></tr>
</table>

### Terminal, file transfer and text editor

<table>
  <tr><td width="50%"><b>SSH and serial sessions in tabs, with saved connections</b><br><img src="docs/images/terminal.png" alt="Terminal with three session tabs showing switch interface and route output, and a sidebar of saved connections" width="440"></td><td width="50%"><b>SFTP client with local and remote files (light theme)</b><br><img src="docs/images/file-transfer.png" alt="File Transfer connected to an SFTP server, with local and remote file lists and transfer activity" width="440"></td></tr>
  <tr><td width="50%"><b>Compare two device configurations with a scrollable, line-by-line change map</b><br><img src="docs/images/text-editor-compare.png" alt="Text Editor comparing two switch configurations side by side with added lines highlighted and a change map" width="440"></td><td width="50%"></td></tr>
</table>

### Local network, utilities and settings

<table>
  <tr><td width="50%"><b>IP configuration with public IP and IPv4 settings</b><br><img src="docs/images/local-network.png" alt="Local network IP configuration showing the current adapter settings and an IPv4 settings editor" width="440"></td><td width="50%"><b>Review hosts file changes before saving</b><br><img src="docs/images/hosts-file-review.png" alt="Review hosts file changes dialog showing one changed line before saving" width="440"></td></tr>
  <tr><td width="50%"><b>IPv4 subnet calculator (light theme)</b><br><img src="docs/images/network-utilities.png" alt="IPv4 subnet calculator showing network, broadcast, usable range and masks for a /26" width="440"></td><td width="50%"><b>Update checks in App settings</b><br><img src="docs/images/settings-updates.png" alt="App settings Updates section with automatic and beta update options and the latest stable release" width="440"></td></tr>
</table>

## Download

[Download the latest stable release](https://github.com/TechSavage1989/network-toolkit/releases/latest). It provides the portable package **`Network_Toolkit_<version>_Portable.zip`**, the executable **`Network_Toolkit.exe`** and **`SHA256SUMS`** for checking the files.

**Linux (beta):** the Linux edition is an opt-in beta while it gains every Windows feature ([plan](docs/LINUX_PLAN.md)). Download **`Network_Toolkit_<version>_Linux_x86_64.tar.gz`** (or **`_Linux_aarch64.tar.gz`** for 64-bit Arm) from the [releases page](https://github.com/TechSavage1989/network-toolkit/releases); the same **`SHA256SUMS`** lists it.

## Requirements

- Windows x64 for the packaged portable application.
- Linux beta: a 64-bit x86 or Arm Linux desktop (Ubuntu, Debian, Mint, Fedora, Arch and similar). Google Chrome, Chromium, Microsoft Edge, Brave or Vivaldi gives the app its own window; otherwise it opens in your default browser.
- Network access and permissions appropriate to the tools being used.
- Windows administrative approval for protected system changes, when requested.
- Compatible remote services, such as an SSH server with SFTP enabled, for connection-dependent features.

## Getting started

1. Download the stable portable package and extract it to a location you control.
2. Start **Network_Toolkit.exe**.
3. Select a tool from the navigation menu.
4. Enter a target or create a terminal connection, then run only the operations you are authorized to perform.
5. Review any changes to network configuration before confirming them.

On Linux (beta), extract the `.tar.gz`, run **`./network-toolkit`** from its folder, and use **App settings → Add to applications menu** to start it from your desktop's menu next time. Some Windows features arrive on Linux in later betas; the [Linux plan](docs/LINUX_PLAN.md) lists what is ready.

## How to use

The topic guides explain each area of the app:

- [Getting started with diagnostics](docs/guide/diagnostics.md)
- [Terminal](docs/guide/terminal.md)
- [Text Editor](docs/guide/editor.md)
- [File transfers](docs/guide/transfers.md)
- [Network configuration](docs/guide/network-config.md)
- [Settings and updates](docs/guide/settings-and-updates.md)

## Responsible use

Use scanning, configuration, capture and connection features only on networks and systems you own or are authorized to administer. Protected system changes require review and Windows approval. Keep credentials private and check the destination before transferring or overwriting files.

## Updates and settings

Use **Check for updates** to check manually. Beta updates are available only after opting in to the beta channel. Downloading and installing an update requires explicit approval; packages are verified before installation. Your saved profiles, settings and credential vault must be preserved.

Open **App settings** to search General, Appearance, Terminal, Editor, Graphs, Transfers, Network, Updates and Confirmations. Preferences, saved targets and connection profiles are stored in the app data folder; diagnostic results stay in the current session until you export them. See the [change history](CHANGELOG.md) for what each release changed.

## Development

Read [AGENTS.md](AGENTS.md) and the [development handoff](docs/DEVELOPMENT_HANDOFF.md) before changing anything; the shared organization standard is in [docs/REPOSITORY_STANDARDS.md](docs/REPOSITORY_STANDARDS.md). Use the scripts and vendored dependencies already in the repository. See [development instructions](docs/DEVELOPMENT.md) for the supported build and test commands and the [UX development plan](docs/UX_POLISH_PLAN.md) for planned work. The distributed Windows executable retains the filename **Network_Toolkit.exe**.
