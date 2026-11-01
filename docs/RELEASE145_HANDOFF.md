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
