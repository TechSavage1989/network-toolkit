# Network Toolkit v1.17.141 Beta 5 - Editor and file explorer usability candidate

Based on published v1.17.140 Beta 4. Development PR #16:
https://github.com/TechSavage1989/network-toolkit/pull/16

## Scope

- Preserve the already-open Editor Files/Connections sidebar rather than
  recreating controls on repeated selection, and keep focus on the live
  connection-settings field without stealing focus from later input.
- Preserve fractional comparison-map drag positions instead of rounding
  every movement to an aligned source-line boundary. No document data,
  undo, diff or remote file semantics are changed.
- Add keyboard Arrow/Home/End control, Shift increments and double-click
  reset to the TFTP/SFTP file explorer divider. Pointer movement uses a
  bounded animation-frame cadence, live geometry, cancellation rollback
  and one final persistent update.
- Add focused Node regression tests. Preserve the full existing Editor
  browser, diagnostics, file transfer and terminal test suites.

## Validation and retained risks

- Run the full JavaScript/Go/Python suite and existing Windows/browser
  integration checks. In particular, verify narrow Editor Connections
  controls at 390px, continuous wrapped comparison-map motion, local and
  SFTP file explorer behavior and accessible divider interaction.
- No new saved credentials, network permissions, transfer destinations,
  packet capture behavior or automatic update installations.
- Existing Editor comparison-motion and narrow sidebar test failures are
  not considered fixed without passing the actual relevant browser checks.
  Historical Windows native SFTP cancellation CI concerns remain separate.
- Publish as opt-in prerelease only after the exact-source Windows version
  resources, portable ZIP, binary-only updater, release tag, SHA256 assets
  and stable channel are verified. Do not overwrite prior releases.
- Physical Windows and network-device tests are not provided by synthetic CI.

PUBLICATION PENDING.
