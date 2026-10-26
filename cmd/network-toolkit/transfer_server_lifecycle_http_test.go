package main

import (
	"encoding/json"
	"net"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
)

// The same authenticated endpoint used by the Start/Stop buttons must start
// real listeners and return state the UI can use. Run on loopback only.
func TestTransferServerStartStopThroughHTTP(t *testing.T) {
	app := testApplication(t)
	freeTCP := func() int {
		l, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Skipf("test host does not allow listening sockets: %v", err)
		}
		defer l.Close()
		return l.Addr().(*net.TCPAddr).Port
	}
	freeUDP := func() int {
		l, err := net.ListenPacket("udp", "127.0.0.1:0")
		if err != nil {
			t.Skipf("test host does not allow UDP sockets: %v", err)
		}
		defer l.Close()
		return l.LocalAddr().(*net.UDPAddr).Port
	}
	for _, tc := range []struct {
		protocol           string
		port               int
		username, password string
	}{
		{protocol: "tftp", port: freeUDP()},
		{protocol: "sftp", port: freeTCP(), username: "device-backups", password: "temporary-secret"},
	} {
		t.Run(tc.protocol, func(t *testing.T) {
			start, _ := json.Marshal(map[string]interface{}{
				"action": "start", "protocol": tc.protocol, "root": filepath.Join(t.TempDir(), "Backups"),
				"bind": "127.0.0.1", "port": tc.port, "access": "readwrite",
				"username": tc.username, "password": tc.password,
			})
			response := requestApp(app, http.MethodPost, "/api/transfer/server", string(start), true)
			if response.Code != http.StatusOK {
				t.Fatalf("start: %d %s", response.Code, response.Body.String())
			}
			if strings.Contains(response.Body.String(), tc.password) && tc.password != "" {
				t.Fatal("Server response exposed a password")
			}
			var status struct {
				Running bool   `json:"running"`
				Address string `json:"address"`
				Root    string `json:"root"`
			}
			if err := json.Unmarshal(response.Body.Bytes(), &status); err != nil {
				t.Fatal(err)
			}
			if !status.Running || status.Root == "" || status.Address == "" {
				t.Fatalf("Invalid start status: %+v", status)
			}
			get := requestApp(app, http.MethodGet, "/api/transfer/server?protocol="+tc.protocol, "", true)
			if get.Code != http.StatusOK || !strings.Contains(get.Body.String(), `"running":true`) {
				t.Fatalf("GET status: %d %s", get.Code, get.Body.String())
			}
			stop := requestApp(app, http.MethodPost, "/api/transfer/server", `{"action":"stop","protocol":"`+tc.protocol+`"}`, true)
			if stop.Code != http.StatusOK || !strings.Contains(stop.Body.String(), `"running":false`) {
				t.Fatalf("stop: %d %s", stop.Code, stop.Body.String())
			}
		})
	}
}
