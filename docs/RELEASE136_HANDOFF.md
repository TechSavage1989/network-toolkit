# Network Toolkit 1.17.136 - published stable beta opt-in bridge

Published 2026-10-31T07:04:53Z, stable release ID 429777651,
draft=false, prerelease=false.
https://github.com/TechSavage1989/network-toolkit/releases/tag/v1.17.136
Exact source/tag: 1069e8bbcc1ef97d96ac9d81d93e4d318b341934.
Release workflow 41178922281: build and publish SUCCESS.
GitHub latest stable feed resolves v1.17.136 even after the
subsequent v1.17.137 prerelease was published.

The stable release adds a saved, off-by-default Beta updates option
under App settings > Updates. Stable-only checks still target the
official releases/latest feed. Opted-in checks enumerate official
releases and validate numeric tags, supported stable/prerelease
metadata, canonical asset URLs, sizes and strict checksums. Per-channel
cache validators remain isolated. Downloads and installation always
need the existing explicit approvals and installation guards.
Disabling beta updates does not downgrade an installed beta.
The stable 1.17.136 bridge can be installed by the old 1.17.135
stable-only updater, enabling subsequent opt-in beta updates.

Published GitHub assets and SHA-256 digests:
- Network_Toolkit.exe: 21,091,840 bytes;
  1fa7e9861b6e1547de30fad6061e6eec41c41957a67c75ddb159ae4ae3d736f4
- Network_Toolkit_1.17.136_Portable.zip: 14,364,783 bytes;
  0097262fbea2b32bdcf0cb6efb4848020909bf6f853b0ae9204602249870ce72
- Network_Toolkit_1.17.136_Update.zip: 8,061,554 bytes;
  41dfb54aecbe964dcf2886ac3f78c17e4010554ee9e967d4e589ea5a6f02607d
- SHA256SUMS: 292 bytes;
  a6b7fe3f321b37f390104abb30b3f22fa95f19e6600933cd6f9d1956224d557c

The earlier stable v1.17.135 release and its original four asset
IDs, sizes and digests were retained. The release workflow verified
source and packages; no independent public download rehash was made.
This documentation record does not alter published source or assets.

---

# Network Toolkit 1.17.136 - stable beta opt-in bridge (candidate)

## Purpose
The installed 1.17.135 updater accepts only stable GitHub latest-release
metadata. It cannot see or select prereleases. To support user-requested
in-app beta enrollment without changing already-published 1.17.135 bytes,
publish a compatible stable v1.17.136 bridge first. The first Phase 1 beta
will be a separate v1.17.137 prerelease flagged as GitHub prerelease.
Its numeric tag/version keeps the existing signed/verified PE and strict
updater package format intact.

## Behavior
- App settings > Updates adds a saved Beta updates (opt in) checkbox.
- Stable is the default. The beta option searches the trusted canonical
  GitHub releases list and selects the newest validated numeric version,
  stable or prerelease, while stable-only checks use releases/latest.
- The frontend sends the channel explicitly for beta checks/downloads.
  Backend rejects cross-channel or stale-version downloads, validates
  release metadata/asset URL/size and uses segregated cache entries.
- No automatic download, install or restart. Existing checksum, zip
  allowlist, Windows executable/shortcut, ownership, settings and active
  operation protections are retained. Opt out does not downgrade an
  already-installed beta; a newer stable release is required.
- docs/RELEASE_CHANNEL.txt records stable or beta for the reviewed
  GitHub release publisher, and the app's compiled channel must match.
  Beta releases are published as actual GitHub prereleases, not as the
  GitHub latest stable release.

## Acceptance (pending)
Review test coverage for stable default, beta selection, invalid metadata,
channel switching, cache validator isolation, former empty-body clients,
no automatic installations, and beta opt-out. Run exact Go/JS/Python,
browser/native CI, PE/resource and ZIP validation. Publish stable bridge
to the existing updater feed; verify all public assets and unchanged 135.
Only after bridge validation, build/publish 1.17.137 beta with Phase 1
responsive terminal changes. Record any unresolved historical Windows
SFTP/Editor/PowerShell test failures rather than waiving them.

Not a claim of completed publication or physical Windows testing.
