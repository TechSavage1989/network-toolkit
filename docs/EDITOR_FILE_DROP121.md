# File-manager drops into the editor

Dragging a Windows or SFTP file from this window's file list onto the document
area opens it through the same validated text-read path as the file manager's
Edit action. An already-open file is activated without rereading or discarding
its dirty text. Other open documents and their undo history remain intact.

This is not text insertion, a file move, a remote login or a transfer approval.
Folders are not text documents. Binary/encoding/size limits, pending editor
operations and detached-window file permissions continue to apply. Unsupported
or changed sources show an error without opening a path supplied by another
application. Normal OS-file import and outward file copies remain separate.

The HTML gesture pins a real same-renderer row, selection, listing and transport
identity at drag start and validates them again at drop. The native row-pointer
adapter accepts actual matching pointer events and computes the drop target
itself; it accepts no caller-provided paths or credentials. Both paths share the
existing local/remote editor open pipeline and ownership/lease machinery.

Focused unit tests cover provenance, cancellation, folders, stale connections,
malformed payloads and overlapping reads. `editor-file-drop121.browser.cjs` uses
genuine browser mouse gestures against the full app, with synthetic authenticated
local/SFTP responses in main and detached normal/narrow layouts. Native OLE and
live-device behavior require their separate acceptance evidence; this browser
fixture does not claim either.
