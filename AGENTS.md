# Network Toolkit development

Read DEVELOPMENT_HANDOFF.md before editing or building. Start from the current
main for new work and preserve all existing fixes.

Published release: 1.17.82. Version 1.17.83 is implemented, built and validated
on review/hosts-editor-1.17.83, awaiting publication authorization. Read the
first handoff section. Preserve terminal selection/prompt guards, settings gear,
Hosts/Overview/copy, terminal/sidebar/detach/Connect, editor, folders/credentials/
macros, command input, SMTP and updater protections, plus all 83 features.

Future releases need their own authorization. The 2026-10-26 implementation
request did not explicitly authorize 83 publication. Main pushes publish new
versions automatically. Main network-toolkit is the sole public source/update
feed. Never replace older tags/assets, change access, or write the retired feed.

Exact tested 83 runtime/source: a384a47d5b8782fb821eda492fa9ad92a3ca540d
Tree: ebec06684689f0ad48cfcd8e5661c6da4a740232
Checks 40258417658 passes all three jobs. Detailed Windows shell and progress-window
logs were inspected; independent steps prevent masked command failures. All
713 packaged source files match; review artifact 12623039560 and local
portable/EXE bytes match. Browser artifact 12623776589 and screenshots verify.

This three-file final review record uses [skip ci] and changes no runtime files.
If the user authorizes publication, promote the exact tested runtime commit,
verify the official assets/feed, then update final publication facts. Main remains
12156a2897604bc7c78292f76c3aed6c9f227d29 with unchanged 82 release assets.
Full native WebView2 interaction, hardware and actual install/restart remain
unverified. Keep the handoff current and report checks honestly.
