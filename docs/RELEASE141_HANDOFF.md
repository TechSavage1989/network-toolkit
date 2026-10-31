# Network Toolkit v1.17.141 Beta 5 - published interface and workflow polish

Published: 2026-10-31T19:06:59Z, prerelease=true, draft=false, GitHub release 430129340.
Release: https://github.com/TechSavage1989/network-toolkit/releases/tag/v1.17.141
Tagged/reviewed source, merged PR #16: e2b8facd51834cfa93e2f587e01cd9d1c2696a89
Release workflow 41232480174: build and publish both SUCCESS.

## Delivered and preserved

- Editor Files/Connections navigation retains the mounted form on repeated selection. Navigation revisions still protect pending access operations, and detached Editor approval refresh behavior is retained.
- Connection Settings focuses the currently visible Server or connection source field without replacing the active form or overriding a later user focus.
- Editor comparison-map interactions preserve continuous fractional aligned-scroll positions rather than quantizing every drag movement to a line start. No changes to document content, undo/redo or connection ownership.
- TFTP/SFTP file explorer divider adds keyboard resizing (Arrow, Shift, Home, End), double-click reset, frame-coalesced live pointer updates, final-position commits and cancellation/blur rollback with accessible position labels.
- New regression coverage checks all first-party loaded browser asset syntax. This detected and prevented a temporary malformed selector string from being released.
- Saved settings, credentials, SSH/Telnet/serial/FTP protocols, network privileges, packet capture, user confirmation and stable Network_Toolkit.exe filename are unchanged.

## Exact release validation

The exact-source release workflow passed application Go tests, 44 Python tests, 1,541 JavaScript tests (zero failures), generated/validated 12 Windows resources, Windows x64 GUI compilation, a 1,289-member portable source package and strict binary-only update ZIP checks.

GitHub-reported official asset sizes and SHA-256 digests:
- Network_Toolkit.exe: 21,102,592 bytes; 3bd808cda411d34dd60df546ad444cc3ccd2161e5a13765885a769fcdb5ba561
- Network_Toolkit_1.17.141_Portable.zip: 14,418,425 bytes; 74191f0c2d0b881e414c24992bfcd3aa30cb52362571841c62df1b9963799e33
- Network_Toolkit_1.17.141_Update.zip: 8,064,656 bytes; 5a61a642abf055a9139d9f44e53c3e07ec98aa9acec06a0bade89a7325d9ecc6
- SHA256SUMS: 292 bytes; 3b006fa5134076b2e936b29fd7299014a9ed6798ecde9d216dd19f70527b6aa7

GitHub tag, release target and merged main source agree; the beta feed lists 141 first while the stable-only latest feed remains 136. Published 140 tag/assets were unchanged. Verification used GitHub-reported asset metadata; no independent public-byte download/rehash was performed.

## Known limitations and separate checks

The latest targeted Editor browser job at source f9038de4 passed the compact/narrow sidebar controls and retained Files/Connections navigation. Other broader Editor continuous comparison-motion tests and Windows integration jobs remained in progress at the time of publication. Do not count pending tests as passes, or assert that the historical comparison offset problem is universally resolved. Native SFTP cancellation CI failures in prior runs remain outside this release scope and have not been repaired here.

Synthetic/hosted browser and Windows CI are not physical customer network-device measurements, and no end-user device load benchmark was performed. This is an opt-in beta, not a stable promotion.

The candidate record below is retained for traceability. Its original PUBLICATION PENDING label is historical; v1.17.141 has been published. This documentation-only follow-up modifies no tagged code, release note body, binary, asset, or checksum.

---

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
