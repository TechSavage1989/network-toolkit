# Diagnostics

Use Network Toolkit to test reachability, routes, name resolution and TCP connectivity.

## Test a host
Open **Test a target** in the Diagnose group and enter the hostname or IP address once. The tabs at the top switch between tests; the target stays the same:
1. **Ping:** replies, loss and response times.
2. **Trace route:** the sequence of hops to the target.
3. **DNS lookup:** name lookup and records.
4. **TCP ports:** whether a particular service is reachable.

Each tab shows a dot for its latest run against the current target: green finished, yellow running, red failed, faint not yet run. Switching tabs never starts a test. Test a target reopens the test you used last, and command search (Ctrl+Shift+P) still finds Ping, Trace route, DNS lookup and TCP ports by name.

## Other diagnostics
Network discovery, monitoring, wireless information, routes, ARP and connections are available as separate tools. Network tests operate only on networks and equipment you are authorized to inspect.

## Safety
Review the chosen target, scope and options before starting any scans. A timeout or an unanswered probe does not by itself prove the device is down. See [Network configuration](network-config.md) for protected system changes.
