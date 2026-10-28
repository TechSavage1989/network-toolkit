# Compact tools and two-document comparison map

Local implementation based on main `4747873292eca9f09577ac2803756113557d0bcf`
and confirmed published `v1.17.110` (`a71f98c4f418077cfeefa5546cfb77de1d8e7742`).
Publication of this usability change as v1.17.111 was explicitly authorized on
2026-10-28. Hosted checks and asset verification are required before completion.

## Shared layout rules

- Tool identity belongs in the selected navigation item and window/tab context.
  The shared page heading no longer repeats that identity; its actions and mobile
  navigation remain available. Capture retains its toolbar's accessible name.
- Working controls start on the left. Shared toolbar variables in
  `web/assets/app-chrome.css` define 28px controls, 6px gaps and compact padding.
  Status, endpoints, filenames and connection context remain meaningful labels.
- Keep document actions, explicit Detach and Move / merge visible. Put secondary
  editor transformations in the keyboard-accessible More disclosure. New document
  has one tab-strip action (also present when empty); Ctrl+N is unchanged.
- The editor has no extra title row. Close editor and Full editor retain their
  existing contexts and unsaved-close behavior. Names and unsaved indicators stay
  on tabs. The normal toolbar owns Open file; comparison adds no duplicate file
  creation/opening controls. Unavailable Find/Replace/Go-to controls do not consume
  comparison space, and return when comparison closes.
- Comparison selectors, Compare, Swap sides and Previous/Next occupy a compact
  wrapping row. Options expands Sync scroll, Map zoom and the color legend.
  Open local file and Connected session keep their existing independent saved
  disclosures. Child preferences and capability boundaries are unchanged.
- Do not hide controls merely to meet a pixel target. Preserve keyboard access,
  per-window access, settings and every document operation when consolidating UI.

## Map contract

The adjacent Left and Right tracks share **comparison rows**, not line numbers.
Each row holds separate nullable document-line identities. Unchanged regions are
muted; additions green, removals red and modifications amber. Missing lines are
blank alignment gaps. Theme severity colors remain shared with existing controls.
Each document has its own viewport marker and accessible real-line range.

Fit uses exactly the available map height. Zoom is a multiple of Fit: 1.5×, 2×,
3× or 5×, including for short documents. It never changes editor font size. The
existing serialized `scale` values 0, .5, 1, 2, 4 encode these as `1 + scale`,
retaining the backend transfer allowlist without adding capabilities or fields.

Click/drag starts anywhere in a track; pointer capture continues the same gesture.
In Fit, wheel input scrolls that document. Zoomed wheel input pans the shared map
and navigates the pointed track's document. Pixel, fractional, line and page deltas
are handled and do not bubble to the app. Home/End, arrows and page keys also work.
With Sync scroll disabled the other document stays put. With it enabled, positions
are translated through each document's alignment and measured wrapping geometry.
Previous/Next deliberately position both sides at the selected difference.

Recalculation disables stale map navigation and hides stale markers. Editing,
swapping, wrapping, font/option changes and resizing refresh measured offsets.
Programmatic scroll acknowledgements prevent feedback loops. Map drawing groups
adjacent equal kinds rather than creating a separate SVG element for every line.
Existing bounded comparison budgets and document/history/ownership limits remain.

## Regression gates and evidence

`scripts/editor-layout-map.browser.cjs` uses actual Chromium rendering and pointer
input with synthetic documents and an exact published110 baseline. The existing
Checks workflow prepares that baseline and runs the new gate. It covers:

- Main, detached and modal editors at 1440×1000, 390×1000 and 820×540, both themes;
  local runs also exercise device pixel ratios 1, 1.25, 1.5 and 2.
- Paired screenshots and visible document-height measurements; no page overflow.
- Separate tracks, unchanged/added/removed/modified/blank regions; empty, short,
  unequal and heavily changed inputs; per-document marker pixel coordinates.
- Fit wheel, fractional wheel, all zoom levels, pointer click/drag, keyboard,
  independent/synchronized scrolling, wrapping, resize, swaps, edits and undo.
- Existing editor gates retain first-paste creation, saved disclosure preferences,
  source/save conflicts, close consent, history, tabs, failed launches and transfers.

Measured visible document heights at 1440×1000 (CSS px, dark; light also passes):

| Context | Before text | After text | Before comparison | After comparison |
|---|---:|---:|---:|---:|
| Embedded | 649.1 | 727.8 | 493.6 | 627.3 |
| Detached | 649.1 | 764.8 | 493.6 | 664.3 |
| Modal | 749.0 | 810.0 | 593.5 | 709.5 |

At 390×1000 the comparison gains are 78.5px embedded, 115.5px detached and
75.5px modal. Reclaimed space belongs to actual document viewports.

Native Windows tests use isolated synthetic files and real installed WebView2:
unsaved-close consent, real tab mouse gestures, acknowledged cross-window ownership,
merge, cancellation, failed launch and actual maximize/restore. Native screenshots
are captured from the exact test window, not the user's desktop.

Test maintenance changes only follow removed controls (New tab, More and outside
graph clicks). Clipboard assertions retain exact Windows CRLF expectations and
exact LF expectations elsewhere; they do not discard or normalize arbitrary text.

Local validation on 2026-10-28: all 1,027 JavaScript and 42 Python checks passed;
all eight required Go package suites passed in Ubuntu; all ten dedicated Windows
workflow groups and both native editor suites passed. All 49 browser scripts
(48 existing plus the new map/layout gate) passed, including the actual five-minute
updater scenario and temporal graph baseline/candidate checks. Windows GUI build,
release metadata validation, unchanged resource generation and canonical vendor
comparison passed. Linux/Windows vulnerability scans found zero reachable or
imported-package vulnerabilities; the existing unused module-only OpenPGP advisory
GO-2026-5932 remains, with no fixed version reported. These are local checks, not
hosted CI or a security audit.

Initial failures were retained in the evidence: checkout line endings, restricted
loopback/download/temp access, Linux-only assumptions in a broad Windows Go run,
old duplicate-control selectors, Windows clipboard LF/CRLF expectations, and an
independent-track loop-variable shadowing error caught by real pointer tests.
Corrections did not change backend behavior or relax release gates. Final focused
browser/native checks and build use the final application source.

## Limits and release discipline

Browser device-pixel ratios are not physical multi-monitor DPI certification.
Fractional browser wheel input is not a physical trackpad hardware test. Native
tests use the current host display; live devices, private settings/configurations,
user vaults, reboot and installation are outside this evidence.

Required Linux Go suites run in Ubuntu, since broad cross-platform Go tests include
Linux permission/device-path expectations. A broad Windows run additionally exposed
existing platform/shortcut assertions; these were not changed or relabeled as passing.
The workflow's dedicated Windows tests are run separately. Hosted CI/publication
must still run on any subsequently authorized publication source. Never waive gates,
change repository access, move old tags or replace assets based on local evidence.

Keep screenshots, test logs and detailed measurements in the review evidence bundle;
only synthetic fixtures belong in source. Future authorized releases must increment
all version metadata and rotate the current root release note using the established
process. The authorized111 preparation advances metadata and archives the110 note unchanged.
