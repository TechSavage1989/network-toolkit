package desktop

import (
	"encoding/binary"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"unicode"
)

// Discovery layout and minimum bootstrap version are adapted from the Wails
// Go-native WebView2 loader, itself a port of OpenWebView2Loader. See
// LOADER_SOURCE.txt and LOADER_LICENSE.txt for the pinned source and ISC notice.
var minimumBootstrapVersion = [4]uint32{86, 0, 616, 0}

func parseRuntimeVersion(text string) ([4]uint32, error) {
	var version [4]uint32
	parts := strings.Split(text, ".")
	if len(parts) != len(version) {
		return version, errors.New("the runtime registration does not contain a four-part version")
	}
	for i, part := range parts {
		if part == "" {
			return version, errors.New("the runtime version has an empty component")
		}
		for _, c := range part {
			if c < '0' || c > '9' {
				return version, errors.New("the runtime version contains a nonnumeric component")
			}
		}
		n, err := strconv.ParseUint(part, 10, 32)
		if err != nil {
			return version, errors.New("the runtime version component is out of range")
		}
		version[i] = uint32(n)
	}
	return version, nil
}

func compareRuntimeVersion(a, b [4]uint32) int {
	for i := range a {
		if a[i] < b[i] {
			return -1
		}
		if a[i] > b[i] {
			return 1
		}
	}
	return 0
}

// installedRuntimePath accepts a literal local Windows installation directory.
// It never resolves relative paths, environment overrides, UNC shares, device
// namespaces, traversal components, or an executable beside the application.
// Keeping this path policy independent of host-OS filepath rules makes its
// Windows input cases testable on the Linux build host.
func installedRuntimePath(directory string) (dllPath, version string, err error) {
	directory = strings.ReplaceAll(directory, "/", "\\")
	directory = strings.TrimRight(directory, "\\")
	if len(directory) < 4 || directory[1] != ':' || directory[2] != '\\' ||
		!((directory[0] >= 'A' && directory[0] <= 'Z') || (directory[0] >= 'a' && directory[0] <= 'z')) {
		return "", "", errors.New("the runtime registration must name a local absolute Windows folder")
	}
	parts := strings.Split(directory[3:], "\\")
	for _, component := range parts {
		if component == "" || component == "." || component == ".." || strings.TrimRight(component, " .") != component || strings.ContainsAny(component, `<>:"|?*`) {
			return "", "", errors.New("the runtime registration contains an invalid path component")
		}
		for _, r := range component {
			if unicode.IsControl(r) {
				return "", "", errors.New("the runtime registration contains a control character")
			}
		}
	}
	version = parts[len(parts)-1]
	parsed, err := parseRuntimeVersion(version)
	if err != nil {
		return "", "", err
	}
	if compareRuntimeVersion(parsed, minimumBootstrapVersion) < 0 {
		return "", "", errors.New("the installed WebView2 Runtime is too old for this application")
	}
	return directory + `\EBWebView\x64\EmbeddedBrowserWebView.dll`, version, nil
}

// validateRuntimeDLL checks the regular file and minimal PE identity before
// Windows loads it. Windows remains responsible for loading the complete PE.
// This is an architecture/format check, not an Authenticode trust assertion.
func validateRuntimeDLL(filename string) error {
	info, err := os.Lstat(filename)
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return errors.New("the registered WebView2 Runtime DLL is not a regular file")
	}
	f, err := os.Open(filename)
	if err != nil {
		return err
	}
	defer f.Close()
	var dos [64]byte
	if _, err := f.ReadAt(dos[:], 0); err != nil {
		return fmt.Errorf("read the WebView2 Runtime header: %w", err)
	}
	if string(dos[:2]) != "MZ" {
		return errors.New("the registered WebView2 Runtime DLL has an invalid executable header")
	}
	offset := int64(binary.LittleEndian.Uint32(dos[60:]))
	if offset < 64 || offset > 1024*1024 || offset+26 > info.Size() {
		return errors.New("the registered WebView2 Runtime DLL has an invalid PE offset")
	}
	var pe [26]byte
	if _, err := f.ReadAt(pe[:], offset); err != nil {
		return fmt.Errorf("read the WebView2 Runtime PE header: %w", err)
	}
	if string(pe[:4]) != "PE\x00\x00" || binary.LittleEndian.Uint16(pe[24:]) != 0x20b {
		return errors.New("the registered WebView2 Runtime DLL is not a valid PE32+ image")
	}
	if binary.LittleEndian.Uint16(pe[4:]) != 0x8664 {
		return errors.New("the registered WebView2 Runtime DLL does not match this x64 application")
	}
	sections := binary.LittleEndian.Uint16(pe[6:])
	optionalSize := binary.LittleEndian.Uint16(pe[20:])
	flags := binary.LittleEndian.Uint16(pe[22:])
	if flags&0x2000 == 0 || optionalSize < 112 || sections == 0 || sections > 96 || offset+24+int64(optionalSize)+int64(sections)*40 > info.Size() {
		return errors.New("the registered WebView2 Runtime image is not a complete DLL")
	}
	return nil
}
