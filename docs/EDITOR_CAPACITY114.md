# Editor capacity and window transfer (114)

## Explicit limits

- Local, imported, FTP/FTPS and SFTP text files: **16 MiB encoded**, up from 8 MiB. Binary files remain download-only. BOM, UTF-8/UTF-16, line endings and exact bytes are preserved by the existing codecs.
- An in-memory transferred document may contain **32 MiB of UTF-8 text** after decoding. The full serialized document batch, including original/saved text and all retained undo/redo, is limited to **256 MiB**. An oversized move is refused without dropping text or history; this is not a promise that every 16 MiB document with arbitrarily large history can move.
- JSON file-write requests have an explicit `16 MiB × 6 + 16 KiB` bound, allowing escaped control characters without changing the encoded-file limit. This is a substantial peak-memory case, not an unbounded request. A process-wide cancellable admission gate serializes heavy decoding, file codec/save work and editor snapshot responses before reading their bodies. Concurrent windows cannot each allocate a maximum-sized request simultaneously; small activity/pulse/ACK routes stay independent.
- Comparison remains separately bounded at 2,097,152 characters / 30,000 lines per document and the existing diff-work budget. Formatting and syntax highlighting retain their existing smaller budgets. No operation silently truncates the source.

## Large-file mode

Above 2 MiB characters, ordinary editing uses plain text, a virtual visible-line gutter and temporarily pauses soft Wrap. The user's Wrap preference remains saved and resumes for smaller documents. Search, line navigation, editing, selection, copy, exact save and undo/redo remain available. The status explicitly identifies this mode and its history budget.

Future edits in this mode retain at most 80 undo entries and an estimated 128 MiB of UTF-16 history strings, with at least the latest undo entry. This bound is applied while editing, **never merely because a document moves windows**. Existing normal-document behavior is unchanged. Transfers preserve all retained history or are refused.

## Avoided work, unchanged ownership

The backend copies mutable snapshot containers instead of repeatedly JSON-encoding/decoding immutable text. The renderer checkpoints unchanged documents using a metadata/version key instead of serializing full text every second. Changes, missing ownership and transfers still send complete bounded snapshots. Pending transfers are polled at 100 ms; idle polling remains 1 s. Source release still requires the destination's authoritative lease/ACK; timeout or lost responses never create competing editable copies.

Terminal polls at 250 ms while a detached child is open (to discover child-initiated Return), 100 ms during pending transitions and 1 s with no child. An explicit main-window Return request wakes its existing poll immediately. Session ownership and reconnect/authentication behavior are unchanged.

## Evidence and limits

`editor_capacity114_test.go` verifies >8 MiB Unicode/BOM/EOL roundtrips, exact 16 MiB acceptance, one-byte-over refusal without creating a destination, real temporary-file saves, escaped JSON and snapshot mutation isolation. The optional stress measurement includes test payload construction as well as handler work; it is not a pure production-server memory measurement.

The provisional 32 MiB encoded candidate was not accepted: the hosted narrow browser opened it in 1,954 ms and painted the test input in 377 ms, but the automation input round-trip took 21,778 ms and subsequent Undo timed out. The profile was dominated by idle/fetch time; the exact stall mechanism is not proven. The release therefore uses the practical **16 MiB encoded ceiling (twice the prior release)** rather than claiming unverified 32 MiB responsiveness. The failed measurements remain in the development evidence; no text was truncated to pass them.

Earlier isolated 32 MiB backend stress probes measured 458 ms / sampled 336,179,152-byte Go heap for ASCII and 2,575 ms / 956,922,376 bytes for control text with 201,326,742 request bytes. Those probes describe the rejected higher-capacity candidate, not the final file limit. Sampling every 10 ms can miss a higher instantaneous peak. At the selected 16 MiB limit, the same isolated probes measured 215 ms / 159,998,464-byte sampled Go heap for ASCII (16,777,367 request bytes), and 1,530 ms / 507,590,856 bytes for control text (100,663,447 request bytes). They include test payload construction and handler work, not just production-server memory. Normal/narrow/child browser acceptance remains part of the final exact-source gate.

Actual same-runner synthetic Windows tests used a real separate editor process and retained exact text, undo, selection and exclusive ownership over three detach/return cycles. The 4 MiB baseline median was 5,091 ms detached / 2,794 ms returned; the candidate measured 3,357 ms / 1,344 ms (approximately 34% / 52% faster in that run). Candidate 16 MiB medians were 5,618 ms / 3,305 ms. The measurement includes the test command round-trip and does not predict a user's machine. Final acceptance still requires the unchanged 5-second input bounds, exact save/search/undo, gutter geometry and actual normal/narrow/child captures at the selected 16 MiB limit. No live remote device or user's files are used.
