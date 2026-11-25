# Network Configuration

Open **Local network** in the Configure group. Its tabs show IP configuration, the routing table, ARP / neighbors, connections, Wi-Fi and the **Hosts file**. Protected changes to network and system state require explicit review and the appropriate Windows/UAC permission.

IP configuration shows each IPv4 address with its full subnet mask and prefix, for example `192.168.120.11` with `255.255.254.0 (/23)`; the Overview's adapters and Local IPv4 card show the mask the same way. When you edit the settings, **Subnet mask or prefix length** accepts either form (`255.255.254.0`, `23` or `/23`).

## Before applying a change
1. Select the correct adapter or system item.
2. Review the intended changes to IP settings, network interface state or the Hosts file.
3. Verify that remote access will remain available and that required administrative privileges are present.
4. Approve the specific operation in Windows when prompted.

Changing an interface or its IP configuration can disconnect your current network session. The application must not remove approval or review steps to reduce click count. Use these functions only on systems that you are authorized to configure.
