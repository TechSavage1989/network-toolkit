# Editor documents, comparison and window-state contract

The editor already supported up to twelve documents. This update adds keyboard
navigation between those tabs, a New-tab control, editable side-by-side comparison,
and a change map. First typing or pasting into an empty editor now creates an
Untitled document immediately, including the first edit in Undo. Navigating to
another Toolkit tool does not discard open documents or unsaved text. Document
contents are kept in session memory, not written to settings or autosaved to disk.
Explicit Close still confirms discarding unsaved changes.

Compare two open documents using the selectors. Open files or create a text tab
from the comparison toolbar. Each pane edits the same document as its normal tab;
Undo, dirty state, language choice and Save are not independent copies. Use Previous
or Next difference, or click/drag anywhere in the change map. The map distinguishes
added, removed and changed lines; its outlined rectangle shows the visible region.
Arrows, Page Up/Down, Home and End work when the map is focused. Fit document shows
the full overview. Other map scales are pixels per logical comparison line,
independent of editor font size. Wheel scrolling over a zoomed map pans the overview
without moving document text. Wrapped-line measurements map each pane's actual
pixel heights back to comparison rows, including insertions and deletions.

Limits are explicit: twelve tabs; existing file reads up to 8 MiB; comparison up
to 2,097,152 JavaScript characters and 30,000 lines per side, with at most 2,000,000
LCS cells in the changed region after common prefix/suffix trimming. Comparison
falls back with a visible explanation when the budget is exceeded; both complete
documents remain editable. Syntax painting uses the existing 250,000-character
limit for ordinary languages and a 1,000,000-character limit for the bounded
network-configuration lexer. Lexer token/output limits or a larger document show
plain text with an explicit notice, never truncated document text.

The window coordinator uses an allowlisted in-memory adapter: stable document UID,
source binding, exact text/save metadata, Undo/Redo, selection and scroll, language,
and optional comparison/preferences. Snapshots freeze the selected source only
after complete validation; the destination is staged and frozen until its ownership
ACK. Failed transfers can restore the source without a discard prompt. Checkpoints
copy state without freezing and never persist it to disk. The adapter rejects
pending operations and unsupported source bindings rather than silently changing
how Save works. Native coordinator/proxy authorization and launch validation are
separate from this UI adapter and must pass their own tests.

Validation: focused Node tests cover bounded diff reconstruction/geometry,
empty-paste retention, tab views, dirty-close cancellation, save-revision races,
Save-As destination reservation and snapshot/rollback/activation. Browser fixtures
`editor-empty-paste.browser.cjs` and `editor-workflows108.browser.cjs` exercise actual
clipboard input, rendered colors/wrap, map controls and synthetic configurations.
The authored fixtures are not evidence of execution: use the final exact-source CI
reports and screenshots for browser/native acceptance. No live device or private
user document is required by these tests.
