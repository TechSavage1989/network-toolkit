# Editor saved connections (113)

The editor's Connected session chooser lists existing file connections separately
from saved SSH and FTP/FTPS profiles. Listing, selecting and Refresh do not connect.
Connect is explicit. SSH reuses a matching authenticated terminal when available;
otherwise the existing SFTP host-key verification, vault/destination approval and
manual sign-in flows run. FTP/FTPS uses the existing client; its operations may
open their own transport, so a ready FTP browser is not a permanently live socket.
Telnet, serial and TFTP profiles are not text-file sessions and are omitted.

The existing Files sidebar hosts connection controls inside the editor, including
manual FTP sign-in. Closing it does not disconnect or discard documents. A pending
operation or unrelated active file client cannot be silently replaced by choosing
a saved profile. Save existing files and disconnect explicitly before replacing
that client, or use its existing connected-session entry. Host/profile changes
during approval fail closed. Late client renders cannot overwrite an editor or
server page. A cancelled authentication leaves the original documents intact.

Detached editors still have document-only authority by default. After the main
window approves Files access, they may list minimal saved SSH metadata (profile
ID, display name, host, port, protocol). They never receive credential references,
key paths, saved passwords, private keys, settings or a connection API. Request
connection queues at most one two-minute request per editor and is bound to the
current explorer approval. The main window explicitly reviews the destination and
uses its existing sign-in controls. This action does not grant the new connection
to the child: Update access and a separate main approval are still required.
Cancel, expiry, revocation, window exit or profile edits invalidate pending review.
Main review and child cancellation are serialized; whichever commits first wins.
A completed review cannot be undone by a late cancellation and no cancellation
closes a connection. Declining a request never connects.

Tests use synthetic data only. Focused Go race tests cover metadata allowlists,
capability denial, request idempotency, dirty-document/transport retention,
revocation/reapproval/close/expiry/stale profiles and cancellation/review races.
JavaScript tests cover explicit actions, single-flight, protocol downgrade,
matching-terminal reuse, pending-operation refusal and deferred profile/config
changes during credential approval. The isolated hosted browser fixture
`scripts/editor-saved-connections113.browser.cjs` verifies real rendered controls
at1440 and390px, existing verification/manual sign-in, simulated native-unlock
cancellation, visible manual FTP/FTPS settings, dirty identity retention and child
request/revoke behavior. Browser mocks are not a real vault/native-dialog or live
network test; existing actual Windows native security gates remain required.
