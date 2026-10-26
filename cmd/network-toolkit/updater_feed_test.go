package main

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
)

func feedFixture(version string) []byte {
	prefix := "https://github.com/" + updateRepository + "/releases/download/v" + version + "/"
	b, _ := json.Marshal(map[string]any{"tag_name": "v" + version, "body": "Release notes", "assets": []map[string]any{{"name": "Network_Toolkit_" + version + "_Update.zip", "browser_download_url": prefix + "Network_Toolkit_" + version + "_Update.zip", "size": 100}, {"name": "SHA256SUMS", "browser_download_url": prefix + "SHA256SUMS", "size": 100}}})
	return b
}
func TestUpdateReleaseMetadata(t *testing.T) {
	r, e := updateParseRelease(feedFixture("1.17.59"))
	if e != nil || r.Version != "1.17.59" {
		t.Fatal(r, e)
	}
	for _, change := range []func(string) string{
		func(s string) string {
			return strings.ReplaceAll(s, "github.com/TechSavage1989", "evil.example/TechSavage1989")
		},
		func(s string) string { return strings.ReplaceAll(s, "_Update.zip", "_Portable.zip") },
		func(s string) string { return strings.ReplaceAll(s, `"tag_name":`, `"draft":true,"tag_name":`) },
		func(s string) string { return strings.ReplaceAll(s, `"tag_name":`, `"prerelease":true,"tag_name":`) },
		func(s string) string { return strings.ReplaceAll(s, `"size":100`, `"size":0`) },
	} {
		if _, e := updateParseRelease([]byte(change(string(feedFixture("1.17.59"))))); e == nil {
			t.Fatal("unsafe metadata accepted")
		}
	}
}
func TestUpdateSafeURLs(t *testing.T) {
	for _, u := range []string{"http://github.com/a", "https://github.com.evil.test/a", "https://user@github.com/a", "https://github.com:444/a", "https://evil.example/a"} {
		if updateSafeURL(u) {
			t.Fatal(u)
		}
	}
	if !updateSafeURL("https://release-assets.githubusercontent.com/a") {
		t.Fatal("CDN refused")
	}
}

type updateRoundTrip func(*http.Request) (*http.Response, error)

func (f updateRoundTrip) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func TestUpdateFetchErrors(t *testing.T) {
	for _, status := range []int{404, 403, 429, 500} {
		c := &http.Client{Transport: updateRoundTrip(func(r *http.Request) (*http.Response, error) {
			return &http.Response{StatusCode: status, Body: io.NopCloser(strings.NewReader(""))}, nil
		})}
		if _, e := updateFetch(context.Background(), c, updateAPIURL, 100); e == nil {
			t.Fatal(status)
		}
	}
}
func TestUpdateFetchLimit(t *testing.T) {
	c := &http.Client{Transport: updateRoundTrip(func(r *http.Request) (*http.Response, error) {
		if r.Header.Get("Authorization") != "" {
			t.Fatal("unexpected credentials")
		}
		return &http.Response{StatusCode: 200, ContentLength: -1, Body: io.NopCloser(strings.NewReader("12345"))}, nil
	})}
	if _, e := updateFetch(context.Background(), c, updateAPIURL, 4); e == nil {
		t.Fatal("unbounded body")
	}
}

func TestUpdateFeedUsesMainRepositoryOnly(t *testing.T) {
	if updateRepository != "TechSavage1989/network-toolkit" || updateAPIURL != "https://api.github.com/repos/TechSavage1989/network-toolkit/releases/latest" || updateReleasesURL != "https://github.com/TechSavage1989/network-toolkit/releases/latest" {
		t.Fatal("updater must use the main repository exclusively")
	}
	old := strings.ReplaceAll(string(feedFixture("1.17.65")), "network-toolkit/", "network-toolkit-releases/")
	if _, err := updateParseRelease([]byte(old)); err == nil {
		t.Fatal("retired feed assets accepted")
	}
}

func TestUpdateFetchRequestsFreshMetadata(t *testing.T) {
	c := &http.Client{Transport: updateRoundTrip(func(r *http.Request) (*http.Response, error) {
		if r.Header.Get("Cache-Control") != "no-cache" {
			t.Fatal("release checks must request revalidation")
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader("ok"))}, nil
	})}
	if _, err := updateFetch(context.Background(), c, updateAPIURL, 100); err != nil {
		t.Fatal(err)
	}
}
