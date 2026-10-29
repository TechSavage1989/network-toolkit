# Editor sidebar / drive wheel candidate for 1.17.117

This is a local, unreleased change based on the completed 1.17.116 Terminal
hotfix (`d4d453ea54ed468037f3bf768653b5f51c27bfcb` in the integration checkout;
lead-recorded published source `f13c4df906a0ed8124308f8f95db571a7a9be532`).
It does not authorize source upload, a version bump, publication, or changes to
existing tags/assets/access. The Terminal hotfix handler is unchanged.

## Requested behavior

- Replace the editor's horizontal Files / Connections header with the existing
  Terminal/transfer-client vertical rail. The same `terminal-sidebar-rail`,
  `terminal-sidebar-tabs`, and `terminal-sidebar-tab` styles supply its 43-pixel
  rail, 42-pixel buttons, icon sizes, active edge, colors, spacing and focus ring.
  Files uses the shared folder icon. Connections uses the shared plug icon and
  compact visible label **Connect**, with accessible name/title **Connections**.
- No new heading row takes back the recovered vertical space. Windows files and
  SFTP remain within Files. The panel still supports existing FTP / FTPS paths.
- Clicking the current side tab collapses content; either side tab can reopen
  its panel. The actual rail stays visible, matching the other sidebars. Arrow
  Up/Down, Home and End select rail tabs without accidentally collapsing them;
  Windows/SFTP retains horizontal tab keys. Handled rail navigation keys do not
  leak into editor document shortcuts;
  unhandled commands such as Ctrl/Cmd+S, Ctrl/Cmd+F and global Ctrl/Cmd+K
  keep bubbling to their existing handlers. Native Enter/Space activation stays
  available. Enter/Space and double-click collapse the resize divider; drag, arrow and size-limit keys retain resizing.
- The saved width remains the total expanded sidebar width, not width plus a
  new rail. The collapsed shell is 44 pixels (shared rail plus outer edge),
  replacing the earlier 27-pixel vertical-text reopen button. Existing main and
  detached preference formats are unchanged. Compact drawers preserve desktop
  collapsed/width preferences and retain their full available editor height.
- Mousewheel over an overflowing drive strip moves it horizontally. Pixel,
  line and page deltas are handled; horizontal touchpad input, Shift-wheel,
  Ctrl/Meta zoom, edge chaining, keyboard buttons and touch scrolling remain
  native. File-list and other page wheel input is not intercepted.

## Retained boundaries

The rail wraps rather than recreates the existing content host. `host.hidden`
continues to gate sidebar availability and pending detached responses, and the
collapsed content is also inert. Files/Connections selection retains mounted
connection forms, selected folders and open/dirty documents. Approval, saved
profiles, credential use, host verification, active transports and detached
SFTP-only access are not broadened. No user device, credential, configuration
or desktop was accessed. Private reference images were locally materialized
and visually inspected before edits; none is a repository/test/public asset.

## Local validation

- `node --test scripts/*.test.cjs`: **1,294 passed**, zero failures.
- The focused tests include rail DOM reuse, selected-tab collapse, vertical keys,
  pointer/keyboard resize, retained preferences and access races, plus wheel
  units/modifiers/edge/nonoverflow/non-strip behavior. Existing saved/active
  SSH/FTP and detached credential restrictions remain covered.
- Changed JavaScript and browser fixtures pass `node --check`.
- `git diff --check` passes.

**Not yet executed:** hosted browser/native tests. No local browser was launched;
this executor's earlier socket/bwrap restrictions are respected. Explorer-height
improvement is not yet reported as a measured result, and screenshots have not
yet been generated for this candidate. Local unit success is not visual or
native-runtime acceptance.

## Required hosted evidence

Run the new fixture with the exact published 1.17.116 source materialized outside
this checkout and the runner's supported Playwright installation:

```sh
PLAYWRIGHT_MODULE="$RUNNER_TEMP/ntk-editor112/node_modules/playwright" \
NTK_FILES_BASELINE_DIR="$RUNNER_TEMP/ntk-files-baseline116" \
TOOLKIT_TEST_OUTPUT="$RUNNER_TEMP/ntk-editor112-results" \
node scripts/editor-sidebar117.browser.cjs
```

`editor-sidebar117.browser.cjs` pins the baseline sidebar JS, editor CSS and
remote-browser JS hashes. It records before/after actual explorer geometry and
requires at least 20 pixels of height gain, preserved total expanded width, no
added heading, no page overflow and the shared rail dimensions. This threshold
is an acceptance requirement, not a claimed measurement. It covers normal and
390-pixel narrow layouts in both themes, in main and detached windows (eight
paired cases). It captures baseline, Files, Connections and failure screenshots
and `editor-sidebar117-results.json`.

The fixture also exercises real side-tab clicks and keyboard input, pointer and
keyboard resize, collapse/reopen, wide–narrow–wide intent, actual vertical and
horizontal drive wheel events, actual file-list wheel non-hijack, keyboard drive
selection, modifier/edge guards, permission-request waiting, dirty/form identity
and absence of new connection/credential calls. Native touch hardware is not
claimed tested. Keep existing `editor-files115.browser.cjs`,
`editor-saved-connections113.browser.cjs`, `editor-views112.browser.cjs` and
`editor-toolbar113.browser.cjs` in the hosted suite.

The legacy Files115 fixture selects a panel idempotently now that clicking the
active rail tab intentionally collapses it. The toolbar113 historical-height
benchmark accounts for the real 44-pixel collapsed shell rather than the old
27-pixel button; its equal-workspace-width and viewport-height checks remain.
The lead owns workflow integration and final evidence/publication decisions.
