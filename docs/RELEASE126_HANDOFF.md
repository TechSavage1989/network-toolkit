# Network Toolkit 1.17.126 — reliable administrative-state discovery

Published 2026-10-30T19:14:20Z; actual public downloads/source/feed verified
19:15:13.806831Z and independently 19:15:42.398434Z. Delivered 19:15:32 UTC.
Release 429399452: https://github.com/TechSavage1989/network-toolkit/releases/tag/v1.17.126
Source/tag ed90d1e5d4769fc4bbc97484bf7e263d1794fd4e;
tree 58e9852b3b8b74c580f6d29a0cf71c59abbb7e91. Main promotion used the
expected 125 head 0f5b46718ad133d2ae740112b797bbd46eea3449, non-force, and
was read back at 19:11:48 UTC. Existing visibility/access and prior assets remain.

This is a focused repair for the user's actual 125 report: Enable and Disable
were both unavailable because the administrative query did not produce a usable
state. No unknown-state bypass, new action or unrelated UI change was introduced.
The administrative enum mapping itself was correct and remains unchanged.

A read-only diagnostic tested the published production code unchanged in run
41110299103, source 4f200e20c255f571f6b8537defc079b786e4a8e5. Its five-second
query timed out after 5,035 ms. The identical script, with diagnostic streams
separated and a larger diagnostic budget, returned valid JSON in 13,080 ms:
eight rows, eight matching GUID/index pairs and four manageable interfaces.
There were 1,638 JSON bytes and 681 bytes of module-progress CLIXML on stderr.
Combining those streams can corrupt JSON even when the query succeeds. These
observations reproduce concrete production defects on hosted Windows; the exact
timing/cause on the user's own device was not directly measured.

The fix drains stdout and stderr into separate bounded buffers, parses only
stdout after a successful command, and suppresses read-only module progress.
The query cap is 20 seconds. IP inventory and IP/network preflight have 25-second
budgets inside the unchanged 30-second HTTP write deadline. Hosts remains at
eight seconds. A non-cancelled Enable/Disable refresh can use 25 seconds; already
cancelled actions and unrelated refresh paths retain their existing shorter
limits. Invalid JSON, process failure, cancellation, unknown state, conflicting
identities, locked/unsupported interfaces and stale revisions remain fail-closed.

The existing reviewed action, stable GUID/index/revision checks, exact unique
InputObject and one-shot just-in-time UAC are unchanged. No arbitrary shell,
module path, alias or persistent privilege is introduced. No automatic retry,
re-enable or restart is added. No actual user or CI NIC was enabled/disabled,
and no new visible UAC consent transition was tested.

The corrected exact-source Windows production gate passed: cold inventory took
4,243 ms, returned eight rows/eight identity matches/four manageable interfaces;
authenticated IP GET returned HTTP 200 in 822 ms with 19 native/supplemental rows
and four manageable interfaces; read-only request preparation took 785 ms.
The whole diagnostic passed in 5.86 seconds. It never dispatched a POST, helper
or mutation; its injected mutation callback would panic if invoked. All 15
network mock/guard groups also passed. This is actual read-only Windows proof,
not a claim of live NIC mutation, visible UAC or universal device compatibility.

The same-source packaging run independently passed its Windows gate: cold read
5,201 ms, authenticated GET 1,081 ms and preflight 1,077 ms, with the same eight
rows/eight identity matches/four manageable interfaces. The cold read exceeded
the old five-second cap but remained within the corrected budget. No network
mutation was performed in this repetition either.

Local validation: 1,387 JavaScript and 42 Python tests, all TestLocal (0.368 s),
15 focused network race groups (1.079 s), Windows cross-compilation and whitespace
checks passed. Hosted core passed 1,387 JS, 42 Python and eight ordinary Go
packages (application 29.591 s, not a full hosted race claim). Full Windows,
credential/capability security and dependency gates passed. Dependency findings:
zero npm and zero reachable/imported Go issues; the existing unused OpenPGP
GO-2026-5932 module advisory remains. Synthetic browser acceptance passed seven
groups/12 intercepted requests at 1440/820/390, all actual PNGs inspected, no
action-card overflow or page errors. The UI fixture is not live Windows state.

Original exact-source run 41111961750: all seven SUCCESS; last job completed 2026-10-30T19:18:52Z.
The initial review branch name did not match the existing portable-package guard,
so a packaging-compatible branch ran the identical source without code/test edits.
Same-source package review 41112659139: six SUCCESS / one comparison FAILURE; last job completed 2026-10-30T19:26:14Z.
Duplicate main run 41113241678: six SUCCESS / one comparison FAILURE; last job completed 2026-10-30T19:31:25Z.
All three real Chromium five-minute updater-timer steps passed.
The package-review editor job failed one logged detached390 wrapped right-map
Fit comparison case: peak/p95 paired offset 3.65625 px, all 40 inputs answered,
zero stalls and max response 68.1 ms. This is a retained checkpoint; its cause
is not established and it is not classified as fixture-only. No repair or rerun
was performed for this observation. Duplicate Windows passed again: cold read
8,250 ms, GET 842 ms and preflight 819 ms; the same eight rows/eight matches/four
manageable interfaces were verified, without any adapter mutation.

The duplicate editor job also failed one logged main390 right-map Fit case at
4.546875 px paired offset; its 40 inputs were answered, zero stalls and max
response 26.4 ms. These are failing-case measurements, not aggregate 48-case
statistics. The unchanged comparison boundary remains unresolved.

Historical comparison/gutter/motion, Alt-editor transfers, resize, Hosts/gear and
visible-UAC observations are retained. A passing repetition is not an unrelated
repair. No unqualified all-seven or universal reliability claim was used for
scoped publication. The complete 125 closure, including its real user failure
and original/duplicate failures, travels with this source and remains historical.

Release workflow 41113242240 build/publish succeeded. All four actual public
files match API digests, external/root/source manifests and the reviewed EXE and
portable bytes. Exactly 1,201 Git source files / 1,208 safe portable members,
AMD64 GUI PE 126, nine canonical icons, twelve resources and the four-file update
allowlist were verified. Latest feed/tag/main/notes agree. Prior 125 release,
body, tag and asset identities/sizes/digests/timestamps/URLs are unchanged except
public download counts. Evidence is under toolkit126-evidence, including
public/proof.json, security-review/PUBLIC126_PROOF.json and exact-source receipts.
This five-document closure changes no published application or download bytes.

Official references:
- https://learn.microsoft.com/en-us/powershell/module/microsoft.powershell.core/about/about_redirection?view=powershell-5.1
- https://learn.microsoft.com/en-us/previous-versions/windows/desktop/legacy/hh968170(v=vs.85)

No new future scope, live adapter mutation or repository permission change is
implied. Preserve all previous release assets and Windows/security boundaries.
