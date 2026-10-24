# Network Toolkit development handoff

## Start here

The current application source is **1.17.63** in the public repository
https://github.com/TechSavage1989/network-toolkit on `main`. Start from latest
main, not earlier chat ZIPs. Preserve the owner's README.md.

This release starts from exact 1.17.63 commit
8df9b401111e09767591491bc973b4160a17a9bc, retaining its native server adapter
selectors, layout and behavior. Only update-feed routing, release packaging,
version metadata, regression tests and documentation are changed.

## Single public source and release destination

Use network-toolkit for future source and release updates. The owner requested
revised 1.17.63 itself switch feeds and be offered through the old repository
once, so 1.17.62 clients can migrate. Original 1.17.63 was already published on both repositories. The owner
authorized revised63 replacement and revised source publication on 2026-10-24,
and stated nobody has 1.17.63. Keep the same-version installation caveat for
any copy downloaded earlier. Do not silently overwrite or move existing tags.
Do not delete repositories or change their access/security settings.

An already-installed original 1.17.63 cannot discover a same-version update;
it needs manual EXE replacement. Earlier clients can receive the revised63
bridge once its replacement publication is verified. No later release uses the second feed.

The existing main-push Release workflow publishes a new version once, with
standalone EXE, source Portable.zip, binary-only Update.zip and combined
SHA256SUMS. The updater accepts only the new repository's exact asset URLs.
The strict Update.zip has four members and the same executable bytes as the
standalone asset. Never use Portable.zip for the in-app updater.

## Build and checks

Use Go 1.23.2 as declared in `go.mod`, vendored modules (`-mod=vendor`), Node 24
and Python 3 (local validation used 3.12; CI selects 3.13). Windows x64 is the
shipping target. The application requires WebView2 for its Windows desktop UI.

From the repository root:

```sh
python3 scripts/github_release.py validate
python3 -m unittest discover -s scripts -p '*_test.py' -v
go test -mod=vendor . ./internal/terminal ./internal/desktop
node --test scripts/*.test.cjs
python3 scripts/build_windows_resources.py
sh ./build.sh
```

On Windows, `BUILD_WINDOWS.cmd` builds the same GUI executable:
`build/Network_Toolkit.exe`. `Open_Terminal.cmd` opens its terminal mode.
Keep `vendor/`, bundled web assets, the vendor database and the compile-time
`resource_windows_amd64.syso`; regenerate resources after version changes.
The Go build uses CGO_ENABLED=0, GOOS=windows, GOARCH=amd64, GOTOOLCHAIN=local,
`-buildvcs=false -trimpath -ldflags='-s -w -H windowsgui'`.

Complete release packaging:

```sh
python3 scripts/github_release.py package --tag v1.17.63 --exe build/Network_Toolkit.exe
```

Use the tracked checkout for release packaging; `--allow-untracked-source` is
only for a consciously reviewed local snapshot. Binary-only packaging is also available:

```sh
python3 scripts/package_public_update.py --exe build/Network_Toolkit.exe --output-dir public-dist
```

Do not commit generated EXEs, archives, build caches or checksum inventories.

## Updater and installation invariants

Official executable names normalize once to `Network_Toolkit.exe`; custom
names and existing stable-name collisions are preserved. Future updates retain
that stable launch path. Settings are retained. Updater journals, staging,
helpers, logs and the single immediately previous rollback executable belong
under the settings directory, separated by installation identity. Do not add
permanent sidecars beside the EXE. Brief atomic transaction/recovery files may
remain until verified cleanup is safe, especially across drives or with locks.

Migration requires committed installation, old helper exit and a genuinely
healthy canonical reopen. Cleanup is hash- and journal-verified, not a broad
filename deletion. Exactly matching shortcuts in the current user's Desktop
and Start Menu can be retargeted with original backups in settings; pinned,
shared and other-location shortcuts are not migrated. Preserve custom fields,
icons, arguments and unrelated or changed files. Relevant implementation and
regressions are `updater_*`, `lifecycle_*`, `terminal_views_lifecycle.go` and
`internal/desktop/startup*`.

The public feed uses GitHub Releases `/releases/latest`, stable `vX.Y.Z` tags,
`Network_Toolkit_X.Y.Z_Update.zip` and an external `SHA256SUMS`. The update ZIP
must contain exactly four members under `Network_Toolkit_X.Y.Z/`:

1. `Network_Toolkit_X.Y.Z_Windows_x64.exe`
2. `UPDATING.txt`
3. `THIRD_PARTY_NOTICES.txt`
4. `SHA256SUMS` (hashes the other three members)

The versioned filename **inside this ZIP is deliberate compatibility**, even
though the installed/direct-download executable is `Network_Toolkit.exe`.
Do not rename the ZIP member without testing all supported older updater
versions. The source-containing portable package uses the stable EXE name and is not the
public updater payload. Validate outer ZIP checksum, inner executable checksum,
PE x64 GUI resources, version and exact member allowlist. Checksums provide
integrity, not publisher authentication; releases are not Authenticode signed.
Automatic checks/reminders do not authorize automatic download or installation.

## Validation status and unresolved work

Revised 1.17.63 currently passes 359 JavaScript checks, 31 Python checks,
release metadata/resource validation, root/desktop/terminal Go suites and
Windows x64 GUI cross-compilation. Independent migration/revision-mode review passed. Hosted CI and remote
publication verification remain pending. Read VALIDATION.txt for the latest verified status.

1.17.63 passed the required root/desktop/terminal Go suites, all 358 JavaScript
checks, fourteen Python checks, release/resource validation and Windows x64 GUI
cross-compilation. The then-private Checks workflow also passed rendered server and
client regressions with real Chromium: native selector/client style parity,
adapter/port alignment, mouse and keyboard selection, saved-address reload,
activity resizing, sidebar docking and five window sizes in both themes.
The public ZIP was checked against its exact four-member allowlist and both
internal and external SHA-256 checksums. Native Windows/WebView2 execution
remains unverified; this does not establish Windows updater runtime behavior.

1.17.62 passed focused/root/desktop/terminal Go checks, updater race checks,
Windows executable and test cross-compilation, 14 Python tests and 355 JS tests
when packaged. See `VALIDATION.txt` for precise limits. Some full `go test ./...`
network/interface tests need privileges unavailable in the Linux sandbox.
Native Windows/WebView2, real shortcut COM APIs, mapped-image replacement,
canonical reopen and elevated helper identity were not executed here.

The last user-confirmed working Windows version was **1.17.60**; user confirmation
of **1.17.62 and 1.17.63 is pending**. Do not describe cross-compilation as Windows runtime
verification. First useful next step is a real-device bootstrap to 1.17.63 from the installed version,
checking visible startup, stable EXE path, shortcut preservation, retained
settings, one rollback copy and safe cleanup/recovery.

The reported missing automatic update banner has **no proven root cause**.
Manual checking succeeding does not establish the original cause. Improvements
wait for saved preferences independently of unrelated system/job requests,
retry early transient failures and display automatic-check status. Reproduce
on Windows and record preferences, last/next check state and timings before
claiming the original issue is fixed. Never log credentials or private results.

## Continuing development

1. Start from latest `main`, inspect current changes and the user's next
   requested improvement, and retain existing unrelated work.
2. Make focused changes with regression tests. Increment the version for each
   new distributable build; keep Go/UI/package metadata, README/release notes
   and Windows resources consistent.
3. Run the checks above and applicable Windows runtime tests. Record honest
   validation limits and user-confirmed results.
4. Update this handoff whenever implementation, release routing, validation,
   known issues or next steps change. Keep it current so new development chats
   do not inherit stale assumptions. AGENTS.md points new chats here.
5. Sync authorized source/tests/build assets and handoff to network-toolkit.
   Inspect remote HEAD first; preserve unrelated work and never force-overwrite.
   Its release workflow builds all four assets together for the new version.
   Do not publish future builds to the second repository beyond any explicitly
   authorized migration exception, and never overwrite an old release.
6. Verify remote commit, terminal workflow outcomes, release tag/assets/hashes
   and updater metadata. Share the main repository release link and describe
   any one-time bootstrap requirement. Do not claim unperformed runtime tests.

## Explicit same-version repair and recovery

Ordinary main pushes never replace an existing release. A separately approved
same-version correction uses Release workflow_dispatch with revise_existing,
exact tag, original tag commit, original SHA256SUMS file digest, and exact new
source commit. It must be dispatched from main after new-source checks pass.
The revision helper is restricted to network-toolkit, not the binary bridge repo.
Before dispatch, retain original assets and metadata in durable private storage;
the job also verifies and saves a 90-day original-release recovery artifact
before replacing any asset. Publishing is blocked on backup failure.

Revised assets are verified locally, uploaded one at a time with SHA256SUMS
last, and rechecked against GitHub digests. Only then is the existing lightweight
tag moved to the tested new commit and the release notes refreshed. The release
record is retained. No repository permission or credential change is needed.

If an upload, verification, tag move or notes edit fails, STOP. Do not rerun the
workflow blindly: its strict original-state guard intentionally rejects partial
replacement. Preserve the failed run, original recovery bundle and built new
artifacts. Read current release ID, every asset name/size/digest and tag commit.
Classify each asset as exact original, exact approved new, missing or unknown.
Unknown bytes, an unexpected tag or extra assets require owner review; do not
overwrite them. Record exactly which actions completed, including uncertain
outcomes. Reconcile read-only before asking for the specific remaining repair.

With appropriate authorization, finish only the missing approved new uploads
(checksums last), or restore exact original asset bytes from the durable backup
and their original manifest. Verify all remote hashes, tag/source provenance
and release metadata before declaring recovery. A tag move or published asset
replacement is never inferred from a successful partial upload. Do not delete
an entire release or repository to recover. Existing clients reject mixed
ZIP/checksum combinations; nevertheless ask users to wait while reconciling.

When manual dispatch is unavailable, the one-time release-revision-v1.17.63
branch is a bounded alternative. Only that exact branch triggers request mode,
with .github/revision-request.json containing tag, old_commit, old_manifest and
new_commit. The first three fields are hard-pinned to the original63 release;
new_commit must equal current main. The request commit must descend from that
source and change only this request JSON. Build detaches the selected source.
Before replacement, both source main and request branch must still match their
recorded commits. Replay after successful replacement fails the original-tag
and original-asset guards. Ordinary main pushes never use request mode.
