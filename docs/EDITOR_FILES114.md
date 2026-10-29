# Editor Files and shared tabs (1.17.114)

The editor uses the same flat, bottom-accent tab shape, spacing, labels and close
controls as Terminal. A dot indicates unsaved changes and the accessible tab name
continues to announce them. View-local tabs, drag/reorder, Compare to, split views,
window moves and their existing safeguards are retained.

The editor toolbar has one Files action rather than another Connected session
control. More stays in the toolbar; it no longer repeats Open local path or New
at local path. Existing explicit local open/new/save operations remain available
from the file browser and editor actions. No file is saved merely by browsing.

## Files sidebar

- Windows files lists the current user's drives and mounted volumes in a compact
  selector. Selecting a disconnected drive reports its listing error; it does
  not mark every network or removable drive available in advance.
- SFTP keeps active and saved connections at the top. Selection is read-only;
  Use, Connect or Request connection is an explicit action. A current unrelated
  busy/live file client is not silently replaced.
- Current connection, refresh, disconnect and settings remain near the top while
  the directory list scrolls. Full authentication and secondary settings expand
  within the sidebar; credentials are never added to labels or selection values.
- Main-window FTP/FTPS retains the established authentication form and certificate
  checks. Separate editors retain approved SSH-only access. Saved requests still
  require main-window review and a separate Update access approval.
- Remote path entry and the existing directory picker remain secondary actions
  within Files; opening documents is distinct from copying or transferring files.

Native transfer behavior is documented with the transfer implementation. This
UI change does not itself grant a new local root, remote connection or credential
capability. Browser evidence uses synthetic profiles and files, not user data.
