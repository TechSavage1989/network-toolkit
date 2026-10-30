# Graph interval and follow controls

- In Condensed mode, an explicit Recent scale is the displayed time window, not
  a spacing hint. The selected interval, navigator and visible-span readout agree.
  Following the 120 correction, the underlying Fit age-to-pixel curve is not
  stretched to the selected duration: older selected history occupies the left
  cluster, and recent spacing cannot shrink merely because the duration grows.
  A mature window shorter than the baseline curve expands visible distances to
  fill both ends; its original curve ages and requested uncompressed count remain
  unchanged. This is the user's explicit filled-left preference, not a claim that
  arbitrarily short windows can also retain exactly identical point spacing.
  Manual zoom or interval resizing shows Custom. If the five-record/two-second
  zoom floor expands an undersized request, Custom and the actual interval are
  shown rather than falsely retaining the preset label.
- All/Fit retains the existing default ten-second recent spacing, compression
  controls and older left history cluster. An explicit window excludes older
  records from the view, not storage. Requested time before recording began has
  no invented records; a mature graph still places its real older cluster on the
  left. Original timestamps and sample identities are never stretched or edited.
  Sparse startup and sub-two-second recordings keep their existing exceptions.
- Follow latest releases held graph selection and follows the newest matching
  result without opening, closing or replacing the existing Details pane.
- Resizing only the history navigator's left edge preserves the right anchor and
  existing following/paused state. A following window keeps its new duration as
  samples arrive. Right-edge resizing or moving the complete interval pauses
  following. Pointer and keyboard controls share the same boundary checks.
- No packet-capture, connection, file-access, credential or native-window
  capability is added. Existing retention and partial-summary limits remain.

The focused synthetic browser fixture is `scripts/graph-scale119.browser.cjs`.
It records exact intervals and same-age Fit-versus-Recent point positions in normal/narrow,
light/dark layouts, actual Follow clicks with all Details visibility states,
left-handle pointer movement through a new sample, keyboard resize and paused
navigation. Local source tests do not substitute for its hosted rendered result.

### Exact-source review correction

The first hosted four-layout acceptance passed interval, proportional-shape,
Details-preservation and left-handle behavior. Pixel review found narrow preset
text clipped by duplicate native-select arrow padding; the bounded control-width
adjustment exposes the full preset and Custom labels. At 390 px, Custom may wrap
Curve into the existing responsive controls' second row; controls remain
readable and reachable, but unchanged toolbar height is not claimed. A paused
Fit stores an internal fixed viewport but is still All, not Custom. Legacy
assertions requiring Follow to close Details or fractional Condensed windows to
snap to sample endpoints were replaced with the newly requested
visibility/exact-window contract; Linear endpoint assertions remain. All stored-
source, minimum-point/time and temporal-render safeguards remain in force. Final
exact-source review 40995231720 passed all four focused layouts and the affected
legacy graph checks; current actual pixels were reviewed. This does not change
the separate comparison-motion caveat recorded in RELEASE119_HANDOFF.md.

### Post-release baseline-spacing finding

The user subsequently clarified that Recent must preserve Fit's baseline pixel/
time spacing and configured uncompressed count, not merely normalized shape.
The119 scale projection changes that spacing: the inspected Fit/5m reference
pair showed about126 px versus8.6 px one-second gaps at the same plot width.
This remains an open119 regression; the earlier four-layout acceptance did not
assert this invariant. See RELEASE119_HANDOFF.md; the120 repair is described below.

### 120 baseline-density regression

119 incorrectly multiplied the detailed curve's time parameters by the full
selected interval. At a 1,262px plot and three newest uncompressed points, its
5-minute view reduced the normal 126.2px one-second gap to about 8.6px. The
correction keeps the baseline curve and its inverse shared by points, ticks,
hit testing, pan and anchored zoom. The displayed interval is separate from the
finite detailed-curve horizon; aggregation includes only real selected records.
Long windows never shrink the baseline, short mature windows expand to fill,
and initial short history keeps its natural startup fill. Five-record/two-second
limits, exact left/right history anchors and Follow/Details behavior are retained.

The existing graph119 browser fixture now measures Fit versus Recent5m on the
same 303-record synthetic source, verifies both endpoints for a short4s window,
and saves synthetic before/after density screenshots. Actual hosted execution
and pixel review are required; local model tests are not rendered acceptance.
