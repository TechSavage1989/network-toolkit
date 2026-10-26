package main

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCredentialAPISeparatesSecretsFromSettings(t *testing.T) {
	a := testApplication(t)
	secret := "vault-secret-canary"
	body := `{"name":"Core switch","username":"admin","kind":"password","secret":"` + secret + `"}`
	saved := requestApp(a, http.MethodPost, "/api/credentials", body, true)
	if saved.Code != http.StatusOK {
		t.Fatalf("save credential: %d %s", saved.Code, saved.Body.String())
	}
	var response struct {
		Entry credentialMetadata `json:"entry"`
	}
	if err := json.Unmarshal(saved.Body.Bytes(), &response); err != nil || response.Entry.ID == "" {
		t.Fatalf("decode saved credential: %v %s", err, saved.Body.String())
	}
	listed := requestApp(a, http.MethodGet, "/api/credentials", "", true)
	if strings.Contains(listed.Body.String(), secret) {
		t.Fatal("credential list exposed the secret")
	}
	state := requestApp(a, http.MethodGet, "/api/state", "", true)
	if strings.Contains(state.Body.String(), secret) {
		t.Fatal("ordinary settings exposed the credential secret")
	}
	raw, err := os.ReadFile(a.dataPath)
	if err == nil && strings.Contains(string(raw), secret) {
		t.Fatal("ordinary settings file contains the credential secret")
	}
	fetched := requestApp(a, http.MethodGet, "/api/credentials/"+response.Entry.ID, "", true)
	if fetched.Code != http.StatusOK || !strings.Contains(fetched.Body.String(), secret) {
		t.Fatalf("explicit credential retrieval failed: %d %s", fetched.Code, fetched.Body.String())
	}
}

func TestCredentialIDPersistsWithoutTerminalSecrets(t *testing.T) {
	a := testApplication(t)
	a.dataPath = filepath.Join(filepath.Dir(a.dataPath), "Network_Toolkit.settings.json")
	body := `{"terminalProfiles":[{"id":"p1","name":"Core","type":"ssh","host":"192.0.2.10","username":"admin","credentialId":"credential-abc","password":"must-not-save","privateKey":"must-not-save","passphrase":"must-not-save"}]}`
	response := requestApp(a, http.MethodPut, "/api/state", body, true)
	if response.Code != http.StatusOK {
		t.Fatalf("save profile: %d %s", response.Code, response.Body.String())
	}
	raw, err := os.ReadFile(a.dataPath)
	if err != nil {
		t.Fatal(err)
	}
	text := string(raw)
	if !strings.Contains(text, "credential-abc") {
		t.Fatal("credential reference was not retained")
	}
	for _, forbidden := range []string{"must-not-save", "password", "privateKey", "passphrase"} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("terminal secret field survived settings normalization: %s", forbidden)
		}
	}
}
