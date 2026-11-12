package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
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

// applyOpenWithArgument accepts the one bare file path that Windows' own
// "Open with" entry passes. When the user picks a program in "Choose another
// app > Look for another app on this PC", Windows writes
// HKCU\Software\Classes\Applications\<program>\shell\open\command itself (if
// that key has no command yet) as "<program>" "%1": the program receives the
// file path alone, without --open-file. Such a start is the same request as
// --editor-standalone --open-file "<path>", and the flag set is parsed again
// in that form, so every check that follows (the edition, the launch request
// with editorLaunchPath, forwarding to the separate Text Editor program) sees
// exactly that. args are the program's arguments as given (os.Args[1:]).
//
// The path is accepted only alone: no switch beside it (the Text Editor
// program's standalone mode comes from its edition, not from a switch), never
// two or more, never after "--", and never anything with switch syntax: a
// leading "-", or on Windows a leading "/" that is not a UNC path ("//server"),
// since an absolute Windows path never starts with either. The Terminal
// program opens no file and refuses it.
func applyOpenWithArgument(flags *flag.FlagSet, args []string, edition, goos string) error {
	if flags.NArg() == 0 {
		return nil
	}
	program := programNameFor(goos, edition)
	if edition == "terminal" {
		return fmt.Errorf("%s opens %s only and does not open files. Open a file with %s or %s --editor-standalone --open-file \"<file>\".", program, editionTitle(edition), programNameFor(goos, standaloneEditorMode), programNameFor(goos, ""))
	}
	other := false
	flags.Visit(func(*flag.Flag) { other = true })
	refused := fmt.Errorf("%s opens a file given on its own (%s \"<file>\") or with --open-file \"<file>\". Nothing else can be given beside it.", program, program)
	if other || flags.NArg() != 1 || len(args) != 1 || args[0] != flags.Arg(0) {
		return refused
	}
	path := flags.Arg(0)
	if strings.HasPrefix(path, "-") || goos == "windows" && strings.HasPrefix(path, "/") && !strings.HasPrefix(path, "//") {
		return refused
	}
	return flags.Parse([]string{"--" + standaloneEditorMode, "--open-file=" + path})
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
