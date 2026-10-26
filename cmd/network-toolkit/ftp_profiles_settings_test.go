package main

import "testing"

func TestFTPProfilesNeverPersistPasswords(t *testing.T) {
	state := map[string]any{"ftpProfiles": []any{map[string]any{"id": "site1", "name": "Production FTPS", "host": "switch.example", "port": float64(990), "security": "implicit", "username": "user", "password": "forbidden-secret", "privateKey": "also forbidden", "credentialId": "credential-123"}}}
	sanitizeFTPProfiles(state)
	out := state["ftpProfiles"].([]map[string]any)
	if len(out) != 1 || out[0]["credentialId"] != "credential-123" || out[0]["security"] != "implicit" {
		t.Fatalf("profile metadata altered: %#v", out)
	}
	if _, ok := out[0]["password"]; ok {
		t.Fatal("plaintext password was kept")
	}
	if _, ok := out[0]["privateKey"]; ok {
		t.Fatal("private key was kept")
	}
}
func TestFTPProfilesSanitizeInvalidInput(t *testing.T) {
	state := map[string]any{"ftpProfiles": []any{map[string]any{"id": "a", "name": "Site", "host": "host", "port": float64(21), "username": "user", "security": "bogus", "credentialId": "../../secret"}, map[string]any{"id": "a", "name": "duplicate", "host": "host", "port": float64(21), "username": "user"}, map[string]any{"id": "bad", "name": "Bad", "host": "host", "port": float64(-1), "username": "user"}}}
	sanitizeFTPProfiles(state)
	items := state["ftpProfiles"].([]map[string]any)
	if len(items) != 1 || items[0]["security"] != "explicit" || items[0]["credentialId"] != "" {
		t.Fatalf("not sanitized: %#v", items)
	}
}
