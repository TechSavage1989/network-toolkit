# Beta 9 reliability audit

Published as scoped opt-in v1.17.145. Preserve the full original eleven-area roadmap.
Final validation and limitations supersede the planned candidate coverage below.

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
with six concurrent synthetic tool records and 600 rows per cycle. The synthetic job/version, discovery and list registries return to six retained
records. Production list rendering also has pre-existing count caps; this test
is a cleanup invariant, not a claim that all production registries were unbounded. Measurements emitted as diagnostics include elapsed
milliseconds, process CPU user/system milliseconds, heap and RSS deltas. These are
finite process observations, not a memory-leak proof or live network benchmark.
One cloud sample: 60.2 ms elapsed, 71.4 ms user / 6.9 ms system CPU,
3,994,424-byte heap and 14,155,776-byte RSS deltas; no performance threshold is
inferred from one run. Garbage-collector timing and shared host load affect values.

New browser fixture records Chromium TaskDuration, JSHeapUsedSize, Nodes,
Documents and JSEventListeners before/after 80 navigation transitions, with six
synthetic running tools, cancellation, temporary engine interruption and delayed
shutdown response. It forbids unapproved mutations and external/device requests.
First candidate 61788415a6a00532672194f99d4ed7e371aa9b0b passed the hosted
80-transition fixture on PR run 41260498266/job 124712587536 with no page errors.
Its raw heap changed 6,234,640 -> 16,967,012 bytes and DOM nodes 1,891 -> 119,979;
TaskDuration was 13.569118 seconds. These pre-GC values include temporary garbage
and do not establish retained-memory growth or a leak clearance. The final fixture
adds an explicit post-GC observation and runs unchanged tagged Beta 8 as a control,
which must reproduce the delayed-shutdown race while the candidate must reject it.
Final control and post-GC results are pending hosted validation.

Remaining: real concurrent capture/transfer/scan/output load, physical Windows
multi-hour working-set/CPU/latency soak, network interruption/reconnect and complete
Editor/native SFTP acceptance. Beta publication is not stable certification.

## Final hosted controls and resource observations

Final candidate 15ddba6 / released tree 7825281: push 41260830577 and PR
41260834361 each have 14 successful groups and the unchanged Editor motion
failure. Main duplicate: completed with 14 successful groups and the same failed Editor motion group. Full Windows/native, broad
browser/graph, security/core and new lifecycle checks passed before publication.
All eleven areas remain in scope; live-network/physical acceptance is not implied.

The hosted full app completed 80 navigation transitions across 16 pages with six
synthetic jobs, cancellation, interruption/recovery and settings dismissal, with
no browser errors. Original 144 reproduces the delayed shutdown race; 145 rejects
that delayed reply. Controlled push-run post-GC observations:

| Observation | Original 144 | Candidate 145 |
| --- | ---: | ---: |
| Used JS heap bytes | 9,226,092 | 9,210,168 |
| DOM nodes | 38,946 | 38,946 |
| Event listeners | 492 | 492 |
| Browser TaskDuration seconds | 8.753694 | 8.701477 |

These are finite comparable synthetic observations, not proof of memory-leak
absence, speed improvement or multi-hour endurance. Raw heap growth without GC
was not interpreted as a production leak. The 1,000-batch six-tool local fixture
retains six active registry records; saved preferences and unrelated probe views
are preserved. Public portable source/EXE/updater/manifest all independently
verified; details and open Editor/native/physical gates are in RELEASE145_HANDOFF.
