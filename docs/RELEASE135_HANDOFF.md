# Network Toolkit 1.17.135 - paced output command formatting (candidate)

Based on the published v1.17.134 release and its documentation-only main
handoff, this version updates only the formatting/filtering of the
completed paced multiline SSH terminal capture.

The user supplied a screenshot showing two lines for a submitted command,
one as "Command 439: ...", followed by a separate bare device echo.
The requested behavior is: Include commands enabled shows exactly the
submitted command once and the associated output, without numbered headings.
With Include commands disabled, exclude both commands and recognized echoes
but preserve every command's output and the full paste range.

Implementation only changes pacedOutput() in web/assets/terminal-extras.js.
It uses the exact accepted command text from the existing capture and
skips a matching initial prompt-prefixed or bare echo at most once per
step. It preserves later matching lines (possible legitimate output)
and retains internal output blank lines. Blank commands never get synthetic
"[Enter]" labels. Last paste and Last command both use pacedOutput(),
as do Copy/Select and their common output path. Other terminal capture
paths are unchanged.

Regression tests cover command numbers around 439, bare and prompt-prefixed
echoes, both include modes, 458 synthetic steps, copied output, legitimate
command-like content after the initial echo, and terminal history.

Review PR #10. Run exact source Go/JS/Python, the Windows x64 GUI build,
native/browser/security CI and binary packaging before publication. Prior
v1.17.134 CI observations and earlier Windows/Editor failures are retained.
Do not claim unrelated issues have been fixed.

Publication pending: verify release tag/source, latest updater feed, four
assets and their SHA-256, and unchanged v1.17.134 release/tag/assets.
No customer device was used for testing.
