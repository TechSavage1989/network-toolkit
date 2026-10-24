package desktop

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFitWindowBounds(t *testing.T) {
	tests := []struct {
		name        string
		saved, work Rect
		want        Rect
	}{
		{"keep visible", Rect{100, 100, 1100, 800}, Rect{0, 0, 1920, 1040}, Rect{100, 100, 1100, 800}},
		{"removed monitor", Rect{2100, 100, 3100, 800}, Rect{0, 0, 1920, 1040}, Rect{460, 170, 1460, 870}},
		{"negative monitor", Rect{-1700, 0, -500, 800}, Rect{-1920, 40, 0, 1080}, Rect{-1700, 40, -500, 840}},
		{"new smaller screen", Rect{200, 200, 2000, 1400}, Rect{0, 0, 1024, 728}, Rect{0, 0, 1024, 728}},
		{"minimum on tiny work area", Rect{10, 10, 20, 20}, Rect{0, 0, 640, 440}, Rect{0, 0, 640, 440}},
		{"corrupt integer bounds", Rect{-2147483648, 0, 2147483647, 800}, Rect{0, 0, 1920, 1040}, Rect{340, 110, 1580, 930}},
		{"defaults", Rect{}, Rect{0, 40, 1920, 1080}, Rect{340, 150, 1580, 970}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := fitWindowBounds(tt.saved, tt.work, 720, 520)
			if got != tt.want {
				t.Fatalf("bounds = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestWindowStateRoundTripAndReplacement(t *testing.T) {
	file := filepath.Join(t.TempDir(), windowStateFilename)
	state := windowState{Version: 1, Bounds: Rect{-1700, 100, -500, 900}, Maximized: true}
	for _, maximized := range []bool{true, false} {
		state.Maximized = maximized
		if err := writeWindowState(file, state); err != nil {
			t.Fatal(err)
		}
		got, err := readWindowState(file)
		if err != nil || got != state {
			t.Fatalf("state = %+v, %v; want %+v", got, err, state)
		}
	}
	entries, err := os.ReadDir(filepath.Dir(file))
	if err != nil || len(entries) != 1 {
		t.Fatalf("temporary state files remain: %v, %v", entries, err)
	}
}

func TestWindowStateRejectsCorruption(t *testing.T) {
	file := filepath.Join(t.TempDir(), windowStateFilename)
	for _, data := range []string{
		`{"version":2,"bounds":{"left":0,"top":0,"right":1000,"bottom":800}}`,
		`{"version":1,"bounds":{"left":50,"top":0,"right":1,"bottom":800}}`,
		`{"version":1,"bounds":{"left":-2147483648,"top":0,"right":2147483647,"bottom":800}}`,
		`{`, strings.Repeat(" ", 8193),
	} {
		if err := os.WriteFile(file, []byte(data), 0600); err != nil {
			t.Fatal(err)
		}
		if _, err := readWindowState(file); err == nil {
			t.Fatal("corrupt or oversized state was accepted")
		}
	}
}

func TestSafeDownloadName(t *testing.T) {
	tests := map[string]string{
		`C:\private\reports\report.JSON`: "report.json",
		`../../results.csv`:              "results.csv",
		`\\server\share\report.html`:     "report.html",
		"bad\x00name.exe":                "bad_name.txt",
		"CON.json":                       "Network_Toolkit_CON.json",
		"LPT¹.log":                       "Network_Toolkit_LPT¹.log",
		"report:stream.csv":              "report_stream.csv",
		"":                               "Network_Toolkit_Report.txt",
		"...":                            "Network_Toolkit_Report.txt",
		" report.html. ":                 "report.html",
		"报告.json":                        "报告.json",
		"capture.PCAPNG":                 "capture.pcapng",
		"capture.pcap":                   "capture.pcap",
		"Packet_1.bin":                   "Packet_1.bin",
	}
	for input, expected := range tests {
		if got := safeDownloadName(input); got != expected {
			t.Errorf("safeDownloadName(%q) = %q, want %q", input, got, expected)
		}
	}
	long := safeDownloadName(strings.Repeat("x", 500) + ".json")
	if len(long) > 125 || !strings.HasSuffix(long, ".json") {
		t.Fatalf("suggested name not bounded: %d", len(long))
	}
}

func TestDownloadExtensionMatchesReport(t *testing.T) {
	for _, filename := range []string{`C:\Reports\scan.JSON`, `\\server\share\scan.json`} {
		if err := validateDownloadExtension(filename, ".json"); err != nil {
			t.Errorf("valid path %q: %v", filename, err)
		}
	}
	for _, filename := range []string{`C:\Reports\scan.exe`, `C:\Reports\scan.json:payload`, `C:\Reports\NUL.json`, "bad\x00name.json", "report.json. ", "scan.csv"} {
		if err := validateDownloadExtension(filename, ".json"); err == nil {
			t.Errorf("invalid report name accepted: %q", filename)
		}
	}
	if err := validateDownloadExtension("report.htm", ".html"); err != nil {
		t.Errorf("equivalent HTML extension rejected: %v", err)
	}
}
