# Network Toolkit development

Read DEVELOPMENT_HANDOFF.md before editing or building. Start from current
main and preserve existing changes. Update the handoff whenever behavior,
release destination, validation, known issues or next steps change. Follow
repository checks and release safeguards.

Current release: 1.17.71 retains the SMTP password for repeat tests and clears
it on switching tools. Keep it only in the open form, outside app state,
settings and history. Temporary request options must still be scrubbed.
Preserve all earlier features and the single final update-install action.

The user authorized this specific 71 source upload and publication on
2026-10-25 after the review build. Future releases need their own authorization.
Main pushes publish new versions automatically. Main network-toolkit is the
only source/update destination. Never replace old tags/assets or change access.

Published source/tag: 0b3ffe7cbc12be9778429651507b442f33073aa1.
Full runtime Checks 40163863165 passed on source 90433f9, with only subsequent
documentation changes before publication. Release 40164554695 passed. All
four official downloads, all 677 packaged source files and unchanged released70
live feed/download/staging are verified. Ten older releases/tags are preserved.
See the first handoff section for exact hashes, provenance and validation limits.
This final documentation follow-up must not change or republish the runtime.
Native Windows/WebView2, live SMTP and actual native install/restart remain
unverified here. Do not treat mocked browser tests as native runtime proof.
