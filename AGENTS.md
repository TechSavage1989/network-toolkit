# Network Toolkit development

Read docs/DEVELOPMENT_HANDOFF.md before editing or building. Start from latest main
and preserve working features, settings and previous release safeguards.

Current verified published version: 1.17.91.
Source/tag: b41eb360d29abc8a1a332028ee8fadd9fd9d6c62
Tree: 4f839ed0a8c5cb4599f4b3c587d649cd6635814b
Full Checks 40302662081 and Release 40304158166 pass on this source.
513 JavaScript tests, 33 release guards, required Go/native Windows and browser
checks pass. UI91: 4 groups / 14 layouts; existing UI90,
Local Network and UI88 checks and covered regressions pass. Review/local/public
files match and all 759 source files match the tag. All
30 older releases/assets and 30 older tags are unchanged.
Released90 verified/staged91 in 24.41s. No actual installation/restart or
full native WebView2 interaction is claimed.

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
