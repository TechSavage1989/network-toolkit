package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"networktoolkit/internal/macros"
)

const terminalMacroLibraryFilename = "Network_Toolkit.macros.json"
const terminalMacroLibraryLimit = 4 << 20

type terminalMacroBinding struct {
	ProfileID   string `json:"profileId"`
	MacroID     string `json:"macroId"`
	OnConnect   bool   `json:"onConnect"`
	OnReconnect bool   `json:"onReconnect"`
}

type terminalMacroLibrary struct {
	Version  int                    `json:"version"`
	Macros   []macros.Definition    `json:"macros"`
	Bindings []terminalMacroBinding `json:"bindings"`
	Revision string                 `json:"revision,omitempty"`
}

func validateTerminalMacroLibrary(library terminalMacroLibrary) (terminalMacroLibrary, error) {
	if library.Version != 0 && library.Version != 1 {
		return library, errors.New("This macro library version is not supported.")
	}
	library.Version = 1
	if len(library.Macros) > 500 || len(library.Bindings) > 1000 {
		return library, errors.New("A library can contain at most 500 macros and 1000 startup bindings.")
	}
	if library.Macros == nil {
		library.Macros = []macros.Definition{}
	}
	if library.Bindings == nil {
		library.Bindings = []terminalMacroBinding{}
	}
	ids, shortcuts := map[string]bool{}, map[string]bool{}
	for index, definition := range library.Macros {
		validated, err := macros.Validate(definition)
		if err != nil {
			return library, fmt.Errorf("Macro %d: %w", index+1, err)
		}
		if ids[validated.ID] {
			return library, errors.New("Each macro must have a unique ID.")
		}
		if validated.Shortcut != "" && shortcuts[validated.Shortcut] {
			return library, fmt.Errorf("The shortcut %s is already assigned to another macro.", validated.Shortcut)
		}
		ids[validated.ID], shortcuts[validated.Shortcut] = true, true
		library.Macros[index] = validated
	}
	bindings := map[string]bool{}
	for _, binding := range library.Bindings {
		if strings.TrimSpace(binding.ProfileID) == "" || len(binding.ProfileID) > 80 || !ids[binding.MacroID] {
			return library, errors.New("A startup binding must name a saved session and an existing macro.")
		}
		key := binding.ProfileID + "\x00" + binding.MacroID
		if bindings[key] {
			return library, errors.New("A startup macro can be assigned only once to the same saved session.")
		}
		bindings[key] = true
	}
	return library, nil
}

func readTerminalMacroLibrary(root *storageRoot) (terminalMacroLibrary, error) {
	library := terminalMacroLibrary{Version: 1, Macros: []macros.Definition{}, Bindings: []terminalMacroBinding{}}
	raw, err := readStorageFile(root, terminalMacroLibraryFilename, terminalMacroLibraryLimit)
	if errors.Is(err, os.ErrNotExist) {
		return library, nil
	}
	if err != nil {
		return library, err
	}
	if err := json.Unmarshal(raw, &library); err != nil {
		return library, fmt.Errorf("The saved macro library is invalid: %w", err)
	}
	library, err = validateTerminalMacroLibrary(library)
	if err != nil {
		return library, err
	}
	digest := sha256.Sum256(raw)
	library.Revision = hex.EncodeToString(digest[:])
	return library, nil
}

func (a *application) handleTerminalMacroLibrary(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodPut {
		methodError(w)
		return
	}
	var requested terminalMacroLibrary
	if r.Method == http.MethodPut {
		if err := decodeTerminalBody(w, r, terminalMacroLibraryLimit, &requested); err != nil {
			jsonError(w, 400, err.Error())
			return
		}
		validated, err := validateTerminalMacroLibrary(requested)
		if err != nil {
			jsonError(w, 400, err.Error())
			return
		}
		requested = validated
	}
	a.stateMu.Lock()
	defer a.stateMu.Unlock()
	root, err := openStorageRoot(filepath.Dir(a.dataPath), true)
	if err != nil {
		jsonError(w, 500, "The macro library could not be opened: "+err.Error())
		return
	}
	defer root.Close()
	library, err := readTerminalMacroLibrary(root)
	if err != nil {
		jsonError(w, 500, err.Error())
		return
	}
	if r.Method == http.MethodPut {
		if requested.Revision != library.Revision {
			jsonError(w, 409, "The macro library changed in another window. Reload it before saving again.")
			return
		}
		requested.Revision = ""
		raw, marshalErr := json.MarshalIndent(requested, "", "  ")
		if marshalErr != nil {
			jsonError(w, 400, marshalErr.Error())
			return
		}
		if err := writeStorageAtomicLimit(root, terminalMacroLibraryFilename, raw, terminalMacroLibraryLimit); err != nil {
			jsonError(w, 500, "The macro library could not be saved: "+err.Error())
			return
		}
		digest := sha256.Sum256(raw)
		requested.Revision = hex.EncodeToString(digest[:])
		library = requested
	}
	jsonResponse(w, 200, library)
}

// A linked window can automate only sessions assigned to its capability. A
// group run stays with its original sessions while their displays move.
func (a *application) terminalAutomationScope(r *http.Request) (map[string]bool, error) {
	viewID := terminalRequestView(r)
	if viewID == "" {
		return nil, nil
	}
	info, ok := a.views.info(viewID)
	if !ok || info.State != "detached" && info.State != "return_requested" {
		return nil, errors.New("This terminal is moving between windows or has closed.")
	}
	allowed := map[string]bool{}
	if info.Workspace {
		for _, id := range info.SessionIDs {
			allowed[id] = true
		}
	} else {
		allowed[info.SessionID] = true
	}
	return allowed, nil
}

func terminalMacroRunAllowed(run macros.Run, scope map[string]bool) bool {
	for _, target := range run.Targets {
		if scope != nil && !scope[target.ID] {
			return false
		}
	}
	return true
}

func terminalMacroRefsAllowed(refs []macros.TargetRef, scope map[string]bool) bool {
	for _, target := range refs {
		if scope != nil && !scope[target.ID] {
			return false
		}
	}
	return true
}

func (a *application) handleTerminalAutomation(w http.ResponseWriter, r *http.Request) {
	scope, err := a.terminalAutomationScope(r)
	if err != nil {
		jsonError(w, 409, err.Error())
		return
	}
	path := strings.TrimPrefix(r.URL.Path, "/api/terminal-automation/")
	if path == "library" {
		a.handleTerminalMacroLibrary(w, r)
		return
	}
	engine := a.terminalAutomation
	if engine == nil {
		jsonError(w, 503, "Terminal automation is unavailable.")
		return
	}
	switch path {
	case "targets":
		if r.Method != http.MethodGet {
			methodError(w)
			return
		}
		targets := []macros.Target{}
		for _, target := range engine.Targets() {
			if scope == nil || scope[target.ID] {
				targets = append(targets, target)
			}
		}
		jsonResponse(w, 200, map[string]any{"targets": targets})
		return
	case "runs":
		switch r.Method {
		case http.MethodGet:
			runs := []macros.Run{}
			for _, run := range engine.List() {
				if terminalMacroRunAllowed(run, scope) {
					runs = append(runs, run)
				}
			}
			jsonResponse(w, 200, map[string]any{"runs": runs})
		case http.MethodPost:
			var request macros.StartRequest
			if err := decodeTerminalBody(w, r, terminalMacroLibraryLimit+1<<20, &request); err != nil {
				jsonError(w, 400, err.Error())
				return
			}
			if !terminalMacroRefsAllowed(request.Targets, scope) {
				jsonError(w, 403, "A destination is not assigned to this terminal window.")
				return
			}
			run, err := engine.Start(request)
			if err != nil {
				jsonError(w, 409, err.Error())
				return
			}
			jsonResponse(w, 201, map[string]any{"run": run})
		case http.MethodDelete:
			ids := []string{}
			for _, run := range engine.List() {
				if terminalMacroRunAllowed(run, scope) {
					ids = append(ids, run.ID)
				}
			}
			jsonResponse(w, 200, map[string]any{"cleared": engine.ClearFinished(ids)})
		default:
			methodError(w)
		}
		return
	case "send":
		if r.Method != http.MethodPost {
			methodError(w)
			return
		}
		var request macros.SendRequest
		if err := decodeTerminalBody(w, r, 128<<10, &request); err != nil {
			jsonError(w, 400, err.Error())
			return
		}
		if !terminalMacroRefsAllowed(request.Targets, scope) {
			jsonError(w, 403, "A destination is not assigned to this terminal window.")
			return
		}
		if request.Execute {
			if strings.TrimSpace(request.Text) == "" || len(request.Text) > macros.MaxStepTextBytes {
				jsonError(w, 400, "Enter commands no larger than 64 KiB.")
				return
			}
			definition := macros.Definition{ID: "send-commands", Name: "Sent commands", Steps: macros.FromCommands(request.Text), WaitForPrompt: true, Prompt: macros.DefaultPrompt, TimeoutMs: macros.DefaultTimeoutMs, PacingMs: macros.DefaultPacingMs, AutoPager: true}
			run, err := engine.Start(macros.StartRequest{Macro: definition, Targets: request.Targets})
			if err != nil {
				jsonError(w, 409, err.Error())
				return
			}
			jsonResponse(w, 201, map[string]any{"run": run, "targets": []macros.SendTargetResult{}})
			return
		}
		result, err := engine.Send(request)
		if err != nil {
			jsonError(w, 409, err.Error())
			return
		}
		jsonResponse(w, 200, result)
		return
	}
	parts := strings.Split(path, "/")
	if len(parts) < 2 || len(parts) > 3 || parts[0] != "runs" || parts[1] == "" {
		jsonError(w, 404, "This terminal automation action was not found.")
		return
	}
	run, err := engine.Get(parts[1])
	if err != nil {
		jsonError(w, 404, err.Error())
		return
	}
	if !terminalMacroRunAllowed(run, scope) {
		jsonError(w, 403, "This run includes sessions outside this terminal window.")
		return
	}
	if len(parts) == 2 && r.Method == http.MethodGet {
		jsonResponse(w, 200, map[string]any{"run": run})
		return
	}
	if len(parts) == 2 && r.Method == http.MethodDelete {
		if err := engine.Clear(run.ID); err != nil {
			status := http.StatusNotFound
			if errors.Is(err, macros.ErrRunActive) {
				status = http.StatusConflict
			}
			jsonError(w, status, err.Error())
			return
		}
		jsonResponse(w, 200, map[string]any{"cleared": []string{run.ID}})
		return
	}
	if len(parts) == 3 && parts[2] == "control" && r.Method == http.MethodPost {
		var request struct {
			Action string `json:"action"`
		}
		if err := decodeTerminalBody(w, r, 1024, &request); err != nil {
			jsonError(w, 400, err.Error())
			return
		}
		run, err = engine.Control(run.ID, request.Action)
		if err != nil {
			jsonError(w, 409, err.Error())
			return
		}
		jsonResponse(w, 200, map[string]any{"run": run})
		return
	}
	if len(parts) == 3 && parts[2] == "output" && r.Method == http.MethodGet {
		output, err := engine.Output(run.ID, r.URL.Query().Get("sessionId"))
		if err != nil {
			jsonError(w, 404, err.Error())
			return
		}
		if r.URL.Query().Get("format") == "json" {
			jsonResponse(w, 200, map[string]string{"text": string(output)})
		} else {
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			_, _ = w.Write(output)
		}
		return
	}
	methodError(w)
}
