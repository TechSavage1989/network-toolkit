# Network Toolkit development

Read docs/DEVELOPMENT_HANDOFF.md before editing or building. Start from latest main
and preserve working features, settings and previous release safeguards.

Current published version: 1.17.109. The authorized vertical-space layout batch
is published and independently verified. Source/tag:
d3494a879604fb2523e0867ad1fb175a2f9abbf5; tree
9dda3650d5c794a4fc1c734976619819ce87fbfa. All six exact-source Checks and automatic
Release passed; actual normal/narrow/native editor evidence, four public assets,
992 exact source files, reviewed package/binary identity, PE icons and latest feed
were verified. Read docs/RELEASE109_HANDOFF.md and DEVELOPMENT_HANDOFF.md for
measured gains, limits and duplicate-main closure. This docs-only handoff is not
a new build. Preserve every published tag/asset and all existing safeguards.
Post-release graph-outline flicker and unsupported editor tab dragging are open
follow-up issues; see the109 handoff. Do not treat synthetic checks as user-runtime
acceptance or mix a later fix into the immutable109 source/tag/assets.

Previous published version: 1.17.108. The authorized activity/bookmark/graph/editor
batch is published and independently verified. Source/tag:
59a82b13bb31899206d383ea58166ec9a4f4faf0; tree
c2e0b5db0b51ef4501bf4e72d5ccd61db671483e. All six exact-source Checks and automatic
Release passed; actual hosted editor windows/close consent, normal/narrow pixels,
four public assets,981 exact source files, reviewed binary/package identity and
latest feed verified. Read docs/RELEASE108_HANDOFF.md and
DEVELOPMENT_HANDOFF.md for evidence, limits and duplicate-main closure. This
handoff-only change is not a new build. Preserve all published tags/assets,
repository access and existing credential/native/host-key/pager safeguards.

Previous published version: 1.17.107. The authorized discovery/appearance/capture/
file-workflow batch is published and independently verified. Source/tag:
bafabd88f9dd728acf45862226995e654821082d; tree
68f7aafdec7652507134ef1ae358bb96ad8298b7. All six exact-source Checks and automatic
Release passed; actual hosted Explorer/OLE, new-feature/legacy pixels, four public
assets, 937 source files, reviewed binary/package identity and latest feed verified.
Read docs/RELEASE107_HANDOFF.md and docs/WORKFLOWS107.md for scope, compatibility
limits and duplicate-main closure. This later [skip ci] handoff is not a new build.
Keep every published release immutable; this record authorizes no new feature.

Previous published version: 1.17.106. The authorized transfer-location/listener
copy controls and macro Library/recording fixes are published and independently
verified. Source/tag: b8be5353877911fa422e69880f6b7363cda47509;
tree da3fd8490a74aa5d80be4b6708b98e3bedeabc26. All five exact-source required
Checks and automatic Release passed. Four public assets, 903 source files,
reviewed EXE/portable identity and latest update feed verified. Read
RELEASE106_HANDOFF.md and DEVELOPMENT_HANDOFF.md under docs/ for full evidence,
limits and duplicate-main closure. This handoff-only commit is not another build.

Published 105 banner/discovery fixes and 104 vault, pager/defaults/archives, graph
and canonical icon safeguards remain intact. Read docs/CREDENTIAL_SECURITY.md,
docs/SECURITY104_HANDOFF.md and docs/CONDENSED_GRAPH_SCALING.md as applicable.
No new feature/release or live user devices/credentials access is authorized by
this historical handoff. Keep every published tag/asset immutable.

The user expressly chose master-password encryption with Windows-account
remembered unlock across restarts and reboots after same-user-risk disclosure.
Remember stores a DPAPI-protected random key, not the password. Established
sessions stay connected on manual Lock and forget; new saved-credential uses
are blocked. Recovery is optional and migration is explicit native setup.
The retained old DPAPI vault requires separate user-confirmed cleanup. No actual
user vault/settings access or migration is authorized by this development record.

The prior 104 reviewed/published source is 554f360613aa960a28ee459a44018da39a0bbcff.
This handoff-only closure does not replace published assets or authorize a new
feature/release. Keep future work scoped, preserve explicit settings and existing
release safeguards, and coordinate concurrent files before editing.

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
