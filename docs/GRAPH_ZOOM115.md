# Graph wheel direction and zoom floor

Graph wheel input now defaults to **down = zoom in**. The existing **Range…**
dialog contains **Invert wheel zoom**, persisted with the graph time settings.
The setting also controls packet-graph wheel direction; packet bucket/time
resolution is otherwise unchanged. Timeline navigator wheels still pan time.
Browser/system modifier-wheel shortcuts remain untouched.

RTT/result graph windows use actual retained timestamps rather than a fixed
maximum multiplier. Ordinary populated views retain at least five raw samples
and at least two seconds between their first and last visible raw samples. A
sparse recording therefore needs a wider interval. Condensed history totals do
not count as five visible raw points: its recent projection is bounded too.
The requested compression count/curve and ten-second reference remain saved;
the data floor constrains an otherwise unusable magnification.

A recording with fewer than five timed samples cannot be magnified further and
keeps its natural startup layout. If the entire recording spans less than two
seconds, all available samples are retained without invented times or samples.
A deliberately selected archived-only interval stays an honest summary; it does
not pull unrelated later raw samples into view to pretend the minimum is met.
Unknown timestamps remain unknown. Duplicate timestamps are not fabricated into
separate times. Explicit ranges narrower than the floor expand to real nearby
sample bounds while preserving the requested end where possible.

The same floor is applied to direct zoom, wheel zoom, manual navigator windows,
mode changes and resized layouts. At the floor, Plus is disabled and additional
wheel-in input leaves the interval stable, rather than scrolling the outer tool.
The displayed elapsed label still describes the actual rendered time span.
Selection, original measurements and stored results are not modified.

## Verification

Focused unit coverage includes persisted strict-boolean inversion, default and
inverted pixel/line wheel directions, modifier bypass, sparse/fast/duplicate and
short startup histories, repeated maximum zoom, both pointer anchors and narrow
resize, clipping, source preservation, ordinary pan/Follow retention, and stable
limit input. The Go application restart test covers persisted inversion alongside
existing compression settings.

`scripts/graph-zoom115.browser.cjs` uses `PLAYWRIGHT_MODULE` and
`TOOLKIT_TEST_OUTPUT` (or `NTK_BROWSER_OUTPUT_DIR`) with synthetic local request
fixtures. It exercises actual wheel input and Range controls in normal/narrow,
light/dark, linear/condensed views, captures intermediate bounds, tests repeated
hard-stop input and timeline panning, and records screenshots. Hosted execution
and exact final-source acceptance are required; writing the fixture is not a
claim that it passed. No live devices or private user data are used.
