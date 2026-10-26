# Network Toolkit development

Read docs/DEVELOPMENT_HANDOFF.md before editing or building. Start from latest main
and preserve working features, settings and previous release safeguards.

Current verified published version: 1.17.89.
Source/tag: eb318708454130b09d93204021b23b138657d6a2
Tree: 9be08c4672db7b0bfc6ea08360c137329f22ef44
Full Checks 40289268615 and Release 40290524884 pass.
Required JavaScript, release guards, Go/native Windows and browser checks pass.
Local Network layout/refresh: 5 browser groups / 12 cases; all previous 88 layout
and covered regressions pass. Review/local/public files match and all
749 source files match the tag. All 28 older releases/assets and
28 older tags are unchanged. Released88 verified/staged89 in 14.56s.
No actual installation/restart or full native WebView2 interaction is claimed.

The user authorized this publication and it is complete. Future releases need
authorization. Main network-toolkit is the sole public source/update feed.
Never replace old tags/assets, change access or write the retired feed. This
final documentation-only record uses [skip ci]. See handoff for scope and proof.

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
