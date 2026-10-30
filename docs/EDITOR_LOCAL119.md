# Detached editor local Files access

Version 119 separates local file access from remote connection approval. A live,
authenticated detached editor can immediately browse Windows drives and folders,
open local text, and use the existing local file-management actions. The main
window need not be open and no Request access roundtrip is needed for Windows
files. The existing owner backend remains alive for detached windows; this is
not an independent backend or recovery after that process exits.

Windows account permissions still apply. There is no elevation, master-token
handoff, persistent filesystem grant, credential delegation or new connection
capability. Explicit text saves still require the current document UID/lease and
revision hash. Explorer mutations still refuse registered open documents and
their ancestor folders. Existing staged copy, no-clobber, confirmation, path,
size/depth limits and cancellation checks remain.

Connect and SFTP remain separate: the main window must explicitly approve its
currently connected transports. Reconnection, saved profile review, passwords,
vault unlock and terminal control retain their existing boundaries. Revoking
remote access no longer disables local browsing or an already-started local
copy. Closing/returning the detached editor does invalidate its local copy
publication and document authority. Imports pin the live renderer identity;
remote exports retain both the pinned transport and exact approval identity.

Tests use synthetic browser content and temporary files only. The new hosted
Windows test launches the real production editor child with no main GUI, marks
the main workspace absent, opens a temporary file and saves through its real
lease, then verifies bytes and captures the child. This is not a user-desktop,
visible UAC, live-device, or backend-process-loss test.
