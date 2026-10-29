# 1.17.113 candidate — acceptance pending

User request2026-10-28 22:29:50UTC, build/publication approval22:32:03UTC,
saved-connection addition22:33:25UTC. Baseline is published112 source/main
4d05f6a5a4eb1c315d0a19ccf2eb8daedf2dcb1c, tree
3f9278fe6a5a3b1781d71efbd9354e4decf7ac8e. Preserve its split views, per-view
tabs, comparison maps and approved file explorer; do not restore older110 UI.
The separate Work task completed112 closure402442e877acb29cd410af76e811c213aac65a78;
its five docs-only changes and terminal CI evidence are retained intact.

Scope: remove redundant local-file disclosure without losing operations; keep
More and Connected session compact in the primary toolbar, expand only contents;
explicit saved file-capable connections using established authentication and
approved-window boundaries. No credential/session capability shortcuts.
User additions23:01/23:02UTC: tab-context Compare to (same-window documents)
and adjustable normal split/comparison dividers with consistent interaction.
Further addition23:13UTC: clearly colored editable comparisons, intraline marks,
visual-only alignment gaps with source line counts, and a compact difference key.
Only the comparison renderer uses pinned offline CodeMirror state/view; existing
diff algorithm, document history/leases and ordinary/split textareas remain.
Read [EDITOR_TOOLBAR113.md](EDITOR_TOOLBAR113.md) for the compact controls,
context comparison and divider behavior.
Read [EDITOR_SAVED_CONNECTIONS113.md](EDITOR_SAVED_CONNECTIONS113.md) for the
main/child authentication and explicit renewed-access boundaries.

Required: focused layout/connection/approval/race regressions, all current hosted
Checks (including browser-editor112), actual before/after main/detached/modal
pixels and measured working-space gain, existing native/security/graph/updater
checks, exact package/source/PE identity, expected-head non-force publication and
all public assets/feed/prior-release verification. No local desktop automation,
live devices, private configurations or user files. Do not claim authored tests
passed. Final gate results and precise limits belong in the published closure.
