# Editor views and Files sidebar

> Version119 update: detached Windows Files access is now independent of main-window
> approval. The historical approval description below remains applicable only to
> remote SFTP transports. See [Detached local Files](EDITOR_LOCAL119.md).

Version112 places menus, actions and settings above the tabs. Each text pane owns
its own tab strip. Split view is independent of comparison: Move to left/right
view, tab context menus and deliberate same-window drops move the document itself.
All further opens and New start on the left. Swap exchanges only the selected
comparison pair. Additional tabs keep their assigned view. Single view reunites
the documents; saving, dirty-close consent, undo and file bindings stay attached
to the same document object. Native window transfers include bounded UID-only
view membership and selected tabs, independently of comparison options.

Files opens a resizable sidebar using the existing Windows/SFTP browser. Local
and remote location tabs share navigation history, address entry, breadcrumbs,
filter/sort, selection, keyboard commands, context actions, copy/paste, new files
and folders, rename/move, preview, upload/download, progress and confirmed delete.
The sidebar follows app colors, compact control heights and spacing. At narrow
widths it becomes an overlay drawer. It closes without disconnecting sessions.

Detached editors retain document-only access by default. Request access in Files,
then review and allow in the main window. The approval allows local file actions
and captures only currently connected SFTP transports. It does not delegate
credential management, settings, terminal control or generic text writes. Revoke
from Editor window access in the main Files sidebar. For a new/reconnected server,
use Update access and approve again. In-flight operations already accepted may
finish. Detached connection controls remain in the main workspace. A separate
editor can continue saving its owned document through its existing lease after
explorer approval is revoked; those are distinct permissions.

Explorer mutations refuse registered open editor files and their containing
folders. Close the document first; dirty-close confirmation still applies. A
reservation prevents a new document lease or Save As from claiming a destination
while an explorer operation is changing it. Existing filesystem checks, staged
uploads, overwrite confirmations, revision hashes and pinned SSH transports remain.
Paths normalize dot segments; local checks resolve existing filesystem links.
These checks do not certify remote symlink aliases or local hard-link identity;
external filesystem changes still use the editor's existing revision conflict check.
Local OS/native shell drag support remains platform dependent; no file contents
or credentials are added to native document-tab drag data.

Acceptance must run on isolated hosted runners. New focused browser coverage uses
synthetic local/SFTP fixtures; Go tests use temporary folders and local test SFTP
servers to verify actual file operations, approval boundaries, leases and stale
transport rejection. Existing browser and real Windows/WebView2 gates are retained.
Never automate the user's desktop or launch test windows locally without approval.
