# Network Toolkit 1.17.134 - paced-paste output capture (candidate)

Based on latest main after published v1.17.133. The user requested that
Last command and Last paste recognize all commands and output submitted
through the SSH Terminal's one-line-at-a-time multiline paste mode.

The backend records each accepted line with the transport's output cursor
and exposes its output using existing authorized run/session scope.
Frontend history links a pre-send paste marker to its run, then adds the
full paste output and a separate final nonblank accepted command record
when the run finishes. The Macros Last command / Last paste shortcuts use
accepted lines rather than treating the original pasted text as executed.
Blank lines stay part of the literal paced run. Stopped, disconnected,
rejected and interrupted runs cannot report unsent lines as completed.

The scoped fix retains prompt pacing, per-line send behavior, native
terminal connection transport, transfer limits, credentials, settings,
Windows permissions, executable identity and updater safeguards.

Review PR #9. Validate exact Go and JavaScript regression tests, security,
native Windows build, browser behavior and release checks. Prior v1.17.133
reported unresolved Windows PowerShell echo and Editor browser layout/
comparison failures. Do not silently waive or claim those fixed.

PUBLICATION PENDING. Verify version/tag/source identity, all four public
release assets, checksums, latest release/updater feed, and immutability
of v1.17.133. Record actual workflow results and limitations after release.
