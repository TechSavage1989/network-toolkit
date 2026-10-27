# Condensed graph scaling

## Stable recent resolution

Condensed maps actual elapsed time, not result indices. Let `R` be the existing
**Condensed history: keep individual results for the latest (seconds)** setting
(default 60 seconds), `S` the displayed time span, and `a` a sample's age relative
to the displayed right endpoint. The plotting width excludes the existing axis
inset and 12-pixel right gutter.

Once `S >= 3R`, the latest `R` occupies the right half of that usable width. Its
mapping is independent of `S`. With the same viewport and sampling cadence,
adding old history or new arrivals does not squeeze recent samples. Irregular
intervals remain irregular; duplicate timestamps share a coordinate. Changing
`R`, viewport width or the actual sampling cadence intentionally changes pixels
per sample. A changing Y-axis label width can also slightly change usable width.

The older half alone adapts as retained history grows. Define distance from the
right edge, normalized to the usable width:

- For `a <= R`, `d = log(1 + a/R) / log(4)`.
- For `a > R`, `d = 1/2 + beta * log(1 + (a/R - 1)/beta) / (2 log(4))`.
- `beta` is the unique positive solution of
  `beta * log(1 + (S/R - 1)/beta) = log(4)`.

At `a = R`, both regions have the same position and first derivative: there is
no abrupt change of speed. The derivative decreases with age, so older time
intervals become progressively denser. At `a = S`, `d = 1`; at `a = 0`, `d = 0`.
The oldest representative uses its original first timestamp, not its group's
midpoint. Fit therefore pins both endpoints to the same plotting boundaries,
including raw/group transitions. The existing marker/stroke gutters remain.

A bounded 64-entry layout cache avoids repeatedly solving the same curve for
each mark. Only a new visible span/recent setting requires the small monotone
scalar solve. Projection, inverse axis ticks, pointer-anchored zoom and direct
plot dragging use the same mapping. The full-history navigator remains linear
in recorded time, as do keyboard/API time-step pans.

## Short histories and exact ranges

For `0 < S < 3R`, Fit fills the available width with
`d = log(1 + 3a/S) / log(4)`. This warm-up meets the stable mapping exactly at
`S = 3R`, including the age derivative. A one-point/equal-timestamp history is
centered; an empty history creates no point. Two distinct points pin both edges.

There is a physical limit: while a short history first fills the graph, constant
pixel spacing and both pinned endpoints cannot all hold as the number of points
changes. Fit endpoints take priority during this bounded warm-up. Recent spacing
is fixed thereafter, rather than shrinking indefinitely with total duration.

The mapping also supports chosen timeframes and zoomed/panned ranges. It never
stretches actual samples to invented timestamps: an explicit range outside the
recorded extent keeps its genuine blank interval. Fit/All uses retained extent;
a held selection or an intentionally fixed historical interval remains held
until existing Fit/Follow controls change it.

## Grouping and data integrity

Existing grouping is retained: diagnostic histories up to 1,400 visible samples
keep individual markers; larger histories progressively group older samples
with bounded display work. The configured recent interval keeps original
individual samples. This grouping is display-only. Every contributor, original
timestamp, zero, failure and status remains accessible. Group details report
actual start/end, count, failures and RTT min/mean/max. Extrema bars and the Y-axis
use represented source values, not only averages. The existing packet aggregation
and linear overview are unchanged; packet plotting and drag/zoom share the same
forward/inverse time mapping.

## Focused verification

Deterministic numerical tests compare the identical recent section over spans
from 180 seconds to 1,000,000 seconds, at normal/narrow widths; verify the smooth
join, monotone compression, warm-up continuity and inverse drag/zoom. Renderer
unit tests cover 706/1,400/1,401/7,201-point growth, selection, filters, resizing,
raw/group endpoints and the existing 10,000-point retention rollover. Additional
fixtures retain duplicate/irregular timestamps, gaps, spikes, zero and failures
with exact contributor details. Existing Linear, timeline/navigation, grouping
and packet tests remain enforced.

The existing UI98 browser suite compares actual recent marker pixels and source
hover/held selection before and after 706 to 7,201 samples at 1440- and 390-pixel
window widths. It records numerical evidence and four comparison screenshots
(`ui98-condensed-stable-<width>-<count>.png`). Main-plot drag is checked against
the inverse, and UI97 keeps the existing 1ms wheel-anchor tolerance using the
actual rounded browser pointer coordinate.

Local focused checks are not a claim of hosted browser/native acceptance.
Required build/CI and rendered screenshot inspection must establish those
results on the final integrated source before delivery.
