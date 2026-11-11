package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
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
//
// Where the file lives and how it starts (security review 11): never in the
// data folder, which the user may have put where others can write, but in
// %LOCALAPPDATA%\NetworkToolkit-Setup, each download in its own folder; both
// are created with a protected access list that lets only this user in
// (fullInstallMakePrivateDir) and must be plain folders nobody else can change
// (fullInstallCheckPrivateDir), checked again before Setup starts. Setup is
// opened once without write or delete sharing (fullInstallOpenHeld), hashed
// from that handle, and started with CreateProcess while the handle is still
// open, so the program that starts is the one that was verified.

const (
	fullInstallMaxSetup = updateMaxEXE // updateRemoveVerified reads up to this size
	fullInstallDirGlob  = ".network-toolkit-setup-*"
)

var (
	errFullInstallNoAsset = errors.New("This release has no Windows Setup and SHA256SUMS.")
	errFullInstallNoNewer = errors.New("no newer release with Setup")
	// fullInstallLaunch starts Setup; it runs while the verified file is held.
	fullInstallLaunch = fullInstallStartSetup
	fullInstallOpen   = fullInstallOpenProgram
	fullInstallRoot   = fullInstallDefaultRoot
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
	// Running is the version of this copy and Newer whether Version is newer
	// than it; a fallback is offered only when it is (never a downgrade).
	Running string `json:"running"`
	Newer   bool   `json:"newer"`
	url     string
	sumsURL string
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
	starting bool // Setup is being verified and started (the lock is not held for that)
}

func (a *application) fullInstallStatus(editor editorRegistrationStatus) fullInstallStatus {
	status := fullInstallStatus{Supported: runtime.GOOS == "windows" && runtime.GOARCH == "amd64" && a.windowMode == "main", Version: model.Version}
	status.Portable = !editor.ThisIsInstalled
	status.Installed, status.InstallLocation = editor.Installed, editor.InstallLocation
	if editor.Installed {
		status.InstalledExists = editorRegFileState(editorRegJoin(editor.InstallLocation, editorRegAppFile)) == editorRegPresent
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
			release.Running = version
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
	// Only a newer release may stand in: an older one would downgrade the
	// program, could save over settings a newer version wrote, and its Setup
	// would not take over this copy's Text Editor registration.
	if best == nil || !updateNewer(best.Version, version) {
		return nil, fmt.Errorf("No installer was found for version %s: its release has no Windows Setup, and no newer release offers one. %w", version, errFullInstallNoNewer)
	}
	best.Fallback, best.Running, best.Newer = true, version, true
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
	if a.fullInstall.busy || a.fullInstall.starting {
		a.fullInstall.mu.Unlock()
		jsonError(w, http.StatusConflict, "Setup is already being looked up, downloaded or started.")
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
		if !request.Confirm || found == nil || request.Version != found.Version || (found.Version != model.Version && !updateNewer(found.Version, model.Version)) {
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
		// Verifying and starting happen without the lock (status reads go on);
		// starting refuses a second start meanwhile.
		held := *download
		a.fullInstall.starting = true
		a.fullInstall.mu.Unlock()
		ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
		err := fullInstallLaunchVerified(ctx, &held)
		cancel()
		a.fullInstall.mu.Lock()
		a.fullInstall.starting = false
		if errors.Is(err, errFullInstallChanged) {
			// A file that changed since it was verified is deleted (if it
			// still has the verified bytes nothing is lost), never started.
			fullInstallDiscard(download)
			a.fullInstall.download, a.fullInstall.progress = nil, nil
			a.fullInstall.mu.Unlock()
			jsonError(w, http.StatusConflict, err.Error())
			return
		}
		if err != nil {
			a.fullInstall.mu.Unlock()
			jsonError(w, http.StatusServiceUnavailable, "Setup was not started: "+err.Error())
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
	editor, err := editorRegState(platform.registry, exe, platform.presence)
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
	root, err := fullInstallRoot()
	if err != nil {
		return nil, errors.New("Setup could not be saved in a folder only you can change: " + err.Error())
	}
	fullInstallSweep(root)
	dir, err := fullInstallNewDir(root)
	if err != nil {
		return nil, errors.New("Setup could not be saved in a folder only you can change: " + err.Error())
	}
	path := filepath.Join(dir, found.Name)
	if err := updateWriteExclusive(path, setup, 0700); err != nil {
		_ = os.Remove(path)
		_ = os.Remove(dir)
		return nil, errors.New("Setup could not be saved: " + err.Error())
	}
	return &fullInstallDownload{Version: found.Version, Name: found.Name, Size: int64(len(setup)), SHA256: updateHash(setup), Folder: dir, path: path, dir: dir}, nil
}

func (a *application) setFullInstallProgress(progress *updateProgress) {
	a.fullInstall.mu.Lock()
	a.fullInstall.progress = progress
	a.fullInstall.mu.Unlock()
}

var errFullInstallChanged = errors.New("The downloaded Setup changed after it was verified, so it was not started and was deleted. Download it again.")

// The folder for downloads: %LOCALAPPDATA%\NetworkToolkit-Setup on Windows,
// created owner-only and accepted only as a plain folder only this user can
// change. The data folder is never used: the user may have put it anywhere.
func fullInstallDefaultRoot() (string, error) {
	base, err := os.UserCacheDir()
	if err != nil || base == "" {
		return "", errors.New("the per-user application data folder is unavailable")
	}
	root := filepath.Join(base, "NetworkToolkit-Setup")
	if err := fullInstallMakePrivateDir(root); err != nil && !errors.Is(err, os.ErrExist) {
		return "", err
	}
	if err := fullInstallCheckPrivateDir(root); err != nil {
		return "", err
	}
	return root, nil
}

// One owner-only folder per download, with a name nobody can guess.
func fullInstallNewDir(root string) (string, error) {
	for attempt := 0; attempt < 16; attempt++ {
		dir := filepath.Join(root, strings.TrimSuffix(fullInstallDirGlob, "*")+randomHex(12))
		err := fullInstallMakePrivateDir(dir)
		if errors.Is(err, os.ErrExist) {
			continue
		}
		if err != nil {
			return "", err
		}
		if err := fullInstallCheckPrivateDir(dir); err != nil {
			_ = os.Remove(dir)
			return "", err
		}
		return dir, nil
	}
	return "", errors.New("no new folder name was free")
}

// fullInstallLaunchVerified checks the folders again, opens Setup once
// without write or delete sharing, hashes it from that handle and starts it
// while the handle is open. Any failed check refuses to start it.
func fullInstallLaunchVerified(ctx context.Context, download *fullInstallDownload) error {
	root, err := fullInstallRoot()
	if err != nil {
		return err
	}
	if !strings.EqualFold(filepath.Dir(download.dir), root) || !strings.EqualFold(filepath.Dir(download.path), download.dir) {
		return errors.New("the downloaded Setup is not where it was saved")
	}
	if err := fullInstallCheckPrivateDir(download.dir); err != nil {
		return err
	}
	held, err := fullInstallOpenHeld(download.path)
	if err != nil {
		return err
	}
	defer held.Close()
	digest := sha256.New()
	n, err := io.Copy(digest, io.LimitReader(held, fullInstallMaxSetup+1))
	if err != nil {
		return err
	}
	if n != download.Size || n > fullInstallMaxSetup || hex.EncodeToString(digest.Sum(nil)) != download.SHA256 {
		return errFullInstallChanged
	}
	return fullInstallLaunch(ctx, download.path)
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
