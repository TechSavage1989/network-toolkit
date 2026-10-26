package main

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"io"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
	"time"
	"unicode/utf16"
)

func localTestAdapter() localIPAdapter {
	metric := uint32(17)
	a := localIPAdapter{ID: "{01234567-89ab-cdef-0123-456789abcdef}", Index: 9, Name: "Ethernet", Status: "Up", Editable: true, AddressEditable: true,
		Addresses: []localIPAddress{{"192.0.2.42", 24, "manual"}}, Gateways: []string{"192.0.2.1"}, DNSServers: []string{"1.1.1.1"}, dnsRegistration: "both", gatewayMetric: &metric}
	a.Revision = localAdapterRevision(a)
	return a
}
func localTestIPRequest(a localIPAdapter) localIPRequest {
	address, prefix, gateway := primaryLocalIP(a)
	mode, dnsMode := "static", "manual"
	if a.DHCP {
		mode = "dhcp"
	}
	if a.DNSAutomatic {
		dnsMode = "automatic"
	}
	return localIPRequest{AdapterID: a.ID, Revision: a.Revision, Mode: mode, Address: address, PrefixLength: prefix, Gateway: gateway, DNSMode: dnsMode, DNSServers: append([]string{}, a.DNSServers...)}
}

func TestLocalIPPlanPreservesIdentityAndMetadata(t *testing.T) {
	a := localTestAdapter()
	r := localTestIPRequest(a)
	r.Address, r.DNSServers = "192.0.2.43", []string{"9.9.9.9", "149.112.112.112"}
	_, steps, err := planLocalIP(localIPSnapshot{Adapters: []localIPAdapter{a}}, r)
	if err != nil {
		t.Fatal(err)
	}
	want := [][]string{
		{"interface", "ipv4", "set", "address", "name=9", "source=static", "address=192.0.2.43", "mask=255.255.255.0", "gateway=192.0.2.1", "gwmetric=17", "store=persistent"},
		{"interface", "ipv4", "set", "dnsservers", "name=9", "source=static", "address=9.9.9.9", "validate=no", "register=both"},
		{"interface", "ipv4", "add", "dnsservers", "name=9", "address=149.112.112.112", "index=2", "validate=no"},
	}
	if len(steps) != len(want) {
		t.Fatalf("unexpected steps: %#v", steps)
	}
	for i, step := range steps {
		if !reflect.DeepEqual(step.Arguments, want[i]) {
			t.Fatalf("step %d: %#v", i, step.Arguments)
		}
	}
	r = localTestIPRequest(a)
	r.Mode, r.DNSMode, r.DNSServers = "dhcp", "automatic", []string{}
	_, steps, err = planLocalIP(localIPSnapshot{Adapters: []localIPAdapter{a}}, r)
	if err != nil || len(steps) != 2 || strings.Contains(strings.Join(steps[0].Arguments, " "), "mask=") {
		t.Fatalf("DHCP plan: %#v, %v", steps, err)
	}
	if !reflect.DeepEqual(steps[1].Arguments, []string{"interface", "ipv4", "set", "dnsservers", "name=9", "source=dhcp", "register=both"}) {
		t.Fatalf("registration changed: %#v", steps[1])
	}
}

func TestLocalIPAliasesPermitDNSButBlockAddressReplacement(t *testing.T) {
	a := localTestAdapter()
	a.Addresses = append(a.Addresses, localIPAddress{"192.0.2.80", 24, "manual"})
	a.AddressEditable, a.AddressNote = false, "Additional addresses are preserved."
	a.Revision = localAdapterRevision(a)
	r := localTestIPRequest(a)
	r.DNSServers = []string{"8.8.8.8"}
	_, steps, err := planLocalIP(localIPSnapshot{Adapters: []localIPAdapter{a}}, r)
	if err != nil || len(steps) != 1 || steps[0].Arguments[3] != "dnsservers" {
		t.Fatalf("DNS-only edit rejected: %#v, %v", steps, err)
	}
	r.Address = "192.0.2.41"
	if _, steps, err = planLocalIP(localIPSnapshot{Adapters: []localIPAdapter{a}}, r); err == nil || len(steps) != 0 {
		t.Fatal("alias replacement was not blocked")
	}
	r = localTestIPRequest(a)
	a.Index++
	a.Revision = localAdapterRevision(a)
	if _, _, err = planLocalIP(localIPSnapshot{Adapters: []localIPAdapter{a}}, r); !errors.Is(err, errLocalConfigConflict) {
		t.Fatalf("reused interface index was accepted: %v", err)
	}
}

func TestLocalIPValidationRejectsUnsafeOrAmbiguousInputs(t *testing.T) {
	base := localTestIPRequest(localTestAdapter())
	cases := map[string]func(*localIPRequest){
		"shell text":        func(r *localIPRequest) { r.Address = "192.0.2.3 & whoami" },
		"IPv6":              func(r *localIPRequest) { r.Address = "2001:db8::1" },
		"network":           func(r *localIPRequest) { r.Address = "192.0.2.0" },
		"broadcast":         func(r *localIPRequest) { r.Address = "192.0.2.255" },
		"prefix zero":       func(r *localIPRequest) { r.PrefixLength = 0 },
		"prefix 33":         func(r *localIPRequest) { r.PrefixLength = 33 },
		"gateway self":      func(r *localIPRequest) { r.Gateway = r.Address },
		"duplicate DNS":     func(r *localIPRequest) { r.DNSServers = []string{"1.1.1.1", "1.1.1.1"} },
		"IPv6 DNS":          func(r *localIPRequest) { r.DNSServers = []string{"::1"} },
		"adapter injection": func(r *localIPRequest) { r.AdapterID = "9 interface ipv6" },
		"missing revision":  func(r *localIPRequest) { r.Revision = "" },
	}
	for name, change := range cases {
		t.Run(name, func(t *testing.T) {
			r := base
			change(&r)
			if validateLocalIPRequest(r) == nil {
				t.Fatal("invalid input accepted")
			}
		})
	}
	for _, prefix := range []int{31, 32} {
		r := base
		r.Address = "192.0.2.0"
		r.PrefixLength = prefix
		if err := validateLocalIPRequest(r); err != nil {
			t.Fatalf("/%d rejected: %v", prefix, err)
		}
	}
}

func TestLocalIPApplyStopsOnAdapterReuseAndReportsPartialState(t *testing.T) {
	a := localTestAdapter()
	r := localTestIPRequest(a)
	r.Address, r.DNSServers = "192.0.2.44", []string{"9.9.9.9"}
	current := a
	runs := 0
	ops := localIPApplyOps{read: func(context.Context) (localIPSnapshot, error) {
		return localIPSnapshot{Adapters: []localIPAdapter{current}}, nil
	}, run: func(_ context.Context, _ localNetshStep) (string, error) {
		runs++
		current.ID = "{fedcba98-7654-3210-fedc-ba9876543210}"
		current.Revision = localAdapterRevision(current)
		return "", nil
	}}
	result := executeLocalIP(context.Background(), r, ops)
	if result.ErrorCode != "conflict" || !result.Partial || runs != 1 || result.Result.IP == nil || result.Result.IP.Adapters[0].ID != current.ID {
		t.Fatalf("replacement adapter was mutated or partial state lost: %#v, runs=%d", result, runs)
	}
	current = a
	runs = 0
	ops.run = func(_ context.Context, _ localNetshStep) (string, error) {
		runs++
		current.Addresses = []localIPAddress{{r.Address, r.PrefixLength, "manual"}}
		current.Revision = localAdapterRevision(current)
		return "Windows detail", errors.New("command failed")
	}
	result = executeLocalIP(context.Background(), r, ops)
	if result.Error == "" || !result.Partial || !strings.Contains(result.Error, "Windows detail") || result.Result.IP.Adapters[0].Addresses[0].Address != r.Address || runs != 1 {
		t.Fatalf("partial command failure hidden: %#v", result)
	}
}

func TestLocalIPApplyRejectsStaleStateAndCancelledWork(t *testing.T) {
	a := localTestAdapter()
	r := localTestIPRequest(a)
	r.DNSServers = []string{"8.8.8.8"}
	reads, runs := 0, 0
	ops := localIPApplyOps{read: func(context.Context) (localIPSnapshot, error) {
		reads++
		current := a
		if reads > 1 {
			current.DNSServers = []string{"9.9.9.9"}
			current.Revision = localAdapterRevision(current)
		}
		return localIPSnapshot{Adapters: []localIPAdapter{current}}, nil
	}, run: func(context.Context, localNetshStep) (string, error) { runs++; return "", nil }}
	result := executeLocalIP(context.Background(), r, ops)
	if result.ErrorCode != "conflict" || result.Partial || runs != 0 {
		t.Fatalf("stale state mutated: %#v", result)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	result = executeLocalIP(ctx, r, ops)
	if result.ErrorCode != "cancelled" || runs != 0 {
		t.Fatalf("cancelled work executed: %#v", result)
	}
}

func TestLocalIPApplyStopsIfSettingsChangeBetweenSteps(t *testing.T) {
	a := localTestAdapter()
	r := localTestIPRequest(a)
	r.Address, r.DNSServers = "192.0.2.44", []string{"9.9.9.9"}
	current, reads, runs := a, 0, 0
	ops := localIPApplyOps{read: func(context.Context) (localIPSnapshot, error) {
		reads++
		if reads == 4 {
			current.DNSServers = []string{"8.8.8.8"}
			current.Revision = localAdapterRevision(current)
		}
		return localIPSnapshot{Adapters: []localIPAdapter{current}}, nil
	}, run: func(context.Context, localNetshStep) (string, error) {
		runs++
		current.Addresses = []localIPAddress{{r.Address, r.PrefixLength, "manual"}}
		current.Revision = localAdapterRevision(current)
		return "", nil
	}}
	result := executeLocalIP(context.Background(), r, ops)
	if result.ErrorCode != "conflict" || !result.Partial || runs != 1 || result.Result.IP.Adapters[0].DNSServers[0] != "8.8.8.8" {
		t.Fatalf("external change overwritten during multi-step apply: %#v, runs=%d", result, runs)
	}
}

func TestLocalIPVerifyDoesNotClaimEmptyDNSWhileServersRemain(t *testing.T) {
	a := localTestAdapter()
	r := localTestIPRequest(a)
	r.DNSServers = []string{}
	a.DNSServers = []string{}
	a.DNSAutomatic = true
	a.effectiveDNS = []string{"192.0.2.53"}
	if localIPMatchesRequest(localIPSnapshot{Adapters: []localIPAdapter{a}}, a.Index, r) {
		t.Fatal("unconfirmed DNS clear reported as success")
	}
	a.effectiveDNS = []string{}
	if !localIPMatchesRequest(localIPSnapshot{Adapters: []localIPAdapter{a}}, a.Index, r) {
		t.Fatal("verified empty DNS list rejected")
	}
}

func TestLocalHostsEncodingAndLineEndingPreservation(t *testing.T) {
	text := "# retained comment\r\n127.0.0.1 localhost\n::1 localhost-v6\r# final comment"
	utf16Bytes := func(big bool) []byte {
		units := utf16.Encode([]rune(text))
		b := make([]byte, 2+len(units)*2)
		b[0], b[1] = 0xff, 0xfe
		if big {
			b[0], b[1] = 0xfe, 0xff
		}
		for i, v := range units {
			if big {
				binary.BigEndian.PutUint16(b[2+i*2:], v)
			} else {
				binary.LittleEndian.PutUint16(b[2+i*2:], v)
			}
		}
		return b
	}
	for name, original := range map[string][]byte{"UTF-8": []byte(text), "UTF-8 BOM": append([]byte{0xef, 0xbb, 0xbf}, []byte(text)...), "UTF-16 LE": utf16Bytes(false), "UTF-16 BE": utf16Bytes(true)} {
		t.Run(name, func(t *testing.T) {
			doc, err := decodeHostsDocument(original, true, nil)
			if err != nil {
				t.Fatal(err)
			}
			if doc.encoding != name || doc.newline != "Mixed" {
				t.Fatalf("encoding/newlines: %#v", doc)
			}
			unchanged, err := encodeHostsDocument(doc, normalizeHostsNewlines(text), nil)
			if err != nil || !bytes.Equal(unchanged, original) {
				t.Fatalf("unchanged file bytes changed: %v", err)
			}
			edited, err := encodeHostsDocument(doc, strings.Replace(normalizeHostsNewlines(text), "localhost-v6", "ip6-localhost", 1)+"\n", nil)
			if err != nil {
				t.Fatal(err)
			}
			roundTrip, err := decodeHostsDocument(edited, true, nil)
			if err != nil {
				t.Fatal(err)
			}
			if roundTrip.encoding != name || !strings.HasPrefix(roundTrip.text, "# retained comment\r\n127.0.0.1 localhost\n") || !strings.HasSuffix(roundTrip.text, "\r\n") {
				t.Fatalf("edited file lost encoding, original terminators or final newline: %q (%s)", roundTrip.text, roundTrip.encoding)
			}
		})
	}
	for _, data := range [][]byte{{0xff, 0xfe, 0x61}, {0xff, 0xfe, 0, 0xd8}, {0, 0}} {
		if _, err := decodeHostsDocument(data, true, nil); err == nil {
			t.Fatalf("invalid encoding accepted: %x", data)
		}
	}
}

func TestLocalHostsValidationPreservesCommentsAndAllowsBlockingEntries(t *testing.T) {
	valid := "# user comment\n0.0.0.0 ads.example # blocked\n127.0.0.1 localhost alias_name\n::1 localhost\n"
	warnings, err := validateHostsText(valid)
	if err != nil || len(warnings) != 1 {
		t.Fatalf("valid hosts entries: %#v %v", warnings, err)
	}
	for _, content := range []string{"not-an-ip localhost\n", "127.0.0.1\n", "127.0.0.1 bad/name\n", "fe80::1%3 scoped\n", "127.0.0.1 bad\x00name\n", strings.Repeat("#", localHostsLimit+1)} {
		if _, err = validateHostsText(content); err == nil {
			t.Fatalf("invalid hosts content accepted: %q", content[:min(len(content), 80)])
		}
	}
}

func localTestHostsOps() localHostsFileOps {
	return localHostsFileOps{
		replace: func(path, temporary, backup string) error {
			if err := os.Link(path, backup); err != nil {
				return err
			}
			return os.Rename(temporary, path)
		},
		create: func(path, temporary string) error {
			if err := os.Link(temporary, path); err != nil {
				return err
			}
			return os.Remove(temporary)
		},
	}
}

func TestLocalHostsSaveBackupStaleGuardAndAtomicCreation(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "hosts")
	original := []byte("# original\r\n127.0.0.1 localhost\r\n")
	if err := os.WriteFile(path, original, 0600); err != nil {
		t.Fatal(err)
	}
	r := localHostsRequest{Revision: localHostsRevision(true, original), Content: "# original\n127.0.0.1 localhost\n192.0.2.4 switch.example\n"}
	backup, err := saveHostsFile(context.Background(), path, r, localTestHostsOps())
	if err != nil {
		t.Fatal(err)
	}
	old, err := os.ReadFile(backup)
	if err != nil || !bytes.Equal(old, original) {
		t.Fatalf("backup not preserved: %q, %v", old, err)
	}
	current, _ := os.ReadFile(path)
	if !bytes.Contains(current, []byte("switch.example\r\n")) {
		t.Fatalf("new line terminators changed: %q", current)
	}
	if _, err = saveHostsFile(context.Background(), path, r, localTestHostsOps()); !errors.Is(err, errLocalConfigConflict) {
		t.Fatalf("stale hosts edit accepted: %v", err)
	}
	after, _ := os.ReadFile(path)
	if !bytes.Equal(current, after) {
		t.Fatal("stale edit altered live hosts")
	}
	missing := filepath.Join(dir, "missing-hosts")
	request := localHostsRequest{Revision: localHostsRevision(false, nil), Content: "127.0.0.1 localhost\n"}
	ops := localTestHostsOps()
	ops.create = func(path, temporary string) error {
		if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
			t.Fatal("live file created before staging completed")
		}
		data, err := os.ReadFile(temporary)
		if err != nil || !bytes.Equal(data, []byte("127.0.0.1 localhost\r\n")) {
			t.Fatalf("publish received partial data: %q %v", data, err)
		}
		return os.ErrPermission
	}
	if _, err = saveHostsFile(context.Background(), missing, request, ops); !errors.Is(err, os.ErrPermission) {
		t.Fatalf("publish failure hidden: %v", err)
	}
	if _, err = os.Stat(missing); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("failed publish left a partial live hosts file")
	}
	entries, _ := os.ReadDir(dir)
	for _, entry := range entries {
		if strings.HasSuffix(entry.Name(), ".tmp") {
			t.Fatalf("staging file leaked: %s", entry.Name())
		}
	}
	if _, err = saveHostsFile(context.Background(), missing, request, localTestHostsOps()); err != nil {
		t.Fatal(err)
	}
}

func TestLocalHostsCreationRaceNeverReplacesAnotherFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "hosts")
	ops := localTestHostsOps()
	publish := ops.create
	ops.create = func(path, temporary string) error {
		if err := os.WriteFile(path, []byte("# another editor\n"), 0600); err != nil {
			return err
		}
		return publish(path, temporary)
	}
	_, err := saveHostsFile(context.Background(), path, localHostsRequest{Revision: localHostsRevision(false, nil), Content: "127.0.0.1 localhost\n"}, ops)
	if !errors.Is(err, errLocalConfigConflict) {
		t.Fatalf("creation race not reported: %v", err)
	}
	actual, _ := os.ReadFile(path)
	if string(actual) != "# another editor\n" {
		t.Fatalf("another editor's file was replaced: %q", actual)
	}
}

func TestLocalHostsRejectsSymbolicLinkParents(t *testing.T) {
	dir := t.TempDir()
	real := filepath.Join(dir, "real")
	if err := os.Mkdir(real, 0700); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, "linked")
	if err := os.Symlink(real, link); err != nil {
		t.Skip("symbolic links are unavailable for this account")
	}
	if _, err := readHostsDocument(filepath.Join(link, "hosts"), nil); err == nil {
		t.Fatal("symlink parent was followed")
	}
}

func TestLocalToolsJSONRejectsMissingNullAndAmbiguousFields(t *testing.T) {
	revision := strings.Repeat("a", 64)
	for name, body := range map[string]string{
		"missing content": `{"revision":"` + revision + `"}`,
		"null content":    `{"revision":"` + revision + `","content":null}`,
		"duplicate":       `{"revision":"` + revision + `","content":"","content":"# second"}`,
		"case variant":    `{"revision":"` + revision + `","Content":""}`,
		"unknown":         `{"revision":"` + revision + `","content":"","path":"elsewhere"}`,
		"trailing":        `{"revision":"` + revision + `","content":""} {}`,
		"null object":     `null`,
	} {
		t.Run(name, func(t *testing.T) {
			r := httptest.NewRequest("POST", "/", strings.NewReader(body))
			r.Header.Set("Content-Type", "application/json")
			var target localHostsRequest
			if decodeLocalJSON(httptest.NewRecorder(), r, &target, 1024) == nil {
				t.Fatal("ambiguous input accepted")
			}
		})
	}
	r := httptest.NewRequest("POST", "/", strings.NewReader(`{"revision":"`+revision+`","content":""}`))
	r.Header.Set("Content-Type", "application/json; charset=utf-8")
	var target localHostsRequest
	if err := decodeLocalJSON(httptest.NewRecorder(), r, &target, 1024); err != nil {
		t.Fatalf("explicit empty content rejected: %v", err)
	}
}

func TestLocalToolsManagerSerializesOperationsAndCancelsOnClose(t *testing.T) {
	entered := make(chan struct{})
	var calls atomic.Int32
	m := newLocalToolsManagerWithBackend(context.Background(), localToolsBackend{available: true, apply: func(ctx context.Context, _ localToolsRequest) localApplyResult {
		calls.Add(1)
		close(entered)
		<-ctx.Done()
		return localApplyResult{Error: ctx.Err().Error(), ErrorCode: "cancelled"}
	}})
	op, err := m.start(localToolsRequest{Kind: "hosts"})
	if err != nil {
		t.Fatal(err)
	}
	<-entered
	if _, err = m.start(localToolsRequest{Kind: "hosts"}); !errors.Is(err, errLocalConfigBusy) {
		t.Fatalf("concurrent mutation accepted: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err = m.Close(ctx); err != nil {
		t.Fatal(err)
	}
	result, ok := m.operation(op.ID)
	if !ok || result.Status != "failed" || result.ErrorCode != "cancelled" || calls.Load() != 1 {
		t.Fatalf("cancelled operation lost: %#v", result)
	}
	if _, err = m.start(localToolsRequest{Kind: "hosts"}); !errors.Is(err, context.Canceled) {
		t.Fatalf("closed manager accepted work: %v", err)
	}
}

func TestLocalToolsAPIAuthenticationConflictAndAsyncResult(t *testing.T) {
	a := testApplication(t)
	snapshot := localHostsSnapshot{Available: true, Editable: true, Exists: true, Content: "# original\n", Revision: localHostsRevision(true, []byte("# original\n")), Warnings: []string{}}
	var applied atomic.Int32
	a.localTools = newLocalToolsManagerWithBackend(a.ctx, localToolsBackend{available: true, readHosts: func(context.Context) (localHostsSnapshot, error) { return snapshot, nil }, apply: func(_ context.Context, r localToolsRequest) localApplyResult {
		applied.Add(1)
		fresh := snapshot
		fresh.Content = r.Hosts.Content
		fresh.Revision = localHostsRevision(true, []byte(fresh.Content))
		return localApplyResult{Message: "Saved", Result: localToolsResult{Hosts: &fresh}}
	}})
	if r := requestApp(a, "GET", "/api/local-tools/hosts", "", false); r.Code != 401 {
		t.Fatalf("unauthenticated local tools status %d", r.Code)
	}
	body, _ := json.Marshal(localHostsRequest{Revision: strings.Repeat("a", 64), Content: "# edited\n"})
	if r := requestApp(a, "POST", "/api/local-tools/hosts", string(body), true); r.Code != 409 || !strings.Contains(r.Body.String(), `"errorCode":"conflict"`) {
		t.Fatalf("stale API request: %d %s", r.Code, r.Body.String())
	}
	if applied.Load() != 0 {
		t.Fatal("stale API request mutated state")
	}
	body, _ = json.Marshal(localHostsRequest{Revision: snapshot.Revision, Content: "# edited\n"})
	response := requestApp(a, "POST", "/api/local-tools/hosts", string(body), true)
	if response.Code != 202 {
		t.Fatalf("apply was not asynchronous: %d %s", response.Code, response.Body.String())
	}
	var reply struct {
		Operation localToolsOperation `json:"operation"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &reply); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(time.Second)
	for {
		op, ok := a.localTools.operation(reply.Operation.ID)
		if ok && op.Status == "completed" {
			if op.Result.Hosts == nil || op.Result.Hosts.Content != "# edited\n" {
				t.Fatalf("refreshed hosts missing: %#v", op)
			}
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("asynchronous operation did not complete")
		}
		time.Sleep(time.Millisecond)
	}
	if response = requestApp(a, "GET", "/api/local-tools/operations/"+reply.Operation.ID, "", true); response.Code != 200 {
		t.Fatalf("operation lookup: %d", response.Code)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := a.localTools.Close(ctx); err != nil {
		t.Fatal(err)
	}
}

type localShortWriter struct{ bytes.Buffer }

func (w *localShortWriter) Write(p []byte) (int, error) {
	if len(p) > 3 {
		p = p[:3]
	}
	return w.Buffer.Write(p)
}
func TestLocalHelperFramesAreBoundedAndStrict(t *testing.T) {
	request := localHelperRequest{Version: localToolsProtocolVersion, Request: localToolsRequest{Kind: "hosts", Hosts: &localHostsRequest{Revision: strings.Repeat("a", 64), Content: "# example\n"}}}
	var out localShortWriter
	if err := writeLocalFrame(&out, request); err != nil {
		t.Fatal(err)
	}
	var got localHelperRequest
	if err := readLocalFrame(&out, &got); err != nil || !reflect.DeepEqual(request, got) {
		t.Fatalf("short-write framing: %#v %v", got, err)
	}
	for name, payload := range map[string][]byte{"zero": {0, 0, 0, 0}, "oversized": {1, 0, 128, 0}, "truncated": {10, 0, 0, 0, '{'}} {
		t.Run(name, func(t *testing.T) {
			if err := readLocalFrame(bytes.NewReader(payload), &got); err == nil {
				t.Fatal("invalid frame accepted")
			}
		})
	}
	for _, body := range []string{`null`, `{"version":1,"request":{"kind":"hosts","command":"whoami"}}`, `{"version":1} {}`} {
		var frame bytes.Buffer
		_ = binary.Write(&frame, binary.LittleEndian, uint32(len(body)))
		frame.WriteString(body)
		if err := readLocalFrame(&frame, &got); err == nil {
			t.Fatalf("invalid protocol object accepted: %s", body)
		}
	}
	for _, id := range []string{"", strings.Repeat("a", 31), strings.Repeat("A", 32), "../../../../../../../../../../.."} {
		if validLocalHelperID(id) {
			t.Fatalf("invalid pipe id accepted: %q", id)
		}
	}
	if !validLocalHelperID("0123456789abcdef0123456789abcdef") {
		t.Fatal("valid private pipe id rejected")
	}
	var buffer localOutputBuffer
	large := bytes.Repeat([]byte{'a'}, 100<<10)
	if n, err := buffer.Write(large); err != nil || n != len(large) || len(buffer.data) != 64<<10 {
		t.Fatal("bounded subprocess output did not drain input")
	}
	if _, err := io.Copy(&buffer, bytes.NewReader(large)); err != nil {
		t.Fatal(err)
	}
}
