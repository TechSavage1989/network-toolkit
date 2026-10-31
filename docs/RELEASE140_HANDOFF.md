# Network Toolkit v1.17.140 Beta 4 - published graph and file-browser performance

Published 2026-10-31T18:35:55Z as GitHub release 430114376 (draft=false, prerelease=true).
Release: https://github.com/TechSavage1989/network-toolkit/releases/tag/v1.17.140
Tag/source/merge PR #15: 9dc5a75696d2e816da8ef13409d4e267f490fdcb
Exact source Release run 41229976291: build SUCCESS, publish SUCCESS.

## Delivered

- Graph History range endpoints scan samples without sorting the entire history; filtered samples already chronological do not require re-sorting. Unordered samples retain stable timestamp sorting.
- SFTP/FTP/FTPS client file browser reuses sorted directory listings through repeated selection and keyboard operations, invalidating for a new listing, search, sort key or direction.
- TFTP/SFTP server file explorer reuses sorted/filtered views until a listing or visibility/sort option changes; each file's modified/type sort key is evaluated once per listing.
- Existing credentials, permissions, network diagnostic semantics, packet capture, saved profiles and the stable portable updater are unchanged.

## Exact release checks

Release workflow passed application Go tests, 44 Python tests, 1,534 JavaScript tests with zero failures, Windows x64 compilation, icon and version resources, a 1,285-member portable source bundle and strict binary-only update package checks.
GitHub release records show these four assets:
- Network_Toolkit.exe: 21,098,496 bytes; SHA-256 5341accc8aff69278b95919d84850c28e92a5691db5bc01a3a46f23143f91b76
- Network_Toolkit_1.17.140_Portable.zip: 14,406,343 bytes; SHA-256 bc6b9f4b26d593b6257d78e677540a9214b2820f336dd4fd32f35e97481b9062
- Network_Toolkit_1.17.140_Update.zip: 8,063,502 bytes; SHA-256 b7f3cb87723c98e5a60e7e6dc675a699f7fe7fc7ffd6ed411f0815b07053be1a
- SHA256SUMS: 292 bytes; SHA-256 847a74f8b1a5a43959774b6d4456329f9e232bcb2dd360452cc0643318400616

The tag matches published source; beta appears first in the releases list, while the stable releases/latest stays v1.17.136. The four prior Beta 3 assets retain original IDs and SHA-256 digests. This verification checked release metadata, not a separate independent asset-byte download.

## Unresolved CI and physical-device limits

Core regression/Release checks passed. At this handoff, broader browser and Windows CI jobs were still in progress, so their results must not be considered passing. One duplicate PR Windows CI job reported a cancellation-state mismatch in the existing TestNativeSFTPDirect121InstalledWebView2 test: its trace said "Windows copy cancelled. Original files are unchanged." but the test observed the state "finished". The cause is not established and no fix is claimed. Previous Editor comparison alignment and narrow-window sidebar CI failures remain tracked. This beta did not change native SFTP transfer code or Editor alignment.
No physical Windows end-user performance benchmark or live-device SSH/Telnet/Serial speed test has been completed. Synthetic tests establish deterministic behavior, not universal performance improvements.

This is a documentation-only post-release closure. It changes no tagged source, published executable, release body, checksum or asset.

---

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
