# Network Toolkit v1.17.144 Beta 8 - publication verified

Published 2026-10-31T22:56:54Z, GitHub prerelease ID 430235856.
GitHub release:
https://github.com/TechSavage1989/network-toolkit/releases/tag/v1.17.144
Source tag/PR #19 merge: 866b64ef49cfd2ef40d7b4c8a5fa2aa24f1f3397.
Release workflow 41250238391: build SUCCESS and publish SUCCESS.

## Actual release scope

- Preserved the original eleven-area roadmap, recorded in the permanent
  docs/DEVELOPMENT_ROADMAP.md, linked from AGENTS.md, README.md and
  docs/DEVELOPMENT_HANDOFF.md for all future chats.
- Shared diagnostic, discovery and results tables derive one column value
  per sorted row and reuse one filtered/sorted row set per initial paint
  or refresh; original ordering, pagination, sort direction and labels
  remain unchanged.
- Empty-query discovery skips unnecessary search-field preparation;
  nonempty IP, MAC and text searches retain exact original behavior.
- Long Ping/Monitor histories use bounded summary reductions, avoiding
  excessive Math.min/Math.max argument arrays.
- Report HTML uses fewer temporary collections and retains byte-identical
  output in synthetic 3/250/1500-host regression tests.
- Application status chrome avoids repeated unchanged accessibility
  property and status DOM writes.

## Validation and published assets

Exact tagged source successfully passed the Release workflow's Go
application/Terminal lifecycle tests, JavaScript regression suite,
Python publication guards, Windows x64 resource generator and GUI
compile, portable source ZIP package, binary-only updater ZIP policy,
and publication verification. Local JavaScript tests: 1,555 passed.
Local Python release tests: 44 passed. Local synthetic benchmark
evidence and per-area deferred work: docs/BETA8_APPWIDE_AUDIT.md.

GitHub-reported release assets:
- Network_Toolkit.exe: 21,110,272 bytes
  SHA-256 1f0cff429eba6dd5c6f4f3c61dd1fccec9d481fb705404da072407a0805e6c7e
- Network_Toolkit_1.17.144_Portable.zip: 14,462,030 bytes
  SHA-256 ff5244886a1986e59c449881b19a4adbe3ac66e3a202812f88c6afa6598a5caf
- Network_Toolkit_1.17.144_Update.zip: 8,066,978 bytes
  SHA-256 bc39aabbf75b3b076300f2f91d6a66566f019093e1b5cb01ddead05075cf2ebc
- SHA256SUMS: 292 bytes
  SHA-256 10a771348d30a79d167d90885c6b288d17bdf875a8b93a1e9c43f6df941c57fe

The release target, Git tag and merged source all agree. The official
release list includes v1.17.144 as the latest opt-in beta; stable
releases/latest remains v1.17.136. Hashes above are GitHub metadata
and are not an independent public-byte download and rehash.

## CI qualifications and remaining roadmap

PR #19 Checks run 41250019191 had 10 of 14 job groups passed and
four browser/Windows integration groups in progress at last check.
Do not relabel those pending jobs as passing. The known Editor 390px
comparison-map motion offset (transient 3-5 CSS px over its original
2px threshold) is retained from earlier release CI. No Editor source
fix was included in Beta 8; it is not claimed resolved. Historical
native SFTP cancellation and multi-hour physical Windows/live-device
stress acceptance also remain outside these results.

A successful release workflow is not evidence that every optional
browser test has finished. This is an opt-in beta, not stable.

## New chat and subsequent work

Read AGENTS.md, docs/DEVELOPMENT_ROADMAP.md,
docs/DEVELOPMENT_HANDOFF.md, this handoff and
docs/BETA8_APPWIDE_AUDIT.md. Check live GitHub main/release/PR/CI
state before modifying anything. Continue the full eleven-area
roadmap, not only the Editor. Next proposed Beta 9 (v1.17.145)
focuses on extended simultaneous workloads, memory/CPU instrumentation,
cancel/recover, cross-tool UI behavior, native Windows and device tests.
Version may change if evidence requires further beta releases.

This is a documentation-only post-publication closure. The tagged
source, executable and all published assets remain unchanged.

---

# Network Toolkit v1.17.144 Beta 8 - application-wide performance and polishing

Status: DEVELOPMENT CANDIDATE. NOT PUBLISHED.
Starting point: v1.17.143 Beta 7, published at cdef07de894b68b472f083bf3082fb905c7ffa9e, plus documentation-only main commit ea6ba1ef369de0a344114c0d1306895181748a99.
Branch: beta/1.17.144-application-wide-performance.
Authoritative continuing plan: docs/DEVELOPMENT_ROADMAP.md.

This beta reviews every one of the eleven roadmap areas and implements measurable changes across multiple tools. It is not an Editor-only or Terminal-only beta. Record measured improvements, regression results, code paths and deferred work here before publication.

Preserve settings, credentials, scan/network permissions, terminal transports, capture accuracy, report data, stable Windows EXE name, opt-in beta updates, all older releases and their checksums.
Run full JS/Go/Python, browser/native Windows, security, resource, build, packaging and feed checks. Disclose any remaining Editor 390px motion and native SFTP cancellation regressions. No physical Windows or live-device certification is claimed without testing.

## Implementation and measured evidence

The full eleven-area disposition and reproducible before/after measurements
are in docs/BETA8_APPWIDE_AUDIT.md. This beta changes shared result list
sorting/refresh, network discovery empty-query work, Ping/Monitor summary
aggregation, report export and application status chrome. Other tools remain
in the phase plan with a documented deferred acceptance status.

Code changed: web/index.html, web/assets/result-filters.js,
web/assets/report-export.js, web/assets/app-chrome.js,
scripts/performance-beta144-appwide.test.cjs, version metadata and
Windows COFF resources. Source changes are kept small to avoid a broad
visual or network-transport redesign.

Local complete JavaScript suite: 1,555 passed, zero failed.
Local Python resource/release guards: 44 passed, zero failed.
Windows resource generation: 12 resources; verified source/binary SHA.
Local synthetic measurements: five-thousand-row sorting about 350ms
to 311ms median; unfiltered ten-thousand-row discovery 11.8ms to
0.19ms; fifteen-hundred-host HTML report 57.4ms to 54.3ms
with byte-identical output. Counts: one derived sort key per row;
one list row-set per initial render/refresh; no redundant ARIA writes.
These are code-local observations, not physical Windows benchmarks.

Known risk register retained: narrow Editor comparison-map transient
3-5px movement over its existing 2px expectation, historical interrupted
SFTP cancellation checks, and no sustained real Windows/network-device soak.
Beta 9 stress testing and stable promotion remain separate.

## Continuation after this beta

Keep docs/DEVELOPMENT_ROADMAP.md as the authoritative roadmap. After
publication, prepend a verified release closure here and in
docs/DEVELOPMENT_HANDOFF.md and AGENTS.md, update the README,
mark v1.17.144 published in the roadmap, and retain the full Beta 9
concurrent workload matrix for the next chat.

PUBLICATION PENDING.
