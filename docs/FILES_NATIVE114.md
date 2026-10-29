# Files sidebar and Windows Explorer copies (114)

The local drive picker lists current-user logical/mapped drives plus Windows
volume mount folders. Enumeration does not probe every filesystem. Selecting an
unavailable/removable/network location reports its normal listing error.

## Copy workflow

- Drop browser-granted files or folders onto the local/SFTP Files pane. Review
  the destination and collision choices, then explicitly confirm Copy/Upload.
- Files permit explicit overwrite, rename or skip. Existing folders require
  rename or skip; there is no silent recursive merge or move. Cancellation can
  leave already completed files in the new folder, and reports that honestly.
- To export selected local/SFTP items, choose **Copy to Windows Explorer…** in
  the file context menu. After preparation, drag **Drag prepared copy** into an
  Explorer folder. Preparation stages actual bytes, including SFTP downloads;
  the OS never receives remote paths or authentication data. Windows handles
  its destination collision prompt. Only COPY is offered, never MOVE.
- This prepared-copy workflow requires the native Windows host. Browser users
  retain explicit Download. FTP/FTPS native folder uploads remain unavailable
  because their existing transport cannot guarantee atomic no-clobber upload;
  their established explicit transfer controls remain unchanged.

## Boundaries

100 top-level selections,1,500 tree items,32 folder levels,512MiB per browser
upload file and2GiB per prepared selection. Staging is caller-bound, expires
when unused and is removed after a reconciled native completion. No hover starts
preparation. Mouse release revokes the exact arm generation, including delayed
requests. Lost claim/start/finish responses reconcile the same job; no second
OLE copy is inferred from a timeout. Time expiry alone never deletes active OLE
source bytes.

Local recursive imports retain the selected `os.Root` and directory identities;
root/ancestor replacements refuse subsequent operations/publication. Symlinks,
shortcuts and special export files are rejected. Remote export pins the existing
SSH transport and stat size, and checks aggregate bytes before staging. Each
remote operation remains subject to the server's permissions and capabilities.
Detached editors need their existing explicit Files approval; copies add no
credential, connection, or arbitrary native-path capability.

Native OLE exposes only synchronous IDataObject, not asynchronous shell-copy
capability, so completion precedes cleanup. No WebView host objects or web
messaging are enabled. Background polling is a fixed authenticated same-origin
endpoint and claims only while the owning foreground window has the mouse held.
Reference: Microsoft [DoDragDrop](https://learn.microsoft.com/en-us/windows/win32/api/ole2/nf-ole2-dodragdrop),
[IDataObject](https://learn.microsoft.com/en-us/windows/win32/api/objidl/nn-objidl-idataobject),
[IDropSource](https://learn.microsoft.com/en-us/windows/win32/api/oleidl/nn-oleidl-idropsource).

## Verification

Focused Go-race tests cover local roots/ancestor swaps, browser-byte collision
publication, caller isolation, gesture tombstones, lost acknowledgements,
real synthetic loopback SFTP files/empty folders and changing remote sizes.
JS tests cover recursive preflight/collision/cancellation and released gestures.
Windows test binaries cross-compile locally; this is not native runtime proof.

The hosted `TestNativeExplorerFiles114` gate must pass actual Explorer folder
input, file+folder output, quick-release negative, Escape cancellation and a
synthetic NTFS junction negative. If WebView follows the junction target, folder
input must not be released as safe. This fixture uses synthetic temporary data;
it does not establish arbitrary device-fleet or user-desktop compatibility.

### Active native cancellation

An active OLE gesture owns Escape; the page does not remove its modal or delete
staging underneath it. Rejected/uncertain cancellation leaves the review visible.
A window close or revoked lease requests cancellation, but a stationary held
mouse can keep the native OLE loop active. Release the mouse or press Escape to
finish it. On the next real input-state change, `QueryContinueDrag` checks the
canceled context before any drop decision. The native owner and staging remain
alive until OLE returns; no synthetic input or global hook forces the gesture.
Microsoft documents this input-driven contract in
[QueryContinueDrag](https://learn.microsoft.com/en-us/windows/win32/api/oleidl/nf-oleidl-idropsource-querycontinuedrag).

During a started drag, one coalesced authenticated read of that exact job runs
at the existing 200ms status cadence with a one-second request bound. Denied,
missing, revoked or unavailable status cancels the source; it never reconnects
or probes a device. Revocation takes effect when OLE next processes real input,
not by claiming an instantaneous stationary-mouse wake-up.

After OLE/COM release, the host attempts the existing idempotent finish receipt
with the same token/job under a three-second cleanup-only context. No authentication
exception is added. Confirmed main-window return or child-process exit also
cleans that owner's jobs while its backend is alive. Shutdown does not reclaim
another surviving editor child's active staging. If the backend is interrupted
or no longer accepts the receipt, the private temporary prepared copy may remain;
it is not silently deleted by a new native filesystem capability. Original files
are unchanged. Native MOVE and deletion of original sources are never offered.

The hosted fixture retains the real owner and stage during a stationary hold,
then moves/releases over a separate empty Explorer destination. It requires a
cancelled outcome, no destination output, unchanged originals, authenticated
stage cleanup and bounded owner completion after input resumes. The earlier
immediate-close-while-stationary assumption was not an OS contract and is not
claimed as delivered behavior.
