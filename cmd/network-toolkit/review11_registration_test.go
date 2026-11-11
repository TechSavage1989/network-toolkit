package main

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

// Security review 11, items 3 to 5 (Text Editor registration).

// Item 3: Explorer expands %1, %L and the like inside the program path too.
func TestReviewPercentInProgramPathIsRefused(t *testing.T) {
	exe := `C:\Tools\100%L done\Network_Toolkit.exe`
	reg := fixtureRegistry()
	before := reg.dump()
	platform, _ := testEditorPlatform(reg)
	status, err := editorRegStatus(platform, exe)
	if err != nil || status.CanRegister || status.PathOK || !strings.Contains(status.Message, "%") {
		t.Fatalf("%+v %v", status, err)
	}
	for _, takeover := range []bool{false, true} {
		if _, err := registerEditorCopy(platform, exe, true, takeover); !errors.Is(err, errEditorRegPercent) {
			t.Fatalf("register with %% in the path: %v", err)
		}
	}
	if reg.dump() != before {
		t.Fatal("a refused registration wrote")
	}
}

// Item 4: a copy that other users can change would run for every
// double-clicked text file.
func TestReviewWritableLocationIsRefused(t *testing.T) {
	for name, access := range map[string]editorRegAccessCheck{
		"others can write the folder": func(path string) (string, error) {
			if path == editorRegDir(testCopyA) {
				return `BUILTIN\Users`, nil
			}
			return "", nil
		},
		"others can write the program": func(path string) (string, error) {
			if path == testCopyA {
				return `Everyone`, nil
			}
			return "", nil
		},
		"cannot be checked": func(string) (string, error) { return "", errors.New("synthetic access failure") },
	} {
		t.Run(name, func(t *testing.T) {
			reg := fixtureRegistry()
			before := reg.dump()
			platform, _ := testEditorPlatform(reg)
			platform.access = access
			status, err := editorRegStatus(platform, testCopyA)
			if err != nil || status.CanRegister || status.LocationProblem == "" {
				t.Fatalf("%+v %v", status, err)
			}
			if name != "cannot be checked" && (!strings.Contains(status.LocationProblem, "double-click") || !strings.Contains(status.LocationProblem, "user folder")) {
				t.Errorf("the refusal should explain why and how to fix it: %q", status.LocationProblem)
			}
			for _, takeover := range []bool{false, true} {
				if _, err := registerEditorCopy(platform, testCopyA, false, takeover); !errors.Is(err, errEditorRegUnsafe) {
					t.Fatalf("register from a writable location: %v", err)
				}
			}
			if reg.dump() != before {
				t.Fatal("a refused registration wrote")
			}
		})
	}
	// Remove still works from such a location: it only takes back.
	reg := fixtureRegistry()
	platform, _ := testEditorPlatform(reg)
	if _, err := registerEditorCopy(platform, testCopyA, false, false); err != nil {
		t.Fatal(err)
	}
	platform.access = func(string) (string, error) { return "Everyone", nil }
	if _, err := removeEditorCopy(platform, testCopyA); err != nil {
		t.Fatalf("remove from a now writable location: %v", err)
	}
}

// Item 5: a drive that is not connected is not a missing program.
func TestReviewDisconnectedDriveIsNotMissing(t *testing.T) {
	present := map[string]bool{`C:\`: true, `C:\Tools\Network Toolkit\Network_Toolkit.exe`: true, `\\server\share\`: true}
	stat := func(path string) (os.FileInfo, error) {
		if present[path] {
			return nil, nil
		}
		if strings.HasPrefix(path, `E:`) {
			return nil, errors.New("synthetic access denied")
		}
		return nil, os.ErrNotExist
	}
	for path, want := range map[string]editorRegPresence{
		`C:\Tools\Network Toolkit\Network_Toolkit.exe`: editorRegPresent,
		`C:\Gone\Network_Toolkit.exe`:                  editorRegAbsent,
		`D:\Portable\Network_Toolkit.exe`:              editorRegUnreachable,
		`\\server\share\NTK\Network_Toolkit.exe`:       editorRegAbsent,
		`\\other\share\NTK\Network_Toolkit.exe`:        editorRegUnreachable,
		`E:\NTK\Network_Toolkit.exe`:                   editorRegUnreachable,
	} {
		if got := editorRegClassify(path, stat); got != want {
			t.Errorf("%s: %v, want %v", path, got, want)
		}
	}
	reg := fixtureRegistry()
	platform, _ := testEditorPlatform(reg)
	if _, err := registerEditorCopy(platform, testCopyB, true, false); err != nil {
		t.Fatal(err)
	}
	platform.presence = func(path string) editorRegPresence {
		if path == testCopyB {
			return editorRegUnreachable
		}
		return editorRegPresent
	}
	before := reg.dump()
	status, err := editorRegStatus(platform, testCopyA)
	if err != nil || status.OwnerMissing || !status.OwnerUnreachable || status.CanRemove || !strings.Contains(status.Message, "not connected") {
		t.Fatalf("%+v %v", status, err)
	}
	if _, err := cleanupEditorCopy(platform, testCopyA, testCopyB); err == nil || reg.dump() != before {
		t.Fatalf("clean up of a registration on a disconnected drive: %v", err)
	}
}

// Item 5: Clean up carries the program the user saw and runs only while
// that registration is still another copy's whose program is missing.
func TestReviewCleanupNeedsTheOwnerTheUserSaw(t *testing.T) {
	reg := fixtureRegistry()
	platform, _ := testEditorPlatform(reg)
	if _, err := registerEditorCopy(platform, testCopyB, true, false); err != nil {
		t.Fatal(err)
	}
	before := reg.dump()
	// B exists: no clean up, whatever the page says.
	if _, err := cleanupEditorCopy(platform, testCopyA, testCopyB); !errors.Is(err, errEditorRegStale) {
		t.Fatalf("clean up of a present copy: %v", err)
	}
	platform, _ = testEditorPlatform(reg, testCopyB)
	// Remove is for this copy's own registration only.
	if _, err := removeEditorCopy(platform, testCopyA); !errors.Is(err, errEditorRegNothing) {
		t.Fatalf("remove of another copy's registration: %v", err)
	}
	// A different owner than the one now registered: refused.
	if _, err := cleanupEditorCopy(platform, testCopyA, `F:\Seen\Earlier\Network_Toolkit.exe`); !errors.Is(err, errEditorRegStale) {
		t.Fatalf("clean up with a stale owner: %v", err)
	}
	if reg.dump() != before {
		t.Fatal("a refused clean up changed the registry")
	}
	// Through the route, with the owner the page showed.
	oldPlatform, oldExe := windowsIntegrationPlatform, windowsIntegrationExecutable
	t.Cleanup(func() { windowsIntegrationPlatform, windowsIntegrationExecutable = oldPlatform, oldExe })
	windowsIntegrationPlatform = func() (editorRegPlatform, bool) { return platform, true }
	windowsIntegrationExecutable = func() (string, error) { return testCopyA, nil }
	a := &application{windowMode: "main"}
	call := func(path, body string) int {
		w := httptest.NewRecorder()
		a.handleWindowsIntegration(w, httptest.NewRequest(http.MethodPost, windowsIntegrationPath+path, strings.NewReader(body)))
		return w.Code
	}
	if code := call("/text-editor/remove", `{}`); code != http.StatusConflict || reg.dump() != before {
		t.Fatalf("the remove route cleaned up another copy's registration: %d", code)
	}
	if code := call("/text-editor/cleanup", `{}`); code != http.StatusBadRequest && code != http.StatusConflict || reg.dump() != before {
		t.Fatalf("clean up without the owner: %d", code)
	}
	owner, _ := json.Marshal(map[string]string{"owner": testCopyB})
	if code := call("/text-editor/cleanup", string(owner)); code != http.StatusOK || reg.has(editorRegPortable) {
		t.Fatalf("clean up with the owner the page showed: %d", code)
	}
}
