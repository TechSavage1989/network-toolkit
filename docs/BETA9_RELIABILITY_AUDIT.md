# Beta 9 reliability audit

Candidate, not published. Preserve the full original eleven-area roadmap.

| Area | Candidate work / acceptance status |
| --- | --- |
| Application interface | Delayed polling cannot revive shutdown; repeated hosted navigation and settings dismissal planned |
| Network diagnostics | Brief/full reply races, interruption recovery and retired metadata cleanup; six-tool synthetic cycles pass |
| Network discovery | Retired discovery filter/job references released; real approved-subnet scanning not performed |
| Graphs and monitoring | Active sample data retained through polling; repeated monitor navigation and existing hosted graph gates required |
| Packet capture | Display polling stops after shutdown; existing packet tests and hosted navigation required; no real capture soak |
| File transfers | Existing full regression/native Windows coverage required; no protocol or permissions changes; native interruption acceptance remains open |
| Text Editor | Repeated navigation covered by new hosted fixture; existing source/sidebar/motion failures retained; no comparison fix claimed |
| Terminal | Not primary scope; existing JS and native ownership/transport gates remain; no live session or user desktop used |
| Reports and results | Retired job-backed views released, saved filters retained; golden report test timezone made deterministic with unchanged hashes |
| Network configuration | Stale success/error replies cannot overwrite newer operation or retry after shutdown; no mutation semantics changed |
| Settings and updater | Preferences untouched; settings dismissal included in hosted fixture; release/package/channel/consent verification required |

## Resource observations

`node --test scripts/reliability-beta145.test.cjs` runs 1,000 sequential snapshots
with six concurrent synthetic tool records and 600 rows per cycle. Job/version,
discovery and list registries return to six retained records, rather than growing
with all 6,000 historical IDs. Measurements emitted as diagnostics include elapsed
milliseconds, process CPU user/system milliseconds, heap and RSS deltas. These are
finite process observations, not a memory-leak proof or live network benchmark.
One cloud sample: 60.2 ms elapsed, 71.4 ms user / 6.9 ms system CPU,
3,994,424-byte heap and 14,155,776-byte RSS deltas; no performance threshold is
inferred from one run. Garbage-collector timing and shared host load affect values.

New browser fixture records Chromium TaskDuration, JSHeapUsedSize, Nodes,
Documents and JSEventListeners before/after 80 navigation transitions, with six
synthetic running tools, cancellation, temporary engine interruption and delayed
shutdown response. It forbids unapproved mutations and external/device requests.
Results pending; do not claim this fixture has passed before hosted execution.

Remaining: real concurrent capture/transfer/scan/output load, physical Windows
multi-hour working-set/CPU/latency soak, network interruption/reconnect and complete
Editor/native SFTP acceptance. Beta publication is not stable certification.
