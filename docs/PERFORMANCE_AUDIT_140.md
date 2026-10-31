# Network Toolkit application-wide performance and polish audit

## Baseline and scope

Starting baseline: published v1.17.139 Beta 3. The next candidate is v1.17.140 Beta 4.
This audit is based on source inspection and CI observations, not an instrumented
physical Windows performance benchmark. A test that proves a call is avoided
does not establish a measured wall-clock speedup.

## Development tracks

| Area | Verified code-level observation | Beta 4 action and measurement |
| --- | --- | --- |
| Time graphs and monitoring | GraphHistory.range sorts the complete timestamp list to find two endpoints; GraphHistory.select always sorts returned samples. | Linear range extrema and skip re-sorting when already chronological. Test out-of-order imports, exact timestamps, invalid entries, range boundaries and comparator counts. |
| SFTP / FTP / FTPS browser | Sorting a directory repeats on row selection, keyboard actions and navigation even with unchanged entries. | Memoize one sorted view per pane/array/sort/filter/reverse combination; invalidate when a listing or view option changes. Test directory-first, natural names, selection and changed connection lists. |
| TFTP / SFTP server explorer | Ordered and filtered views are rebuilt on selection; modified/type comparator repeatedly derives metadata. | Cache ordered/visible listings by pane and options and compute modified/type sort keys once per listing. Test hidden files, changes, sorting and unchanged source rows. |
| Editor | Recent main CI browser-editor112 failure: narrow 390px sidebar control wait timeout, and right comparison-map p95 offset 4.546875 px. | Separate root-cause UI fixes and responsive testing. No unverified claim of resolution in this release. |
| Diagnostics, discovery and result tables | The project has configurable probe execution, sortable lists and large result presentation. | Benchmark scanning/large result lists and cancellation before changing concurrency or result semantics. Preserve approved targets and controls. |
| Packet capture | Packet graphs, maps, list/detail and refresh paths are sizeable interactive modules. | Record refresh frequency, DOM updates, memory and loss under synthetic captures; preserve capture and filter correctness. |
| Reports and exports | Report builders create formatted HTML and values from retained results. | Measure large export and view generation, ensure no truncation and retained data fidelity. |
| App startup, navigation and settings | Separate app chrome, navigation, preferences and window recovery modules are present. | Measure cold/warm startup, page-switch responsiveness and workspace restoration with profiler traces. |
| Terminals | v1.17.137 to v1.17.139 already addressed pane fit, split dragging and output batching. | Include in simultaneous-workload regression tests; no Terminal-only performance priority in Beta 4. |

## Acceptance criteria

- Preserve user-visible behaviors, credentials, permissions, display order,
  selections, history, settings, stable executable name and updater consent.
- Do not change network scan concurrency or packet capture rates without
  benchmark evidence and operational safety review.
- Validate existing JS/Go/Python tests, focused new regressions, Windows x64
  resources/build and browser/native CI. Record CI failures without waiving
  or relabeling them.
- Where available record comparable before/after counts and timings.
- Physical Windows hardware and live-device tests remain necessary before
  stable promotion. Do not claim them based on synthetic CI alone.

## Later polishing stages

Beta 5: consistent compact layouts, sidebar controls, settings and feedback.
Beta 6: prolonged-operation reliability, cancellation/recovery, concurrent
tool workloads, leak detection and Windows acceptance.
