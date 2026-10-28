package main

// Explorer drops carry browser-granted File bytes, never a page-supplied source
// path. Existing ServeHTTP session/origin/host checks protect this endpoint.
import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"
)

const maxExplorerDropBytes int64 = 512 << 20

type explorerDropDestination struct {
	Protocol  string `json:"protocol"`
	Root      string `json:"root"`
	Path      string `json:"path"`
	Name      string `json:"name"`
	Overwrite bool   `json:"overwrite"`
}

func validExplorerName(name string) bool {
	if name == "" || name == "." || name == ".." || len(name) > 255 || !utf8.ValidString(name) || strings.ContainsAny(name, "/\\<>:\"|?*") || strings.TrimRight(name, " .") != name {
		return false
	}
	for _, c := range name {
		if c < 32 || c == 127 {
			return false
		}
	}
	base := strings.ToUpper(strings.SplitN(name, ".", 2)[0])
	if base == "CON" || base == "PRN" || base == "AUX" || base == "NUL" || ((strings.HasPrefix(base, "COM") || strings.HasPrefix(base, "LPT")) && len([]rune(base)) == 4 && strings.ContainsRune("123456789¹²³", []rune(base)[3])) {
		return false
	}
	return true
}
func validateExplorerDropName(name string) error {
	if !validExplorerName(name) {
		return errors.New("choose a single valid filename without reserved names, trailing dots/spaces or path separators")
	}
	switch strings.ToLower(filepath.Ext(name)) {
	case ".lnk", ".url":
		return errors.New("shortcut/link drops are not supported; choose regular files")
	}
	return nil
}

// Stage beside the target through an os.Root handle. Root operations confine
// every lookup even if another process changes a path during upload. Link is
// an atomic create-if-absent publication; Rename is only used after explicit
// overwrite. No original is truncated while the bytes are arriving.
func writeExplorerDrop(ctx context.Context, root, sub, name string, overwrite bool, source io.Reader, beforeCommit func() (func(), error)) (int64, error) {
	if err := validateExplorerDropName(name); err != nil {
		return 0, err
	}
	if root == "" || !filepath.IsAbs(root) {
		return 0, errors.New("choose a configured shared-server root")
	}
	for _, segment := range strings.Split(strings.ReplaceAll(sub, "\\", "/"), "/") {
		if segment != "" && segment != "." && !validExplorerName(segment) {
			return 0, errors.New("invalid destination folder segment")
		}
	}
	if _, err := transferPath(root, sub, true); err != nil {
		return 0, err
	}
	rootInfo, err := os.Lstat(root)
	if err != nil {
		return 0, err
	}
	if !rootInfo.IsDir() || rootInfo.Mode()&(os.ModeSymlink|os.ModeIrregular) != 0 {
		return 0, errors.New("shared root is no longer a regular directory")
	}
	confined, err := os.OpenRoot(root)
	if err != nil {
		return 0, err
	}
	defer confined.Close()
	openedInfo, err := confined.Stat(".")
	if err != nil || !os.SameFile(rootInfo, openedInfo) {
		return 0, errors.New("shared root changed while opening destination")
	}
	directory := filepath.Clean(filepath.FromSlash(sub))
	dir, err := confined.OpenRoot(directory)
	if err != nil {
		return 0, fmt.Errorf("open destination folder: %w", err)
	}
	defer dir.Close()
	directoryInfo, err := dir.Stat(".")
	if err != nil {
		return 0, err
	}
	if info, e := dir.Lstat(name); e == nil {
		if !info.Mode().IsRegular() {
			return 0, errors.New("destination is not a regular file")
		}
		if !overwrite {
			return 0, errors.New("destination already exists; choose overwrite, skip or rename")
		}
	} else if !os.IsNotExist(e) {
		return 0, e
	}
	stage := ".ntk-drop-" + randomTransferID() + ".tmp"
	output, err := dir.OpenFile(stage, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return 0, fmt.Errorf("create destination staging file (check permissions): %w", err)
	}
	defer dir.Remove(stage)
	total, err := io.Copy(output, io.LimitReader(&explorerContextReader{ctx: ctx, reader: source}, maxExplorerDropBytes+1))
	if err == nil && total > maxExplorerDropBytes {
		err = errors.New("file exceeds the 512 MiB drop limit")
	}
	if err == nil {
		err = output.Sync()
	}
	closeErr := output.Close()
	if err == nil {
		err = closeErr
	}
	if err != nil {
		return total, err
	}
	if err = ctx.Err(); err != nil {
		return total, err
	}
	unlock, err := beforeCommit()
	if err != nil {
		return total, err
	}
	defer unlock()
	if err = ctx.Err(); err != nil {
		return total, err
	}
	now, err := os.Lstat(root)
	if err != nil || !now.IsDir() || now.Mode()&(os.ModeSymlink|os.ModeIrregular) != 0 || !os.SameFile(rootInfo, now) {
		return total, errors.New("shared root changed before publication")
	}
	if _, err := transferPath(root, sub, true); err != nil {
		return total, err
	}
	currentDirectory, err := confined.Stat(directory)
	if err != nil || !os.SameFile(directoryInfo, currentDirectory) {
		return total, errors.New("destination folder changed during upload; no file was published")
	}
	// Check again under the manager's root lock. A swap to a link or special file
	// is never an allowed overwrite. os.Root keeps both operations confined.
	if info, e := dir.Lstat(name); e == nil {
		if !info.Mode().IsRegular() {
			return total, errors.New("destination changed to an unsupported file type")
		}
		if !overwrite {
			return total, errors.New("destination appeared during upload; choose overwrite, skip or rename")
		}
	} else if !os.IsNotExist(e) {
		return total, e
	}
	if overwrite {
		err = dir.Rename(stage, name)
	} else {
		err = dir.Link(stage, name)
	}
	if err != nil {
		return total, fmt.Errorf("publish copied file: %w", err)
	}
	if !overwrite {
		if err := dir.Remove(stage); err != nil {
			return total, fmt.Errorf("file copied, but local staging cleanup failed for %s: %w", stage, err)
		}
	}
	return total, nil
}

type explorerContextReader struct {
	ctx    context.Context
	reader io.Reader
}

func (r *explorerContextReader) Read(b []byte) (int, error) {
	if e := r.ctx.Err(); e != nil {
		return 0, e
	}
	return r.reader.Read(b)
}

func (a *application) handleTransferDropUpload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodError(w)
		return
	}
	if a.transfers == nil {
		jsonError(w, 503, "Shared-file manager is unavailable")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxExplorerDropBytes+(64<<10))
	reader, err := r.MultipartReader()
	if err != nil {
		jsonError(w, 400, "Expected one browser-selected file and destination")
		return
	}
	// Destination metadata must precede file bytes. This permits validation before
	// staging, bounded parsing and cancellation without a whole-file memory copy.
	part, err := reader.NextPart()
	if err != nil || part.FormName() != "destination" || part.FileName() != "" {
		jsonError(w, 400, "Missing upload destination")
		return
	}
	var destination explorerDropDestination
	decoder := json.NewDecoder(io.LimitReader(part, 16<<10))
	decoder.DisallowUnknownFields()
	if err = decoder.Decode(&destination); err != nil || !transferProtocol(destination.Protocol) {
		jsonError(w, 400, "Invalid shared-file destination")
		return
	}
	part.Close()
	if destination.Root != a.transfers.root(destination.Protocol) {
		jsonError(w, 409, "Shared root changed. Drop the files again into the intended folder.")
		return
	}
	part, err = reader.NextPart()
	if err != nil || part.FormName() != "file" || part.FileName() == "" {
		jsonError(w, 400, "Choose one regular file")
		return
	}
	defer part.Close()
	if err = validateExplorerDropName(part.FileName()); err != nil {
		jsonError(w, 400, err.Error())
		return
	}
	// Reject trailing parts before publication, including accidental folder batches.
	input := &explorerSinglePartReader{part: part, reader: reader}
	total, err := writeExplorerDrop(r.Context(), destination.Root, destination.Path, destination.Name, destination.Overwrite, input, func() (func(), error) {
		a.transfers.mu.Lock()
		if a.transfers.roots[destination.Protocol] != destination.Root {
			a.transfers.mu.Unlock()
			return nil, errors.New("shared root changed during upload; nothing was published")
		}
		return a.transfers.mu.Unlock, nil
	})
	if err != nil {
		jsonError(w, 400, "Copy did not complete cleanly: "+err.Error())
		return
	}
	jsonResponse(w, 200, map[string]any{"ok": true, "bytes": total, "name": destination.Name, "message": "File copied; original source unchanged"})
}

// Enforce one-file requests before the atomic publication point.
type explorerSinglePartReader struct {
	part     *multipart.Part
	reader   *multipart.Reader
	finished bool
}

func (r *explorerSinglePartReader) Read(b []byte) (int, error) {
	n, err := r.part.Read(b)
	if err == io.EOF && !r.finished {
		r.finished = true
		if _, next := r.reader.NextPart(); next != io.EOF {
			return n, errors.New("unexpected additional item; folder/batch payload rejected")
		}
	}
	return n, err
}
