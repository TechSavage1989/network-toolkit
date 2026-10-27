# Network Toolkit development

Read docs/DEVELOPMENT_HANDOFF.md before editing or building. Start from latest main
and preserve working features, settings and previous release safeguards.

Current verified review build: 1.17.98, based on main 8a3c876.
Source: 78360788f4f1b426cc9c7d11cfd0988340379422
Tree: 891660297914c5c331dd6b4858af69161c67b857
Full Checks 40468142795 passes application, native Windows and browser jobs.
667 JavaScript tests, 35 safeguards, required Go/native/browser suites and all
7 updater timer cases pass, including the real five-minute poll. Local/GitHub
portable files and embedded EXEs match; 808 source files, 55 historical notes
and 14 editor digests verify. See handoff for evidence and platform limits.
The user authorized implementation/build delivery, not publication. The public
update remains 1.17.97. This final three-file documentation child uses [skip ci].

Current verified published version: 1.17.97.
Source/tag: c57ad0e52da1d40cda8a6d268e4161928a686a61
Tree: e670de6d771d9ee18663960a586b4898176c1fd5
Full Checks 40458096035 and Release 40458095982 pass on this source.
659 JavaScript tests, 35 safeguards and the required Go/native Windows/browser
suites pass, including all 7 updater timer cases and the real five-minute poll.
UI97: 4 groups, 8 scale cases, 72 diagnostic layouts and 17 packet cases;
retained UI96/95/94/93/92 and earlier application regressions also pass.
Prepared/public EXE and publication-candidate/public portable files match.
All 799 packaged source files, 54 historical notes and 14 editor digests verify.
All 36 older releases/assets and 36 older tags are unchanged.
Released96 detected/verified/staged97 in 10.06s; no installation/restart
or complete native WebView2 interaction is claimed. See handoff for evidence.

The user authorized publication and it is complete. Future releases need
authorization. Main network-toolkit is the sole public source/update feed.
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
