# Network Toolkit 1.17.130 — automatic profile names and type badges

Published 2026-10-31T03:13:29Z, release 429690714. Release run 41163257124 succeeded.
Actual four public downloads/source/manifests/updater/feed verified 03:14:21.181667Z;
independent fresh receipts and source/PE/public-byte recheck passed 03:15:04.117528Z.
https://github.com/TechSavage1989/network-toolkit/releases/tag/v1.17.130
Source/tag 77ceeec0b9b258dc7e3a1dfc29f9992ad3d5d884;
tree 6b88d9a83a666f2363c496b21e7ec45cb2c6fd9a.

## Delivered behavior

Blank profile names stay automatic: a reliable learned CLI name is preferred,
then IP/configured host or serial COM endpoint. Saving blank before or after
learning does not freeze the learned name as manual. Explicit manual names take
precedence until cleared. Saved addresses, ports, users, credentials and connection
identity remain unchanged. SSH, Telnet, Serial, CMD and PowerShell type badges
supplement the name/endpoint in tabs, Saved and Recent without changing transport.

New Recent records retain revision zero until their own positive acknowledgment;
existing records keep their own revision. Polling must not promote an unacknowledged
new record's revision. The transient creation marker is excluded from persisted
settings. Backend conflict checks, pending rename/clear distinctions and legacy
revision-zero handling remain intact. This fixes the async save rejection found
in pixel review; it does not weaken revision/ownership safeguards.

Readable tabs reserve 200px; protocol-bearing Saved/Recent rows reserve 244px in
the existing scrollable area. At a 390px viewport width, horizontal overflow remains:
right-side controls/badges may require scrolling. Geometry and rendered labels
were checked; simultaneous full-row visibility and narrow right-side clickability
are NOT claimed. No global sidebar resizing or universal mobile-layout claim.

## Exact-source validation and actual runtime

Final local 1,477 JavaScript and 42 Python tests, full application Go, focused race
checks and Windows AMD64 test cross-compilation passed. Accepted exact-source
PR 41161793745 passed all 14 jobs, including aggregate Windows. Focused runtime is
not used as a substitute for that complete acceptance.

Final browser push/PR reports each contain 90 checks / 12 screenshots and zero errors,
covering responsive/theme/name/type cases. Actual final 390light Saved and 1440dark
Recent pixels were inspected; synthetic serial fixtures are not physical COM proof.

Actual Windows WebView2 native 130 push/PR reports and saved/final pixels passed
without error toasts. Blank Save before/after real synthetic prompt output stays
automatic; manual Save and clear, three badge surfaces and authenticated persisted
state restoration passed. A fresh scoped detached child and return preserve
session provenance and parent profile; explicit reconnect and the normal async
save verify persisted Recent identity/provenance. This is synthetic loopback
Telnet, not live SSH, physical serial, native page reload or full-app restart.
Native bootstrap navigation security and scoped child privacy remain unchanged.

Final native 127 visible-target gesture checks passed all 5 phases, 6 detach/return
timing records, label/revision/selection and single-loopback-transport checks.
Actual reorder pixels show the intended live tab in the third position. The test
exposes the target inside the visible strip before the real gesture; production
drag/ownership behavior was not weakened. PR aggregate Windows also passed its
native names/gestures and 4 MiB editor stages.

## Failed observations retained

Initial 45f7d49 failed actual narrow PowerShell tab width and a forbidden native
location.reload fixture. Candidate 7747237 then exposed zero Saved-row identity
width and the incorrect expectation that a scoped child receives the global saved
profile list. Those fixtures now use authenticated state/fresh child scope rather
than weakening navigation or sharing private global state.

Candidate 54822b5 green focused runtime jobs were NOT accepted: its final screenshot
showed a save-rejection toast after reconnect because success was captured before
the debounced write. This led to the real new-Recent revision race correction and
strict awaiting of persisted backend state/error-free pixels. Its aggregate native
fixture also targeted a clipped tab outside the strip; earlier failure is retained.

Final review push 41161788626 completed 13 jobs successfully and one Windows job
failed the legacy editor outside-window release:
real backend never recorded pending transfer. Later native stages were skipped,
not failed. Consequential artifact errors are retained. This duplicate failure is not claimed
transient or fixed by the accepted
PR counterrun. All earlier 129/128 failures remain in their historical handoffs.

Post-merge main duplicate 41163257051 completed successfully on the exact published
source: all 14 jobs passed. API updated_at is 2026-10-31T03:31:00Z; terminal success
was confirmed at 03:31:50 UTC. This final main result does not erase the earlier
review-push failure or rejected green-job/error-pixel observations above.

## Actual published package and prior-release immutability

Actual public portable/EXE bytes equal the independently reviewed package: 1,244
exact tracked source files, 1,251 safe portable members, CRC/both manifests,
AMD64 GUI PE32+, file 1.17.130.0/product 1.17.130, 12 resource leaves and 9 icons.
All four public assets match size/digest metadata. Update ZIP has the exact safe
four-member allowlist, matching EXE and documentation. Release notes equal the
reviewed root notes; source/tag/main at verification and latest feed agree.

Public SHA-256 values:
Network_Toolkit.exe
0c205a02ea7c0df0cae1fa85914be3e0a22b06bcb2f05f160cc1f33f708f0df0
Network_Toolkit_1.17.130_Portable.zip
2fedd447408b8abbe906febb3a1dd8000ecd0f99f3127f00fbfa4d079f866de8
Network_Toolkit_1.17.130_Update.zip
34742109e8f4adb547074b77558c4034246a2663830abfdf9d3e2833ca55f1e6
SHA256SUMS
e9629b69ad244ac8f6a648cdcbd688abe576e4aa489da6c536786b17fe96fbdc

Prior 129 release 429656958 and tag 0de3c5708ce7885ae105ff5ee3601b18336941d4 remain
unchanged: full API record except legitimate download counts, exact tag and asset
IDs/sizes/digests/URLs/timestamps. Version 128 remains untagged/unpublished, subsumed
by 129. This source-only closure changes no root release notes, version, workflows,
published source tag or release asset bytes. Historical notes and failed observations
are preserved; no repository visibility/access or blanket authority changes.

## Historical130 development observations (superseded by closure above)

# Network Toolkit 1.17.130 — automatic profile names and type badges (candidate)

Not yet published. Requested2026-10-31 02:05 UTC. Initial implementation/local
checks completed, but scoped fixes follow failed initial hosted browser/native
checks. New exact-source validation and actual public verification are pending. Published129 evidence below is historical, not proof
that130 is accepted. Version128 remains untagged/unpublished and subsumed by129.

## Requested behavior and acceptance boundaries

A blank saved-profile display name means automatic naming, not a manual override.
Prefer a reliably recognized CLI name, then the actual connection endpoint:
IP/configured hostname or serial COM port. A learned name visible while saving a
blank profile must not be frozen into manual metadata; later reliable device-name
changes must still update its display. Unknown/ambiguous prompts retain bounded
fallback behavior. This is not universal prompt recognition or identity proof.

Explicit manual names retain precedence until explicitly cleared. Saving/reloading,
reconnecting, edits to existing profiles and recent-session reuse must preserve
the distinction between automatic and manual names. Display revisions must prevent
stale main/detached snapshots from undoing rename/clear. Addresses, ports, users,
credential references, profile IDs and saved connection identity must not change
merely because a display name is blank or learned.

Subtle connection-type badges identify SSH, Telnet, Serial, CMD and PowerShell in
tabs, Saved and Recent. Badges supplement rather than replace name/endpoint labels;
small layouts/themes and long labels must remain legible, with controls usable.
CMD/PowerShell local sessions must not be mislabeled as network connections.
Type distinctions must not rely solely on color or modify session transport.

Existing session ownership/transfer, prompt-paced submission, Pause/Stop,
uncertain-start prevention, bounded resize scheduling and selection/scroll
safeguards remain required. No automatic command execution or credential change
is authorized by display metadata. Fixtures must be synthetic; private user input,
screenshots, device names/IPs and secrets must not enter source/test artifacts.

## Validation and publication pending

Record final exact source/tree and actual local counts only after tests finish.
Require focused tests for blank create/edit/clear and explicit names, changing
learned names after blank-save, fallback IP/host/COM, reload/reconnect, stale
revision and matching Saved/Recent identity. Verify actual rendered type badges
and name/endpoint layout for all five types, themes/narrow widths and relevant
main/detached lifecycle flows. Browser/native mock coverage is not live SSH proof.
Retain all required aggregate/runtime/security/package gates and earlier failures.

Before publication acceptance, verify exact packaged source/manifests/PE, actual
public downloads and hashes, tag/source/latest updater feed and immutable129
release/assets. Archive129 notes byte-identically. Do not rewrite published bytes,
change repository visibility/access or silently treat a prior passing run as130
acceptance. Final130 release metadata/hash/feed fields remain pending.


## Local candidate checks (2026-10-31 02:20 UTC)

All 1,467 JavaScript tests and 42 Python tests passed, together with the full
application Go tests, focused serial display-name race checks and Windows AMD64
test cross-compilation. Hosted browser/native checks and package/public proof
remain pending; local compilation is not runtime acceptance.

## Initial hosted observations and scoped corrections (02:27 UTC)

Initial local source1ffe8c4/tree9258a80 passed 1,467 JavaScript tests,42 Python
tests, full application Go, focused race checks and Windows test cross-compilation.
Its hosted candidate45f7d49 failed the new focused browser/native checks; these
failures remain retained and are not superseded by the local passes.

Actual390px browser rendering gave the PowerShell tab name zero width while the
badge occupied66px and close control30px. The scoped correction sets a200px minimum
tab width in the existing scroll strip so name, badge and close remain readable.
The strict visibility assertion remains; fresh exact-source runtime proof is pending.

Native blank-save before/after learning, explicit manual save, clear and all three
badge-surface assertions passed before the fixture attempted location.reload.
Native bootstrap security intentionally blocks that navigation. The fixture is
being corrected to verify authenticated persisted-state restoration and a fresh
detached-child bootstrap, without weakening that security boundary. This must
not be described as native page-reload or full-application-restart coverage.
New candidate aggregate tests and actual hosted/browser/native/package/public
acceptance are pending. No live SSH/device compatibility is established.

Corrected local candidate: 1,468 JavaScript tests passed, including the narrow-tab
minimum-space regression; Windows test cross-compilation passed. Production Go
and Python sources are unchanged from the earlier full passes. Hosted checks on
the corrected source remain pending; original failed runs are retained.

## Second hosted correction (7747237 retained)

The corrected browser run passed every 390px tab, then exposed zero remaining
Saved-row identity width in the narrow sidebar. Protocol-bearing Saved/Recent
rows now reserve 244px within the existing horizontally scrollable list; labels,
badges and folder/delete controls remain independent. Strict visual assertions
are unchanged. No global sidebar resizing or hidden controls are introduced.

Native130 progressed through authenticated persistence to a real detached child.
That child correctly receives only narrow presentation preferences, not the
parent's global saved-profile list. The fixture must assert that absence, check
revisioned automatic session metadata, and verify the persisted parent profile
on return. It must not require or introduce global profile disclosure to children.
Original failed evidence remains; fresh corrected hosted acceptance is pending.

Combined second correction: all 1,470 JavaScript tests and Windows AMD64 test
cross-compilation passed locally. Hosted runtime/package proof remains pending.

## Asynchronous persistence defect found in pixel review

Candidate54822b5 passed both new runtime jobs, but its push final screenshot showed
a settings-save rejection after reconnect. The fixture had captured success before
the debounced write finished. A new Recent record inherited a saved profile's
revision, and polling could raise a new record's revision before its own first
acknowledgement. Backend rejection of nonzero new-record revisions was correct.

New records now remain revision zero until a positive acknowledgement for their
own ID; existing Recent records retain their own revision. The transient creation
marker is excluded from persisted settings by the existing backend whitelist.
Pending rename/clear and legacy revision-zero records remain distinct. Backend
conflict guards are unchanged. Native130 now awaits successful normal persistence,
checks backend Recent identity/provenance, and rejects storage/error-toast failures.
Original green-job/failed-pixel evidence is retained, not counted as acceptance.

The wider readable tabs also exposed a native127 gesture fixture error: its third
draft target lay beyond the clipped tab strip over the sidebar, correctly causing
a detach. Three prior detach/return cycles had passed. The fixture must expose
its reorder target and verify the pointer lies within the visible strip before
performing the real gesture; ownership assertions and production behavior stay.

Integrated creation-race and native fixture candidate: 1,477 JavaScript tests,
42 Python tests, complete application Go tests, focused race tests and Windows
AMD64 test cross-compilation all passed locally. Fresh hosted acceptance remains
required; the earlier package is superseded, not promoted.
