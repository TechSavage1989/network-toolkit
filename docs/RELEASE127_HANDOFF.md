# Network Toolkit 1.17.127 — paced paste and device labels (candidate)

Not yet published. Hosted browser, Go/Windows, security and package checks remain
pending; local checks alone are not release acceptance.

## Terminal behavior

Paste review offers literal one-line-at-a-time sending for complete SSH command
lists at a recognized empty prompt. It preserves indentation, blank lines and
template-looking text. The selected session generation and ownership revision
remain fixed. Limits are 64 KiB and 1,024 lines; incomplete final lines and terminal
control characters require raw paste or explicit editing. No Enter is silently
added to unfinished input.

The runner waits for fresh line-boundary output ending in a prompt that remains
stable for 100 ms, with a 30-second timeout. Prompt recognition is a heuristic,
not proof that a device accepted a command. No automatic pager answer is sent.
Pause and Stop are available in Runs, but cannot undo commands already sent.
Pending and uncertain starts cannot silently submit again across review closure.
Raw, single-line and bracketed paste and existing explicit settings remain intact.

Tabs, Saved and Recent show a recognized CLI device name above a smaller endpoint.
Without a recognized name, only the endpoint is shown. Config-mode suffixes are
excluded. A configured hostname is shown honestly when no IP is available.
Stored custom names, connection addresses, credentials and recent-session identity
are preserved. Learned display metadata is bounded; single-session transfers carry
only that session's validated name, not the complete name registry.

## Evidence and limits

Combined source f25fcd2 passed 1,404 JavaScript tests and 42 Python tests locally.
Real browser/Windows execution and final packaged-source checks are pending.
Tests and captures use synthetic data only; no live client configuration is sent.

Whole-block SSH paste can interleave device echo and prompt output. The original
remote byte stream was not captured, so no font/xterm defect or complete original
root cause is claimed. Paced paste mitigates that workflow rather than rewriting
received terminal text. Earlier editor comparison/native-transfer/UAC limitations
remain; this release does not claim unrelated fixes or universal compatibility.

Existing repository visibility/access and all prior release tags/assets remain
unchanged. Final exact-source CI and publication evidence will be recorded before
release promotion.
