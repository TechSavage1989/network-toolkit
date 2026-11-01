## Beta 13 publication verification

Published v1.17.149 Beta 13 on 2026-11-01T05:06:03Z from
bf76d7506393d4a1902c55656a5490897abf8922. The official Release
workflow 41275438541 passed build, Windows portable/update package
validation and publish; four GitHub-reported assets have SHA-256
digests. Stable latest remains v1.17.136.

The focused SFTP timeout regression passed on a hosted Windows
runner. Candidate Checks 41275225328 had 11 successful job groups
and four browser/native Windows groups still pending at release
verification. Main Checks 41275438525 likewise had 11 passed and
four pending. Prior Editor 390px motion and real-device/native
endurance acceptance remain open, and no speed or memory gain is
claimed from this scoped fix.

# Beta 13 - SFTP response-timeout audit

**Candidate only, not published.** No physical user desktop or live client
network was used.

## Cause and remediation

Standalone SFTP operations can impose socket deadlines. SFTP operations
borrowing a connected SSH terminal cannot set the shared socket deadline
without risking disruption to the terminal. A silent SFTP server could
therefore leave a borrowed browsing request waiting indefinitely even
though the connection's timeout setting was present.

Beta 13 adds a timeout to each SFTP protocol exchange over a borrowed SSH
connection. Timeout closes only its SFTP subsystem and returns
context.DeadlineExceeded. Each following request starts a new timer, so
long transfers are not subject to a single fixed lifetime limit. The
interactive shell's SSH socket deadline is not altered.

The isolated SSH regression hangs a directory reply, checks the timeout
closes the scoped channel, verifies shell continuity and opens another SFTP
request on the same authenticated connection. Hosted Windows CI runs this
test. It is not a real-device or multi-hour soak.

## Eleven-area acceptance matrix

| Area | Beta 13 coverage and remaining work |
| --- | --- |
| Application interface | Existing browser navigation/layout tests; visual polish deferred |
| Network diagnostics | Existing Ping, Trace, DNS, TCP tests retained, live concurrency deferred |
| Network discovery | Existing discovery/retention tests retained, live large scans deferred |
| Graphs and monitoring | Existing graph/history tests retained, physical endurance deferred |
| Packet capture | Existing capture tests retained, live high-volume soak deferred |
| File transfers | Shared SFTP idle-response timeout and scoped cleanup added; other protocols unchanged |
| Text Editor | Original 390px motion/sidebar CI requirement unchanged and unresolved |
| Terminal | Test that SFTP timeout does not close interactive SSH shell or prevent reuse |
| Reports and results | Existing export/retention tests retained, no output changes |
| Network configuration | Adapter/Hosts permissions and actions unchanged |
| Settings and updater | Stable Network_Toolkit.exe, beta opt-in and explicit install approval unchanged |

## Publication gates

Verify exact source against Go/JS/Python/security and Windows/browser CI,
native resource version, executable, portable/update ZIPs, SHA-256 digests,
beta/stable feeds. Keep all known CI failures and physical SFTP/Windows
acceptance limitations. Do not change old release assets or claim
stable certification.
