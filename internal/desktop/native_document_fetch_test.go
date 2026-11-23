package desktop

import (
	"encoding/json"
	"strings"
	"testing"
)

func documentFetchTestEvent(t *testing.T, address string) nativeDocumentFetchEvent {
	t.Helper()
	raw, err := json.Marshal(map[string]any{"requestId": "request-1", "frameId": "frame-1", "resourceType": "Document", "request": map[string]string{"url": address, "method": "GET"}})
	if err != nil {
		t.Fatal(err)
	}
	event, err := decodeNativeDocumentFetchEvent(string(raw))
	if err != nil {
		t.Fatal(err)
	}
	return event
}

func TestNativeBootstrapDocumentFetchAllowsOnlyInitialRootRequest(t *testing.T) {
	const appURL = "http://127.0.0.1:1234/"
	for _, scenario := range []string{"initial", "unarmed", "child-session", "redirect", "post", "fragment", "query", "other-document", "other-origin", "noncanonical"} {
		t.Run(scenario, func(t *testing.T) {
			var permit nativeDocumentFetchPermit
			if scenario != "unarmed" && !permit.begin() {
				t.Fatal("initial native permit was not armed")
			}
			event := documentFetchTestEvent(t, appURL)
			session := ""
			switch scenario {
			case "child-session":
				session = "child-target"
			case "redirect":
				event.RedirectedRequestID = "previous-request"
			case "post":
				event.Request.Method = "POST"
			case "fragment":
				event.Request.URL += "#same-document"
			case "query":
				event.Request.URL += "?reload=1"
			case "other-document":
				event.Request.URL += "other"
			case "other-origin":
				event.Request.URL = "http://localhost:1234/"
			case "noncanonical":
				event.Request.URL = "http://127.0.0.1:01234/"
			}
			if allowed := permit.consume(appURL, session, event); allowed != (scenario == "initial") {
				t.Fatalf("request allowed=%t for %s", allowed, scenario)
			}
			if permit.pending || permit.consume(appURL, "", documentFetchTestEvent(t, appURL)) {
				t.Fatal("a later root request reused the startup permit")
			}
			if scenario != "unarmed" && permit.begin() {
				t.Fatal("an already-started native host rearmed its document permit")
			}
		})
	}
}

func TestNativeBootstrapDocumentFetchRejectsUnresolvableEvents(t *testing.T) {
	valid := `{"requestId":"request-1","frameId":"frame-1","resourceType":"Document","request":{"url":"http://127.0.0.1:1234/","method":"GET"}}`
	for name, raw := range map[string]string{
		"malformed":       "{",
		"missing-id":      strings.Replace(valid, `"request-1"`, `""`, 1),
		"nul-id":          strings.Replace(valid, `"request-1"`, `"request\u0000"`, 1),
		"missing-frame":   strings.Replace(valid, `"frame-1"`, `""`, 1),
		"other-resource":  strings.Replace(valid, `"Document"`, `"Fetch"`, 1),
		"missing-method":  strings.Replace(valid, `"GET"`, `""`, 1),
		"response-stage":  strings.Replace(valid, `"frameId"`, `"responseStatusCode":200,"frameId"`, 1),
		"null-response":   strings.Replace(valid, `"frameId"`, `"responseStatusCode":null,"frameId"`, 1),
		"response-error":  strings.Replace(valid, `"frameId"`, `"responseErrorReason":"Aborted","frameId"`, 1),
		"oversized-event": valid + strings.Repeat(" ", 64<<10),
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := decodeNativeDocumentFetchEvent(raw); err == nil {
				t.Fatal("invalid interception could be continued")
			}
		})
	}
}

func TestNativeBootstrapDocumentFetchResolutionPreservesRequestAndAbortsDenials(t *testing.T) {
	event := documentFetchTestEvent(t, "http://127.0.0.1:1234/")
	event.RequestID = "request-\"quoted\\id"
	for _, allowed := range []bool{false, true} {
		method, parameters := nativeDocumentFetchCommand(event, allowed)
		var decoded map[string]string
		if err := json.Unmarshal([]byte(parameters), &decoded); err != nil || decoded["requestId"] != event.RequestID {
			t.Fatal("request identity was not JSON encoded", err)
		}
		if allowed {
			if method != "Fetch.continueRequest" || len(decoded) != 1 {
				t.Fatal("initial request was modified instead of continued unchanged")
			}
		} else if method != "Fetch.failRequest" || decoded["errorReason"] != "Aborted" || len(decoded) != 2 {
			t.Fatal("denied document was not aborted at the request stage")
		}
	}
}
