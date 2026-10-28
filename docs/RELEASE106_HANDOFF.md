# 1.17.106 — published transfer copies and macro fixes

## 1.17.106 published and independently verified

Published 2026-10-28:
https://github.com/TechSavage1989/network-toolkit/releases/tag/v1.17.106
Release ID: 426132432
Exact reviewed/published source and tag: b8be5353877911fa422e69880f6b7363cda47509
Tree: da3fd8490a74aa5d80be4b6708b98e3bedeabc26
Baseline: 105 closure main 30ecbf2a992beef6f0045272db3cc36acd0b27bb.
User-authorized expected-head non-force promotion followed exact-source acceptance.
This later docs-only closure does not rebuild, move tags or replace public assets.

Completed scope:
- Explicit Copy location controls for TFTP/SFTP/FTP shared server files, server
  listener addresses/endpoints, SFTP/FTP client files and TFTP send/receive setup.
  Network locations, protocol URLs where unambiguous, server-relative/remote paths
  and local disk paths have distinct labels; copies never launch or transfer.
- Wildcard listeners require an explicit concrete active-adapter choice. Loopback
  and configured/stopped states remain clear; unscoped IPv6 link-local destinations
  are rejected rather than guessed. Actual ports and IPv6 brackets are preserved.
  FTP-client/FTPS ambiguous URLs are omitted; TFTP complete filenames, including
  nested/leading slashes and Unicode, round-trip through URL encoding correctly.
- Narrow allowlisted immutable snapshots contain no credentials/tokens. Selection
  or destination changes do not alter an open copy dialog/context action. Clipboard
  denial and failed fallback preserve the previous value and show an honest error.
- Starting macros preserves the user's Library/Runs selection, including manual
  changes during pending requests. Pause/Stop controls retain DOM identity through
  input/poll/chrome updates. Pause stops capture only, not terminal input delivery.
  Failed/missing review construction keeps captured steps for a visible retry.

Required exact-source Checks 40606459967 completed SUCCESS, all five jobs:
- Application 122547061967: Go lifecycle/asset tests, 809 JavaScript tests, 42 Python
  guards, Windows x64 GUI build and reviewed portable package.
- Dependency 122547061816: canonical vendor exact match, zero reachable/imported-
  package vulnerabilities on Linux/Windows; unused/unvendored OpenPGP advisory
  GO-2026-5932 remains disclosed. This is not an independent security audit.
- Security/new-feature browser 122547061479: existing credential/pager/banner/
  discovery regressions plus native clipboard success/denial, all supported copy
  routes, frozen snapshots, keyboard/cancel, and main/separate/detached macro UI.
- Windows 122547061732: existing native vault/DPAPI/bootstrap and transport gates.
- Full browser 122547061674: existing graph, copy, server/client responsive, terminal
  and session regressions; actual five-minute updater timer 301,991 ms and historical
  negative control. No required assertion or real-time gate was bypassed.

Actual normal/narrow transfer copy dialogs, main/separate/detached recorder and
retained review/Library screenshots, existing server/client layouts, native vault,
credential and pager pixels were inspected. New feature reports have errors=[].
The supplied shared-file screenshot was inspected before implementation. User-
reported recorder timing/window is unconfirmed; tests reproduce the identified
control-replacement failure synthetically, not on a real user device.

Release 40608363956 completed SUCCESS (build 122553457399, publish 122554167193).
All four public downloads were independently checked against GitHub digests/sizes,
inner/outer manifests, safe ZIP inventories/CRCs, x64 GUI 1.17.106 PE/version and
all 903 packaged source files. Public EXE and portable ZIP exactly match reviewed
artifact 12781520240. Updater contains only the versioned executable, SHA256SUMS,
THIRD_PARTY_NOTICES.txt and UPDATING.txt under Network_Toolkit_1.17.106/.
Latest release/update feed and tag point to 106. Prior 105 release 426056352, tag,
body and all four original asset IDs/sizes/SHA256s were independently reconciled.
No access change, retired-feed write or old release replacement occurred.

Public SHA256:
- Network_Toolkit.exe: d363b33ddf1ea0af5e66a7090e743c3a3d367c8495bbcff57eba08ed35b54813
- Network_Toolkit_1.17.106_Portable.zip: df9becf4d841b086e8f0dbcd2636e806a1b608bd93e756db38bd3e9c368b99bf
- Network_Toolkit_1.17.106_Update.zip: 25ab2f7ea60414d31cff14cbbe1269a12bf15296c662748d0498186febd8c9e1
- SHA256SUMS: 6f01044fd12c3886a55db9dc64392350ea9aa37ab3077c6b6e88d8b97f0465b5

Automatic duplicate-main Checks 40608337579 completed SUCCESS at 00:39:32 UTC:
application 122553366729, dependency 122553366719, full browser 122553366353,
security/new-feature browser 122553366657 and Windows 122553366698 all passed.
No optional rerun or application change followed publication. This [skip ci]
closure changes only AGENTS.md, DEVELOPMENT_HANDOFF.md, RELEASE106_HANDOFF.md
and VALIDATION.txt; published source/tag/assets remain immutable.

Limits: all browser/device/clipboard fixtures are synthetic; no user switch,
credentials, settings, migration, native installation/reboot or live macro run
was accessed. Active adapters do not prove remote firewall/routing reachability.
Existing 105 and 104 security, paging/archive, defaults, graph and icon safeguards
remain intact. No credential backend, native launcher or transfer execution path
was broadened. Future changes require new user scope.

## Publication bookkeeping note

The original final index-blob approval call paused, then succeeded without a
retry or alternate route. All uploaded blob SHAs and the created tree matched
local immutable source. Before promotion, required exact-source gates passed
and fresh main/review/latest/tag-absence reads matched. A temporary-file loss
prevented the saved prior 105 metadata comparison from running at that point;
promotion continued with its expected-head lease. Prior 105 release/tag/body and
asset identities/hashes were reconciled after promotion against recorded 105
publication data and unchanged archived release-note bytes, before final public
verification was reported. Durable final proofs were moved into workspace storage.

# Original candidate and validation history

Earlier pending/checkpoint statements below describe prepublication source. The
verified release and later documentation-only closure above supersede them.

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
