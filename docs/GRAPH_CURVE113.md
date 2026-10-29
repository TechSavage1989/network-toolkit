# Graph scale and compression

Linear plots and their timestamp axis use the same measured-sample domain. A short
new run fills naturally at the selected time scale (30 seconds for the default
All view); once that interval has filled, the first and last displayed samples
bind the plotting edges. Historical ranges bind their actual recorded endpoints.
The grid may show the requested interval while empty; no synthetic points are
created. Endpoint circles are clipped at their exact data position, not shifted
inward to make room for a radius. Genuine missing values, sequence gaps and large
recording-time gaps are not joined by a line.

Condensed **Recent scale** sets recent time spacing, rather than dropping older
history. Its default matches approximately 30-second Linear spacing. The newest
configured number of points remain uncompressed; older time follows a monotonic
curve into one left-edge history cluster. The full-history overview remains
explicitly linear and shows coverage; the main graph and its labeled time axis
share the nonlinear mapping. Cluster details retain all available originals and
existing archived-summary limits. Existing raw retention limits are unchanged.

**Curve…** opens a compact dialog without reserving graph space. Drag the curve
point horizontally to alter compression onset and vertically to change strength.
Arrow keys provide the same control (Shift makes larger steps); sliders give
labeled equivalents. Set the uncompressed count numerically, choose Gentle,
Balanced or Dense history, or Reset. Changes are immediate and persist through
the existing settings store. They do not change scans, original results or a held
sample identity. The dashed preview boundary marks where the recent normal-spacing
region meets older compression.

A narrow window, zoomed scale or large genuine gap can fit fewer recent points.
The Curve button then shows **effective/requested**, and its tooltip explains the
limit. The saved count is not overwritten. Once a recording has filled its plot, maturity
is retained across later scale changes. Zooming out beyond the available history
clamps the effective scale to recorded data; that effective scale then stays fixed
as new samples arrive, rather than squeezing recent points on every update. In an extreme gap only one recent
point may fit; ordering remains monotonic instead of folding older points back
across the plot. Historical range navigation binds the selected actual sample
endpoints; the requested interval is not fabricated as extra samples. Panning/zooming therefore
snaps the final viewport to recorded endpoint instants; an arbitrary between-sample
pointer time is not promised to stay fixed. Hover and clicked sample identity remain
exact.

Implementation uses a bounded 257-entry monotonic integral table and inverse
lookup. Packet-capture interval projection retains its separate legacy behavior.
No new package, network request or persistent raw-history store is introduced.

Validation: focused tests cover projection inversion, irregular/tied timestamps,
large gaps and width limits, recent density through growth, raw/summary rollover,
selection, clipping and state persistence through a real application restart.
`scripts/graph-curve113.browser.cjs` adds actual browser curve drag/keyboard,
presets/reset/persistence, startup/mature endpoints and normal/narrow light/dark
captures. Hosted browser results must be recorded by the release lead; authoring
this fixture is not a claim of a rendered pass or live-device testing.
