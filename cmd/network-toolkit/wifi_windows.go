//go:build windows

package main

import (
	"context"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"unicode/utf16"
	"unsafe"
)

// Native WLAN API only: no netsh key=clear, subprocess output, profile export,
// temporary XML, or extra runtime. API buffers are freed on every path.
// https://learn.microsoft.com/windows/win32/api/wlanapi/nf-wlanapi-wlangetprofile
// https://learn.microsoft.com/windows/win32/nativewifi/wi-fi-access-location-changes
var localWLANDLL = syscall.NewLazyDLL(filepath.Join(localSystemDirectory, "wlanapi.dll"))
var (
	localWLANOpen     = localWLANDLL.NewProc("WlanOpenHandle")
	localWLANClose    = localWLANDLL.NewProc("WlanCloseHandle")
	localWLANFree     = localWLANDLL.NewProc("WlanFreeMemory")
	localWLANEnum     = localWLANDLL.NewProc("WlanEnumInterfaces")
	localWLANQuery    = localWLANDLL.NewProc("WlanQueryInterface")
	localWLANProfiles = localWLANDLL.NewProc("WlanGetProfileList")
	localWLANProfile  = localWLANDLL.NewProc("WlanGetProfile")
)

type localWLANGUID struct {
	Data1 uint32
	Data2 uint16
	Data3 uint16
	Data4 [8]byte
}

type localWLANInterfaceInfo struct {
	GUID        localWLANGUID
	Description [256]uint16
	State       uint32
}
type localWLANInterfaceList struct {
	Count uint32
	Index uint32
	Items [1]localWLANInterfaceInfo
}
type localWLANProfileInfo struct {
	Name  [256]uint16
	Flags uint32
}
type localWLANProfileList struct {
	Count uint32
	Index uint32
	Items [1]localWLANProfileInfo
}
type localWLANSSID struct {
	Length uint32
	Value  [32]byte
}
type localWLANAssociation struct {
	SSID          localWLANSSID
	BSSType       uint32
	BSSID         [6]byte
	_             [2]byte
	PHYType       uint32
	PHYIndex      uint32
	SignalQuality uint32
	RXRate        uint32
	TXRate        uint32
}
type localWLANSecurity struct {
	Enabled        uint32
	OneX           uint32
	Authentication uint32
	Cipher         uint32
}
type localWLANConnection struct {
	State       uint32
	Mode        uint32
	ProfileName [256]uint16
	Association localWLANAssociation
	Security    localWLANSecurity
}

func localWLANGUIDFromString(value string) (localWLANGUID, error) {
	var out localWLANGUID
	if !validAdapterID(value) {
		return out, &localWiFiAPIError{"validation", "The wireless adapter identity is invalid."}
	}
	data, err := hex.DecodeString(strings.ReplaceAll(value[1:len(value)-1], "-", ""))
	if err != nil || len(data) != 16 {
		return out, &localWiFiAPIError{"validation", "The wireless adapter identity is invalid."}
	}
	out.Data1, out.Data2, out.Data3 = binary.BigEndian.Uint32(data[:4]), binary.BigEndian.Uint16(data[4:6]), binary.BigEndian.Uint16(data[6:8])
	copy(out.Data4[:], data[8:])
	return out, nil
}

func (g localWLANGUID) String() string {
	return fmt.Sprintf("{%08X-%04X-%04X-%02X%02X-%02X%02X%02X%02X%02X%02X}", g.Data1, g.Data2, g.Data3,
		g.Data4[0], g.Data4[1], g.Data4[2], g.Data4[3], g.Data4[4], g.Data4[5], g.Data4[6], g.Data4[7])
}

func localWLANError(status uintptr, current bool) error {
	if status == 0 {
		return nil
	}
	switch uint32(status) {
	case 5: // ERROR_ACCESS_DENIED, including WLAN plaintext-read policy.
		if current {
			return &localWiFiAPIError{"permission", "Windows denied current Wi-Fi connection details. Check Windows Settings > Privacy & security > Location. Saved profiles can still be viewed when permitted."}
		}
		return &localWiFiAPIError{"permission", "Windows denied access to this Wi-Fi profile or its stored key."}
	case 1168, 1167: // ERROR_NOT_FOUND / ERROR_DEVICE_NOT_CONNECTED.
		return &localWiFiAPIError{"not_found", "The selected Wi-Fi adapter or profile is no longer available. Refresh saved profiles."}
	case 50, 120: // ERROR_NOT_SUPPORTED / ERROR_CALL_NOT_IMPLEMENTED.
		return &localWiFiAPIError{"unsupported", "This Windows installation does not support the requested Wi-Fi profile operation."}
	case 1058, 1062, 1722: // Service disabled/not active/RPC unavailable.
		return &localWiFiAPIError{"unavailable", "The Windows WLAN AutoConfig service is unavailable. Enable the Wi-Fi adapter and WLAN AutoConfig service, then refresh."}
	case 1223:
		return &localWiFiAPIError{"cancelled", "The Windows administrator prompt was cancelled. The Wi-Fi key was not shown."}
	default:
		return &localWiFiAPIError{"failed", fmt.Sprintf("Windows could not read Wi-Fi information (error %d). Refresh and try again.", uint32(status))}
	}
}

type localNativeWiFi struct {
	handle syscall.Handle
}

func openLocalWiFi(ctx context.Context) (*localNativeWiFi, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if localSystemDirectoryError != nil {
		return nil, &localWiFiAPIError{"unavailable", "Windows could not locate its native Wi-Fi API."}
	}
	for _, proc := range []*syscall.LazyProc{localWLANOpen, localWLANClose, localWLANFree, localWLANEnum, localWLANQuery, localWLANProfiles, localWLANProfile} {
		if err := proc.Find(); err != nil {
			return nil, &localWiFiAPIError{"unavailable", "The Windows native Wi-Fi API is unavailable on this computer."}
		}
	}
	var negotiated uint32
	api := &localNativeWiFi{}
	status, _, _ := localWLANOpen.Call(2, 0, uintptr(unsafe.Pointer(&negotiated)), uintptr(unsafe.Pointer(&api.handle)))
	if status != 0 {
		return nil, localWLANError(status, false)
	}
	if err := ctx.Err(); err != nil {
		api.close()
		return nil, err
	}
	return api, nil
}

func (api *localNativeWiFi) close() {
	if api.handle != 0 {
		localWLANClose.Call(uintptr(api.handle), 0)
		api.handle = 0
	}
}

func (api *localNativeWiFi) interfaces(ctx context.Context) ([]localWiFiInterfaceRef, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	var pointer *localWLANInterfaceList
	status, _, _ := localWLANEnum.Call(uintptr(api.handle), 0, uintptr(unsafe.Pointer(&pointer)))
	if pointer != nil {
		defer localWLANFree.Call(uintptr(unsafe.Pointer(pointer)))
	}
	if status != 0 {
		return nil, localWLANError(status, false)
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if pointer == nil || pointer.Count > 256 {
		return nil, &localWiFiAPIError{"failed", "Windows returned an invalid or excessively large Wi-Fi adapter list."}
	}
	result := make([]localWiFiInterfaceRef, 0, pointer.Count)
	for _, item := range unsafe.Slice(&pointer.Items[0], int(pointer.Count)) {
		result = append(result, localWiFiInterfaceRef{ID: item.GUID.String(), Description: syscall.UTF16ToString(item.Description[:]), State: item.State})
	}
	return result, nil
}

func (api *localNativeWiFi) current(ctx context.Context, id string) (*localWiFiConnection, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	guid, err := localWLANGUIDFromString(id)
	if err != nil {
		return nil, err
	}
	var size, valueType uint32
	var pointer *localWLANConnection
	status, _, _ := localWLANQuery.Call(uintptr(api.handle), uintptr(unsafe.Pointer(&guid)), 7, 0,
		uintptr(unsafe.Pointer(&size)), uintptr(unsafe.Pointer(&pointer)), uintptr(unsafe.Pointer(&valueType)))
	runtime.KeepAlive(guid)
	if pointer != nil {
		defer localWLANFree.Call(uintptr(unsafe.Pointer(pointer)))
	}
	if status != 0 {
		return nil, localWLANError(status, true)
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if pointer == nil || size < uint32(unsafe.Sizeof(localWLANConnection{})) || pointer.Association.SSID.Length > 32 {
		return nil, &localWiFiAPIError{"failed", "Windows returned invalid current Wi-Fi connection details."}
	}
	ssid, ssidHex := localWiFiSSID(pointer.Association.SSID.Value[:pointer.Association.SSID.Length])
	signal := pointer.Association.SignalQuality
	if signal > 100 {
		signal = 100
	}
	return &localWiFiConnection{ProfileName: syscall.UTF16ToString(pointer.ProfileName[:]), SSID: ssid, SSIDHex: ssidHex,
		Authentication: localWLANAuthentication(pointer.Security.Authentication), Encryption: localWLANCipher(pointer.Security.Cipher), SignalQuality: signal}, nil
}

func (api *localNativeWiFi) profiles(ctx context.Context, id string) ([]localWiFiProfileRef, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	guid, err := localWLANGUIDFromString(id)
	if err != nil {
		return nil, err
	}
	var pointer *localWLANProfileList
	status, _, _ := localWLANProfiles.Call(uintptr(api.handle), uintptr(unsafe.Pointer(&guid)), 0, uintptr(unsafe.Pointer(&pointer)))
	runtime.KeepAlive(guid)
	if pointer != nil {
		defer localWLANFree.Call(uintptr(unsafe.Pointer(pointer)))
	}
	if status != 0 {
		return nil, localWLANError(status, false)
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if pointer == nil || pointer.Count > 4096 {
		return nil, &localWiFiAPIError{"failed", "Windows returned an invalid or excessively large saved Wi-Fi profile list."}
	}
	result := make([]localWiFiProfileRef, 0, pointer.Count)
	for _, item := range unsafe.Slice(&pointer.Items[0], int(pointer.Count)) {
		name := syscall.UTF16ToString(item.Name[:])
		if validateLocalWiFiRequest(localWiFiRevealRequest{InterfaceID: id, ProfileName: name}) != nil || item.Flags & ^uint32(3) != 0 {
			return nil, &localWiFiAPIError{"failed", "Windows returned an invalid saved Wi-Fi profile identity."}
		}
		result = append(result, localWiFiProfileRef{Name: name, Flags: item.Flags})
	}
	return result, nil
}

func (api *localNativeWiFi) profile(ctx context.Context, id, name string, plaintext bool) ([]byte, uint32, error) {
	if err := ctx.Err(); err != nil {
		return nil, 0, err
	}
	if err := validateLocalWiFiRequest(localWiFiRevealRequest{InterfaceID: id, ProfileName: name}); err != nil {
		return nil, 0, &localWiFiAPIError{"validation", err.Error()}
	}
	guid, err := localWLANGUIDFromString(id)
	if err != nil {
		return nil, 0, err
	}
	profileName, _ := syscall.UTF16PtrFromString(name)
	var flags, granted uint32
	if plaintext {
		flags = 4 // WLAN_PROFILE_GET_PLAINTEXT_KEY, explicit selected-profile call only.
	}
	var pointer *uint16
	status, _, _ := localWLANProfile.Call(uintptr(api.handle), uintptr(unsafe.Pointer(&guid)), uintptr(unsafe.Pointer(profileName)), 0,
		uintptr(unsafe.Pointer(&pointer)), uintptr(unsafe.Pointer(&flags)), uintptr(unsafe.Pointer(&granted)))
	runtime.KeepAlive(guid)
	runtime.KeepAlive(profileName)
	if pointer != nil {
		defer localWLANFree.Call(uintptr(unsafe.Pointer(pointer)))
	}
	if status != 0 {
		return nil, 0, localWLANError(status, false)
	}
	if pointer == nil {
		return nil, 0, &localWiFiAPIError{"failed", "Windows returned an empty Wi-Fi profile."}
	}
	// The API has no size output. Its contract is a NUL-terminated UTF-16
	// allocation; bound the scan and reject oversized profiles before parsing.
	count := 0
	for count < localWiFiXMLLimit/2 && *(*uint16)(unsafe.Add(unsafe.Pointer(pointer), count*2)) != 0 {
		count++
	}
	units := unsafe.Slice(pointer, count)
	// Clear the native allocation before WlanFreeMemory. Metadata may contain
	// encrypted keyMaterial too, so clear that buffer on both paths.
	defer clear(units)
	if count == localWiFiXMLLimit/2 {
		return nil, 0, &localWiFiAPIError{"unsupported", "The selected Wi-Fi profile exceeds its supported size."}
	}
	if err := ctx.Err(); err != nil {
		return nil, 0, err
	}
	runes := utf16.Decode(units)
	defer clear(runes)
	data := []byte(string(runes))
	if len(data) > localWiFiXMLLimit {
		clear(data)
		return nil, 0, &localWiFiAPIError{"unsupported", "The selected Wi-Fi profile exceeds its supported size."}
	}
	return data, flags & 3, nil
}

func localWLANAuthentication(value uint32) string {
	values := map[uint32]string{1: "open", 2: "shared", 3: "WPA", 4: "WPAPSK", 5: "WPANone", 6: "WPA2", 7: "WPA2PSK", 8: "WPA3ENT192", 9: "WPA3SAE", 10: "OWE", 11: "WPA3ENT"}
	if name := values[value]; name != "" {
		return name
	}
	return fmt.Sprintf("Unknown (%d)", value)
}

func localWLANCipher(value uint32) string {
	values := map[uint32]string{0: "none", 1: "WEP40", 2: "TKIP", 4: "AES", 5: "WEP104", 6: "BIP-CMAC-128", 8: "GCMP-128", 9: "GCMP-256", 10: "CCMP-256", 11: "BIP-GMAC-128", 12: "BIP-GMAC-256", 13: "BIP-CMAC-256", 0x100: "Group cipher", 0x101: "WEP"}
	if name := values[value]; name != "" {
		return name
	}
	return fmt.Sprintf("Unknown (%d)", value)
}

func localWiFiProcessSID(process syscall.Handle) (string, error) {
	var token syscall.Token
	if err := syscall.OpenProcessToken(process, syscall.TOKEN_QUERY, &token); err != nil {
		return "", err
	}
	defer token.Close()
	user, err := token.GetTokenUser()
	if err != nil {
		return "", err
	}
	return user.User.Sid.String()
}

func localWiFiOwnSID() (string, error) {
	process, err := syscall.GetCurrentProcess()
	if err != nil {
		return "", err
	}
	return localWiFiProcessSID(process)
}

func readLocalWiFi(ctx context.Context) (localWiFiSnapshot, error) {
	api, err := openLocalWiFi(ctx)
	if err != nil {
		return localWiFiSnapshot{Elevated: localElevated(), Interfaces: []localWiFiInterface{}, Note: localWiFiError(err, localElevated()).Error}, nil
	}
	defer api.close()
	return readLocalWiFiFrom(ctx, api, localElevated())
}

func revealLocalWiFi(ctx context.Context, request localWiFiRevealRequest) localWiFiResult {
	elevated := localElevated()
	api, err := openLocalWiFi(ctx)
	if err != nil {
		return localWiFiError(err, elevated)
	}
	defer api.close()
	selection, err := selectLocalWiFiProfile(ctx, api, request)
	if err != nil {
		return localWiFiError(err, elevated)
	}
	result := revealLocalWiFiSelected(ctx, api, selection, elevated)
	if !request.Elevate || !result.CanElevate || result.ErrorCode != "permission" {
		return result
	}
	sid, err := localWiFiOwnSID()
	if err != nil {
		return localWiFiFailure("failed", "Windows could not verify the requesting account for administrator approval.", false)
	}
	helper := localWiFiHelperRequest{InterfaceID: selection.id, ProfileName: selection.ref.Name, ProfileFlags: selection.ref.Flags,
		RequesterSID: sid, Fingerprint: localWiFiFingerprint(selection.id, selection.ref, selection.parsed), MetadataKnown: selection.parsed != nil}
	return localElevatedWiFi(ctx, helper)
}

func localPerformWiFiHelper(ctx context.Context, request localWiFiHelperRequest, parent syscall.Handle) localWiFiResult {
	if err := validateLocalWiFiHelper(&request); err != nil {
		return localWiFiFailure("validation", err.Error(), false)
	}
	parentSID, err := localWiFiProcessSID(parent)
	if err != nil {
		return localWiFiFailure("permission", "Windows could not verify the account that requested this Wi-Fi key.", false)
	}
	ownSID, err := localWiFiOwnSID()
	if err != nil || parentSID != request.RequesterSID || (request.ProfileFlags&2 != 0 && parentSID != ownSID) {
		return localWiFiFailure("permission", "A per-user Wi-Fi profile must be read with the same Windows account. Administrator approval using a different account cannot reveal this profile.", false)
	}
	api, err := openLocalWiFi(ctx)
	if err != nil {
		return localWiFiError(err, true)
	}
	defer api.close()
	selection, err := selectLocalWiFiProfile(ctx, api, localWiFiRevealRequest{InterfaceID: request.InterfaceID, ProfileName: request.ProfileName})
	if err != nil {
		return localWiFiError(err, true)
	}
	if err := validateLocalWiFiHelperSelection(request, selection, parentSID, ownSID); err != nil {
		return localWiFiError(err, true)
	}
	return revealLocalWiFiSelected(ctx, api, selection, true)
}

func localElevatedWiFi(ctx context.Context, request localWiFiHelperRequest) localWiFiResult {
	response, _, err := localHelperRoundTrip(ctx, localHelperRequest{Version: localToolsProtocolVersion, WiFi: &request})
	if err != nil {
		if errors.Is(err, syscall.Errno(1223)) {
			return localWiFiFailure("cancelled", "The Windows administrator prompt was cancelled. The Wi-Fi key was not shown.", false)
		}
		return localWiFiError(err, false)
	}
	if response.WiFi == nil || response.Result != nil {
		return localWiFiFailure("failed", "The administrator helper returned an invalid Wi-Fi response.", false)
	}
	if err := ctx.Err(); err != nil {
		if response.WiFi.Reply != nil {
			response.WiFi.Reply.Secret = ""
		}
		return localWiFiError(err, false)
	}
	return *response.WiFi
}
