package desktop

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
	"unicode"
)

const windowStateFilename = "Network_Toolkit.window.json"

type windowState struct {
	Version   int  `json:"version"`
	Bounds    Rect `json:"bounds"`
	Maximized bool `json:"maximized"`
}

func validRect(r Rect) bool {
	// Use 64-bit arithmetic before subtraction. A corrupt state file must not
	// overflow a Win32 coordinate or request a huge backing surface.
	w, h := int64(r.Right)-int64(r.Left), int64(r.Bottom)-int64(r.Top)
	return w > 0 && h > 0 && w <= 65536 && h <= 65536
}

func readWindowState(filename string) (windowState, error) {
	f, err := os.Open(filename)
	if err != nil {
		return windowState{}, err
	}
	defer f.Close()
	b, err := io.ReadAll(io.LimitReader(f, 8193))
	if err != nil {
		return windowState{}, err
	}
	if len(b) > 8192 {
		return windowState{}, errors.New("window state is too large")
	}
	var state windowState
	if err := json.Unmarshal(b, &state); err != nil {
		return windowState{}, err
	}
	if state.Version != 1 || !validRect(state.Bounds) {
		return windowState{}, errors.New("window state has unsupported or invalid bounds")
	}
	return state, nil
}

func writeWindowState(filename string, state windowState) error {
	if !validRect(state.Bounds) {
		return errors.New("cannot save invalid window bounds")
	}
	state.Version = 1
	b, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	b = append(b, '\n')
	f, err := os.CreateTemp(filepath.Dir(filename), "Network_Toolkit.window-*.tmp")
	if err != nil {
		return err
	}
	tmp := f.Name()
	defer os.Remove(tmp)
	if _, err = f.Write(b); err != nil {
		f.Close()
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	// Go's Windows os.Rename uses MoveFileExW with REPLACE_EXISTING. The temp
	// file is beside the destination, so no partial JSON file becomes visible.
	return os.Rename(tmp, filename)
}

// fitWindowBounds keeps the entire restored frame in one actual monitor's work
// area. This also recovers saved positions after monitor removal or rearrangement.
func fitWindowBounds(saved, work Rect, minimumWidth, minimumHeight int32) Rect {
	if !validRect(work) {
		work = Rect{Right: 1280, Bottom: 800}
	}
	ww, wh := int64(work.Right)-int64(work.Left), int64(work.Bottom)-int64(work.Top)
	w, h := int64(saved.Right)-int64(saved.Left), int64(saved.Bottom)-int64(saved.Top)
	if !validRect(saved) {
		w, h = 1240, 820
	}
	w = min(max(w, int64(max(minimumWidth, 1))), ww)
	h = min(max(h, int64(max(minimumHeight, 1))), wh)
	x, y := int64(saved.Left), int64(saved.Top)
	// Center defaults and completely off-screen bounds in the selected monitor.
	if !validRect(saved) || saved.Right <= work.Left || saved.Left >= work.Right || saved.Bottom <= work.Top || saved.Top >= work.Bottom {
		x = int64(work.Left) + (ww-w)/2
		y = int64(work.Top) + (wh-h)/2
	}
	x = max(int64(work.Left), min(x, int64(work.Right)-w))
	y = max(int64(work.Top), min(y, int64(work.Bottom)-h))
	return Rect{int32(x), int32(y), int32(x + w), int32(y + h)}
}

var exportExtensions = map[string]string{
	".html": "HTML report", ".htm": "HTML report", ".json": "JSON report",
	".csv": "CSV report", ".txt": "Text report", ".log": "Text log",
	".pcap": "Packet capture", ".pcapng": "Packet capture", ".bin": "Packet bytes",
}

// safeDownloadName strips both Windows and URL path separators before cleaning a
// suggested export name. Suggestions never determine the directory or an
// executable extension; the user chooses the actual destination in Save As.
func safeDownloadName(suggested string) string {
	name := path.Base(strings.ReplaceAll(suggested, "\\", "/"))
	name = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) || strings.ContainsRune(`<>:"/\|?*`, r) {
			return '_'
		}
		return r
	}, name)
	name = strings.Trim(name, " .")
	ext := strings.ToLower(path.Ext(name))
	stem := strings.TrimSuffix(name, path.Ext(name))
	if _, allowed := exportExtensions[ext]; !allowed {
		ext = ".txt"
	}
	stem = strings.Trim(stem, " .")
	if stem == "" || stem == "_" {
		stem = "Network_Toolkit_Report"
	}
	// DOS device names remain reserved even when followed by an extension.
	device := strings.ToUpper(strings.SplitN(stem, ".", 2)[0])
	if isReservedDeviceName(device) {
		stem = "Network_Toolkit_" + stem
	}
	runes := []rune(stem)
	if len(runes) > 120 {
		stem = string(runes[:120])
	}
	return stem + ext
}

func isReservedDeviceName(name string) bool {
	switch name {
	case "CON", "PRN", "AUX", "NUL", "CLOCK$", "CONIN$", "CONOUT$":
		return true
	}
	for _, prefix := range []string{"COM", "LPT"} {
		if strings.HasPrefix(name, prefix) {
			suffix := strings.TrimPrefix(name, prefix)
			if strings.Contains("123456789¹²³", suffix) && len([]rune(suffix)) == 1 {
				return true
			}
		}
	}
	return false
}

func validateDownloadExtension(filename, extension string) error {
	base := path.Base(strings.ReplaceAll(filename, "\\", "/"))
	actual := strings.ToLower(path.Ext(base))
	if actual != extension && !(extension == ".html" && actual == ".htm") && !(extension == ".htm" && actual == ".html") {
		return fmt.Errorf("use a %s filename for this report", extension)
	}
	if strings.ContainsAny(base, `<>:"/\|?*`) || strings.TrimRight(base, " .") != base {
		return errors.New("choose a normal file name without reserved characters")
	}
	if isReservedDeviceName(strings.ToUpper(strings.SplitN(base, ".", 2)[0])) {
		return errors.New("choose a normal file name instead of a reserved device name")
	}
	for _, r := range base {
		if unicode.IsControl(r) {
			return errors.New("choose a file name without control characters")
		}
	}
	return nil
}
