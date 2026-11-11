package main

import (
	"archive/zip"
	"bytes"
	"context"
	"debug/pe"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path"
	"path/filepath"
	"strings"
	"testing"

	"networktoolkit/internal/model"
)

// updateTestProgramPE is a minimal Windows x64 GUI image whose resource
// section names the product, the version and a program's original file name,
// like the release builds of each program.
func updateTestProgramPE(version, original string) []byte {
	resource := append(updateUTF16("Network Toolkit"), updateUTF16("NetworkToolkit")...)
	resource = append(resource, updateUTF16(version)...)
	resource = append(resource, updateUTF16(original)...)
	b := make([]byte, 0x200)
	b[0], b[1] = 'M', 'Z'
	binary.LittleEndian.PutUint32(b[0x3c:], 0x80)
	copy(b[0x80:], []byte{'P', 'E', 0, 0})
	h := pe.FileHeader{Machine: pe.IMAGE_FILE_MACHINE_AMD64, NumberOfSections: 1, SizeOfOptionalHeader: 240, Characteristics: 2}
	var header bytes.Buffer
	binary.Write(&header, binary.LittleEndian, h)
	binary.Write(&header, binary.LittleEndian, pe.OptionalHeader64{Magic: 0x20b, Subsystem: 2, NumberOfRvaAndSizes: 16})
	section := pe.SectionHeader32{SizeOfRawData: uint32(len(resource)), PointerToRawData: 0x200}
	copy(section.Name[:], ".rsrc")
	binary.Write(&header, binary.LittleEndian, section)
	copy(b[0x84:], header.Bytes())
	return append(b, resource...)
}

type updateTestRelease struct {
	main, editor, terminal []byte
	// members to leave out, swap or add for the negative cases
	omit  map[string]bool
	swap  bool
	extra map[string][]byte
}

// updateTestReleaseZip builds an Update ZIP of the new contract: the main
// program under its released member name plus the separate programs.
func updateTestReleaseZip(t *testing.T, version string, r updateTestRelease) (string, []byte, []byte) {
	t.Helper()
	root := "Network_Toolkit_" + version + "/"
	members := map[string][]byte{"Network_Toolkit_" + version + "_Windows_x64.exe": r.main, "UPDATING.txt": []byte("synthetic"), "THIRD_PARTY_NOTICES.txt": []byte("synthetic")}
	if r.editor != nil {
		members[updateCompanionMember(version, standaloneEditorMode)] = r.editor
	}
	if r.terminal != nil {
		members[updateCompanionMember(version, "terminal")] = r.terminal
	}
	if r.swap {
		editor, terminal := updateCompanionMember(version, standaloneEditorMode), updateCompanionMember(version, "terminal")
		members[editor], members[terminal] = members[terminal], members[editor]
	}
	var sums strings.Builder
	for name, data := range members {
		if !r.omit["sum:"+name] {
			fmt.Fprintf(&sums, "%s  %s\n", updateHash(data), name)
		}
	}
	members["SHA256SUMS"] = []byte(sums.String())
	for name, data := range r.extra {
		members[name] = data
	}
	var buf bytes.Buffer
	z := zip.NewWriter(&buf)
	for name, data := range members {
		if r.omit[name] {
			continue
		}
		w, err := z.Create(root + name)
		if err != nil {
			t.Fatal(err)
		}
		w.Write(data)
	}
	z.Close()
	name := "Network_Toolkit_" + version + "_Update.zip"
	return name, buf.Bytes(), []byte(fmt.Sprintf("%s  %s\n", updateHash(buf.Bytes()), name))
}

func updateTestPrograms(version string) updateTestRelease {
	return updateTestRelease{main: updateTestProgramPE(version, "Network_Toolkit.exe"), editor: updateTestProgramPE(version, "Network_Toolkit_TextEditor.exe"), terminal: updateTestProgramPE(version, "Network_Toolkit_Terminal.exe")}
}

// released12144ExtractPackage is updateExtractPackage exactly as v1.21.44 and
// every release since v1.17.63 shipped it (frozen copy). It stands in for the
// updater of a released version reading an Update ZIP of this version.
func released12144ExtractPackage(filename string, archive, checksums []byte, current string) (string, []byte, error) {
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

func TestReleasedUpdaterInstallsOnlyTheMainProgramFromTheNewUpdateZip(t *testing.T) {
	programs := updateTestPrograms("1.21.45")
	name, archive, sums := updateTestReleaseZip(t, "1.21.45", programs)
	version, executable, err := released12144ExtractPackage(name, archive, sums, "1.21.44")
	if err != nil || version != "1.21.45" || !bytes.Equal(executable, programs.main) {
		t.Fatalf("a released updater must accept the new Update ZIP and take only Network_Toolkit.exe: %v", err)
	}
	// The legacy entry point of this version gives the same answer.
	if version, executable, err = updateExtractPackage(name, archive, sums, "1.21.44"); err != nil || !bytes.Equal(executable, programs.main) {
		t.Fatalf("updateExtractPackage: %s %v", version, err)
	}
}

func TestUpdateExtractReleaseVerifiesTheSeparatePrograms(t *testing.T) {
	good := updateTestPrograms("1.21.45")
	name, archive, sums := updateTestReleaseZip(t, "1.21.45", good)
	pkg, err := updateExtractRelease(name, archive, sums, "1.21.44", false)
	if err != nil || !bytes.Equal(pkg.main, good.main) || !bytes.Equal(pkg.companions["Network_Toolkit_TextEditor.exe"], good.editor) || !bytes.Equal(pkg.companions["Network_Toolkit_Terminal.exe"], good.terminal) || len(pkg.companions) != 2 {
		t.Fatalf("new format: %v %d", err, len(pkg.companions))
	}
	// A package of the earlier format has no separate programs.
	name, archive, sums = updateTestReleaseZip(t, "1.21.45", updateTestRelease{main: good.main})
	if pkg, err = updateExtractRelease(name, archive, sums, "1.21.44", false); err != nil || pkg.companions != nil {
		t.Fatalf("earlier format: %v %v", err, pkg.companions)
	}
	for _, tc := range []struct {
		name string
		r    updateTestRelease
		want string
	}{
		{"only the Text Editor", updateTestRelease{main: good.main, editor: good.editor}, "only one of the separate"},
		{"only the Terminal", updateTestRelease{main: good.main, terminal: good.terminal}, "only one of the separate"},
		{"separate program without a checksum line", updateTestRelease{main: good.main, editor: good.editor, terminal: good.terminal, omit: map[string]bool{"sum:" + updateCompanionMember("1.21.45", "terminal"): true}}, "Checksum"},
		{"programs swapped", updateTestRelease{main: good.main, editor: good.editor, terminal: good.terminal, swap: true}, "does not match its package"},
		{"another version", updateTestRelease{main: good.main, editor: updateTestProgramPE("1.21.44", "Network_Toolkit_TextEditor.exe"), terminal: good.terminal}, "version"},
		{"a separate program as the main program", updateTestRelease{main: good.terminal, editor: good.editor, terminal: good.terminal}, "not Network_Toolkit.exe"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			name, archive, sums := updateTestReleaseZip(t, "1.21.45", tc.r)
			if _, err := updateExtractRelease(name, archive, sums, "1.21.44", false); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("want an error with %q, got %v", tc.want, err)
			}
		})
	}
	// The running version only for installing its own separate programs.
	name, archive, sums = updateTestReleaseZip(t, "1.21.45", good)
	if _, err = updateExtractRelease(name, archive, sums, "1.21.45", false); err == nil {
		t.Fatal("same version accepted for an update")
	}
	if pkg, err = updateExtractRelease(name, archive, sums, "1.21.45", true); err != nil || len(pkg.companions) != 2 {
		t.Fatalf("same version for the separate programs: %v", err)
	}
	if _, err = updateExtractRelease(name, archive, sums, "1.21.46", true); err == nil {
		t.Fatal("an older version accepted")
	}
}

// withRunningProgram makes updateCurrentExecutable name a main program of this
// version in a folder of the test's own.
func withRunningProgram(t *testing.T, data []byte) string {
	t.Helper()
	dir := t.TempDir()
	exe := filepath.Join(dir, "Network_Toolkit.exe")
	if err := os.WriteFile(exe, data, 0700); err != nil {
		t.Fatal(err)
	}
	updateNamingState.Lock()
	prior := updateNamingState.status
	updateNamingState.status = updateExecutableNamingStatus{CurrentPath: exe, CanonicalPath: exe, SHA256: updateHash(data)}
	updateNamingState.Unlock()
	t.Cleanup(func() { updateNamingState.Lock(); updateNamingState.status = prior; updateNamingState.Unlock() })
	return dir
}

func TestUpdateStageKeepsTheThreeProgramsTogether(t *testing.T) {
	dir := withRunningProgram(t, updateTestProgramPE(model.Version, "Network_Toolkit.exe"))
	a := testApplication(t)
	next := updateTestPrograms("99.0.0")
	pkg := updatePackage{version: "99.0.0", main: next.main, companions: map[string][]byte{"Network_Toolkit_TextEditor.exe": next.editor, "Network_Toolkit_Terminal.exe": next.terminal}}
	if err := a.updateStagePackage(pkg); err != nil {
		t.Fatal(err)
	}
	staged := a.updater.staged
	if len(staged.Companions) != 2 || staged.Companions[0].Name != "Network_Toolkit_TextEditor.exe" || staged.Companions[1].SHA256 != updateHash(next.terminal) {
		t.Fatalf("staged companions: %+v", staged.Companions)
	}
	for _, c := range staged.Companions {
		if b, err := os.ReadFile(filepath.Join(staged.dir, "candidate-"+c.Name)); err != nil || updateHash(b) != c.SHA256 {
			t.Fatalf("staged %s: %v", c.Name, err)
		}
	}
	encoded, _ := json.Marshal(a.updateStatusLocked()["staged"])
	if !strings.Contains(string(encoded), `"companions":[{"name":"Network_Toolkit_TextEditor.exe"`) {
		t.Fatalf("the status does not show the staged programs: %s", encoded)
	}
	stageDir := staged.dir
	a.cleanupStagedUpdate()
	if _, err := os.Stat(stageDir); !os.IsNotExist(err) {
		t.Fatal("a stage that was never installed must be removed with its separate programs")
	}
	// Separate programs beside the running program: a package without them is refused.
	if err := os.WriteFile(filepath.Join(dir, "Network_Toolkit_Terminal.exe"), next.terminal, 0700); err != nil {
		t.Fatal(err)
	}
	if err := a.updateStagePackage(updatePackage{version: "99.0.0", main: next.main}); err == nil || !strings.Contains(err.Error(), "does not include the separate") {
		t.Fatalf("a package without the installed separate programs was staged: %v", err)
	}
	if a.updater.staged != nil {
		t.Fatal("a refused package left a stage")
	}
}

func TestInstallCompanionsPlacesBothOrNeither(t *testing.T) {
	programs := map[string][]byte{"Network_Toolkit_TextEditor.exe": updateTestProgramPE("1.21.45", "Network_Toolkit_TextEditor.exe"), "Network_Toolkit_Terminal.exe": updateTestProgramPE("1.21.45", "Network_Toolkit_Terminal.exe")}
	rename := func(target, candidate, backup string) error {
		if err := os.Rename(target, backup); err != nil {
			return err
		}
		return os.Rename(candidate, target)
	}
	t.Run("missing programs are created", func(t *testing.T) {
		dir := t.TempDir()
		if err := updateInstallCompanions(dir, programs, rename); err != nil {
			t.Fatal(err)
		}
		for name, data := range programs {
			if b, err := os.ReadFile(filepath.Join(dir, name)); err != nil || !bytes.Equal(b, data) {
				t.Fatalf("%s: %v", name, err)
			}
		}
		entries, _ := os.ReadDir(dir)
		if len(entries) != 2 {
			t.Fatalf("transaction files left: %v", entries)
		}
	})
	t.Run("a program of another build is replaced, the backup removed", func(t *testing.T) {
		dir := t.TempDir()
		if err := os.WriteFile(filepath.Join(dir, "Network_Toolkit_TextEditor.exe"), []byte("older build"), 0700); err != nil {
			t.Fatal(err)
		}
		if err := updateInstallCompanions(dir, programs, rename); err != nil {
			t.Fatal(err)
		}
		if b, _ := os.ReadFile(filepath.Join(dir, "Network_Toolkit_TextEditor.exe")); !bytes.Equal(b, programs["Network_Toolkit_TextEditor.exe"]) {
			t.Fatal("not replaced")
		}
		entries, _ := os.ReadDir(dir)
		if len(entries) != 2 {
			t.Fatalf("transaction files left: %v", entries)
		}
	})
	t.Run("an open program stops both and nothing stays", func(t *testing.T) {
		dir := t.TempDir()
		// The Text Editor is created first; the Terminal is "open" and cannot be replaced.
		if err := os.WriteFile(filepath.Join(dir, "Network_Toolkit_Terminal.exe"), []byte("older build, running"), 0700); err != nil {
			t.Fatal(err)
		}
		open := func(target, candidate, backup string) error {
			return errors.New("The process cannot access the file because it is being used by another process.")
		}
		err := updateInstallCompanions(dir, programs, open)
		if err == nil || !strings.Contains(err.Error(), "close it if it is open") {
			t.Fatalf("want a refusal, got %v", err)
		}
		if _, err := os.Stat(filepath.Join(dir, "Network_Toolkit_TextEditor.exe")); !os.IsNotExist(err) {
			t.Fatal("the program created before the failure was not removed")
		}
		if b, _ := os.ReadFile(filepath.Join(dir, "Network_Toolkit_Terminal.exe")); string(b) != "older build, running" {
			t.Fatal("the open program changed")
		}
		entries, _ := os.ReadDir(dir)
		if len(entries) != 1 {
			t.Fatalf("transaction files left: %v", entries)
		}
	})
	t.Run("a replaced program is put back when the next one fails", func(t *testing.T) {
		dir := t.TempDir()
		for _, name := range []string{"Network_Toolkit_TextEditor.exe", "Network_Toolkit_Terminal.exe"} {
			if err := os.WriteFile(filepath.Join(dir, name), []byte("older "+name), 0700); err != nil {
				t.Fatal(err)
			}
		}
		calls := 0
		second := func(target, candidate, backup string) error {
			calls++
			if strings.HasSuffix(target, "Network_Toolkit_Terminal.exe") && strings.Contains(candidate, "candidate") {
				return errors.New("sharing violation")
			}
			return rename(target, candidate, backup)
		}
		if err := updateInstallCompanions(dir, programs, second); err == nil {
			t.Fatal("failure accepted")
		}
		for _, name := range []string{"Network_Toolkit_TextEditor.exe", "Network_Toolkit_Terminal.exe"} {
			if b, _ := os.ReadFile(filepath.Join(dir, name)); string(b) != "older "+name {
				t.Fatalf("%s was not put back: %q", name, b)
			}
		}
		entries, _ := os.ReadDir(dir)
		if len(entries) != 2 || calls != 3 {
			t.Fatalf("transaction files left: %v (%d replace calls)", entries, calls)
		}
	})
	t.Run("a package without one program places nothing", func(t *testing.T) {
		dir := t.TempDir()
		if err := updateInstallCompanions(dir, map[string][]byte{"Network_Toolkit_TextEditor.exe": programs["Network_Toolkit_TextEditor.exe"]}, rename); err == nil {
			t.Fatal("accepted")
		}
		if entries, _ := os.ReadDir(dir); len(entries) != 0 {
			t.Fatalf("placed: %v", entries)
		}
	})
}

// A copy updated by a released updater installs this version's separate
// programs from the version's own release: a synthetic GitHub (release
// metadata, Update ZIP and SHA256SUMS) on a local server.
// companionReleaseFixture serves this version's release metadata, Update ZIP
// and SHA256SUMS at their trusted GitHub URLs from a local server: every
// trusted host resolves to it, nothing leaves the machine.
type companionReleaseFixture struct {
	client    *http.Client
	programs  updateTestRelease
	name      string
	archive   []byte
	serve     []byte // the Update ZIP served (archive unless a test changes it)
	sums      []byte
	offline   bool // every request fails as without a connection
	requests  []string
	onRequest func() // called for each request before it is answered
}

func newCompanionReleaseFixture(t *testing.T) *companionReleaseFixture {
	t.Helper()
	f := &companionReleaseFixture{programs: updateTestPrograms(model.Version)}
	f.name, f.archive, f.sums = updateTestReleaseZip(t, model.Version, f.programs)
	f.serve = f.archive
	base := "https://github.com/" + updateRepository + "/releases/download/v" + model.Version + "/"
	metadata := fmt.Sprintf(`{"tag_name":"v%s","body":"synthetic","draft":false,"prerelease":true,"assets":[{"name":%q,"browser_download_url":%q,"size":%d},{"name":"SHA256SUMS","browser_download_url":%q,"size":%d}]}`, model.Version, f.name, base+f.name, len(f.archive), base+"SHA256SUMS", len(f.sums))
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		f.requests = append(f.requests, r.Host+r.URL.Path)
		if f.onRequest != nil {
			f.onRequest()
		}
		switch {
		case r.Host == "api.github.com" && r.URL.Path == "/repos/"+updateRepository+"/releases/tags/v"+model.Version:
			io.WriteString(w, metadata)
		case r.Host == "github.com" && strings.HasSuffix(r.URL.Path, "/"+f.name):
			w.Write(f.serve)
		case r.Host == "github.com" && strings.HasSuffix(r.URL.Path, "/SHA256SUMS"):
			w.Write(f.sums)
		default:
			http.NotFound(w, r)
		}
	})
	server := httptest.NewTLSServer(mux)
	t.Cleanup(server.Close)
	transport := server.Client().Transport.(*http.Transport).Clone()
	addr := server.Listener.Addr().String()
	transport.DialContext = func(ctx context.Context, network, _ string) (net.Conn, error) {
		if f.offline {
			return nil, errors.New("synthetic: no network connection")
		}
		return (&net.Dialer{}).DialContext(ctx, network, addr)
	}
	transport.TLSClientConfig.InsecureSkipVerify = true // the fixture's certificate is not GitHub's
	f.client = &http.Client{Transport: transport, CheckRedirect: updateHTTPClient().CheckRedirect}
	return f
}

func TestInstallCompanionsFromTheVersionsOwnRelease(t *testing.T) {
	fixture := newCompanionReleaseFixture(t)
	programs, name, archive, client := fixture.programs, fixture.name, fixture.archive, fixture.client
	install := func(t *testing.T, running []byte) (string, error) {
		dir := withRunningProgram(t, running)
		a := testApplication(t)
		a.updater.mu.Lock()
		defer a.updater.mu.Unlock()
		err := a.updateInstallCompanionsFromRelease(context.Background(), client, updateReleaseTagURL+model.Version)
		return dir, err
	}
	t.Run("the official build gets its separate programs", func(t *testing.T) {
		fixture.requests = nil
		dir, err := install(t, programs.main)
		if err != nil {
			t.Fatal(err)
		}
		for name, data := range map[string][]byte{"Network_Toolkit_TextEditor.exe": programs.editor, "Network_Toolkit_Terminal.exe": programs.terminal} {
			if b, err := os.ReadFile(filepath.Join(dir, name)); err != nil || !bytes.Equal(b, data) {
				t.Fatalf("%s: %v", name, err)
			}
		}
		if status := companionStatusFor(dir); status.Needed {
			t.Fatalf("still needed: %+v", status)
		}
		want := []string{"api.github.com/repos/" + updateRepository + "/releases/tags/v" + model.Version, "github.com/" + updateRepository + "/releases/download/v" + model.Version + "/SHA256SUMS", "github.com/" + updateRepository + "/releases/download/v" + model.Version + "/" + name}
		if strings.Join(fixture.requests, " ") != strings.Join(want, " ") {
			t.Fatalf("requests %v, want %v", fixture.requests, want)
		}
	})
	t.Run("another build of the main program gets nothing", func(t *testing.T) {
		other := append(updateTestProgramPE(model.Version, "Network_Toolkit.exe"), 0)
		dir, err := install(t, other)
		if err == nil || !strings.Contains(err.Error(), "not the official build") {
			t.Fatalf("want a refusal, got %v", err)
		}
		if entries, _ := os.ReadDir(dir); len(entries) != 1 {
			t.Fatalf("programs were added: %v", entries)
		}
	})
	t.Run("a changed download gets nothing", func(t *testing.T) {
		fixture.serve = append(append([]byte{}, archive...), 0)
		defer func() { fixture.serve = archive }()
		dir, err := install(t, programs.main)
		if err == nil || !strings.Contains(err.Error(), "Checksum") {
			t.Fatalf("want a checksum refusal, got %v", err)
		}
		if entries, _ := os.ReadDir(dir); len(entries) != 1 {
			t.Fatalf("programs were added: %v", entries)
		}
	})
}

func TestCompanionStatusReportsMissingAndOtherPrograms(t *testing.T) {
	dir := t.TempDir()
	status := companionStatusFor(dir)
	if !status.Needed || len(status.Programs) != 2 || status.Programs[0].State != "missing" || status.Programs[1].State != "missing" {
		t.Fatalf("%+v", status)
	}
	original := separateProgramMatches
	t.Cleanup(func() { separateProgramMatches = original })
	separateProgramMatches = func(path, edition string) bool { return edition == "terminal" }
	for _, name := range []string{"Network_Toolkit_TextEditor.exe", "Network_Toolkit_Terminal.exe"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("x"), 0700); err != nil {
			t.Fatal(err)
		}
	}
	status = companionStatusFor(dir)
	if !status.Needed || status.Programs[0].State != "other" || status.Programs[1].State != "current" {
		t.Fatalf("%+v", status)
	}
	separateProgramMatches = func(string, string) bool { return true }
	if companionStatusFor(dir).Needed {
		t.Fatal("current programs reported as needed")
	}
}

func TestPlanWithSeparateProgramsIsValidatedStrictly(t *testing.T) {
	stage, err := os.MkdirTemp(t.TempDir(), ".network-toolkit-update-")
	if err != nil {
		t.Fatal(err)
	}
	target := t.TempDir()
	digest := strings.Repeat("a", 64)
	write := func(companions []updatePlanCompanion, targetDir string) error {
		p := updatePlan{Target: "Network_Toolkit.exe", TargetDir: targetDir, WorkingDir: target, OriginalSHA256: digest, CandidateSHA256: digest, Version: "1.21.45", ParentPID: 1, Nonce: digest, Companions: companions}
		b, _ := json.Marshal(p)
		os.Remove(filepath.Join(stage, "plan.json"))
		if err := os.WriteFile(filepath.Join(stage, "plan.json"), b, 0600); err != nil {
			t.Fatal(err)
		}
		_, err := updateReadPlan(stage)
		return err
	}
	ok := []updatePlanCompanion{{Target: "Network_Toolkit_TextEditor.exe", OriginalSHA256: digest, CandidateSHA256: digest}, {Target: "Network_Toolkit_Terminal.exe", CandidateSHA256: digest}}
	if err := write(ok, target); err != nil {
		t.Fatal(err)
	}
	for name, companions := range map[string][]updatePlanCompanion{
		"unknown program":       {{Target: "other.exe", CandidateSHA256: digest}},
		"the main program":      {{Target: "Network_Toolkit.exe", CandidateSHA256: digest}},
		"twice":                 {ok[0], ok[0]},
		"no candidate checksum": {{Target: "Network_Toolkit_Terminal.exe"}},
		"bad checksum":          {{Target: "Network_Toolkit_Terminal.exe", CandidateSHA256: "zz"}},
		"upper-case checksum":   {{Target: "Network_Toolkit_Terminal.exe", CandidateSHA256: strings.Repeat("A", 64)}},
		"path":                  {{Target: `..\Network_Toolkit_Terminal.exe`, CandidateSHA256: digest}},
		"three":                 {ok[0], ok[1], ok[1]},
	} {
		if err := write(companions, target); err == nil {
			t.Fatalf("%s accepted", name)
		}
	}
	if err := write(ok, ""); err == nil {
		t.Fatal("separate programs accepted in a plan without a target folder")
	}
}
