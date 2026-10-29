# Compact editor controls (113)

The editor keeps More and Connected session in the primary toolbar. Expanding
More reveals only the additional action row; the More control does not become a
separate full-width heading. Connected session reveals its existing controls
without a redundant heading row. Open file and New tab remain available; typed
paths are under More → Open local path / New at local path. Files retains the
Windows/SFTP explorer. Cancelling a path dialog leaves the document unchanged.

See [EDITOR_SAVED_CONNECTIONS113.md](EDITOR_SAVED_CONNECTIONS113.md) for explicit
saved-profile connection, host verification and separate-editor approval rules.
Selecting a profile or refreshing the list does not connect.

## Compare a tab

Open a document tab's context menu and choose **Compare to…**, then another open
document in the same editor window. The clicked tab is the left document; the
chosen tab is the right document. The chooser includes paths, sources, tab numbers
and unsaved markers to distinguish similarly named documents. It excludes the
clicked tab and is disabled when there is no other document. Keyboard context-menu
navigation and Escape cancellation use the existing tab-menu conventions.

The comparison uses the same editable document objects, not copies. A temporary
comparison opened this way restores the prior view grouping when closed, provided
no explicit view move supersedes it. Closed or transferred-away tabs are never
restored. Moving windows uses the established acknowledged ownership process;
this chooser does not fetch or copy documents from another window.

## Read aligned differences

Comparison uses the same line-diff algorithm with an offline, source-aware editable
view. Added, removed and modified content share their color meaning with both
maps and the compact key. Changed portions within a line receive a stronger mark.
Blank alignment blocks are visual-only: they have no source characters or line
numbers, and they are excluded from normal copy/paste, undo and saved file bytes.
Corresponding rows account for wrapping in both pane widths. Large comparisons
retain the existing explicit work limits rather than silently claiming full alignment.

Ordinary single/split editing is unchanged. The comparison view uses pinned local
CodeMirror state/view components, not a CDN, external document service or a second
diff algorithm. Toolkit's document identity, history and acknowledged transfer
rules remain authoritative. Unfinished text composition blocks destructive close
and view-move operations until it is finished; it is not discarded as a shortcut.

## Resize panes

Normal Split view and Compare documents use the same narrow resize rail. Drag it
horizontally, or focus it and use Left/Right arrows. Home and End use the available
limits; Enter or double-click restores an even split. Escape cancels an active
drag. Each pane keeps a minimum usable width, clamped to the available viewport.
The rail exposes its position to assistive technology and follows the app's
existing border/accent/focus styling without adding another toolbar row.

Resizing changes presentation only: it does not insert newlines or alter document
text, history or ownership. Comparison wrapping, highlights, synchronized scrolling
and both document maps are recalculated through the existing layout pipeline.
The proportion is window-local; no new saved-setting or transfer-schema field is
introduced. A separate editor window starts with its own proportion.

## Verification scope

Focused tests cover source/state boundaries and interrupted actions. Hosted
browser fixtures exercise actual controls, pointer/keyboard resizing, comparison
selection, wrapped geometry, dirty-state retention and normal/narrow main,
separate-window and modal layouts. Before/after measurements use immutable112
source. These are synthetic fixtures, not a test of private documents, production
devices, a user's desktop, physical multi-monitor DPI or hardware trackpads.
Final exact-source results are recorded in [RELEASE113_HANDOFF.md](RELEASE113_HANDOFF.md).
