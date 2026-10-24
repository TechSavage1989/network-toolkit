package desktop

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInstalledRuntimePath(t *testing.T) {
	for _, directory := range []string{
		`C:\Program Files (x86)\Microsoft\EdgeWebView\Application\140.0.3485.81`,
		`C:\Users\张三\AppData\Local\Microsoft\EdgeWebView\Application\140.0.3485.81\`,
		`D:/Applications/Microsoft/EdgeWebView/Application/140.0.3485.81`,
	} {
		got, version, err := installedRuntimePath(directory)
		if err != nil {
			t.Errorf("valid registered path %q: %v", directory, err)
			continue
		}
		want := strings.TrimRight(strings.ReplaceAll(directory, "/", "\\"), "\\") + `\EBWebView\x64\EmbeddedBrowserWebView.dll`
		if got != want || version != "140.0.3485.81" {
			t.Errorf("path=%q version=%q; want path=%q", got, version, want)
		}
	}
	for _, directory := range []string{
		`140.0.3485.81`, `C:140.0.3485.81`, `\140.0.3485.81`,
		`\\server\share\140.0.3485.81`, `\\.\C:\140.0.3485.81`, `\\?\C:\140.0.3485.81`,
		`C:\Runtime\..\140.0.3485.81`, `C:\Runtime\.\140.0.3485.81`, `C:\Runtime\\140.0.3485.81`,
		`C:\Runtime:stream\140.0.3485.81`, `C:\Runtime \140.0.3485.81`,
		"C:\\Runtime\x00\\140.0.3485.81", `%LOCALAPPDATA%\Microsoft\140.0.3485.81`,
		`C:\Runtime\140.0.3485.81 beta`, `C:\Runtime\140.0.3485`, `C:\Runtime\85.0.564.63`,
	} {
		if _, _, err := installedRuntimePath(directory); err == nil {
			t.Errorf("invalid runtime path accepted: %q", directory)
		}
	}
}

func TestRuntimeVersionValidationAndOrder(t *testing.T) {
	for _, invalid := range []string{"", "1.2.3", "1.2.3.4.5", "140..0.1", "-1.2.3.4", "+1.2.3.4", "1.2.3. 4", "1.2.3.4 dev", "1.2.3.4294967296"} {
		if _, err := parseRuntimeVersion(invalid); err == nil {
			t.Errorf("invalid version accepted: %q", invalid)
		}
	}
	for _, pair := range []struct {
		version string
		order   int
	}{
		{"85.9.999.9", -1}, {"86.0.615.999", -1}, {"86.0.616.0", 0}, {"86.0.616.1", 1}, {"140.0.3485.81", 1},
	} {
		version, err := parseRuntimeVersion(pair.version)
		if err != nil || compareRuntimeVersion(version, minimumBootstrapVersion) != pair.order {
			t.Errorf("wrong version order for %s: %v", pair.version, err)
		}
	}
}

func TestRuntimeDLLIdentityValidation(t *testing.T) {
	valid := func() []byte {
		data := make([]byte, 512)
		copy(data, "MZ")
		binary.LittleEndian.PutUint32(data[60:], 64)
		copy(data[64:], "PE\x00\x00")
		binary.LittleEndian.PutUint16(data[68:], 0x8664)
		binary.LittleEndian.PutUint16(data[70:], 1)
		binary.LittleEndian.PutUint16(data[84:], 240)
		binary.LittleEndian.PutUint16(data[86:], 0x2022)
		binary.LittleEndian.PutUint16(data[88:], 0x20b)
		return data
	}
	file := filepath.Join(t.TempDir(), "runtime.dll")
	write := func(data []byte) {
		t.Helper()
		if err := os.WriteFile(file, data, 0600); err != nil {
			t.Fatal(err)
		}
	}
	write(valid())
	if err := validateRuntimeDLL(file); err != nil {
		t.Fatalf("valid minimal DLL identity rejected: %v", err)
	}
	for _, test := range []struct {
		name string
		edit func([]byte) []byte
	}{
		{"x86 architecture", func(b []byte) []byte { binary.LittleEndian.PutUint16(b[68:], 0x14c); return b }},
		{"ARM64 architecture", func(b []byte) []byte { binary.LittleEndian.PutUint16(b[68:], 0xaa64); return b }},
		{"EXE instead of DLL", func(b []byte) []byte { binary.LittleEndian.PutUint16(b[86:], 0x22); return b }},
		{"invalid DOS signature", func(b []byte) []byte { b[0] = 0; return b }},
		{"invalid PE signature", func(b []byte) []byte { b[64] = 0; return b }},
		{"bad PE offset", func(b []byte) []byte { binary.LittleEndian.PutUint32(b[60:], 0xffffffff); return b }},
		{"truncated headers", func(b []byte) []byte { return b[:90] }},
		{"no sections", func(b []byte) []byte { binary.LittleEndian.PutUint16(b[70:], 0); return b }},
	} {
		t.Run(test.name, func(t *testing.T) {
			write(test.edit(valid()))
			if err := validateRuntimeDLL(file); err == nil {
				t.Fatal("invalid DLL identity accepted")
			}
		})
	}
	if err := validateRuntimeDLL(filepath.Dir(file)); err == nil {
		t.Error("directory accepted as a runtime DLL")
	}
}
