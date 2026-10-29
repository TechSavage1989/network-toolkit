# Editor Files and Connections

The editor has a persistent, collapsible sidebar. Files and Connections use the
same compact tab language as Terminal. Files contains Windows files / SFTP
browsing, navigation and file operations. Connections contains active and saved
choices, authentication, host verification and session management. Selecting a
sidebar tab or collapsing the sidebar does not connect, disconnect, replace a
transport, reconstruct the existing browser form or change an open document.
Existing explicit saved-profile and detached-access approval flows remain.

The sidebar starts expanded on a wide new workspace and collapsed on a narrow
new workspace. Its Files rail and existing toolbar button reopen it. Main-window
width, collapse, primary tab and file-location choices persist in normal app
settings. A detached window retains these non-secret UI choices in its own
session storage; it gains no application-settings write capability. Existing
remote Open/create-by-path operations remain in a compact Files footer.

Local drives are buttons, not a dropdown. Each shows a drive letter or full mount
path, followed by its actual volume label when available. A path plus drive type
is the distinct fallback. Selected roots use pressed state; long labels have
full tooltips and horizontal overflow rather than adding permanent rows.
Unavailable selections report the existing listing error and retain the prior
folder and selected root.

## Bounded Windows volume metadata

Mounted roots still come from the existing Windows inventory. Optional label
reads use GetVolumeInformationW, in at most four in-flight workers. A listing
waits at most 75 ms total for labels, then returns path/type fallbacks. Reads are
cached for 30 seconds, capped at 256 entries; an unavailable device cannot create
one worker on every refresh. Network drive labels are not probed. Refreshing the
listing may obtain a label that completed after the first bounded wait.

Only the label read's locked OS thread temporarily adds SEM_FAILCRITICALERRORS,
then restores its original error mode, preventing removable-media error dialogs.
No process-wide setting, disk contents or network connection is changed.
Primary API references:
- https://learn.microsoft.com/windows/win32/api/fileapi/nf-fileapi-getvolumeinformationw
- https://learn.microsoft.com/windows/win32/api/errhandlingapi/nf-errhandlingapi-setthreaderrormode

## Verification

Focused JavaScript tests cover lexical app-state persistence, fixed DOM identity,
connection guards, child protocol refusal, unique drive labels and selected-root
rendering. Injected Go race tests cover cache reuse, network exclusion, bounded
wait and worker count. TestLocalDriveNames115 exercises the hosted Windows drive
inventory. editor-files115.browser.cjs covers actual main/child normal/narrow
sidebar switching, input retention, collapse, directory height, unavailable
selection and dirty-document identity with synthetic data. Existing saved-login
and explorer fixtures use the new explicit Connections tab without weakening
host-key, vault or file-operation assertions. Hosted execution and pixel review
are required separately; local unit tests do not prove native Windows behavior.
