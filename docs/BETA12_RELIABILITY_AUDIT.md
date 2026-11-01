# Beta 12 - SFTP cancellation and concurrent operation audit

Candidate source, not published. No real-device traffic or desktop access.

## SFTP cancellation and terminal lifetime

The borrowed SSH connection must outlive a cancelled scoped SFTP subsystem. Production source explicitly checks context cancellation before allocating a channel, after negotiation and after the operation. A cancellation close that breaks a pending read must return context.Canceled rather than a late EOF; pre-cancelled requests must not create a channel. Synthetic regression: a stalled listing, cancellation, server-side SFTP channel close, live shell write, subsequent SFTP probe and pre-cancelled no-new-channel check.

These checks do not demonstrate all native SFTP interruption/cancel behaviors with real servers, nor guarantee remote mutations can be reversed after cancellation. Existing confirmation/permission checks are preserved.

## Cross-tool and resource observations

Baseline Beta 9 tests exercise 1,000 snapshots of six diagnostic types; Beta 12 adds 800 simulated snapshots across Ping, Trace route, DNS, TCP, discovery and monitoring, with 21 synthetic interruptions and four UI page selections. After each successful snapshot, only six job-backed views and filters should remain. Failed polls preserve prior data. Record elapsed milliseconds, process CPU user/system milliseconds, heap/RSS deltas in CI output; do not infer memory-leak absence or performance improvement from a single shared-host run. No real capture data, device scan or transfer happens in the synthetic stress fixture.

## Editor and other deferred acceptance

The original 390px visual comparison-motion p95 limit is 2 CSS pixels. The last published builds have reproduced around 3-5 pixel offsets. Beta 12 does not weaken the threshold or claim that source alignment and sidebar resizing are fixed. Other open criteria: native SFTP cancellation on real devices, physical Windows multi-hour resource soak, full cross-tool capture/transfer load, and real network interruption/reconnect. Keep all eleven areas in the permanent roadmap.
