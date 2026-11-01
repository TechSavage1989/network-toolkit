# Diagnostics

Use Network Toolkit to test reachability, routes, name resolution and TCP connectivity.

## Test a host
1. Open **Ping** and enter the target hostname or IP address. Start the test and inspect replies, loss and response times.
2. Use **Trace route** to see the sequence of hops to the target.
3. Use **DNS** to check name lookup and records.
4. Use **TCP ports** to test whether a particular service is reachable.

Each tool currently has its own input. The owner-approved target-first shared workspace is planned for Phase 6, not yet implemented. Search or press Ctrl+Shift+P to find all diagnostic tools by name.

## Other diagnostics
Network discovery, monitoring, wireless information, routes, ARP and connections are available as separate tools. Network tests operate only on networks and equipment you are authorized to inspect.

## Safety
Review the chosen target, scope and options before starting any scans. A timeout or an unanswered probe does not by itself prove the device is down. See [Network configuration](network-config.md) for protected system changes.
