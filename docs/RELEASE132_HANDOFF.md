# Network Toolkit 1.17.132 - Reconnect all and clearer release notes (candidate)

Requested: provide Reconnect all from the terminal tab right-click menu and a
small secondary dropdown beside the existing active Reconnect button. Make
What's New a high-level summary about the effect of changes; reserve Additional
Details for the more specific technical information. Publish a small numbered
update to verify the existing in-app updater. The previous published version is
1.17.131 until release/source/feed verification of 132 completes.

## Delivered scope

- The existing single-session Reconnect button remains the primary action.
  Its adjacent arrow expands a compact Reconnect all button below. The secondary
  option is also available on tab context menus and through keyboard navigation.
- Bulk reconnection uses current-window ownership and the existing
  terminalReconnectAllowed checks. It excludes other detached windows, busy,
  transferring, unowned or ineligible sessions; takes a review snapshot and
  checks token identity/revision and eligibility again before each request.
  All eligible session types and connected/closed/error states are considered.
- Confirmation warns that already-connected sessions and queued commands may
  be interrupted. Requests are serialized; parallel bulk starts are blocked.
  If reauthentication is needed, the batch pauses. Previous transcript data,
  existing tabs and saved profiles are not intentionally removed.
- The updater's What's New preview prioritizes items from the explicit
  structured section, no longer promoting details into the summary. Additional
  details expands the remaining technical notes. Legacy unstructured notes
  remain compatible and HTML is always escaped.
- Durable future-release writing rules are in docs/RELEASE_NOTES_STYLE.md,
  linked from AGENTS.md and docs/DEVELOPMENT.md. The root release notes use
  the exact section headings and prior131 notes are archived unchanged.

## Validation and publication gates

New focused JS tests cover ownership, stale revisions, cancellation, batch
serialization and authentication stop, alongside the two-button toolbar.
A real browser test verifies narrow and normal layouts, arrow and Escape
behavior, context-menu initiation, cancellation and actual sequential mock
backend requests without credentials or physical devices.

The exact final versioned branch must pass release metadata, JavaScript,
Python, Go, Windows AMD64 binary and resource generation checks, browser/native
rendered workflows and source/archive/updater packaging. Retain failures and
workload measurement limits. The known Editor comparison-motion regression
from 131 remains unrelated and unresolved unless proven otherwise.

Publication needs an authorized main merge and complete GitHub Release workflow,
four correct uploaded files, verified latest source/tag/feed/checksums, previous
131 release and original assets unchanged. This document is a candidate until
evidence is recorded; do not call version 132 published prematurely.
