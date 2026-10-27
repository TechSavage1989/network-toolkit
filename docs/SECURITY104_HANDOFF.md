# 1.17.104 security release — completed

## 1.17.104 published and independently verified

Published 2026-10-27:
https://github.com/TechSavage1989/network-toolkit/releases/tag/v1.17.104
Release ID: 425987316
Exact reviewed/published source and tag: 554f360613aa960a28ee459a44018da39a0bbcff
Tree: da4e0a4193a7454e54c8cd12b19e3fc364fc0d47
Expected main baseline: 8d0a8724f5b2bfea607b8ca32982f2abad5ee5ac.
Explicit user approval of that exact promotion resolved an approval-review stop;
the single non-force expected-head retry succeeded. No access change or bypass.
This later handoff-only commit does not move the tag, rebuild or replace assets.

Completed combined scope:
- Authenticated master-password vault with optional offline recovery and native
  setup/unlock/rotation/recovery. Remember defaults on, as expressly requested,
  across app restarts and reboots; only a CurrentUser DPAPI-protected random key
  is remembered, never the master password. All credential metadata is encrypted.
- Backend-only, destination-bound saved credential IDs, native destination consent,
  SSH/FTPS server identity checks, no saved-secret GET or plaintext reconnect
  cache, fail-closed non-Windows storage and native-only capability bootstrap.
- Condensed All-mode newest spacing approximately Linear 30 seconds, progressively
  compressing older history; supplied canonical icon and embedded resources.
- Screenshot-matched pager suffix recognition, repeated/manual paging and accurate
  last-command/last-paste selection; Auto pager on and Ctrl+C copy-selection only
  for new/unset values, preserving explicit existing choices.

Required exact-source Checks40581168624 completed SUCCESS, all five jobs:
- Application122460611320: full Go lifecycle/crypto tests, 751 JavaScript tests,
  42 Python guards, Windows x64 GUI build and review portable package.
- Dependency122460611297: checksum-verified canonical vendor exact match; Linux
  and Windows govulncheck report zero reachable/imported-package vulnerabilities.
  Unused/unvendored OpenPGP module advisory GO-2026-5932 remains disclosed; this
  is not a blanket vulnerability-free claim or an independent security audit.
- Security/pager browser122460611322: synthetic success, denial, missing/locked
  status, metadata-only/write-only editor and paging/selection/default flows.
- Windows122460611443: real DPAPI/DACL/bootstrap and native vault lifecycle,
  input-boundary, cancellation, recovery, stale-instance and reset tests.
- Full browser122460611132: graph/layout/session regressions and actual native
  five-minute updater timer (301764 ms), plus historical negative control.

Actual final setup/unlock/recovery dialog pixels, normal/narrow vault and pager
screens, and normal/narrow million-history graph captures were inspected.
Graph newest gaps: 24.83 vs Linear30 25.40 px normally; 7.56 vs 7.73 px narrowly,
under 3% difference and stable from two through 1,010,000 arrivals. The million
case retains 65 recent arrivals and one 1,009,935-count historical cluster.
Earlier capture and saved-session fixture failures were repaired without relaxing
security assertions; the final exact-source run above passed every required gate.

Release40583930470 completed SUCCESS (build122470300491, publish122471235829).
All four public assets were downloaded independently using the release API's exact
URLs. GitHub digests/sizes, inner/outer manifests, ZIP safety/CRCs, x64 GUI PE/version,
updater allowlist and all 883 packaged source files were verified. Public EXE and
portable ZIP exactly match the reviewed build. Latest release/feed points to104;
prior103 release425758158, body, asset IDs and digests remain unchanged.

Public SHA256:
- Network_Toolkit.exe: 064e5467fab0c9c2008ea5cadb36529692cbbafc83537e3967453b7be60da9a2
- Network_Toolkit_1.17.104_Portable.zip: 7d663cc8998c7d832afbb99ef28d538b2ba12f856357e636bfba105f912111ad
- Network_Toolkit_1.17.104_Update.zip: 1be73563cb918e55e7a9e6b54521fad6bac6ff9ab280127dbadc949e825ea804
- SHA256SUMS: 512a30b27f36783391466c4beb943625ab64911201ad39673f1231fb74b3044e
Updater contains only Network_Toolkit_1.17.104_Windows_x64.exe, SHA256SUMS,
THIRD_PARTY_NOTICES.txt and UPDATING.txt under Network_Toolkit_1.17.104/.

Automatic duplicate-main Checks40583930478 completed SUCCESS at 21:23:23 UTC:
application122470299484, dependency122470299834, security browser122470299903,
full browser122470299954 and Windows122470300064 all passed. No optional rerun
or additional application change was needed.

First-use and security limits:
- Existing legacy vaults are not automatically opened/migrated. Use Password
  manager's explicit native setup. Cancellation/failure before verified activation
  preserves the old copy; corrupt v2 never silently falls back to legacy.
- The old DPAPI-only file remains a weaker-protected backup. After verifying the
  migration and recovery, the user may separately confirm legacy cleanup. Backups
  elsewhere and deleted disk blocks are not guaranteed securely erased.
- Remembered unlock remains accessible to software running as the same Windows
  account. Uncheck Remember for session-only unlock. Optional recovery is another
  secret to keep separately; there is no vendor recovery if all unlock means fail.
- Manual Lock and forget keeps established sessions but blocks subsequent saved
  credential use/reconnect. Already resolved/in-flight authentication may finish.
  Config reset/deletion forgets unlock, not the encrypted vault; deleting one
  connection never deletes the whole vault. Whole-directory rollback is not
  cryptographically prevented. Remote credential revocation is a server action.
- WebView2 API minimum is 122.0.2365.46, not a recommended old security baseline.
  Use the latest patched Evergreen x64 runtime from Microsoft's official page:
  https://developer.microsoft.com/microsoft-edge/webview2/#download
  Unsupported runtimes fail with a clear download/update message; no automatic
  installation or weaker filtering fallback. Windows 10/11 x64 remains required.
- The renderer retains legitimate local-shell capability; this is not a strong
  boundary against renderer compromise, same-user malware or administrators.
  Memory/paging/dumps and terminal exports can retain secrets. See
  CREDENTIAL_SECURITY.md for precise implementation, threat model and limits.
- All tests were synthetic. No actual user credentials/settings were accessed,
  migrated or deleted. No actual-user installation/reboot or live device-fleet
  validation is claimed. Pager patterns and history retention are bounded, not
  universal device support or unlimited raw output retention.

No optional rerun, unrelated feature, access change, retired-feed write or old
release replacement was performed. Future implementation needs new user scope.

## Final review artifact evidence

Verified downloads from Checks40581168624:
- Review portable12769922182: adee4b3b69974298d75084d2aecc38bdf83fcfd38ef0bacd11df5da7388fdb5e
- Security/pager12770350800: 64d4b84417d0e68487550b846fe4683026070664673aba9aabbf75f8179c0bf1
- Native12769452444: 1364ab0c80102ae09f850b459dbb33a4c73c8e0d3705f48bae3ee62f5bc3e3fb
- Dependency12769727287: 69523dc293ef200b10bf5a1ebc44781e84e7d0997ed9ebe8f2a7131df5b04d95
- Full browser12769423933: edde0e001cd8497b7cb8d2aed50cb401850cf8854ee34a1cda3ce0d6e85fb56d
Artifacts have finite GitHub retention; public release assets are the durable
user downloads. The source archive intentionally contains its pre-publication
validation note, not this later docs-only closure.

All decision holds and pending gates below are historical, not current blockers.
They are retained to distinguish early evidence from final-source verification.

---

# Historical candidate record (superseded by the publication above)

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
