# Network Toolkit development

Read docs/DEVELOPMENT_HANDOFF.md before editing or building. Start from latest main
and preserve working features, settings and previous release safeguards.

Current verified published version: 1.17.98.
Source/tag: f727135d5789ec2015d3d5ae5c1853b9b5c97f8e
Tree: e9ec0584c719dc0b7d88bfc3e5c131b90bf2242e
Full Checks 40470397974 and Release 40470397914 pass on this source.
667 JavaScript tests, 35 safeguards and required Go/native Windows/browser
suites pass, including all seven updater timer cases and the real five-minute poll.
UI98: five groups, 24 diagnostic layouts and 11 packet cases; retained
UI97/96/95/94/93/92 and other application regressions also pass.
Prepared/public EXEs and publication-candidate/public portable files match.
All 808 packaged source files, 55 historical notes and 14 editor digests verify.
All 37 older releases/assets/bodies and 37 tags remain unchanged.
Released97 detected, verified and staged98 in 12.09s; no installation/restart
or complete native WebView2 interaction is claimed. See handoff for evidence.

Publication was explicitly authorized and is complete. Future releases require
authorization. Main network-toolkit remains the sole source/update feed.
Never replace old tags/assets, change access or write the retired feed. This
final three-file documentation record uses [skip ci].

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
