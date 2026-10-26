package main

import (
	"context"
	"encoding/hex"
	"errors"
	"sort"
	"strings"
)

type localWiFiInterfaceRef struct {
	ID, Description string
	State           uint32
}
type localWiFiProfileRef struct {
	Name  string
	Flags uint32
}

// Injectable native operations make the security/selection rules testable
// with fixtures. Tests never enumerate or retrieve this computer's keys.
type localWiFiProvider interface {
	interfaces(context.Context) ([]localWiFiInterfaceRef, error)
	current(context.Context, string) (*localWiFiConnection, error)
	profiles(context.Context, string) ([]localWiFiProfileRef, error)
	profile(context.Context, string, string, bool) ([]byte, uint32, error)
}

type localWiFiAPIError struct {
	code, message string
}

func (e *localWiFiAPIError) Error() string { return e.message }

func localWiFiError(err error, elevated bool) localWiFiResult {
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return localWiFiFailure("cancelled", "The Wi-Fi key request was cancelled or timed out.", false)
	}
	var api *localWiFiAPIError
	if errors.As(err, &api) {
		return localWiFiFailure(api.code, api.message, api.code == "permission" && !elevated)
	}
	return localWiFiFailure("failed", "Windows could not read this Wi-Fi profile. Refresh saved profiles and try again.", false)
}

func localWiFiState(state uint32) string {
	switch state {
	case 0:
		return "not_ready"
	case 1:
		return "connected"
	case 2:
		return "ad_hoc_network_formed"
	case 3:
		return "disconnecting"
	case 4:
		return "disconnected"
	case 5:
		return "associating"
	case 6:
		return "discovering"
	case 7:
		return "authenticating"
	default:
		return "unknown"
	}
}

func localWiFiMetadata(ref localWiFiProfileRef, parsed localWiFiParsedProfile) localWiFiProfile {
	kind, note := localWiFiKeyKind(parsed)
	return localWiFiProfile{Name: ref.Name, Scope: localWiFiScope(ref.Flags), SSID: parsed.ssid, SSIDHex: parsed.ssidHex,
		Authentication: parsed.authentication, Encryption: parsed.encryption, KeyKind: kind,
		CanReveal: kind == "passphrase" || kind == "wep_key" || kind == "raw_psk", Note: note}
}

func readLocalWiFiFrom(ctx context.Context, api localWiFiProvider, elevated bool) (localWiFiSnapshot, error) {
	snapshot := localWiFiSnapshot{Available: true, Elevated: elevated, Interfaces: []localWiFiInterface{}}
	interfaces, err := api.interfaces(ctx)
	if err != nil {
		return snapshot, err
	}
	for _, iface := range interfaces {
		if err := ctx.Err(); err != nil {
			return snapshot, err
		}
		item := localWiFiInterface{ID: iface.ID, Description: iface.Description, State: localWiFiState(iface.State), Profiles: []localWiFiProfile{}}
		// Current connection metadata can be denied by Windows location privacy.
		// Saved profiles remain useful and are enumerated independently.
		if iface.State == 1 || iface.State == 2 {
			item.Current, err = api.current(ctx, iface.ID)
			if err != nil {
				item.CurrentError = localWiFiError(err, elevated).Error
			}
		}
		refs, e := api.profiles(ctx, iface.ID)
		if e != nil {
			item.ProfilesError = localWiFiError(e, elevated).Error
		} else {
			for _, ref := range refs {
				if err := ctx.Err(); err != nil {
					return snapshot, err
				}
				p := localWiFiProfile{Name: ref.Name, Scope: localWiFiScope(ref.Flags), KeyKind: "unknown"}
				// Never request WLAN_PROFILE_GET_PLAINTEXT_KEY during enumeration.
				data, flags, e := api.profile(ctx, iface.ID, ref.Name, false)
				if e == nil {
					parsed, pe := parseLocalWiFiProfile(data, false)
					if pe == nil && parsed.name == ref.Name && flags == ref.Flags {
						p = localWiFiMetadata(ref, parsed)
					} else {
						p.Note = "This profile changed or has an unsupported format. Refresh saved profiles."
					}
				} else {
					result := localWiFiError(e, elevated)
					p.Note = result.Error
					p.CanReveal = result.ErrorCode == "permission"
				}
				clear(data)
				p.Current = item.Current != nil && item.Current.ProfileName == ref.Name
				item.Profiles = append(item.Profiles, p)
			}
		}
		sort.SliceStable(item.Profiles, func(i, j int) bool {
			if item.Profiles[i].Current != item.Profiles[j].Current {
				return item.Profiles[i].Current
			}
			a, b := strings.ToLower(item.Profiles[i].Name), strings.ToLower(item.Profiles[j].Name)
			if a == b {
				return item.Profiles[i].Name < item.Profiles[j].Name
			}
			return a < b
		})
		snapshot.Interfaces = append(snapshot.Interfaces, item)
	}
	sort.SliceStable(snapshot.Interfaces, func(i, j int) bool {
		if (snapshot.Interfaces[i].State == "connected") != (snapshot.Interfaces[j].State == "connected") {
			return snapshot.Interfaces[i].State == "connected"
		}
		return strings.ToLower(snapshot.Interfaces[i].Description) < strings.ToLower(snapshot.Interfaces[j].Description)
	})
	if len(snapshot.Interfaces) == 0 {
		snapshot.Note = "Windows did not report a wireless adapter. Connect or enable a Wi-Fi adapter to view its saved profiles."
	}
	return snapshot, ctx.Err()
}

type localWiFiSelection struct {
	id          string
	ref         localWiFiProfileRef
	parsed      *localWiFiParsedProfile
	metadataErr error
}

func selectLocalWiFiProfile(ctx context.Context, api localWiFiProvider, request localWiFiRevealRequest) (localWiFiSelection, error) {
	selection := localWiFiSelection{id: request.InterfaceID}
	if err := validateLocalWiFiRequest(request); err != nil {
		return selection, &localWiFiAPIError{"validation", err.Error()}
	}
	if err := ctx.Err(); err != nil {
		return selection, err
	}
	interfaces, err := api.interfaces(ctx)
	if err != nil {
		return selection, err
	}
	found := false
	for _, iface := range interfaces {
		if strings.EqualFold(iface.ID, request.InterfaceID) {
			selection.id, found = iface.ID, true
			break
		}
	}
	if !found {
		return selection, &localWiFiAPIError{"not_found", "The selected wireless adapter is no longer present. Refresh saved profiles."}
	}
	refs, err := api.profiles(ctx, selection.id)
	if err != nil {
		return selection, err
	}
	found = false
	for _, ref := range refs {
		if ref.Name == request.ProfileName { // Windows names are case-sensitive.
			selection.ref, found = ref, true
			break
		}
	}
	if !found {
		return selection, &localWiFiAPIError{"not_found", "The selected Wi-Fi profile is no longer present for this account. Refresh saved profiles."}
	}
	data, flags, err := api.profile(ctx, selection.id, selection.ref.Name, false)
	defer clear(data)
	if err != nil {
		var apiError *localWiFiAPIError
		if errors.As(err, &apiError) && apiError.code == "permission" {
			// A listed all-user profile can deny metadata read. Its exact name,
			// scope and caller account are still bound to an explicit UAC retry.
			selection.metadataErr = err
			return selection, nil
		}
		return selection, err
	}
	parsed, err := parseLocalWiFiProfile(data, false)
	if err != nil {
		return selection, &localWiFiAPIError{"unsupported", "This Wi-Fi profile has an unsupported format."}
	}
	if flags != selection.ref.Flags || parsed.name != selection.ref.Name {
		return selection, &localWiFiAPIError{"conflict", "The selected Wi-Fi profile changed. Refresh saved profiles."}
	}
	selection.parsed = &parsed
	return selection, nil
}

func revealLocalWiFiSelected(ctx context.Context, api localWiFiProvider, selection localWiFiSelection, elevated bool) localWiFiResult {
	if err := ctx.Err(); err != nil {
		return localWiFiError(err, elevated)
	}
	if selection.metadataErr != nil {
		return localWiFiError(selection.metadataErr, elevated)
	}
	if selection.parsed == nil {
		return localWiFiFailure("failed", "Windows did not return the selected Wi-Fi profile.", false)
	}
	kind, note := localWiFiKeyKind(*selection.parsed)
	if kind == "none" || kind == "enterprise" {
		return localWiFiFailure("no_key", note, false)
	}
	if kind == "unknown" {
		return localWiFiFailure("unsupported", note, false)
	}
	data, flags, err := api.profile(ctx, selection.id, selection.ref.Name, true)
	defer clear(data)
	if err != nil {
		return localWiFiError(err, elevated)
	}
	if err = ctx.Err(); err != nil {
		return localWiFiError(err, elevated)
	}
	parsed, err := parseLocalWiFiProfile(data, true)
	defer func() { parsed.secret = "" }()
	if err != nil {
		return localWiFiFailure("failed", "Windows returned an invalid Wi-Fi key response.", false)
	}
	if flags != selection.ref.Flags || parsed.name != selection.ref.Name || localWiFiFingerprint(selection.id, selection.ref, &parsed) != localWiFiFingerprint(selection.id, selection.ref, selection.parsed) {
		return localWiFiFailure("conflict", "The selected Wi-Fi profile changed. Refresh saved profiles.", false)
	}
	// Microsoft documents success with an encrypted key when plaintext access
	// is denied. Never display that encrypted blob as a user's password.
	if !parsed.protectedKnown {
		return localWiFiFailure("unsupported", "Windows did not identify whether the stored key is plaintext.", false)
	}
	if parsed.protected {
		return localWiFiFailure("permission", "Windows did not allow this account to read the Wi-Fi key in plaintext.", !elevated)
	}
	if parsed.secret == "" || len(parsed.secret) > 1024 || strings.ContainsRune(parsed.secret, 0) {
		return localWiFiFailure("no_key", "Windows did not return a usable stored Wi-Fi key.", false)
	}
	if kind == "raw_psk" {
		decoded, e := hex.DecodeString(parsed.secret)
		clear(decoded)
		if e != nil || len(parsed.secret) != 64 {
			return localWiFiFailure("unsupported", "Windows returned an unsupported raw pre-shared key format.", false)
		}
	}
	refs, err := api.profiles(ctx, selection.id)
	if err != nil {
		return localWiFiError(err, elevated)
	}
	present := false
	for _, ref := range refs {
		if ref.Name == selection.ref.Name && ref.Flags == selection.ref.Flags {
			present = true
			break
		}
	}
	if !present {
		return localWiFiFailure("conflict", "The selected Wi-Fi profile changed. Refresh saved profiles.", false)
	}
	if err := ctx.Err(); err != nil {
		return localWiFiError(err, elevated)
	}
	return localWiFiResult{Reply: &localWiFiRevealResponse{InterfaceID: selection.id, ProfileName: selection.ref.Name,
		SSID: parsed.ssid, SSIDHex: parsed.ssidHex, Authentication: parsed.authentication, Encryption: parsed.encryption,
		Scope: localWiFiScope(selection.ref.Flags), KeyKind: kind, Secret: parsed.secret}}
}

func validateLocalWiFiHelperSelection(request localWiFiHelperRequest, selection localWiFiSelection, parentSID, helperSID string) error {
	if err := validateLocalWiFiHelper(&request); err != nil {
		return &localWiFiAPIError{"validation", err.Error()}
	}
	if request.RequesterSID != parentSID || (request.ProfileFlags&2 != 0 && helperSID != parentSID) {
		return &localWiFiAPIError{"permission", "A per-user Wi-Fi profile must be read with the same Windows account. Administrator approval using a different account cannot reveal this profile."}
	}
	if !strings.EqualFold(request.InterfaceID, selection.id) || request.ProfileName != selection.ref.Name || request.ProfileFlags != selection.ref.Flags {
		return &localWiFiAPIError{"conflict", "The selected Wi-Fi profile changed during administrator approval. Refresh saved profiles."}
	}
	parsed := selection.parsed
	if !request.MetadataKnown {
		parsed = nil
	}
	if localWiFiFingerprint(selection.id, selection.ref, parsed) != request.Fingerprint {
		return &localWiFiAPIError{"conflict", "The selected Wi-Fi profile changed during administrator approval. Refresh saved profiles."}
	}
	return nil
}
