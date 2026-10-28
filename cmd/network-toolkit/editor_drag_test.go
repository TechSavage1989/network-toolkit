package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func editorDragFixture(t *testing.T) (*application, *editorViewRegistry, string, string, string, *atomic.Int32) {
	t.Helper()
	a, m, launches := editorFixture(t)
	batch := editorFixtureBatch()
	if _, err := m.checkpoint("", batch, map[string]uint64{}); err != nil {
		t.Fatal(err)
	}
	first, second, token := randomHex(16), randomHex(16), randomHex(32)
	m.mu.Lock()
	m.views[first] = &editorWindow{info: editorViewInfo{ID: first, State: "ready"}, token: token}
	m.views[second] = &editorWindow{info: editorViewInfo{ID: second, State: "ready"}, token: randomHex(32)}
	m.mu.Unlock()
	return a, m, first, second, token, launches
}

func editorDragClock(m *editorViewRegistry) *atomic.Int64 {
	clock := new(atomic.Int64)
	clock.Store(time.Date(2026, 10, 28, 0, 0, 0, 0, time.UTC).UnixNano())
	m.mu.Lock()
	m.now = func() time.Time { return time.Unix(0, clock.Load()) }
	m.mu.Unlock()
	return clock
}

func editorClaimedDrag(t *testing.T, m *editorViewRegistry, destination string) editorTransferRequest {
	t.Helper()
	ticket := randomHex(16)
	if _, err := m.registerDrag("", ticket, editorFixtureUID, 1); err != nil {
		t.Fatal(err)
	}
	caller, detach := destination, false
	if destination == "new" {
		caller, detach = "", true
	}
	if _, err := m.claimDrag(caller, ticket, detach); err != nil {
		t.Fatal(err)
	}
	return editorTransferRequest{ID: ticket, DragTicket: ticket, Destination: destination, Batch: editorFixtureBatch(), Leases: map[string]uint64{editorFixtureUID: 1}}
}

func TestEditorDragClaimUsesExistingSingleOwnerACK(t *testing.T) {
	_, m, destination, other, _, launches := editorDragFixture(t)
	clock := editorDragClock(m)
	req := editorClaimedDrag(t, m, destination)
	if launches.Load() != 0 || len(m.transfers) != 0 || m.documents[editorFixtureUID].owner != "" || m.documents[editorFixtureUID].pending != "" {
		t.Fatal("claim moved, froze or launched without the source offer")
	}
	source := m.state("")["dragRequests"].([]editorDragRequest)
	if len(source) != 1 || source[0].Ticket != req.ID || source[0].UID != editorFixtureUID || source[0].Destination != destination || source[0].Lease != 1 {
		t.Fatal("captured source UID/lease/destination lost", source)
	}
	for _, caller := range []string{destination, other} {
		raw, _ := json.Marshal(m.state(caller))
		if strings.Contains(string(raw), req.ID) || strings.Contains(string(raw), editorFixtureUID) || strings.Contains(string(raw), "private-document-canary") || len(m.state(caller)["incoming"].([]any)) != 0 {
			t.Fatal("drag leaked another renderer's document or ticket")
		}
	}
	if _, err := m.claimDrag(destination, req.ID, false); err != nil {
		t.Fatal("same receiver could not retry lost claim response", err)
	}
	if _, err := m.claimDrag(other, req.ID, false); err == nil {
		t.Fatal("second receiver stole claimed gesture")
	}
	if out, err := m.cancelDrag("", req.ID); err != nil || out["state"] != "claimed" {
		t.Fatal("racing dragend revoked a successful drop", out, err)
	}
	offer, err := m.offer("", req)
	if err != nil || offer["state"] != "offered" || m.drags[req.ID].state != "consumed" {
		t.Fatal("ticket was not consumed with ordinary offer insertion", offer, err)
	}
	if len(m.state("")["dragRequests"].([]editorDragRequest)) != 0 || m.documents[editorFixtureUID].owner != "" || m.documents[editorFixtureUID].pending != req.ID {
		t.Fatal("offer bypassed source pending ownership")
	}
	accepted, err := m.accept(destination, req.ID)
	if err != nil || accepted["state"] != "accepted" || m.documents[editorFixtureUID].owner != destination || m.documents[editorFixtureUID].lease != 2 {
		t.Fatal("existing destination ACK did not transfer lease once", err)
	}
	clock.Add(int64(31 * time.Second))
	m.state("")
	if out, err := m.cancelDrag("", req.ID); err != nil || out["state"] != "consumed" || m.documents[editorFixtureUID].owner != destination {
		t.Fatal("ticket expiry/cancellation reclaimed accepted transfer", err)
	}
	if out, err := m.offer("", req); err != nil || out["state"] != "accepted" || m.documents[editorFixtureUID].lease != 2 {
		t.Fatal("lost-offer response retry lost canonical accepted state", out, err)
	}
}

func TestEditorDragOfferRejectsWrongUIDLeaseDestinationAndNonce(t *testing.T) {
	for _, kind := range []string{"UID", "lease", "destination", "nonce", "without-ticket", "multiple-documents", "foreign-source"} {
		t.Run(kind, func(t *testing.T) {
			_, m, destination, other, _, _ := editorDragFixture(t)
			req := editorClaimedDrag(t, m, destination)
			original := req.ID
			caller := ""
			switch kind {
			case "UID":
				req.Batch.Documents[0].UID, req.Batch.Active = editorFixtureUID2, editorFixtureUID2
				req.Leases = map[string]uint64{editorFixtureUID2: 1}
			case "lease":
				req.Leases[editorFixtureUID] = 2
			case "destination":
				req.Destination = other
			case "nonce":
				req.ID = randomHex(16)
			case "without-ticket":
				req.DragTicket = ""
			case "multiple-documents":
				d := req.Batch.Documents[0]
				d.UID, d.ID = editorFixtureUID2, "other-local"
				req.Batch.Documents = append(req.Batch.Documents, d)
			case "foreign-source":
				caller = other
			}
			if _, err := m.offer(caller, req); err == nil {
				t.Fatal("invalid drag offer accepted")
			}
			if len(m.transfers) != 0 || m.drags[original].state != "claimed" || m.documents[editorFixtureUID].owner != "" || m.documents[editorFixtureUID].pending != "" {
				t.Fatal("rejected offer consumed ticket or changed ownership")
			}
		})
	}
}

func TestEditorDragExpiryCannotRenewOrOffer(t *testing.T) {
	_, m, destination, _, _, _ := editorDragFixture(t)
	clock := editorDragClock(m)
	ticket := randomHex(16)
	first, err := m.registerDrag("", ticket, editorFixtureUID, 1)
	if err != nil {
		t.Fatal(err)
	}
	clock.Add(int64(20 * time.Second))
	repeat, err := m.registerDrag("", ticket, editorFixtureUID, 1)
	if err != nil || repeat["expiresAt"] != first["expiresAt"] {
		t.Fatal("registration retry renewed gesture", repeat, err)
	}
	if _, err := m.claimDrag(destination, ticket, false); err != nil {
		t.Fatal(err)
	}
	clock.Add(int64(10 * time.Second))
	if _, err := m.claimDrag(destination, ticket, false); err == nil {
		t.Fatal("claim valid at or after exact expiry")
	}
	if _, err := m.registerDrag("", ticket, editorFixtureUID, 1); err == nil {
		t.Fatal("expired ticket renewed")
	}
	req := editorTransferRequest{ID: ticket, DragTicket: ticket, Destination: destination, Batch: editorFixtureBatch(), Leases: map[string]uint64{editorFixtureUID: 1}}
	if _, err := m.offer("", req); err == nil || len(m.transfers) != 0 {
		t.Fatal("expired claim started a transfer")
	}
	if len(m.state("")["dragRequests"].([]editorDragRequest)) != 0 || m.documents[editorFixtureUID].owner != "" {
		t.Fatal("expired claim remained actionable or lost source")
	}
}

func TestEditorDragCancellationBeforeRegistrationIsTombstoned(t *testing.T) {
	_, m, destination, _, _, _ := editorDragFixture(t)
	ticket := randomHex(16)
	if out, err := m.cancelDrag("", ticket); err != nil || out["state"] != "cancelled" {
		t.Fatal(out, err)
	}
	if _, err := m.registerDrag("", ticket, editorFixtureUID, 1); err == nil {
		t.Fatal("delayed registration resurrected cancelled drag")
	}
	if _, err := m.claimDrag(destination, ticket, false); err == nil {
		t.Fatal("cancelled ticket claimed")
	}
	if _, err := m.cancelDrag(destination, ticket); err == nil {
		t.Fatal("foreign renderer cancelled source gesture")
	}
}

func TestEditorDragClaimCancellationRaceHasOneSafeWinner(t *testing.T) {
	for i := 0; i < 25; i++ {
		_, m, destination, _, _, _ := editorDragFixture(t)
		ticket := randomHex(16)
		if _, err := m.registerDrag("", ticket, editorFixtureUID, 1); err != nil {
			t.Fatal(err)
		}
		var wg sync.WaitGroup
		wg.Add(2)
		go func() { defer wg.Done(); _, _ = m.claimDrag(destination, ticket, false) }()
		go func() { defer wg.Done(); _, _ = m.cancelDrag("", ticket) }()
		wg.Wait()
		m.mu.Lock()
		state := m.drags[ticket].state
		if state != "claimed" && state != "cancelled" || len(m.transfers) != 0 || m.documents[editorFixtureUID].pending != "" {
			t.Error("race created an invalid state or transfer", state)
		}
		m.mu.Unlock()
	}
}

func TestEditorDragOnlyOneReceiverCanClaim(t *testing.T) {
	_, m, first, second, _, _ := editorDragFixture(t)
	ticket := randomHex(16)
	if _, err := m.registerDrag("", ticket, editorFixtureUID, 1); err != nil {
		t.Fatal(err)
	}
	var wins atomic.Int32
	var wg sync.WaitGroup
	for _, caller := range []string{first, second} {
		wg.Add(1)
		go func(caller string) {
			defer wg.Done()
			if _, err := m.claimDrag(caller, ticket, false); err == nil {
				wins.Add(1)
			}
		}(caller)
	}
	wg.Wait()
	if wins.Load() != 1 {
		t.Fatal("claim did not choose exactly one receiver", wins.Load())
	}
}

func TestEditorDragStaleOrBusySourceAndClosedReceiverFailClosed(t *testing.T) {
	for _, kind := range []string{"lease", "owner", "deleted", "busy", "pending", "orphan", "source-close", "receiver-close", "shutdown"} {
		t.Run(kind, func(t *testing.T) {
			a, m, destination, _, _, _ := editorDragFixture(t)
			ticket := randomHex(16)
			if _, err := m.registerDrag("", ticket, editorFixtureUID, 1); err != nil {
				t.Fatal(err)
			}
			m.mu.Lock()
			d := m.documents[editorFixtureUID]
			switch kind {
			case "lease":
				d.lease++
			case "owner":
				d.owner = destination
			case "deleted":
				delete(m.documents, editorFixtureUID)
			case "busy":
				d.busy = 1
			case "pending":
				d.pending = randomHex(16)
			case "orphan":
				d.orphan = true
			case "source-close":
				m.homeClosing = true
			case "receiver-close":
				m.views[destination].info.State = "returned"
			case "shutdown":
				a.shutdownRequested.Store(true)
			}
			m.mu.Unlock()
			if _, err := m.claimDrag(destination, ticket, false); err == nil || len(m.transfers) != 0 {
				t.Fatal("stale gesture claimed")
			}
		})
	}
}

func TestEditorDragExplicitDetachIsSourceOnlyAndFailurePreservesDocument(t *testing.T) {
	a, m, destination, _, _, launches := editorDragFixture(t)
	ticket := randomHex(16)
	if _, err := m.registerDrag("", ticket, editorFixtureUID, 1); err != nil {
		t.Fatal(err)
	}
	if _, err := m.claimDrag(destination, ticket, true); err == nil {
		t.Fatal("receiver could force new-window detach")
	}
	if _, err := m.claimDrag("", ticket, false); err == nil {
		t.Fatal("same-window reorder became a cross-window claim")
	}
	if _, err := m.claimDrag("", ticket, true); err != nil || launches.Load() != 0 {
		t.Fatal("explicit zone claim itself launched a window", err)
	}
	a.launchEditorView = func(context.Context, editorViewLaunch) (<-chan error, error) {
		launches.Add(1)
		return nil, errors.New("synthetic unavailable executable")
	}
	req := editorTransferRequest{ID: ticket, DragTicket: ticket, Destination: "new", Batch: editorFixtureBatch(), Leases: map[string]uint64{editorFixtureUID: 1}}
	for i := 0; i < 2; i++ {
		out, err := m.offer("", req)
		if err != nil || out["state"] != "failed" || m.documents[editorFixtureUID].owner != "" || m.documents[editorFixtureUID].pending != "" {
			t.Fatal("failed launch lost source or canonical failed outcome", out, err)
		}
	}
	if launches.Load() != 1 || m.drags[ticket].state != "consumed" {
		t.Fatal("lost response launched twice or reused a ticket")
	}
}

func TestEditorDragRetentionIsBoundedAndUnknownTicketNeverClaims(t *testing.T) {
	_, m, destination, _, _, _ := editorDragFixture(t)
	clock := editorDragClock(m)
	old := randomHex(16)
	if _, err := m.cancelDrag("", old); err != nil {
		t.Fatal(err)
	}
	for i := 1; i < editorDragLimit; i++ {
		if _, err := m.cancelDrag("", randomHex(16)); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := m.registerDrag("", randomHex(16), editorFixtureUID, 1); err == nil || len(m.drags) != editorDragLimit {
		t.Fatal("gesture budget silently exceeded")
	}
	clock.Add(int64(editorDragRetention))
	if _, err := m.claimDrag(destination, old, false); err == nil {
		t.Fatal("pruned cancelled ticket claimed")
	}
	if _, err := m.registerDrag("", randomHex(16), editorFixtureUID, 1); err != nil || len(m.drags) != 1 {
		t.Fatal("expired gesture metadata leaked capacity", err)
	}
}

func TestEditorDragHTTPIsScopedSecretFreeAndStrict(t *testing.T) {
	a, m, destination, _, token, _ := editorDragFixture(t)
	ticket := randomHex(16)
	registration := map[string]any{"ticket": ticket, "uid": editorFixtureUID, "lease": 1}
	if r := editorHTTP(a, "", "/api/editor-views/drags", registration); r.Code != 401 {
		t.Fatal("anonymous registration", r.Code)
	}
	if r := editorHTTP(a, token, "/api/editor-views/drags", registration); r.Code != 409 {
		t.Fatal("child registered another owner's UID", r.Code)
	}
	for _, value := range []map[string]any{
		{"ticket": ticket, "uid": editorFixtureUID, "lease": 0},
		{"ticket": strings.ToUpper(ticket), "uid": editorFixtureUID, "lease": 1},
		{"ticket": "https://foreign.invalid/" + ticket, "uid": editorFixtureUID, "lease": 1},
		{"ticket": ticket, "uid": editorFixtureUID, "lease": 2},
	} {
		if r := editorHTTP(a, a.token, "/api/editor-views/drags", value); r.Code != 409 {
			t.Fatal("invalid registration", value, r.Code)
		}
	}
	if r := editorHTTP(a, a.token, "/api/editor-views/drags", registration); r.Code != 200 {
		t.Fatal(r.Code, r.Body.String())
	}
	for _, suffix := range []string{"claim", "detach", "cancel"} {
		if r := editorHTTP(a, token, "/api/editor-views/drags/"+suffix, map[string]any{"ticket": ticket, "destination": "main"}); r.Code != 400 {
			t.Fatal("action accepted destination override", suffix, r.Code)
		}
	}
	for _, origin := range []string{"https://foreign.invalid", "null"} {
		raw, _ := json.Marshal(map[string]any{"ticket": ticket})
		r := httptest.NewRequest(http.MethodPost, "http://"+a.host+"/api/editor-views/drags/claim", bytes.NewReader(raw))
		r.Header.Set("X-Session-Token", token)
		r.Header.Set("Origin", origin)
		w := httptest.NewRecorder()
		a.ServeHTTP(w, r)
		if w.Code != 403 {
			t.Fatal("foreign origin accepted", origin, w.Code)
		}
	}
	response := editorHTTP(a, token, "/api/editor-views/drags/claim", map[string]any{"ticket": ticket})
	if response.Code != 200 {
		t.Fatal(response.Code, response.Body.String())
	}
	var result map[string]any
	if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil || len(result) != 2 || result["state"] != "claimed" || result["expiresAt"] == nil {
		t.Fatal("claim response exposed more than state/expiry", response.Body.String())
	}
	if m.drags[ticket].destination != destination {
		t.Fatal("destination was not the authenticated receiver")
	}
	if r := editorGET(a, token, "/api/editor-views/drags"); r.Code != 405 {
		t.Fatal("drag inventory exposed", r.Code)
	}
	if r := editorHTTP(a, token, "/api/editor-views/drags/"+ticket+"/claim", map[string]any{}); r.Code != 404 {
		t.Fatal("ticket accepted in URL", r.Code)
	}
	other, _, _, _, _, _ := editorDragFixture(t)
	if r := editorHTTP(other, other.token, "/api/editor-views/drags/claim", map[string]any{"ticket": ticket}); r.Code != 404 {
		t.Fatal("foreign engine claimed gesture", r.Code)
	}
	for _, bad := range []string{`{"ticket":"` + ticket + `","text":"private-document-canary"}`, `{"ticket":"` + ticket + `"} {}`, fmt.Sprintf(`{"ticket":"%s"}`, strings.Repeat("a", 1100))} {
		r := requestApp(a, "POST", "/api/editor-views/drags/claim", bad, true)
		if r.Code != 400 || strings.Contains(r.Body.String(), "private-document-canary") {
			t.Fatal("invalid body accepted or reflected", r.Code)
		}
	}
}

// Only an authenticated, well-formed unknown ticket is transient. A cancelled,
// expired, stale or competing claim must never enter the quick-drop retry path.
func TestEditorDragHTTPQuickDropStatusContract(t *testing.T) {
	a, m, _, other, token, launches := editorDragFixture(t)
	clock := editorDragClock(m)
	post := func(auth, action, ticket string, want int) {
		t.Helper()
		r := editorHTTP(a, auth, "/api/editor-views/drags/"+action, map[string]any{"ticket": ticket})
		if r.Code != want {
			t.Fatalf("%s: got %d want %d: %s", action, r.Code, want, r.Body.String())
		}
		if strings.Contains(r.Body.String(), editorFixtureUID) || strings.Contains(r.Body.String(), "private-document-canary") || strings.Contains(r.Body.String(), ticket) {
			t.Fatal("claim response leaked document or ticket")
		}
	}
	register := func(ticket string) {
		t.Helper()
		r := editorHTTP(a, a.token, "/api/editor-views/drags", map[string]any{"ticket": ticket, "uid": editorFixtureUID, "lease": 1})
		if r.Code != 200 {
			t.Fatal(r.Code, r.Body.String())
		}
	}
	ticket := randomHex(16)
	post(token, "claim", ticket, 404)
	post(a.token, "detach", ticket, 404)
	if len(m.drags) != 0 || len(m.transfers) != 0 || launches.Load() != 0 {
		t.Fatal("unknown claim created state or launched")
	}
	register(ticket)
	post(token, "claim", ticket, 200)
	post(token, "claim", ticket, 200)
	post(m.views[other].token, "claim", ticket, 409)
	post(token, "claim", "malformed", 409)
	// Expiry does not extend when a claimant retries or change document ownership.
	clock.Add(int64(editorDragLifetime))
	post(token, "claim", ticket, 410)
	post(a.token, "detach", ticket, 410)
	post(token, "claim", ticket, 410)
	if len(m.transfers) != 0 || m.documents[editorFixtureUID].owner != "" {
		t.Fatal("expired claim moved document")
	}
	cancelled := randomHex(16)
	post(a.token, "cancel", cancelled, 200)
	post(token, "claim", cancelled, 410)
	post(a.token, "detach", cancelled, 410)
	r := editorHTTP(a, a.token, "/api/editor-views/drags", map[string]any{"ticket": cancelled, "uid": editorFixtureUID, "lease": 1})
	if r.Code != 409 {
		t.Fatal("late registration bypassed cancellation", r.Code)
	}
	stale := randomHex(16)
	register(stale)
	m.mu.Lock()
	m.documents[editorFixtureUID].lease++
	m.mu.Unlock()
	post(token, "claim", stale, 409)
	m.mu.Lock()
	m.homeClosing = true
	m.mu.Unlock()
	post(a.token, "detach", randomHex(16), 409)
}
