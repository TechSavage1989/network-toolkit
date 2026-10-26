package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"networktoolkit/internal/model"
)

func TestLiveJobPollingChangesAfterRetainedEventsFill(t *testing.T) {
	a := testApplication(t)
	j := &Job{ID: strings.Repeat("d", 24), Tool: "discover", Target: "192.0.2.0/24", Status: "running", Events: []model.Event{}}
	a.jobs[j.ID] = j
	for i := 0; i < maxEvents; i++ {
		a.recordJobEvent(j, model.Event{Message: fmt.Sprintf("retained event %d", i)})
	}
	before := requestApp(a, http.MethodGet, "/api/jobs?brief=1", "", true)
	if before.Code != http.StatusOK {
		t.Fatal(before.Code, before.Body.String())
	}
	progress := map[string]any{"scope": "subnet", "completed": 190, "total": 254}
	a.recordJobEvent(j, model.Event{Message: "progress beyond retained event limit", Data: map[string]any{"type": "progress", "value": progress}})
	progress["completed"] = 0 // A producer cannot mutate an already published observation.
	after := requestApp(a, http.MethodGet, "/api/jobs?brief=1", "", true)
	if after.Code != http.StatusOK || bytes.Equal(before.Body.Bytes(), after.Body.Bytes()) {
		t.Fatal("A running scan became invisible to brief polling once its retained history filled.")
	}
	var brief []struct {
		ID            string `json:"id"`
		Status        string `json:"status"`
		EventCount    int    `json:"eventCount"`
		EventSequence uint64 `json:"eventSequence"`
	}
	if err := json.Unmarshal(after.Body.Bytes(), &brief); err != nil {
		t.Fatal(err)
	}
	if len(brief) != 1 || brief[0].ID != j.ID || brief[0].Status != "running" || brief[0].EventCount != maxEvents || brief[0].EventSequence != maxEvents+1 {
		t.Fatalf("Polling metadata lost its bounded count or monotonic cursor: %+v", brief)
	}
	full := requestApp(a, http.MethodGet, "/api/jobs/"+j.ID, "", true)
	var result Job
	if full.Code != http.StatusOK || json.Unmarshal(full.Body.Bytes(), &result) != nil {
		t.Fatal("Full running result unavailable:", full.Body.String())
	}
	if len(result.Events) != maxEvents || result.EventSequence != brief[0].EventSequence || result.Events[0].Message != "retained event 1" {
		t.Fatalf("Brief and full cursors or bounded retention disagree: %+v", result)
	}
	envelope, ok := result.Events[len(result.Events)-1].Data.(map[string]any)
	if !ok {
		t.Fatal("Missing latest progress envelope")
	}
	value, ok := envelope["value"].(map[string]any)
	if !ok || value["completed"] != float64(190) || value["scope"] != "subnet" {
		t.Fatalf("Latest progress was lost or changed after publication: %+v", envelope)
	}
}

func TestLiveInventoryRetainsHostsAfterTheirEventsRollOff(t *testing.T) {
	a := testApplication(t)
	j := &Job{ID: strings.Repeat("e", 24), Tool: "discover", Target: "192.0.2.0/24", Status: "running", Events: []model.Event{}}
	a.jobs[j.ID] = j
	first := map[string]any{"address": "192.0.2.1", "responded": true, "completed": true, "vendor": "Original vendor"}
	a.recordJobEvent(j, model.Event{Data: map[string]any{"type": "host", "value": first}})
	first["vendor"] = "Changed after publication"
	for i := 0; i < maxEvents; i++ {
		a.recordJobEvent(j, model.Event{Data: map[string]any{"type": "progress", "value": map[string]any{"scope": "subnet", "completed": 1, "total": 254, "completedProbes": i}}})
	}
	a.recordJobEvent(j, model.Event{Data: map[string]any{"type": "host", "value": map[string]any{"address": "192.0.2.2", "responded": false, "completed": true}}})
	full := requestApp(a, http.MethodGet, "/api/jobs/"+j.ID, "", true)
	var result Job
	if full.Code != http.StatusOK || json.Unmarshal(full.Body.Bytes(), &result) != nil {
		t.Fatal("Full live result unavailable:", full.Body.String())
	}
	if len(result.Events) != maxEvents || len(result.LiveHosts) != 2 {
		t.Fatalf("Activity history and live inventory did not retain independent limits: events=%d hosts=%d", len(result.Events), len(result.LiveHosts))
	}
	earliest, ok := result.LiveHosts["192.0.2.1"].(map[string]any)
	if !ok || earliest["vendor"] != "Original vendor" || earliest["responded"] != true {
		t.Fatalf("An earlier device disappeared or its frozen data changed: %+v", earliest)
	}
	for _, event := range result.Events {
		encoded, _ := json.Marshal(event.Data)
		if bytes.Contains(encoded, []byte("192.0.2.1")) {
			t.Fatal("Fixture did not roll the earlier host out of activity history")
		}
	}
	brief := requestApp(a, http.MethodGet, "/api/jobs?brief=1", "", true)
	if bytes.Contains(brief.Body.Bytes(), []byte("liveHosts")) || bytes.Contains(brief.Body.Bytes(), []byte("Original vendor")) {
		t.Fatal("Brief polling unexpectedly copied the full live inventory")
	}
}

func TestLiveSampleTotalsSurviveActivityHistoryTrimming(t *testing.T) {
	a := testApplication(t)
	j := &Job{ID: strings.Repeat("f", 24), Tool: "monitor", Status: "running", Events: []model.Event{}}
	a.jobs[j.ID] = j
	for i := 1; i <= maxEvents+20; i++ {
		stats := map[string]any{"sent": i, "received": i, "lossPercent": 0}
		a.recordJobEvent(j, model.Event{Data: map[string]any{"type": "sample", "value": map[string]any{"sequence": i}, "statistics": stats}})
		stats["sent"] = 0
	}
	response := requestApp(a, http.MethodGet, "/api/jobs/"+j.ID, "", true)
	var job Job
	if response.Code != http.StatusOK || json.Unmarshal(response.Body.Bytes(), &job) != nil {
		t.Fatal("live sample job unavailable")
	}
	if len(job.Events) != maxEvents || job.SampleStatistics.(map[string]any)["sent"] != float64(maxEvents+20) {
		t.Fatal("lifetime sample totals were lost or changed after publication")
	}
}
