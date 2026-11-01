# File Transfers

Network Toolkit currently offers separate SFTP, FTP/FTPS and TFTP workflows. A single File Transfer tool with a protocol selector is approved for Phase 6, but not yet implemented. The existing Transfer navigation group includes both SFTP / FTP / FTPS and TFTP.

## Choose a protocol
- **SFTP:** SSH-based file browsing and transfers. It may reuse an already connected and authenticated terminal session where the remote device supports the SFTP subsystem.
- **FTP/FTPS:** file transfer using FTP, with TLS when FTPS is selected and supported.
- **TFTP:** lightweight file transfer where appropriate; be aware it does not provide SSH-style authentication or encryption.

## Before transferring
Review the remote host, protocol, local and remote paths, overwrite confirmation and permissions. Do not assume files are safe to replace without checking their destination and the consequence of overwriting.

Use the visible **Reset panes** action in the two-pane browser to restore the default layout. The divider can also be reset by double-clicking.

## During a transfer
Monitor progress and cancellations. Closing a borrowed SFTP operation should not terminate the interactive SSH terminal. Preserve open sessions and avoid writing incomplete data over valid files.
