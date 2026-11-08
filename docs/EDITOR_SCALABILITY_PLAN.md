# Text Editor scalability

Owner requirement (2026-11-08): the editor must work effectively with text files of any size without making opening, scrolling, typing or the rest of the app laggy. This supersedes the bounded-file acceptance criteria for the unpublished v1.21.20 Beta 1 candidate. PR #139 remains a draft until the full replacement is implemented and verified.

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

## Current status

- Existing candidate: reduces repeated gutter rendering and large comparison work, but still uses full strings, full-state checkpoints and fixed input limits. It does not meet the revised requirement.
- Implemented foundation: disk-backed persistent AVL piece tree; bounded UTF-16-position windows; fragment edits; undo/redo; streamed UTF-8/UTF-16 import and save; cancellation; existing document lease integration; local overwrite/conflict checks and staged remote save. It is not connected to the visible editor yet.
- Verified locally with generated fixtures: randomized editing/undo/redo, Unicode and encoding boundaries, extremely long lines, 64 MiB exact saved content, ownership transfer, stale-lease rejection, Save As consent and external modification. All application packages compile. Hosted Windows native regressions remain required.
- Engine benchmark, Windows synthetic fixtures: 256 MiB imported in 2.104 seconds with approximately 0.324 MiB retained Go heap; 2 GiB imported in 19.811 seconds with approximately 2.516 MiB retained Go heap. The same batch of 100 edits plus 100 viewport reads took 28.7 ms and 30.3 ms respectively. These measurements exclude UI, network transfer, OS cache and disk storage; they are not end-to-end application latency claims.
- Required before publication: lease-aware API and lifetime management, virtual editor integration, remote and comparison paths, full regression checks and performance evidence. No unrestricted-size support is claimed until those paths are complete.
