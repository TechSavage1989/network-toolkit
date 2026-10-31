# Network Toolkit 1.17.127 — paced paste and device labels

Published 2026-10-30T23:12:03Z; actual public downloads/source/feed verified
2026-10-30T23:13:27.407082Z. Release429573909; Release workflow41142748790
completed successfully on the exact reviewed source below.

Source/tag: 3085e0729be9f7cbbe372f9959c8985f5255b220;
tree: ca4095f5bad98c1f2b3c64232a5582062a3fa9d1.
PR #1 was explicitly approved, marked ready and fast-forward merged to this exact
tested commit on 2026-10-30. Existing visibility/access remain unchanged.
Release: https://github.com/TechSavage1989/network-toolkit/releases/tag/v1.17.127

## Terminal behavior

Paste review offers literal one-line-at-a-time sending for complete SSH command
lists at a recognized empty prompt. It preserves indentation, blank lines and
template-looking text. The selected session generation and ownership revision
remain fixed. Limits are 64 KiB and 1,024 lines; incomplete final lines and terminal
control characters require raw paste or explicit editing. No Enter is silently
added to unfinished input.

The runner waits for fresh line-boundary output ending in a prompt that remains
stable for 100 ms, with a 30-second timeout. Prompt recognition is a heuristic,
not proof that a device accepted a command. No automatic pager answer is sent.
Pause and Stop are available in Runs, but cannot undo commands already sent.
Pending and uncertain starts cannot silently submit again across review closure.
Raw, single-line and bracketed paste and existing explicit settings remain intact.

Tabs, Saved and Recent show a recognized CLI device name above a smaller endpoint.
Without a recognized name, only the endpoint is shown. Config-mode suffixes are
excluded. A configured hostname is shown honestly when no IP is available.
Stored custom names, connection addresses, credentials and recent-session identity
are preserved. Learned display metadata is bounded; single-session transfers carry
only that session's validated name, not the complete name registry.

## Validation and retained limits

Final local source 63b4563f01e974351b4aa814db2fb69840634c1a has the exact tree
above and passed 1,407 JavaScript tests and 42 Python tests. Review push
41140043643 completed with all nine jobs successful: check, dependency-security,
browser-security, browser, browser-workflows107, browser-editor112,
browser-terminal127, native-terminal127 and windows-terminal. All original gates
were retained; the new focused native job did not replace the full Windows suite.

Real Chromium synthetic tests cover 24 paced-paste groups across main, terminal
and detached windows, including literal bytes, Pause/Stop, uncertain-start
retention and duplicate prevention. Six name/IP viewport/theme cases cover
fallback, persistence, matching Saved/Recent identities and saved custom names.
Inspected desktop layouts show the name above smaller endpoint text. Narrow
390px labels can ellipsize; this is not a claim of complete mobile optimization.

Actual Windows WebView2 focused job 124351825628 passed three detach/return
cycles, visible main/returned name-over-IP geometry, IP-only fallback, preserved
saved name and hidden single-session child-strip metadata. Mouse gestures,
selection/scroll, cancellation and ownership checks passed. This uses synthetic
loopback Telnet, not a live SSH device. Matching Saved/Recent endpoint display
is covered by browser fixtures, not the native saved-row capture whose default
port differs from the live fixture's random port.

Duplicate PR run 41140049305 finished seven jobs successful and two failed:
browser-editor112 failed compact-sidebar keyboard focus; windows-terminal failed
read-only adapter inventory and own-session/direct Explorer SFTP native-copy
checks, followed by missing artifact uploads. These observations are retained,
not silently treated as fixed by the successful same-source push. Historical
comparison-motion, native-transfer, keyboard-resize, focus and UAC limitations
also remain. No live network mutation or visible UAC transition was tested.

Independent exact-source package review verified 1,213 tracked source files,
1,220 safe portable members, CRC and both manifests, Windows AMD64 GUI PE32+,
file version 1.17.127.0, 12 resource leaves and all nine canonical icons.
Reviewed EXE SHA-256:
377a1d064385abd6df83fb6641abfad4016843b5da81d507d1c806c090e0b540
Reviewed portable SHA-256:
d1aa87d1fe860e855a87e0d783ba4da8066de252b9a1aba001d348e357c5228f
Actual public EXE and portable bytes equal the reviewed package. All four public
asset sizes/digests, outer and source manifests, update four-member allowlist,
CRC/safe paths, same EXE and exact update documentation passed. Release notes,
tag, main at verification and latest updater feed match the exact reviewed source.
The prior126 tag and release metadata/assets are unchanged except download counts.
Public update ZIP SHA-256:
2f4434a86c89222aecf69bd63d160376f0f2956e7f63d22cb881f9bd5e682ab3
Public SHA256SUMS SHA-256:
10bba8ce8ca67ef39319748c0e9e214a4b4155f6e51ddcb5128195a13b6b7e6b

Post-merge Checks run41142748784 ended with eight successful jobs and browser
reported cancelled. Every recorded browser step, including the actual five-minute
update timer and artifact upload, completed successfully; the job-level cancellation
is retained and not represented as an all-nine-job pass. Publication proof does not
erase original failed duplicate observations.

Whole-block SSH paste can interleave device echo and prompt output. The original
remote byte stream was not captured, so no font/xterm defect or complete original
root cause is claimed. Paced paste mitigates that workflow rather than rewriting
received terminal text. Prompt recognition remains heuristic, not confirmation
that a device accepted commands. Tests use synthetic data only; no client
configuration, screenshots, credentials or secrets are published.

This closure is source-only: it must not replace published tags or asset bytes.
It grants no blanket future approval and changes no repository access.
