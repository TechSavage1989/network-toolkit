# v1.17.149 Beta 13 - release candidate handoff

**NOT PUBLISHED.** Based on v1.17.148 (published source
72d11fd8ffd5ccf719a028e7786dbc7b26868d75), stable v1.17.136.
Branch: beta/1.17.149-shared-sftp-timeout-reliability.

## Implemented changes

- Per-response idle timeout applies to SFTP exchanges on a borrowed
  authenticated terminal SSH connection. Timeout closes only the SFTP
  subsystem and returns a deadline error.
- Per-response timer resets for subsequent requests instead of imposing
  an overall transfer duration limit. No shared SSH socket deadline change.
- Isolated synthetic SSH test covers stalled directory reply, timeout,
  scoped channel cleanup, shell continuity and a new SFTP request.
- Hosted Windows CI runs this same SFTP regression.
- Versions, embedded Windows resource, portable package and root release
  notes prepared for 1.17.149.

## Preserved behavior and open gaps

Credentials, saved options, permissions, terminal ownership, SSH trust,
transfer approvals, report results, fixed Network_Toolkit.exe, beta opt-in
and explicit update installation consent remain unchanged. Original
eleven-area roadmap is in docs/DEVELOPMENT_ROADMAP.md.

The Editor 390px common-row comparison motion still exceeds its original
2px requirement; not a fixed regression or waived test. Real SFTP device
interruption, native cancellation and physical Windows multi-hour soak
remain open. Synthetic Windows tests are not real-device proof.

## Release verification required

Record exact GitHub branch/merge/tag SHA, CI job successes and failures,
Windows build, four official assets, checksums and beta/stable feeds. Never
relabel pending checks as passed or overwrite previous releases.
