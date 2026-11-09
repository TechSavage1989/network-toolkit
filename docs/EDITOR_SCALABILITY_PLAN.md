# Text Editor scalability

Owner requirement (2026-11-08): the editor must work effectively with text files of any size without making opening, scrolling, typing or the rest of the app laggy. This supersedes the bounded-file acceptance criteria for the unpublished v1.21.20 Beta 1 candidate. PR #139 carries the replacement and its verification.

## Design

- Keep large documents in a private disk-backed buffer. A balanced piece tree records byte, UTF-16 and line counts; it stores offsets into an append-only file rather than copies of the document.
- Undo and redo retain tree roots and edit fragments. They do not copy the entire source for every keystroke.
- Load only a bounded viewport into the renderer, with nearby pages prefetched. Long single lines must also be paged; DOM size and input work must not grow with total file length.
- Run import, indexing, search, comparison and save outside the UI thread, with progress and cancellation. Opening and saving still require physical disk or network I/O; zero time for unbounded input is not a possible guarantee.
- Save from a stable buffer revision using a temporary file, preserve encoding and line endings, detect external changes, and retain explicit overwrite consent. Newer edits made during save remain dirty.
- Move document handles between windows under the existing owner/lease protocol. Never send a full multi-gigabyte document or undo history in a recovery checkpoint.
- Preserve local and supported remote editing, split views, comparison, search, selection, undo/redo, Save As and dirty-close safeguards. A partial viewport must never be saved as if it were the complete file.
- Comparisons use streamed, paged results. They must not inherit a file-size or line-count ceiling from the current renderer or quadratic diff matrix.

## Verification

Use generated fixtures only. Cover UTF-8, BOM-marked UTF-16LE/BE, CRLF/LF/CR, Unicode at page boundaries, extremely long lines, millions of short lines, scattered edits, undo/redo, save while typing, external modification, cancellation, window ownership and recovery.

Measure 16 MiB, 256 MiB and multi-gigabyte files. Record open/index progress, viewport request latency, input-to-paint latency, UI long tasks, memory use, exact saved bytes and checksums. Interactive memory and work must depend on the viewport and edit history, not repeated full-file materialization. Initial indexing, whole-file searches and saves may take time proportional to bytes processed, while the app remains responsive.

## Implementation and verification

- Normal local, SFTP, FTP/FTPS and browser file opens use the engine for large text. Small documents retain the existing surface and switch to the paged editor as they grow. Browser handles stream imports and saves, with source conflict checks before the staged write commits.
- The paged editor keeps selections and document positions outside the viewport. Search and replacement run on snapshots, and edits made during a save stay dirty. Transfers carry disk handles and revisions, including the undo history.
- The comparison engine streams both revisions, writes paged differences to temporary storage and keeps the UI available for progress, navigation and cancellation. Large comparisons highlight whole lines and can group distant rearrangements. File, line and viewport packet sizes are independent. The size of a viewport or request does not limit the document.
- The real application browser fixture uses generated files, authenticated production editor handlers and the normal open, edit, compare and save actions. A run with two 2 GiB files opened both in 36.3 seconds, compared them in 3.5 seconds, and saved in 12.3 seconds. The 27 input samples had 6.6 ms input-to-frame p95. Frame p99 was 8.5 ms, the maximum frame gap was 50.2 ms, and no long tasks were recorded. It verified two differences, typing during save, undo, release of the document grants, and exact SHA-256 over 2,147,483,674 saved bytes.
- The browser storage fixture covers file import, writable handles, outside changes, reload, growing drafts, undo and CRLF Save As. The viewport fixture covers 8 GiB logical offsets and a single very long logical line. Focused race, encoding, conflict and remote lease tests use generated files and local protocol fixtures.
- The measured timings are from an isolated browser on Windows. Hosted native and platform checks run separately. Temporary disk usage grows with source and save size.
