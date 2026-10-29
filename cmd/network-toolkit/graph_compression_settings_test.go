package main

import (
	"context"
	"encoding/json"
	"reflect"
	"testing"
)

func TestGraphCompressionPreferencesSurviveApplicationRestart(t *testing.T) {
	a := testApplication(t)
	body := `{"graphTimeRange":{"durationMs":10000,"anchor":"latest","start":"","end":"","recentMs":60000,"invertWheelZoom":true,"compression":{"count":37,"strength":0.75,"curve":0.2}}}`
	response := requestApp(a, "PUT", "/api/state", body, true)
	if response.Code != 200 {
		t.Fatalf("state write: %d %s", response.Code, response.Body.String())
	}
	b := newApplication(context.Background(), a.dataPath)
	defer b.stop()
	b.host = a.host
	response = requestApp(b, "GET", "/api/state", "", true)
	var got, want map[string]any
	if err := json.Unmarshal(response.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal([]byte(body), &want); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got["graphTimeRange"], want["graphTimeRange"]) {
		t.Fatal("graph preference was changed or dropped during state restart")
	}
}
