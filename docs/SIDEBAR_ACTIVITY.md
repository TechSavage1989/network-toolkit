# Live sidebar activity

A small green dot overlays a tool's existing navigation icon when the local
engine reports ongoing work. It does not add a column, widen the navigation or
move its label. The same dot appears in the collapsed icon rail and narrow-screen
navigation drawer. Hover text and the button's accessible name describe the
activity, so its meaning is not conveyed by color alone.

The dot is separate from saved-connection bookmarks:

- **Connected sessions** include idle terminal sessions, including sessions owned
  by another or detached window, and retained SSH file-browser connections.
- **Servers running** means successfully running/listening, even while idle.
- **Operations running** means actual running diagnostic or transfer work, not a
  queued request, previous result, completed operation or failed attempt.
- **Captures running** means an active capture, not opening the packet-capture
  page, loading saved packets or awaiting/declining administrator approval.

The SFTP / FTP / FTPS navigation item combines its applicable activity. Current
FTP/FTPS client requests create a transport for each operation; their cached
client form is not an idle authenticated backend session and does not light the
dot by itself. Existing manual FTP behavior is unchanged.

## State and update boundary

The frontend reads authenticated `GET /api/activity`. Its response contains only
allowlisted sidebar page IDs and numeric `connections`, `servers`, `operations`
and `captures` counts. It does not contain credentials, profile details, session
IDs, hostnames, paths, packet data or vault contents. Counts come from existing
backend lifecycles; opening navigation never connects, scans, probes a peer,
starts a transfer, enumerates privileged capture engines or launches a helper.

Each main window uses a coalesced one-second read, with a 2.5-second request
timeout and a four-second snapshot freshness bound. Engine disconnection, failed
or malformed reads clear the dots rather than retaining a misleading green state.
Late responses from an old request or disconnected generation are ignored. Focus
and return from a hidden window request a fresh snapshot. No activity is saved to
settings or restored from a previous application run. Terminal-only and detached
views have no primary sidebar and do not poll this endpoint; their sessions remain
included in the main window's backend-owned count.

A passive status snapshot does not test remote reachability. Peer loss is reflected
when observed by the existing transport lifecycle; this feature adds no network
probe or keepalive traffic merely to display a dot. No system security setting or
capture permission policy changes.

## Validation

`scripts/sidebar-activity.test.cjs` covers count sanitization, inactive state,
multiple/last sessions, idle servers, metadata restoration, unchanged nodes,
coalescing, timeout, errors, late responses, offline/restart generations and
focus/page lifecycle handling. `scripts/sidebar-activity108.browser.cjs` exercises
the production navigation against synthetic backend responses, two main windows,
terminal-only mode, light/dark, collapsed/narrow layouts and browser DPR2, with
actual screenshots and before/after geometry assertions. Backend lifecycle and
side-effect boundaries are tested separately. Synthetic browser state and browser
DPR are not live-device or native Windows DPI validation; final hosted execution
and screenshot acceptance are recorded in the release handoff.
