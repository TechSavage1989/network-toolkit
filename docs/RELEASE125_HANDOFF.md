# Network Toolkit 1.17.125 — selected adapter Enable/Disable

Scope is only explicit Enable adapter and Disable adapter in the existing Local
Network > IP configuration > Network actions. Restart adapter and all other 124
features remain. Disabled adapters are retained in the selector; administrative
state is separate from operational link state. Unknown state fails closed.
The disabled-only inventory does not invent IPv4/DNS values or make them editable.

The existing one-action elevated helper and review/serialization boundaries are
retained. Requests remain typed action, exact GUID/index/revision, adapter scope
and explicit confirmation; no renderer script, alias, wildcard or program.
The helper refreshes current identity/state after any UAC wait. Disable review
warns about loss of all connections and remaining disabled until explicit Enable.
No automatic re-enable, restart, retry or credential/access change is introduced.

Validation is mock/construction and intercepted-browser only. No live user or CI
adapter is disabled/enabled, and no visible UAC transition is tested. Windows
Enable-NetAdapter/Disable-NetAdapter use a unique current InputObject rather than
a friendly-name pattern. Read-only Get-NetAdapter inventory includes disabled
interfaces omitted from IP inventory. Existing IP/DNS configuration guards stay.

Official behavior references:
- https://learn.microsoft.com/powershell/module/netadapter/enable-netadapter
- https://learn.microsoft.com/powershell/module/netadapter/disable-netadapter
- https://learn.microsoft.com/powershell/module/netadapter/get-netadapter

Focused UI unit checks passed before integration; exact-source hosted UI,
Windows mock, security, application and package results are recorded at release
closure. Existing unchanged comparison/Alt-transfer/Hosts/gear and UAC evidence
limits remain; this focused change does not claim to fix those observations.
All old release tags/assets and repository visibility/access are preserved.
