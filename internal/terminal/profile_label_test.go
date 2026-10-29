package terminal

import (
	"context"
	"encoding/json"
	"runtime"
	"testing"
)

func TestSavedProfileDisplayNameAndFallback(t *testing.T) {
	for _, kind := range []string{"ssh", "telnet", "serial", "cmd", "powershell"} {
		t.Run(kind, func(t *testing.T) {
			serial := "/dev/ttyS0"
			if runtime.GOOS == "windows" {
				serial = "COM4"
			}
			req, err := normalizeConnectionRequest(ConnectRequest{Type: kind, DisplayName: "Core <lab>", ProfileID: "saved", Host: "192.0.2.8", Username: "operator", SerialPort: serial}, true)
			if err != nil {
				t.Fatal(err)
			}
			s := newLiveSession(context.Background(), "fixture", req)
			defer s.cancel()
			if s.info.Label != "Core <lab>" {
				t.Fatalf("saved name lost: %q", s.info.Label)
			}
			raw, err := json.Marshal(req)
			if err != nil {
				t.Fatal(err)
			}
			var decoded map[string]any
			if err = json.Unmarshal(raw, &decoded); err != nil {
				t.Fatal(err)
			}
			if _, ok := decoded["DisplayName"]; ok {
				t.Fatal("internal display name leaked into request JSON")
			}
			req.DisplayName = ""
			fallback := newLiveSession(context.Background(), "fallback", req)
			defer fallback.cancel()
			if fallback.info.Label == "" || fallback.info.Label == s.info.Label {
				t.Fatal("missing sensible fallback")
			}
		})
	}
}
