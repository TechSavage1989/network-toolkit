# Network Toolkit v1.17.146 Beta 10 - editor alignment candidate

Status: DEVELOPMENT CANDIDATE. Not built, tagged or published. Do not claim that this resolves the full hosted Editor regression.

Baseline: published opt-in Beta 9 v1.17.145 at 84c3c53d5c2dcdee13fd04842fafa5c8693679d2. Stable remains v1.17.136.

## Scope

- Use the median correction from three or more visible matched comparison rows when synchronizing the Editor panes. This reduces the effect of one temporarily displaced painted source line during narrow-window scrolling.
- Preserve the original first-row correction when only one or two matched source rows are visible.
- Add a regression test for the paired, multi-row, reversed and no-match cases.
- No document source, undo/redo, selections, credentials, network permissions, transfer behavior, profile preferences, executable naming or updater consent was changed.

## Validation and release gates

- Await exact-source GitHub Checks, including browser-editor112, Go/JS/Python/security and hosted Windows tests.
- Maintain the original 2px 390px map-motion assertion, alignment and sidebar assertions; do not waive failures.
- If the tests pass, update version metadata and release notes, build and publish a new immutable v1.17.146 prerelease through the official workflow; verify all four assets and both update channels.
- No physical Windows multi-hour soak, live-device operation or native SFTP interruption acceptance is claimed by this change.
- Review the full eleven-area development roadmap before subsequent edits. The next beta should continue SFTP, full-app performance, and native Windows acceptance, not become Editor-only.

## User-facing release-note draft, not yet published

What's New:
- Smoother, more consistent file comparison scrolling, especially when windows are narrow.

Additional Details:
- Comparison calibration now uses the median of three or more visible matched source rows and preserves the prior behavior with one or two matches.
- No changes to document contents, stored data, security permissions, portable updater, or network transports.
- The longstanding Editor 390px/browser and native SFTP/physical Windows acceptance gaps are not declared resolved.

## Prepared release source metadata

Version metadata, HTML footer, native Windows version resource source/object,
root beta release notes and README have been prepared for v1.17.146. The
v1.17.145 root note was preserved byte-for-byte in docs/releases. The
actual Windows build, broad Checks, publication, and release verification
must be recorded separately; no physical device acceptance is inferred.

## Exact-source candidate CI review

Beta 10 branch source 7910fded068c44296ad72cc9d61491af78730000, Checks run 41263728044: release metadata, Go lifecycle, full JavaScript suite (1,565 tests), Windows x64 compilation, native Terminal and Editor diagnostics, dependency/browser security and Beta 9 lifecycle regressions passed. The 390px browser-editor112 motion test still fails its original 2px threshold with p95 offsets 4.140625, 4.453125 and 3.3125 CSS pixels in the same three cases as Beta 9; no end-to-end motion fix is claimed. Other hosted browser groups were still running when the release was prepared. This is an opt-in prerelease candidate, not stable certification. A parallel Beta 9 documentation-only main commit caused a PR merge conflict, so the candidate was integrated using a two-parent merge preserving main's newer Beta 9 publication documentation and the Beta 10 application tree. Exact merged-source release workflow validation remains required before publication.
