# Network Configuration

Some tools inspect local network adapter, address, routing and Hosts information. Protected changes to network and system state require explicit review and the appropriate Windows/UAC permission.

## Before applying a change
1. Select the correct adapter or system item.
2. Review the intended changes to IP settings, network interface state or the Hosts file.
3. Verify that remote access will remain available and that required administrative privileges are present.
4. Approve the specific operation in Windows when prompted.

Changing an interface or its IP configuration can disconnect your current network session. The application must not remove approval or review steps to reduce click count. Use these functions only on systems that you are authorized to configure.
