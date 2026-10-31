# Network Toolkit 1.17.129 — terminal pane resize scheduling

Published 2026-10-31T01:59:27Z, release 429656958; actual public downloads,
source/manifests/updater/feed verified 02:01:35.447379Z and independently rechecked
against fresh public receipts 02:02:26.029257Z. Release run 41157694951 succeeded.
https://github.com/TechSavage1989/network-toolkit/releases/tag/v1.17.129

This combined release includes the128 naming corrections and129 resize fixes.
Version 128 remains untagged/unpublished, confirmed by fresh 404 tag/release reads;
its source is included through ancestor 60a183. PR2 closed merged through129's
exact commit; that does not mean128 was separately released or its failures waived.
Exact reviewed 129 release notes and package bytes remain unchanged. This source-only
handoff explains the combined features without retroactively rewriting those bytes.

Final source 0de3c5708ce7885ae105ff5ee3601b18336941d4;
tree 00ab469d2ef3f2bff8da7d8ecf94474b2a1bdbee;
local a832c594593c0d8249094af3381a04618aa19e7a has that exact tree.
Previous published release is127, source 3085e0729be9f7cbbe372f9959c8985f5255b220.

## Included naming corrections from the unpublished128 candidate

Manual name takes precedence over learned CLI hostname, then endpoint fallback.
Names appear above smaller endpoints without changing saved connection addresses,
ports, users or credentials. Use device name / IP clears the manual display
override while preserving identifying profile text; explicit rename/save enables
it again. Backend revisions reject stale rename/clear acknowledgments across
main/detached windows. Numeric names and bounded documented network/shell prompt
families are recognized; arbitrary/hostless/ambiguous custom prompts retain a
reliable prior name or endpoint. Naming does not expand paced-send readiness or
prove remote command acceptance. See TERMINAL_PROMPT_NAMES.md for boundaries.
The synthetic state server now matches already-correct production legacy manual
name inference; this fixture correction is not a product naming repair.

## Reproduction and bounded correction

A synthetic 10ms movement sequence reproduced starvation in the previous trailing
60ms local-fit timer: resetting it repeatedly postponed xterm reflow until movement
stopped. A separate 20ms geometry sequence reproduced the same starvation in the
previous trailing120ms transport debounce. These are scheduler reproductions,
not a complete visual reproduction of the user's machine or live remote TUI.

Local fitting now retains the first pending16ms deadline and reads the latest
geometry when it runs. A per-terminal-view observer also detects pane/body changes
when the outer stage size does not change; it follows retained/reparented nodes
and disconnects with existing display disposables. Zero, infinite and otherwise
invalid geometry is skipped. Selection and scroll restoration remain protected.

PTY resize transport retains the first pending120ms deadline while replacing its
payload with the latest valid dimensions. Single-flight acknowledgments, session
epochs, no-op checks, ownership/restoration/transfer/page guards and prompt/output
revision safety remain. No-op acknowledged fits do not destabilize name detection.
Prompt-paced submission, Pause/Stop and uncertain-start safeguards are unchanged.

Local xterm reflow and remote application redraw are different: the latter depends
on acknowledged PTY resizing and the remote application. No synthetic remote
redraw or automatic input is sent by the application. Static text and arbitrary
remote programs cannot be promised universal reformatting or compatibility.

## Exact-source validation snapshot

Final local 1,451 JavaScript and 42 Python tests passed. Exact-source independent
package proof at 2026-10-31T01:34:27.738469Z passed artifact 13024549963 from
push 41155463505:1,233 source files,1,240 safe portable members, CRC/both manifests,
AMD64 GUI PE32+, file 1.17.129.0/product 1.17.129, 12 resource leaves and 9 icons.
Reviewed EXE SHA-256:
dbcb850abb14abdaeca1fb30e0b8f08bc002ff2cb25c3767c6ab051969ef2ab6
Reviewed portable SHA-256:
bc76e68a5d8d536a2b7868fe7a5386bd851d05129b14cfaef499c55bb2706ed0
This is exact-source package acceptance, not actual public-download verification.

On final 0de3c57, focused browser resize passed both push and PR. Inspected PR
artifact 13024794044 contains 8 contained-geometry matrix checks, 69 resize
acknowledgments, 43→32 columns during held drag and no script errors; actual pixels
show selection retained and both panes contained. PR native resize passed with
66 actual Telnet NAWS packets,26 pointerdown samples,10 checks and one connection.
Inspected alternate-screen/returned screenshots show size markers within the pane
and retained scrollback. Actual Windows WebView2/Win32 loopback Telnet is tested,
not live SSH devices or universal remote-TUI compatibility.

Duplicate push native resize failed initial common selection/scroll setup BEFORE
any129 resize phase. Its report lacks failing values/screenshots. The cause is
unproven; a successful PR counterrun does not erase it or establish flakiness.
Earlier adcd963 all four focused passes remain earlier-source evidence only.

Three original 128 Alt-editor failures remain. Passive tracing initially changed
neither coordinates/timing nor acknowledgment behavior; original failure cause
was not established as a product defect. A subsequent shared native fixture
correction supplies pointer settling and correct final-point/Alt-release ordering,
with no production change or gate removal. Three initial actual corrected
cross-transfer proofs passed; the fourth PR diagnostic originally failed at 300s
visible-owner startup before UI/gesture. Its one authorized retry passed, with the
final point before mouseup, Alt release 161ms later and broker claim 8ms later.
This is actual fixture/runtime evidence, not a product fix for legacy editor geometry.

Accepted exact-source PR 41155467693 completed all 12 jobs successfully after that
one startup retry. All original outcomes remain retained. Duplicate push on the same source had two failed jobs and three reported
assertions. The editor job failed compact-sidebar main390dark control wait after
15s (150px height gain and badge normal; cause unresolved), and the existing
editor-motion114 p95 offset 4.546875px. Native resize failed initial selection
before any resize phase. Its cause remains unproven. Passing PR evidence does
not establish that any of these duplicate conditions are fixed.

The separate 128 final review had 9/10 passes. Its one bounded browser retry failed
again at the existing motion-offset baseline 4.546875, row 256/p95-sensitive sampling
check (90 frames,40 answered requests,zero stalls). That defect is not claimed fixed. Version 128 was not published or tagged;
129 passed its own complete exact-source12-job acceptance, rather than bypassing
or relabeling128's failing checks. No two release workflows were raced.

Post-merge duplicate Checks41157694962 completed2026-10-31T02:15:36Z:
11 jobs succeeded and browser-editor112 failed. Requested terminal/name/resize,
native/full Windows and security checks passed; broad browser including its real
five-minute updater timer passed. The retained legacy editor-motion114 failure
was main390 left-map fit/negative gesture: p95 offset4.140625px, maximum4.609375px,
40 inputs answered within2 frames and zero stalls. This is not an all-main pass or
a claim that the legacy alignment condition was fixed. Accepted PR all12 and
actual public verification remain unchanged;130 acceptance is separately pending.

The inherited broad browser timeout increased 20→25 minutes while keeping every
check and the real five-minute updater poll. Prior 127's exact 20-minute cancellation
despite successful logged steps suggests timeout-boundary pressure; it does not
prove the original cause. Preserve all previous failed/cancelled observations.

## Actual publication and immutable prior release

All four actual public downloads passed size/digest checks and exact manifests.
Public portable and EXE bytes equal the independently reviewed package; 1,233
tracked source files and 1,240 safe ZIP members, CRC and both manifests passed.
Updater has the exact safe four-member allowlist, matching EXE and documentation.
Published notes equal the reviewed root release body. Tag/main at verification,
release source and latest updater feed point to the exact commit/tree above.
Independent checks re-read fresh public receipts and rechecked source/PE/bytes.

Public asset SHA-256 values:
Network_Toolkit.exe
dbcb850abb14abdaeca1fb30e0b8f08bc002ff2cb25c3767c6ab051969ef2ab6
Network_Toolkit_1.17.129_Portable.zip
bc76e68a5d8d536a2b7868fe7a5386bd851d05129b14cfaef499c55bb2706ed0
Network_Toolkit_1.17.129_Update.zip
96739fa46e3cab1a62a7f3b9d8ea7d6f8f73c6528898b608dd707d9aa6ac8fa3
SHA256SUMS
ffa160cb25218d577b5f2e6d31a4e1f895003962afe6dcfbdc92e1c67102eecf

Prior 127 source/tag 3085e0729be9f7cbbe372f9959c8985f5255b220 and release 429573909
remain unchanged: entire release record except legitimate download counts,
asset IDs/sizes/digests/URLs/timestamps, body and exact tag receipt match baseline.
Fresh version 128 tag/release requests returned 404;128 remains unpublished.

This source-only closure changes no release notes, version, workflows, binary or
package assets, tags, repository visibility/access or general authorization.
Preserve all original observations and historical notes. Private user screenshots,
configuration, names/IPs and credentials are excluded from repository artifacts.
