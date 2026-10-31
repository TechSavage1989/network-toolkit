# Network Toolkit v1.17.144 Beta 8 - application-wide performance and polishing

Status: DEVELOPMENT CANDIDATE. NOT PUBLISHED.
Starting point: v1.17.143 Beta 7, published at cdef07de894b68b472f083bf3082fb905c7ffa9e, plus documentation-only main commit ea6ba1ef369de0a344114c0d1306895181748a99.
Branch: beta/1.17.144-application-wide-performance.
Authoritative continuing plan: docs/DEVELOPMENT_ROADMAP.md.

This beta reviews every one of the eleven roadmap areas and implements measurable changes across multiple tools. It is not an Editor-only or Terminal-only beta. Record measured improvements, regression results, code paths and deferred work here before publication.

Preserve settings, credentials, scan/network permissions, terminal transports, capture accuracy, report data, stable Windows EXE name, opt-in beta updates, all older releases and their checksums.
Run full JS/Go/Python, browser/native Windows, security, resource, build, packaging and feed checks. Disclose any remaining Editor 390px motion and native SFTP cancellation regressions. No physical Windows or live-device certification is claimed without testing.

## Implementation and measured evidence

The full eleven-area disposition and reproducible before/after measurements
are in docs/BETA8_APPWIDE_AUDIT.md. This beta changes shared result list
sorting/refresh, network discovery empty-query work, Ping/Monitor summary
aggregation, report export and application status chrome. Other tools remain
in the phase plan with a documented deferred acceptance status.

Code changed: web/index.html, web/assets/result-filters.js,
web/assets/report-export.js, web/assets/app-chrome.js,
scripts/performance-beta144-appwide.test.cjs, version metadata and
Windows COFF resources. Source changes are kept small to avoid a broad
visual or network-transport redesign.

Local complete JavaScript suite: 1,555 passed, zero failed.
Local Python resource/release guards: 44 passed, zero failed.
Windows resource generation: 12 resources; verified source/binary SHA.
Local synthetic measurements: five-thousand-row sorting about 350ms
to 311ms median; unfiltered ten-thousand-row discovery 11.8ms to
0.19ms; fifteen-hundred-host HTML report 57.4ms to 54.3ms
with byte-identical output. Counts: one derived sort key per row;
one list row-set per initial render/refresh; no redundant ARIA writes.
These are code-local observations, not physical Windows benchmarks.

Known risk register retained: narrow Editor comparison-map transient
3-5px movement over its existing 2px expectation, historical interrupted
SFTP cancellation checks, and no sustained real Windows/network-device soak.
Beta 9 stress testing and stable promotion remain separate.

## Continuation after this beta

Keep docs/DEVELOPMENT_ROADMAP.md as the authoritative roadmap. After
publication, prepend a verified release closure here and in
docs/DEVELOPMENT_HANDOFF.md and AGENTS.md, update the README,
mark v1.17.144 published in the roadmap, and retain the full Beta 9
concurrent workload matrix for the next chat.

PUBLICATION PENDING.
