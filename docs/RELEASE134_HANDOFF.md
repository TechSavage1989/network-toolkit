# Network Toolkit 1.17.134 - published paced-paste history correction

Published 2026-10-31T06:12:51Z as stable release 429758455.
Release: https://github.com/TechSavage1989/network-toolkit/releases/tag/v1.17.134
Release/tag/source commit: 71750e8b48512735716a6f86146dd9722deff30a
Tree: 1ad4ab9fe5da301d4e2a289bba7227ff785d7c6d
Release build/publish run 41175477675: both jobs SUCCESS; latest GitHub
release feed resolves v1.17.134. Previous v1.17.133 release 429742311,
original tag/source and all four asset IDs, sizes and GitHub SHA-256 digests
remain unchanged. No prior release was overwritten.

## Delivered

The SSH Terminal's one-line-at-a-time paste now records its accepted backend
commands and associated output for Last paste, Last command, Copy/Select and
the Macros Last command/Last paste save shortcuts. The entire accepted paste
is available in Last paste, while Last command references the final nonblank
accepted command. Unsent commands after stop, disconnect or failed start
are not promoted. Prompt pacing, device writes, auth, session ownership and
generation, settings, stable Windows EXE name and updater safeguards remain.

## Build, tests, scope

The exact-release build passed 1,508 JavaScript tests (zero failures),
42 Python tests, application Go tests, Windows x64 compilation, regenerated
native icon/version resources, and verified a 1,266-member portable package
with a strictly bounded binary-only update package. Source review PR #9 was
merged as the release source commit above.

GitHub-reported assets and SHA-256:
- Network_Toolkit.exe: 21,084,672 bytes;
  fb42d8818dbf914df6db372895da3d466e1c3c16816b37872b826d4c17e2ce6d
- Network_Toolkit_1.17.134_Portable.zip: 14,341,637 bytes;
  97226e9db0985f34e2ce1030e9245eb6b215cc9f9f6ea2749dc73d579ff9e6a8
- Network_Toolkit_1.17.134_Update.zip: 8,057,622 bytes;
  0f071cb9b8b7e772e52782db4aa3ddaf0298539569c1a95a74409b676342a106
- SHA256SUMS: 292 bytes;
  793dba482fc81bec0a2671089c1393fe9c5dfb12a131829d5d301b83ae0223c2

The release workflow verified the generated packages and GitHub metadata
reports all four public assets uploaded. No separate independent public-byte
download and rehash was performed; do not claim that proof. No real user
device, physical serial device or live SSH timing benchmark was run.

## Retained CI observations

The development push Checks run 41175186481 has a Windows-terminal failure:
TestNativeSFTPDirect121InstalledWebView2 did not receive the expected detached
editor child acknowledgment. Root cause is unverified; the test is outside
the changed paced capture code. Editor/browser comparison and Windows
PowerShell echo failures from v1.17.133 are still historically recorded.
PR Checks run 41175190213 and post-merge main Checks run 41175477724
had several independent browser/Windows jobs still in progress at the time
this release closure was prepared. Their pending results are not passes.
Neither the code change nor this documentation claims to fix these cases.

This documentation-only closure does not modify v1.17.134 tagged source,
binary bytes, release notes or assets, and grants no authority for unrelated
changes.

---

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
