# 1.17.107 candidate handoff

## Baseline and authority

The user explicitly approved the attached ten-section specification as a single
implementation/build/push/publication batch after asking to confirm understanding.
Current maintained baseline was rechecked before editing: main
`a3a611681eb6194263cdb15facfd540826398735`, tree
`538d50bbc8c592a6d821d2b5679a8031ee998806`; published106 source/tag
`b8be5353877911fa422e69880f6b7363cda47509` and release426132432.
No concurrent build was active. Isolated integration preserves published tags,
assets, repository access and the existing update feed/workflow.

## Implemented scope

1. Exact saved terminal-profile metadata badges in relevant discovery/device
   results, explicit chooser/review, live profile updates, protocol/port and
   scoped-IP validation. No saved-credential/active-session inference or connect.
2. Shared header source button removed. Sidebar controls, validation/settings
   and immutable recorded-source disclosure below results remain.
3. Named tool-scoped filter CRUD with explicit duplicate/replace/delete handling,
   bounded sanitized persistence, existing multi-value query/status/type semantics
   and no scan/connection side effects.
4. Conservative deterministic highlighting, HPE-prose negative controls,
   precedence and per-rule preview/settings; device ANSI and terminal data remain
   intact. Prior106 engine reproduces the ordinary-prose misclassification.
5. Expanded local monospace candidates, fallback/detection notice and font-ready
   terminal refitting through existing guarded resize paths. No font installation.
6. One shared transactional color picker across relevant settings, validated
   HEX/RGB(A), copying, presets/recent/saved colors, Apply/Cancel/reset and bounded
   nonsecret cross-window palette preferences. Explicit custom themes survive.
7. Ordinary capture adapter inventory is unelevated; exact GUID resolution and
   helper launch happen only for explicit capture Start. Existing restricted
   helper identity checks remain. Cancellation is visible, not automatically retried.
8. Clearer explorer navigation and reviewed browser-granted file drops, bounded
   staged local/SFTP publication, explicit collision/cancel/partial failures,
   preserved copy locations and legacy Add by local path.

## Intentional compatibility and evidence limits

FTP/FTPS remote Explorer drops fail closed because FTP has no portable atomic
no-clobber rename guarantee. Existing manual FTP transfers remain available;
shared FTP server drops are local confined copies and work through that path.
SFTP overwrite publication requires the supported negotiated atomic-replace
extension; unsupported capabilities fail closed. Folder/shortcut drops are
rejected before any partial import. File drop/picker batches are bounded to
100 files / 512 MiB each; existing explicit local-path operations remain separate.

Only synthetic profiles, credentials, transcripts and files are used. No live
capture, user device, real user vault/settings/file transfer, system security
configuration or font installation occurs during validation. Browser DPR cases
at1/1.25/1.5/2 are not actual Windows display-scaling validation. The native
Explorer/WebView fixture tests the hosted runner at its current environment.
Color contrast notices are advisory, not an accessibility certification.

## Gates at candidate preparation

Final local aggregate: 859/859 JavaScript tests and 42/42 Python release/layout
guards passed; all five new browser fixture syntax checks, release version/resource
validation and diff checks passed. Independent bounded review closed the missing-
adapter fallback, nested local destination replacement and unowned remote staging
cleanup edges; dedicated regressions are included. Exact hosted source/run IDs
will be recorded after source freeze. The local environment
has no usable required Go1.26.10 compiler; Go/native/browser execution is assigned
to hosted checks. Known local Chromium restrictions were not bypassed.

First hosted review a9f4e080 (Checks40615764044) built successfully and passed
application, dependency and unchanged security-browser gates, but is rejected for
publication: new browser checks exposed keyboard focus and cramped graph-toolbar
issues, plus fixture endpoint selection, image routes, Windows canonical-root and
pending-request timing errors. Corrections preserve the original assertions and
require a new exact-source run. Detached fit acceptance remains under diagnosis;
real Explorer was not reached after its Windows prerequisite failed. Independent
legacy browser scripts now continue collecting evidence after an earlier failure
without changing the job result or removing any assertion/timer.

Required hosted Checks retain application, dependency, Windows, security browser
and full browser/updater gates, with a sixth browser-workflows107 job for the new
features. Native Windows acceptance explicitly enables real Explorer mouse drag
into installed WebView2 with synthetic files; unavailable required desktop proof
is a failure, not a skipped pass. Existing native vault/DPAPI/bootstrap checks
and real five-minute updater timer remain unchanged.

Before main promotion: inspect all exact-source jobs, real normal/narrow and
native screenshots, reviewed package/source/EXE identity; report scope, limits
and results. Fresh main lease/release metadata must match. Promotion uses the
established non-force expected-head workflow only. After Release: independently
download all public assets, verify hashes/manifests/ZIP safety/source bytes/icon/
PE/version/update allowlist/feed, preserve prior106 metadata, and observe the
already-running duplicate-main checks before docs-only closure. No optional
reruns or old release replacement.
