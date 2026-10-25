# Network Toolkit development

Read DEVELOPMENT_HANDOFF.md before editing or building. Start from current
main and preserve existing changes. Update the handoff whenever behavior,
release destination, validation, known issues or next steps change. Follow
repository checks and release safeguards.

Current work: 1.17.71 review build for SMTP password retention after tests.
Read the first handoff section for exact scope and validation. The password
stays only in the open form and is cleared on switching tools. Never persist
it in app state/settings/history. Keep temporary request options scrubbed.
Preserve earlier features, especially the single final update-install action.

Use review/hosts-editor-1.17.71 for source checks and the portable review artifact.
Main pushes publish new versions automatically. The current request covers the
focused change and review build; prior publication authorization was specific
to 1.17.70. Do not promote 71 to main or publish without a new user instruction.
Published release remains 1.17.70, source/tag
538657ac04477b9192cfcd547f5ddc34d2f3eaed. Never replace earlier tags/assets or
change repository access. Main network-toolkit is the only source/update feed.
Native Windows/WebView2, live SMTP and actual native install/restart are not
verified here. Record completed checks honestly in the handoff and validation.
