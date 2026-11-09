package main

import (
	"encoding/json"
	"net/http/httptest"
	"networktoolkit/internal/editorbuffer"
	"strings"
	"testing"
	"time"
)

func TestDiskEditorCompareMixedDocumentsAndLease(t *testing.T) {
	a, _, _, lease := diskFixture(t)
	id := strings.Repeat("1", 32)
	call := func(action string, body map[string]any) *httptest.ResponseRecorder {
		body["action"], body["id"] = action, id
		return editorHTTP(a, a.token, "/api/editor-views/buffers/compare", body)
	}
	t.Cleanup(func() { call("cancel", map[string]any{}) })
	left := map[string]any{"uid": editorFixtureUID, "lease": lease + 1, "revision": 1}
	right := map[string]string{"text": "before\ninserted\nλ😀\n"}
	if r := call("start", map[string]any{"left": left, "right": right}); r.Code != 409 {
		t.Fatal("comparison accepted another owner's lease", r.Code, r.Body)
	}
	left["lease"] = lease
	if r := call("start", map[string]any{"left": left, "right": right}); r.Code != 202 {
		t.Fatal(r.Code, r.Body)
	}
	deadline := time.Now().Add(3 * time.Second)
	for {
		r := call("status", map[string]any{})
		var state diskComparisonStatus
		if r.Code != 200 || json.Unmarshal(r.Body.Bytes(), &state) != nil || state.Error != "" {
			t.Fatal(r.Code, r.Body)
		}
		if state.Done {
			if state.Count != 1 || state.LeftLines != 3 || state.RightLines != 4 || state.Rows != 4 || state.ChangedRows != 1 {
				t.Fatal(state)
			}
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("comparison did not finish")
		}
		time.Sleep(time.Millisecond)
	}
	changed := call("changed-rows", map[string]any{"start": 0, "limit": 8})
	var runs []diskChangedRun
	if changed.Code != 200 || json.Unmarshal(changed.Body.Bytes(), &runs) != nil || len(runs) != 1 ||
		runs[0].Count != 1 || runs[0].Offset != 0 || runs[0].LeftCount != 0 || runs[0].RightCount != 1 {
		t.Fatal(changed.Code, changed.Body)
	}
	page := call("page", map[string]any{"start": 0, "limit": 5})
	var result struct {
		Differences []editorbuffer.Difference `json:"differences"`
	}
	if page.Code != 200 || json.Unmarshal(page.Body.Bytes(), &result) != nil || len(result.Differences) != 1 || result.Differences[0] != (editorbuffer.Difference{Left: 1, Right: 1, RightCount: 1}) {
		t.Fatal(page.Code, page.Body)
	}
	location := call("locate", map[string]any{"side": "left", "line": 1, "end": 2, "limit": 5})
	var mapping struct {
		Aligned  bool  `json:"aligned"`
		PeerLine int64 `json:"peerLine"`
	}
	if location.Code != 200 || json.Unmarshal(location.Body.Bytes(), &mapping) != nil || !mapping.Aligned || mapping.PeerLine != 2 {
		t.Fatal(location.Code, location.Body)
	}
	marks := call("map", map[string]any{"start": 0, "end": 3})
	if marks.Code != 200 || strings.TrimSpace(marks.Body.String()) != `{"left":[0,0,0],"right":[0,2,0]}` {
		t.Fatal(marks.Code, marks.Body)
	}
	if oversized := call("map", map[string]any{"start": 0, "end": diskCompareMapWindow + 1}); oversized.Code != 400 {
		t.Fatal(oversized.Code, oversized.Body)
	}
	geometry := call("alignment", map[string]any{"side": "left", "line": 1, "end": 2})
	if geometry.Code != 200 || strings.TrimSpace(geometry.Body.String()) != `{"gaps":[{"line":1,"count":1}],"row":1}` {
		t.Fatal(geometry.Code, geometry.Body)
	}
	for _, check := range []struct {
		side string
		row  int
		want string
	}{
		{"left", 1, `{"gap":1,"line":1}`},
		{"right", 1, `{"line":1}`},
		{"left", 2, `{"line":1}`},
		{"right", 2, `{"line":2}`},
	} {
		result := call("alignment-row", map[string]any{"side": check.side, "line": check.row})
		if result.Code != 200 || strings.TrimSpace(result.Body.String()) != check.want {
			t.Fatal(result.Code, result.Body)
		}
	}
	for side, want := range map[string]string{"left": "[0,-1,1,2]", "right": "[0,1,2,3]"} {
		result := call("alignment-wrap", map[string]any{"side": side, "rows": []int{0, 1, 2, 3}})
		if result.Code != 200 || strings.TrimSpace(result.Body.String()) != want {
			t.Fatal(result.Code, result.Body)
		}
	}
	if result := call("alignment-wrap", map[string]any{"side": "left", "rows": []int{4}}); result.Code != 400 {
		t.Fatal(result.Code, result.Body)
	}
	if edit := diskCall(a, a.token, lease, map[string]any{"action": "edit", "revision": 1, "from": 0, "to": 0, "text": "new "}); edit.Code != 200 {
		t.Fatal(edit.Code, edit.Body)
	}
	if stale := call("status", map[string]any{}); stale.Code != 409 && stale.Code != 410 {
		t.Fatal("stale result stayed available", stale.Code, stale.Body)
	}
}
