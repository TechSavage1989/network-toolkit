# 1.17.105 — published banner and discovery improvements

## 1.17.105 published and independently verified

Published 2026-10-27:
https://github.com/TechSavage1989/network-toolkit/releases/tag/v1.17.105
Release ID: 426056352
Exact reviewed/published source and tag: 1d716c1916833d9fb14dee44016bf0f75e568305
Tree: 31fafbe7e2b327254e68b95cb2bb3bfa46f0a8cc
Baseline: main 1197870a348b71558af1ce3402601aa09cfaccc6, the completed 104 closure.
Expected-head, non-force promotion succeeded after all exact-source gates passed.
This later handoff-only record does not rebuild, move the tag or replace assets.

Completed user-authorized scope:
- SSH authentication-banner bare-LF normalization prevents cumulative staircase
  indentation while preserving CRLF/lone CR, callback boundaries, UTF-8/ANSI and
  normal PTY/Telnet/serial bytes. The screenshot-shaped negative control reproduces
  the original symptom; normal/narrow corrected banners and later prompts align.
- Discovery filters accept multiline clipboard lists, whitespace/comma-separated
  alternatives and quoted phrases. MAC colon/hyphen/plain/Cisco-dotted forms
  normalize; complete IP addresses match exactly. Text searches retain relevant
  device fields. Existing outcome/type/view controls remain AND constraints.
  Shared discovery/TCP/ping/neighbor views use the same semantics. Network-only
  persistence accepts 32,768 UTF-16 units/98,304 UTF-8 bytes; oversized paste is
  rejected explicitly without partial replacement, not silently truncated.
- Open TCP ports are keyboard-accessible connection actions. Recorded verified
  service evidence, when present, is preferred; port numbers remain hints, and unknown
  services require a choice. Terminal/file clients prepare the clicked host/port
  with blank fresh authentication and no auto-connect. Browser/RDP reuse the
  constrained native launcher. Busy/pending existing file operations are not
  displaced. Independent numeric port-list copy controls remain available.

Required exact-source Checks 40592954170 completed SUCCESS, all five jobs:
- Application 122501580365: Go lifecycle/banner/filter API tests, 787 JavaScript
  tests, 42 Python guards, Windows x64 GUI build and review portable package.
- Dependency 122501580515: canonical vendor exact match; Linux/Windows scans
  report zero reachable/imported-package vulnerabilities. Unused/unvendored
  OpenPGP advisory GO-2026-5932 remains disclosed, not a blanket audit claim.
- Security/new-feature browser 122501580500: credential and pager regressions,
  byte-boundary banner replay, native multiline clipboard filters, port routing,
  independent copy and zero-unintended-launch checks with synthetic fixtures.
- Windows 122501580636: existing DPAPI/native vault/bootstrap/security acceptance.
- Full browser 122501580585: existing layout/graph/clipboard/session regressions,
  six-layout scan/copy checks and actual five-minute updater timer 301,577 ms,
  plus historical negative control. No timer shortcut or skipped required gate.

Actual normal/narrow banner, filter/pinned-header and port/chooser PNGs, clean
light/dark scan layouts and native vault/credential/pager images were inspected.
Earlier missing asset allowlist, exact prompt-space fixture, responsive Results
selection fixture and aggregate port-copy compatibility failures were fixed;
existing security/clipboard/layout assertions were not weakened for acceptance.
Short fixture summary/icon/toast artifacts are not live-device acceptance claims.

Release 40595062787 completed SUCCESS (build 122508776981, publish 122509396402).
All four public assets were independently downloaded from their exact release
URLs. GitHub digests/sizes, manifests, ZIP safety/CRCs, x64 GUI PE/version, all 897
packaged source files and the updater four-file allowlist were verified. Public
EXE and portable ZIP exactly match the reviewed package. Latest release/update
feed and tag point to 105. Prior 104 release 425987316, body, asset IDs, sizes and
digests remain unchanged. No repository access or retired update feed was changed.

Public SHA256:
- Network_Toolkit.exe: a2fa3a3b8669ba468234936f21e7bcf9e6e32506e609d5edebe6e14b75757442
- Network_Toolkit_1.17.105_Portable.zip: ebb043802f5ca1e0c2ede0bbfa7526b677364a0c0a78525c423aa33d97f2b57a
- Network_Toolkit_1.17.105_Update.zip: 6a089a514a382b5671a87ed16945772e4e61ab11844e657454b1c8c542aea307
- SHA256SUMS: 7a034efd40ede195105107280ce5e93cb30c8adbaa8e1d800c15bcd9c158cf43
Updater members are only Network_Toolkit_1.17.105_Windows_x64.exe, SHA256SUMS,
THIRD_PARTY_NOTICES.txt and UPDATING.txt under Network_Toolkit_1.17.105/.

Automatic duplicate-main Checks 40595062882 completed SUCCESS at 22:42:57 UTC:
application 122508777810, dependency 122508777455, browser 122508777815,
security/new-feature browser 122508777994 and Windows 122508778002 all passed.
No optional rerun or application change followed publication. This closure changes
only AGENTS.md, DEVELOPMENT_HANDOFF.md, RELEASE105_HANDOFF.md and VALIDATION.txt
with [skip ci]; published source, tags and assets remain immutable.

Limits: all fixture/device data is synthetic; no real user switch, credentials,
settings, migration, installation or reboot was accessed. This is a focused
banner fix, not universal terminal-format/device-fleet certification. Existing
104 vault threat-model limitations, bounded pager patterns/archives and explicit
saved preferences remain unchanged. No credential-backend/vaultcrypto/native
launcher behavior was broadened. Future implementation needs new user scope.

# Original candidate and validation history

The following sections retain prepublication checkpoints. Their pending/failed
status statements describe those earlier snapshots, not the final release above.

The user authorized implementation and publication of three changes on
2026-10-27: SSH login-banner formatting, multi-value discovery filtering, and
explicit TCP-port connection actions. Baseline main is
`1197870a348b71558af1ce3402601aa09cfaccc6`, closure tree
`d052531be61be98642f59bdeb13819c05e7d92ff`. Published 104 source/tag and assets
remain immutable. This candidate record does not claim publication or passed
hosted gates before their terminal results are recorded.

## Banner evidence and scope

The supplied Aruba CX screenshot shows cumulative rightward indentation and
mid-word wrapping in the login banner, while the subsequent password prompt is
left aligned. Actual pixels were inspected. A bundled-xterm synthetic replay
reproduces this staircase with bare LF and corrects it with CRLF. The screenshot
alone cannot establish the transport or exact received bytes; no live switch
or raw user transcript was accessed.

`SSH BannerCallback` previously forwarded authentication-banner bytes unchanged
before requesting a PTY. The renderer intentionally uses `convertEol:false`.
The fix inserts CR only before bare LF in this banner callback, retaining state
across callbacks for fragmented CRLF. CRLF, lone CR, UTF-8 and controls remain
unchanged. Normal PTY/Telnet/serial bytes, ANSI behavior, dimensions, fonts,
pager processing and output archives are not rewritten.

Primary protocol/rendering references:
- https://datatracker.ietf.org/doc/html/rfc4252#section-5.4
- https://xtermjs.org/docs/api/terminal/interfaces/iterminaloptions/#converteol

## Discovery contract

The query uses OR across newline/whitespace/comma-separated entries, with
quoted phrases for multi-word text. Existing status/type/responder controls
remain AND constraints. MAC punctuation/case normalizes; complete IP addresses
match exactly instead of matching a longer neighboring address. Ordinary text
uses searchable device fields. Clipboard handling must retain list separators
before a single-line input can discard newlines.

TCP-port actions require an explicit user gesture. Reliable service information
is preferred; conventional port numbers are only hints. Unknown services need
a tool choice. Existing terminal/file dialogs prepare a fresh destination with
no automatic credential reuse; browser/RDP reuse the strictly validated native
route. Existing vault binding, native consent, host identity and insecure-protocol
restrictions stay intact. Rendering results must never initiate a connection.

## Validation and publication gates

- Shared banner fixtures are verified against production Go normalization at
  every byte boundary, including fragmented UTF-8/ANSI, repeated callbacks,
  mixed endings and a local synthetic SSH banner-to-shell integration.
- Real bundled-xterm browser acceptance checks normal/narrow columns, intended
  indentation, later prompt alignment, unmodified raw PTY LF/ANSI, and a genuine
  LF-only negative control. Inspect saved PNGs, not just passing assertions.
- Discovery unit/browser coverage must exercise native clipboard multiline
  paste, multiple/duplicate/empty queries, normalized addresses, status/type
  constraints, keyboard/click actions, unknown-service choice and mocked launch
  boundaries without real network/device connections.
- All existing exact-source application, dependency, Windows native security,
  browser/pager and real five-minute updater gates remain required. Local Go and
  Chromium execution are unavailable in this executor; hosted results provide
  that evidence. Never mislabel unrun checks as passing.
- Before promotion, verify review source, tree, portable inventory and x64 GUI
  PE/version. Use expected-head non-force promotion only after fresh baseline
  checks. After Release, independently verify all public asset digests/manifests,
  source bytes, reviewed-binary identity, updater allowlist and unchanged 104
  metadata. Observe automatically started duplicate-main Checks to completion.

No actual user credentials/settings, migration, native user installation or live
device fleet validation is included. No universal terminal-format guarantee,
unlimited archives, access change, old asset replacement or retired-feed write.

## Integrated local checkpoint before hosted review

All three features and their bounded async guards are integrated. Final local
JavaScript aggregate: 785/785 pass; Python release/layout guards: 42/42 pass.
Version/resource metadata validates as 1.17.105; modified JS and inline script
syntax and git whitespace checks pass. The prior root104 note was moved unchanged.
Local Go/Chromium checks were not run; they still require hosted acceptance.

Targeted independent source review found and closed discovery-prefill races with
pending profile selection/load, disconnect-before-connect, key-file read and
credential-save local-key reads. Prefill refuses these operations rather than
displacing them. Terminal key-file reads are bound to the original dialog, input
elements and request generation, preventing stale completion/error/cleanup from
affecting a new destination. Deferred-response tests cover each path. There are
no credential-backend, vaultcrypto or native desktop changes from 104 closure.

Long filter lists survive backend API save and new-application reload: the
network-only limit is 32,768 UTF-16 units (matching the input), with a 98,304-byte
UTF-8 ceiling; generic query limits and the overall 2 MiB state cap are unchanged.
Local synthetic matching measured about 12 ms for 1,024 rows ×1,000 MAC alternatives
and 177 ms for 1,024 rows ×700 ordinary-text alternatives; these are illustrative
executor measurements, not universal performance promises. Oversized paste is
rejected visibly without partially replacing the current query.

Browser, native Windows, dependency, build and package results are pending.
Do not promote from these local results alone.

## First hosted checkpoint and focused repairs

Review 0c04945483af2b99568384874efa75e140a95edd (tree ea2208ea) ran
Checks 40589587198. Windows native and dependency-security jobs passed. The banner
Go fixtures and shell transition passed. The application dependency-serving test
correctly caught 404s for the two new embedded discovery scripts: their exact
filenames were missing from the strict HTTP allowlist. Added only those two
entries, retaining deny-by-default and unsafe-path checks.

Existing credential/pager browser cases passed; the new normal-width banner
rendered aligned and its actual PNG was inspected. Its shell-prompt assertion
incorrectly expected a written trailing space to disappear. Corrected the exact
expected string while retaining cursor/ANSI checks; bundled-xterm buffer replay
passes at both widths and all byte boundaries. Independent discovery browser
steps now run after another test failure, without ignoring failures or weakening
assertions. First-run discovery browser cases had not yet run. New exact-source
hosted validation remains required; this failed checkpoint is not acceptance.

## Copy-action compatibility repair before acceptance

Review 8015cfbe37f050ccaa65ac486be309489a258013 / Checks40591037403 passed
application, dependency, Windows native and security/new-feature browser jobs.
Actual normal/narrow banner, filter, sticky-header and port-action pixels were
inspected. The long browser gate correctly caught a compatibility regression:
interactive TCP-port buttons caused the existing value-copy helper to omit the
independent numeric open-port-list copy control. Restore that narrowly labeled,
validated numeric aggregate only, preserving all sensitive/editable/terminal and
arbitrary-control exclusions. The existing long-browser assertion is retained,
with new unit and native-browser clipboard coverage checking no connection launch.
Independent targeted review and all 787 JavaScript / 42 Python tests pass locally.
All five exact-source hosted gates must run again before promotion; the previous
four passing jobs and reviewed package are not acceptance for this repaired tree.
