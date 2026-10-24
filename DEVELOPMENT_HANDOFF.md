# Network Toolkit development handoff

## Start here

The current application source is **1.17.63** in the private repository
https://github.com/TechSavage1989/network-toolkit on `main`. Fetch the latest
`main` and read this file before continuing. Do not restart from an old 1.17.21
ZIP or overwrite newer source with earlier chat attachments. Preserve the
owner's existing `README.md`; the detailed application manual is `README.txt`.

This release starts from the verified 1.17.62 private-main snapshot at
c88a05b41dfe707f730c0bfddfa2442cda365f3a. Server Listen on adapter now uses
the same native select and shared control appearance as SFTP/FTP client
connection sources. The native popup does not expand the settings panel.
Existing listening addresses, running-server locking, saved settings and
adapter-left/port-right layout are retained. No updater transport, installation,
credential or SSH/SFTP session lifecycle changes are included.

The Checks workflow now runs rendered transfer server and client regressions
in an isolated Chromium job. Browser test dependencies remain outside the app.
Read VALIDATION.txt for completed checks and platform limits.

## Private source, public binaries

- Keep source, tests, vendored dependencies, build resources and development
  documentation in this **private** repository.
- https://github.com/TechSavage1989/network-toolkit-releases is the separate
  **public binary-only** repository and updater feed. Never upload source,
  private portable archives, development ZIPs or repository credentials there.
- Existing Checks/Release workflows remain private-repository workflows. A
  `main` push validates and may create a private release when that version is
  not already published. They do not publish to the public feed. No new
  credentials or security changes are needed for this handoff.

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

Private packaging:

```sh
python3 scripts/github_release.py package --tag v1.17.63 --exe build/Network_Toolkit.exe
```

Use the tracked checkout for release packaging; `--allow-untracked-source` is
only for a consciously reviewed local snapshot. Public packaging is separate:

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
versions. The private portable package uses the stable EXE name and is not the
public updater payload. Validate outer ZIP checksum, inner executable checksum,
PE x64 GUI resources, version and exact member allowlist. Checksums provide
integrity, not publisher authentication; releases are not Authenticode signed.
Automatic checks/reminders do not authorize automatic download or installation.

## Validation status and unresolved work

1.17.63 verification is in progress. The required local Go suites, existing
355 JavaScript checks, three new adapter-to-startup regressions and fourteen
Python checks have passed. Hosted rendered checks and final packaging are
pending; update this section with their terminal results before release.

1.17.62 passed focused/root/desktop/terminal Go checks, updater race checks,
Windows executable and test cross-compilation, 14 Python tests and 355 JS tests
when packaged. See `VALIDATION.txt` for precise limits. Some full `go test ./...`
network/interface tests need privileges unavailable in the Linux sandbox.
Native Windows/WebView2, real shortcut COM APIs, mapped-image replacement,
canonical reopen and elevated helper identity were not executed here.

The last user-confirmed working Windows version was **1.17.60**; user confirmation
of **1.17.62 and 1.17.63 is pending**. Do not describe cross-compilation as Windows runtime
verification. First useful next step is a real-device 1.17.60-to-1.17.62 update,
checking visible startup, stable EXE path, shortcut preservation, retained
settings, one rollback copy and safe cleanup/recovery.

The reported missing automatic update banner has **no proven root cause**.
Manual checking succeeding does not establish the original cause. Improvements
wait for saved preferences independently of unrelated system/job requests,
retry early transient failures and display automatic-check status. Reproduce
on Windows and record preferences, last/next check state and timings before
claiming the original issue is fixed. Never log credentials or private results.

## Continuing development

1. Start from latest private `main`, inspect current changes and the user's next
   requested improvement, and retain existing unrelated work.
2. Make focused changes with regression tests. Increment the version for each
   new distributable build; keep Go/UI/package metadata, README/release notes
   and Windows resources consistent.
3. Run the checks above and applicable Windows runtime tests. Record honest
   validation limits and user-confirmed results.
4. Sync source/tests/build assets and updated handoff to the private repository.
   Inspect the remote first; preserve unrelated work and never force-overwrite.
5. Build the separate binary-only public package, verify hashes and allowlist,
   then publish the authorized new version to the public release repository.
   Never replace an existing release silently or leak private source.
6. Verify remote commit, workflow terminal outcomes, release tag/assets/hashes
   and updater metadata. Share a private-source link and public-binary link.
   Future source syncing accompanies user-requested builds, not unsolicited
   development changes.
