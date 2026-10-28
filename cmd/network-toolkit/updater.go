package main

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"debug/pe"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"networktoolkit/internal/model"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf16"
)

const updateMaxPackage = 128 << 20
const updateMaxEXE = 96 << 20
const updateReleasesURL = "https://github.com/TechSavage1989/network-toolkit/releases/latest"

var updateVersionPattern = regexp.MustCompile(`^(0|[1-9][0-9]{0,4})\.(0|[1-9][0-9]{0,4})\.(0|[1-9][0-9]{0,4})$`)

type stagedUpdate struct {
	Version string `json:"version"`
	SHA256  string `json:"sha256"`
	Size    int    `json:"size"`
	dir     string
}
type appUpdater struct {
	mu          sync.Mutex
	requestGate sync.RWMutex
	staged      *stagedUpdate
	installing  bool
	latest      *updateRelease
	checked     bool
	networkBusy bool
	progress    *updateProgress
	feed        updateFeedCache
}

func updateNewer(candidate, current string) bool {
	if !updateVersionPattern.MatchString(candidate) || !updateVersionPattern.MatchString(current) {
		return false
	}
	a, b := strings.Split(candidate, "."), strings.Split(current, ".")
	for i := range a {
		x, _ := strconv.Atoi(a[i])
		y, _ := strconv.Atoi(b[i])
		if x != y {
			return x > y
		}
	}
	return false
}
func updateHash(b []byte) string { h := sha256.Sum256(b); return hex.EncodeToString(h[:]) }
func updateVerifySum(manifest []byte, name string, content []byte) error {
	if len(manifest) > 64<<10 {
		return errors.New("Checksum file is too large.")
	}
	found := ""
	for _, line := range strings.Split(string(manifest), "\n") {
		line = strings.TrimSuffix(line, "\r")
		if line == "" {
			continue
		}
		if len(line) < 67 || line[64:66] != "  " {
			return errors.New("Invalid SHA256SUMS format.")
		}
		digest, err := hex.DecodeString(line[:64])
		if err != nil || len(digest) != 32 {
			return errors.New("Invalid SHA256 checksum.")
		}
		if line[66:] == name {
			if found != "" {
				return errors.New("Duplicate checksum entry.")
			}
			found = strings.ToLower(line[:64])
		}
	}
	if found == "" || found != updateHash(content) {
		return errors.New("Checksum mismatch or missing entry. Download the package and SHA256SUMS from the same official release.")
	}
	return nil
}
func updateReadZipFile(f *zip.File, limit uint64) ([]byte, error) {
	if f.UncompressedSize64 > limit {
		return nil, errors.New("Update member exceeds its size limit.")
	}
	r, e := f.Open()
	if e != nil {
		return nil, e
	}
	defer r.Close()
	b, e := io.ReadAll(io.LimitReader(r, int64(limit)+1))
	if e != nil {
		return nil, e
	}
	if uint64(len(b)) > limit {
		return nil, errors.New("Update member exceeds its size limit.")
	}
	return b, nil
}
func updateUTF16(s string) []byte {
	u := utf16.Encode([]rune(s + "\x00"))
	b := make([]byte, len(u)*2)
	for i, v := range u {
		binary.LittleEndian.PutUint16(b[i*2:], v)
	}
	return b
}
func updateValidatePE(b []byte, version string) error {
	f, e := pe.NewFile(bytes.NewReader(b))
	if e != nil {
		return errors.New("Update executable is not a valid Windows application.")
	}
	defer f.Close()
	if f.Machine != pe.IMAGE_FILE_MACHINE_AMD64 {
		return errors.New("Update must be Windows x64.")
	}
	h, ok := f.OptionalHeader.(*pe.OptionalHeader64)
	if !ok || h.Subsystem != 2 {
		return errors.New("Update must be a Windows x64 GUI application.")
	}
	section := f.Section(".rsrc")
	if section == nil {
		return errors.New("Update has no product version resource.")
	}
	resource, e := section.Data()
	if e != nil {
		return e
	}
	if !bytes.Contains(resource, updateUTF16("Network Toolkit")) || !bytes.Contains(resource, updateUTF16(version)) || !bytes.Contains(resource, updateUTF16("NetworkToolkit")) {
		return errors.New("Update product/version does not match its package.")
	}
	return nil
}
func updateExtractPackage(filename string, archive, checksums []byte, current string) (string, []byte, error) {
	if len(archive) > updateMaxPackage {
		return "", nil, errors.New("Update package exceeds 128 MiB.")
	}
	if err := updateVerifySum(checksums, filename, archive); err != nil {
		return "", nil, err
	}
	prefix := "Network_Toolkit_"
	suffix := "_Portable.zip"
	if strings.HasSuffix(filename, "_Update.zip") {
		suffix = "_Update.zip"
	}
	if !strings.HasPrefix(filename, prefix) || !strings.HasSuffix(filename, suffix) {
		return "", nil, errors.New("Choose the official Network_Toolkit_VERSION_Portable.zip release asset.")
	}
	version := strings.TrimSuffix(strings.TrimPrefix(filename, prefix), suffix)
	if !updateNewer(version, current) {
		return "", nil, errors.New("Choose a release newer than the installed version. Downgrades and same-version replacements are not allowed.")
	}
	z, e := zip.NewReader(bytes.NewReader(archive), int64(len(archive)))
	if e != nil {
		return "", nil, e
	}
	if len(z.File) > 3000 {
		return "", nil, errors.New("Update contains too many files.")
	}
	root := prefix + version + "/"
	exeName := prefix + version + "_Windows_x64.exe"
	var executable, manifest []byte
	seen := map[string]bool{}
	for _, f := range z.File {
		name := f.Name
		if name != path.Clean(name) || strings.ContainsAny(name, "\\:\x00") || !strings.HasPrefix(name, root) || f.Mode()&os.ModeSymlink != 0 || seen[strings.ToLower(name)] {
			return "", nil, errors.New("Update contains an unsafe or duplicate archive path.")
		}
		seen[strings.ToLower(name)] = true
		switch name {
		case root + exeName:
			executable, e = updateReadZipFile(f, updateMaxEXE)
		case root + "SHA256SUMS":
			manifest, e = updateReadZipFile(f, 64<<10)
		}
		if e != nil {
			return "", nil, e
		}
	}
	if len(executable) == 0 || len(manifest) == 0 {
		return "", nil, errors.New("Update is missing its executable or internal checksums.")
	}
	if e = updateVerifySum(manifest, exeName, executable); e != nil {
		return "", nil, e
	}
	if e = updateValidatePE(executable, version); e != nil {
		return "", nil, e
	}
	return version, executable, nil
}
func (a *application) updateBusy() []string {
	busy := []string{}
	a.mu.RLock()
	for _, j := range a.jobs {
		if j.Status == "running" || j.Status == "queued" {
			busy = append(busy, "Diagnostic jobs")
			break
		}
	}
	a.mu.RUnlock()
	if a.terminals != nil {
		for _, s := range a.terminals.List() {
			if s.State != "closed" && s.State != "error" && s.State != "disconnected" {
				busy = append(busy, "Terminal sessions")
				break
			}
		}
	}
	if a.packetCaptures != nil {
		for _, s := range a.packetCaptures.list() {
			if captureActive(s.State) {
				busy = append(busy, "Packet capture")
				break
			}
		}
	}
	if a.transfers != nil {
		a.transfers.mu.Lock()
		if len(a.transfers.servers) > 0 {
			busy = append(busy, "Transfer servers")
		}
		a.transfers.mu.Unlock()
	}
	if a.localTools != nil {
		a.localTools.mu.Lock()
		if a.localTools.busy || a.localTools.wifiBusy {
			busy = append(busy, "Local configuration change")
		}
		a.localTools.mu.Unlock()
	}
	a.browserMu.Lock()
	if len(a.browserConnections) > 0 || a.browserConnecting > 0 {
		busy = append(busy, "Remote file connections")
	}
	a.browserMu.Unlock()
	if a.editorViews != nil && a.editorViews.hasLiveViews() {
		busy = append(busy, "Detached editor documents or window transfers")
	}
	return busy
}
func (a *application) updateStatusLocked() map[string]any {
	return map[string]any{"currentVersion": model.Version, "executable": updateNamingStatus(), "restartedAfterUpdate": a.updateHealthDir != "", "finalizing": a.updateFinalizing.Load(), "supported": runtime.GOOS == "windows" && runtime.GOARCH == "amd64" && a.windowMode == "main", "mode": "direct", "latest": a.updater.latest, "checked": a.updater.checked, "checkAfter": a.updater.feed.checkAfterMillis(), "hasUpdate": a.updater.latest != nil && updateNewer(a.updater.latest.Version, model.Version), "releasesUrl": updateReleasesURL, "staged": a.updater.staged, "installing": a.updater.installing, "networkBusy": a.updater.networkBusy, "progress": a.updater.progress, "busy": a.updateBusy()}
}
func (a *application) handleUpdates(w http.ResponseWriter, r *http.Request) {
	a.updater.mu.Lock()
	defer a.updater.mu.Unlock()
	if r.URL.Path == "/api/updates/status" && r.Method == http.MethodGet {
		jsonResponse(w, 200, a.updateStatusLocked())
		return
	}
	if r.Method != http.MethodPost {
		methodError(w)
		return
	}
	if runtime.GOOS != "windows" || runtime.GOARCH != "amd64" || a.windowMode != "main" {
		jsonError(w, 409, "Install updates from the main Windows x64 application window.")
		return
	}
	if a.updater.networkBusy {
		jsonError(w, 409, "An update check or download is already running.")
		return
	}
	if a.updater.installing {
		jsonError(w, 409, "An update is already installing.")
		return
	}
	if a.handleUpdateFeedLocked(w, r) {
		return
	}
	switch r.URL.Path {
	case "/api/updates/open-releases":
		if err := openUpdateReleases(r.Context()); err != nil {
			jsonError(w, 500, "Could not open the release page: "+err.Error())
			return
		}
		jsonResponse(w, 200, map[string]bool{"ok": true})
	case "/api/updates/import":
		r.Body = http.MaxBytesReader(w, r.Body, updateMaxPackage+(128<<10))
		if e := r.ParseMultipartForm(1 << 20); e != nil {
			jsonError(w, 400, "Unable to read update files: "+e.Error())
			return
		}
		defer r.MultipartForm.RemoveAll()
		if len(r.MultipartForm.File["package"]) != 1 || len(r.MultipartForm.File["checksums"]) != 1 {
			jsonError(w, 400, "Choose one portable ZIP and one SHA256SUMS file.")
			return
		}
		f, h, e := r.FormFile("package")
		if e != nil {
			jsonError(w, 400, e.Error())
			return
		}
		defer f.Close()
		archive, e := io.ReadAll(io.LimitReader(f, updateMaxPackage+1))
		if e != nil {
			jsonError(w, 400, e.Error())
			return
		}
		c, _, e := r.FormFile("checksums")
		if e != nil {
			jsonError(w, 400, e.Error())
			return
		}
		defer c.Close()
		sums, e := io.ReadAll(io.LimitReader(c, (64<<10)+1))
		if e != nil {
			jsonError(w, 400, e.Error())
			return
		}
		version, binary, e := updateExtractPackage(h.Filename, archive, sums, model.Version)
		if e != nil {
			jsonError(w, 400, e.Error())
			return
		}
		if e = a.updateStageBinary(version, binary); e != nil {
			jsonError(w, 409, e.Error())
			return
		}
		jsonResponse(w, 200, a.updateStatusLocked())
	case "/api/updates/install":
		var request struct {
			Confirm bool   `json:"confirm"`
			Version string `json:"version"`
		}
		if e := decodeBody(w, r, 4096, &request); e != nil {
			jsonError(w, 400, e.Error())
			return
		}
		staged := a.updater.staged
		if !request.Confirm || staged == nil || request.Version != staged.Version {
			jsonError(w, 409, "Review the staged update and explicitly confirm its version before installing.")
			return
		}
		if busy := a.updateBusy(); len(busy) > 0 {
			jsonError(w, 409, "Finish or stop these activities before updating: "+strings.Join(busy, ", ")+".")
			return
		}
		if e := startUpdateInstaller(staged); e != nil {
			jsonError(w, 409, "Update could not start; the current app is unchanged. "+e.Error())
			return
		}
		a.updater.installing = true
		jsonResponse(w, 202, map[string]any{"ok": true, "restarting": true})
		go func() { time.Sleep(500 * time.Millisecond); a.stop() }()
	default:
		jsonError(w, 404, "Unknown update action.")
	}
}
func updateWriteExclusive(name string, b []byte, mode os.FileMode) error {
	f, e := os.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_EXCL, mode)
	if e != nil {
		return e
	}
	_, e = f.Write(b)
	if e == nil {
		e = f.Sync()
	}
	ce := f.Close()
	if e == nil {
		e = ce
	}
	return e
}
func updateFileHash(name string) (string, error) {
	b, e := os.ReadFile(name)
	if e != nil {
		return "", e
	}
	if len(b) > updateMaxEXE {
		return "", fmt.Errorf("Executable exceeds size limit")
	}
	return updateHash(b), nil
}

func (a *application) updateStageBinary(version string, binary []byte) error {
	exe, e := updateCurrentExecutable()
	if e != nil {
		return e
	}
	exe, e = filepath.EvalSymlinks(exe)
	if e != nil {
		return e
	}
	root, e := updateStorageRoot(filepath.Dir(a.dataPath), filepath.Dir(exe))
	if e != nil {
		return e
	}
	dir, e := os.MkdirTemp(root, ".network-toolkit-update-")
	if e != nil {
		return errors.New("The settings update folder is not writable. Check the app settings location and try again.")
	}
	if e = updateWriteExclusive(filepath.Join(dir, "candidate.exe"), binary, 0700); e != nil {
		updateDiscardStage(dir, updateHash(binary))
		return e
	}
	if old := a.updater.staged; old != nil {
		updateDiscardStage(old.dir, old.SHA256)
	}
	a.updater.staged = &stagedUpdate{Version: version, SHA256: updateHash(binary), Size: len(binary), dir: dir}
	return nil
}

// Uninstalled imports belong to this session. Successful installers retain their
// recovery backup and result log; never remove those from a running app.
func (a *application) cleanupStagedUpdate() {
	a.updater.mu.Lock()
	defer a.updater.mu.Unlock()
	if !a.updater.installing && a.updater.staged != nil {
		_ = updateDiscardStage(a.updater.staged.dir, a.updater.staged.SHA256)
		a.updater.staged = nil
	}
}

// A never-installed candidate belongs to this session. Unknown extra files
// prevent directory removal, rather than being recursively erased.
func updateDiscardStage(dir, digest string) error {
	if err := updateRemoveVerified(filepath.Join(dir, "candidate.exe"), digest); err != nil {
		return err
	}
	return os.Remove(dir)
}
