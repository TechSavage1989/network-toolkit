# Network Toolkit development

Read docs/DEVELOPMENT_HANDOFF.md before editing or building. Start from latest main
and preserve working features, settings and previous release safeguards.

Current published version: 1.17.101. Source/tag:
2493e1978565b5302098df7dc7ed940c4764a6ef
Exact-source Checks 40493778300 and Release 40495692846 passed.
All four public assets, updater contents/feed, PE/version and 811 source files
verify and match the reviewed executable/portable build. Duplicate main Checks
40495692791 is still under observation; review and release are complete.
See docs/DEVELOPMENT_HANDOFF.md for evidence and honest limits.

The separate stable-recent Condensed update is authorized for an isolated build
and review-branch push only. Do not promote/publish it without further approval.
Never replace old tags/assets, change access or write the retired update feed.
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
