package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"networktoolkit/internal/model"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

// A local test server stands in for the release feed and its downloads: a
// synthetic Setup file and its SHA256SUMS. Requests keep their official URLs
// (the updater's host allowlist applies) and are answered by the server.
type fullInstallFixture struct {
	t        *testing.T
	mu       sync.Mutex
	hosts    []string
	paths    []string
	releases map[string]string // API path -> JSON, absent = 404
	files    map[string][]byte // download path -> bytes
	launched []string
	dir      string
	exe      string
}

func fullInstallReleaseJSON(version string, prerelease bool, assets map[string]int) string {
	prefix := "https://github.com/" + updateRepository + "/releases/download/v" + version + "/"
	var list []map[string]any
	for name, size := range assets {
		list = append(list, map[string]any{"name": name, "browser_download_url": prefix + name, "size": size})
	}
	b, _ := json.Marshal(map[string]any{"tag_name": "v" + version, "prerelease": prerelease, "assets": list})
	return string(b)
}

func newFullInstallFixture(t *testing.T) *fullInstallFixture {
	f := &fullInstallFixture{t: t, releases: map[string]string{}, files: map[string][]byte{}, dir: t.TempDir()}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		release, isRelease := f.releases[r.URL.Path]
		file, isFile := f.files[r.URL.Path]
		f.mu.Unlock()
		switch {
		case isRelease:
			_, _ = w.Write([]byte(release))
		case isFile:
			w.Header().Set("Content-Length", fmt.Sprint(len(file)))
			_, _ = w.Write(file)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)
	forward := &http.Transport{}
	t.Cleanup(forward.CloseIdleConnections)
	updatePollTransport(t, updateRoundTrip(func(r *http.Request) (*http.Response, error) {
		f.mu.Lock()
		f.hosts = append(f.hosts, r.URL.Host)
		f.paths = append(f.paths, r.URL.Path)
		f.mu.Unlock()
		local := r.Clone(r.Context())
		local.URL.Scheme, local.URL.Host, local.Host = "http", strings.TrimPrefix(server.URL, "http://"), ""
		return forward.RoundTrip(local)
	}))
	program := filepath.Join(f.dir, "Portable Copy")
	if err := os.MkdirAll(program, 0700); err != nil {
		t.Fatal(err)
	}
	f.exe = filepath.Join(program, "Network_Toolkit.exe")
	oldExe, oldPlatform, oldLaunch, oldAny := windowsIntegrationExecutable, windowsIntegrationPlatform, fullInstallLaunch, fullInstallAnyPlatform
	t.Cleanup(func() {
		windowsIntegrationExecutable, windowsIntegrationPlatform, fullInstallLaunch, fullInstallAnyPlatform = oldExe, oldPlatform, oldLaunch, oldAny
	})
	windowsIntegrationExecutable = func() (string, error) { return f.exe, nil }
	platform, _ := testEditorPlatform(newFakeRegistry())
	windowsIntegrationPlatform = func() (editorRegPlatform, bool) { return platform, true }
	fullInstallAnyPlatform = true
	fullInstallLaunch = func(_ context.Context, path string) error {
		f.mu.Lock()
		defer f.mu.Unlock()
		f.launched = append(f.launched, path)
		return nil
	}
	return f
}

// Publish a release with Setup and SHA256SUMS; sums replaces the checksum file.
func (f *fullInstallFixture) publish(version string, setup []byte, sums string) {
	name := fullInstallSetupName(version)
	if sums == "" {
		sums = updateHash(setup) + "  " + name + "\n" + strings.Repeat("0", 64) + "  Network_Toolkit_" + version + "_Update.zip\n"
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.releases["/repos/"+updateRepository+"/releases/tags/v"+version] = fullInstallReleaseJSON(version, true, map[string]int{name: len(setup), "SHA256SUMS": len(sums)})
	f.files["/"+updateRepository+"/releases/download/v"+version+"/"+name] = setup
	f.files["/"+updateRepository+"/releases/download/v"+version+"/SHA256SUMS"] = []byte(sums)
}

func (f *fullInstallFixture) app() *application {
	return &application{windowMode: "main", dataPath: filepath.Join(f.dir, "data", "Network_Toolkit.settings.json")}
}

func (f *fullInstallFixture) call(a *application, action string, body any) (int, map[string]any) {
	f.t.Helper()
	if err := os.MkdirAll(filepath.Dir(a.dataPath), 0700); err != nil {
		f.t.Fatal(err)
	}
	b, _ := json.Marshal(body)
	w := httptest.NewRecorder()
	a.handleWindowsIntegration(w, httptest.NewRequest(http.MethodPost, windowsIntegrationPath+action, strings.NewReader(string(b))))
	var out map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &out)
	return w.Code, out
}

// Setup files and folders anywhere under the data folder.
func (f *fullInstallFixture) setupFiles() []string {
	var found []string
	_ = filepath.Walk(filepath.Join(f.dir, "data"), func(path string, info os.FileInfo, err error) error {
		if err == nil && (strings.HasPrefix(info.Name(), ".network-toolkit-setup-") || strings.HasSuffix(info.Name(), "_Setup.exe")) {
			found = append(found, path)
		}
		return nil
	})
	return found
}

func synthSetup(fill string) []byte {
	return append([]byte("MZ synthetic Setup fixture "), []byte(strings.Repeat(fill, 4096))...)
}

func TestFullInstallFindsVerifiesAndStartsOnlyAfterConfirmation(t *testing.T) {
	f := newFullInstallFixture(t)
	setup := synthSetup("s")
	f.publish(model.Version, setup, "")
	a := f.app()
	if code, out := f.call(a, "/full-install/download", map[string]any{"version": model.Version, "confirm": true}); code != http.StatusConflict {
		t.Fatalf("download before find: %d %v", code, out)
	}
	code, out := f.call(a, "/full-install/find", map[string]any{"channel": "stable"})
	if code != http.StatusOK {
		t.Fatalf("find: %d %v", code, out)
	}
	found := out["install"].(map[string]any)["found"].(map[string]any)
	if found["version"] != model.Version || found["fallback"] != false || found["name"] != fullInstallSetupName(model.Version) || int(found["size"].(float64)) != len(setup) {
		t.Fatalf("found %v", found)
	}
	if len(f.setupFiles()) != 0 {
		t.Fatal("finding Setup downloaded something")
	}
	// The download needs the explicit confirmation of this exact version.
	for _, body := range []map[string]any{{"version": model.Version}, {"version": "0.0.1", "confirm": true}} {
		if code, _ := f.call(a, "/full-install/download", body); code != http.StatusConflict {
			t.Fatalf("download %v: %d", body, code)
		}
	}
	code, out = f.call(a, "/full-install/download", map[string]any{"version": model.Version, "confirm": true})
	if code != http.StatusOK {
		t.Fatalf("download: %d %v", code, out)
	}
	download := out["install"].(map[string]any)["download"].(map[string]any)
	if download["sha256"] != updateHash(setup) || int(download["size"].(float64)) != len(setup) {
		t.Fatalf("download %v", download)
	}
	if progress := out["install"].(map[string]any)["progress"].(map[string]any); progress["phase"] != "ready" {
		t.Fatalf("progress %v", progress)
	}
	files := f.setupFiles()
	if len(files) != 2 {
		t.Fatalf("expected one folder with one Setup file, found %v", files)
	}
	saved, err := os.ReadFile(a.fullInstall.download.path)
	if err != nil || string(saved) != string(setup) {
		t.Fatalf("saved Setup differs: %v", err)
	}
	// Setup starts only after the second confirmation, of this version.
	for _, body := range []map[string]any{{"version": model.Version}, {"version": "0.0.1", "confirm": true}} {
		if code, _ := f.call(a, "/full-install/start", body); code != http.StatusConflict || len(f.launched) != 0 {
			t.Fatalf("start %v: %d launched=%v", body, code, f.launched)
		}
	}
	code, out = f.call(a, "/full-install/start", map[string]any{"version": model.Version, "confirm": true})
	if code != http.StatusOK || len(f.launched) != 1 || f.launched[0] != a.fullInstall.download.path || out["install"].(map[string]any)["started"] != true {
		t.Fatalf("start: %d %v launched=%v", code, out, f.launched)
	}
	// Only the updater's hosts were contacted: the release API and downloads.
	for _, host := range f.hosts {
		if host != "api.github.com" && host != "github.com" {
			t.Fatalf("request to %s", host)
		}
	}
	if !strings.Contains(strings.Join(f.paths, " "), "/releases/tags/v"+model.Version) {
		t.Fatalf("the running version's release was not asked for: %v", f.paths)
	}
	// A started Setup's file stays until a later run sweeps it; discard removes it.
	if code, _ := f.call(a, "/full-install/discard", map[string]any{}); code != http.StatusOK || len(f.setupFiles()) != 0 {
		t.Fatalf("discard left %v", f.setupFiles())
	}
}

func TestFullInstallChecksumMismatchKeepsNothing(t *testing.T) {
	for name, sums := range map[string]func(setup []byte) string{
		"wrong digest": func(setup []byte) string {
			return strings.Repeat("a", 64) + "  " + fullInstallSetupName(model.Version) + "\n"
		},
		"missing entry": func(setup []byte) string {
			return updateHash(setup) + "  Network_Toolkit_" + model.Version + "_Update.zip\n"
		},
		"duplicate": func(setup []byte) string {
			line := updateHash(setup) + "  " + fullInstallSetupName(model.Version) + "\n"
			return line + line
		},
	} {
		t.Run(name, func(t *testing.T) {
			f := newFullInstallFixture(t)
			setup := synthSetup("m")
			f.publish(model.Version, setup, sums(setup))
			a := f.app()
			if code, _ := f.call(a, "/full-install/find", map[string]any{}); code != http.StatusOK {
				t.Fatal("find failed")
			}
			code, out := f.call(a, "/full-install/download", map[string]any{"version": model.Version, "confirm": true})
			if code != http.StatusBadGateway || !strings.Contains(fmt.Sprint(out["error"]), "Checksum") && !strings.Contains(fmt.Sprint(out["error"]), "checksum") {
				t.Fatalf("mismatch: %d %v", code, out)
			}
			if files := f.setupFiles(); len(files) != 0 || a.fullInstall.download != nil {
				t.Fatalf("an unverified download was kept: %v", files)
			}
			if code, _ := f.call(a, "/full-install/start", map[string]any{"version": model.Version, "confirm": true}); code != http.StatusConflict || len(f.launched) != 0 {
				t.Fatal("Setup started without a verified download")
			}
		})
	}
}

func TestFullInstallRefusesAFileChangedAfterVerification(t *testing.T) {
	f := newFullInstallFixture(t)
	f.publish(model.Version, synthSetup("v"), "")
	a := f.app()
	f.call(a, "/full-install/find", map[string]any{})
	if code, _ := f.call(a, "/full-install/download", map[string]any{"version": model.Version, "confirm": true}); code != http.StatusOK {
		t.Fatal("download failed")
	}
	if err := os.WriteFile(a.fullInstall.download.path, synthSetup("x"), 0700); err != nil {
		t.Fatal(err)
	}
	code, out := f.call(a, "/full-install/start", map[string]any{"version": model.Version, "confirm": true})
	if code != http.StatusConflict || len(f.launched) != 0 || !strings.Contains(fmt.Sprint(out["error"]), "changed") {
		t.Fatalf("changed file: %d %v launched=%v", code, out, f.launched)
	}
	if a.fullInstall.download != nil {
		t.Fatal("the changed download is still offered")
	}
}

func TestFullInstallFallsBackOnlyWhenThisVersionHasNoSetup(t *testing.T) {
	latest := "9.9.9"
	setup := synthSetup("l")
	sums := updateHash(setup) + "  " + fullInstallSetupName(latest) + "\n"
	for name, prepare := range map[string]func(f *fullInstallFixture){
		"no release for this version": func(f *fullInstallFixture) {},
		"release without Setup": func(f *fullInstallFixture) {
			f.releases["/repos/"+updateRepository+"/releases/tags/v"+model.Version] = fullInstallReleaseJSON(model.Version, true, map[string]int{"SHA256SUMS": 10, "Network_Toolkit_" + model.Version + "_Update.zip": 10})
		},
	} {
		t.Run(name, func(t *testing.T) {
			f := newFullInstallFixture(t)
			prepare(f)
			f.releases["/repos/"+updateRepository+"/releases/latest"] = fullInstallReleaseJSON(latest, false, map[string]int{fullInstallSetupName(latest): len(setup), "SHA256SUMS": len(sums)})
			f.files["/"+updateRepository+"/releases/download/v"+latest+"/"+fullInstallSetupName(latest)] = setup
			f.files["/"+updateRepository+"/releases/download/v"+latest+"/SHA256SUMS"] = []byte(sums)
			a := f.app()
			code, out := f.call(a, "/full-install/find", map[string]any{"channel": "stable"})
			if code != http.StatusOK {
				t.Fatalf("find: %d %v", code, out)
			}
			found := out["install"].(map[string]any)["found"].(map[string]any)
			if found["version"] != latest || found["fallback"] != true {
				t.Fatalf("fallback %v", found)
			}
			if code, _ := f.call(a, "/full-install/download", map[string]any{"version": latest, "confirm": true}); code != http.StatusOK {
				t.Fatal("the offered fallback could not be downloaded")
			}
		})
	}
	t.Run("nothing at all", func(t *testing.T) {
		f := newFullInstallFixture(t)
		a := f.app()
		code, out := f.call(a, "/full-install/find", map[string]any{"channel": "stable"})
		if code != http.StatusBadGateway || a.fullInstall.found != nil {
			t.Fatalf("%d %v", code, out)
		}
	})
}

func TestFullInstallParseRejectsUntrustedAssets(t *testing.T) {
	good := fullInstallReleaseJSON("1.2.3", false, map[string]int{fullInstallSetupName("1.2.3"): 100, "SHA256SUMS": 100})
	if r, err := fullInstallParseRelease([]byte(good), false); err != nil || r.Version != "1.2.3" || r.Size != 100 {
		t.Fatal(r, err)
	}
	for _, bad := range []string{
		strings.ReplaceAll(good, "github.com/TechSavage1989", "evil.example/TechSavage1989"),
		strings.ReplaceAll(good, `"size":100`, `"size":0`),
		strings.ReplaceAll(good, `"tag_name":`, `"draft":true,"tag_name":`),
		strings.ReplaceAll(good, `"prerelease":false`, `"prerelease":true`),
		strings.ReplaceAll(good, `"size":100`, fmt.Sprintf(`"size":%d`, fullInstallMaxSetup+1)),
	} {
		if _, err := fullInstallParseRelease([]byte(bad), false); err == nil {
			t.Fatalf("accepted %s", bad)
		}
	}
}

func TestFullInstallSharesTheUpdatersNetworkSlot(t *testing.T) {
	f := newFullInstallFixture(t)
	f.publish(model.Version, synthSetup("n"), "")
	a := f.app()
	a.updater.networkBusy = true
	code, out := f.call(a, "/full-install/find", map[string]any{})
	if code == http.StatusOK || !strings.Contains(fmt.Sprint(out["error"]), "update check or download is running") || len(f.hosts) != 0 {
		t.Fatalf("find during an update download: %d %v", code, out)
	}
	a.updater.networkBusy = false
	if code, _ := f.call(a, "/full-install/find", map[string]any{}); code != http.StatusOK || a.updater.networkBusy {
		t.Fatal("the slot was not released")
	}
}

func TestFullInstallTellsWhetherTheInstalledProgramFindsTheData(t *testing.T) {
	base := t.TempDir()
	defaultDir := filepath.Join(base, "NetworkToolkit")
	for name, c := range map[string]struct {
		config appStorageConfig
		same   bool
		says   string
	}{
		"default":  {appStorageConfig{RootDir: defaultDir, DefaultDir: defaultDir}, true, "uses the same data folder"},
		"chosen":   {appStorageConfig{RootDir: filepath.Join(base, "Chosen"), DefaultDir: defaultDir}, true, "where the installed program reads it too"},
		"explicit": {appStorageConfig{RootDir: filepath.Join(base, "Explicit"), DefaultDir: defaultDir, Explicit: true}, false, "will not see this copy's settings"},
	} {
		a := &application{storage: newAppStorage(c.config)}
		note := a.fullInstallDataNote(filepath.Join(base, "Network_Toolkit.exe"))
		if note == nil || note.Same != c.same || !strings.Contains(note.Note, c.says) {
			t.Errorf("%s: %+v", name, note)
		}
	}
}

func TestWindowsIntegrationOnlyInTheMainWindow(t *testing.T) {
	for _, mode := range []string{standaloneEditorMode, "terminal", "detached", "editor"} {
		a := &application{windowMode: mode}
		w := httptest.NewRecorder()
		a.handleWindowsIntegration(w, httptest.NewRequest(http.MethodPost, windowsIntegrationPath+"/text-editor/register", strings.NewReader(`{}`)))
		if w.Code != http.StatusNotFound {
			t.Fatalf("%s window serves the route: %d", mode, w.Code)
		}
	}
	a := &application{windowMode: "main"}
	w := httptest.NewRecorder()
	a.handleWindowsIntegration(w, httptest.NewRequest(http.MethodGet, windowsIntegrationPath+"/text-editor/register", nil))
	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("a GET may never change anything: %d", w.Code)
	}
}
