package main

import (
	"archive/zip"
	"bytes"
	"debug/pe"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"networktoolkit/internal/model"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func updateTestPE(version string) []byte {
	resource := append(updateUTF16("Network Toolkit"), updateUTF16("NetworkToolkit")...)
	resource = append(resource, updateUTF16(version)...)
	b := make([]byte, 0x200)
	b[0] = 'M'
	b[1] = 'Z'
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
func updateTestPackage(t *testing.T, version string, extra map[string][]byte) (string, []byte, []byte) {
	t.Helper()
	root := "Network_Toolkit_" + version
	exe := root + "_Windows_x64.exe"
	binary := updateTestPE(version)
	entries := map[string][]byte{root + "/" + exe: binary, root + "/SHA256SUMS": []byte(fmt.Sprintf("%s  %s\n", updateHash(binary), exe))}
	for k, v := range extra {
		entries[k] = v
	}
	var b bytes.Buffer
	z := zip.NewWriter(&b)
	for name, data := range entries {
		w, e := z.Create(name)
		if e != nil {
			t.Fatal(e)
		}
		w.Write(data)
	}
	z.Close()
	name := root + "_Update.zip"
	return name, b.Bytes(), []byte(fmt.Sprintf("%s  %s\n", updateHash(b.Bytes()), name))
}
func TestUpdateVersions(t *testing.T) {
	for _, tc := range []struct {
		a, b string
		want bool
	}{{"1.17.59", "1.17.58", true}, {"2.0.0", "1.17.58", true}, {"1.17.58", "1.17.58", false}, {"1.17.9", "1.17.58", false}, {"01.18.0", "1.17.58", false}, {"1.18.0-beta", "1.17.58", false}, {"999999.1.1", "1.1.1", false}} {
		if got := updateNewer(tc.a, tc.b); got != tc.want {
			t.Errorf("%s > %s = %v", tc.a, tc.b, got)
		}
	}
}
func TestUpdatePackageValidation(t *testing.T) {
	name, b, s := updateTestPackage(t, "1.17.59", nil)
	v, exe, e := updateExtractPackage(name, b, s, "1.17.58")
	if e != nil || v != "1.17.59" || len(exe) == 0 {
		t.Fatalf("valid package: %s %v", v, e)
	}
	if _, _, e = updateExtractPackage(name, b, s, "1.17.59"); e == nil {
		t.Fatal("same version allowed")
	}
	if _, _, e = updateExtractPackage(name, append(b, 1), s, "1.17.58"); e == nil {
		t.Fatal("tampered archive allowed")
	}
	for _, bad := range []string{"../escape", "Network_Toolkit_1.17.59/../escape", "Network_Toolkit_1.17.59/C:evil", "Network_Toolkit_1.17.59/source\\evil"} {
		name, b, s = updateTestPackage(t, "1.17.59", map[string][]byte{bad: []byte("x")})
		if _, _, e = updateExtractPackage(name, b, s, "1.17.58"); e == nil {
			t.Errorf("unsafe path allowed: %s", bad)
		}
	}
}
func TestUpdateChecksumRejectsMissingDuplicateAndMalformed(t *testing.T) {
	b := []byte("hello")
	line := fmt.Sprintf("%s  file\n", updateHash(b))
	for _, s := range []string{"", line + line, "bad  file\n", strings.Repeat("g", 64) + "  file\n", fmt.Sprintf("%s  other\n", updateHash(b))} {
		if updateVerifySum([]byte(s), "file", b) == nil {
			t.Fatalf("accepted %q", s)
		}
	}
	if e := updateVerifySum([]byte(line), "file", b); e != nil {
		t.Fatal(e)
	}
}
func TestUpdatePERejectsWrongProductAndVersion(t *testing.T) {
	if e := updateValidatePE(updateTestPE("1.17.59"), "1.17.59"); e != nil {
		t.Fatal(e)
	}
	if updateValidatePE(updateTestPE("1.17.59"), "1.17.60") == nil {
		t.Fatal("wrong version accepted")
	}
	if updateValidatePE([]byte("MZbad"), "1.17.59") == nil {
		t.Fatal("bad PE accepted")
	}
}
func TestUpdateExclusiveWriteNeverOverwrites(t *testing.T) {
	name := filepath.Join(t.TempDir(), "x")
	if e := updateWriteExclusive(name, []byte("old"), 0600); e != nil {
		t.Fatal(e)
	}
	if updateWriteExclusive(name, []byte("new"), 0600) == nil {
		t.Fatal("overwrote existing")
	}
	b, _ := os.ReadFile(name)
	if string(b) != "old" {
		t.Fatal("changed original")
	}
}
func TestUpdateRestartArgsRemovePrivateHealth(t *testing.T) {
	got := updateRestartArgs([]string{"--data-dir", "C:/custom", "--update-health", "old", "--no-browser", "--update-health=old"})
	if strings.Join(got, "|") != "--data-dir|C:/custom|--no-browser" {
		t.Fatal(got)
	}
}
func TestUpdatePlanRejectsInvalidDirectory(t *testing.T) {
	if _, e := updateReadPlan(t.TempDir()); e == nil {
		t.Fatal("accepted arbitrary directory")
	}
	if _, e := updateReadPlan("relative"); e == nil {
		t.Fatal("accepted relative directory")
	}
}

func TestUpdateBusyActivities(t *testing.T) {
	a := &application{jobs: map[string]*Job{"x": {Status: "running"}}, localTools: &localToolsManager{busy: true}, transfers: &transferHostManager{servers: map[string]*transferHost{"sftp": {}}}, browserConnections: map[string]*browserFileConnection{"x": {}}}
	busy := strings.Join(a.updateBusy(), ",")
	for _, want := range []string{"Diagnostic jobs", "Local configuration change", "Transfer servers", "Remote file connections"} {
		if !strings.Contains(busy, want) {
			t.Errorf("missing %s in %s", want, busy)
		}
	}
	a.jobs["x"].Status = "completed"
	a.localTools.busy = false
	delete(a.transfers.servers, "sftp")
	delete(a.browserConnections, "x")
	if len(a.updateBusy()) != 0 {
		t.Fatal("idle app reported busy")
	}
}
func TestUpdateArchiveRejectsDuplicateMembers(t *testing.T) {
	name, b, _ := updateTestPackage(t, "1.17.59", nil)
	old, e := zip.NewReader(bytes.NewReader(b), int64(len(b)))
	if e != nil {
		t.Fatal(e)
	}
	var buf bytes.Buffer
	z := zip.NewWriter(&buf)
	for _, f := range old.File {
		r, _ := f.Open()
		data, _ := io.ReadAll(r)
		r.Close()
		w, _ := z.Create(f.Name)
		w.Write(data)
		w, _ = z.Create(f.Name)
		w.Write(data)
	}
	z.Close()
	sum := []byte(fmt.Sprintf("%s  %s\n", updateHash(buf.Bytes()), name))
	if _, _, e = updateExtractPackage(name, buf.Bytes(), sum, "1.17.58"); e == nil {
		t.Fatal("duplicate members allowed")
	}
}
func TestUpdatePlanRejectsTraversalTarget(t *testing.T) {
	dir, e := os.MkdirTemp(t.TempDir(), ".network-toolkit-update-")
	if e != nil {
		t.Fatal(e)
	}
	p := updatePlan{WorkingDir: t.TempDir(), Target: "../outside.exe", OriginalSHA256: strings.Repeat("a", 64), CandidateSHA256: strings.Repeat("b", 64), Version: "1.17.59", ParentPID: 1, Nonce: strings.Repeat("c", 64)}
	b, _ := json.Marshal(p)
	os.WriteFile(filepath.Join(dir, "plan.json"), b, 0600)
	if _, e = updateReadPlan(dir); e == nil {
		t.Fatal("traversal target allowed")
	}
	p.Target = "app.exe"
	b, _ = json.Marshal(p)
	os.WriteFile(filepath.Join(dir, "plan.json"), b, 0600)
	if _, e = updateReadPlan(dir); e != nil {
		t.Fatal(e)
	}
}

// Opt-in release smoke check against the actual compiled and packaged artifact.
func TestUpdateBuiltArtifact(t *testing.T) {
	filename := os.Getenv("NETWORK_TOOLKIT_UPDATE_TEST_PACKAGE")
	if filename == "" {
		t.Skip("No release package supplied")
	}
	archive, e := os.ReadFile(filename)
	if e != nil {
		t.Fatal(e)
	}
	sums, e := os.ReadFile(filepath.Join(filepath.Dir(filename), "SHA256SUMS"))
	if e != nil {
		t.Fatal(e)
	}
	version, binary, e := updateExtractPackage(filepath.Base(filename), archive, sums, "1.17.58")
	if e != nil {
		t.Fatal(e)
	}
	if version != model.Version || len(binary) < 1<<20 {
		t.Fatalf("Wrong actual build: %s, %d bytes", version, len(binary))
	}
}

func TestUpdateRestartPreservesRelativeDataDirectory(t *testing.T) {
	originalDir := t.TempDir()
	installDir := t.TempDir()
	target := filepath.Join(installDir, "toolkit.exe")
	p := updatePlan{WorkingDir: originalDir, Args: []string{"--data-dir", "relative settings", "--port", "0"}}
	for _, health := range []string{"", filepath.Join(installDir, ".network-toolkit-update-test")} {
		cmd := updateRestartCommand(target, p, health)
		if cmd.Dir != originalDir {
			t.Fatalf("restart changed working directory: %s", cmd.Dir)
		}
		if filepath.Join(cmd.Dir, cmd.Args[2]) != filepath.Join(originalDir, "relative settings") {
			t.Fatal("restart changed relative settings folder")
		}
		if len(p.Args) != 4 {
			t.Fatal("restart mutated saved arguments")
		}
	}
}
