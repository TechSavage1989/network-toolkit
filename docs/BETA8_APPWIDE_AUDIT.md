# v1.17.144 Beta 8 - application-wide performance and polish audit

## Purpose and scope

Complete unfinished work from the original eleven-area roadmap documented in
docs/DEVELOPMENT_ROADMAP.md. The previous v1.17.140 Beta 4 performance and
v1.17.141 Beta 5 interface phases were partial; a published beta did not
establish that every tool had been optimized. This beta performs targeted
low-risk improvements in shared paths used by multiple tools, not an
Editor-only fix.

### Area-by-area disposition

| Roadmap area | This beta | Exact evidence / follow-up |
| --- | --- | --- |
| Application interface | Improved | web/assets/app-chrome.js avoids redundant accessibility and status DOM writes; web/index.html reuses one filtered list per refresh. Cold/warm native startup timing still pending |
| Network diagnostics | Improved | web/index.html derives list sort keys once per source row; Ping sample summary is streaming and safe for long runs. Live Ping/Trace route/DNS/TCP cancellation load pending |
| Network discovery | Improved | web/index.html skips search-field assembly when the filter query is empty and retains existing typed address matching. Large scan concurrency and physical network load not changed |
| Graphs and monitoring | Improved shared data pipeline | web/assets/result-filters.js calculates RTT extremes without spreading large sample arrays. Existing graph rendering/zoom paths retained. Real-time graph CPU and sustained history profiling pending |
| Packet capture | Reviewed, unchanged | web/assets/packet-capture.js and the packet-capture regression suite remain applicable. High-volume physical capture soak and resource profiling pending |
| File transfers | Reviewed, unchanged | Existing remote-browser/transfer-explorer listing caches remain from Beta 4. SFTP/FTP/FTPS/TFTP interrupted transfer and native cancellation work pending |
| Text Editor | Reviewed, unchanged | Known 390px CodeMirror map-motion transient 3-5px drift is retained in docs/RELEASE143_HANDOFF.md, including failed hosted CI. No test threshold relaxed |
| Terminal | Regression only | Existing output batching, splitting and saved sessions retained. Multi-session real-device load and long-running memory tracking pending |
| Reports and results | Improved | web/assets/report-export.js uses fewer temporary key arrays and a Set for selected columns; exact HTML/text digest regression tested at 3/250/1500 hosts. Large shared lists avoid repeat filtering/sorting during one paint |
| Network configuration | Reviewed, unchanged | Existing IP/adapter/Hosts validation and confirmation behavior preserved. Native rollback/rejection acceptance pending |
| Settings and updater | Reviewed, unchanged | No settings schema, credentials, update feed or install-consent change. The existing protected release packaging and beta feed checks remain mandatory |

The absence of a new code edit in an area is not a claim that its performance
or reliability was certified. Beta 9 should extend the full remaining matrix.

## Reproducible local evidence

Synthetic comparisons on the same source and test host, not a physical
customer Windows benchmark:

| Workload | Previous v1.17.143 | Beta 8 candidate | Interpretation |
| --- | ---: | ---: | --- |
| 5,000-row shared sort, median | About 350.128 ms | About 310.583 ms | About 11% lower synthetic elapsed time; column extractor now called once per row |
| 10,000-row discovery list, empty query | About 11.787 ms | About 0.186 ms | Avoided search token field construction, not a faster network scan |
| 1,500-host HTML report | About 57.438 ms | About 54.311 ms | Slightly lower synthetic render time, identical output |
| Initial list render plus two refreshes | 7 filtered/sorted passes | 3 passes | Shared cached-by-call row set, not persistent stale list cache |
| 50 repeated unchanged app chrome syncs | Repeated ARIA/status writes | Zero redundant attribute writes after initial sync | New host/status values still update immediately |

Measurements are local, synthetic samples and can vary by host, workload and
configuration. Source/UI and report semantics are protected by exact data
comparison, not inferred from elapsed time alone.

## Validation and preservation

- Local complete JavaScript suite: 1,555 tests passed, 0 failed.
- Python release/resource guard suite: 44 passed, 0 failed.
- Windows resource generator produced all 12 expected resources; binary
  resource object SHA matches the generator output; source code verified
  against exact GitHub blobs.
- Focused tests cover stable sort tie-breaking, missing columns, IP/numeric/
  date values, list pass counts, empty/non-empty discovery filters,
  180k Ping samples and 140k monitoring samples without argument overflow,
  byte-identical report outputs and idempotent accessibility updates.
- Go 1.26.10 toolchain and real Windows/WebView2/browser acceptance depend
  on GitHub hosted workflow here, not local Go 1.23 checks.
- Keep scanning and probe concurrency unchanged, preserve list ordering,
  saved settings/credentials, snapshot/report bytes, capture and transfer
  rights and user-controlled update consent.
- Preserve historical Editor 390px CI failures and native SFTP concerns.
  Do not call these repaired by unrelated result-list work.
- Publication requires exact-source GitHub Go, JS, Python, security,
  Windows resource/build, portable source ZIP, binary-only updater ZIP and
  artifact checksum/update-feed verification.

## Next phase

After publishing Beta 8, update the permanent roadmap and release handoff.
The next proposed Beta 9 is extended multi-tool reliability and stress:
simultaneous monitoring, discovery, capture, SSH/Telnet/serial, file transfers,
memory/CPU sampling, cancellation, repeated navigation, window cleanup,
export consistency, and updater recovery. See DEVELOPMENT_ROADMAP.md.
