# Editor windows and document ownership

The native Windows application can move the active tab or all open documents to a new editor window, another editor window, or the main workspace. **Detach editor** is a direct compact toolbar action for moving all open documents to a new editor window. When no documents are open, it reads **Open separately** and creates one blank Untitled draft before using the same protected move. Repeated clicks cannot create parallel launches; a rejected launch leaves the draft editable in its original window. **Move / merge**, beside it, retains the explicit destination selector and the individual **Move tab** and **Move all / detach editor** actions. Existing windows appear by name, and detached windows can choose **Main workspace**.

Drag a tab within the tab strip to reorder it, or focus a tab and use **Alt+Shift+Left/Right**. Reordering preserves document objects, active identity, comparison pairing, text/history and view state. A native-capable drag can move a tab into another Toolkit editor's document area, or into the explicit **Drop tab here to open a new editor window** target shown during dragging. These gestures feed the same lease/acknowledgement pipeline as the buttons. Only an opaque short-lived ticket enters drag data; document text, paths and credentials do not. A fast drop may briefly precede source registration: only the same ticket’s not-yet-registered response is retried for up to 1.5 seconds after that drop. Cancelled, expired, conflicting or uncertain responses are not retried; the original document stays protected by the existing ownership protocol.

Dropping on the bare Windows desktop, outside all accepted targets, pressing Escape, or using a foreign/expired ticket does not detach or clone a document. Bare-desktop drag-out is not supported; use the explicit in-app detach target or the compact button. The drop targets are not a generic native filesystem/window-control bridge. A move preserves the document identity, unsaved text, undo/redo stacks, encoding/BOM/line endings, caret/selection, scroll position and comparison state. New empty receivers also apply the source's current editor appearance; an existing window retains its own appearance.

## One editable owner

The source is frozen before a move. The destination stages the full batch read-only and asks the local engine to atomically accept its document leases. Only a confirmed acceptance enables editing there; only that acceptance removes the original tabs. A rejected launch or cancelled unaccepted move restores the source. An unreadable/lost acceptance response is **uncertain**, not a failed move: the source remains protected until the authoritative registry resolves it. Cancelling an already accepted move cannot resurrect another editable copy. The source acknowledges receipt only after releasing or restoring its tabs; unacknowledged outcomes are not silently evicted from the bounded transfer ledger. If all 64 ledger slots are unresolved, a new move is rejected until an existing outcome is resolved.

The private `/editor-view/<id>` document uses the existing native-only loopback bootstrap. Tokens and document text are not placed in URLs, process arguments, HTML bootstrap, logs, settings or public APIs. Child capabilities cannot access the vault, connect devices, start diagnostics or use general terminal APIs. Child file writes use document UID + current lease and the existing hash/no-clobber text-write handlers. Existing authenticated SSH transports are pinned; credentials and JavaScript callbacks are never serialized.

## Closing and recovery

Closing a detached editor returns its documents to the main workspace and waits for acknowledged ownership before retiring the native view. A pending move or save prevents close. Both primary workspaces—the main app and the independent Terminal, whose SFTP browser can open an editor—use unsaved-document confirmation. Their native hosts do not treat a five-second timeout as permission to discard work. Successful closes still follow the normal shutdown path; detached terminal children retain their separate return-to-owner policy. Cancel leaves the workspace usable; an unanswered close request shows a visible warning and can be retried.

The engine retains bounded, in-memory last-acknowledged snapshots of eligible idle documents. One unsupported desktop handle or pending file does not prevent checkpointing the other eligible tabs. A **confirmed process exit**, not a missing heartbeat alone, permits recovery. The recovery notice explicitly warns that edits made since the last acknowledged checkpoint may be missing. If the engine becomes unavailable, the loaded editor is not automatically closed; keep it open and use **Save copy**. This is not durable crash recovery across an engine/OS restart.

## Explicit limits

- At most 12 documents per receiving editor; each text/history entry is at most 8 MiB, undo/redo stacks at most 80 entries, and a whole transfer at most 32 MiB. A limit rejects the entire move without truncation.
- Local files, local drafts and documents on existing retained SSH/terminal transports are supported. A desktop File System Access handle and request-scoped FTP/FTPS credentials cannot be safely serialized. Such documents remain in the original window with a clear message; save as a local file before moving. Save is not silently downgraded.
- No real device connections, user files or user-desktop testing are part of synthetic acceptance. Browser fixtures exercise real bundled UI in separate pages but a synthetic registry. Native Windows acceptance separately checks the production bootstrap, ownership endpoints, actual child window lifetime, and consent-required closing.
- The capability is absent in static/offline previews and older backends. Those modes retain the ordinary editor and do not register or mirror documents.

## Focused acceptance

`node --test scripts/editor-windows.test.cjs` covers unsupported modes, rejected launch/offer, acceptance, lost offer/ACK responses, cancellation after acceptance, protected close, and scoped new-child writes.

`node scripts/editor-windows108.browser.cjs` exercises two actual browser contexts, production editor controls/coordinator, text/history/position preservation, merge, rollback and wide/narrow screenshots. Its registry is synthetic and it does not claim native execution.

With installed WebView2 on the hosted Windows runner, `NETWORK_TOOLKIT_NATIVE_EDITOR_TEST=1 go test ./internal/desktop -run TestNativeEditorCloseApprovalInstalledWebView2 -count=1 -v` holds the real native close beyond five seconds, confirms no irreversible closing notification, cancels through UI Automation, continues editing, and captures the actual native host. The app-level two-window runtime test provides separate real registry/child-process proof.


## Full-width detached layout and focused110 coverage

A detached editor uses its own single-column shell at every width, including a maximized window. The main pane explicitly occupies grid column1; merely hiding the parent sidebar is insufficient because the parent grid column2 placement otherwise creates a blank implicit track on wide displays. No main navigation or settings capabilities are added to child windows.

`scripts/editor-window110.browser.cjs` requires `NTK_EDITOR_BASELINE109_DIR` containing exact published109 source. It compares actual baseline/candidate main rectangles at390,760,761,1100,1101,1440,1920,2048,2560 and3840 CSS pixels; proves the old blank track; checks main-left/full-width, dirty/history/selection/scroll retention across resizing, keyboard reorder and compact control visibility. Native pointer gestures and actual OS maximize/restore use the separate hosted Windows acceptance fixture. Authored tests do not imply execution; final exact-source CI and actual screenshots remain required. Browser wide viewports do not claim a physically2048px native runner desktop or real multi-DPI testing.
