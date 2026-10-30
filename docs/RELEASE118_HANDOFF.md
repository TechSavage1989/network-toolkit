# Network Toolkit 1.17.118 — Connect sidebar consistency

## Published identity and authorization

- Release: [v1.17.118](https://github.com/TechSavage1989/network-toolkit/releases/tag/v1.17.118), ID **428544757**.
- Published: **2026-10-30T00:56:56Z**. Actual public verification: **2026-10-30T01:08:14.010Z**.
- Source/tag/main at publication: `2be7a62d18865275bd323c26b07f1520b689b560`.
- Frozen tree: `e4fbfeaed734c5f556cbbab4e4ed3a1ce9202d52`.
- Local reviewed source: `951ee7f0c5749271442eed388fdf93c174772f8f`, with the same frozen tree.
- Previous main lease: `0806cc155d0373b94639e536d2d60ceba710b40b`, completed 117 documentation.
- Expected-head, non-force promotion was read back at **00:54:24.495Z**.

On 2026-10-30 at 00:24:26 UTC the user requested Connect first in the Editor
sidebar and a consistent, distinct Connect icon throughout application sidebars.
At 00:26:01 UTC the user explicitly requested the build and publication. This
record describes that bounded authorization, not a future blanket grant. No
repository visibility/access, live user session, credential or device was changed.

## Scope: four production presentation lines

1. The existing shared SVG registry defines one dedicated `connect` plug icon.
2. Editor rail descriptors place Connect before Files and use that icon.
3. The shared transfer-client Connect tab uses the icon for SFTP/SSH, FTP/FTPS
   and TFTP clients.
4. The shared transfer-server Connect tab uses it for SFTP, FTP and TFTP servers.

The Editor's selected panel and saved preferences are not reset. Default Files
and explicitly saved Connect selection remain intact. Existing keyboard order,
labels, accessible titles, sizing, active states, collapse/resize controls and
connection actions are retained. The Terminal sidebar has Saved, Recent, Macros
and SFTP rather than a Connect tab; its unrelated icons were not replaced.
Serial/port-scan icons and ordinary Connect action buttons are also unchanged.
No raster asset, icon package, connection/authentication handler, privilege path,
document owner/lease or resize logic was added or changed.

Version metadata, the normal release-note rotation, focused tests and an early
artifact step accompany those four lines. The 117 root release note was moved
unchanged into the historical archive; 118 is the sole current root note. All
seven existing workflow jobs remain enabled. Comparison, Hosts, native-transfer
and responsive-resize problems are not fixes in this release.

## Acceptance before and after publication

### Focused UI and local checks

- **1,303/1,303 JavaScript tests** and **42 Python tests** passed locally.
- An independent bounded run passed **63 focused tests**.
- `connect-sidebar118.browser.cjs` passed **14/14 actual rendered cases**, with
  all 14 screenshots individually inspected: Editor main/detached, 1440/390,
  light/dark (eight cases), plus six shared client/server surfaces.
- The focused test checks Connect-first DOM order, identical dedicated SVG,
  selected-panel retention, keyboard Home/End/arrows/Enter/Space, collapse and
  retained dirty-document identity. It does not connect or start a server merely
  by selecting a tab. Its error list was empty.
- The retained 117 sidebar fixture passed all eight contexts/24 groups, retaining
  its measured **39 px** explorer-height gain. Connect icons remain **17×17 px**;
  Editor rail buttons remain **42×65 px**. Transfer rail height is unchanged.
- Actual Terminal selection, X-close and reorder safeguards passed all four
  116 cases, including post-drag X clicks. Four local-shell-control browser cases
  passed. No private configuration or user credential was used in screenshots.

### Why there were two pre-release review refs

The first review ref was `review/connect-sidebar-118`. The existing package-proof
step is deliberately conditioned on `review/hosts-editor-*`, so its application
job passed but skipped package creation. That was caught before promotion.
A second standard package-review ref, `review/hosts-editor-118`, was created at
**the identical source SHA and tree**. No source or workflow change, failed-job
suppression, destructive rerun or asset replacement was used. Both original
failures remain recorded; the second run's success is not their diagnosis.

### Exact run inventories

All runs below use source `2be7a62d18865275bd323c26b07f1520b689b560`.

| Job | First review 40981423089 | Package review 40981780134 | Duplicate main 40982593582 |
| --- | --- | --- | --- |
| check | SUCCESS, 123822143906 | SUCCESS, 123823304947 | SUCCESS, 123825958755 |
| browser-security | SUCCESS, 123822144177 | SUCCESS, 123823305001 | SUCCESS, 123825958957 |
| dependency-security | SUCCESS, 123822144221 | SUCCESS, 123823305025 | SUCCESS, 123825958996 |
| windows-terminal | FAILURE, 123822144281 | SUCCESS, 123823305090 | FAILURE, 123825958934 |
| browser-editor112 | FAILURE, 123822144206 | SUCCESS, 123823304757 | SUCCESS, 123825958976 |
| browser-workflows107 | SUCCESS, 123822144187 | SUCCESS, 123823304978 | SUCCESS, 123825958986 |
| browser | SUCCESS, 123822144212 | SUCCESS, 123823305022 | SUCCESS, 123825958959 |

First review ended **five SUCCESS/two FAILURE**, with its last job completing
**01:00:11Z**. Package review ended **all seven SUCCESS**, with its last job
completing **01:04:33Z**. Duplicate main: **six SUCCESS/one FAILURE (native Alt-editor transfer), last job completed 01:11:35Z**.
Automatic Release **40982593497** completed successfully: build **123825958151**
and publish **123826752977** both SUCCESS. No blanket all-runs-green claim is made.

### Retained failures: do not erase or call fixture-only

- First-review Windows native editor transfer failed after **87.31 s** at
  `editor_native_windows_test.go:147`: actual Alt-mouse drag between two
  production WebViews, **"real backend never recorded pending transfer"**.
  Native dirty-close consent passed **10.90 s** beforehand. Subsequent dependent
  native stages were skipped; missing downstream artifact uploads were secondary.
- Duplicate-main Windows failed at the same Alt-mouse/pending-transfer checkpoint
  after **75.94 s**, following dirty-close consent passing **10.82 s**. The
  helper/coordinator/fixture for this checkpoint is unchanged from 117. The exact
  cause is unresolved; a matching checkpoint is not proof of identical cause.
- First-review Editor Views failed at `editor-views112.browser.cjs:71` after
  changing a detached viewport from 390 to 1440 and pressing ArrowRight on the
  resize handle: `Number(aria-valuenow) > 300` was false. Twelve main groups and
  six detached groups had passed first, including local/approved-SFTP operations;
  the page error list was empty. The screenshot showed the restored wide sidebar
  at approximately 300 px. Actual focus and the exact ARIA value were not logged.
  The fixture immediately focuses after resizing; a responsive focus/timing race
  is plausible, **not established**. Resize/responsive handlers and the fixture
  are unchanged; only Editor rail order/icon changed in production.
- The same-source package review passed both native transfer (**75.91 s**) and
  Editor Views; duplicate-main Editor Views also passed. These passes establish
  successful executions, not a fix or a reclassification of the earlier failures.

The bounded source review found no resize-handler change or evidence attributing
these failures to the four presentation lines. Scoped publication retained the
known limitations; no speculative unrelated patch was made. Existing 116/117
comparison, Hosts and native-drag records remain intact even where 118 runs pass.

### Security, dependency and native coverage

Exact-source security-browser/application/dependency checks passed. Independent
review verified seven security reports, current normal/narrow credential pixels,
Terminal/pager outputs, native vault image identity and digest/CRC-bound artifacts.
Go scans found **zero reachable and zero imported-package findings**; the existing
unused module-level OpenPGP advisory **GO-2026-5932** remains disclosed. Full and
runtime npm audits reported zero findings, and the pinned bundle reproduced.

The package-review full Windows job independently passed actual cwd/token tests
(**0.34 s**), explicit helper execution (**0.43 s**), all three real WebView2
bootstrap routes (**10.18 s**), vault captures (**0.34 s**), capture launch-spy
boundary (**0.02 s**), dirty-close consent (**9.77 s**), native editor transfer
(**75.91 s**), Explorer files/folders/cancel (**15.05 s**), Terminal names/gestures/
scoped actions/transport preservation (**23.17 s**) and 4 MiB editor transfer
(**18.70 s**). The helper ran on an already-elevated isolated runner:
**UACInvoked=false**. This does not prove visible UAC consent, an unelevated-to-
elevated transition, cross-account consent or any operation on the user's desktop.
Synthetic/isolated tests are not a claim that live production devices were used.

## Public package and updater proof

The reviewed portable artifact was **12944309399**, from package review:
13,566,113-byte artifact ZIP, SHA-256
`f8f91f4c5e988cd4a8babae44617f966605b243a2667127d90b4aeb268c981c6`.
Its portable bytes and embedded EXE match the actual published downloads exactly.

| Public asset | ID | Bytes | SHA-256 |
| --- | --- | ---: | --- |
| Network_Toolkit.exe | 690441181 | 20,772,864 | `0ebad04808968993cbb3c2d3b5992fe40737d9fb91e1f5ea2a0fb2c0f65f03c9` |
| Network_Toolkit_1.17.118_Portable.zip | 690441184 | 13,753,539 | `8513e9ff05751da6e0fb7d042f2cdce44817b320e5e4c2d9d65ea267c046fa52` |
| Network_Toolkit_1.17.118_Update.zip | 690441179 | 7,946,870 | `464ea4af10b2a1b42deb2ddca7020fbd9b8e99b09dc369a10dde0a55481666ff` |
| SHA256SUMS | 690441178 | 292 | `49b1dd2969a5e3b93e0b66452e3c9fb53ad39e769d5c14e2efd5efce7337275a` |

All four actual downloads were checked against GitHub metadata and checksums.
The portable contains **1,143 exact frozen-source files** and **1,150 members**;
both manifests, ZIP CRC/safe paths and source bytes were verified. The EXE is
AMD64/GUI with version **1.17.118**, all **nine** icon payloads exactly matching
the canonical ICO and **12** PE resources, including the expected manifest.
The updater retains its exact four-file update allowlist. Latest-release feed,
tag/main identity and release body matching the source note were verified.

Prior 117 release **428498861**, tag
`51aa026f07e201979b3adb44d048c66c06508826`, body and original four asset identities,
sizes, hashes, URLs and timestamps remain unchanged. No historical release was
rebuilt, replaced or retagged. This five-document closure changes documentation
only; it does not modify the reviewed/published source package or release body.

## Durable local evidence

The private engineering workspace retains `toolkit118-evidence/` with source
review, focused pixel acceptance, exact job inventories/logs, artifact metadata/
SHA/CRC proofs, package and public proofs, promotion readback and previous-release
comparison. Relevant records include `review1/connect118-acceptance.json`,
`review1/views112-failure-readonly.json`, `review1/security-review/`,
`package-ref/security-review/WINDOWS_REVIEW.json`, `public/proof.json`,
`public/verification.json` and `closure-jobs-*.json`. Those local paths are
provenance notes, not public download links or claims of user-desktop storage.
