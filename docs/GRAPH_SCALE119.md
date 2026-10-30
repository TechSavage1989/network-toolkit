# Graph interval and follow controls

- In Condensed mode, an explicit Recent scale is the displayed time window, not
  a spacing hint. The Fit curve's normalized proportions are scaled into that
  interval. The dropdown, axis bounds, navigator and visible-span readout agree.
  Manual zoom or interval resizing shows Custom. If the five-record/two-second
  zoom floor expands an undersized request, Custom and the actual interval are
  shown rather than falsely retaining the preset label.
- All/Fit retains the existing default ten-second recent spacing, compression
  controls and older left history cluster. An explicit window excludes older
  records from the view, not storage. Requested time before recording began is
  blank; original timestamps and sample identities are never stretched or edited.
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
It records exact intervals and normalized point positions in normal/narrow,
light/dark layouts, actual Follow clicks with all Details visibility states,
left-handle pointer movement through a new sample, keyboard resize and paused
navigation. Local source tests do not substitute for its hosted rendered result.

### Exact-source review correction

The first hosted four-layout acceptance passed interval, proportional-shape, Details-preservation and left-handle behavior. Pixel review found narrow preset text clipped by duplicate native-select arrow padding; the bounded control-width trade keeps the same row footprint while exposing the full preset and Custom labels. A paused Fit stores an internal fixed viewport but is still All, not Custom. Legacy assertions requiring Follow to close Details or fractional Condensed windows to snap to sample endpoints were replaced with the newly requested visibility/exact-window contract; Linear endpoint assertions remain. All stored-source, minimum-point/time and temporal-render safeguards remain in force. Corrected hosted pixel/legacy acceptance is required before claiming these follow-ups passed.
