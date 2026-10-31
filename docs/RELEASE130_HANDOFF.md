# Network Toolkit 1.17.130 — automatic profile names and type badges (candidate)

Not yet published. Requested2026-10-31 02:05 UTC; implementation, final local
checks, hosted browser/native/security/package acceptance and actual public
verification are pending. Published129 evidence below is historical, not proof
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
