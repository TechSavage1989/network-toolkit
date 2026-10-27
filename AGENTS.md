# Network Toolkit development

Read docs/DEVELOPMENT_HANDOFF.md before editing or building. Start from latest main
and preserve working features, settings and previous release safeguards.

Current published version: 1.17.100. Source/tag:
783981434a615307b09eeb7f83ef2671b04b3d44
Full exact-source review Checks 40486961204 and Release 40489248759 passed.
All four public assets, updater inventory, PE/version, matching executable and
810 packaged source files verify. Source and executable match the reviewed build.
Duplicate main Checks 40489248768 also passed all jobs on the published source.
See docs/DEVELOPMENT_HANDOFF.md for implementation, hashes and honest limits.

Publication was authorized and is complete. Future builds/publication require
separate authorization. Never replace old tags/assets, change access or write
the retired update feed. Final handoff-only records use [skip ci].

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
