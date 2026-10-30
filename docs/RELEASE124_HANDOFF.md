# Network Toolkit 1.17.124 — Recent endpoints and reviewed network maintenance

Published 2026-10-30T16:50:53Z; actual public downloads/source/feed verified
16:51:36.213209Z, independently verified 16:51:49.273239Z.
Release 429273949: https://github.com/TechSavage1989/network-toolkit/releases/tag/v1.17.124
Source/tag 043851be8c8a23fa4711699855d74aa4c01afce1;
tree e88776b3f1748f1b47b6f338c5c40e1542d4d2b6. Expected-head/non-force main
promotion was verified 16:48UTC. The original large UI upload waited for its
specific approval; it completed normally at 16:25 without a duplicate retry.
Repository visibility/access, prior 123 release/tag/body/assets and all older
published artifacts remain unchanged.

Mature Condensed Recent graphs now anchor their real first/last visible records,
not a fractional requested boundary before the first arrival. Requested window
bounds remain separate for the navigator/preset; axis labels use actual instants,
and the tooltip distinguishes selected duration from recorded duration. Startup,
Linear, 10-second baseline density, curve/count and older-history behavior remain.
The final fixture passed 48 advancing frames at 1440/390 in light/dark, including
irregular cadence, missing and failed samples, plus fractional historical windows.
Every endpoint gap was exactly 0 px. Published 123 reproduced a 38.43 px left gap.
All 12 legacy curve layouts, count 500 persistence/reload, baseline 112 comparison
and exact 5.217391304 s selected versus 5 s recorded semantics passed. Actual graph
pixels were inspected; this is synthetic Chromium, not live-device/WebView2 proof.

Local Network adds six explicitly reviewed actions: adapter-scoped DHCP IPv4
release/renew and adapter restart; computer-wide Flush DNS, Register DNS and
Reset Winsock. Strict typed requests, current GUID/index/revision, static/complex
DHCP guards and existing one-shot just-in-time UAC remain. Native DHCP uses only
a uniquely matched real interface identity. A dispatched synchronous helper keeps
the serialization fence until it exits; timeout/cancellation is not rollback.
Winsock reset is NOT full TCP/IP reset, does not rewrite static addressing, and
never schedules restart. Results include scope, errors/partial status and Copy.
All real network changes remain explicit user actions inside the application.

The new seven Windows mock/construction groups passed; all six browser action
flows, Cancel/no request, static and unsaved guards, normal form restore/blur,
simulated UAC cancel/conflict/partial results, Copy and manual-reboot notice passed.
Actual 1440/820/390 action panels were inspected with 0 px overflow. No actual
DHCP/DNS/restart/reset, live adapter mutation, new visible UAC transition or real
clipboard result was tested. Existing native/security suites passed separately;
that does not turn synthetic maintenance tests into live-action validation.

TCP Ports now uses its own code-native rack/ports/magnifier SVG in navigation and
tool metadata; Connect retains its dedicated plug. Four new full/collapsed/narrow
light/dark modes and 14 existing Connect cases passed; all six icon PNGs were
inspected, with final bytes identical to the inspected first review.

Review 1 had test-harness omissions: ambiguous Cancel matching the header X and
footer button, an unhandled existing editor metadata POST, and a legacy graph
caption assertion requiring visible instead of selected/recorded. Review 2 passed
all six actions but its direct draft-map reset while a field remained focused
reintroduced an unsaved draft and correctly blocked later actions. Final fixtures
use real field restore/Tab/selection and strict metadata, scope and span assertions.
No production code changed after the first candidate. Independent baseline setup
now runs after unrelated failures rather than cascading absent-fixture errors.
These original failures remain recorded, not called successful executions.

Final hosted core passed 1,384 JavaScript, 42 Python and eight ordinary Go packages
(application 32.393 s, not a full hosted race claim). Focused local network race and
Windows compilation passed. The initial local aggregate passed 1,382 JS / 42 Python
and five Go packages; three Linux discovery/platform packages hit sandbox netlink/
neighbor-read EPERM, so that full local Go command is not a pass. Fresh hosted
core resolved those tests. Dependency scans: zero npm and zero reachable/imported
Go findings; the existing unused OpenPGP GO-2026-5932 advisory remains.

Final review 41091930040: all seven SUCCESS; last job completed
2026-10-30T16:59:19Z. Duplicate main 41093184596: six SUCCESS/one FAILURE
(comparison motion); last job completed 2026-10-30T17:08:03Z. Both real Chromium timer
steps passed. The duplicate failed 1/48 comparison cases: main 390 px wrapped
right-map Fit movement, peak/p95 paired-row offset 4.546875 px. All 1,920 inputs
were answered (maximum 78.3 ms), with zero stalls and source-byte retention checks
passing. Editor production assets and motion fixture are byte-identical to 123;
that does not establish a cause or prove zero influence from other shared code.
The existing comparison boundary remains unresolved, not called fixture-only.
No production repair, rerun or rebuild was performed for this observation.
New 124 graph/network/icon and Windows/core/security checks passed again;
the reviewed package and public-byte proofs remain unchanged.

Release 41093184796 build/publish passed. All four actual public files match API
hashes and the reviewed portable/EXE bytes. Exactly 1,193 source files / 1,200 portable
members, both manifests, AMD64 GUI PE 124 / nine canonical icons/12 resources,
updater four-file allowlist, latest feed/tag/main/notes and prior 123 immutable
metadata were verified. No previous release was replaced.

Preserve historical comparison/gutter/motion, Alt-editor tab-transfer, Hosts
focus, gear-keyboard and visible-UAC limitations. A later passing run is not a
repair of those observations. No unrelated behavior, live-user device access,
repository permission change or future release is authorized by this closure.

Microsoft references:
- https://learn.microsoft.com/en-us/windows/win32/api/iphlpapi/nf-iphlpapi-ipreleaseaddress
- https://learn.microsoft.com/en-us/windows/win32/api/iphlpapi/nf-iphlpapi-iprenewaddress
- https://learn.microsoft.com/en-us/windows/win32/api/iphlpapi/nf-iphlpapi-getinterfaceinfo
- https://learn.microsoft.com/en-us/windows-server/administration/windows-commands/ipconfig
- https://learn.microsoft.com/en-us/powershell/module/netadapter/restart-netadapter?view=windowsserver2025-ps
- https://learn.microsoft.com/en-us/windows-server/administration/windows-commands/netsh-winsock
- https://learn.microsoft.com/en-us/troubleshoot/windows-server/networking/reset-tcp-ip-net-shell
