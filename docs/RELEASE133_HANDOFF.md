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

