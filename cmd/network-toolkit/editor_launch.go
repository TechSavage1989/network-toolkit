package main

import (
	"context"
	"errors"
	"flag"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"
)

const (
	editorLaunchEndpoint  = "/api/editor-launch"
	standaloneEditorMode  = "editor-standalone"
	standaloneEditorTitle = appName + " Text Editor"
)

type editorLaunchRequest struct {
	Path string `json:"path,omitempty"`
}

type editorLaunchOptions struct {
	show       *bool
	standalone *bool
	file       *string
}

func registerEditorLaunchOptions(flags *flag.FlagSet) editorLaunchOptions {
	return editorLaunchOptions{
		show:       flags.Bool("editor", false, "Open the Text Editor in the main workspace"),
		standalone: flags.Bool(standaloneEditorMode, false, "Open the independent Text Editor window"),
		file:       flags.String("open-file", "", "Open a local file in Text Editor"),
	}
}

func (o editorLaunchOptions) request(flags *flag.FlagSet, incompatible bool) (*editorLaunchRequest, error) {
	openFileSet := false
	flags.Visit(func(f *flag.Flag) { openFileSet = openFileSet || f.Name == "open-file" })
	if !*o.show && !*o.standalone && !openFileSet {
		return nil, nil
	}
	if incompatible || (*o.show && *o.standalone) || flags.NArg() != 0 {
		return nil, errors.New("The Text Editor launch request cannot be combined with another application mode.")
	}
	request := &editorLaunchRequest{}
	if openFileSet {
		path, err := editorLaunchPath(*o.file)
		if err != nil {
			return nil, err
		}
		request.Path = path
	}
	return request, nil
}

type editorLaunchInbox struct {
	mu       sync.Mutex
	requests []editorLaunchRequest
}

func (q *editorLaunchInbox) add(request editorLaunchRequest) bool {
	q.mu.Lock()
	defer q.mu.Unlock()
	for _, pending := range q.requests {
		if pending == request {
			return true
		}
	}
	if len(q.requests) >= 12 {
		return false
	}
	q.requests = append(q.requests, request)
	return true
}

func (q *editorLaunchInbox) take() []editorLaunchRequest {
	q.mu.Lock()
	defer q.mu.Unlock()
	requests := q.requests
	q.requests = nil
	return requests
}

func editorLaunchPath(path string) (string, error) {
	if path == "" {
		return "", errors.New("Choose a file to open in Text Editor.")
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", errors.New("The file path could not be resolved.")
	}
	return checkedEditorLaunchPath(abs)
}

func checkedEditorLaunchPath(path string) (string, error) {
	path, err := absoluteExplorerPath(path)
	if err != nil {
		return "", errors.New("Choose a valid local text file path.")
	}
	if runtime.GOOS == "windows" {
		volume := filepath.VolumeName(path)
		if strings.HasPrefix(volume, "\\\\?\\") || strings.HasPrefix(volume, "\\\\.\\") || strings.Contains(path[len(volume):], ":") {
			return "", errors.New("Choose a regular file path in Text Editor.")
		}
	}
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() {
		return "", errors.New("The selected file is unavailable or is not a regular file.")
	}
	return path, nil
}

func prepareEditorWorkspace(root, mode string, request *editorLaunchRequest) (release func(), reused bool, err error) {
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	for {
		release, err = acquireTerminalWindowLock(filepath.Join(root, "Network_Toolkit.window.lock"))
		if !errors.Is(err, ErrTerminalWindowInUse) {
			return release, false, err
		}
		// Leave deliberate additional main launches alone. Only Explorer and
		// Text Editor shortcut launches target the registered workspace.
		if request == nil {
			return nil, false, nil
		}
		// Verify the process on loopback before sending the selected file path.
		if activateWorkspaceWindow(ctx, root, mode, editorLaunchEndpoint, nil) == nil &&
			activateWorkspaceWindow(ctx, root, mode, editorLaunchEndpoint, request) == nil {
			return nil, true, nil
		}
		select {
		case <-ctx.Done():
			return nil, false, errors.New("The existing Text Editor workspace is busy. Try opening the file again.")
		case <-time.After(100 * time.Millisecond):
		}
	}
}

func (a *application) handleEditorLaunch(w http.ResponseWriter, r *http.Request, activation bool) {
	if r.Method != http.MethodPost {
		methodError(w)
		return
	}
	if !activation {
		jsonResponse(w, http.StatusOK, map[string]any{"requests": a.editorLaunchInbox.take()})
		return
	}
	var request *editorLaunchRequest
	if err := decodeEditorBody(w, r, 32<<10, &request); err != nil {
		jsonError(w, http.StatusBadRequest, "The Text Editor launch request is invalid.")
		return
	}
	if a.shutdownRequested.Load() || a.ctx.Err() != nil {
		jsonError(w, http.StatusServiceUnavailable, "The workspace is closing.")
		return
	}
	if request != nil {
		if request.Path != "" {
			path, err := checkedEditorLaunchPath(request.Path)
			if err != nil {
				jsonError(w, http.StatusBadRequest, err.Error())
				return
			}
			request.Path = path
		}
		if !a.editorLaunchInbox.add(*request) {
			jsonError(w, http.StatusServiceUnavailable, "The editor is waiting to open other files.")
			return
		}
		a.activatePrimary()
	}
	jsonResponse(w, http.StatusOK, map[string]any{"ok": true, "windowMode": a.windowMode, "instanceId": a.windowInstanceID})
}
