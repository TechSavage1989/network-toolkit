# Network Toolkit development

ACTIVE HOLD: The user explicitly forbids a new application build or publication
until they authorize it. Source fixes and source-only tests are authorized.
Work is isolated on fix/editor-shell-review-no-build. Do not push main, bump
versions, run build/package scripts or trigger the normal build workflows. The
branch-specific source test workflow runs JS and static browser fixtures only.

Read DEVELOPMENT_HANDOFF.md before editing or building. Start from latest main,
preserve unrelated work and all previous fixes, and follow release safeguards.

Current published release: 1.17.85. Terminal Display contains independent local
shell syntax controls and saved close confirmation; collapsed main tool icons
navigate without opening the rail. The first handoff section has full behavior,
verification, exact artifacts, source and limits. Preserve the 84 editor's 300
language choices / local formatting, and all earlier runtime/UI/update fixes.

The user explicitly authorized publishing 85 when complete. It is published and
verified. Future releases need their own authorization. Main pushes publish new
versions automatically. Main network-toolkit is the sole public source/update
feed. Never replace old tags/assets, change access or write the retired feed.

Published source/tag: 427e50d765835f632f44ca4a9b185241169dd91c
Tree: 358b647ddf32872ab6a6ca5a5bd9ee185790da54
Full Checks 40271392085 and Release 40272271825 pass.
503 JS / 31 Python guards, required Go suites, native Windows shell/progress and
all browser steps pass. All four public assets and 741 source files verify
and equal the reviewed build. All 24 older releases/assets/tags are unchanged.
Released84 handlers verified/staged live85 in 10.41s with isolated settings,
original test EXE unchanged and no installation/restart.

This final three-file docs record uses [skip ci]; do not move the 85 tag, replace
assets or republish. Full native WebView2 interaction, hardware and actual update
installation/restart remain unverified. Keep the handoff current and be precise.
