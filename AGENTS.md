# Network Toolkit development

Read DEVELOPMENT_HANDOFF.md before editing or building. Start from current
main and preserve existing changes. Keep the handoff current and follow
repository checks and release safeguards.

Current work: 1.17.73 hides terminal command-input scrollbar tracks in compact
and expanded modes while preserving scrolling, editing and execution. Read the
first handoff section. The user explicitly requested fix and publish for this
version on 2026-10-25; no further approval is needed after validation.

Use review/hosts-editor-1.17.73 for prepublication checks and portable artifact.
Main pushes publish new versions automatically. Main network-toolkit is the
sole source/update feed. Never overwrite earlier tags/assets or change access.
Published baseline remains 1.17.72, source/tag
3ca1db2fb52132e36a7610e006cfb606e6d9a973, until this new release.

Preserve session folders, protected credential links, macro bindings, the 71
SMTP password lifetime and the single final updater install action. Record
completed checks, official hashes and live updater staging in the handoff.
Native Windows/WebView2, hardware and native install/restart remain unverified.
