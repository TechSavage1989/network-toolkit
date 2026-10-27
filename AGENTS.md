# Network Toolkit development

Read docs/DEVELOPMENT_HANDOFF.md before editing or building. Start from latest main
and preserve working features, settings and previous release safeguards.

Current verified published version: 1.17.96.
Source/tag: 8c5e64e4b2bdab0aa9219650c72eeb58d77bb374
Tree: aee336e0f9d5ef6a00ea25008abd5bd09cdcbf78
Full Checks 40446884740 and Release 40448315958 pass on this source.
647 JavaScript tests, 35 release guards and the required Go/native Windows/browser
checks pass. UI96: 3 groups / 48 base, 24 selected and 12 packet layouts, with
48 time-axis cases. UI95/94/93/92/91/90, Local Network/UI88 and covered regressions
also pass, including all 7 native updater timer cases and the real five-minute poll.
Review/local/public EXE and portable files match; all 796 packaged source files,
53 historic notes and 14 editor-library digests verify. All 35 older releases/assets
and 35 older tags are unchanged. Released95 verified/staged96 in 9.66s.
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

The exact tracked root allowlist is enforced by `scripts/repository_layout_test.py`
in existing Checks and release CI. Put new files under the documented directories,
not at root. Intentional new root entries require a reviewed allowlist change and
justification; the sole current release note is derived from the application version.
