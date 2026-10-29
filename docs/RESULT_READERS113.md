# Concise result readers (113)

Ordinary result Details present one labeled sample field set. Unique TTL/hop-limit,
protocol, error and extra recorded fields remain. Copy details is readable text;
Copy JSON and report exports are explicit structured-output actions, unchanged in
source content. Raw record, raw result and complete-host object-dump panels are
removed. Generic objects are escaped labeled fields, arrays are readable entries,
and nested content is bounded (200 list entries, 12 levels, 2,000 nested nodes)
with an explicit export notice rather than silently truncating source data.

Audit scope: Ping/Monitor graph and list details; trace/TCP/DNS/quick/identity
results; HTTP/TLS summaries; Discovery/ARP probe details; local adapter/route/socket
readers; SMTP and transfer results. Redundant HTTP metrics, TLS validation details
and quick-check hostname subtrees are consolidated. Unknown extra recorded fields
in specialized readers remain under Additional information. Implementation-only
request/job/event/window identifiers and underscore-prefixed UI bookkeeping are
not ordinary facts; interface/process IDs and protocol evidence remain.

Terminal output, configuration editor, packet protocol/hex/ASCII inspection,
HTTP body preview, system command output, genuine activity/error logs and explicit
structured exports remain intentional technical tools. Transfer/terminal/macros,
utilities, saved targets and report commands were inspected, not broadly redesigned.
No transport, credentials, probe execution, row selection or export source changed.

Validation: focused renderer/selection/copy tests, existing graph selection/browser
regressions adapted to visible labeled fields, and result-readers113.browser.cjs.
The hosted fixture requires exact published112 via NTK_UI_BASELINE_DIR (or
NTK_TOOLBAR_BASELINE_DIR) and records synthetic light/dark 1440/390 before/after
pixels, scrolling measurements, exact JSON copying and diagnostic-reader inventory.
Authored browser checks are not execution evidence until the exact-source hosted
run passes. Private user screenshots/configurations are never test fixtures.
