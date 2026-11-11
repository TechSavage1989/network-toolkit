package main

import (
	"context"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// App settings for the Windows integration of the main window:
//
//	GET  /api/windows-integration                         both features' status
//	POST /api/windows-integration/text-editor/register    {contextMenu, takeover}
//	POST /api/windows-integration/text-editor/remove      {}
//	POST /api/windows-integration/full-install/find       {channel}
//	POST /api/windows-integration/full-install/download   {version, confirm}
//	POST /api/windows-integration/full-install/start      {version, confirm}
//	POST /api/windows-integration/full-install/discard    {}
//	POST /api/windows-integration/open-installed          {confirm}
//
// Every change happens on a POST from the page, never at startup. The routes
// exist in the main window only (portable or installed); the standalone Text
// Editor and Terminal and child windows do not serve them.
const windowsIntegrationPath = "/api/windows-integration"

var (
	windowsIntegrationPlatform   = editorRegDefaultPlatform
	windowsIntegrationExecutable = currentProgramPath
)

func currentProgramPath() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	if resolved, err := filepath.EvalSymlinks(exe); err == nil {
		exe = resolved
	}
	return filepath.Clean(exe), nil
}

type windowsIntegrationData struct {
	Supported  bool                     `json:"supported"`
	TextEditor editorRegistrationStatus `json:"textEditor"`
	Install    fullInstallStatus        `json:"install"`
	Error      string                   `json:"error,omitempty"`
}

func (a *application) windowsIntegrationStatus() windowsIntegrationData {
	data := windowsIntegrationData{}
	platform, supported := windowsIntegrationPlatform()
	if !supported || a.windowMode != "main" {
		return data
	}
	data.Supported = true
	exe, err := windowsIntegrationExecutable()
	if err != nil {
		data.Error = "The program's location could not be read: " + err.Error()
		return data
	}
	status, err := editorRegState(platform.registry, exe, platform.missing)
	if err != nil {
		data.Error = err.Error()
	}
	data.TextEditor = status
	data.Install = a.fullInstallStatus(status)
	return data
}

func (a *application) handleWindowsIntegration(w http.ResponseWriter, r *http.Request) {
	platform, supported := windowsIntegrationPlatform()
	if a.windowMode != "main" {
		http.NotFound(w, r)
		return
	}
	if r.URL.Path == windowsIntegrationPath {
		if r.Method != http.MethodGet {
			methodError(w)
			return
		}
		jsonResponse(w, http.StatusOK, a.windowsIntegrationStatus())
		return
	}
	if r.Method != http.MethodPost {
		methodError(w)
		return
	}
	if !supported {
		jsonError(w, http.StatusNotImplemented, "This is available in the Windows program.")
		return
	}
	exe, err := windowsIntegrationExecutable()
	if err != nil {
		jsonError(w, http.StatusInternalServerError, "The program's location could not be read: "+err.Error())
		return
	}
	switch strings.TrimPrefix(r.URL.Path, windowsIntegrationPath) {
	case "/text-editor/register":
		var request struct {
			ContextMenu bool `json:"contextMenu"`
			Takeover    bool `json:"takeover"`
		}
		if err := decodeBody(w, r, 1024, &request); err != nil {
			jsonError(w, http.StatusBadRequest, err.Error())
			return
		}
		status, err := registerEditorCopy(platform, exe, request.ContextMenu, request.Takeover)
		a.writeEditorRegResult(w, status, err)
	case "/text-editor/remove":
		var request struct{}
		if err := decodeBody(w, r, 1024, &request); err != nil {
			jsonError(w, http.StatusBadRequest, err.Error())
			return
		}
		status, err := removeEditorCopy(platform, exe)
		a.writeEditorRegResult(w, status, err)
	case "/open-installed":
		var request struct {
			Confirm bool `json:"confirm"`
		}
		if err := decodeBody(w, r, 1024, &request); err != nil {
			jsonError(w, http.StatusBadRequest, err.Error())
			return
		}
		status, err := editorRegState(platform.registry, exe, platform.missing)
		if err != nil {
			jsonError(w, http.StatusServiceUnavailable, err.Error())
			return
		}
		installed := filepath.Join(status.InstallLocation, editorRegAppFile)
		if !request.Confirm || !status.Installed || status.ThisIsInstalled {
			jsonError(w, http.StatusConflict, "The installed program is not available from this copy.")
			return
		}
		if platform.missing(installed) {
			jsonError(w, http.StatusConflict, "The installed program's file is missing at "+installed+". Run Setup to repair it.")
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()
		if err := fullInstallOpen(ctx, installed); err != nil {
			jsonError(w, http.StatusServiceUnavailable, "The installed program could not be started: "+err.Error())
			return
		}
		jsonResponse(w, http.StatusOK, map[string]any{"ok": true, "path": installed})
	default:
		if strings.HasPrefix(r.URL.Path, windowsIntegrationPath+"/full-install/") {
			a.handleFullInstall(w, r, platform, exe)
			return
		}
		jsonError(w, http.StatusNotFound, "Unknown Windows integration action.")
	}
}

func (a *application) writeEditorRegResult(w http.ResponseWriter, status editorRegistrationStatus, err error) {
	if err == nil {
		jsonResponse(w, http.StatusOK, map[string]any{"textEditor": status})
		return
	}
	code := http.StatusConflict
	switch {
	case errors.Is(err, errEditorRegTakeover):
		jsonResponse(w, code, map[string]any{"error": err.Error(), "needsTakeover": true, "textEditor": status})
		return
	case errors.Is(err, errEditorRegInstalled), errors.Is(err, errEditorRegName), errors.Is(err, errEditorRegNothing):
	default:
		code = http.StatusServiceUnavailable
	}
	jsonResponse(w, code, map[string]any{"error": err.Error(), "textEditor": status})
}

// The hidden switch --text-editor-registration=<action> runs one action of
// "Use this copy's Text Editor in Windows" and exits, for the hosted Setup
// check (scripts/installer/verify-windows.ps1). It accepts no other switch and
// no argument, opens no window, settings or data folder, and shows no message.
// Exit codes: 0 done; 2 failed; 3 the installed program is registered; 4
// another copy is registered (use takeover); 5 the program file is not named
// Network_Toolkit.exe; 6 nothing of this copy's to remove.
const textEditorRegistrationFlag = "text-editor-registration"

func runTextEditorRegistrationCommand(action string) int {
	platform, supported := windowsIntegrationPlatform()
	if !supported || runtime.GOOS != "windows" {
		return 2
	}
	exe, err := windowsIntegrationExecutable()
	if err != nil {
		return 2
	}
	switch action {
	case "register", "register-menu", "takeover", "takeover-menu":
		_, err = registerEditorCopy(platform, exe, strings.HasSuffix(action, "-menu"), strings.HasPrefix(action, "takeover"))
	case "remove", "cleanup":
		var status editorRegistrationStatus
		if status, err = editorRegState(platform.registry, exe, platform.missing); err == nil {
			if action == "remove" && status.State != editorRegThis {
				err = errEditorRegNothing
				if status.State == editorRegSetup {
					err = errEditorRegInstalled
				}
			} else if action == "cleanup" && status.State == editorRegThis {
				err = errEditorRegNothing
			} else {
				_, err = removeEditorCopy(platform, exe)
			}
		}
	default:
		return 2
	}
	switch {
	case err == nil:
		return 0
	case errors.Is(err, errEditorRegInstalled):
		return 3
	case errors.Is(err, errEditorRegTakeover):
		return 4
	case errors.Is(err, errEditorRegName):
		return 5
	case errors.Is(err, errEditorRegNothing):
		return 6
	}
	return 2
}
