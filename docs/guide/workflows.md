# Tool workflows

This reference preserves the detailed behavior documented previously under WORKFLOWS107.md. User-facing overview: [diagnostics](diagnostics.md), [settings and updates](settings-and-updates.md).

# Discovery, appearance and file workflows

These controls build on the 1.17.106 vault, terminal and transfer workflows.
They do not authorize a scan, terminal connection or upload merely by opening
settings, loading a filter or previewing a color.

## Saved sessions beside results

A **Saved SSH**, **Saved Telnet** or counted **Saved sessions** button identifies
saved terminal profiles for that exact IP address. Protocol and port are shown;
an endpoint-specific result must also match its protocol and port. A host-level
result lists saved configurations, not proof that a service is available. Open
the button to choose a profile and review the normal connection dialog. Vault
unlock, destination approval and host-key verification still apply.

IPv6 forms are normalized, but interface scope is not guessed. An unscoped
link-local address has no badge. Hostnames are not silently resolved to infer a
match. Changes to saved profiles update badges without another scan.

## Named filters and recorded sources

**Saved filters** in a pinned results toolbar can save, load, rename, update or
delete a named query and its applicable options. The dialog lists exactly which
options are included. Presets are scoped to the tool/result kind; incompatible
options are rejected rather than silently applied. Duplicate names are not
silently overwritten; Update and Delete require confirmation.

Discovery retains its multiline/space/comma alternatives, normalized MACs,
exact complete IPs and quoted text phrases. Presets contain no results,
credentials, scan options or connection actions. Limits are 20 per scope and
64 total, with bounded queries and total saved text. Existing current filters
are not converted into unnamed presets or replaced during upgrade.

The sidebar still selects the source for future diagnostics. **Recorded
source** below ordinary results shows that run's recorded source, even after the
sidebar selection changes. In a fitted ping/monitor graph, select an entry and
expand its existing **Run details** to see the recorded source without taking
space from the graph or sample rows. Before the first sample, Run details is
available in the empty result area. The redundant source button is no longer
in the header.

## Terminal appearance

Toolkit's automatic highlights are decorations on parsed terminal text, not
rewrites of received bytes. Recognized address/URL/prompt/interface and contextual
status or configuration rules take precedence over broad word matches. Ordinary
copyright/login prose is not interpreted as a configuration just because its
first word is “User” or “to”. Device-supplied colors remain distinct and are not
stripped. Rule switches, previews and reset live in Display; existing custom
colors and explicit disabled settings are retained.

The font field offers installed monospace candidates and permits a named local
font. Nothing is downloaded or installed. Missing fonts use the installed
fallback stack, with a detection notice. Missing glyphs can use OS fallback.
The live sample and existing size control remain; font readiness and display
changes remeasure terminal cells through the normal backend-resize path.

## Shared colors

App accent, diagnostic, terminal and packet colors use one editor with current
and draft previews, visual controls, strict HEX/RGB values, copy buttons,
presets, recent colors and saved swatches. **Apply** commits the draft; **Cancel**,
Escape and navigation restore the current setting. Reset is previewed before
Apply. Only transparency-capable settings expose alpha. Existing quick presets
and explicit HEX edits remain immediate actions where shown.

Recent colors retain up to 12 entries; saved colors retain up to 24. Both are
bounded color strings only, not arbitrary theme/configuration objects. Contrast
and semantic-color similarity notices are advisories, not a claim of universal
accessibility compliance; intentional custom colors are not silently rewritten.
Text status labels remain important as a second cue.

## Packet Capture approval

Opening/reopening Packet Capture, viewing retained results and refreshing
ordinary adapters do not launch an administrator helper. Adapter IDs remain
specific to the selection. An explicit Start can require Windows approval for
Packet Monitor. Dismissing approval leaves a visible failure and does not retry
until the user starts again. The main application stays unelevated; no UAC,
driver permission, service or scheduled-task settings are changed.

See [Packet Capture privileges](PACKET_CAPTURE_PRIVILEGES.md) for the operation
boundary and platform limitations.

## Files and Windows Explorer drops

File panels distinguish local paths, shared-root paths and remote paths, with
navigation and readable entries. The 1.17.106 **Copy location** formats remain
available and never initiate a transfer.

Supported file drops show the exact destination and a review before copying.
Original files are not moved or deleted. Name collisions have explicit
skip/rename/replace choices, and progress/cancellation/per-file errors distinguish
partial completion from complete success. A remote drop is an upload and needs
explicit destination confirmation; dragging over a panel sends no file data.
Folder drops are rejected as a whole where safe structured import is unsupported.
The reviewed file picker/drop path accepts up to 100 files, each at most 512 MiB.
The existing explicit Add by local path workflow remains available separately.
Shared-root confinement, path validation and existing permissions remain in force.

FTP/FTPS remote Explorer drops are deliberately unavailable: portable FTP rename
semantics cannot guarantee atomic no-clobber publication. Existing manual FTP
transfers remain available. Supported SFTP uploads use staged publication and
fail closed when required safe rename/replace capabilities are unavailable.
Shared FTP/FTPS server files are local-root copies and are not subject to this
remote FTP limitation.

## Verification scope

Tests use synthetic profiles, credentials, transcripts and files. No user device,
user file, real packet capture or Windows security configuration change is part
of these checks. Browser device-pixel-ratio cases are not the same as changing
real Windows display scaling. Hosted native WebView/Explorer acceptance covers
the runner's actual environment; user-specific display, installation, driver,
server and network behavior still varies. Exact completed results and remaining
limits are recorded in the development/release handoff.
