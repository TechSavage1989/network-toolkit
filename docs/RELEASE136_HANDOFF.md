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
