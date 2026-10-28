# Packet Capture privilege boundary (1.17.107)

Opening Packet Capture, changing views, inspecting retained/imported packets and
refreshing adapters do not launch an administrator helper. Ordinary adapter
inventory uses the existing Windows `GetAdaptersAddresses` read path plus
read-only adapter metadata. These adapters are candidates, not proof that a
Packet Monitor engine can capture each one. The UI explains that limitation.

Only **Start capture** requests the existing fixed-role administrator helper.
Windows may prompt every time an actual capture is started. Toolkit does not
disable UAC, elevate the main app, change driver permissions or install a
privileged service/task. A cancelled Windows approval leaves a failed capture
with “No capture was started”; polling or navigation does not retry it. Press
Start again only when you want another attempt. Pending Start clicks are
coalesced in the UI and the manager already rejects concurrent active captures.

Ordinary choices carry a stable Windows adapter GUID. After explicit Start,
the helper resolves it against the full current Packet Monitor component list.
Existing unique driver/MAC matching remains intact: unknown, disappeared or
ambiguous adapters fail rather than silently capturing another adapter or all
adapters. “All configurable network adapters” remains an explicit choice.
Legacy live/file component IDs remain accepted. The helper's old `interfaces`
operation is now rejected before any launch; parent/image/pipe identity checks,
restricted capture options and ephemeral process lifetime are unchanged.

Microsoft documents administrator privileges for Packet Monitor initialization:
https://learn.microsoft.com/en-us/windows/win32/pktmon/packetmonitor/nf-packetmonitor-packetmonitorinitialize

## Synthetic validation and limits

- `packet_capture_adapters_test.go`: stable GUID inventory, ambiguity/invalid
  identifiers, exact adapter selection and legacy explicit choices.
- `packet_capture_privilege_windows_test.go`: actual Windows pipe boundary with
  a synthetic adapter reader and launch spy. Repeated capabilities/interface/
  retained-result requests launch nothing; obsolete/malformed helper requests
  are rejected; explicit Start receives synthetic UAC cancellation exactly once;
  polling does not retry, and a fresh Start can try again. No real runas call,
  Packet Monitor initialization or packet capture is executed by this test.
- `scripts/packet-capture.test.cjs`: navigation, ordinary refresh failure,
  stable adapter choices across both engines, duplicate Start and cancellation.
- `scripts/packet-capture-privilege.browser.cjs`: production browser controls
  with intercepted synthetic APIs, repeated navigation/refresh, pending double
  click, cancellation/reopen, explicit retry and normal/narrow screenshots.

Required Windows/native and real-browser execution is a hosted release gate.
Local unit/source checks alone do not establish Windows behavior. No actual
user adapter, packet capture, credentials, UAC setting or live device is tested.
Capture permissions, driver availability and real adapter support still depend
on the user's Windows installation; real capture is not promised without UAC.
