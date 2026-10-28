# Network Toolkit development

Read docs/DEVELOPMENT_HANDOFF.md before editing or building. Start from latest main
and preserve working features, settings and previous release safeguards.

Authorized development target: 1.17.106. The user explicitly approved the combined
server/client location-copy improvements and macro Library/recording fixes for
implementation, build and publication. See docs/RELEASE106_HANDOFF.md. Start from
105 closure main 30ecbf2a992beef6f0045272db3cc36acd0b27bb, preserve all existing
security/release safeguards, and keep 105 published tags/assets immutable.
No live user device, credential, settings or macro execution is authorized.

Current published version: 1.17.105. The user-authorized SSH banner alignment,
multi-item discovery filtering and explicit TCP-port actions are published and
independently verified. Exact source/tag:
1d716c1916833d9fb14dee44016bf0f75e568305 (tree 31fafbe7e2b327254e68b95cb2bb3bfa46f0a8cc).
All five exact-source required Checks and Release passed; four public assets,
source bytes, reviewed binary/portable identity and latest update feed verified.
Read docs/RELEASE105_HANDOFF.md and docs/DEVELOPMENT_HANDOFF.md for evidence,
limits and duplicate-main closure. This docs-only closure is not another build.

Published 104 credential security, graph, canonical icon, terminal pager/defaults
and output archives remain intact. Read docs/SECURITY104_HANDOFF.md,
docs/CREDENTIAL_SECURITY.md and docs/CONDENSED_GRAPH_SCALING.md as applicable.
No new feature/release or live user devices/credentials access is authorized by
this historical handoff. Keep every published tag/asset immutable.

The user expressly chose master-password encryption with Windows-account
remembered unlock across restarts and reboots after same-user-risk disclosure.
Remember stores a DPAPI-protected random key, not the password. Established
sessions stay connected on manual Lock and forget; new saved-credential uses
are blocked. Recovery is optional and migration is explicit native setup.
The retained old DPAPI vault requires separate user-confirmed cleanup. No actual
user vault/settings access or migration is authorized by this development record.

The prior 104 reviewed/published source is 554f360613aa960a28ee459a44018da39a0bbcff.
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
