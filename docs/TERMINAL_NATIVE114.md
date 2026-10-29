# Native Terminal transfer acceptance and selection (114)

`TestNativeTerminalTransferInstalledWebView2` uses the actual installed Windows
WebView2 host and production detached-child executable, authenticated page and
stdin-only bootstrap. Its sole transport is a private loopback TCP fixture that
emits fixed synthetic text and never interprets input. No device, shell,
credential, production desktop or capture is used.

Run with `NETWORK_TOOLKIT_NATIVE_TERMINAL_TEST=1` and
`TOOLKIT_NATIVE_TEST_OUTPUT=<unique directory>`; choose
`TOOLKIT_NATIVE_TIMING_LABEL=113-baseline` or `114-candidate`. The same test-only
fixture can be applied to the immutable113 application source for baseline
measurements. Do not apply114 production changes to that baseline.

Three detach/return cycles record action-to-destination-editable timestamps,
backend ownership revisions, original session ID, retained output SHA-256 and
connection-accept count (must remain one). The source must stop accepting input
before acceptance is reported. Results are actual elapsed measurements, not a
promise of the same speed on other machines. Compare repeated observations on
the same runner; process/WebView initialization remains part of the measurement.

The optional `NETWORK_TOOLKIT_NATIVE_TERMINAL_GESTURES=1` extension is114-only:
actual Win32 mouse motion reorders a tab, Escape cancels, then an outside-client
release detaches through production ownership handling. Candidate coverage also
checks saved-profile names in actual tabs, cancellation of bulk confirmations,
Disconnect all retaining output, and Close all excluding a session owned by
another native window. Input and screenshots
are confined to validated fixture PIDs, executable identity and Toolkit window
class. The fixture does not synthesize DOM drag events or expose a production
bridge. It must pass on Windows before native gesture support is claimed.

`NETWORK_TOOLKIT_NATIVE_TERMINAL_SELECTION=1` additionally establishes selected
synthetic output and a non-bottom scroll position, then requires both through
all three candidate transfers. Nonempty command/pager archives retain their
separate existing coverage; unsolicited fixture output does not prove those.

Selection is an optional adjunct to the existing bounded emulator snapshot:
coordinates, length and output fingerprint, not extra plaintext or input.
Older snapshots without it remain valid. The full restored buffer is verified
before selection is applied; a mismatch blocks acceptance and clears the
incorrect selection. Resize anchoring uses public xterm buffer/cell/marker and
selection APIs, tracks wrapped logical positions, and compares the exact
selected text after reflow. Stale/trimmed selections are not redirected to
unrelated text. Existing256KiB emulator and screen snapshot limits are unchanged.

Local checks can validate bundled xterm parsing, serialization, Unicode reflow,
coordinate bounds and stale-output refusal, plus cross-compile the Windows
fixture. They do not substitute for actual hosted Windows/pixel acceptance.
