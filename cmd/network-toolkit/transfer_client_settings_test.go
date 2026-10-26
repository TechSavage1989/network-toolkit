package main

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"testing"
)

func TestTransferClientLayoutPersistsOnlyKnownPreferences(t *testing.T) {
	state := map[string]any{"transferClientLayout": map[string]any{
		"sftp": map[string]any{"width": float64(350), "activityHeight": float64(340), "collapsed": true, "tab": "recent", "side": "right", "password": "never-save"},
		"ftp":  map[string]any{"width": float64(10000), "activityHeight": float64(-5), "tab": "bad", "side": "diagonal"},
		"tftp": map[string]any{"width": float64(300), "activityHeight": float64(240), "tab": "saved"},
		"bad":  map[string]any{"password": "never-save"},
	}}
	sanitizeTransferClientSettings(state)
	rows := state["transferClientLayout"].(map[string]any)
	if len(rows) != 3 {
		t.Fatalf("unexpected protocols: %#v", rows)
	}
	for _, key := range []string{"sftp", "ftp", "tftp"} {
		if len(rows[key].(map[string]any)) != 5 {
			t.Fatalf("untrusted layout fields: %#v", rows[key])
		}
	}
	saved := rows["sftp"].(map[string]any)
	if saved["width"] != 350 || saved["activityHeight"] != 340 || saved["tab"] != "recent" || saved["collapsed"] != true || saved["side"] != "right" {
		t.Fatalf("preferences lost: %#v", saved)
	}
	if rows["tftp"].(map[string]any)["side"] != "left" {
		t.Fatalf("missing docking preference did not default left: %#v", rows["tftp"])
	}
	invalid := rows["ftp"].(map[string]any)
	if invalid["width"] != 300 || invalid["activityHeight"] != 0 || invalid["tab"] != "settings" || invalid["side"] != "left" {
		t.Fatalf("invalid preferences accepted: %#v", invalid)
	}
}

func TestTransferClientMetadataPersistsWithoutSecrets(t *testing.T) {
	a := testApplication(t)
	a.dataPath = filepath.Join(t.TempDir(), "Network_Toolkit.settings.json")
	const pin = "SHA256:abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNO123"
	body := `{"terminalAppearance":{"scheme":"pastelLavender","fontFamily":"Consolas","colors":{"foreground":"#AaBbCc"}},"terminalSidebarTab":"files","sftpRecent":[{"id":"recent1","name":"SFTP router","type":"ssh","host":"router.lan","username":"admin","port":22,"authMethod":"password","credentialId":"vault-1","fingerprint":"` + pin + `","lastUsed":123456,"password":"do-not-save","privateKey":"do-not-save"}],"sftpVerifiedHostKeys":{"router.lan:22":"` + pin + `","bad":"do-not-save"},"tftpProfiles":[{"id":"tftp1","name":"Core config","host":"core.lan","port":69,"operation":"download","remotePath":"startup.cfg","localPath":"C:\\Backups\\startup.cfg","blockSize":1428,"timeoutSec":3,"retries":5,"overwrite":false,"password":"do-not-save"}],"tftpRecent":[{"id":"tr1","name":"Recent","host":"192.0.2.4","port":69,"operation":"upload","remotePath":"config","lastUsed":9876,"secret":"do-not-save"}]}`
	response := requestApp(a, http.MethodPut, "/api/state", body, true)
	if response.Code != http.StatusOK {
		t.Fatalf("save: %d %s", response.Code, response.Body.String())
	}
	raw, err := os.ReadFile(a.dataPath)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(raw, []byte("do-not-save")) {
		t.Fatal("unrecognized secret retained in settings")
	}
	var state map[string]any
	if err = json.Unmarshal(raw, &state); err != nil {
		t.Fatal(err)
	}
	appearance := state["terminalAppearance"].(map[string]any)
	if appearance["scheme"] != "pastelLavender" || appearance["colors"].(map[string]any)["foreground"] != "#aabbcc" {
		t.Fatalf("palette did not persist: %#v", appearance)
	}
	if state["terminalSidebarTab"] != "files" {
		t.Fatal("SFTP sidebar tab lost on save")
	}
	sftp := state["sftpRecent"].([]any)[0].(map[string]any)
	if sftp["credentialId"] != "vault-1" || sftp["fingerprint"] != pin {
		t.Fatalf("recent SFTP credential link / pin lost: %#v", sftp)
	}
	keys := state["sftpVerifiedHostKeys"].(map[string]any)
	if len(keys) != 1 || keys["router.lan:22"] != pin {
		t.Fatalf("incorrect verified host key map: %#v", keys)
	}
	tftp := state["tftpProfiles"].([]any)[0].(map[string]any)
	if tftp["remotePath"] != "startup.cfg" || tftp["operation"] != "download" {
		t.Fatalf("TFTP profile lost: %#v", tftp)
	}
	if state["tftpRecent"].([]any)[0].(map[string]any)["operation"] != "upload" {
		t.Fatal("TFTP recent entry lost")
	}
	reloaded := newApplication(context.Background(), a.dataPath)
	defer reloaded.stop()
	var loaded map[string]any
	if err = json.Unmarshal(reloaded.state, &loaded); err != nil {
		t.Fatal(err)
	}
	if loaded["terminalAppearance"].(map[string]any)["scheme"] != "pastelLavender" {
		t.Fatal("preset changed during reload")
	}
	if len(loaded["sftpRecent"].([]any)) != 1 || len(loaded["tftpRecent"].([]any)) != 1 {
		t.Fatal("recent connections changed during reload")
	}
}

func TestTransferClientMetadataRejectsMalformedImports(t *testing.T) {
	data := map[string]any{
		"sftpRecent":           []any{map[string]any{"id": "r1", "name": "Recent", "host": "switch", "port": float64(22), "username": "root", "password": "unsafe", "credentialId": "../secret", "fingerprint": "fake"}, map[string]any{"id": "r1", "name": "Duplicate", "host": "switch", "port": float64(22), "username": "root"}},
		"tftpProfiles":         []any{map[string]any{"id": "x", "name": "Bad", "host": "example", "port": float64(99999)}, map[string]any{"id": "a", "name": "Okay", "host": "backup-host", "port": float64(69), "operation": "evil", "retries": float64(1e9), "password": "bad"}},
		"sftpVerifiedHostKeys": map[string]any{"bad": "fingerprint", "router.lan:22": "SHA256:abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNO123"},
	}
	sanitizeTransferClientSettings(data)
	rows := data["sftpRecent"].([]map[string]any)
	if len(rows) != 1 || rows[0]["credentialId"] != "" || rows[0]["fingerprint"] != "" {
		t.Fatalf("SFTP unsafe metadata accepted: %#v", rows)
	}
	if _, ok := rows[0]["password"]; ok {
		t.Fatal("SFTP password was persisted")
	}
	tftp := data["tftpProfiles"].([]map[string]any)
	if len(tftp) != 1 || tftp[0]["operation"] != "download" || tftp[0]["retries"] != 5 {
		t.Fatalf("malformed TFTP fields accepted: %#v", tftp)
	}
	if _, ok := tftp[0]["password"]; ok {
		t.Fatal("TFTP password was persisted")
	}
}

func TestEveryTerminalColorPresetSurvivesSanitization(t *testing.T) {
	for _, scheme := range []string{"app", "dark", "light", "mobaCurrent", "pastelDusk", "pastelMint", "pastelLavender", "rosePine", "catppuccin", "nord", "tokyoNight", "gruvbox", "dracula", "solarizedDark", "solarizedLight", "campbell", "custom"} {
		state := map[string]any{"terminalAppearance": map[string]any{"scheme": scheme, "colors": map[string]any{"foreground": "#112233"}}}
		sanitizeTerminalSettings(state)
		appearance := state["terminalAppearance"].(map[string]any)
		if appearance["scheme"] != scheme {
			t.Errorf("theme %s changed to %v", scheme, appearance["scheme"])
		}
	}
}

func TestServerSidebarSettingsPersistWithoutUntrustedKeys(t *testing.T) {
	state := map[string]any{
		"transferServerSidebar": map[string]any{
			"sftp":    map[string]any{"width": float64(355), "activityHeight": float64(420), "collapsed": true, "password": "not-a-setting"},
			"ftp":     map[string]any{"width": float64(410), "activityHeight": float64(180), "collapsed": false, "privateKey": "not-a-setting"},
			"tftp":    map[string]any{"width": float64(20000), "activityHeight": "bad", "collapsed": "yes"},
			"invalid": map[string]any{"secret": "not-a-setting"},
		},
	}
	sanitizeTransferClientSettings(state)
	rows, ok := state["transferServerSidebar"].(map[string]any)
	if !ok {
		t.Fatal("no sanitized sidebar state")
	}
	if len(rows) != 3 {
		t.Fatalf("unexpected sidebar protocols: %#v", rows)
	}
	for _, protocol := range []string{"sftp", "tftp", "ftp"} {
		record, ok := rows[protocol].(map[string]any)
		if !ok {
			t.Fatalf("missing protocol %s", protocol)
		}
		if len(record) != 3 {
			t.Fatalf("unknown/secret fields persisted for %s: %#v", protocol, record)
		}
	}
	if rows["sftp"].(map[string]any)["collapsed"] != true || rows["sftp"].(map[string]any)["width"] != 355 {
		t.Fatalf("resizable SFTP pane was not retained: %#v", rows["sftp"])
	}
	if rows["ftp"].(map[string]any)["collapsed"] != false || rows["ftp"].(map[string]any)["width"] != 410 {
		t.Fatalf("FTP pane was not retained: %#v", rows["ftp"])
	}
	if rows["tftp"].(map[string]any)["collapsed"] != false || rows["tftp"].(map[string]any)["width"] != 320 {
		t.Fatalf("malformed TFTP layout was not normalized: %#v", rows["tftp"])
	}
	if rows["sftp"].(map[string]any)["activityHeight"] != 420 || rows["ftp"].(map[string]any)["activityHeight"] != 180 || rows["tftp"].(map[string]any)["activityHeight"] != 0 {
		t.Fatalf("activity height settings were not normalized: %#v", rows)
	}
}
