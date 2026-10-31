# Network Toolkit 1.17.131 - terminal input and multi-session performance release candidate

User authorized preparing and publishing a completed update. Version 1.17.130
remains the actual public release until the 131 release workflow and external
public asset verification pass. Code PR #5 was merged on main as
80e925494521c5dd8c73a5838341b1b4e6881fbe without moving previous tags.

Ctrl+A selects the guarded editable command range excluding prompt/output,
including wrapped input; unsafe or unknown contexts retain control-key delivery
to the connected terminal. The performance scope limits redundant UI work:
one-frame toolbar updates, delayed cosmetic output-only chrome refresh, hidden
highlight scan suspension, no-op fit avoidance and direct splitter-track CSS.
No change to transport, credential, permission, existing transfer ownership,
stored application settings or updater behavior is authorized.

The source core passed 1,488 JavaScript, 42 Python, Go and Windows x64 build
checks. Source push 41164495366 passed the scoped compact Editor sidebar check,
whereas same-source PR 41164500428 recorded a 390px Editor gear-focus timeout.
The root cause has not been established. No unrelated Editor bug is claimed
resolved; preserve both original observations and their artifact evidence.

Versioned acceptance remains pending until all relevant new exact-source CI
jobs complete and the publication is independently verified against actual
public bytes, source/manifests, PE/Windows version/icon resources, the strict
four-member binary updater, latest feed/tag and previous130 immutability.
Synthetic browser and loopback-device coverage is not evidence of universal
remote CLI handling or measured user-hardware performance gains.
