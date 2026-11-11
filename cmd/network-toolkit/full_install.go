package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"networktoolkit/internal/model"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"
)

// "Install the full version" (App settings > Updates, portable program on
// Windows). It finds Setup of the running version on the official release
// feed, downloads it only after the user agreed, verifies it against that
// release's SHA256SUMS with the updater's own check (updateVerifySum), and
// starts it, interactively, only after a second confirmation. All network
// access goes through the updater's download path (updateFetch,
// updateFetchProgress, updateHTTPClient: HTTPS to the updater's hosts only)
// and reserves the updater's network slot, so an update and this download
// never run at the same time. Nothing partial or unverified is ever written:
// the file is verified in memory first and checked again before it starts.

const (
	fullInstallMaxSetup = updateMaxEXE // updateRemoveVerified reads up to this size
	fullInstallDirGlob  = ".network-toolkit-setup-*"
)

var (
	errFullInstallNoAsset = errors.New("This release has no Windows Setup and SHA256SUMS.")
	fullInstallLaunch     = fullInstallStartSetup
	fullInstallOpen       = fullInstallStartSetup
)

func fullInstallSetupName(version string) string {
	return "Network_Toolkit_" + version + "_Windows_x64_Setup.exe"
}

func fullInstallTagURL(version string) string {
	return "https://api.github.com/repos/" + updateRepository + "/releases/tags/v" + version
}

type fullInstallRelease struct {
	Version  string `json:"version"`
	Name     string `json:"name"`
	Size     int64  `json:"size"`
	Beta     bool   `json:"beta"`
	Fallback bool   `json:"fallback"`
	url      string
	sumsURL  string
}

type fullInstallDownload struct {
	Version string `json:"version"`
	Name    string `json:"name"`
	Size    int64  `json:"size"`
	SHA256  string `json:"sha256"`
	Folder  string `json:"folder"`
	path    string
	dir     string
}

type fullInstallData struct {
	RootDir   string `json:"rootDir"`
	Explicit  bool   `json:"explicit"`
	Custom    bool   `json:"custom"`
	Same      bool   `json:"same"`
	Installed string `json:"installedRootDir,omitempty"`
	Note      string `json:"note"`
}

type fullInstallStatus struct {
	Supported       bool                 `json:"supported"`
	Portable        bool                 `json:"portable"`
	Version         string               `json:"version"`
	Installed       bool                 `json:"installed"`
	InstallLocation string               `json:"installLocation,omitempty"`
	InstalledExists bool                 `json:"installedExists"`
	Data            *fullInstallData     `json:"data,omitempty"`
	Found           *fullInstallRelease  `json:"found,omitempty"`
	Download        *fullInstallDownload `json:"download,omitempty"`
	Progress        *updateProgress      `json:"progress,omitempty"`
	Busy            bool                 `json:"busy"`
	Started         bool                 `json:"started"`
}

type fullInstallState struct {
	mu       sync.Mutex
	busy     bool
	found    *fullInstallRelease
	download *fullInstallDownload
	progress *updateProgress
	started  bool
}

func (a *application) fullInstallStatus(editor editorRegistrationStatus) fullInstallStatus {
	status := fullInstallStatus{Supported: runtime.GOOS == "windows" && runtime.GOARCH == "amd64" && a.windowMode == "main", Version: model.Version}
	status.Portable = !editor.ThisIsInstalled
	status.Installed, status.InstallLocation = editor.Installed, editor.InstallLocation
	if editor.Installed {
		status.InstalledExists = !editorRegFileMissing(filepath.Join(editor.InstallLocation, editorRegAppFile))
	}
	status.Data = a.fullInstallDataNote(editor.Executable)
	a.fullInstall.mu.Lock()
	defer a.fullInstall.mu.Unlock()
	status.Found, status.Download, status.Busy, status.Started = a.fullInstall.found, a.fullInstall.download, a.fullInstall.busy, a.fullInstall.started
	if a.fullInstall.progress != nil {
		progress := *a.fullInstall.progress
		status.Progress = &progress
	}
	return status
}

// Whether the installed program will find this copy's data (storage_location.go):
// both read the data folder choice from the default folder, so only a run with
// an explicit --data-dir uses a folder the installed program does not know.
func (a *application) fullInstallDataNote(exe string) *fullInstallData {
	if a.storage == nil {
		return nil
	}
	current := a.storage.status()
	data := &fullInstallData{RootDir: current.RootDir, Explicit: current.Explicit, Custom: !storagePathsEqual(current.RootDir, current.DefaultDir)}
	switch {
	case current.Explicit:
		installed := current.DefaultDir
		if config, err := resolveAppStorage(filepath.Dir(exe), ""); err == nil {
			installed = config.RootDir
		}
		data.Installed = installed
		data.Same = storagePathsEqual(installed, current.RootDir)
		if data.Same {
			data.Note = "This copy was started with --data-dir, which names the same folder the installed program uses: " + current.RootDir + "."
		} else {
			data.Note = "This copy was started with --data-dir and uses " + current.RootDir + ". The installed program's shortcuts do not pass that switch, so it will use " + installed + " and will not see this copy's settings and saved data. After installing, open App settings > General in the installed program and choose " + current.RootDir + " as the data folder (clear Copy current saved settings, because the folder already holds your data), then restart it. Or start the installed program with --data-dir \"" + current.RootDir + "\"."
		}
	case data.Custom:
		data.Same = true
		data.Note = "This copy uses the data folder you chose, " + current.RootDir + ". That choice is saved in " + current.DefaultDir + ", where the installed program reads it too, so it uses the same settings and saved data."
	default:
		data.Same = true
		data.Note = "The installed program uses the same data folder, " + current.RootDir + ", with the same settings and saved data."
	}
	return data
}

func storagePathsEqual(a, b string) bool {
	return a != "" && storagePathKey(a) == storagePathKey(b)
}

// fullInstallParseRelease reads one release of the official repository and
// returns its Setup and SHA256SUMS assets, validated like the updater's
// package: the tag is v<version>, the release is published, and each asset URL
// is the repository's own download URL with a plausible size.
func fullInstallParseRelease(data []byte, acceptBeta bool) (*fullInstallRelease, error) {
	var item struct {
		Tag        string `json:"tag_name"`
		Draft      bool   `json:"draft"`
		Prerelease bool   `json:"prerelease"`
		Assets     []struct {
			Name string `json:"name"`
			URL  string `json:"browser_download_url"`
			Size int64  `json:"size"`
		} `json:"assets"`
	}
	if err := json.Unmarshal(data, &item); err != nil {
		return nil, errors.New("GitHub returned invalid release metadata")
	}
	version := strings.TrimPrefix(item.Tag, "v")
	if item.Tag != "v"+version || !updateVersionPattern.MatchString(version) || item.Draft || (item.Prerelease && !acceptBeta) {
		return nil, errors.New("The release feed did not provide a published version")
	}
	result := &fullInstallRelease{Version: version, Name: fullInstallSetupName(version), Beta: item.Prerelease}
	base := "https://github.com/" + updateRepository + "/releases/download/" + item.Tag + "/"
	for _, asset := range item.Assets {
		if asset.Name != result.Name && asset.Name != "SHA256SUMS" {
			continue
		}
		if asset.URL != base+asset.Name || asset.Size <= 0 {
			return nil, errors.New("Release asset URL or size does not match the trusted update repository")
		}
		if asset.Name == result.Name {
			if result.url != "" || asset.Size > fullInstallMaxSetup {
				return nil, errors.New("Invalid Setup asset")
			}
			result.url, result.Size = asset.URL, asset.Size
		} else {
			if result.sumsURL != "" || asset.Size > 64<<10 {
				return nil, errors.New("Invalid checksum asset")
			}
			result.sumsURL = asset.URL
		}
	}
	if result.url == "" || result.sumsURL == "" {
		return nil, errFullInstallNoAsset
	}
	return result, nil
}

// fullInstallFind looks for Setup of the running version first. Only when that
// release or its Setup is missing does it offer the latest release of the
// update channel, marked as a fallback so the page says so.
func fullInstallFind(ctx context.Context, client *http.Client, version, channel string) (*fullInstallRelease, error) {
	data, err := updateFetch(ctx, client, fullInstallTagURL(version), updateFeedMaxBody)
	if err == nil {
		release, parseErr := fullInstallParseRelease(data, true)
		if parseErr == nil && release.Version == version {
			return release, nil
		}
		if parseErr != nil && !errors.Is(parseErr, errFullInstallNoAsset) {
			return nil, parseErr
		}
	} else if !errors.Is(err, errUpdateNotFound) {
		return nil, err
	}
	endpoint := updateAPIURL
	if channel == "beta" {
		endpoint = updateBetaAPIURL
	}
	data, err = updateFetch(ctx, client, endpoint, updateFeedMaxBody)
	if err != nil {
		return nil, err
	}
	var best *fullInstallRelease
	if channel == "beta" {
		var entries []json.RawMessage
		if err := json.Unmarshal(data, &entries); err != nil || len(entries) > 30 {
			return nil, errors.New("GitHub returned an invalid release listing")
		}
		for _, raw := range entries {
			candidate, err := fullInstallParseRelease(raw, true)
			if err != nil {
				continue
			}
			if best == nil || updateNewer(candidate.Version, best.Version) {
				best = candidate
			}
		}
	} else if best, err = fullInstallParseRelease(data, false); err != nil {
		best = nil
	}
	if best == nil {
		return nil, errors.New("Setup for version " + version + " is not on the release page, and no other release with Setup was found.")
	}
	best.Fallback = true
	return best, nil
}

// Reserve the updater's network slot for one round trip; an update check or
// download in progress refuses this action and the other way round.
func (a *application) fullInstallReserveNetwork() error {
	a.updater.mu.Lock()
	defer a.updater.mu.Unlock()
	if a.updater.installing {
		return errors.New("An update is installing.")
	}
	if a.updater.networkBusy {
		return errors.New("An update check or download is running. Try again when it has finished.")
	}
	a.updater.networkBusy = true
	return nil
}

func (a *application) fullInstallReleaseNetwork() {
	a.updater.mu.Lock()
	a.updater.networkBusy = false
	a.updater.mu.Unlock()
}

func (a *application) handleFullInstall(w http.ResponseWriter, r *http.Request, platform editorRegPlatform, exe string) {
	action := strings.TrimPrefix(r.URL.Path, windowsIntegrationPath+"/full-install/")
	var request struct {
		Version string `json:"version"`
		Channel string `json:"channel"`
		Confirm bool   `json:"confirm"`
	}
	if err := decodeBody(w, r, 1024, &request); err != nil {
		jsonError(w, http.StatusBadRequest, err.Error())
		return
	}
	if runtime.GOOS != "windows" || runtime.GOARCH != "amd64" {
		if !fullInstallAnyPlatform {
			jsonError(w, http.StatusNotImplemented, "Setup runs on Windows x64.")
			return
		}
	}
	a.fullInstall.mu.Lock()
	if a.fullInstall.busy {
		a.fullInstall.mu.Unlock()
		jsonError(w, http.StatusConflict, "Setup is already being looked up or downloaded.")
		return
	}
	switch action {
	case "find":
		channel := request.Channel
		if channel == "" {
			channel = "stable"
		}
		if channel != "stable" && channel != "beta" {
			a.fullInstall.mu.Unlock()
			jsonError(w, http.StatusBadRequest, "Choose the Stable or Beta update channel.")
			return
		}
		a.fullInstall.busy = true
		a.fullInstall.mu.Unlock()
		found, err := a.fullInstallFindLocked(r.Context(), channel)
		a.fullInstall.mu.Lock()
		a.fullInstall.busy = false
		if err == nil {
			a.fullInstall.found = found
		}
		a.fullInstall.mu.Unlock()
		if err != nil {
			jsonError(w, http.StatusBadGateway, err.Error())
			return
		}
	case "download":
		found := a.fullInstall.found
		if !request.Confirm || found == nil || request.Version != found.Version {
			a.fullInstall.mu.Unlock()
			jsonError(w, http.StatusConflict, "Look up Setup again and confirm the download.")
			return
		}
		a.fullInstall.busy = true
		a.fullInstall.progress = &updateProgress{Phase: "checksums", Version: found.Version}
		a.fullInstall.mu.Unlock()
		_ = http.NewResponseController(w).SetWriteDeadline(time.Now().Add(6 * time.Minute))
		download, err := a.fullInstallDownload(r.Context(), found)
		a.fullInstall.mu.Lock()
		a.fullInstall.busy = false
		if err != nil {
			a.fullInstall.progress = nil
			a.fullInstall.mu.Unlock()
			jsonError(w, http.StatusBadGateway, err.Error())
			return
		}
		if old := a.fullInstall.download; old != nil {
			fullInstallDiscard(old)
		}
		a.fullInstall.download, a.fullInstall.started = download, false
		a.fullInstall.progress = &updateProgress{Phase: "ready", Version: found.Version, Received: download.Size, Total: download.Size}
		a.fullInstall.mu.Unlock()
	case "start":
		download := a.fullInstall.download
		if !request.Confirm || download == nil || request.Version != download.Version {
			a.fullInstall.mu.Unlock()
			jsonError(w, http.StatusConflict, "Download and verify Setup, then confirm that it should start.")
			return
		}
		// The file is read once more right before it starts: a file that
		// changed on disk since it was verified is deleted, never started.
		b, err := updateNamingReadFile(download.path, fullInstallMaxSetup)
		if err != nil || updateHash(b) != download.SHA256 {
			fullInstallDiscard(download)
			a.fullInstall.download, a.fullInstall.progress = nil, nil
			a.fullInstall.mu.Unlock()
			jsonError(w, http.StatusConflict, "The downloaded Setup changed after it was verified, so it was deleted. Download it again.")
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
		err = fullInstallLaunch(ctx, download.path)
		cancel()
		if err != nil {
			a.fullInstall.mu.Unlock()
			jsonError(w, http.StatusServiceUnavailable, "Setup could not be started: "+err.Error())
			return
		}
		a.fullInstall.started = true
		a.fullInstall.mu.Unlock()
	case "discard":
		if download := a.fullInstall.download; download != nil {
			fullInstallDiscard(download)
		}
		a.fullInstall.download, a.fullInstall.progress, a.fullInstall.started = nil, nil, false
		a.fullInstall.mu.Unlock()
	default:
		a.fullInstall.mu.Unlock()
		jsonError(w, http.StatusNotFound, "Unknown install action.")
		return
	}
	editor, err := editorRegState(platform.registry, exe, platform.missing)
	if err != nil {
		editor = editorRegistrationStatus{Executable: exe}
	}
	jsonResponse(w, http.StatusOK, map[string]any{"install": a.fullInstallStatus(editor)})
}

// Tests run the Windows-only actions on any platform.
var fullInstallAnyPlatform = false

func (a *application) fullInstallFindLocked(ctx context.Context, channel string) (*fullInstallRelease, error) {
	if err := a.fullInstallReserveNetwork(); err != nil {
		return nil, err
	}
	defer a.fullInstallReleaseNetwork()
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	return fullInstallFind(ctx, updateHTTPClient(), model.Version, channel)
}

func (a *application) fullInstallDownload(ctx context.Context, found *fullInstallRelease) (*fullInstallDownload, error) {
	if err := a.fullInstallReserveNetwork(); err != nil {
		return nil, err
	}
	defer a.fullInstallReleaseNetwork()
	ctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()
	client := updateHTTPClient()
	sums, err := updateFetch(ctx, client, found.sumsURL, 64<<10)
	if err != nil {
		return nil, err
	}
	a.setFullInstallProgress(&updateProgress{Phase: "download", Version: found.Version, Total: found.Size})
	setup, err := updateFetchProgress(ctx, client, found.url, fullInstallMaxSetup, func(received, total int64) {
		a.setFullInstallProgress(&updateProgress{Phase: "download", Version: found.Version, Received: received, Total: total})
	})
	if err != nil {
		return nil, err
	}
	a.setFullInstallProgress(&updateProgress{Phase: "verifying", Version: found.Version, Received: int64(len(setup)), Total: int64(len(setup))})
	// The updater's own verification: the SHA256SUMS line for this exact
	// asset name must match. A mismatch leaves nothing on disk.
	if err := updateVerifySum(sums, found.Name, setup); err != nil {
		return nil, err
	}
	if !bytes.HasPrefix(setup, []byte("MZ")) {
		return nil, errors.New("The downloaded file is not a Windows program.")
	}
	root, err := a.fullInstallFolder()
	if err != nil {
		return nil, err
	}
	fullInstallSweep(root)
	dir, err := os.MkdirTemp(root, strings.TrimSuffix(fullInstallDirGlob, "*"))
	if err != nil {
		return nil, errors.New("Setup could not be saved in the app data folder: " + err.Error())
	}
	path := filepath.Join(dir, found.Name)
	if err := updateWriteExclusive(path, setup, 0700); err != nil {
		_ = os.Remove(path)
		_ = os.Remove(dir)
		return nil, errors.New("Setup could not be saved in the app data folder: " + err.Error())
	}
	return &fullInstallDownload{Version: found.Version, Name: found.Name, Size: int64(len(setup)), SHA256: updateHash(setup), Folder: dir, path: path, dir: dir}, nil
}

func (a *application) setFullInstallProgress(progress *updateProgress) {
	a.fullInstall.mu.Lock()
	a.fullInstall.progress = progress
	a.fullInstall.mu.Unlock()
}

// The download lives beside the updater's staged files, in the data folder.
func (a *application) fullInstallFolder() (string, error) {
	exe, err := windowsIntegrationExecutable()
	if err != nil {
		return "", err
	}
	return updateStorageRoot(filepath.Dir(a.dataPath), filepath.Dir(exe))
}

// A verified download that was never started, or one whose Setup has since
// finished, is removed; a file that changed is left alone, like the updater's
// staged files, and so is a folder that holds anything else.
func fullInstallDiscard(download *fullInstallDownload) {
	if download == nil {
		return
	}
	if err := updateRemoveVerified(download.path, download.SHA256); err == nil {
		_ = os.Remove(download.dir)
	}
}

// Folders of earlier runs: their Setup has finished (or never started).
// Only a folder holding nothing but one official Setup file is removed.
func fullInstallSweep(root string) {
	dirs, _ := filepath.Glob(filepath.Join(root, fullInstallDirGlob))
	for _, dir := range dirs {
		entries, err := os.ReadDir(dir)
		if err != nil || len(entries) > 1 {
			continue
		}
		if len(entries) == 1 {
			name := entries[0].Name()
			if !strings.HasPrefix(name, "Network_Toolkit_") || !strings.HasSuffix(name, "_Windows_x64_Setup.exe") || !entries[0].Type().IsRegular() {
				continue
			}
			if os.Remove(filepath.Join(dir, name)) != nil {
				continue // Setup is still running from it.
			}
		}
		_ = os.Remove(dir)
	}
}

// At exit: a download that was never started is removed. A started Setup may
// still be running from its file; the next download sweeps it.
func (a *application) cleanupFullInstall() {
	a.fullInstall.mu.Lock()
	defer a.fullInstall.mu.Unlock()
	if download := a.fullInstall.download; download != nil && !a.fullInstall.started {
		fullInstallDiscard(download)
		a.fullInstall.download = nil
	}
}
