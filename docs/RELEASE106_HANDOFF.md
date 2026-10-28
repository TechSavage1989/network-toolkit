# 1.17.106 candidate — transfer locations and macro controls

The user explicitly approved the complete batch for implementation, build and
publication on 2026-10-27. Baseline is verified 105 closure main
30ecbf2a992beef6f0045272db3cc36acd0b27bb / tree 458ec195bc5b4c9595fc292267e41be78cb07ca3.
Published 105 source/tag 1d716c1916833d9fb14dee44016bf0f75e568305 and assets remain
immutable. No source access policy, old release or retired feed is changed.

## Supplied image and current copy gaps

The supplied screenshot's actual pixels show Shared files under local C:\TFTP,
with AOS-CX_6100-6000_10_18_0001.swi and AOS-CX_6200_10_18_1010.swi. Row actions
are Rename/move, Download and Delete; no network location or Copy is present.
The listener address is outside this crop and was not inferred from the image.
Source inspection confirms root-disk-path, fingerprint and activity copies exist,
but listener text lacks Copy and remote-client Copy full path omits the host.

Supported servers are TFTP, SFTP, FTP/plain and explicit FTPS. Supported clients
are TFTP send/receive (not directory browsing), SFTP, and FTP/plain/explicit or
implicit FTPS. This work does not invent an HTTP file server or new transport.

## Copy contract

- Distinct labels for network host/path, server-relative or remote path, and
  local disk path; copy listener address/endpoint and relevant existing client
  destination data. Preserve actual ports and bracket IPv6 endpoints. Use protocol-appropriate
  URL path/filename encoding only for labeled URLs, not literal paths.
- Wildcard binds require an explicit concrete active-interface choice, never a
  remotely reachable 0.0.0.0/:: claim. Loopback is local-only; stopped/configured
  state remains clear. Active-interface presence does not prove routability.
- Copy values contain no credential, password, token or private-key data. Use
  narrow explicit controls rather than weakening generic sensitive/interactive
  copy exclusions. A copy gesture must not transfer, download, launch, start or
  stop a server, execute a macro or contact a user device.
- Snapshot the selected item/destination during asynchronous choice/copy. Repeated
  rendering, selection changes, clipboard denial and cancel must be safe and
  preserve useful state with honest accessible feedback.

## Macro cause and bounded patch

Starting a macro explicitly called openView('runs') after the start response.
Removing that navigation preserves the user's current Library/Runs selection,
including manual changes while a request is pending, without hiding run history.

Recorder rendering replaced its full innerHTML on every input, poll and terminal
chrome sync. This destroys Pause/Stop nodes and can discard a click between native
mouse-down/up or keyboard activation. Stable controls update only status/labels.
Stop & review previously cleared recording before constructing the review;
retaining captured steps until a real review opens permits visible retry on error.
The user's exact event timing/window type is unconfirmed; test main, separate
Terminal and detached-session modes rather than guessing which they used.

Prepared focused macro/model/detach/tab tests pass 57/57; old-code negative controls
fail both new recorder regressions. The full-app browser fixture is authored but
was not executed locally. All APIs and commands in it are synthetic; pausing only
stops capture, not ordinary input delivery. Manual Runs choice stays respected.

## Required acceptance and publication

Run the final combined JavaScript/Python guards, syntax/resource/metadata checks,
and all five existing exact-source hosted jobs: application/build, dependency,
Windows native, short security/new-feature browser and full browser including the
real five-minute updater timer. Local Go and Chromium execution remain unavailable;
do not mislabel unrun stages. Inspect actual normal/narrow clipboard and macro
recording/review pixels, not just assertion text.

Review the exact-source portable archive, manifests, safe inventory and x64 GUI
PE/version before expected-head non-force promotion. After automatic Release,
independently download all four public assets and verify hashes/sizes/manifests,
all tracked packaged source bytes, reviewed EXE/portable identity, updater
allowlist, latest feed/tag and unchanged 105 metadata. Observe the automatic main
Checks to terminal, then publish docs-only [skip ci] closure without rebuilding.

Current state: implementation/integration and final acceptance are in progress.
This candidate note is not a passed hosted gate or published 106 claim. No live
user credentials/settings, switch, installation/reboot or device-fleet testing.

## Integrated local checkpoint before hosted acceptance

All requested copy and macro changes are integrated on the preserved 105 closure.
Final local JavaScript aggregate passes 809/809; Python release/layout guards
pass 42/42. Metadata/resource validation reports 1.17.106, changed JavaScript and
browser fixture syntax plus whitespace checks pass. The prior root 105 release
note was moved unchanged into docs/releases/; the canonical icon is unchanged.
Local Go and Chromium were not executed; exact-source hosted gates remain required.

Independent focused source review found no remaining blocker after bounded
corrections: remote FTP URLs are omitted rather than guessing login-root versus
absolute-path semantics, TFTP URLs encode the complete filename (including nested
and leading slashes), and unscoped IPv6 link-local addresses are not offered as
usable destinations. Explicit scope IDs are preserved without URL guessing.
Primary format references: RFC 1738 section 3.2.2 and RFC 3617 section 2:
https://www.rfc-editor.org/rfc/rfc1738
https://www.rfc-editor.org/rfc/rfc3617

Allowlisted frozen snapshots contain no credential objects or secret values.
Copying remains a clipboard-only action with existing honest failure handling;
no backend/native launcher/authentication changes are included. Existing generic
copy exclusions, 105 discovery copy actions, vault security and pager archives stay
intact. Main/terminal/detached macro browser and normal/narrow transfer clipboard
fixtures are prepared but still require actual hosted execution and pixel review.
