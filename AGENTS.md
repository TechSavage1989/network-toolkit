# Network Toolkit development

Read docs/DEVELOPMENT_HANDOFF.md before editing or building. Start from latest main
and preserve working features, settings and previous release safeguards.

Current published version: 1.17.103. The accumulated-history cluster/theme
release and its required review, Release and duplicate-main checks are complete.
Read docs/DEVELOPMENT_HANDOFF.md and docs/CONDENSED_GRAPH_SCALING.md.

The combined 1.17.104 candidate is in development, not yet published. Read
docs/SECURITY104_HANDOFF.md before resuming it. The user confirmed master-password
encryption with Windows-account remembered unlock across restarts and reboots
after disclosure of the same-user risk. Established sessions stay connected;
manual Lock and forget blocks new credential uses. Recovery is optional during
explicit native setup. No actual user vault migration or settings change is
authorized here. Finish required exact-source/native/rendered checks before
the authorized combined publication. Terminal pager/default and canonical icon
updates are also part of this build. No competing release or unrelated scope.

The next user-authorized work combines credential security and graph recent
spacing matching the supplied 30-second reference. The security lead owns the
combined build/main promotion/release. Keep graph work isolated and coordinate
its patch; do not publish a competing graph release or overwrite concurrent work.
Preserve Local Network pinned filters and unrelated behavior. Required exact-source
checks and actual rendered acceptance remain mandatory before safe promotion.

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
