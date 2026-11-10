# Windows default terminal: hosted experiment findings

Status: functional test passed on the owner's Windows 11 PC (2026-11-10); not yet releasable (needs a trusted MSIX signer and product integration). The release criterion was a hosted (or owner-run) test showing both original shells (Command Prompt and Windows PowerShell) opening in Network Toolkit, with interactive input working, while Network Toolkit is the selected default. A public release also needs a trusted MSIX signer; the repository has none, and fixture certificates stay on disposable runners.

## How the prototype works

- `scripts/windows-terminal/DefaultTerminalExperiment.appxmanifest` declares the `com.microsoft.windows.console.host` and `com.microsoft.windows.terminal.host` app extensions. The console host is Microsoft Windows Terminal Stable's OpenConsole (`{2EACA947-7F5F-4CFA-BA87-8F7FBEEFBE69}`). The terminal is the Network Toolkit broker (`{B60E0829-5BF3-4A4A-ACB9-C661D85B7282}`), a packaged COM server.
- `ConsoleHandoff.cpp` (the broker) implements `ITerminalHandoff3`. It accepts a handoff only from Microsoft's OpenConsole in the same user and session. It then starts `Network_Toolkit.exe --terminal-handoff` with the console pipes.
- `.github/workflows/windows-default-terminal-experiment.yml` builds the broker and the x64 app on `windows-11-arm`, signs a throwaway MSIX, selects the pair in `HKCU\Console\%%Startup`, launches `cmd` and PowerShell with a new console, and restores everything afterwards. The job log prints the evidence, the console and broker processes started during the test, and Windows warnings and errors. Artifact downloads are not reachable from every reviewer.

## What the hosted runs showed (2026-11-10)

| Run | Shells ran as | Result |
| --- | --- | --- |
| 42746029077 | Separate standard account (Secondary Logon) in runneradmin's session | conhost handed off. But Windows started OpenConsole and the broker as **runneradmin**, the session owner, so OpenConsole followed runneradmin's own default (Windows Terminal). The account's direct broker activation got `0x80070005`. |
| 42746869734 | runneradmin, elevated (`S-1-16-12288`) | No handoff. The direct broker activation as runneradmin worked at every stage (`entry` through `class-revoke`). |
| 42747555091 | Interactive scheduled task at the limited run level | Still `S-1-16-12288`: the account has no split token. |
| 42748061959 | runneradmin, shells at medium integrity (Safer normal-user token) | No handoff, not even to Windows Terminal (the control). |
| 42748832687 | Same, recording account facts | runneradmin is the **built-in Administrator** (`…-500`), `EnableLUA=1`, `FilterAdministratorToken` unset, elevation type 1 (no split token), `HKCU\Console\ForceV2=1`, delegation read back correctly. |

Conclusions:

1. **The broker is not the blocker.** Windows activates packaged COM servers (OpenConsole and the broker) as the user who owns the interactive session, and activating the broker as that user works.
2. **This hosted desktop can't test a default terminal.** Windows does not apply a default terminal choice for the built-in Administrator without Admin Approval Mode. Even selecting Microsoft's own Windows Terminal has no effect there.
3. **A second account doesn't help.** A separate account inside that desktop does get its console handed off, but its OpenConsole and broker run as the desktop owner. The broker correctly refuses a different user.

## Owner test on a physical Windows 11 PC (2026-11-10)

The owner installed the side-loaded x64 test package from the "Windows default terminal test package" workflow (run 42750697252) and selected "Network Toolkit Default Terminal Experiment" as the default terminal. They reported that **both Command Prompt and Windows PowerShell opened in Network Toolkit, typing worked in both, and nothing looked wrong** (title, resizing, closing). They then ran the remove script, which restored their previous default terminal and removed the package and certificate. This meets the functional release criterion: the broker, Microsoft's OpenConsole handoff and the Network Toolkit renderer work together for a real user.

Still needed before release:
- A trusted MSIX signer, for a public package that installs without trusting a test certificate.
- Product integration: a packaged install path, a settings entry, and documentation of what changes when Network Toolkit is the default.

## What a valid test needs

A standard (or UAC-filtered) user who owns their own interactive session, with Network Toolkit selected for that user. Options:

- An interactive session for a standard account on a hosted runner, for example a loopback Remote Desktop session. A Windows Server runner allows a second session without disconnecting the runner. It is not yet known whether the default terminal feature is active on Server.
- A self-hosted Windows 11 runner signed in as a standard user.
- An owner-run check on a Windows 11 PC, with a side-loaded test package and its certificate trusted only for that check.

## Changes made for the hosted fixture

- **Hosted account only:** the worker runs as the hosted runner's session owner, starts the shells with a medium-integrity token, and selects Microsoft's Windows Terminal first as a control.
- **Client integrity:** the broker accepts a client at the same or lower integrity than itself, never higher. It still requires the same user and session, and an exact match for OpenConsole.
- **Diagnostics:** they are compiled only into the experiment package and written only for the hosted runner account.
