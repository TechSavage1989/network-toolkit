# Network Toolkit v1.17.140 Beta 4 - application-wide performance candidate

Based on the published v1.17.139 Beta 3 and latest main documentation closure.
This candidate advances performance beyond Terminal: graph sample selection,
SFTP/FTP directory browsing and TFTP/SFTP server file explorer.

## Changes

- Graph time-range computation scans for earliest/latest without a full sort.
  Chronologically ordered selected data avoids an unnecessary sort; unordered
  data still sorts with the original comparator. Recorded data is not mutated.
- SFTP and FTP/FTPS file browser caches the sorted listing for repeated
  selection and keyboard actions. Replacement listings and sort/filter
  preference changes invalidate the cache.
- Server file explorer caches ordered/filtered directory views, including
  hidden-file and sort-option changes; dates and file types are precomputed
  for sort comparisons.
- Added focused list-cache and graph range regression tests. Source audit
  and future cross-tool work are documented in docs/PERFORMANCE_AUDIT_140.md.

## Safeguards and release status

No changes to connection authentication, network scan concurrency, packet
capture settings, server transfer permissions, files, persistent profiles,
terminal transports or updater/install confirmations.
Prior historical Editor comparison/390px sidebar CI failures are tracked
as separate, not fixed by these changes.
This candidate is not yet verified on physical Windows, and does not
establish universal measured speedups. Do not publish or change existing
release assets until exact-source checks, package and release guard verification.

PUBLICATION PENDING.

## Versioned candidate status

The Go, web, native Windows version resources, root release notes and README now identify 1.17.140. The original v1.17.139 root release notes are archived without editing. Full exact-source CI, Windows build and publication remain pending. No live-device performance measurements have been performed.
