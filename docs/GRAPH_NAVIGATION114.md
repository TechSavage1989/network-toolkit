# Graph navigation and spacing (114)

The default condensed recent spacing now matches the **10-second** normal
linear timeframe at the same plot width and recorded cadence. Explicit recent
scales still override it; saved custom detail scales remain proportional.
Linear All keeps its existing natural startup behavior. Curve strength/ramp,
uncompressed count, history summaries, source identities and clipping remain.

Growing old history accumulates in the left-edge cluster instead of changing
recent spacing. A requested recent count that cannot physically fit is still
shown as effective/requested and is never silently rewritten. Endpoint pinning
starts only after natural startup has filled the view.

**Follow latest** releases the held graph selection and follows the newest
matching graph/list result. It does not call the Details-opening path. Explicit
sample or row selection still opens Details normally. A deliberately panned
viewport stays paused until following is requested. Packet Follow also avoids
implicit packet-detail reads and preserves any manually selected inspector
snapshot and its open/hidden state.

Wheel input over the full-history timeline pans the visible interval. Horizontal
trackpad motion or vertical mouse-wheel motion work; positive movement advances
toward newer history, negative toward older history. Movement clamps at retained
history, preserves interval duration, and coalesces to one render per frame.
Control/Meta/Alt browser shortcuts and pointer-drag handling are not intercepted.

The zoom readout shows visible **elapsed time** in seconds, minutes or hours
(with subsecond units for short capture intervals), not an internal multiplier.
A condensed label includes the original history represented by its left cluster;
its accessible description explicitly says nonlinear, not evenly spaced time.
Exact endpoint timestamps remain in the tooltip. Packet capture uses the same
elapsed label without changing its existing interval projection or capture path.

Focused coverage includes 10-second width/cadence equivalence, stable spacing
through growth, wheel normalization/clamping/coalescing, no Details-opening call,
actual-span labels, and the existing graph/selection/capture regressions.
`scripts/graph-navigation114.browser.cjs` exercises actual wheel gestures,
follow/hidden-Details behavior, live updates and normal/narrow light/dark pixels.
Authored browser checks are not a rendered pass; exact hosted results must be
recorded in the release handoff. No live devices or user captures are required.
