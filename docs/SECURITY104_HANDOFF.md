# 1.17.104 combined candidate — implementation and validation in progress

2026-10-27 update: the user confirmed master password with remembered unlock
across app restarts and Windows reboots after explicit same-user risk disclosure.
The former decision hold below is historical and superseded. Publication is
approved after required combined checks, not before. Main/latest103 remain
unchanged until promotion. No actual vault/settings were accessed or migrated.

Current additional scope: authenticated v2 vault/Argon2id, native setup/unlock/
rotation/recovery dialogs, optional offline recovery, CurrentUser DPAPI remembered
random key, configuration-bound reset/lock, cross-instance transaction guard,
verified explicit legacy migration with separate cleanup, action-only web UI;
plus terminal pager correctness and new/unset Auto pager + Ctrl+C copy defaults.
See CREDENTIAL_SECURITY.md and TERMINAL_PAGING.md. Earlier graph/icon/bootstrap
work and its green evidence are retained, but that evidence does not validate the
new v2 code. First combined hosted checkpoint is pending; local JS/Python,
formatting and source review are preliminary only. Native Windows dialogs and
new DPAPI/lifecycle tests must run and their captured pixels must be inspected.

## First combined hosted checkpoint and focused repairs

Review source 729ccfa59dde8b70debf6593b39c573e9702c8b4, tree
e44646258677d070ad97f09ddbee0ad58e37a251 (local19af31d) ran Checks40577022470.
Windows122446077053, security/pager browser122446076819 and dependency122446077494
passed. Crypto/lifecycle native tests included pending-authentication isolation,
configuration-bound remembered restart/reset, stale-instance protection, native
Unicode/overlimit input, recovery re-entry and default-No/cancellation behavior.
Canonical vendor comparison was empty; both scans reported zero reachable and
imported-package vulnerabilities, retaining the documented unused OpenPGP notice.
Security/pager artifact12767677719 SHA256
8e3d631e2879d5cf3b837ea781e96460ee589c6df816d59fe35c770df92f5be0
was downloaded/verified; normal/narrow vault and pager screenshots were reviewed.

Application122446077092 exposed a preexisting test assumption of exactly one
settings-directory file. The new permanent transaction lock is intentional and
must not be unlinked. The repair asserts the two exact regular filenames, not a
relaxed count or wildcard. Other Go packages, including authenticated crypto,
passed. Actual first setup PNG was black in the client area despite passing
control tests: capture ran during WM_INITDIALOG. Test-only post-paint capture
and stronger blank-client rejection must pass and its real pixels be inspected
before final acceptance. Unlock and recovery screenshots were correctly rendered.
These findings are not represented as final combined-source success.

A tool-catalog interruption and later automated approval stop occurred during
blob upload. Completed objects were checked by exact SHA before continuation.
The user's explicit renewed public-source/publication approval (2026-10-27)
resolved the denied operations on one retry each; no alternative route or bypass
was used. Main/latest remained103. No user credential data was involved.

## Historical independent checkpoint (before confirmed choices)

Recorded 2026-10-27. This is an isolated development branch, not a released
1.17.104 application. Published main/latest remain unchanged:

- Main: `8d0a8724f5b2bfea607b8ca32982f2abad5ee5ac`.
- Latest: v1.17.103, release 425758158, source/tag
  `b9f365e6a8df0a04be7daf3a083e2334a63f9f52`.
- Development branch: `security/credential-boundary-1.17.104`.
- Tested source: `f65625a4f7d84673e0496c1881e396a175603c52`.
- Tested tree: `88fba32be7d799a652f7001960f22a1159b3c9b8`.
- Local mirror commit `b73a47bd71a36d02f02bbc74d64b93c25a21e6a3` has that
  identical tree. A subsequent documentation-only closure does not rebuild it.

## Required decisions before dependent implementation/publication

1. Confirm the actual cryptographic unlock and recovery policy: master password
   with offline recovery versus a supported protected-key mechanism. Ordinary
   Hello/PIN consent followed by automatic DPAPI decryption is not a vault lock.
2. Confirm whether locking disconnects existing credential-backed sessions or
   preserves established connections. No silent choice has been made.

The user authorized security development/publication, the 30-second-reference
graph adjustment in the same build, and the supplied canonical icon. That does
not authorize accessing actual credentials or changing installed security settings.
All tests used synthetic fixtures. No existing vault was migrated or deleted.
Stop after this green independent checkpoint; do not invent additional features
or repeat unchanged checks while waiting for the decisions.

## Independent work implemented

See [CREDENTIAL_SECURITY.md](CREDENTIAL_SECURITY.md) for precise protections and
limits. Saved secrets no longer return through credential GET/list APIs; backend
credential IDs are destination-bound with native default-No approval and server
identity verification. Reconnect resolves IDs afresh rather than caching plaintext.
Non-Windows vault access fails closed. Windows encrypted files are created
atomically with a protected current-user/SYSTEM/Administrators DACL and exclusive,
non-inheritable handles. No filename collision changes an existing file.

The full app capability is injected by the native WebView host, not served in
public HTTP HTML. Independent terminal activation uses a separate limited token.
Only the initial exact native document navigation receives a one-use permit;
later frames, full navigation and reload are denied before network access. Genuine
fragment navigation works. Close/reopen the native window for a fresh bootstrap.
WebView2 API floor is 122.0.2365.46; recommend latest patched Evergreen x64, not
that old minimum. Unsupported runtimes fail with an official download link;
there is no automatic installation or weaker fallback.

This remains an automatically unlocked, per-user DPAPI vault. True cryptographic
locking/recovery, native secret entry and transactional migration are NOT done.
The renderer still has legitimate local shell capability, so this is not a strong
boundary against a compromised renderer or same-user malware, nor an audit or
security certification. Ordinary confirmation is not cryptographic authentication.

Condensed All-mode geometry now uses `H = 3 * recentMs / (2 * ln(4))`:
default H approximately 64.921 seconds. Newest spacing matches the local slope
of Linear 30 seconds, progressively compressing older points to the left.
Explicit timeframes/ranges and packet semantics remain unchanged. See
[CONDENSED_GRAPH_SCALING.md](CONDENSED_GRAPH_SCALING.md).

The supplied editable SVG is preserved byte-for-byte at
`assets/source/Network_Toolkit_Icon.original.svg` (SHA256
`d23938cfbf503b5534e22febdfd62167cb9a2ab1d2eee66c9c8829315896ce99`).
Safe derived SVG, PNG, nine-frame ICO, manifest and embedded Windows resources
are integrated. DTD/external entity/script/resource handling fails closed in the
generator; the original Illustrator editor data is not a runtime embedded asset.
No image redesign or generated replacement artwork was used.

Go is pinned to supported patched 1.26.10. Pinned module versions are unchanged.
Canonical checksum verification exposed six historical compatibility edits in
x/crypto SSH; those files now exactly match upstream v0.58.0, restoring FIPS-mode
checks and hybrid ML-KEM support. This does not enable FIPS mode or certify the app.
Synthetic tests cover modern ML-KEM/Curve25519 negotiation and existing legacy
opt-in behavior, without bypassing host identity approval.

## Exact-source validation

[Checks 40565910973](https://github.com/TechSavage1989/network-toolkit/actions/runs/40565910973)
completed SUCCESS. All five jobs passed:

- Application/build 122407258204: Go package/lifecycle tests, 713 JavaScript
  checks, 42 Python guards, icon resource generation and Windows x64 GUI build.
- Windows 122407258385: real DPAPI/tamper and DACL/creation/collision tests,
  activation-only isolation, real WebView2 root and detached-document bootstrap,
  local shells, installer progress and timestamp/statistics tests.
- Dependency 122407258090: module checksums, exact canonical vendor comparison,
  Linux and Windows govulncheck v1.9.0 scans.
- Security browser 122407257916: metadata-only and write-only editor behavior,
  denial/success/stale destination/repeated resize flows at 1280 and 390 px.
- Full browser 122407258048: graph/layout/navigation/export regressions and
  actual native Chromium five-minute updater timer (302046 ms), not a shortened
  substitute. The historical negative-control fixture still reproduces its bug.

Actual WebView2 Runtime 155.0.4283.62 loaded both synthetic pages, retained the
original document, and allowed exactly one document HTTP request. Forbidden and
exact-URL iframes, full navigation and reload were blocked without unexpected
network requests. The prior failed predicate incorrectly depended on a Fetch
Metadata header unavailable at pre-network interception; the initial-only permit
fix did not weaken assertions. COM mock objects/vtables are pinned for Go 1.26.

Scans found zero reachable-symbol or imported-package vulnerabilities. Module-only
GO-2026-5932 affects unused/unvendored x/crypto/openpgp, has no fixed version, and
is disclosed rather than called a blanket vulnerability-free result.

Final security and graph normal/narrow screenshots were inspected. Actual newest
one-second gaps are 24.83 px versus Linear30 25.40 at 1440 px, and 7.56 versus
7.73 at 390 px (under 3% difference), stable from two to 1,010,000 arrivals.
Old records retain a single left cluster and exact represented count. Icon
small-size pixels and safe-reference equivalence were also inspected.

Verified artifact digests (GitHub retention is seven days):
- `transfer-browser-checks`, 12764531251:
  `d62e03058803740bcce9257b4824baf4680c80c18bb31c02dfdfb69e2a8d002f`.
- `credential-security-browser-checks`, 12762877462:
  `12f4b8ca524d11e0c2a9f2aa33835f95dcc6260be35cae0d2075c0a1e3b74346`.
- `dependency-security-review`, 12763515786 (GitHub reported digest):
  `129dcbe8c94126a8fd37a25a8c96665c7c3ef8812393134609d75b9fcc354f55`.

No actual-user Windows installation/update, human confirmation-dialog interaction,
Explorer/taskbar visual inspection, or final release-package acceptance is claimed.
Local Go/browser execution was unavailable; hosted tests supplied that evidence.

## Resume/release gates

After the decisions, implement only the agreed remaining design and explicit,
versioned transactional setup/migration. Preserve old data until verified recovery;
never silently fall back or irreversibly remove old copies. Reassess tests against
the final implementation rather than treating this checkpoint as its acceptance.

Version metadata deliberately remains 1.17.103 while held. Before publishing104,
rotate the prior root release note unchanged to docs/releases, update all version
consumers, regenerate the icon/version .syso, prepare final notes/validation and
obtain exact-source passing Checks plus the review portable/package acceptance.
The existing `review/hosts-editor-1.17.104` branch prefix enables review packaging.
Fresh-check main and promote with an expected-head, non-force update only when
the agreed scope is complete. Verify Release/feed/package/source provenance and
unchanged old assets. Never replace old tags/assets, alter access or use the
retired feed. No final publication is authorized by this green-checkpoint note.
