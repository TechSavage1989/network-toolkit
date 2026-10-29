# Editor capacity and window transfer (114)

## Explicit limits

- Local, imported, FTP/FTPS and SFTP text files: **32 MiB encoded**, up from 8 MiB. Binary files remain download-only. BOM, UTF-8/UTF-16, line endings and exact bytes are preserved by the existing codecs.
- An in-memory transferred document may contain **64 MiB of UTF-8 text** after decoding. The full serialized document batch, including original/saved text and all retained undo/redo, is limited to **256 MiB**. An oversized move is refused without dropping text or history; this is not a promise that every 32 MiB document with arbitrarily large history can move.
- JSON file-write requests have an explicit `32 MiB × 6 + 16 KiB` bound, allowing escaped control characters without changing the encoded-file limit. This is a substantial peak-memory case, not an unbounded request. A process-wide cancellable admission gate serializes heavy decoding, file codec/save work and editor snapshot responses before reading their bodies. Concurrent windows cannot each allocate a maximum-sized request simultaneously; small activity/pulse/ACK routes stay independent.
- Comparison remains separately bounded at 2,097,152 characters / 30,000 lines per document and the existing diff-work budget. Formatting and syntax highlighting retain their existing smaller budgets. No operation silently truncates the source.

## Large-file mode

Above 2 MiB characters, ordinary editing uses plain text, a virtual visible-line gutter and temporarily pauses soft Wrap. The user's Wrap preference remains saved and resumes for smaller documents. Search, line navigation, editing, selection, copy, exact save and undo/redo remain available. The status explicitly identifies this mode and its history budget.

Future edits in this mode retain at most 80 undo entries and an estimated 128 MiB of UTF-16 history strings, with at least the latest undo entry. This bound is applied while editing, **never merely because a document moves windows**. Existing normal-document behavior is unchanged. Transfers preserve all retained history or are refused.

## Avoided work, unchanged ownership

The backend copies mutable snapshot containers instead of repeatedly JSON-encoding/decoding immutable text. The renderer checkpoints unchanged documents using a metadata/version key instead of serializing full text every second. Changes, missing ownership and transfers still send complete bounded snapshots. Pending transfers are polled at 100 ms; idle polling remains 1 s. Source release still requires the destination's authoritative lease/ACK; timeout or lost responses never create competing editable copies.

Terminal polls at250ms while a detached child is open (to discover child-initiated Return),100ms during pending transitions and1s with no child. An explicit main-window Return request wakes its existing poll immediately. Session ownership and reconnect/authentication behavior are unchanged.

## Evidence and limits

`editor_capacity114_test.go` verifies >8 MiB Unicode/BOM/EOL roundtrips, exact 32 MiB acceptance, one-byte-over refusal without creating a destination, real temporary-file saves, escaped JSON and snapshot mutation isolation. The optional stress measurement includes test payload construction as well as handler work; it is not a pure production-server memory measurement.

On the Linux development executor, a 32 MiB ASCII save took 458 ms with a sampled Go-heap peak of 336,179,152 bytes; worst-case 32 MiB control text (201,326,742 request bytes) took 2,575 ms and 956,922,376 bytes. Sampling every 10 ms can miss a higher instantaneous peak. These figures do not predict user-machine performance.

The synthetic 4 MiB native Windows baseline uses the real separate editor process and preserves exact text, undo, selection and exclusive ownership over three cycles: detach 4,778 / 4,527 / 4,523 ms; return 2,590 / 3,158 / 3,307 ms. This is a **baseline only**, not a speedup claim. Final acceptance requires the same hosted runner conditions for baseline/candidate and actual 16/32 MiB browser interaction captures. The added browser fixture records open/input/checkpoint time, available heap data, long tasks, exact save/search/undo and normal/narrow/child images. No live remote device or user's files are used.
