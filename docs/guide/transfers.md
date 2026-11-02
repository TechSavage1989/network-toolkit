# File Transfers

Open **File Transfer** in the Transfer group. The buttons at the top choose the protocol: SFTP, FTP / FTPS or TFTP. Each protocol's tooltip states its security. File Transfer reopens the protocol you used last, and each protocol keeps its own settings, saved connections and Server/Client mode. Command search (Ctrl+Shift+P) also finds SFTP, FTP / FTPS and TFTP by name.

## Choose a protocol
- **SFTP:** SSH-based file browsing and transfers. It may reuse an already connected and authenticated terminal session where the remote device supports the SFTP subsystem.
- **FTP/FTPS:** file transfer using FTP, with TLS when FTPS is selected and supported.
- **TFTP:** lightweight file transfer where appropriate; be aware it does not provide SSH-style authentication or encryption.

## Before transferring
Review the remote host, protocol, local and remote paths, overwrite confirmation and permissions. Do not assume files are safe to replace without checking their destination and the consequence of overwriting.

Use the visible **Reset panes** action in the two-pane browser to restore the default layout. The divider can also be reset by double-clicking.

## During a transfer
Monitor progress and cancellations. Closing a borrowed SFTP operation should not terminate the interactive SSH terminal. Preserve open sessions and avoid writing incomplete data over valid files.
