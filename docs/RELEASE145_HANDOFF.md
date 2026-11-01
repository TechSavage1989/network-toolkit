# v1.17.145 Beta 9 - published and public downloads verified

Published 2026-11-01T01:47:19Z as opt-in prerelease 430302210:
https://github.com/TechSavage1989/network-toolkit/releases/tag/v1.17.145
PR #20 / tag source: 84c3c53d5c2dcdee13fd04842fafa5c8693679d2.
Tree 782528158ae2609615eae71e5b317294dca6fce6 exactly matches tested candidate
15ddba655d31ca441d32bbf0b35c2be3d935ccf7. Release workflow 41262343931:
build SUCCESS, publish SUCCESS. All four public assets independently downloaded
and verified; beta feed selects 145, stable remains 136. Previous 144 tag,
release body and asset identities/digests remain unchanged.

Both final candidate Checks runs (41260830577 and 41260834361) completed with
14 successful job groups and one failed historical Editor motion group.
Main duplicate Checks 41262343883: completed with 14 successful groups and the same failed Editor motion group.
**Not all CI is green.** The original 2px narrow Editor motion requirement is
unchanged and remains failed; no fix or waiver is claimed. Hosted native Windows,
security, core and changed-scope lifecycle checks passed before publication.
No physical multi-hour Windows/live-device soak or native SFTP interruption
acceptance closure is claimed. Stable certification remains pending.

Continue all eleven areas in docs/DEVELOPMENT_ROADMAP.md. Read
RELEASE145_HANDOFF.md and BETA9_RELIABILITY_AUDIT.md under docs for exact scope,
measurements, preserved failure evidence and next proposed Beta 10 work.
Do not use the user's desktop or overwrite published tags/assets. Credentials,
saved preferences, network permissions and explicit download/install consent
remain unchanged. Historical candidate entries below are retained as history.

## Future release-note presentation preference

User instruction (2026-11-01): keep the updater's **What's New** high-level,
plain-language and about user experience: noticeable improvements, fixes and
workflows. Put implementation details, test metrics and technical jargon under
**Additional Details**. Important user-facing limitations must still be clear
and understandable, not buried solely in technical data. Apply to future
releases; do not rewrite published 145 assets or release notes retroactively.

## Public asset verification

| Asset | Bytes | SHA-256 |
| --- | ---: | --- |
| Network_Toolkit.exe | 21111808 | ce7cda11ae0c509a36bee925f3d7832f70a73d4061fe784d0f85bdd53b834685 |
| Network_Toolkit_1.17.145_Portable.zip | 14479889 | 379593dd7d20470b6e3376a45a338c73868dcbd86eb9c47ef3d8fca9874407a7 |
| Network_Toolkit_1.17.145_Update.zip | 8067202 | be84a6309bc73749912dddf968a12c1243a3f4323ebaaf7301f468617658cce5 |
| SHA256SUMS | 292 | 1f52190e3ea0b13f1de9e6d78129957bd76cad195d388f1cd49938f9a534720b |

GitHub digests and downloaded manifest agree. Both ZIPs pass CRC, member/path,
inner checksum and EXE equality checks. All 1,297 packaged source files match
the tagged source bytes. Updater contains only its versioned EXE, SHA256SUMS,
THIRD_PARTY_NOTICES.txt and UPDATING.txt. Standalone EXE equals the local build;
PE version/resources verified. No downloaded executable was installed/launched.
Previous Beta 8 public EXE/ZIP/manifest bytes also verified against its original
manifest before publication; afterward its release metadata/digests and tag
remain unchanged. Stable 136 release metadata/assets unchanged. The actual public 30-release beta
listing and stable feed were also passed through the unchanged production Go
feed parser in an ephemeral local test: beta 1.17.145, stable 1.17.136 (PASS).
The temporary test was removed; no production updater code changed.

## Final exact-source validation

- Local JavaScript: 1,564/1,564; Python: 44/44. Focused Beta 9: 9/9.
- Local core Go application/Terminal/macros/desktop, Windows cross-build and
  package checks passed using official Go 1.26.10. Sandbox netlink/browser
  restrictions remain local limitations, not passes; hosted core/browser checks
  independently passed on the exact candidate tree.
- Push 41260830577 and PR 41260834361: 14 successful groups each, only
  browser-editor112 failed. Full Windows/native, security/dependency, broad
  browser, graphs/workflows, core and Beta 9 negative-control/lifecycle passed.
- Main duplicate Checks: completed with 14 successful groups and the same failed Editor motion group.
- Exact unchanged Editor main390 p95/max: left reverse 4.140625/4.609375px,
  right forward 4.453125/4.546875px; PR detached right 3.3125/3.65625px.
  Main run 41262343883 reproduces these same three p95/max pairs; artifact
  13065529548 retains the complete Editor evidence. These reproduce baseline
  motion, above the preserved 2px requirement.
  Editor source and geometry tests are unchanged from 144. Sidebar/alignment
  passed the final candidate runs but historical failures are not declared fixed.
- Publication follows the roadmap's scoped opt-in beta policy, not stable
  acceptance. No assertion was relaxed, silenced or relabeled.

## Next continuation

Proposed v1.17.146 Beta 10, not started: continue all eleven areas with extended
concurrent-operation/native interruption and repeated-window evidence; diagnose
narrow Editor motion/sidebar/alignment without loosening thresholds. Obtain
approved independent physical Windows/live-device evidence when available;
no user desktop route here. Further betas or RC only as evidence supports.

## Retained candidate history (superseded by publication above)

# v1.17.145 Beta 9 - concurrent lifecycle reliability candidate

NOT PUBLISHED. Exact-source hosted validation and release verification pending.
Branch: beta/1.17.145-concurrent-reliability.
Baseline main: 93d6fa1997316c4c5245eec25993f715aed602c4.
Baseline published beta: v1.17.144 / 866b64ef49cfd2ef40d7b4c8a5fa2aa24f1f3397.
Stable channel: v1.17.136. Owner and connected identity: TechSavage1989.

Read AGENTS.md, docs/DEVELOPMENT_ROADMAP.md, docs/DEVELOPMENT_HANDOFF.md,
this handoff and docs/BETA9_RELIABILITY_AUDIT.md. Preserve all eleven areas.

## Implemented scope

- Ignore in-flight brief/full diagnostic replies after shutdown; do not restore
  retired full-result metadata or repaint closed workspaces.
- Release version, discovery and job-backed view/filter references only after
  a successful snapshot confirms a previously known job has retired. Preserve
  saved preferences, live jobs, unrelated/synthetic probe views and offline data.
- Ignore obsolete configuration status/error replies after a different operation
  replaces them or shutdown completes. This does not cancel or retry mutations.
- Stop scheduling capture display refreshes after shutdown. No capture policy,
  permissions, network transport or session ownership changes.
- Add race/retention regression tests and synthetic CPU/heap observations;
  add hosted full-application repeated navigation/interruption/cancel checks.
- Pin the pre-existing Beta 8 report golden-byte test process to UTC, matching
  its original unchanged hashes. Production local-time formatting is unchanged.

## Baseline evidence and limits

Rechecked main CI 41250498652: 13 successful job groups, browser-editor112 failed.
The failed group has sidebar resize (>300 expectation), source alignment (10px
observed top offset), and continuous 390px map motion failures. Main motion
p95/max 4.140625/4.609375 and 4.453125/4.546875; detached 3.3125/3.65625.
The original 2px requirement is unchanged. No Editor fix or waiver is included.

Local baseline JS: 1,554/1,555; the report hash mismatch came from the runtime's
America/New_York timezone versus UTC golden bytes. All nine focused Beta 9
checks pass. Original Beta 8 fails the shutdown, stale configuration and retired
cache assertions. The new cleanup API naturally does not exist in Beta 8.

Official Go 1.26.10 is available in temporary cloud tooling. Baseline application,
Terminal and desktop package checks passed. Local diag/deviceinfo interface checks
are blocked by sandbox netlink permission, not passes. Local browser launch is
blocked by sandbox socket permission; no retries or desktop execution are used.
Hosted exact-source browser/native Windows checks are required.

No physical Windows multi-hour soak, live device benchmark, native SFTP acceptance
closure or updater installation is claimed. Prior release assets/tags are immutable.

## First-candidate hosted evidence

Draft PR #20: https://github.com/TechSavage1989/network-toolkit/pull/20.
First remote source 61788415a6a00532672194f99d4ed7e371aa9b0b has tree
9742e8a83a5f0091a5659a41e48cc3b48280f716, verified equal to local source.
New lifecycle browser check passed in both initial push and PR runs.
Core/security/dependency/native isolated groups passed where completed; the broad
browser and full Windows groups were still running at this record.
Full local JS was 1,563/1,563 and Python 44/44; core Go, Windows cross-build and
portable/update package verification passed. These are not native Windows claims.

Artifact inspection found a pre-existing footer still displaying 1.17.138 despite
the current version constant. Final candidate corrects both static version labels
and adds a metadata regression (nine focused tests). Browser instrumentation now
adds a post-GC observation and a pinned original Beta 8 shutdown negative control.
These meaningful additions require a new exact-source CI run, not a blind rerun.
