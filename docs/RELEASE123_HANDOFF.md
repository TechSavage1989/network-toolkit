# Network Toolkit 1.17.123 — consistent Terminal SFTP dragging

Published 2026-10-30T13:39:16Z; actual public downloads/source/feed verified
13:41:28.278568Z, independently verified 13:41:32.337464Z.
Release 429077724: https://github.com/TechSavage1989/network-toolkit/releases/tag/v1.17.123
Source/tag c0530bbb710e1023c68b835a565fbf89e19d74a8;
tree 71d2000215acbb2d027df22d47c090b31720c5f9. Expected-head/non-force main
promotion was verified at 13:37 UTC under the user's requested-build publication
authorization. Repository visibility/access and prior 122 assets remain unchanged.

Terminal SFTP rows now use the same genuine-pointer native copy handoff as the
SFTP tool and Editor, rather than the older DownloadURL Save/export dialog.
Selection is painted on existing rows so routine Terminal refreshes cannot replace
the row holding capture; true folder/listing/filter/session changes still invalidate
it. Main manual download and file-management actions remain. Detached Terminal
WORKSPACE read/navigation/export access is limited to its existing owned SSH
session, revision and original transport. A strict read-only request schema, separate
job namespace and lifetime checks add no credentials, general local-filesystem or
remote-write authority. Single-session detached windows retain their Macros-only
sidebar scope. The 122 physical-input/capture/ACK/cancellation pipeline remains.

The original 122 negative control reproduced exactly one fixture-owned Save
Network Toolkit export dialog, cancelled without save. The first 123 candidate
stopped before gestures because a Windows temporary-directory alias was not
canonicalized in its synthetic server; the test now follows unchanged production
transferRoot confinement. The next candidate's main-file gesture reached no export
request. A deterministic regression reproduced selection-only repaint replacing the
captured row. The final minimal row-cache correction passed unchanged real input
timings; neither confinement nor cancellation/test assertions were weakened.

Final review 41063349840 passed all eight real Terminal main/detached-workspace
file/folder/cancel-before/cancel-during cases (56.93 s), plus the existing ten native
SFTP/Editor file/folder/cancellation/internal-open cases (55.54 s). The published122
Save-dialog negative passed in 40.65 s. Exact destination bytes, nested and empty
folder hierarchy, source retention, session/transcript retention, zero candidate
Save dialogs and dirty-text/undo preservation in existing Editor cases were checked.
All ten actual PNGs were inspected. A transient ownership-move warning is visible
in the child-ready capture; owner ready=true and all subsequent child cases passed,
with normal expected cancellation feedback in the completed capture.

Local 1,371 JavaScript and 42 Python cases passed; application/Terminal/desktop/
vault Go suites passed. Focused backend Go-race passed (5.633 s initially, 2.820 s
with canonical fixture setup); Windows binaries cross-compiled. Final hosted core
confirmed 1,371 JavaScript, 42 Python and eight ordinary Go packages, not a full
hosted race claim. Full Windows/security/dependency/package gates passed before
promotion. Dependency scans found zero reachable/imported-package Go findings and
zero npm findings; the existing unused OpenPGP GO-2026-5932 advisory remains.

Review 41063349840: all seven SUCCESS; last job completed 2026-10-30T13:46:46Z.
Duplicate main 41064715467: six SUCCESS/one FAILURE (known Windows Alt-editor transfer); last job completed 2026-10-30T13:56:30Z.
The duplicate scoped negative/Terminal8/Editor10 suites passed again in
31.26/42.01/44.39 s. Its later native Alt-editor transfer failed (50.69 s):
real backend never recorded a pending transfer. This existing boundary is not
fixed or called fixture-only. Historical comparison/Hosts/visible-UAC limitations
remain. An earlier 123 review also recorded a main390-light second keyboard gear
focus timeout; pointer activation and seven other layouts passed. Relevant editor
files were byte-identical to122 and the new native handler inactive in that fixture,
but its cause remains unresolved. Later passes do not erase that observation.

Release 41064715306 build/publish passed. All four actual public downloads match
the reviewed portable/EXE bytes and API hashes. The 1,186 exact source files/1,193
portable members, both manifests, AMD64 GUI PE/version123/nine canonical icons/
12 resources, updater four-file allowlist, latest feed/tag/main/notes and prior122
immutable release/asset/tag fields were verified. No earlier release was replaced.

Evidence uses synthetic loopback SSH/SFTP and real isolated Windows Explorer folder
destinations, not live user files/devices, separate Desktop Shell, arbitrary external
apps, cross-integrity or visible UAC transitions. New Terminal cases use normal
Win32 timings, not a new exhaustive fast-gesture reliability guarantee. Existing
2 GiB/1,500-entry/32-depth exports and 16 MiB text bounds remain; FTP/FTPS native
export is not added. This source-only closure changes no app or published assets
and authorizes no unrelated repair or future release.

## Public asset identity

- `Network_Toolkit.exe`: 20,886,016 bytes; asset 692237641; SHA-256
  `6109620fa253785605a8d5c32b19454b027e616e42b8f73eff946c38506cb898`.
- `Network_Toolkit_1.17.123_Portable.zip`: 13,983,827 bytes; asset 692237646; SHA-256
  `35379b0bc3eaea9d7b36f042216524f510aabe8858d81f521517b0ca6d4625c6`.
- `Network_Toolkit_1.17.123_Update.zip`: 7,986,261 bytes; asset 692237639; SHA-256
  `224cfc99713c3b776f0d5ca7ce808c93dbc6f76dbc9b1bb67c8228256e6374b3`.
- `SHA256SUMS`: 292 bytes; asset 692237637; SHA-256
  `733f2fe9ca0fba954027030153ff4481c9fbdecf79a775555c506f764b9122c5`.

The candidate handoff packaged inside the immutable release predates publication;
this later source-only record adds final verification and terminal CI outcomes.
