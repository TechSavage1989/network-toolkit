# Compact application frame

The main application no longer reserves a separate breadcrumb/topbar row above
the active tool. Each tool keeps its existing title, commands, configuration and
update/offline notices.

- The system name is beside the sidebar brand. Long names have an exact tooltip
  and keyboard-accessible label; the collapsed brand icon identifies the system.
- Theme and App settings are the original controls in the sidebar footer, beside
  the running diagnostic count. Both remain keyboard accessible in the icon rail.
  Opening Settings from the narrow navigation drawer closes that drawer first,
  and dismissing the dialog returns focus to the visible navigation opener.
- Narrow windows reuse the existing page-heading, diagnostic-command or Terminal
  command slot for the same navigation button. Short local workspaces whose
  heading is hidden use the existing app footer instead; it is one compact row,
  not an overlay. Resizing and changing tools relocate the same control.
- Detached editor and terminal modes already omitted the old topbar. They do not
  acquire new vertical chrome or broader global-settings access. Their existing
  scoped tool controls and window ownership/security behavior are unchanged.

The hidden `crumb` text remains for internal compatibility, not as a layout box.
Engine state, diagnostic counts, backend-authoritative activity dots, theme
preferences, update notices and the existing configuration actions are retained.

## Verification

`scripts/app-chrome109.test.cjs` and `scripts/app-navigation.test.cjs` cover the
single-control identity, accessible metadata, stable compact slots, resizing,
drawer focus and settings return focus. Existing Settings browser scenarios use
the real navigation opener and relocated button; no forced or evaluated clicks.

`scripts/app-chrome109.browser.cjs` requires `NTK_LAYOUT_BASELINE_DIR` pointing to
the exact published 1.17.108 checkout. It serves that version and the candidate
with the same synthetic API, sizes and themes, measures actual tool rectangles
and available workspace height for every navigation tool, and captures normal,
narrow, collapsed, short and DPR2 layouts. Separate editor fixtures measure the
editing surface and comparison panels. Detached shell modes must not shrink.
No hardcoded topbar-height estimate is used as acceptance evidence.

Local JavaScript tests are not rendered evidence. Hosted browser reports and
actual screenshot review are required before any height-gain or visual-pass
claim. Browser DPR tests are not native Windows multi-DPI certification. Fixtures
do not connect devices, start capture, touch user files or change permissions.
