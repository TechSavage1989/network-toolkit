# Network Toolkit 1.17.133 - published Disconnect/Reconnect controls and optional bulk confirmations

## Publication
Published 2026-10-31T05:27:14Z (release ID 429742311).
https://github.com/TechSavage1989/network-toolkit/releases/tag/v1.17.133
Tag/source/main at publication: 2b320d7e89f47d3bb1652e273f37277059957671.
Tree: c0ae209c5a127c6bdd1006d3a5a772826f6e8c7b.
The release is stable, not a draft or prerelease. GitHub latest feed resolves
v1.17.133. Release workflow 41172455793 completed build and publish successfully.
No previous release, tag, or asset was overwritten.

## Delivered user behavior
Disconnect and Reconnect sit side by side in Active Session commands. Their
separate arrows reveal Disconnect all or Reconnect all, while the primary
buttons continue to affect only the active session. Escape/outside-click close
a dropdown without executing its action.

The three independent bulk preferences are terminalConfirmReconnectAll,
terminalConfirmDisconnectAll and terminalConfirmCloseAll. All default to true.
Each dialog offers Do not ask again; the Terminal display settings can turn
each confirmation back on. Existing individual tab-close confirmation is
separate. Preferences persist in normal and detached workspace display state.

Bulk operations revalidate target state, ownership, revision and eligibility
and prevent duplicate starts. They exclude other-window, unowned, transferring
or ineligible sessions. Reconnect all pauses for interactive authentication.
Disconnect all preserves open tabs/output; Close all tabs removes eligible
tabs. Skipping a confirmation does not relax those safeguards.

No new or changed credential access, connection transport, settings folder,
profile format, native updater or executable filename was introduced.
Synthetic browser/native tests do not prove universal device compatibility.

## Exact-source validation and retained failures
Exact release workflow ran 1,505 JavaScript tests successfully, 42 Python
tests, Go package tests, Windows x64 GUI build, and resource regeneration.
The portable archive package verification succeeded with 1,262 safe members;
the binary-only update ZIP retained the strict package format.

Source push CI 41171428723 completed all 14 jobs successfully.
Duplicate PR CI 41171432234 completed 13 successes and one Windows terminal
failure. TestWindowsLocalShellColorTranscript/powershell did not receive
the expected PowerShell command echo, while other local shell lifecycle tests
passed. The failure is retained and no root cause, waiver or fix is claimed.

Post-merge main CI 41172455812 completed 13 successes and one Editor browser
failure, with compact-sidebar gear focus, broader editor sidebar, and narrow
comparison-motion assertions failing. In the motion check, detached 390px right
comparison offset was p95/max 3.65625px with all 40 inputs delivered and no
reported stalls. This previously reported Editor behavior was outside the
release's scope and remains unresolved. The successful exact-source push
does not erase these failed observations.

The focused real-browser disconnect test verified the side-by-side controls,
open/dismiss behavior, and optional confirmation flows at normal/narrow widths.
Actual physical SSH, Telnet, serial hardware and real user usage are not
established by synthetic test fixtures.

## Public release metadata and prior immutability
GitHub-reported downloadable assets, sizes and SHA-256 digests:
- Network_Toolkit.exe: 21,069,312 bytes;
  67dcb864facca6a762f4e30d41cf43209558edd70ec3aaf6019415dffc0db0be
- Network_Toolkit_1.17.133_Portable.zip: 14,323,740 bytes;
  ba5550768d9f0886f89a7361e9f0b26ee7e188e2d7b241c8f507e4234f78efae
- Network_Toolkit_1.17.133_Update.zip: 8,052,278 bytes;
  8d40ff83d5a91a8e127bc11c0f74437e2933fbad4880bad93659449797838d86
- SHA256SUMS: 292 bytes;
  1b6010b2b5601722ac1cf65d2e0d92b61d5b29da2a8e51514734e96477976cbc

The prior v1.17.132 release ID 429728832, source/tag
db432733d9b5065d782fbd0685c5fcf78a6011e4, and all four original asset
IDs, sizes, GitHub SHA-256 digests and timestamps remain unchanged.
This verification checked the release workflow and GitHub public metadata;
separate independent public asset byte downloads were not completed, so no
independent rehashed-download proof is claimed.

This closure is documentation-only and does not modify the published v1.17.133
source/tag/assets or grant permission to change an unrelated feature.

---

# Network Toolkit 1.17.133 - adjacent Disconnect/Reconnect and optional bulk prompts (candidate)

Based on published 1.17.132. The user requested the matching split Disconnect
button immediately beside the existing Reconnect split control, plus optional
confirmation prompts for Reconnect all, Disconnect all and Close all tabs.
The original default for all three bulk confirmations remains on.
The new release must not overwrite 1.17.132 tags, assets or checksums.

## Behavior and safeguards

- Disconnect and Reconnect appear as adjacent primary actions. Each arrow
  opens only its corresponding secondary action below, closes on Escape or
  outside click, and does not execute any action by itself.
- Individual Disconnect and Reconnect operations are unchanged.
- Bulk requests use current-window session ownership and eligibility, exclude
  unowned/detached/transfer-busy sessions and recheck session identity,
  revisions and current state before each mutation. Bulk operations are
  serialized and a second concurrent bulk request is blocked.
- Each bulk confirmation has its own saved boolean preference:
  terminalConfirmReconnectAll, terminalConfirmDisconnectAll,
  terminalConfirmCloseAll. Defaults are true, and the user may disable each
  individually via that action's Do not ask again checkbox, or restore it in
  Terminal display appearance settings. Existing terminalConfirmClose applies
  only to individual live tab closes / Close other tabs and is not repurposed.
- Reconnect all continues to pause on an interactive authentication request.
  Closing tabs removes eligible tabs; disconnecting retains tabs and output.
  Without confirmation, an eligible bulk action starts immediately and retains
  the same session safeguards. Connected work or pending input may be interrupted.
- Preferences persist in normal and detached workspace display state.
  Saved profiles, authentication, session transfer, updater, executable filename
  and settings location are preserved.

## Acceptance requirements

Focused tests must prove primary/dropdown adjacency, independent enablement
and keyboard dismissal; modal cancellation and Do not ask again; restoring
all preferences; default-on behavior; unprompted bulk execution; stale token,
owner, state and transfer protections; no duplicate simultaneous operations.
Run normal/narrow Chromium scenarios and retained 132 reconnect checks without
private devices or live credentials.

Require exact versioned JavaScript/Python/Go, native Windows and browser CI,
security, PE version/resource regeneration, full-source portable packaging,
strict four-member binary update ZIP, release tag/feed/source consistency,
four public assets and unchanged v1.17.132 release and tag.
The prior v1.17.132 review showed the recurring narrow Editor comparison
motion failure. This project does not change Editor code or test limits.
Retain new and prior failures; do not claim an unrelated fix.

This is a release candidate, not publication proof.

