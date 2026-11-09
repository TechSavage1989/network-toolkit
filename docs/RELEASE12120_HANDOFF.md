# Network Toolkit v1.21.20 Beta 1

The editor integration in PR #139 routes large documents through disk storage, a paged editor and streamed comparison. File-size and line-count limits from the original candidate no longer apply to the app's large-file paths. See [editor scalability](EDITOR_SCALABILITY_PLAN.md) for architecture and measured results.

- Normal file open, edit, comparison, save, streamed browser imports and writable browser handles use the new storage path. Smaller documents switch as they grow.
- Generated 16 MiB, 256 MiB and 2 GiB fixtures exercise the normal editor. The 2 GiB case verified typing during save, two navigable differences and exact saved SHA-256. Hosted Checks include that real-file scenario and the browser storage fixture.
- Large-file views use plain text and logical lines. Large comparisons show whole-line differences and may group distant rearrangements.

Publish as a new opt-in beta through the guarded release workflow after both Checks runs pass. Verify the release tag, executable, portable and updater ZIPs, Linux archives, SHA256SUMS and stable-channel isolation.
