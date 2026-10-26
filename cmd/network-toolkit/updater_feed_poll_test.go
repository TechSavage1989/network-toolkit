package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func updatePollResponse(status int, body []byte, headers map[string]string) *http.Response {
	header := make(http.Header)
	for key, value := range headers {
		header.Set(key, value)
	}
	return &http.Response{StatusCode: status, Header: header, Body: io.NopCloser(bytes.NewReader(body)), ContentLength: int64(len(body))}
}

func TestUpdateFeedConditionalRoundTrip(t *testing.T) {
	calls := 0
	client := &http.Client{Transport: updateRoundTrip(func(r *http.Request) (*http.Response, error) {
		calls++
		if r.URL.String() != updateAPIURL || r.Method != http.MethodGet || r.Header.Get("Authorization") != "" || r.Header.Get("Cache-Control") != "no-cache" || r.Header.Get("User-Agent") == "" {
			t.Fatalf("unexpected feed request: %v %v", r.URL, r.Header)
		}
		if calls == 1 {
			if r.Header.Get("If-None-Match") != "" {
				t.Fatal("first request must not have a validator")
			}
			return updatePollResponse(200, feedFixture("1.17.66"), map[string]string{"ETag": `W/"first"`}), nil
		}
		if r.Header.Get("If-None-Match") != `W/"first"` {
			t.Fatal("validated ETag not reused")
		}
		return updatePollResponse(304, nil, map[string]string{"ETag": `"second"`}), nil
	})}
	latest, cache, err := updateFetchRelease(context.Background(), client, updateFeedCache{})
	if err != nil || latest == nil || latest.Version != "1.17.66" || cache.etag != `W/"first"` || !bytes.Equal(cache.payload, feedFixture("1.17.66")) {
		t.Fatalf("first check: latest=%+v cache=%+v err=%v", latest, cache, err)
	}
	previous := latest
	latest, cache, err = updateFetchRelease(context.Background(), client, cache)
	if err != nil || latest == nil || latest.Version != previous.Version || latest.Notes != previous.Notes || latest.packageURL != previous.packageURL || latest == previous || cache.etag != `"second"` || calls != 2 {
		t.Fatalf("304 must reparse and preserve a validated release: latest=%+v cache=%+v err=%v", latest, cache, err)
	}
}

func TestUpdateFeedRejects304WithoutValidCache(t *testing.T) {
	unsafe := strings.ReplaceAll(string(feedFixture("1.17.66")), "github.com/TechSavage1989", "evil.example/TechSavage1989")
	for name, cache := range map[string]updateFeedCache{
		"empty":            {},
		"missing payload":  {etag: `"valid"`},
		"missing etag":     {payload: feedFixture("1.17.66")},
		"malformed etag":   {etag: "not-an-etag", payload: feedFixture("1.17.66")},
		"invalid json":     {etag: `"valid"`, payload: []byte("{")},
		"untrusted assets": {etag: `"valid"`, payload: []byte(unsafe)},
		"oversized":        {etag: `"valid"`, payload: bytes.Repeat([]byte("x"), updateFeedMaxBody+1)},
	} {
		t.Run(name, func(t *testing.T) {
			client := &http.Client{Transport: updateRoundTrip(func(r *http.Request) (*http.Response, error) {
				return updatePollResponse(304, nil, map[string]string{"ETag": `"replacement"`}), nil
			})}
			latest, result, err := updateFetchRelease(context.Background(), client, cache)
			if err == nil || latest != nil || result.etag != cache.etag || !bytes.Equal(result.payload, cache.payload) {
				t.Fatalf("invalid cache accepted or changed: latest=%+v err=%v", latest, err)
			}
		})
	}
}

func TestUpdateFeedMalformedResponsesDoNotPoisonCache(t *testing.T) {
	original := updateFeedCache{etag: `"trusted"`, payload: feedFixture("1.17.66")}
	for name, bad := range map[string][]byte{
		"invalid JSON":      []byte("not JSON"),
		"unsafe repository": []byte(strings.ReplaceAll(string(feedFixture("1.17.67")), "network-toolkit/", "network-toolkit-releases/")),
		"prerelease":        []byte(strings.ReplaceAll(string(feedFixture("1.17.67")), `"tag_name":`, `"prerelease":true,"tag_name":`)),
		"missing asset":     []byte(`{"tag_name":"v1.17.67","assets":[]}`),
	} {
		t.Run(name, func(t *testing.T) {
			calls := 0
			client := &http.Client{Transport: updateRoundTrip(func(r *http.Request) (*http.Response, error) {
				calls++
				if r.Header.Get("If-None-Match") != original.etag {
					t.Fatal("invalid response poisoned validator")
				}
				if calls == 1 {
					return updatePollResponse(200, bad, map[string]string{"ETag": `"bad"`}), nil
				}
				return updatePollResponse(304, nil, nil), nil
			})}
			latest, cache, err := updateFetchRelease(context.Background(), client, original)
			if err == nil || latest != nil || cache.etag != original.etag || !bytes.Equal(cache.payload, original.payload) {
				t.Fatal("invalid response replaced validated data")
			}
			latest, _, err = updateFetchRelease(context.Background(), client, cache)
			if err != nil || latest == nil || latest.Version != "1.17.66" {
				t.Fatalf("last known good payload not retained: %+v %v", latest, err)
			}
		})
	}
}

func TestUpdateFeedLimitsAndTransportErrorsPreserveCache(t *testing.T) {
	original := updateFeedCache{etag: `"trusted"`, payload: feedFixture("1.17.66")}
	for _, length := range []int64{-1, updateFeedMaxBody + 1} {
		t.Run(fmt.Sprint(length), func(t *testing.T) {
			client := &http.Client{Transport: updateRoundTrip(func(r *http.Request) (*http.Response, error) {
				response := updatePollResponse(200, bytes.Repeat([]byte("x"), updateFeedMaxBody+1), map[string]string{"ETag": `"bad"`, "X-Poll-Interval": "60"})
				response.ContentLength = length
				return response, nil
			})}
			latest, cache, err := updateFetchRelease(context.Background(), client, original)
			if err == nil || latest != nil || cache.etag != original.etag || !bytes.Equal(cache.payload, original.payload) || !cache.checkAfter.After(time.Now()) {
				t.Fatalf("size/error state not retained: %+v %v", cache, err)
			}
		})
	}
	client := &http.Client{Transport: updateRoundTrip(func(r *http.Request) (*http.Response, error) {
		return nil, errors.New("offline")
	})}
	latest, cache, err := updateFetchRelease(context.Background(), client, original)
	if err == nil || latest != nil || cache.etag != original.etag || !bytes.Equal(cache.payload, original.payload) {
		t.Fatal("transport failure changed cached release")
	}
}

func TestUpdateFeedSuccessfulResponseWithoutETagClearsValidator(t *testing.T) {
	calls := 0
	client := &http.Client{Transport: updateRoundTrip(func(r *http.Request) (*http.Response, error) {
		calls++
		if calls == 2 && r.Header.Get("If-None-Match") != "" {
			t.Fatal("validator from older payload retained")
		}
		return updatePollResponse(200, feedFixture("1.17.67"), nil), nil
	})}
	cache := updateFeedCache{etag: `"old"`, payload: feedFixture("1.17.66")}
	for i := 0; i < 2; i++ {
		latest, next, err := updateFetchRelease(context.Background(), client, cache)
		if err != nil || latest == nil || latest.Version != "1.17.67" || next.etag != "" {
			t.Fatalf("valid untagged response failed: %+v %v", latest, err)
		}
		cache = next
	}
}

func TestUpdateFeedServerCooldownHeaders(t *testing.T) {
	now := time.Date(2026, 10, 25, 1, 0, 0, 0, time.UTC)
	for _, test := range []struct {
		name    string
		status  int
		headers map[string]string
		wait    time.Duration
	}{
		{"retry seconds", 503, map[string]string{"Retry-After": "120"}, 2 * time.Minute},
		{"retry date", 503, map[string]string{"Retry-After": now.Add(3 * time.Minute).Format(http.TimeFormat)}, 3 * time.Minute},
		{"exhausted reset", 200, map[string]string{"X-RateLimit-Remaining": "0", "X-RateLimit-Reset": strconv.FormatInt(now.Add(4*time.Minute).Unix(), 10)}, 4 * time.Minute},
		{"reset not exhausted", 200, map[string]string{"X-RateLimit-Remaining": "1", "X-RateLimit-Reset": strconv.FormatInt(now.Add(4*time.Minute).Unix(), 10)}, 0},
		{"poll interval", 304, map[string]string{"X-Poll-Interval": "300"}, 5 * time.Minute},
		{"latest deadline", 429, map[string]string{"Retry-After": "60", "X-Poll-Interval": "120", "X-RateLimit-Remaining": "0", "X-RateLimit-Reset": strconv.FormatInt(now.Add(4*time.Minute).Unix(), 10)}, 4 * time.Minute},
		{"forbidden fallback", 403, nil, time.Minute},
		{"rate limit fallback", 429, nil, time.Minute},
		{"minimum after expired guidance", 429, map[string]string{"Retry-After": "0", "X-RateLimit-Remaining": "0", "X-RateLimit-Reset": strconv.FormatInt(now.Add(-time.Minute).Unix(), 10)}, time.Minute},
		{"invalid numeric headers", 200, map[string]string{"Retry-After": "-5", "X-Poll-Interval": "1.5", "X-RateLimit-Remaining": "0", "X-RateLimit-Reset": "nope"}, 0},
		{"overflow headers", 200, map[string]string{"Retry-After": "9223372036854775807", "X-Poll-Interval": "9223372036854775807", "X-RateLimit-Remaining": "0", "X-RateLimit-Reset": "9223372036854775807"}, 0},
		{"expired date", 200, map[string]string{"Retry-After": now.Add(-time.Minute).Format(http.TimeFormat)}, 0},
	} {
		t.Run(test.name, func(t *testing.T) {
			header := make(http.Header)
			for key, value := range test.headers {
				header.Set(key, value)
			}
			after := updateFeedCheckAfter(header, test.status, now)
			if test.wait == 0 {
				if !after.IsZero() {
					t.Fatalf("unexpected cooldown: %v", after)
				}
			} else if !after.Equal(now.Add(test.wait)) {
				t.Fatalf("got %v, want %v", after, now.Add(test.wait))
			}
		})
	}
}

func TestUpdateFeedCooldownStopsNetworkAndExpires(t *testing.T) {
	calls := 0
	client := &http.Client{Transport: updateRoundTrip(func(r *http.Request) (*http.Response, error) {
		calls++
		return updatePollResponse(200, feedFixture("1.17.66"), nil), nil
	})}
	cache := updateFeedCache{checkAfter: time.Now().Add(time.Minute)}
	for i := 0; i < 3; i++ {
		latest, next, err := updateFetchRelease(context.Background(), client, cache)
		if !errors.Is(err, errUpdateFeedCooldown) || latest != nil || calls != 0 || !next.checkAfter.Equal(cache.checkAfter) {
			t.Fatal("manual/automatic cooldown bypassed")
		}
	}
	cache.checkAfter = time.Now().Add(-time.Second)
	if latest, _, err := updateFetchRelease(context.Background(), client, cache); err != nil || latest == nil || calls != 1 {
		t.Fatalf("expired cooldown did not recover: %+v %v calls=%d", latest, err, calls)
	}
}

func TestUpdateFeedResponseCooldownAppliesToEveryOutcome(t *testing.T) {
	for _, test := range []struct {
		name    string
		status  int
		body    []byte
		headers map[string]string
		valid   bool
	}{
		{"success", 200, feedFixture("1.17.67"), map[string]string{"X-Poll-Interval": "120"}, true},
		{"unchanged", 304, nil, map[string]string{"X-Poll-Interval": "120"}, true},
		{"invalid metadata", 200, []byte("invalid"), map[string]string{"X-Poll-Interval": "120"}, false},
		{"server unavailable", 503, nil, map[string]string{"Retry-After": "120"}, false},
		{"rate limited", 429, nil, nil, false},
		{"exhausted quota", 403, nil, map[string]string{"X-RateLimit-Remaining": "0", "X-RateLimit-Reset": strconv.FormatInt(time.Now().Add(10*time.Minute).Unix(), 10)}, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			calls := 0
			client := &http.Client{Transport: updateRoundTrip(func(r *http.Request) (*http.Response, error) {
				calls++
				return updatePollResponse(test.status, test.body, test.headers), nil
			})}
			original := updateFeedCache{etag: `"trusted"`, payload: feedFixture("1.17.66")}
			latest, cache, err := updateFetchRelease(context.Background(), client, original)
			if (err == nil) != test.valid || (latest != nil) != test.valid || !cache.checkAfter.After(time.Now().Add(59*time.Second)) {
				t.Fatalf("response cooldown/result lost: %+v %v %+v", latest, err, cache)
			}
			if !test.valid && (cache.etag != original.etag || !bytes.Equal(cache.payload, original.payload)) {
				t.Fatal("failed response poisoned cached metadata")
			}
			if _, _, err = updateFetchRelease(context.Background(), client, cache); !errors.Is(err, errUpdateFeedCooldown) || calls != 1 {
				t.Fatalf("server deadline not enforced before another request: calls=%d err=%v", calls, err)
			}
		})
	}
}

func TestUpdateFeedDeadlineMillisecondsRoundUp(t *testing.T) {
	if (updateFeedCache{}).checkAfterMillis() != 0 {
		t.Fatal("empty cache exposed a deadline")
	}
	now := time.Date(2026, 10, 25, 1, 0, 0, 500001, time.UTC)
	if got := (updateFeedCache{checkAfter: now}).checkAfterMillis(); got != now.UnixMilli()+1 {
		t.Fatalf("browser deadline permits a premature retry: %d", got)
	}
}

func TestUpdateFeedETagValidation(t *testing.T) {
	for _, valid := range []string{`"abc"`, `W/"abc"`, `""`} {
		if updateFeedETag(valid) != valid {
			t.Fatalf("valid ETag rejected: %q", valid)
		}
	}
	for _, invalid := range []string{"abc", "w/\"abc\"", "\"a\nb\"", `"with space"`, `"a"b"`, strings.Repeat("x", 1025)} {
		if updateFeedETag(invalid) != "" {
			t.Fatalf("invalid ETag accepted: %q", invalid)
		}
	}
}

// Exercise the feed handler without the platform-only outer install guard.
func updatePollHandle(a *application, path, body string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	a.updater.mu.Lock()
	defer a.updater.mu.Unlock()
	a.handleUpdateFeedLocked(w, r)
	return w
}

func updatePollTransport(t *testing.T, transport http.RoundTripper) {
	t.Helper()
	old := http.DefaultTransport
	http.DefaultTransport = transport
	t.Cleanup(func() { http.DefaultTransport = old })
}

func TestUpdateFeedHandlerPreservesStatusAndCooldown(t *testing.T) {
	calls := 0
	updatePollTransport(t, updateRoundTrip(func(r *http.Request) (*http.Response, error) {
		calls++
		return updatePollResponse(403, nil, map[string]string{"Retry-After": "120"}), nil
	}))
	a := &application{}
	a.updater.latest, _ = updateParseRelease(feedFixture("1.17.66"))
	a.updater.checked = true
	first := updatePollHandle(a, "/api/updates/check", "")
	if first.Code != 502 || calls != 1 || !a.updater.checked || a.updater.latest.Version != "1.17.66" || a.updater.networkBusy {
		t.Fatalf("first failure disturbed status: %d %s calls=%d", first.Code, first.Body, calls)
	}
	var result struct {
		Error      string `json:"error"`
		CheckAfter int64  `json:"checkAfter"`
	}
	if err := json.Unmarshal(first.Body.Bytes(), &result); err != nil || result.Error == "" || result.CheckAfter <= time.Now().Add(time.Minute).UnixMilli() {
		t.Fatalf("missing error deadline: %s %v", first.Body, err)
	}
	for i := 0; i < 2; i++ {
		blocked := updatePollHandle(a, "/api/updates/check", "")
		if blocked.Code != 429 || calls != 1 {
			t.Fatalf("cooldown request was not blocked: %d %s calls=%d", blocked.Code, blocked.Body, calls)
		}
	}
	status := httptest.NewRecorder()
	a.handleUpdates(status, httptest.NewRequest(http.MethodGet, "/api/updates/status", nil))
	var state struct {
		Checked    bool           `json:"checked"`
		Latest     *updateRelease `json:"latest"`
		CheckAfter int64          `json:"checkAfter"`
	}
	if err := json.Unmarshal(status.Body.Bytes(), &state); err != nil || !state.Checked || state.Latest == nil || state.Latest.Version != "1.17.66" || state.CheckAfter != result.CheckAfter {
		t.Fatalf("status/reload lost cooldown or last good release: %s %v", status.Body, err)
	}
}

func TestUpdateFeedSingleInflightAndResponsiveStatus(t *testing.T) {
	entered, release := make(chan struct{}), make(chan struct{})
	var once sync.Once
	unblock := func() { once.Do(func() { close(release) }) }
	defer unblock()
	var calls atomic.Int32
	updatePollTransport(t, updateRoundTrip(func(r *http.Request) (*http.Response, error) {
		calls.Add(1)
		if deadline, ok := r.Context().Deadline(); !ok || time.Until(deadline) > 20*time.Second {
			return nil, errors.New("feed request missing its bounded deadline")
		}
		close(entered)
		<-release
		return updatePollResponse(200, feedFixture("1.17.66"), nil), nil
	}))
	a := &application{}
	done := make(chan *httptest.ResponseRecorder, 1)
	go func() { done <- updatePollHandle(a, "/api/updates/check", "") }()
	select {
	case <-entered:
	case <-time.After(2 * time.Second):
		t.Fatal("feed request did not start")
	}
	status := httptest.NewRecorder()
	a.handleUpdates(status, httptest.NewRequest(http.MethodGet, "/api/updates/status", nil))
	if status.Code != 200 {
		t.Fatal("status unavailable during check")
	}
	for _, path := range []string{"/api/updates/check", "/api/updates/download"} {
		if blocked := updatePollHandle(a, path, `{"version":"1.17.66"}`); blocked.Code != 409 || calls.Load() != 1 {
			t.Fatalf("single inflight guard lost: %s returned %d", path, blocked.Code)
		}
	}
	unblock()
	select {
	case first := <-done:
		if first.Code != 200 || !a.updater.checked || a.updater.networkBusy {
			t.Fatalf("first check did not complete: %d %s", first.Code, first.Body)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("first check did not finish")
	}
}

func TestUpdateFeedCooldownDoesNotChangeDownloadFetch(t *testing.T) {
	var urls []string
	updatePollTransport(t, updateRoundTrip(func(r *http.Request) (*http.Response, error) {
		urls = append(urls, r.URL.String())
		if r.Header.Get("If-None-Match") != "" || r.Header.Get("Authorization") != "" {
			t.Fatal("metadata headers leaked into download")
		}
		return updatePollResponse(200, []byte("not a package"), map[string]string{"ETag": `"asset"`, "Retry-After": "500"}), nil
	}))
	a := &application{}
	a.updater.latest, _ = updateParseRelease(feedFixture("99.0.0"))
	a.updater.feed = updateFeedCache{etag: `"metadata"`, payload: feedFixture("99.0.0"), checkAfter: time.Now().Add(time.Hour)}
	before := a.updater.feed
	result := updatePollHandle(a, "/api/updates/download", `{"version":"99.0.0"}`)
	if result.Code != 400 || len(urls) != 2 || urls[0] != a.updater.latest.sumsURL || urls[1] != a.updater.latest.packageURL {
		t.Fatalf("download was incorrectly throttled or changed: %d %s %v", result.Code, result.Body, urls)
	}
	if a.updater.feed.etag != before.etag || !bytes.Equal(a.updater.feed.payload, before.payload) || !a.updater.feed.checkAfter.Equal(before.checkAfter) || a.updater.networkBusy || a.updater.staged != nil {
		t.Fatal("download changed feed cache/cooldown or staged invalid data")
	}
}
