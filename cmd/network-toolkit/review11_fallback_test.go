package main

import (
	"context"
	"errors"
	"networktoolkit/internal/model"
	"strings"
	"testing"
)

// Security review 11 (from the reviewer's failing test): the running version
// has no release and the channel's latest is OLDER. No older release may be
// offered (a downgrade could save over newer settings); the lookup says that
// no installer was found for this version instead.
func TestReviewFallbackOffersOlderRelease(t *testing.T) {
	f := newFullInstallFixture(t)
	older := "1.0.1"
	api := strings.TrimPrefix(updateAPIURL, "https://api.github.com")
	f.releases[api] = fullInstallReleaseJSON(older, false, map[string]int{fullInstallSetupName(older): 10, "SHA256SUMS": 100})
	found, err := fullInstallFind(context.Background(), updateHTTPClient(), model.Version, "stable")
	if err == nil {
		t.Logf("running %s, offered %s fallback=%v newer=%v", model.Version, found.Version, found.Fallback, updateNewer(found.Version, model.Version))
		if found.Version == older || !updateNewer(found.Version, model.Version) {
			t.Fatalf("DEFECT: offered %s while running %s", found.Version, model.Version)
		}
		return
	}
	if !errors.Is(err, errFullInstallNoNewer) || !strings.Contains(err.Error(), "No installer was found for version "+model.Version) {
		t.Fatalf("the refusal should say plainly that no installer was found: %v", err)
	}
	// The same through the page's route: nothing is offered.
	a := f.app()
	code, out := f.call(a, "/full-install/find", map[string]any{"channel": "stable"})
	if code == 200 || a.fullInstall.found != nil {
		t.Fatalf("the route offered a release: %d %v", code, out)
	}
}

// A newer release is offered as a fallback, marked newer, with both versions.
func TestReviewFallbackNewerIsMarked(t *testing.T) {
	f := newFullInstallFixture(t)
	newer := "99.0.0"
	api := strings.TrimPrefix(updateAPIURL, "https://api.github.com")
	f.releases[api] = fullInstallReleaseJSON(newer, false, map[string]int{fullInstallSetupName(newer): 10, "SHA256SUMS": 100})
	found, err := fullInstallFind(context.Background(), updateHTTPClient(), model.Version, "stable")
	if err != nil || !found.Fallback || !found.Newer || found.Version != newer || found.Running != model.Version {
		t.Fatalf("%+v %v", found, err)
	}
}
