# Diagnostics

Use Network Toolkit to test reachability, routes, name resolution and TCP connectivity.

## Test a host
Open **Test a target** in the Diagnose group and enter the hostname or IP address once. The tabs at the top switch between tests; the target stays the same:
1. **Device check:** name lookup, ping and selected TCP ports in one run.
2. **Ping:** replies, loss and response times. Select *Run until stopped* to keep watching.
3. **Trace route:** the sequence of hops to the target.
4. **DNS:** name lookup and records.
5. **Ports:** whether particular TCP services are reachable.
6. **Monitor:** repeated ICMP or TCP checks with a response-time chart, for a sample count or until stopped.
7. **Details:** hostname, MAC address and model information a single IPv4 device advertises.

In wide windows the Configuration panel sits beside the results: drag the divider to resize it, double-click it to hide it, and use ⇄ at the bottom of the panel (or of its narrow strip when hidden) to move it to the other side. Entry details, device details in Network discovery and packet details in Packet capture have a position menu (Auto, Right, Left or Bottom).

Each tab shows a dot for its latest run against the current target: green finished, yellow running, red failed, faint not yet run. Switching tabs never starts a test. Test a target reopens the test you used last, and command search (Ctrl+Shift+P) still finds every test by its full name (for example Connection monitor or Device details).

Drag a tab to put the tests in the order you prefer, or focus a tab and press Alt+Shift+Left or Right. Test a target and Test a website each keep their own order, and it is kept when you restart the app. By default Test a target lists Monitor, Ping, Trace route, DNS, Ports, Device check and Details.

Ping, Ports, Monitor and Trace route check one target. To check every device on a network or address range, select **Scan a network in Network discovery**; saved ranges also open there.

## Test a website
Open **Test a website** and enter the address once:
- **HTTP check:** status, timing, redirects and headers. A bare hostname such as `example.com` is checked as `https://example.com`.
- **TLS certificate:** the server certificate, validity and trust result. A full address such as `https://example.com/login` is checked using its hostname.

## Packet capture
Packet capture records this computer's network traffic while a problem happens, then explains what it found in everyday words. You don't need to know how to read packets.
1. Open **Packet capture**. In **Set up a new capture**, say what the problem is, choose the network connection (Wi-Fi or Ethernet) and, if you know it, enter the website or device that fails. Select **Start**.
2. While it records, try the failing website, app or device once. The recording panel shows the time, the packet count and what to do.
3. Select **Stop and analyse**.

The result starts with a verdict, such as *2 problems found* or *No problems found in this capture*. Each problem has a plain headline (for example *The name files.example.com does not exist*), what it usually means and what to check next. **Technical details** and **View example packets** keep the exact evidence.

**Check a website or device** follows one name or address through the recording, step by step: finding its address, reaching it, connecting and its reply. It shows the step where it stopped. It reads the recording only; nothing is sent.

**Conversations** shows whether each connection worked, was refused or got no reply, with service names (such as Remote Desktop or Secure web) and device names learned from name lookups in the recording. **Save as** stores the packets as PCAPNG or PCAP for other packet tools, or as a CSV, JSON or text summary. You can also open a saved PCAP or PCAPNG file.

A recording sees this computer's traffic, not every device on the network, and encrypted contents stay encrypted. A short recording with no findings does not prove the network is healthy.

## Other diagnostics
Network discovery, SMTP test and Packet capture have their own entries. Adapters, routes, ARP, connections, Wi-Fi and the Hosts file are tabs in **Local network**. Network tests operate only on networks and equipment you are authorized to inspect.

## Safety
Review the chosen target, scope and options before starting any scans. A timeout or an unanswered probe does not by itself prove the device is down. See [Network configuration](network-config.md) for protected system changes.
