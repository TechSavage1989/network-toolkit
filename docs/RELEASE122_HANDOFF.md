# Network Toolkit 1.17.122 — release candidate

This source is under validation; v1.17.121 remains the published release.
Development/source-branch validation is authorized. Binary publication requires
the pending separate user approval; no main/tag/release change is authorized by
this file itself. Repository visibility and old releases remain unchanged.

## Focused repair and evidence

The unchanged-production diagnostic control reproduced five failures in twenty
real Windows cases. Held-input polling missed four short admission windows;
one admitted operation negotiated with Explorer only at button release. Later
traces showed DOM capture acknowledgement did not establish native capture
completion. The first two repair iterations still failed two and one cases,
respectively; neither was accepted and their evidence is retained.

The final repair registers metadata only after a genuine drag threshold, keeps
it unarmed until the same gesture exits the window, and cancels late ended or
replaced gestures. No bytes are read at registration. Never-armed canceled jobs
retire their metadata and expiry timer; active/armed receipts are unchanged.
The fixed exact-job callback and owner-UI-thread-only capture completion retain
physical-button, foreground, lease, Escape and foreign-window guards. No input
is synthesized and no drop is forced. Idle polling remains unchanged.

Focused run 41020014856, source 13181fd9c10ee0aadac733fa3d3bfda7c7ad655d,
passed all 40 scenarios in two fresh main/child process sets: 28 real file/folder
copies, eight before/during cancellations and four internal dirty/undo drops.
Original timing schedules and hit-tested coordinates were unchanged from the
negative control. All eight fast copies negotiated before physical release;
all 36 native admissions completed capture transfer. Exact bytes, nested/empty
folders, unchanged sources, cleanup and child trace completeness were checked.
Both child trace sets have 332 events and zero dropped events; all eight actual
PNG captures were independently inspected. The artifact is 12960728806,
370451 bytes, SHA-256
2e2860f339a6a4845612b4451bbd500567206f23fff5c44d495d1253ee8e4ac8.

This proves the bounded repeated workload, not universal OS timing reliability
or a fully isolated explanation for the earlier cold-target delay. The
original uninstrumented 121 duplicate failure is not retroactively labeled
fixture-only. Desktop Shell/cross-integrity/arbitrary-app destinations and
live user devices, credentials and files are not claimed as tested.

Local 1352 JavaScript and 42 Python tests pass; focused Explorer Go-race and
Windows default/tag compilation pass. A broader local app race invocation was
terminated with signal: killed without an assertion; desktop/terminal/vault
packages passed. It is not recorded as an app pass. Exact normal hosted
application/security/dependency/native/package checks are pending.

Full release review, package/public verification and final closure are pending.
Retain existing comparison, Hosts, native Alt-tab and UAC coverage caveats.
