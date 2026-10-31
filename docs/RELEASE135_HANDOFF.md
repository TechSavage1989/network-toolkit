# Network Toolkit 1.17.135 - published paced output formatting correction

Published 2026-10-31T06:37:53Z, stable GitHub release ID 429767777.
Release: https://github.com/TechSavage1989/network-toolkit/releases/tag/v1.17.135
Source/tag/main at publication: 2cf90ac3216ee5f3e3ce59d9b0959bb4109faba6
Tree: 9dc9b763e19b3b2d847912f14ee7a6bd7cf128fb
Release workflow 41177195918: build and publish both SUCCESS.
The latest GitHub release/updater feed resolves 1.17.135 with all four
official assets present. Previous 1.17.134 release ID 429758455,
tag/source and its four original asset IDs, digests and sizes were retained.

## Delivered behavior and scope

For one-line-at-a-time SSH pastes, Last paste and Last command now display
the submitted command itself once when Include commands is on, without any
"Command N:" heading and without duplicated initial device echoes.
When Include commands is off, both submitted lines and recognized initial
echoes are excluded from copied/selected output. The command's actual
response is kept, including internally blank lines and any later output
that happens to repeat a submitted command. Both Copy and Select use
the same output formatter. The full accepted 458-step synthetic list was
validated; no command order or step was silently discarded.

No change was made to backend capture, SSH connection, command execution,
prompt pacing, terminal ownership, session generation, credentials,
profile/settings storage, or the native update executable name.

## Release build and verification

The exact-source release workflow passed 1,512 JavaScript tests, zero
failures, 42 Python tests, application Go tests and Windows x64 compilation.
It regenerated/verified Windows icon and version metadata and packaged a
1,268-member source-inclusive portable ZIP plus the strict binary-only
update ZIP. The exact source was merged through review PR #10.

GitHub-reported public asset sizes and SHA-256:
- Network_Toolkit.exe, 21,085,696 bytes:
  96b0e316f61a5c19a3eca335b4cb661db49a721df4c3e592c3adf1f5c5c16325
- Network_Toolkit_1.17.135_Portable.zip, 14,349,966 bytes:
  f585f255a326be30735969b07750273d0adb94542aaab88f19be9f54cb82e3d9
- Network_Toolkit_1.17.135_Update.zip, 8,058,280 bytes:
  5849d27c1bd5b5e9f73d942d0258ca1bd4fbf3b88f837e3bc20e0b97bf7237de
- SHA256SUMS, 292 bytes:
  8430254055b44fd47a13ce41670550b1e4208c1702e69e8abfedefde5b811c08

Publication verified the exact source/tag/latest release and GitHub asset
metadata. No independent byte-for-byte public asset download and external
rehash was completed in this closure. The automated CLI sessions were
synthetic; physical device timing and arbitrary prompts were not tested.

## CI observations to retain

The release build/publish passed. Separate native/browser Checks workflows
for the versioned review and the post-merge main branch were still
running at this closure, although core Go/JS/Windows build and focused
terminal browser, security, and dependency checks had passed.
Do not count pending checks as successful. Earlier v1.17.134 Windows
detached SFTP acknowledgement and historical Editor/browser and
PowerShell echo failures remain documented and are not claimed fixed.
Do not infer that this formatting change repairs unrelated CI failures.

This is a documentation-only follow-up. It does not alter v1.17.135
published source/tag/assets or grant permission to revise older releases.

---

# Network Toolkit 1.17.135 - paced output command formatting (candidate)

Based on the published v1.17.134 release and its documentation-only main
handoff, this version updates only the formatting/filtering of the
completed paced multiline SSH terminal capture.

The user supplied a screenshot showing two lines for a submitted command,
one as "Command 439: ...", followed by a separate bare device echo.
The requested behavior is: Include commands enabled shows exactly the
submitted command once and the associated output, without numbered headings.
With Include commands disabled, exclude both commands and recognized echoes
but preserve every command's output and the full paste range.

Implementation only changes pacedOutput() in web/assets/terminal-extras.js.
It uses the exact accepted command text from the existing capture and
skips a matching initial prompt-prefixed or bare echo at most once per
step. It preserves later matching lines (possible legitimate output)
and retains internal output blank lines. Blank commands never get synthetic
"[Enter]" labels. Last paste and Last command both use pacedOutput(),
as do Copy/Select and their common output path. Other terminal capture
paths are unchanged.

Regression tests cover command numbers around 439, bare and prompt-prefixed
echoes, both include modes, 458 synthetic steps, copied output, legitimate
command-like content after the initial echo, and terminal history.

Review PR #10. Run exact source Go/JS/Python, the Windows x64 GUI build,
native/browser/security CI and binary packaging before publication. Prior
v1.17.134 CI observations and earlier Windows/Editor failures are retained.
Do not claim unrelated issues have been fixed.

Publication pending: verify release tag/source, latest updater feed, four
assets and their SHA-256, and unchanged v1.17.134 release/tag/assets.
No customer device was used for testing.
