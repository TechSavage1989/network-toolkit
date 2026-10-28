# 1.17.108 candidate handoff

## Baseline and authority

The user explicitly authorized this implementation/build/publication batch,
including later graph, editor comparison/change-map, native window ownership and
network-configuration syntax requests. The user separately reconfirmed source
and release publication in the existing public repository on 2026-10-28; no
repository access change is authorized. Verified main:
ab0d0daeff5e2ea8f5d4da457544fc96b8202fab, tree4b1ab9822ef4c033f320c972f3ed8ce784651448.
Latest published baseline: v1.17.107, release426199324, source/tag
bafabd88f9dd728acf45862226995e654821082d. Preserve existing repository visibility,
release workflow, published assets/tags, settings and security boundaries.

## Scoped implementation

- Authoritative numeric activity counts from actual terminal/file lifetimes,
  listeners, running jobs and captures; no status-induced network operation,
  elevation, credential read or persisted activity inference.
- Compact saved-profile bookmarks inside existing address/port content and
  details, exact IP/protocol/port matching and existing ID-only review/chooser.
- Status-colored graph samples, theme-accent connecting lines, stable hover and
  x-aligned plot selection with the pressed sample identity preserved.
- Existing multi-document editor strengthened with accessible tabs, independent
  editing state and local side-by-side comparison. A bounded full-document change
  map supports Fit/zoom, overview scrolling and click/drag-anywhere navigation.
- Explicit editor/document window transfers preserve single ownership and editing
  state; conservative supported-vendor text configuration recognition keeps
  manual/plain-text overrides. Empty-editor input now creates an in-memory Untitled
  document, with initial Undo and navigation retention; theme and soft-wrap
  rendering are tested in the required editor browser gate.

## Known architecture and evidence limits

FTP/FTPS clients currently use one authenticated transport per operation, not a
persistent idle socket. Activity therefore reports operations, not an idle form.
External browser/RDP launches do not expose a managed session lifetime. Detached
terminal sessions remain included in the main manager snapshot; hidden terminal-
only navigation does not need global activity polling or a broader API allowlist.

Only synthetic documents, profiles and safe test sessions are used. No live user
device/capture, real user vault or file, Windows security-setting change, font
installation or external document-processing service is involved. Publication
requires all existing gates plus new focused browser/pixel acceptance, and exact
public asset/source/version/feed checks. Candidate implementation and authored
tests are not a claim that those gates have passed.

## Editor and configuration limits

See [editor workflows](EDITOR_WORKFLOWS108.md), [window ownership](EDITOR_WINDOWS.md)
and [network syntax](NETWORK_CONFIG_EDITOR.md) for exact bounds and supported
formats. Native moves use explicit controls, not OS tab dragging. Desktop browser
File System Access handles and per-request FTP/FTPS credentials are not transferable;
those tabs remain in the original window with an explanation. Retained SSH and
local-document bindings use scoped leases, pinned transports and existing safe
file-write checks. No credential material is serialized into a document snapshot.

Recovered snapshots are bounded, in memory and last-acknowledged only. Confirmed
child process exit permits recovery; missing heartbeats do not reclaim ownership.
The engine does not promise recovery after an OS/engine restart. Main/independent
Terminal dirty-document consent does not become implicit approval after a timer.
Updater installation refuses open documents/moves and detached editor activity;
automatic feed checks and downloads retain their existing separate behavior.

Reference recognition is lexical, not semantic validation or a complete vendor
parser. Supported families include Aruba CX/AOS-S/Comware, FortiOS, SonicOS CLI
text, PAN-OS set/XML and Cisco IOS/XE/NX-OS/ASA where evidence is reliable. Ambiguous
text stays generic; binary/opaque exports are not decoded. Public tests/screenshots
use synthetic configurations only. Private reference inputs stay outside source,
artifacts, logs and package manifests.

## Corrected review checkpoint (not release acceptance)

First exact-source review2e7540183fb47ae4b7733c3c5cf8500a4b5dc27c / Checks40699835168
passed application, dependency, Windows and security jobs. Actual installed WebView2
editor close and two-host ownership tests passed, as did the existing native vault,
bootstrap and capture-boundary checks. The long browser's real updater timer passed.
The two browser jobs nevertheless failed; that source was not promoted or released.

Corrections preserve the original assertions: dense graph hits now resolve the
nearest timestamp rather than an overlapping transparent hit circle; status-color
oracles use the selected diagnostic palette rather than the old accent-only rule.
Editor navigation captures viewport state before hiding its DOM, and comparison
layers override inherited generic pre clipping. Browser fixtures now use rooted
assets and the existing responsive details Hide control instead of clicking through
its overlay. Visible gutter/palette/hit-test assertions were strengthened.

Corrected combined JavaScript tests passed968/968 and42 Python checks passed.
All required gates must run again on the corrected exact source, with actual
normal/narrow pixels and native proof rechecked. No first-run green is substituted
for this final acceptance. Unrelated diagnostic source-route/neighbor tests cannot
use this local sandbox's netlink interfaces; those required hosted checks remain
unskipped and passed in review1. Local browser launch was blocked before a page
ran; no alternate launch or permission bypass was attempted. Final package and
public asset/source/version/feed verification remain pending.
