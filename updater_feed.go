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
	"strings"
	"time"
)

const updateRepository = "TechSavage1989/network-toolkit"
const updateAPIURL = "https://api.github.com/repos/" + updateRepository + "/releases/latest"

type updateRelease struct {
	Version     string `json:"version"`
	Notes       string `json:"notes"`
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
func updateFetch(ctx context.Context, client *http.Client, raw string, limit int64) ([]byte, error) {
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
	b, e := io.ReadAll(io.LimitReader(response.Body, limit+1))
	if e != nil {
		return nil, e
	}
	if int64(len(b)) > limit {
		return nil, errors.New("Update response exceeds its size limit")
	}
	return b, nil
}
func updateParseRelease(data []byte) (*updateRelease, error) {
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
	if item.Tag != "v"+version || !updateVersionPattern.MatchString(version) || item.Draft || item.Prerelease {
		return nil, errors.New("Update feed did not provide a stable version")
	}
	result := &updateRelease{Version: version, Notes: item.Body, packageName: "Network_Toolkit_" + version + "_Update.zip"}
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
func (a *application) handleUpdateFeedLocked(w http.ResponseWriter, r *http.Request) bool {
	if r.URL.Path != "/api/updates/check" && r.URL.Path != "/api/updates/download" {
		return false
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
		data, e := a.updateFetchUnlocked(ctx, client, updateAPIURL, 2<<20)
		if e != nil {
			jsonError(w, 502, e.Error())
			return true
		}
		latest, e := updateParseRelease(data)
		if e != nil {
			jsonError(w, 502, e.Error())
			return true
		}
		a.updater.latest = latest
		a.updater.checked = true
		jsonResponse(w, 200, a.updateStatusLocked())
		return true
	}
	var request struct {
		Version string `json:"version"`
	}
	if e := decodeBody(w, r, 4096, &request); e != nil {
		jsonError(w, 400, e.Error())
		return true
	}
	latest := a.updater.latest
	if latest == nil || request.Version != latest.Version || !updateNewer(latest.Version, model.Version) {
		jsonError(w, 409, "Check for a newer update before downloading.")
		return true
	}
	sums, e := a.updateFetchUnlocked(ctx, client, latest.sumsURL, 64<<10)
	if e != nil {
		jsonError(w, 502, e.Error())
		return true
	}
	archive, e := a.updateFetchUnlocked(ctx, client, latest.packageURL, updateMaxPackage)
	if e != nil {
		jsonError(w, 502, e.Error())
		return true
	}
	version, binary, e := updateExtractPackage(latest.packageName, archive, sums, model.Version)
	if e != nil {
		jsonError(w, 400, e.Error())
		return true
	}
	if e = a.updateStageBinary(version, binary); e != nil {
		jsonError(w, 409, e.Error())
		return true
	}
	jsonResponse(w, 200, a.updateStatusLocked())
	return true
}

// The updater's network state is reserved, but normal diagnostics and status
// remain responsive while GitHub responds. Caller holds updater.mu.
func (a *application) updateFetchUnlocked(ctx context.Context, client *http.Client, raw string, limit int64) ([]byte, error) {
	a.updater.mu.Unlock()
	defer a.updater.mu.Lock()
	return updateFetch(ctx, client, raw, limit)
}
