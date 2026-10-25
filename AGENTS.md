# Network Toolkit development

Read DEVELOPMENT_HANDOFF.md before editing or building. Start from current
main and preserve existing changes. Update the handoff whenever behavior,
release destination, validation, known issues or next steps change. Follow
repository checks and release safeguards.

Current work: 1.17.72 folders for saved terminal and FTP/FTPS sessions, similar
to macros. Read the first handoff section for scope and validation. Keep folder
labels as bounded nonsecret profile metadata, outside connection payloads.
Preserve IDs, protected credential references, macro bindings, the 71 SMTP
password lifetime and the single final updater install action.

The user explicitly instructed build and publish for this feature on
2026-10-25. No further approval is needed for the scoped 72 source/release
once validation passes. Main pushes publish new versions automatically.
Use review/hosts-editor-1.17.72 for prepublication checks and portable artifact.
Main network-toolkit is the only source/update destination. Never replace old
tags/assets or change repository access. Published baseline remains 1.17.71,
source/tag 0b3ffe7cbc12be9778429651507b442f33073aa1, until the new release.

Record completed checks and publication evidence in the handoff. Native
Windows/WebView2, live hardware and native install/restart remain unverified.
Do not treat isolated browser fixtures as native runtime proof.
