package desktop

import (
	"encoding/json"
	"errors"
	"strings"
)

const nativeDocumentFetchParameters = `{"patterns":[{"urlPattern":"*","resourceType":"Document","requestStage":"Request"}]}`

type nativeDocumentFetchEvent struct {
	RequestID           string `json:"requestId"`
	FrameID             string `json:"frameId"`
	ResourceType        string `json:"resourceType"`
	RedirectedRequestID string `json:"redirectedRequestId"`
	ResponseErrorReason json.RawMessage `json:"responseErrorReason,omitempty"`
	ResponseStatusCode  json.RawMessage `json:"responseStatusCode,omitempty"`
	Request             struct {
		URL    string `json:"url"`
		Method string `json:"method"`
	} `json:"request"`
}

func decodeNativeDocumentFetchEvent(raw string) (nativeDocumentFetchEvent, error) {
	var event nativeDocumentFetchEvent
	if len(raw) > 64<<10 || json.Unmarshal([]byte(raw), &event) != nil ||
		event.RequestID == "" || len(event.RequestID) > 256 || strings.ContainsRune(event.RequestID, '\x00') ||
		event.FrameID == "" || len(event.FrameID) > 256 || event.ResourceType != "Document" ||
		event.Request.URL == "" || len(event.Request.URL) > 8192 || event.Request.Method == "" ||
		event.ResponseErrorReason != nil || event.ResponseStatusCode != nil {
		return nativeDocumentFetchEvent{}, errors.New("The native document interceptor returned an invalid request")
	}
	return event, nil
}

// Fetch is an independent network-stage permit. NavigationStarting and
// WebResourceRequested have their own permits because their callbacks can run
// in either order. No renderer exists when the native initial Navigate arms it.
type nativeDocumentFetchPermit struct {
	started bool
	pending bool
}

func (p *nativeDocumentFetchPermit) begin() bool {
	if p.started {
		return false
	}
	p.started, p.pending = true, true
	return true
}

func (p *nativeDocumentFetchPermit) consume(appURL, sessionID string, event nativeDocumentFetchEvent) bool {
	allowed := p.pending && sessionID == "" && event.Request.Method == "GET" &&
		event.RedirectedRequestID == "" && event.Request.URL == appURL && sameNativeDocument(appURL, event.Request.URL)
	p.pending = false
	return allowed
}

func nativeDocumentFetchCommand(event nativeDocumentFetchEvent, allowed bool) (string, string) {
	if allowed {
		parameters, _ := json.Marshal(struct {
			RequestID string `json:"requestId"`
		}{event.RequestID})
		return "Fetch.continueRequest", string(parameters)
	}
	parameters, _ := json.Marshal(struct {
		RequestID   string `json:"requestId"`
		ErrorReason string `json:"errorReason"`
	}{event.RequestID, "Aborted"})
	return "Fetch.failRequest", string(parameters)
}
