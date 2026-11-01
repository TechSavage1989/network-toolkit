# v1.17.146 Beta 10 - published and verified

Published 2026-11-01T02:23:08Z as opt-in GitHub prerelease ID 430315450.
Release: https://github.com/TechSavage1989/network-toolkit/releases/tag/v1.17.146
Tagged source/merged PR #21: 5debf8f4882608eef4cb27b32bdffd462386cf97.
Exact tagged Release workflow 41264759515: Windows x64 build, JavaScript/Go/release guards, portable/update packaging and publish all succeeded.
Four GitHub-listed assets have SHA-256 digests. Beta feed selects 1.17.146; normal stable releases/latest remains 1.17.136.
The final Beta 10 branch Checks 41263728044 completed with 14 successful job groups and 1 failed browser-editor112 group. The original 390px map motion failure was not resolved: p95 common-row offsets 4.140625, 4.453125, 3.3125 CSS pixels in three cases versus an unchanged 2px threshold, matching Beta 9. No CI waiver, stable promotion or physical Windows/native SFTP interruption acceptance is claimed.
Credentials, saved settings, network permissions, source document integrity, stable Network_Toolkit.exe naming, opt-in beta selection and explicit update install consent are preserved.
No previous tag, published release body or asset was replaced.

## Published asset metadata

| Asset | Size (bytes) | GitHub SHA-256 |
| --- | ---: | --- |
| Network_Toolkit.exe | 21111808 | f1ad127f29abeb91e55a66970d7402e9c232f1e20968546e83cab55d0855c492 |
| Network_Toolkit_1.17.146_Portable.zip | 14489213 | 72f3d2959800cf23c58d3937abd913713194dcaa6b0c7c80fbf794f80be7abe4 |
| Network_Toolkit_1.17.146_Update.zip | 8067443 | 00dcea7f8e049acb4b6f397593b0a954122c94e64a68f08251dee5790319b35a |
| SHA256SUMS | 292 | 4f6b0ec4e36996e00bcefbfbfcdfe03cf12612c31fa41b9ca66435c37dd705e1 |

GitHub-reported sizes and digests above and successful release-package verification are confirmed. Independent local downloading and rehashing of the public release bytes was not performed in this session. Git tag and release target both match 5debf8f4882608eef4cb27b32bdffd462386cf97. The official release body follows the What's New / Additional Details format. No previous published asset was changed.

## Integration and remaining acceptance

The exact prerelease branch Checks run 41263728044 finished 14/15 groups successful; browser-editor112 retained the historic 390px motion failure at the same offsets as Beta 9. Tests for browser, full workflows, security, native Windows Terminal/Editor diagnostics, Go/JS, lifecycle and x64 compilation otherwise passed. The release build/publish workflow 41264759515 passed both jobs. A fresh main Checks run may continue after this publication; this handoff does not claim every subsequent optional check passed.

The source merge was a normal two-parent commit because a parallel main-only Beta 9 publication documentation update conflicted with Beta 10's candidate docs. The newer Beta 9 records were preserved, and Beta 10 app/metadata/notes source was merged without replacing published tags. No device credentials, transfer permissions, stored settings or update consent were changed. Native SFTP interruption, physical Windows multi-hour soak and live networking remain required before stable promotion.

Next scope and eleven-area plan: docs/DEVELOPMENT_ROADMAP.md. Historical Beta 10 candidate record is preserved below.

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
