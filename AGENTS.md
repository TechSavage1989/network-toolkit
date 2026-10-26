# Network Toolkit development

Read docs/DEVELOPMENT_HANDOFF.md before editing or building. Start from latest main
and preserve working features, settings and previous release safeguards.

Current verified published version: 1.17.90.
Source/tag: 3d6c6da7fe214620adf9d32d776c74166dbe2300
Tree: c5644c74335d55220a954d2ed37e390120b5edf9
Full Checks 40295261490 passed at original runtime source:
730d8eb88233187b49985a54c026e945ba5ca2f5
The final documentation-only merge has identical runtime files and EXE bytes;
31 guards and its final repack inventory verify. Final Release
40298071558 passed on the published source. The official portable
matches the final repack; its archived documentation differs from the original
review ZIP. All 753 source files match the final tag.
513 JavaScript tests, 31 release guards, required Go/native Windows and browser
checks passed on the unchanged original runtime. Shared UI: 4 groups / 8 layouts
/ 5 header groups; existing Local Network 5/12, 88 layout 4/9 and covered
regressions pass. All 47 historical notes and the AGENTS organization policy are
preserved. All
29 older releases/assets and 29 older tags are unchanged.
Released89 verified/staged90 in 9.60s. No actual installation/restart or
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
