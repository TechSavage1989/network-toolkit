# Network Toolkit development handoff

## 1.17.69: one final update-install action, publication authorized

Prepared from main 74d4a1103c6887fce793ed8ce24e298526d715b0 after the user
requested removal of the redundant Review install and restart option, then
explicitly said to publish when finished. This authorizes the new 1.17.69 source
and release after validation; no further approval is needed for this build.
The baseline remains published 1.17.68 until publication completes.

app-updater.js has no install-review action. A successful explicit download
automatically changes its still-current notes dialog into the final installation
prompt. Install and restart there is the single installation confirmation.
If that dialog was closed or replaced during download, it is not reopened and
does not replace unrelated work. The staged banner and App settings buttons
also call the final version-bound install action directly without another
dialog. They show the staged version and Save your work reminder.

Explicit download approval, settings flush, fresh staged-version/status checks,
active-operation checks and single-flight installation guards are retained.
No timer or download completion calls install automatically. Failed downloads
stay retryable. Backend installation, staging, recovery and scheduling code are
unchanged, as are the Hosts editor improvements.

Focused UI/controller tests cover direct final confirmation, cancellation then
single-click banner installation, settings installation, repeated clicks,
failed downloads, stale versions, settings-save failures, active operations and
closing a pending download without reopening or installing. Browser regressions
exercise real page DOM transitions and mock install requests at 390/768/1280px,
checking exactly one explicit install POST from dialog/banner/settings. Other
startup/native-timer cases still forbid all download/install calls.

Read the review branch's latest Checks result before publishing the exact tested
source. Verify all four actual official assets, source provenance and an unchanged
released68 check/download/stage after publication. Preserve all older releases,
tags and assets. Update this section and VALIDATION.txt with final results in a
documentation follow-up that must not republish or move the released 69 tag.
Native Windows/WebView2 installation and restart remain unverified here.

## 1.17.68 published: Hosts filter text clipping

Prepared from exact main 99ff403c866953706290e312c425a58429f9567e after the
user reported clipped text in the Hosts Entries status dropdown. The completed
review build on review/hosts-editor-1.17.68 was delivered and the user approved
public source upload and publication on 2026-10-24. This approval covers only
the new 1.17.68 release; preserve all earlier tags, releases and assets. The main
network-toolkit repository remains the only source/update destination.

The 32-pixel selector inherited 9-pixel vertical padding and a 1.5 line height,
leaving only 12 pixels for 16.5-pixel text. hosts-editor.css now supplies compact
padding and explicit line height for the selector and adjacent search field.
The selector keeps its 170-pixel width instead of shrinking to 130 pixels, so
the longest With validation issues label also fits. The 31-pixel find/replace
inputs receive equivalent vertical clearance for the same inherited-padding
problem. No editing, file-write, privilege or updater behavior is changed.

The hosted Hosts browser test now checks the content box against measured
font/line height and all four status labels' widths, with space reserved for
the native arrow. It reproduces the released padding defect as a negative
control. Existing twenty responsive light/dark cases remain; additional
light/dark pages cover device scales 1, 1.25, 1.5 and 2. Toolbar screenshots
show the longest label at each scale. Final review source
9fe18dfff0a3fc37b116807be2601361858a9d6b passed Checks 40158776871, including
all required Go suites, 422 JavaScript tests, 31 Python guards, twenty responsive
cases, four status labels, all four device scales in both themes, existing
client/server checks and all seven native update-timer scenarios. The actual
five-minute poll passed. Inspected normal/light and 150-percent/dark toolbar
screenshots show the complete longest label; the 390-pixel layout is readable.
The downloaded review artifact 12584858650 matches the verified local portable
byte-for-byte. Its SHA-256 is
4b7cb87393194bf9188fe49f902148d994601b4cbdc70a73b8708c51af5352a0.
Native Windows/WebView2 remains unverified; browser/DPI results are Chromium
coverage, not Windows runtime.

Version metadata and compile-time Windows resources are 1.17.68. The existing
review-branch workflow supplies a Windows x64 portable source/build artifact.
Local release/resource validation, browser-script syntax, thirty Hosts model
tests and the Windows x64 GUI cross-build passed before source upload.
Published source/tag is 04bffc583445fa48656c40dc44a80af4c54e5edc (tree
e41000868502bd10d1855c05072dcaaea1011da2). It changes only AGENTS.md, this
handoff and VALIDATION.txt from the reviewed runtime source. Release
40159396027 passed and created release 423132212. Main source Checks
40159396034 passed both jobs, including all compact text/DPI checks, existing
client/server and updater startup checks, and all seven native Chromium timer
scenarios. Its real five-minute poll and focus collision passed.

All four actual public release downloads passed their GitHub asset digests,
outer/inner SHA-256 manifests, both ZIP CRCs, the strict four-member updater
allowlist, PE/version checks and standalone/portable/update EXE identity. All
672 packaged source files match the exact published source commit. The official
EXE is byte-for-byte identical to the approved review build. All seven older
release IDs and asset IDs/sizes/digests are unchanged, and all seven older
source-tag SHAs match the prepublication snapshot.

The exact released 1.17.67 source's unchanged production feed handlers checked
the live main feed, verified/downloaded/staged 1.17.68 in 10.76 seconds. The
staged bytes match the independently downloaded official EXE. The harness used
real HTTPS and isolated temporary settings and called only the platform-
independent feed handlers. No installation/restart occurred and its original
executable hash was unchanged. The raw log, baseline snapshots and download
verification record are retained with this publication's local QA files.
Native Windows execution, hosts-file save/elevation, installation and restart
remain unverified. The user's approval is not native runtime confirmation.

Official release: https://github.com/TechSavage1989/network-toolkit/releases/tag/v1.17.68
EXE SHA-256: 5fda37ea59822b2131febe39c685743cbcdbfbac4020cac31e55828d5af597b5
Portable ZIP SHA-256: b81701b207444e8012a6ca153884386d9fe67cd9a6a24b237d62e1e4424b9e30
Update ZIP SHA-256: 0c5837f10f3c6a30ac7070767d80de3624a8edfa7e9478c6ad581bc400e99ee1
SHA256SUMS file SHA-256: db1fb17890f1df0496709d36577abd866d1e75a85523118ee8e8071533f77d01

This final documentation follow-up changes no runtime code and must not move
the released 1.17.68 tag, replace its assets or republish it. Continue from latest
main for future development, preserving the corrected compact Hosts controls.

## 1.17.67 published: expanded Hosts editor

Prepared from exact main 2d071bec99979397e3d80cec6db5d7d4716c4e3e on
review/hosts-editor-1.17.67 in response to the request for more Hosts editor
features. The user approved public source upload and publication on 2026-10-24
after the completed review build was delivered. The published baseline before
this publication is 1.17.66. This approval covers only the new 1.17.67 release;
preserve every existing tag, release and asset. The main repository remains the
only source and update feed. The retired second repository must not be changed.

New code is hosts-editor-model.js, hosts-editor.js and hosts-editor.css. The
existing index delegates Hosts rendering, controls and validation to the module.
All actual file saving still uses reviewLocalConfiguration/applyLocalConfiguration
and the original local-tools hosts request, revision check, elevation helper,
encoding/newline preservation, atomic replacement and automatic backup code.
No backend file-write or privilege behavior is changed.

Features: Entries/Text views; add/edit mappings with aliases/comments; individual
and bulk enable/disable/delete; text/status filters; duplicate/conflict/invalid
line diagnostics; undo/redo; literal find/replace with case/whole-host options;
line navigation/wrapping; format/sort/deduplicate previews; import/restore or
append UTF-8/UTF-16/Windows-1252 text; draft/loaded-copy UTF-8 exports. Ordinary
comments and unrelated lines are preserved. Exact deduplication retains distinct
comments, alias order and disabled copies. IPv4 plus IPv6 for one name is valid.

Drafts, import bytes and bounded undo history stay in memory and are not placed
in saved app state or reports. Entry lists show 100 rows per page; validation
counts all rows and caps displayed issues at 200. Undo history is bounded to
100 checkpoints and approximately 16 MiB of text. The backend retains its 1 MiB
hosts-file limit. Exported text copies are not byte-for-byte original backups;
original automatic backups remain under the existing protected save process.

Local validation passed all 422 JavaScript tests (30 new Hosts model tests),
31 Python packaging tests, root/desktop/terminal Go suites, release/resource
validation, Windows x64 cross-build and portable/update packaging verification.
Hosted Hosts browser interactions passed on fe9189c (Checks 40156261618):
entry edits, aliases/comments, inactive/active/bulk changes, reviewed saving,
operation locks, reload protection, literal find/replace, validation navigation,
UTF-16 backup import, explicit export and paginated selection. Twenty rendered
cases passed at five window sizes in both themes. The first run caught only a
browser-added export extension; exports now use explicit hosts-draft.txt and
hosts-loaded-copy.txt names. Subsequent polish excludes the source textarea
from global one-click input clearing and supplies complete tab semantics.

Final reviewed runtime source 3c379fb1653b1f24de3f820bc5bf525acfaa9b52 passed
Checks 40156697029, including all twenty Hosts rendered cases, existing client
and server layout checks, automatic startup/release notes, and all seven native
Chromium timer scenarios. The real five-minute poll and focus collision passed
with two checks and no overlap. Review portable artifact 12585125922 matched
the local verified build byte-for-byte and passed ZIP/checksum verification.

Published source/tag is c88448fd1b4e2290b5614e35f8485968c5786413 (tree
656a06eb33bbd5cbbbbf19c8d66994789d1516bc). It changes only AGENTS.md,
this handoff and VALIDATION.txt from the reviewed runtime source. Release
40157685254 passed and created release 423122247. Main source Checks
40157685265 passed both jobs, including all twenty Hosts rendered cases,
existing client/server layouts, updater startup and all seven native Chromium
timer scenarios. Its real five-minute poll and focus collision passed.

All four actual public release downloads passed GitHub asset digests, external
and internal SHA-256 manifests, both ZIP CRCs, strict four-member Update.zip
validation, PE/version checks and standalone/ZIP EXE identity. Every one of the
671 packaged source files matches the published source commit. The official
EXE is byte-for-byte identical to the approved review build. All six older
release IDs and their asset IDs/sizes/digests are unchanged; the 63/64/65/66
tags retain their original verified source commits.

The exact released 1.17.66 source's production feed handlers checked the live
main feed, verified/downloaded/staged 1.17.67 in 9.80 seconds. The staged EXE
matches the separately downloaded official EXE. The harness exercised only
the platform-independent feed handlers with real HTTPS and isolated temporary
settings; no installation/restart occurred and its original executable hash
was unchanged. The verification record and raw log are retained with this
publication's local QA files. Native Windows hosts-file save/elevation,
installation/restart and actual runtime remain unverified.

Official release: https://github.com/TechSavage1989/network-toolkit/releases/tag/v1.17.67
EXE SHA-256: eb16ac88a4b92030516df84f4b6b810e9e3e1afcee8101193efbc001926b7192
Portable ZIP SHA-256: 4fb58cd86f01926e1c73b8c5656056ac81106e7da1ddb964a7dc7f677541e3ff
Update ZIP SHA-256: 8b0ead9ee3826abd0f9e7cef609c64be55128f14e1314a1c75a249059af0605d
SHA256SUMS file SHA-256: dff3f03dee787fb22a07f78e8bf68c173f3133cad18d29abce76516b25c3f2c4

This final documentation follow-up changes no runtime code and must not move
the released 1.17.67 tag, replace its assets or republish it. Continue from latest
main for future work, preserving the new Hosts editor and existing updater.


## 1.17.66 published: concise review and faster detection

Prepared from exact main ed078e0bced8a4d0dbc86a7bcaf9532795cc9da6, including
the final 65 documentation. The user approved public source upload, hosted
checks and publication after those checks pass on 2026-10-25. The install
review removes only its long explanatory paragraph and replaces the large
warning with "Save your work before restarting." Notes/version badges, explicit
confirmation, error messages and all active-operation guards remain. Full
installation/recovery details remain in UPDATING.txt. Release notes show complete
highlights only once; Read more contains additional content, retaining complete
text for shortened highlights. Equal installed/release versions use one badge.

Automatic checks now run every five minutes while visible and online. Focus,
visibility and reconnect events share a two-minute freshness gate, one timer
and the controller single-flight guard. Retries and server cooldowns cannot be
accelerated by focus. The root-bound native timers, saved opt-out, 24-hour
reminders and independent startup are preserved. Hidden/offline checks pause;
this is periodic detection, not server push or OS notifications.

Release metadata uses conditional requests and validated cached release details.
GitHub rate and polling headers govern the next allowed check, including manual
checks. Unauthenticated conditional requests still count against the shared-IP
quota. No account credentials, new infrastructure, or notification permission is
introduced. Installer, staging, cleanup and restart behavior are unchanged.

Local validation passed: 392 JavaScript tests, 31 Python tests, root/terminal/
desktop Go tests, focused updater race tests, release/resource validation and
Windows x64 GUI cross-build. Independent review cleared the changes.
Local browser launch was
blocked by the executor sandbox. Hosted review Checks 40149250883 and main
Checks 40149810832 both passed. The inspected 390/1280px screenshots show the
short warning, explicit confirmation, additional-only Read more and single badge
for equal versions. Seven native Chromium scenarios passed without replacing
native timers; the actual periodic check ran after 300,075.68 ms and never
exceeded one in-flight request. Native Windows/WebView2 and real installation
have not been tested here; user runtime confirmation remains pending.

Release 40149810863 published v1.17.66 at source/tag
de0a635eb0a3270c69e3c704beab1ea1001b55af (tree
45547eaa2635ffafa65287338269a827ee6404f6). All four downloaded official assets
passed their hosted SHA-256 values, outer/inner manifests, both ZIP CRCs,
strict four-member updater validation, PE/version validation and EXE identity.
All 665 source files match the reviewed tree. The exact released 65 production
handlers checked the live main feed, verified/downloaded/staged 66 in 11.21
seconds without installing or modifying the original executable. Its raw result
log is retained alongside the verification records. The 65 tag and all its asset IDs
and digests remain unchanged. This final documentation commit changes no runtime
code and must not republish or alter the released 66 tag/assets.

## Published 1.17.65 baseline

Previous released version: **1.17.65**, source/tag
97a0b4c3902e10acc11b1c78715e1ab7ba5de6c4 (tree
445b2b768df738dd5fb9fbaf33e8f2da4e3fe320), built from exact prior public main
f4754d11a9df4d93c7028a1a65b20536921dc9ef. The user approved this narrow timer
fix and publication on 2026-10-25. Review Checks 40147301635 and 40147544940,
main Checks 40147728836 and Release 40147728832 all passed. Publication and
asset verification are complete; see the final record below. The user's native
Windows/WebView2 confirmation of automatic startup in 65 is still pending.

## 1.17.65 native update timers

The production scheduler environment now wraps both setTimeout and clearTimeout
through the Window root. Previously, bare native functions were invoked as
methods of the scheduler environment, giving them the wrong receiver. The
receiver-enforcing integration fixture reproduces the resulting exception after
nextAt is set but before any live timer is registered. The wrapper preserves
injected test clocks while providing the correct receiver to native browsers.

The native browser suite uses the actual index page and native timers, without
page.clock or timer replacement. Its negative control restores only the exact
released 64 timer wiring; the candidate must demonstrate the positive counterpart.
Hosted review Checks 40147301635 passed on bb2fc58. In Chromium 155.0.8059.0,
the exact released wiring throws Illegal invocation at schedule(), makes zero
feed checks and leaves the stale scheduled state. All six native scenarios pass
with the wrappers, unchanged native timer identities and at most one active
feed check. The real retry occurred about 30.02 seconds after failure. Manual
and toggle cancellation were observed past the old retry deadline. This confirms
the native Chromium defect, not a direct inspection of the user's Windows runtime.

Retained unchanged: saved opt-out and startup preference loading, six-hour checks,
30-second/2-minute retries and bounded backoff, manual checks, per-stage/version
24-hour reminders, safe release notes, stable executable naming, settings backup,
all revised 63 features, and explicit download/install consent. No installer,
cleanup permissions, file-storage or restart behavior is modified. The user
independently removed a stale folder to resolve their earlier cleanup warning;
this release does not claim to fix that separate warning.

Local validation passed: 379 JavaScript tests, 31 Python tests, root/terminal/
desktop Go tests, release/resource validation and Windows x64 GUI cross-build.
Independent review found no confirmed must-fix and passed 75 focused JS tests.
The final test scrolls to update status before screenshots: inspected images
show the old stale schedule versus the fixed last attempt and next check. Native
Windows/WebView2, real installation and restart remain untested here. Retain all
old release tags/assets and use the main repository only; do not write to the
retired update repository or change repository access.

## Published 1.17.64 baseline

The current released application is **1.17.64**, based on verified public main
71df414ea97496c26565df12e9e2ac5a0d3c3203. The last published/user-confirmed
baseline is revised 1.17.63, source tag 6bb00ea7224ce1e9e0b4b2db4b806915fc9947aa.
Repository: https://github.com/TechSavage1989/network-toolkit. Preserve README.md.
Release/source tag v1.17.64 resolves to 991f32387c52a35d32f86d13257ab8ef86014b38.
The source tree exactly matches the privately backed-up local build d1e81be.

## 1.17.64 automatic checks and release notes

The actual index initializer starts the updater after saved update preferences
are read, independently of /status, /system and /jobs. The first scheduler task
has zero delay. Local startup/status reads abort after 10 seconds; feed checks
have a 20-second backend deadline and a 25-second frontend deadline. Timed-out
responses cannot overwrite later results. Feed requests ask for cache
revalidation. Automatic checks ignore cached check timestamps and retain the
existing 30-second, 2-minute and bounded six-hour retry cadence.

A failed full-settings merge caused by intervening diagnostic work no longer
blocks reading the saved updater opt-out. Unsaved work remains unchanged, and
the existing full-state writer refuses to persist startup defaults. A missing
or failed preference read never assumes opt-in: retry the local read instead.
The main banner can render even while unrelated platform startup is pending.

The automatic banner opens a safe What's new review before explicit download;
installation still requires separate Install and restart confirmation. The new
app-update-notes.js helper renders four compact highlights, version badges and
keyboard-accessible full notes as escaped, inert text. It executes no release
body HTML or links. Install review uses notes only when the feed release
matches the staged version; otherwise it honestly reports unavailable notes.
The existing 24-hour per-version/stage snooze is preserved and now explicitly
labelled Remind me tomorrow, including the across-reopen behavior in settings.

Verified code gaps fixed here: the unnecessary five-second delay, indefinitely
pending local startup requests and an unrelated settings-edit guard blocking
updater preferences. These do not prove the cause of the earlier Windows
missing-banner report. Adapter/sidebar controls and installer/restart logic
remain unchanged.

Local validation: 377 JavaScript tests, 31 Python tests, root/desktop/terminal
Go suites, release/resource validation and Windows x64 GUI cross-build passed.
The real-index VM integration covers hung startup dependencies, saved opt-out,
preference timeout/retry, late feed replies, navigation/dialog preservation and
no automatic download/install. Independent scoped review passed 73 updater/notes tests and focused Go
updater checks, with no confirmed must-fix issue.

Rendered Chromium tests were extended for immediate startup and safe notes at
390/768/1280px, with keyboard disclosure and screenshots, and added to Checks.
They could not run locally because Chromium IPC creation was sandbox-blocked
and escalation failed environment setup. They subsequently PASSED on hosted
review Checks 40144964705 and main Checks 40145219932. Downloaded screenshots
at all three widths were inspected: readable version badges/highlights, no
card overflow, visible keyboard focus, disclosure and actions. Script/link
security fixtures remained inert. Native Windows/WebView2 remains unverified.

The user explicitly authorized this public source upload and v1.17.64 release
on 2026-10-24 at 23:56:13 UTC. A private complete prepublication backup was
saved before promotion. Review branch CI and screenshots were verified before
main was moved with a checked expected SHA. Release 40145219925 succeeded on
2026-10-25; main Checks 40145219932 also passed. Published63 remains unchanged.
Future publication remains a separate authorized step; main pushes publish new
versions. Use main network-toolkit only, never the retired update feed.

## Previous 1.17.63 baseline

This release starts from exact 1.17.63 commit
8df9b401111e09767591491bc973b4160a17a9bc, retaining its native server adapter
selectors, layout and behavior. Only update-feed routing, release packaging,
version metadata, regression tests and documentation are changed.

## Single public source and release destination

Use network-toolkit for future source and release updates. The owner requested
revised 1.17.63 itself switch feeds and be offered through the old repository
once, so 1.17.62 clients can migrate. Original 1.17.63 was already published on both repositories. The owner
authorized revised 63 replacement and revised source publication on 2026-10-24,
and stated nobody has 1.17.63. Keep the same-version installation caveat for
any copy downloaded earlier. Do not silently overwrite or move existing tags.
Do not delete repositories or change their access/security settings.

An already-installed original 1.17.63 cannot discover a same-version update;
it needs manual EXE replacement. Earlier clients can receive the revised 63
bridge now published on the old feed. No later release uses the second feed.

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
python3 scripts/github_release.py package --tag v1.17.67 --exe build/Network_Toolkit.exe
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
Windows x64 GUI cross-compilation. Independent migration/revision-mode review
passed. Hosted source Checks, including rendered Chromium server/client layout
regressions, and the guarded main release revision completed successfully.
Read VALIDATION.txt and the release record below for verified provenance.

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

The revised **1.17.63** update is now the user-reported working baseline.
On 2026-10-24 at 23:48:27 UTC, the user confirmed: "the update works."
This confirms the reported update experience, not exhaustive native Windows,
shortcut, rollback, cleanup, elevated-tool or feature testing. Keep the remaining
platform limits above. The missing-banner report still has no proven root cause.

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

## Revised 1.17.63 publication record (2026-10-24)

The released application/source tag is
6bb00ea7224ce1e9e0b4b2db4b806915fc9947aa. Main source Checks run 40143510699
passed; main revision Release run 40143608659 passed. Request-only commit
cf38b17aa28402f0b17a5d4eafa326b8fa27c524 also passed Checks 40143608763.
The original main release ID 423021173 was preserved and its actual v1.17.63
tag now resolves to the revised source. GitHub's historical target_commitish
field may retain the original creation target; use the resolved tag for source
provenance, not that informational creation field.

Official release: https://github.com/TechSavage1989/network-toolkit/releases/tag/v1.17.63
EXE SHA-256: b2e9232330a0098134dcd8c41ba8a1fa6df02908f8acb879aab4bf98695b0f18
Update ZIP SHA-256: 6bf8305211166b14841505e5bca1e4a9358051ac47d45060a865a5dd64fac7d7
Source Portable ZIP SHA-256: 2b1d4dbb3a889925a04139c50657cc7ec0fbfc1fa3e28770533853915e455025
All three downloaded official assets passed external manifests, inner checksums,
strict updater allowlist, PE/version and matching-executable verification.
The exact hosted ZIP is the bridge payload; independently repackaging ZIPs can
change their hashes through timestamps even with identical executable bytes.

The main repository's consumed request-branch push trigger has been removed
from its current workflow. Historical request branches and releases are retained;
old-state checks prevent replay. Ordinary main pushes still publish only new
versions. Explicit manual revision mode remains guarded, not blanket overwrite
permission. This final handoff/validation maintenance does not rebuild or change
the already verified application assets or move their source tag.

Legacy bridge completion: old-feed revision run 40143885776 succeeded,
preserving release ID 423040841 and its historical tag. Its two published assets
are the exact same Update.zip hash above and a binary-only checksum manifest.
The archived original 1.17.62 updater fetched the real old feed and successfully
verified/staged the revised 63 EXE hash above. The unchanged revised 63 updater
then checked the real main feed, reported current/latest63 with no false update,
and rejected same-version download without staging or installation. These are
Go/API/feed compatibility checks, not native Windows GUI/restart tests.
The old repository is retained solely for this completed bootstrap and history;
all later updates use the main repository.


## 1.17.64 publication record (2026-10-25)

Release ID: 423054360
URL: https://github.com/TechSavage1989/network-toolkit/releases/tag/v1.17.64
Source/tag: 991f32387c52a35d32f86d13257ab8ef86014b38
Review Checks: 40144964705; main Checks: 40145219932; Release: 40145219925.
All terminal outcomes succeeded. This documentation follow-up does not rebuild,
replace release assets or move the application source tag.

Published EXE SHA-256:
9d0affc356695edd4bd748d25865fac707973fb68c6d2968bc5609137e9ece61
Published source Portable.zip SHA-256:
25a98904fb51e0db7c5340b5dc5770aa111a4b285d012470956a9d4b0b3d02e5
Published Update.zip SHA-256:
712737fbc25f6e3e06bd5364c3305e353b800789ead09246db2e633bf8d9bc32
Use the hosted Update.zip hash, not a local repackaged ZIP hash: timestamps can
change ZIP bytes even when the executable and members are identical.

All downloaded official assets were verified against the hosted outer manifest.
The Update.zip exact four-member allowlist, both ZIP CRCs, every inner manifest,
Windows x64 GUI/version resources and matching EXE bytes in both ZIPs passed.
The actual unchanged revised 63 updater (source 6bb00ea) fetched main release 64,
verified/downloaded/staged the expected EXE successfully in 12.63 seconds.
The installed executable was unchanged; no installation or restart was attempted.
This is production-feed/API/staging compatibility, not native Windows UI proof.

The improved automatic startup and notes interface runs only after installing
1.17.64; it cannot retroactively change the UI of installed 1.17.63. A subsequent
newer release will exercise that UI on the user's actual installation. No user
confirmation of the new 64 UI or native runtime is recorded yet.


## 1.17.65 publication record (2026-10-25)

Release ID: 423068312
URL: https://github.com/TechSavage1989/network-toolkit/releases/tag/v1.17.65
Source/tag: 97a0b4c3902e10acc11b1c78715e1ab7ba5de6c4
Tree: 445b2b768df738dd5fb9fbaf33e8f2da4e3fe320
Initial review Checks: 40147301635. Final review Checks: 40147544940.
Main Checks: 40147728836. Release: 40147728832. All succeeded.

Published EXE SHA-256:
de50ade7bf554e8d6140e2dca802c37deea2ee575e750f5945390404ee271d40
Published source Portable.zip SHA-256:
671d639e605d6ee745203f92d647ddacbb7d6d8e873c9d790ee8080155d5a3c6
Published Update.zip SHA-256:
6ba4c8b462b1026571e60744b1085216cb6d313db03e4c4c769f168a1e8fd47d

Downloaded official files passed every outer and inner manifest, both ZIP CRCs,
strict four-member Update.zip allowlist, Windows x64 GUI/version checks and exact
EXE identity across all packages. All 663 published source files match the exact
reviewed tree. The hosted EXE matches the privately backed-up local build. Use
the hosted ZIP hashes above, not a separately generated timestamped updater ZIP.
Release 64 ID, tag and all asset digests were verified unchanged after publication.

The unchanged released 64 production updater, sourced from the verified 64
Portable.zip, fetched the real main feed, verified/downloaded/staged the exact
65 EXE in 11.67 seconds. No installation/restart ran, and the original running
executable was unchanged. This proves feed and staging compatibility, not a
native Windows install. Full native UI/install/restart remains unverified here.

The user was told to use Check for updates once from installed 64, install 65,
then reopen and allow an automatic attempt without pressing Check. Only 65 has
the timer fix; publishing it cannot repair the already-running 64 scheduler.
User confirmation of that native automatic attempt is pending. Do not claim it
based on the hosted browser proof. The separate stale-folder cleanup warning
was resolved independently by the user and is not changed by this release.

This documentation-only follow-up does not change application code, rebuild the
released EXE, replace published assets, or move the source tag. Private complete
prepublication and verified-published backups preserve source, build assets,
validation and handoff. Continue using main network-toolkit only.
