# Network Toolkit development

Read docs/DEVELOPMENT_HANDOFF.md before editing or building. Start from latest main
and preserve working features, settings and previous release safeguards.

Current verified published version: 1.17.94.
Source/tag: 3305c38d4fc7da067fed98d54f57abe1cedd3fe4
Tree: d15db7e42a0d6f8f0d0f5eba0844df726c91fbe9
Full Checks 40415580921 and Release 40417147184 pass on this source.
624 JavaScript tests, 35 release guards and the
required Go/native Windows/browser checks pass. UI94: 8 groups /
16 layouts; UI93/92/91/90, Local Network/UI88 and covered regressions
also pass. Review/local/public files match and all
785 packaged source files match the tag. All
33 older releases/assets and
33 older tags are unchanged. Released93 verified/staged94
in 13.11s. No actual installation/restart or full native WebView2 interaction
is claimed.

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

The exact tracked root allowlist is enforced by `scripts/repository_layout_test.py`
in existing Checks and release CI. Put new files under the documented directories,
not at root. Intentional new root entries require a reviewed allowlist change and
justification; the sole current release note is derived from the application version.
