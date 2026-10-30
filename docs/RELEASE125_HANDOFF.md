# Network Toolkit 1.17.125 — selected adapter Enable/Disable

Published 2026-10-30T18:24:37Z; actual public source/downloads/feed verified
18:25:29.843113Z and independently 18:25:56.014749Z. Delivered 18:25:49UTC.
Release 429356950: https://github.com/TechSavage1989/network-toolkit/releases/tag/v1.17.125
Source/tag 0f5b46718ad133d2ae740112b797bbd46eea3449;
tree 94cb3eaa9ea575a4107973c22518691d9b3cf535. Main promotion used an expected-head,
non-force update from 124 closureb6cb94c6 and was read back at18:22:21UTC.
No previous tag, release, downloadable asset or repository access was changed.

Scope is only explicit Enable adapter and Disable adapter in Local Network >
IP configuration > Network actions. Restart adapter and all other 124 functions
remain. Disabled interfaces remain discoverable/selectable; true administrative
state is separate from cable/link state. The disabled-only inventory does not
invent IPv4/DNS settings or make them editable. Unknown state fails closed.

Post-release user observation (2026-10-30 18:30:37 UTC): the selected Ethernet
adapter reported unknown administrative state, so both Enable and Disable stayed
disabled while older actions remained available. This is a real functional
acceptance failure, not resolved by passing mock/UI tests. The 125 network
fixtures did not validate the actual read-only Windows administrative query.
A separate focused 126 investigation reproduced the production query timing out
at 5.035 seconds; the identical query with separate streams returned valid states
in 13.080 seconds and emitted progress CLIXML on stderr. No adapter was mutated.
The cause on the user device is not directly measured; 125 remains unfixed.

The existing serialized one-action elevated helper and explicit review remain.
Only typed actions with current GUID/index/revision and adapter scope are accepted.
The fixed read-only Get-NetAdapter inventory includes hidden/disabled interfaces;
its failure preserves existing IP editing but disables administrative actions.
Duplicate identities and unsuitable/locked interfaces fail closed. The helper
refreshes identity/state after UAC and again matches exactly one InputObject before
Enable-NetAdapter or Disable-NetAdapter. No renderer program/script/alias/wildcard,
persistent privilege, automatic retry/re-enable/restart or secret exposure is added.
Disable review warns that remote/VPN/IPv6 connections stop and the adapter stays
disabled until explicitly enabled. Stored IP/DNS configuration is preserved.
Unconfirmed post-action state is reported partial/unknown rather than success.

Local validation passed 1,387 JavaScript / 42 Python tests, 12 focused network Go
race groups(1.067s) and Windows test cross-compilation. Integrated focused Go
passed 0.014s. Exact hosted core passed 1,387 JavaScript / 42 Python /eight ordinary
Go packages(application32.431s, not a full hosted race claim). All 12 Windows
mock network groups passed, including five new inventory/state/command/execution
groups. Full Windows native/security, dependency and package gates passed.
Npm findings 0; Go reachable/imported findings 0; unchanged unused OpenPGP advisory
GO-2026-5932 remains. No live user/CI NIC enable/disable, other network mutation,
or visible UAC transition was performed; validation uses injected mocks and
intercepted UI. Existing native tests do not turn this into live-NIC proof.

Actual Chromium UI passed seven groups and 12 simulated requests: disabled-adapter
selection, administrative versus link-down state, Enable/Disable refresh, changed
interface index/current revision, exact GUID retention, Cancel/no request, prior
six actions, draft/static guards, simulated cancellation/conflict/partial failure
and no automatic retry. All 1440/820/390 screenshots were inspected; eight action
labels fit without overlap or overflow and Restart remains. No fixture repair
was needed in this 125 hosted run. The existing network test was moved earlier in
its job; no workflow job/assertion was removed or weakened.

Original review 41105114609: six SUCCESS / one FAILURE; last job completed 2026-10-30T18:30:27Z.
Duplicate main 41106368528: five SUCCESS / two FAILURE; last job completed 2026-10-30T18:39:48Z.
Original editor job124232691041 failed the previously documented detached-sidebar
keyboard-resize assertion at editor-views112.browser.cjs:71: Number(aria-valuenow)
>300 was false after a narrow-to-wide viewport change and ArrowRight. The editor
assets and this fixture are byte-identical to124; the same checkpoint is recorded
in RELEASE118_HANDOFF.md. This is not proof of cause or zero influence from other
shared changes. No fixture-only attribution, rerun, rebuild or unrelated repair
is claimed. Historical comparison/gutter/motion, Alt-tab transfer, Hosts/gear and
UAC observations remain; scoped publication did not claim all seven jobs passed.
Both original and duplicate real Chromium updater-timer steps passed.
Duplicate Windows failed the existing native editor ownership-ACK test: the
destination did not acknowledge and source ownership was retained. Duplicate
editor failed one logged main390 wrapped right-map Fit motion case at 4.546875 px
paired offset; its 40 inputs were answered, zero stalls, max response 24.6 ms.
These are existing checkpoints, but their causes are not established. No unrelated
repair or rerun was attempted and no universal no-data-loss claim is made..

Release workflow 41106368558 build/publish succeeded. All four actual public files
match API sizes/digests, manifests and the reviewed EXE/portable bytes. Exactly
1,198 source files / 1,205 safe portable members, AMD64 GUI PE 125, nine canonical
icons/12 resources and four-file update allowlist were verified. Latest feed/tag/
main/notes agree. Prior124 original asset identities, hashes, sizes, timestamps,
URLs, body and tag remain unchanged (download counts excluded). Evidence includes
public/proof.json, security-review/PUBLIC125_PROOF.json and exact-source receipts
under toolkit125-evidence. This five-document closure changes no release bytes.

Official references:
- https://learn.microsoft.com/powershell/module/netadapter/enable-netadapter
- https://learn.microsoft.com/powershell/module/netadapter/disable-netadapter
- https://learn.microsoft.com/powershell/module/netadapter/get-netadapter

Repository development/package instructions and prior safeguards still apply.
This historical record does not authorize unrelated work, real adapter mutations
or repository-access changes. Keep all published tags/assets immutable.
