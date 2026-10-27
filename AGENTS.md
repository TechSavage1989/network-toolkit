# Network Toolkit development

Read docs/DEVELOPMENT_HANDOFF.md before editing or building. Start from latest main
and preserve working features, settings and previous release safeguards.

Authorized development target: 1.17.105, covering SSH authentication-banner
line endings, multi-item discovery filtering and explicit TCP-port actions.
Publication is authorized after exact-source required checks and release asset
verification. See docs/RELEASE105_HANDOFF.md. No user devices or credentials may
be accessed for validation. Keep the published 104 tag/assets immutable.

Current published version: 1.17.104. The combined credential-security, graph,
canonical icon, terminal pager and new/unset-default release is published and
independently verified. Read docs/DEVELOPMENT_HANDOFF.md,
docs/SECURITY104_HANDOFF.md, docs/CREDENTIAL_SECURITY.md and
docs/CONDENSED_GRAPH_SCALING.md before further work.

The user expressly chose master-password encryption with Windows-account
remembered unlock across restarts and reboots after same-user-risk disclosure.
Remember stores a DPAPI-protected random key, not the password. Established
sessions stay connected on manual Lock and forget; new saved-credential uses
are blocked. Recovery is optional and migration is explicit native setup.
The retained old DPAPI vault requires separate user-confirmed cleanup. No actual
user vault/settings access or migration is authorized by this development record.

The reviewed/published source is 554f360613aa960a28ee459a44018da39a0bbcff.
This handoff-only closure does not replace published assets or authorize a new
feature/release. Keep future work scoped, preserve explicit settings and existing
release safeguards, and coordinate concurrent files before editing.

Never replace old tags/assets, change access or write the retired update feed.
Record honest retention/legacy/partial-summary limits; no unbounded raw history.
Final handoff-only records use [skip ci].

## Release-note organization

Keep only the current version's `RELEASE_NOTES_<version>.txt` at the repository
root: release validation, packaging and publication require that path. Use the
version in `internal/model/model.go` (and its matching UI version), not a guessed
filename. When an authorized build advances the version, move the prior note
unchanged into `docs/releases/`, update that directory's index and any references,
and leave the new current note at root. Preserve every historical note; do not
recreate the release-note wall in the root directory. This organization does not
authorize a build, new release, tag move, asset replacement or access change.

## Compact repository layout

Keep application Go source and tests together in `cmd/network-toolkit/`; put
new developer and user guides in `docs/`. `resources.go` at root owns the
embedded UI assets; preserve its embed patterns and paths. Build the command
with `go build ./cmd/network-toolkit`, not `go build .`. Use the root build
wrappers and read `docs/DEVELOPMENT.md` for tests and packaging. Update all
consumers when moving paths. Keep release-note rotation above unchanged.

The exact tracked root allowlist is enforced by `scripts/repository_layout_test.py`
in existing Checks and release CI. Put new files under the documented directories,
not at root. Intentional new root entries require a reviewed allowlist change and
justification; the sole current release note is derived from the application version.
