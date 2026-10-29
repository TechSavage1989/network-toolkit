> Current114 calibration: the default condensed recent spacing now matches the
> 10-second normal timeframe. See [Graph navigation114](GRAPH_NAVIGATION114.md).
> Earlier calibration measurements below remain historical release evidence.

# Condensed RTT history and theme policy

## Fixed recent-time geometry

Condensed RTT diagnostics use a fixed age horizon `H`: the explicit range duration,
the chosen Timeframe duration, or `3 × recentMs / (2 × ln(4))` for All
(default approximately 64.921 seconds when the history-detail scale is 60 seconds),
divided by the existing zoom factor. `H` never depends on total sample count or
run duration. Given age `a` from the displayed right timestamp, for `0 <= a < H`:

`x = plotLeft + plotWidth × (1 - log(1 + 3a/H) / log(4))`

The inverse is `a = H × expm1((1-share) × log(4)) / 3`. Axis labels, points,
pointer hit testing, pan and anchored zoom share this projection. The overview
remains proportional to real elapsed time, not this nonlinear display.

No whole-history Fit warm-up exists. The newest sample starts at the right
boundary; sparse histories leave genuinely unused past space on the left. With
unchanged controls, usable width and sample cadence, recent spacing is identical
as the run grows. Explicit zoom/timeframe/viewport/cadence changes can change it.

The Y-axis label gutter is fixed outside the plot, so changing RTT magnitude does
not gradually shrink its width. Grid, clip rectangle, data coordinates and hit
testing share the same plot boundaries. Marker rendering room outside that
rectangle keeps endpoint circles/strokes/highlights visible without moving their
centers inward. The oldest cluster's center is the visible left grid boundary;
the newest point's center is the right grid boundary, not a grouped centroid.

## One historical cluster, not growing buckets

RTT points older than `H` merge into one fixed-footprint historical marker. It is
not an individual RTT sample. Its horizontal anchor does not depend on its count,
mean timestamp, represented interval or number of original contributors.

The raw storage policy remains bounded: completed ICMP/TCP-monitor runs retain at
most 10,000 raw samples, and live jobs expose at most 2,200 activity events (some
are not samples). Before either window trims an original sample, an independent
run-local prefix summary accumulates its contribution. The live summary is
`job.sampleHistory`; the completed summary is `result.data.history`. Each must be
paired only with its corresponding retained rows; merging both double-counts.

The summary contains counts, successful/measured/lost counts, measured RTT sum,
minimum/maximum, zero count, first/last recorded timestamps and sequences, valid
consecutive-pair jitter sum/count/extrema and the last pair boundary. Failed,
invalid or nonconsecutive measurements do not become zero RTT or invented jitter.
Status counts retain at most 16 labels plus `other`; unknown time counts remain
explicit. There is no indefinitely growing raw array in the summary.

The renderer combines that prefix with retained originals old enough for the
cluster, without changing recent coordinates. When the selected horizon exceeds
available raw retention, a wholly included prefix still belongs to the one left
archive cluster even if retention forced summarization earlier than the ideal
age boundary; details explain this. Exact raw positions cannot be reconstructed.
Min/max bars and Y bounds include represented extremes rather than averages alone.

## Availability, selection and persistence

The cluster's details identify its recorded time span, total/unsuccessful count,
minimum/average/maximum RTT, available retained contributors and summarized-only
count. Selecting it selects its summary, not an arbitrary original measurement.
Retained originals and held original selections remain accessible through the
existing results/details interfaces. JSON/HTML/text exports retain the applicable
history summary and retained rows without pretending discarded rows exist.

A chosen interval that only partly overlaps an archived prefix cannot recover
exact subset statistics; the UI explicitly reports unavailable archived detail
instead of inventing it. Arbitrary filters cannot exactly classify discarded raw
records, so summary/filter scope is stated rather than fabricating matches.
Legacy/imported runs without the new prefix cannot recover already discarded
history from lifetime totals alone. Their missing-summary limitation is explicit.

New/cleared runs use separate state and cannot inherit another host/run's prefix.
Summaries serialize with normal job/result/export data and follow existing session
lifetime; this is not new restart recovery for a run the app itself did not save.
Packet traffic keeps its existing published projection/bucket semantics; the RTT
archive is not silently applied to unrelated packet counts. Linear retains actual
time-proportional spacing and deliberate ranges without Condensed aggregation.

## Theme behavior

RTT traces, successful RTT markers and RTT indicators use the existing `--accent`
CSS token. They repaint when the accent or light/dark theme changes, without a
new run. Failure/loss/warning states, selection highlighting and jitter/packet
semantics retain their distinct styles. The minimal index fallback uses the same
RTT token and leaves failed-probe markers unchanged.

## Focused evidence

Required acceptance is the actual rendered path, not helper formulas alone:
normal/narrow windows with deterministic arrivals through sparse startup, former
warm-up boundaries, raw rollover and a million-sample bounded summary; equal
recent positions, actual visible-grid endpoints, changing Y labels, resize,
summary/source selection, mixed values/failures and live theme changes.
Backend tests cover one million arrivals without a million-element raw array,
live activity trimming, metadata/statistical integrity, snapshot immutability,
JSON restoration and run reset. Existing graph/navigation/packet and release
gates remain enforced. See DEVELOPMENT_HANDOFF.md for exact completed versus
pending evidence, source and release status.

## 1.17.104 newest-point reference

For All at 1×, the default Condensed newest slope equals a time-proportional
30-second Linear view on the same usable plot width. A finite one-second gap
is about 2.24% smaller because the mapping continuously compresses toward the
left. Explicit timeframes/ranges and zoom keep their existing duration behavior.
The history-detail setting is not a promised recent-window width. Duplicate or
irregular timestamps preserve their actual timing; no minimum pixel spacing is
fabricated. Actual browser normal/narrow rendered comparison remains a required
release gate, alongside growth, retention rollover and archive-cluster checks.
