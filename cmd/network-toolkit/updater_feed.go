package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"networktoolkit/internal/model"
	"strconv"
	"strings"
	"time"
)

const updateRepository = "TechSavage1989/network-toolkit"
const updateAPIURL = "https://api.github.com/repos/" + updateRepository + "/releases/latest"
const updateBetaAPIURL = "https://api.github.com/repos/" + updateRepository + "/releases?per_page=30"

const updateFeedMaxBody = 2 << 20

var errUpdateFeedCooldown = errors.New("GitHub has temporarily limited update checks. Please wait until the next permitted check")

// Feed state belongs to this app process, not to a browser window. Only a
// successfully parsed trusted release can replace the cached payload and ETag.
// Download requests deliberately do not share this metadata cache or cooldown.
type updateFeedCache struct {
	etag       string
	payload    []byte
	checkAfter time.Time
	channel    string // Keep stable and beta ETags/payloads isolated.
}

func (cache updateFeedCache) checkAfterMillis() int64 {
	if cache.checkAfter.IsZero() {
		return 0
	}
	// Round up so a millisecond-resolution browser timer cannot retry just
	// before the backend's higher-resolution deadline.
	return cache.checkAfter.Add(time.Millisecond - time.Nanosecond).UnixMilli()
}

func updateFeedETag(raw string) string {
	if len(raw) > 1024 {
		return ""
	}
	value := strings.TrimPrefix(raw, "W/")
	if len(value) < 2 || value[0] != '"' || value[len(value)-1] != '"' {
		return ""
	}
	for _, c := range []byte(value[1 : len(value)-1]) {
		if c < 0x21 || c == '"' || c == 0x7f {
			return ""
		}
	}
	return raw
}

// Use the latest server deadline. Integer parsing is bounded before converting
// seconds to durations; Unix reset dates are restricted to representable calendar
// dates, which also keeps checkAfter inside JavaScript's exact integer range.
func updateFeedCheckAfter(header http.Header, status int, now time.Time) time.Time {
	var after time.Time
	extend := func(candidate time.Time) {
		if candidate.After(now) && candidate.After(after) {
			after = candidate
		}
	}
	secondsAfter := func(raw string) bool {
		raw = strings.TrimSpace(raw)
		if raw == "" || strings.IndexFunc(raw, func(c rune) bool { return c < '0' || c > '9' }) >= 0 {
			return false
		}
		seconds, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || seconds > int64(time.Duration(1<<63-1)/time.Second) {
			return false
		}
		extend(now.Add(time.Duration(seconds) * time.Second))
		return true
	}
	if retry := header.Get("Retry-After"); !secondsAfter(retry) {
		if date, err := http.ParseTime(retry); err == nil {
			extend(date)
		}
	}
	secondsAfter(header.Get("X-Poll-Interval"))
	if strings.TrimSpace(header.Get("X-RateLimit-Remaining")) == "0" {
		reset, err := strconv.ParseInt(strings.TrimSpace(header.Get("X-RateLimit-Reset")), 10, 64)
		if err == nil && reset > 0 && reset <= 253402300799 { // 9999-12-31T23:59:59Z
			extend(time.Unix(reset, 0))
		}
	}
	if status == http.StatusForbidden || status == http.StatusTooManyRequests {
		extend(now.Add(time.Minute))
	}
	return after
}

// The returned cache retains all previous validated data on errors, but records
// cooldown headers even when the response body or status cannot be accepted.
func updateFetchRelease(ctx context.Context, client *http.Client, cache updateFeedCache) (*updateRelease, updateFeedCache, error) {
	return updateFetchReleaseChannel(ctx, client, cache, "stable")
}
func updateFetchReleaseChannel(ctx context.Context, client *http.Client, cache updateFeedCache, channel string) (*updateRelease, updateFeedCache, error) {
	if channel != "stable" && channel != "beta" {
		return nil, cache, errors.New("Unsupported update channel")
	}
	oldChannel := cache.channel
	if oldChannel == "" {
		oldChannel = "stable"
	}
	if oldChannel != channel {
		// A cached stable response must never be interpreted as a beta feed
		// and vice versa. Preserve the service cooldown across the switch.
		cache.etag, cache.payload = "", nil
	}
	cache.channel = channel
	if time.Now().Before(cache.checkAfter) {
		return nil, cache, errUpdateFeedCooldown
	}
	endpoint := updateAPIURL
	if channel == "beta" {
		endpoint = updateBetaAPIURL
	}
	r, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, cache, err
	}
	r.Header.Set("User-Agent", "Network-Toolkit/"+model.Version)
	r.Header.Set("Accept", "application/vnd.github+json")
	r.Header.Set("Cache-Control", "no-cache")
	if etag := updateFeedETag(cache.etag); etag != "" && len(cache.payload) > 0 {
		r.Header.Set("If-None-Match", etag)
	}
	response, err := client.Do(r)
	if err != nil {
		return nil, cache, fmt.Errorf("Unable to reach GitHub: %w", err)
	}
	defer response.Body.Close()
	if after := updateFeedCheckAfter(response.Header, response.StatusCode, time.Now()); after.After(cache.checkAfter) {
		cache.checkAfter = after
	}
	if response.StatusCode == http.StatusNotModified {
		if r.Header.Get("If-None-Match") == "" || len(cache.payload) > updateFeedMaxBody {
			return nil, cache, errors.New("GitHub returned unchanged metadata without a valid cached release")
		}
		// Do not promote a 304 into a trusted release without the same asset,
		// stable-version, size and repository validation used for a fresh 200.
		latest, err := updateParseReleaseFeed(cache.payload, channel)
		if err == nil {
			if etag := updateFeedETag(response.Header.Get("ETag")); etag != "" {
				cache.etag = etag
			}
		}
		return latest, cache, err
	}
	if response.StatusCode == http.StatusNotFound {
		return nil, cache, errors.New("The public update feed is not available yet. Your current installation is unchanged")
	}
	if response.StatusCode == http.StatusForbidden || response.StatusCode == http.StatusTooManyRequests {
		return nil, cache, errors.New("GitHub has temporarily limited update requests. Try again later")
	}
	if response.StatusCode != http.StatusOK {
		return nil, cache, fmt.Errorf("GitHub returned HTTP %d; your current installation is unchanged", response.StatusCode)
	}
	if response.ContentLength > updateFeedMaxBody {
		return nil, cache, errors.New("Update response exceeds its size limit")
	}
	data, err := io.ReadAll(io.LimitReader(response.Body, updateFeedMaxBody+1))
	if err != nil {
		return nil, cache, err
	}
	if len(data) > updateFeedMaxBody {
		return nil, cache, errors.New("Update response exceeds its size limit")
	}
	latest, err := updateParseReleaseFeed(data, channel)
	if err != nil {
		return nil, cache, err
	}
	cache.etag = updateFeedETag(response.Header.Get("ETag"))
	cache.payload = data
	return latest, cache, nil
}

type updateRelease struct {
	Version     string `json:"version"`
	Notes       string `json:"notes"`
	Beta        bool   `json:"beta"`
	packageName string
	packageURL  string
	sumsURL     string
}

func updateSafeURL(raw string) bool {
	u, e := url.Parse(raw)
	if e != nil || u.Scheme != "https" || u.User != nil || u.Port() != "" {
		return false
	}
	switch strings.ToLower(u.Hostname()) {
	case "api.github.com", "github.com", "release-assets.githubusercontent.com", "objects.githubusercontent.com":
		return true
	}
	return false
}
func updateHTTPClient() *http.Client {
	return &http.Client{Timeout: 5 * time.Minute, CheckRedirect: func(r *http.Request, via []*http.Request) error {
		if len(via) >= 5 || !updateSafeURL(r.URL.String()) {
			return errors.New("Unsafe update redirect refused")
		}
		return nil
	}}
}

type updateProgress struct {
	Phase    string `json:"phase"`
	Version  string `json:"version"`
	Received int64  `json:"received"`
	Total    int64  `json:"total"`
}
type updateProgressReader struct {
	reader          io.Reader
	received, total int64
	report          func(int64, int64)
}

func (r *updateProgressReader) Read(b []byte) (int, error) {
	n, err := r.reader.Read(b)
	r.received += int64(n)
	if n > 0 {
		r.report(r.received, r.total)
	}
	return n, err
}
func updateFetch(ctx context.Context, client *http.Client, raw string, limit int64) ([]byte, error) {
	return updateFetchProgress(ctx, client, raw, limit, nil)
}
func updateFetchProgress(ctx context.Context, client *http.Client, raw string, limit int64, report func(int64, int64)) ([]byte, error) {
	if !updateSafeURL(raw) {
		return nil, errors.New("Unsafe update URL refused")
	}
	r, e := http.NewRequestWithContext(ctx, http.MethodGet, raw, nil)
	if e != nil {
		return nil, e
	}
	r.Header.Set("User-Agent", "Network-Toolkit/"+model.Version)
	r.Header.Set("Accept", "application/vnd.github+json")
	r.Header.Set("Cache-Control", "no-cache")
	response, e := client.Do(r)
	if e != nil {
		return nil, fmt.Errorf("Unable to reach GitHub: %w", e)
	}
	defer response.Body.Close()
	if response.StatusCode == 404 {
		return nil, errors.New("The public update feed is not available yet. Your current installation is unchanged")
	}
	if response.StatusCode == 403 || response.StatusCode == 429 {
		return nil, errors.New("GitHub has temporarily limited update requests. Try again later")
	}
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub returned HTTP %d; your current installation is unchanged", response.StatusCode)
	}
	if response.ContentLength > limit {
		return nil, errors.New("Update response exceeds its size limit")
	}
	var reader io.Reader = io.LimitReader(response.Body, limit+1)
	if report != nil {
		total := response.ContentLength
		if total < 0 {
			total = 0
		}
		report(0, total)
		reader = &updateProgressReader{reader: reader, total: total, report: report}
	}
	b, e := io.ReadAll(reader)
	if e != nil {
		return nil, e
	}
	if int64(len(b)) > limit {
		return nil, errors.New("Update response exceeds its size limit")
	}
	return b, nil
}
func updateParseRelease(data []byte) (*updateRelease, error) {
	return updateParseReleaseCandidate(data, false)
}
func updateParseReleaseCandidate(data []byte, acceptBeta bool) (*updateRelease, error) {
	var item struct {
		Tag        string `json:"tag_name"`
		Body       string `json:"body"`
		Draft      bool   `json:"draft"`
		Prerelease bool   `json:"prerelease"`
		Assets     []struct {
			Name string `json:"name"`
			URL  string `json:"browser_download_url"`
			Size int64  `json:"size"`
		} `json:"assets"`
	}
	if e := json.Unmarshal(data, &item); e != nil {
		return nil, errors.New("GitHub returned invalid release metadata")
	}
	version := strings.TrimPrefix(item.Tag, "v")
	if item.Tag != "v"+version || !updateVersionPattern.MatchString(version) || item.Draft || (item.Prerelease && !acceptBeta) {
		return nil, errors.New("Update feed did not provide a stable version")
	}
	result := &updateRelease{Version: version, Notes: item.Body, Beta: item.Prerelease, packageName: "Network_Toolkit_" + version + "_Update.zip"}
	if len(result.Notes) > 16000 {
		result.Notes = result.Notes[:16000]
	}
	base := "https://github.com/" + updateRepository + "/releases/download/" + item.Tag + "/"
	for _, asset := range item.Assets {
		if asset.Name != result.packageName && asset.Name != "SHA256SUMS" {
			continue
		}
		if asset.URL != base+asset.Name || asset.Size <= 0 {
			return nil, errors.New("Release asset URL or size does not match the trusted update repository")
		}
		if asset.Name == result.packageName {
			if result.packageURL != "" || asset.Size > updateMaxPackage {
				return nil, errors.New("Invalid update package asset")
			}
			result.packageURL = asset.URL
		} else {
			if result.sumsURL != "" || asset.Size > 64<<10 {
				return nil, errors.New("Invalid checksum asset")
			}
			result.sumsURL = asset.URL
		}
	}
	if result.packageURL == "" || result.sumsURL == "" {
		return nil, errors.New("Release is missing its binary-only update package or SHA256SUMS")
	}
	return result, nil
}
// Beta opt-in queries the official release list, not the stable-only "latest"
 // endpoint. Only validated, published, non-draft releases are eligible.
 // The newest numeric version wins; a newer stable release can supersede a beta.
func updateParseReleaseFeed(data []byte, channel string) (*updateRelease, error) {
	if channel != "beta" {
		return updateParseRelease(data)
	}
	var entries []json.RawMessage
	if err := json.Unmarshal(data, &entries); err != nil || len(entries) == 0 || len(entries) > 30 {
		return nil, errors.New("GitHub returned an invalid beta release listing")
	}
	var best *updateRelease
	for _, raw := range entries {
		var meta struct {
			Tag string `json:"tag_name"`
			Draft bool `json:"draft"`
		}
		if err := json.Unmarshal(raw, &meta); err != nil {
			return nil, errors.New("GitHub returned malformed beta release metadata")
		}
		version := strings.TrimPrefix(meta.Tag, "v")
		if meta.Draft || meta.Tag != "v"+version || !updateVersionPattern.MatchString(version) {
			continue
		}
		candidate, err := updateParseReleaseCandidate(raw, true)
		if err != nil {
			// A candidate claiming an official version with invalid assets
			// is not silently trusted or substituted.
			return nil, err
		}
		if best == nil || updateNewer(candidate.Version, best.Version) {
			best = candidate
		}
	}
	if best == nil {
		return nil, errors.New("No compatible stable or beta release is available")
	}
	return best, nil
}
func (a *application) handleUpdateFeedLocked(w http.ResponseWriter, r *http.Request) bool {
	if r.URL.Path != "/api/updates/check" && r.URL.Path != "/api/updates/download" {
		return false
	}
	if a.updater.networkBusy {
		jsonError(w, 409, "An update check or download is already running.")
		return true
	}
	a.updater.networkBusy = true
	defer func() { a.updater.networkBusy = false }()
	_ = http.NewResponseController(w).SetWriteDeadline(time.Now().Add(6 * time.Minute))
	timeout := 5 * time.Minute
	if r.URL.Path == "/api/updates/check" {
		timeout = 20 * time.Second
	}
	ctx, cancel := context.WithTimeout(r.Context(), timeout)
	defer cancel()
	client := updateHTTPClient()
	if r.URL.Path == "/api/updates/check" {
		var request struct { Channel string `json:"channel"` }
		if err := decodeBody(w, r, 1024, &request); err != nil {
			jsonError(w, 400, err.Error())
			return true
		}
		channel := request.Channel
		if channel == "" {
			channel = "stable"
		}
		if channel != "stable" && channel != "beta" {
			jsonError(w, 400, "Choose the Stable or Beta update channel.")
			return true
		}
		latest, e := a.updateReleaseUnlocked(ctx, client, channel)
		if e != nil {
			status := http.StatusBadGateway
			if errors.Is(e, errUpdateFeedCooldown) {
				status = http.StatusTooManyRequests
			}
			jsonResponse(w, status, map[string]any{"error": e.Error(), "checkAfter": a.updater.feed.checkAfterMillis()})
			return true
		}
		a.updater.latest = latest
		a.updater.channel = channel
		a.updater.checked = true
		jsonResponse(w, 200, a.updateStatusLocked())
		return true
	}
	var request struct {
		Version string `json:"version"`
		Channel string `json:"channel"`
	}
	if e := decodeBody(w, r, 4096, &request); e != nil {
		jsonError(w, 400, e.Error())
		return true
	}
	latest := a.updater.latest
	channel := request.Channel
	if channel == "" {
		channel = "stable"
	}
	checkedChannel := a.updater.channel
	if checkedChannel == "" {
		checkedChannel = "stable"
	}
	if channel != checkedChannel || latest == nil || request.Version != latest.Version || !updateNewer(latest.Version, model.Version) {
		jsonError(w, 409, "Check for a newer update before downloading.")
		return true
	}
	a.updater.progress = &updateProgress{Phase: "checksums", Version: latest.Version}
	finished := false
	defer func() {
		if !finished {
			a.updater.progress.Phase = "failed"
		}
	}()
	sums, e := a.updateFetchUnlocked(ctx, client, latest.sumsURL, 64<<10)
	if e != nil {
		jsonError(w, 502, e.Error())
		return true
	}
	a.updater.progress = &updateProgress{Phase: "download", Version: latest.Version}
	archive, e := a.updateDownloadUnlocked(ctx, client, latest.packageURL, updateMaxPackage, latest.Version)
	if e != nil {
		jsonError(w, 502, e.Error())
		return true
	}
	a.updater.progress.Phase = "verifying"
	a.updater.mu.Unlock()
	version, binary, e := updateExtractPackage(latest.packageName, archive, sums, model.Version)
	a.updater.mu.Lock()
	if e != nil {
		jsonError(w, 400, e.Error())
		return true
	}
	a.updater.progress.Phase = "staging"
	if e = a.updateStageBinary(version, binary); e != nil {
		jsonError(w, 409, e.Error())
		return true
	}
	if a.updater.staged != nil {
		a.updater.staged.Beta = latest.Beta
	}
	finished = true
	a.updater.progress.Phase = "ready"
	jsonResponse(w, 200, a.updateStatusLocked())
	return true
}

// Caller holds updater.mu and has reserved networkBusy. Release the mutex for
// the network round trip, then publish cache/cooldown state under the same lock.
func (a *application) updateReleaseUnlocked(ctx context.Context, client *http.Client, channel string) (*updateRelease, error) {
	cache := a.updater.feed
	a.updater.mu.Unlock()
	defer func() {
		a.updater.mu.Lock()
		a.updater.feed = cache
	}()
	latest, cache, err := updateFetchReleaseChannel(ctx, client, cache, channel)
	return latest, err
}

// The updater's network state is reserved, but normal diagnostics and status
// remain responsive while GitHub responds. Caller holds updater.mu.
func (a *application) updateFetchUnlocked(ctx context.Context, client *http.Client, raw string, limit int64) ([]byte, error) {
	a.updater.mu.Unlock()
	defer a.updater.mu.Lock()
	return updateFetch(ctx, client, raw, limit)
}

// Only the reserved download writes progress. Status reads remain responsive
// while bytes arrive; no timer or progress read starts an update action.
func (a *application) updateDownloadUnlocked(ctx context.Context, client *http.Client, raw string, limit int64, version string) ([]byte, error) {
	a.updater.mu.Unlock()
	defer a.updater.mu.Lock()
	return updateFetchProgress(ctx, client, raw, limit, func(received, total int64) {
		a.updater.mu.Lock()
		defer a.updater.mu.Unlock()
		if p := a.updater.progress; p != nil && p.Phase == "download" && p.Version == version {
			p.Received = received
			p.Total = total
		}
	})
}
