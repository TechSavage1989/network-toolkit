package main

import (
	"bytes"
	"context"
	"encoding/json"
	"encoding/xml"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

const localWiFiFixtureID = "{01234567-89AB-CDEF-0123-456789ABCDEF}"

func localWiFiFixtureXML(name, auth, cipher, keyType, protected, key string) []byte {
	escape := func(value string) string {
		var out bytes.Buffer
		_ = xml.EscapeText(&out, []byte(value))
		return out.String()
	}
	shared := ""
	if keyType != "" {
		shared = "<sharedKey><keyType>" + escape(keyType) + "</keyType><protected>" + protected + "</protected><keyMaterial>" + escape(key) + "</keyMaterial></sharedKey>"
	}
	return []byte(`<?xml version="1.0" encoding="UTF-16"?><WLANProfile xmlns="` + localWiFiProfileNamespace + `"><name>` + escape(name) +
		`</name><SSIDConfig><SSID><hex>4C61622D57694669</hex><name>Lab-WiFi</name></SSID><nonBroadcast>false</nonBroadcast></SSIDConfig>` +
		`<connectionType>ESS</connectionType><connectionMode>auto</connectionMode><MSM><security><authEncryption><authentication>` + escape(auth) +
		`</authentication><encryption>` + escape(cipher) + `</encryption><useOneX>false</useOneX></authEncryption>` + shared + `</security></MSM></WLANProfile>`)
}

type localWiFiFixtureProvider struct {
	ifaces       []localWiFiInterfaceRef
	refs         []localWiFiProfileRef
	metadata     map[string][]byte
	plaintext    map[string][]byte
	currentValue *localWiFiConnection
	currentErr   error
	metaErr      error
	plainErr     error
	onPlain      func()
	onProfiles   func(int)
	profileCalls []struct {
		id, name string
		plain    bool
	}
	currentCalls, listCalls int
}

func newLocalWiFiFixture(name, auth, cipher, keyType, key string) *localWiFiFixtureProvider {
	return &localWiFiFixtureProvider{
		ifaces:    []localWiFiInterfaceRef{{ID: localWiFiFixtureID, Description: "Fixture Wi-Fi", State: 4}},
		refs:      []localWiFiProfileRef{{Name: name}},
		metadata:  map[string][]byte{name: localWiFiFixtureXML(name, auth, cipher, keyType, "true", "fixture-protected-blob")},
		plaintext: map[string][]byte{name: localWiFiFixtureXML(name, auth, cipher, keyType, "false", key)},
	}
}

func (f *localWiFiFixtureProvider) interfaces(context.Context) ([]localWiFiInterfaceRef, error) {
	return append([]localWiFiInterfaceRef{}, f.ifaces...), nil
}
func (f *localWiFiFixtureProvider) current(context.Context, string) (*localWiFiConnection, error) {
	f.currentCalls++
	return f.currentValue, f.currentErr
}
func (f *localWiFiFixtureProvider) profiles(context.Context, string) ([]localWiFiProfileRef, error) {
	f.listCalls++
	if f.onProfiles != nil {
		f.onProfiles(f.listCalls)
	}
	return append([]localWiFiProfileRef{}, f.refs...), nil
}
func (f *localWiFiFixtureProvider) profile(_ context.Context, id, name string, plaintext bool) ([]byte, uint32, error) {
	f.profileCalls = append(f.profileCalls, struct {
		id, name string
		plain    bool
	}{id, name, plaintext})
	var flags uint32
	for _, ref := range f.refs {
		if ref.Name == name {
			flags = ref.Flags
		}
	}
	if plaintext {
		if f.onPlain != nil {
			f.onPlain()
		}
		return append([]byte{}, f.plaintext[name]...), flags, f.plainErr
	}
	return append([]byte{}, f.metadata[name]...), flags, f.metaErr
}

func localWiFiFixtureReveal(ctx context.Context, f *localWiFiFixtureProvider, name string, elevated bool) localWiFiResult {
	selection, err := selectLocalWiFiProfile(ctx, f, localWiFiRevealRequest{InterfaceID: localWiFiFixtureID, ProfileName: name})
	if err != nil {
		return localWiFiError(err, elevated)
	}
	return revealLocalWiFiSelected(ctx, f, selection, elevated)
}

func TestLocalWiFiMetadataDoesNotRequestOrExposeKeys(t *testing.T) {
	name, secret := "  Réseau 日本語 🛜  ", "fixture plaintext must not be listed"
	f := newLocalWiFiFixture(name, "WPA2PSK", "AES", "passPhrase", secret)
	// Even if Windows returns an unprotected metadata XML, metadata fields
	// must not retain or serialize keyMaterial.
	f.metadata[name] = localWiFiFixtureXML(name, "WPA2PSK", "AES", "passPhrase", "false", secret)
	f.ifaces[0].State = 1
	f.currentErr = &localWiFiAPIError{"permission", "Location access was denied; saved profiles remain available."}
	snapshot, err := readLocalWiFiFrom(context.Background(), f, false)
	if err != nil || !snapshot.Available || len(snapshot.Interfaces) != 1 || len(snapshot.Interfaces[0].Profiles) != 1 {
		t.Fatalf("metadata read failed: %#v, %v", snapshot, err)
	}
	p := snapshot.Interfaces[0].Profiles[0]
	if p.Name != name || p.SSID != "Lab-WiFi" || !p.CanReveal || p.KeyKind != "passphrase" || snapshot.Interfaces[0].CurrentError == "" {
		t.Fatalf("localized metadata or partial current failure lost: %#v", snapshot)
	}
	data, _ := json.Marshal(snapshot)
	if bytes.Contains(data, []byte(secret)) || bytes.Contains(data, []byte("keyMaterial")) || bytes.Contains(data, []byte("fixture-protected-blob")) {
		t.Fatal("secret or encrypted key material leaked into metadata")
	}
	for _, call := range f.profileCalls {
		if call.plain {
			t.Fatal("metadata enumeration requested plaintext")
		}
	}
	f.ifaces[0].State = 4
	f.currentCalls = 0
	if _, err := readLocalWiFiFrom(context.Background(), f, false); err != nil || f.currentCalls != 0 {
		t.Fatal("disconnected metadata unnecessarily queried current connection")
	}
}

func TestLocalWiFiRevealDistinguishesKeyTypes(t *testing.T) {
	cases := []struct {
		name, auth, cipher, keyType, key, kind, errorCode string
	}{
		{"WPA2", "WPA2PSK", "AES", "passPhrase", "  fixture passphrase  ", "passphrase", ""},
		{"WPA3", "WPA3SAE", "AES", "passPhrase", "fixture-sae", "passphrase", ""},
		{"raw PSK", "WPA2PSK", "AES", "networkKey", strings.Repeat("a1", 32), "raw_psk", ""},
		{"invalid raw", "WPA2PSK", "AES", "networkKey", "not-a-64-hex-key", "", "unsupported"},
		{"WEP", "open", "WEP", "networkKey", "0102030405", "wep_key", ""},
		{"Enterprise", "WPA2", "AES", "", "", "", "no_key"},
		{"WPA3 Enterprise", "WPA3ENT192", "GCMP256", "", "", "", "no_key"},
		{"Open", "open", "none", "", "", "", "no_key"},
		{"Enhanced open", "OWE", "AES", "", "", "", "no_key"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := newLocalWiFiFixture(tc.name, tc.auth, tc.cipher, tc.keyType, tc.key)
			result := localWiFiFixtureReveal(context.Background(), f, tc.name, false)
			if result.ErrorCode != tc.errorCode {
				t.Fatalf("unexpected reveal error: %#v", result)
			}
			if tc.errorCode == "" && (result.Reply == nil || result.Reply.Secret != tc.key || result.Reply.KeyKind != tc.kind) {
				t.Fatal("key type or exact key contents changed")
			}
			if tc.errorCode == "no_key" {
				for _, call := range f.profileCalls {
					if call.plain {
						t.Fatal("open/enterprise profile requested plaintext")
					}
				}
			}
		})
	}
}

func TestLocalWiFiSuccessWithProtectedKeyRequiresPermission(t *testing.T) {
	f := newLocalWiFiFixture("Protected", "WPA2PSK", "AES", "passPhrase", "unused")
	f.plaintext["Protected"] = localWiFiFixtureXML("Protected", "WPA2PSK", "AES", "passPhrase", "true", "fixture-encrypted-output")
	for _, elevated := range []bool{false, true} {
		result := localWiFiFixtureReveal(context.Background(), f, "Protected", elevated)
		if result.ErrorCode != "permission" || result.CanElevate != !elevated || result.Reply != nil {
			t.Fatalf("protected result exposed or treated as plaintext: %#v", result)
		}
		data, _ := json.Marshal(result)
		if bytes.Contains(data, []byte("fixture-encrypted-output")) {
			t.Fatal("protected blob escaped via an error")
		}
	}
	f.plaintext["Protected"] = bytes.ReplaceAll(f.plaintext["Protected"], []byte("<protected>true</protected>"), nil)
	if result := localWiFiFixtureReveal(context.Background(), f, "Protected", false); result.ErrorCode != "unsupported" || result.Reply != nil || result.CanElevate {
		t.Fatalf("ambiguous protection status accepted: %#v", result)
	}
}

func TestLocalWiFiParserRejectsAmbiguityAndNamespaceSpoofing(t *testing.T) {
	base := localWiFiFixtureXML("Profile", "WPA2PSK", "AES", "passPhrase", "false", "fixture-secret")
	for _, bad := range [][]byte{
		bytes.Replace(base, []byte("<protected>false</protected>"), []byte("<protected>false</protected><protected>true</protected>"), 1),
		bytes.Replace(base, []byte("<protected>false</protected>"), []byte("<protected>maybe</protected>"), 1),
		bytes.Replace(base, []byte("fixture-secret</keyMaterial>"), []byte("fixture-secret<unexpected/></keyMaterial>"), 1),
		bytes.Replace(base, []byte("</WLANProfile>"), []byte("</WLANProfile><WLANProfile/>"), 1),
		[]byte(`<!DOCTYPE WLANProfile [<!ENTITY secret SYSTEM "file:///etc/hosts">]><WLANProfile/>`),
	} {
		p, err := parseLocalWiFiProfile(bad, true)
		if err == nil || p.secret != "" || strings.Contains(err.Error(), "fixture-secret") {
			t.Fatal("ambiguous/malformed profile accepted or key leaked in parser error")
		}
	}
	spoof := bytes.Replace(base, []byte("<protected>false</protected>"), []byte(`<protected xmlns="urn:foreign">false</protected>`), 1)
	p, err := parseLocalWiFiProfile(spoof, true)
	if err != nil || p.protectedKnown {
		t.Fatal("foreign protected element was treated as WLAN protection metadata")
	}
	spoof = bytes.Replace(base, []byte("<sharedKey>"), []byte(`<sharedKey><keyMaterial xmlns="urn:foreign">wrong</keyMaterial>`), 1)
	p, err = parseLocalWiFiProfile(spoof, true)
	if err != nil || p.secret != "fixture-secret" {
		t.Fatal("foreign keyMaterial changed the selected shared WLAN key")
	}
	meta, err := parseLocalWiFiProfile(base, false)
	if err != nil || meta.secret != "" {
		t.Fatal("metadata parser retained plaintext")
	}
	ssid, encoded := localWiFiSSID([]byte{0xff, 0, 1})
	if ssid != "SSID FF0001" || encoded != "FF0001" {
		t.Fatal("non-text SSID did not retain its exact octets")
	}
}

func TestLocalWiFiExactSelectionAndCancellation(t *testing.T) {
	f := newLocalWiFiFixture("RéSeau", "WPA2PSK", "AES", "passPhrase", "fixture-secret")
	for _, request := range []localWiFiRevealRequest{
		{InterfaceID: localWiFiFixtureID, ProfileName: "réseau"},
		{InterfaceID: "{FFFFFFFF-FFFF-FFFF-FFFF-FFFFFFFFFFFF}", ProfileName: "RéSeau"},
	} {
		if _, err := selectLocalWiFiProfile(context.Background(), f, request); err == nil || localWiFiError(err, false).ErrorCode != "not_found" {
			t.Fatal("a different adapter or case-folded profile was substituted")
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	f.onPlain = cancel
	result := localWiFiFixtureReveal(ctx, f, "RéSeau", false)
	if result.ErrorCode != "cancelled" || result.Reply != nil {
		t.Fatalf("cancelled native result exposed a key: %#v", result)
	}
	f = newLocalWiFiFixture("Profile", "WPA2PSK", "AES", "passPhrase", "fixture-secret")
	f.plaintext["Profile"] = bytes.ReplaceAll(f.plaintext["Profile"], []byte("4C61622D57694669"), []byte("4E65774E6574"))
	if result := localWiFiFixtureReveal(context.Background(), f, "Profile", false); result.ErrorCode != "conflict" || result.Reply != nil {
		t.Fatal("profile replacement between metadata and plaintext reads was not detected")
	}
	f = newLocalWiFiFixture("Profile", "WPA2PSK", "AES", "passPhrase", "fixture-secret")
	f.onProfiles = func(calls int) {
		if calls > 1 {
			f.refs = nil
		}
	}
	if result := localWiFiFixtureReveal(context.Background(), f, "Profile", false); result.ErrorCode != "conflict" || result.Reply != nil {
		t.Fatal("removed profile still returned a key")
	}
}

func TestLocalWiFiHelperBindsAccountScopeAndProfile(t *testing.T) {
	const user = "S-1-5-21-100-200-300-1001"
	const admin = "S-1-5-21-100-200-300-1002"
	f := newLocalWiFiFixture("Private 日本語", "WPA2PSK", "AES", "passPhrase", "fixture-secret")
	f.refs[0].Flags = 2
	selection, err := selectLocalWiFiProfile(context.Background(), f, localWiFiRevealRequest{InterfaceID: localWiFiFixtureID, ProfileName: f.refs[0].Name})
	if err != nil {
		t.Fatal(err)
	}
	r := localWiFiHelperRequest{InterfaceID: selection.id, ProfileName: selection.ref.Name, ProfileFlags: 2, RequesterSID: user,
		Fingerprint: localWiFiFingerprint(selection.id, selection.ref, selection.parsed), MetadataKnown: true}
	if err := validateLocalWiFiHelperSelection(r, selection, user, user); err != nil {
		t.Fatal(err)
	}
	if err := validateLocalWiFiHelperSelection(r, selection, user, admin); err == nil || localWiFiError(err, true).ErrorCode != "permission" {
		t.Fatal("different-account UAC was allowed to substitute a per-user profile")
	}
	if err := validateLocalWiFiHelperSelection(r, selection, admin, admin); err == nil {
		t.Fatal("forged requester SID was accepted")
	}
	changed := selection
	changed.ref.Flags = 0
	if err := validateLocalWiFiHelperSelection(r, changed, user, user); err == nil {
		t.Fatal("changed profile scope accepted")
	}
	changed = selection
	parsed := *selection.parsed
	parsed.ssidHex = "00"
	changed.parsed = &parsed
	if err := validateLocalWiFiHelperSelection(r, changed, user, user); err == nil {
		t.Fatal("changed profile metadata accepted after UAC")
	}
	// All-user profiles can be read after explicit different-account UAC,
	// but the request remains bound to the authenticated original parent.
	selection.ref.Flags = 0
	r.ProfileFlags = 0
	r.Fingerprint = localWiFiFingerprint(selection.id, selection.ref, selection.parsed)
	if err := validateLocalWiFiHelperSelection(r, selection, user, admin); err != nil {
		t.Fatalf("all-user elevation unnecessarily rejected: %v", err)
	}
	// If Windows denied metadata before approval, require exact interface,
	// name and scope while allowing metadata to become readable afterward.
	r.MetadataKnown = false
	r.Fingerprint = localWiFiFingerprint(selection.id, selection.ref, nil)
	if err := validateLocalWiFiHelperSelection(r, selection, user, admin); err != nil {
		t.Fatalf("unreadable metadata could not become readable after UAC: %v", err)
	}
	if err := validateLocalHelperRequest(localHelperRequest{Version: localToolsProtocolVersion, WiFi: &r}); err != nil {
		t.Fatal(err)
	}
	envelope := localHelperRequest{Version: localToolsProtocolVersion, WiFi: &r, Request: localToolsRequest{Kind: "hosts", Hosts: &localHostsRequest{Revision: strings.Repeat("a", 64)}}}
	if validateLocalHelperRequest(envelope) == nil {
		t.Fatal("helper accepted a combined key read and mutation")
	}
}

func TestLocalWiFiRequestValidationPreservesNames(t *testing.T) {
	for _, name := range []string{"  Localisé 日本語  ", strings.Repeat("é", 255), "A&B <test> ; quote \" name"} {
		if err := validateLocalWiFiRequest(localWiFiRevealRequest{InterfaceID: localWiFiFixtureID, ProfileName: name}); err != nil {
			t.Fatalf("exact native name rejected: %v", err)
		}
	}
	for _, name := range []string{"", "bad\x00name", strings.Repeat("x", 256), strings.Repeat("🛜", 128), string([]byte{255})} {
		if validateLocalWiFiRequest(localWiFiRevealRequest{InterfaceID: localWiFiFixtureID, ProfileName: name}) == nil {
			t.Fatal("invalid profile name accepted")
		}
	}
	for _, sid := range []string{"S-1-5--100", "S-1-5-", "S-1-+5-1", "S-1-5-4294967296", "S-1-5"} {
		if validLocalWiFiSID(sid) {
			t.Fatalf("invalid SID accepted: %s", sid)
		}
	}
}

func TestLocalWiFiHTTPIsExplicitPrivateAndUncached(t *testing.T) {
	a := testApplication(t)
	var calls atomic.Int32
	a.localTools.wifi = localWiFiBackend{
		read: func(context.Context) (localWiFiSnapshot, error) {
			return localWiFiSnapshot{Available: true, Interfaces: []localWiFiInterface{}}, nil
		},
		reveal: func(_ context.Context, request localWiFiRevealRequest) localWiFiResult {
			calls.Add(1)
			return localWiFiResult{Reply: &localWiFiRevealResponse{InterfaceID: request.InterfaceID, ProfileName: request.ProfileName, KeyKind: "passphrase", Secret: "fixture-http-secret"}}
		},
	}
	body := `{"interfaceId":"` + localWiFiFixtureID + `","profileName":"Profile","elevate":false}`
	for _, request := range []struct {
		method, path, body string
		auth               bool
		code               int
	}{
		{"GET", "/api/local-tools/wifi", "", false, 401},
		{"GET", "/api/local-tools/wifi/reveal", "", true, 405},
		{"POST", "/api/local-tools/wifi/reveal", body, false, 401},
		{"POST", "/api/local-tools/wifi/reveal", `{"interfaceId":"` + localWiFiFixtureID + `","profileName":"Profile"}`, true, 400},
		{"POST", "/api/local-tools/wifi/reveal", strings.TrimSuffix(body, "}") + `,"profileFlags":0}`, true, 400},
		{"POST", "/api/local-tools/wifi/reveal", strings.TrimSuffix(body, "}") + `,"elevate":true}`, true, 400},
	} {
		r := requestApp(a, request.method, request.path, request.body, request.auth)
		if r.Code != request.code {
			t.Fatalf("explicit request gate: %d, %s", r.Code, r.Body.String())
		}
	}
	if calls.Load() != 0 {
		t.Fatal("invalid or implicit request read a key")
	}
	r := requestApp(a, "POST", "/api/local-tools/wifi/reveal", body, true)
	if r.Code != 200 || !strings.Contains(r.Body.String(), "fixture-http-secret") || !strings.Contains(r.Header().Get("Cache-Control"), "no-store") {
		t.Fatalf("private reveal failed: %d, %s", r.Code, r.Body.String())
	}
	if calls.Load() != 1 || len(a.localTools.operations) != 0 || len(a.localTools.order) != 0 || len(a.jobs) != 0 {
		t.Fatal("key reveal entered diagnostic or mutation history")
	}
	for _, path := range []string{"/api/state", "/api/jobs", "/api/local-tools/wifi"} {
		r = requestApp(a, "GET", path, "", true)
		if strings.Contains(r.Body.String(), "fixture-http-secret") {
			t.Fatal("key leaked outside its direct response")
		}
	}
}

func TestLocalWiFiHTTPCancellationDiscardsLateKey(t *testing.T) {
	a := testApplication(t)
	started, release := make(chan struct{}), make(chan struct{})
	late := &localWiFiRevealResponse{InterfaceID: localWiFiFixtureID, ProfileName: "Profile", Secret: "fixture-late-secret"}
	a.localTools.wifi.reveal = func(context.Context, localWiFiRevealRequest) localWiFiResult {
		close(started)
		<-release // Simulate a native API that returns after request cancellation.
		return localWiFiResult{Reply: late}
	}
	ctx, cancel := context.WithCancel(context.Background())
	body := `{"interfaceId":"` + localWiFiFixtureID + `","profileName":"Profile","elevate":false}`
	r := httptest.NewRequest("POST", "http://"+a.host+"/api/local-tools/wifi/reveal", strings.NewReader(body)).WithContext(ctx)
	r.Header.Set("X-Session-Token", a.token)
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	done := make(chan struct{})
	go func() { a.ServeHTTP(w, r); close(done) }()
	<-started
	busy := requestApp(a, "POST", "/api/local-tools/wifi/reveal", body, true)
	if busy.Code != 409 || !strings.Contains(busy.Body.String(), "busy") {
		t.Fatal("simultaneous reveal was not bounded")
	}
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("cancelled HTTP request waited for the blocked native API")
	}
	close(release)
	wait, stop := context.WithTimeout(context.Background(), time.Second)
	defer stop()
	if err := a.localTools.Close(wait); err != nil {
		t.Fatal(err)
	}
	if w.Body.Len() != 0 || late.Secret != "" {
		t.Fatal("late secret survived cancellation")
	}
}

type localWiFiDeadlineWriter struct {
	*httptest.ResponseRecorder
	deadline time.Time
}

func (w *localWiFiDeadlineWriter) SetWriteDeadline(at time.Time) error { w.deadline = at; return nil }

func TestLocalWiFiElevationRequiresExplicitRequestAndDeadline(t *testing.T) {
	a := testApplication(t)
	var elevated atomic.Bool
	a.localTools.wifi.reveal = func(_ context.Context, r localWiFiRevealRequest) localWiFiResult {
		elevated.Store(r.Elevate)
		return localWiFiFailure("permission", "Fixture policy denied plaintext.", false)
	}
	body := `{"interfaceId":"` + localWiFiFixtureID + `","profileName":"Profile","elevate":true}`
	if r := requestApp(a, "POST", "/api/local-tools/wifi/reveal", body, true); r.Code != 500 || elevated.Load() {
		t.Fatal("UAC request proceeded without a supported bounded response deadline")
	}
	r := httptest.NewRequest("POST", "http://"+a.host+"/api/local-tools/wifi/reveal", strings.NewReader(body))
	r.Header.Set("X-Session-Token", a.token)
	r.Header.Set("Content-Type", "application/json")
	w := &localWiFiDeadlineWriter{ResponseRecorder: httptest.NewRecorder()}
	a.ServeHTTP(w, r)
	if !elevated.Load() || w.Code != http.StatusForbidden || time.Until(w.deadline) < 120*time.Second || time.Until(w.deadline) > 126*time.Second {
		t.Fatal("explicit administrator retry did not keep its direct private response bounded")
	}
}

func TestLocalWiFiHelperFrameHasSeparateSecretResponse(t *testing.T) {
	r := localHelperResponse{Version: localToolsProtocolVersion, WiFi: &localWiFiResult{Reply: &localWiFiRevealResponse{InterfaceID: localWiFiFixtureID, ProfileName: "Fixture", KeyKind: "raw_psk", Secret: strings.Repeat("11", 32)}}}
	var buffer bytes.Buffer
	if err := writeLocalFrame(&buffer, r); err != nil {
		t.Fatal(err)
	}
	var got localHelperResponse
	if err := readLocalFrame(&buffer, &got); err != nil || !reflect.DeepEqual(got, r) || got.Result != nil {
		t.Fatal("dedicated key response changed or entered mutation result")
	}
	if result := localWiFiError(errors.New("unexpected fixture-key text"), false); strings.Contains(result.Error, "fixture-key") {
		t.Fatal("generic native/parsing failure exposed unchecked text")
	}
}

func TestLocalIPv4EligibilityKeepsDisconnectedAndConfigurableVirtualAdapters(t *testing.T) {
	cases := []struct {
		name                      string
		index, flags, kind, state uint32
		want                      bool
	}{
		{"connected Ethernet", 7, 0x80, 6, 1, true},
		{"unplugged Ethernet", 7, 0x80, 6, 2, true},
		{"disconnected Wi-Fi", 7, 0x80, 71, 2, true},
		{"virtual Ethernet", 9, 0x80, 6, 1, true},
		{"WireGuard/Wintun virtual", 9, 0x80, 53, 2, true},
		{"disconnected PPP VPN", 9, 0x80, 23, 2, true},
		{"PPP VPN dormant", 9, 0x80, 23, 5, true},
		{"loopback", 1, 0x80, 24, 1, false},
		{"implementation tunnel", 9, 0x80, 131, 1, false},
		{"receive only", 9, 0x88, 6, 1, false},
		{"not present", 9, 0x80, 6, 6, false},
		{"IPv6 only", 9, 0x100, 6, 1, false},
		{"no interface index", 0, 0x80, 6, 1, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := localIPv4Configurable(localWiFiFixtureID, tc.index, tc.flags, tc.kind, tc.state, 0); got != tc.want {
				t.Fatalf("eligibility %v, want %v", got, tc.want)
			}
		})
	}
	for tunnelType, want := range map[uint32]bool{0: false, 1: true, 2: true, 11: false, 13: false, 14: false, 15: false, 99: false} {
		if localIPv4Configurable(localWiFiFixtureID, 9, 0x80, 131, 2, tunnelType) != want {
			t.Fatalf("tunnel subtype %d eligibility changed", tunnelType)
		}
	}
	if localIPv4Configurable("placeholder", 9, 0x80, 6, 1, 0) {
		t.Fatal("placeholder identity accepted")
	}
}

func TestLocalIPv4DNSOnlyWithoutAddressPreservesEmptySnapshot(t *testing.T) {
	a := localTestAdapter()
	a.DHCP, a.AddressEditable = false, false
	a.Addresses, a.Gateways = []localIPAddress{}, []string{}
	a.Revision = localAdapterRevision(a)
	r := localTestIPRequest(a)
	r.DNSServers = []string{"9.9.9.9"}
	_, steps, err := planLocalIP(localIPSnapshot{Adapters: []localIPAdapter{a}}, r)
	if err != nil || len(steps) != 1 || steps[0].Arguments[3] != "dnsservers" {
		t.Fatalf("addressless DNS-only adapter was unusable: %#v, %v", steps, err)
	}
	current := a
	result := executeLocalIP(context.Background(), r, localIPApplyOps{
		read: func(context.Context) (localIPSnapshot, error) {
			return localIPSnapshot{Adapters: []localIPAdapter{current}}, nil
		},
		run: func(_ context.Context, step localNetshStep) (string, error) {
			if step.Arguments[3] != "dnsservers" {
				t.Fatal("address command generated for an empty DNS-only snapshot")
			}
			current.DNSServers = append([]string{}, r.DNSServers...)
			current.DNSAutomatic = false
			current.Revision = localAdapterRevision(current)
			return "", nil
		},
	})
	if result.Error != "" || len(current.Addresses) != 0 || len(current.Gateways) != 0 {
		t.Fatalf("DNS-only apply did not preserve absent addressing: %#v", result)
	}
	configured := localTestAdapter()
	r.Revision, r.AdapterID = configured.Revision, configured.ID
	if _, steps, err := planLocalIP(localIPSnapshot{Adapters: []localIPAdapter{configured}}, r); err == nil || len(steps) != 0 {
		t.Fatal("empty snapshot could erase an existing IPv4 configuration")
	}
}
