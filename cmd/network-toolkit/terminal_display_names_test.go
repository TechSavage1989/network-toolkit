package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"sync"
	"testing"

	"networktoolkit/internal/terminal"
)

func displayState128(t *testing.T, raw string) map[string]any {
	t.Helper()
	var state map[string]any
	if err := json.Unmarshal([]byte(raw), &state); err != nil {
		t.Fatal(err)
	}
	return state
}

const displayFixture128 = `{"theme":"dark","terminalProfiles":[{"id":"p","name":"Original","nameExplicit":true,"nameRevision":3,"type":"ssh","host":"192.0.2.73","port":22,"username":"operator","credentialId":"credential-reference","authMethod":"key"}],"terminalRecent":[{"id":"r","name":"Original","nameExplicit":true,"nameRevision":3,"type":"ssh","host":"192.0.2.73","port":22,"username":"operator","credentialId":"recent-reference"}],"terminalNameRevision":3}`

func TestDisplayNames128PersistProjectRenameClearAndRestart(t *testing.T) {
	a := &application{state: []byte(displayFixture128), dataPath: filepath.Join(t.TempDir(), "state.json")}
	original := displayState128(t, displayFixture128)
	s := terminal.SessionInfo{ID: "one", ProfileID: "p", Type: "ssh", Host: "192.0.2.73", Port: 22, Username: "operator", Label: "Original", ManualName: "Renamed from child", NameExplicit: true, LabelSource: "manual"}
	rev, err := a.persistTerminalDisplayName(s)
	if err != nil {
		t.Fatal(err)
	}
	if rev != 4 {
		t.Fatalf("revision=%d", rev)
	}
	var saved map[string]any
	if err = json.Unmarshal(a.state, &saved); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"terminalProfiles", "terminalRecent"} {
		row := terminalNameRows(saved, key)[0]
		old := terminalNameRows(original, key)[0]
		if row["name"] != "Renamed from child" || row["nameExplicit"] != true || row["nameRevision"] != float64(4) {
			t.Fatalf("rename not propagated: %+v", row)
		}
		for _, field := range []string{"host", "port", "username", "credentialId", "authMethod", "id"} {
			if !reflect.DeepEqual(row[field], old[field]) {
				t.Fatalf("changed %s", field)
			}
		}
	}
	projected := a.terminalDisplaySession(s)
	if projected.ManualName != "Renamed from child" || projected.NameRevision != 4 {
		t.Fatalf("projection %+v", projected)
	}
	s.ManualName = ""
	s.NameExplicit = false
	s.LabelSource = "default"
	rev, err = a.persistTerminalDisplayName(s)
	if err != nil || rev != 5 {
		t.Fatalf("clear %d %v", rev, err)
	}
	disk, err := os.ReadFile(a.dataPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(disk) != string(a.state) {
		t.Fatal("disk/state differ")
	}
	restarted := &application{state: disk}
	projected = restarted.terminalDisplaySession(terminal.SessionInfo{ID: "one", ProfileID: "p", Type: "ssh", Host: s.Host, Port: 22, Username: s.Username, ManualName: "stale", Label: "stale", LabelSource: "profile"})
	wire, _ := json.Marshal(projected)
	var fields map[string]any
	if err := json.Unmarshal(wire, &fields); err != nil {
		t.Fatal(err)
	}
	if explicit, present := fields["nameExplicit"]; !present || explicit != false {
		t.Fatalf("explicit clear omitted from JSON: %s", wire)
	}
	if projected.ManualName != "" || projected.NameExplicit || projected.LabelSource != "default" || projected.NameRevision != 5 {
		t.Fatalf("clear resurrected on restart: %+v", projected)
	}
	saved = displayState128(t, string(disk))
	row := terminalNameRows(saved, "terminalProfiles")[0]
	if row["name"] != "Renamed from child" || row["nameExplicit"] != false {
		t.Fatal("clear must retain stored label but disable override")
	}
}

func TestDisplayNames128StaleWritesProtectOnlyDisplayFields(t *testing.T) {
	current := displayState128(t, displayFixture128)
	incoming := displayState128(t, displayFixture128)
	old := terminalNameRows(current, "terminalProfiles")[0]
	old["name"] = "Child latest"
	old["nameRevision"] = float64(4)
	current["terminalNameRevision"] = float64(4)
	stale := terminalNameRows(incoming, "terminalProfiles")[0]
	stale["host"] = "192.0.2.74"
	incoming["theme"] = "light"
	if err := reconcileTerminalDisplayNames(incoming, current); err != nil {
		t.Fatal(err)
	}
	if stale["name"] != "Child latest" || stale["nameRevision"] != float64(4) || stale["host"] != "192.0.2.74" || incoming["theme"] != "light" {
		t.Fatalf("stale field merge wrong: %+v", incoming)
	}
	incoming = displayState128(t, displayFixture128)
	current = displayState128(t, displayFixture128)
	p := terminalNameRows(incoming, "terminalProfiles")[0]
	p["name"] = "Parent next"
	if err := reconcileTerminalDisplayNames(incoming, current); err != nil {
		t.Fatal(err)
	}
	r := terminalNameRows(incoming, "terminalRecent")[0]
	if p["nameRevision"] != uint64(4) || r["nameRevision"] != uint64(4) || r["name"] != "Parent next" {
		t.Fatalf("equal revision edit not propagated: %+v", incoming)
	}
	p["nameRevision"] = float64(100)
	if reconcileTerminalDisplayNames(incoming, current) == nil {
		t.Fatal("future revision accepted")
	}
}

func TestDisplayNames128AmbiguousEndpointAndScopedProjection(t *testing.T) {
	state := displayState128(t, displayFixture128)
	p := terminalNameRows(state, "terminalProfiles")[0]
	other := map[string]any{}
	for k, v := range p {
		other[k] = v
	}
	other["id"] = "other-profile"
	other["name"] = "Other saved name"
	state["terminalProfiles"] = []map[string]any{p, other}
	raw, _ := json.Marshal(state)
	a := &application{state: raw, dataPath: filepath.Join(t.TempDir(), "state.json")}
	s := terminal.SessionInfo{ID: "one", ProfileID: "p", Type: "ssh", Host: "192.0.2.73", Port: 22, Username: "operator", ManualName: "Only chosen profile", NameExplicit: true, LabelSource: "manual"}
	if _, err := a.persistTerminalDisplayName(s); err != nil {
		t.Fatal(err)
	}
	saved := displayState128(t, string(a.state))
	if terminalNameRows(saved, "terminalProfiles")[1]["name"] != "Other saved name" || terminalNameRows(saved, "terminalRecent")[0]["name"] != "Original" {
		t.Fatal("ambiguous endpoint renamed unrelated records")
	}
	a.views = &terminalViewRegistry{views: map[string]*terminalView{"child": {info: terminalViewInfo{ID: "child", SessionID: "one"}}}}
	req := httptest.NewRequest("GET", "http://fixture.invalid/api/terminals/events", nil)
	req = req.WithContext(context.WithValue(req.Context(), terminalViewContextKey{}, "child"))
	unrelated := terminal.SessionInfo{ID: "other", ProfileID: "other-profile", Label: "private-unrelated"}
	batch := terminal.Batch{Sessions: []terminal.SessionInfo{s, unrelated}, Events: []terminal.Event{{SessionID: "one", Session: &s}, {SessionID: "other", Session: &unrelated}}}
	got := a.terminalBatchForRequest(req, batch)
	encoded, _ := json.Marshal(got)
	if len(got.Sessions) != 1 || len(got.Events) != 1 || strings.Contains(string(encoded), "private-unrelated") || strings.Contains(string(encoded), "credential-reference") || strings.Contains(string(encoded), "terminalProfiles") {
		t.Fatalf("projection leaked scope: %s", encoded)
	}
	if got.Sessions[0].ManualName != "Only chosen profile" || got.Events[0].Session.ManualName != "Only chosen profile" {
		t.Fatal("status and list metadata diverge")
	}
}

func TestDisplayNames128PersistenceFailureLeavesStateIntact(t *testing.T) {
	dir := t.TempDir()
	blocked := filepath.Join(dir, "blocked")
	if err := os.WriteFile(blocked, []byte("not a directory"), 0600); err != nil {
		t.Fatal(err)
	}
	a := &application{state: []byte(displayFixture128), dataPath: filepath.Join(blocked, "state.json")}
	_, err := a.persistTerminalDisplayName(terminal.SessionInfo{ID: "one", ProfileID: "p", Type: "ssh", Host: "192.0.2.73", Port: 22, Username: "operator", ManualName: "Must not persist", NameExplicit: true})
	if err == nil || string(a.state) != displayFixture128 {
		t.Fatal("failed persistence altered state")
	}
}

func TestDisplayNames128StateAcknowledgementContainsOnlyDisplayFields(t *testing.T) {
	state := displayState128(t, displayFixture128)
	ack := terminalDisplayNamesResponse(state)
	raw, _ := json.Marshal(ack)
	if strings.Contains(string(raw), "credential") || strings.Contains(string(raw), "192.0.2.") || strings.Contains(string(raw), "operator") {
		t.Fatalf("display ack leaked connection data: %s", raw)
	}
	if len(ack["profiles"].([]map[string]any)) != 1 || len(ack["recent"].([]map[string]any)) != 1 {
		t.Fatal("missing display acknowledgements")
	}
}

func TestDisplayNames128OwnerGuardCannotBeBypassedForDisplayRename(t *testing.T) {
	a := &application{views: &terminalViewRegistry{owners: map[string]terminalViewOwner{"one": {ViewID: "child", Revision: 7, Ready: true}}, views: map[string]*terminalView{"child": {info: terminalViewInfo{ID: "child", SessionID: "one", State: "detached"}}}, homePresent: true}}
	called := false
	apply := func() error { called = true; return nil }
	request := httptest.NewRequest("POST", "http://fixture.invalid/api/terminals/one/rename", nil)
	request.Header.Set("X-Terminal-Revision", "7")
	if a.withTerminalOwner(request, "one", "rename", apply) == nil || called {
		t.Fatal("home renamed child-owned session")
	}
	request = request.WithContext(context.WithValue(request.Context(), terminalViewContextKey{}, "child"))
	request.Header.Set("X-Terminal-Revision", "6")
	if a.withTerminalOwner(request, "one", "rename", apply) == nil || called {
		t.Fatal("stale revision renamed session")
	}
	request.Header.Set("X-Terminal-Revision", "7")
	if err := a.withTerminalOwner(request, "one", "rename", apply); err != nil || !called {
		t.Fatalf("current owner denied: %v", err)
	}
	if a.views.owners["one"].Revision != 7 || a.views.owners["one"].ViewID != "child" {
		t.Fatal("rename changed session ownership")
	}
}

func TestDisplayNames128RecentOnlySessionReceivesNewerProfileAndClear(t *testing.T) {
	a := &application{state: []byte(displayFixture128)}
	s := terminal.SessionInfo{ID: "recent-only", Type: "ssh", Host: "192.0.2.73", Port: 22, Username: "operator", NameRevision: 2, ManualName: "Old recent name", NameExplicit: true, LabelSource: "manual"}
	got := a.terminalDisplaySession(s)
	if got.ManualName != "Original" || got.NameRevision != 3 {
		t.Fatalf("newer profile was ignored %+v", got)
	}
	state := displayState128(t, displayFixture128)
	for _, key := range []string{"terminalProfiles", "terminalRecent"} {
		r := terminalNameRows(state, key)[0]
		r["nameExplicit"] = false
		r["nameRevision"] = float64(4)
	}
	a.state, _ = json.Marshal(state)
	got = a.terminalDisplaySession(s)
	if got.ManualName != "" || got.NameExplicit || got.NameRevision != 4 || got.LabelSource != "default" {
		t.Fatalf("newer clear resurrected old name %+v", got)
	}
}

func TestDisplayNames128InvalidAndDeletedRecordRevisionRejected(t *testing.T) {
	for _, value := range []any{true, "3", -1.0, 1.5, 9007199254740992.0, nil} {
		state := displayState128(t, displayFixture128)
		terminalNameRows(state, "terminalProfiles")[0]["nameRevision"] = value
		if validateTerminalDisplayRevisions(state) == nil {
			t.Fatalf("invalid revision accepted: %#v", value)
		}
	}
	incoming := displayState128(t, displayFixture128)
	if reconcileTerminalDisplayNames(incoming, map[string]any{}) == nil {
		t.Fatal("deleted record resurrected with stale revision")
	}
}

func TestDisplayNames128NewSavedNameSynchronizesExistingRecentRevision(t *testing.T) {
	current := displayState128(t, displayFixture128)
	delete(current, "terminalProfiles")
	incoming := displayState128(t, displayFixture128)
	p := terminalNameRows(incoming, "terminalProfiles")[0]
	p["nameRevision"] = float64(0)
	p["name"] = "New saved name"
	if err := reconcileTerminalDisplayNames(incoming, current); err != nil {
		t.Fatal(err)
	}
	recent := terminalNameRows(incoming, "terminalRecent")[0]
	if p["nameRevision"] != recent["nameRevision"] || recent["name"] != "New saved name" {
		t.Fatalf("cross-record revision mismatch: %+v", incoming)
	}
}

func TestDisplayNames128ConcurrentRenamesAndStaleSettings(t *testing.T) {
	a := &application{state: []byte(displayFixture128), dataPath: filepath.Join(t.TempDir(), "state.json")}
	type result struct {
		revision uint64
		name     string
		err      error
	}
	outcomes := make(chan result, 8)
	start := make(chan struct{})
	var wg sync.WaitGroup
	for i := 0; i < 4; i++ {
		wg.Add(2)
		go func(i int) {
			defer wg.Done()
			<-start
			name := fmt.Sprintf("Concurrent name %d", i)
			revision, err := a.persistTerminalDisplayName(terminal.SessionInfo{ID: "one", ProfileID: "p", Type: "ssh", Host: "192.0.2.73", Port: 22, Username: "operator", ManualName: name, NameExplicit: true, LabelSource: "manual"})
			outcomes <- result{revision, name, err}
		}(i)
		go func() {
			defer wg.Done()
			<-start
			body := strings.Replace(displayFixture128, `"theme":"dark"`, `"theme":"light"`, 1)
			request := httptest.NewRequest("PUT", "http://fixture.invalid/api/state", strings.NewReader(body))
			response := httptest.NewRecorder()
			a.handleState(response, request)
			var err error
			if response.Code != 200 {
				err = fmt.Errorf("stale settings response %d: %s", response.Code, response.Body.String())
			}
			outcomes <- result{err: err}
		}()
	}
	close(start)
	wg.Wait()
	close(outcomes)
	revisions := []uint64{}
	names := map[uint64]string{}
	for result := range outcomes {
		if result.err != nil {
			t.Fatal(result.err)
		}
		if result.revision != 0 {
			revisions = append(revisions, result.revision)
			names[result.revision] = result.name
		}
	}
	sort.Slice(revisions, func(i, j int) bool { return revisions[i] < revisions[j] })
	if !reflect.DeepEqual(revisions, []uint64{4, 5, 6, 7}) {
		t.Fatalf("non-monotonic/duplicate revisions: %v", revisions)
	}
	a.stateMu.Lock()
	raw := append([]byte(nil), a.state...)
	a.stateMu.Unlock()
	state := displayState128(t, string(raw))
	if state["theme"] != "light" {
		t.Fatal("name conflict discarded unrelated preference")
	}
	for _, key := range []string{"terminalProfiles", "terminalRecent"} {
		row := terminalNameRows(state, key)[0]
		if row["name"] != names[7] || row["nameRevision"] != float64(7) || row["nameExplicit"] != true {
			t.Fatalf("stale write restored old display metadata: %+v", row)
		}
		if row["host"] != "192.0.2.73" || row["port"] != float64(22) || row["username"] != "operator" {
			t.Fatal("concurrent names changed endpoint identity")
		}
	}
	if terminalNameRows(state, "terminalProfiles")[0]["credentialId"] != "credential-reference" || terminalNameRows(state, "terminalRecent")[0]["credentialId"] != "recent-reference" {
		t.Fatal("concurrent names changed credential reference")
	}
	disk, err := os.ReadFile(a.dataPath)
	if err != nil || string(disk) != string(raw) {
		t.Fatalf("disk/state not atomic: %v", err)
	}
}

func TestDisplayNames128RecentOnlyRenameUsesOnlyUniqueSavedIdentity(t *testing.T) {
	a := &application{state: []byte(displayFixture128), dataPath: filepath.Join(t.TempDir(), "state.json")}
	s := terminal.SessionInfo{ID: "recent-only", Type: "ssh", Host: "192.0.2.73", Port: 22, Username: "operator", ManualName: "Recent rename", NameExplicit: true, LabelSource: "manual"}
	for _, clear := range []bool{false, true} {
		if clear {
			s.ManualName = ""
			s.NameExplicit = false
			s.LabelSource = "default"
		}
		revision, err := a.persistTerminalDisplayName(s)
		if err != nil {
			t.Fatal(err)
		}
		state := displayState128(t, string(a.state))
		p := terminalNameRows(state, "terminalProfiles")[0]
		r := terminalNameRows(state, "terminalRecent")[0]
		if p["nameRevision"] != float64(revision) || r["nameRevision"] != float64(revision) || p["nameExplicit"] != !clear || r["nameExplicit"] != !clear {
			t.Fatalf("unique recent/saved provenance diverged: %+v", state)
		}
		if p["name"] != "Recent rename" || r["name"] != "Recent rename" {
			t.Fatal("clear must preserve both stored names")
		}
		// A renderer adopting these authoritative fields can save again without a false future-revision conflict.
		clone := displayState128(t, string(a.state))
		if err := reconcileTerminalDisplayNames(clone, state); err != nil {
			t.Fatal(err)
		}
	}
	s.ProfileID = "deleted-profile"
	s.NameExplicit = true
	s.ManualName = "Do not rename different saved ID"
	if _, err := a.persistTerminalDisplayName(s); err != nil {
		t.Fatal(err)
	}
	state := displayState128(t, string(a.state))
	if terminalNameRows(state, "terminalProfiles")[0]["name"] != "Recent rename" {
		t.Fatal("nonempty missing profile ID fell back to unrelated endpoint match")
	}
	s.LabelSource = "manual"
	projected := a.terminalDisplaySession(s)
	if projected.ManualName != s.ManualName || projected.LabelSource != "manual" || projected.ProfileID != "deleted-profile" {
		t.Fatalf("missing saved identity adopted unrelated endpoint metadata: %+v", projected)
	}

}
