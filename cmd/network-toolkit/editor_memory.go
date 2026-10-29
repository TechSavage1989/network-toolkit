package main

import (
	"context"
	"net/http"
	"sync"
)

// One owner backend serves every editor renderer. Serialize large decode,
// codec and response allocations before reading the body, independently of
// retained registry bytes. Small activity/pulse/ACK requests never enter here.
var editorMemorySlots = make(chan struct{}, 1)

type editorMemoryContextKey struct{}

func admitEditorMemory(w http.ResponseWriter, r *http.Request) (*http.Request, func(), bool) {
	if r.Context().Value(editorMemoryContextKey{}) == true {
		return r, func() {}, true
	}
	if r.Context().Err() != nil {
		jsonError(w, http.StatusRequestTimeout, "Editor operation cancelled before processing.")
		return r, nil, false
	}
	select {
	case editorMemorySlots <- struct{}{}:
	case <-r.Context().Done():
		jsonError(w, http.StatusRequestTimeout, "Editor operation cancelled while waiting for another file operation.")
		return r, nil, false
	}
	var once sync.Once
	release := func() { once.Do(func() { <-editorMemorySlots }) }
	if r.Context().Err() != nil {
		release()
		jsonError(w, http.StatusRequestTimeout, "Editor operation cancelled before processing.")
		return r, nil, false
	}
	return r.WithContext(context.WithValue(r.Context(), editorMemoryContextKey{}, true)), release, true
}
