# 1.17.105 candidate — banner alignment and discovery workflow

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
