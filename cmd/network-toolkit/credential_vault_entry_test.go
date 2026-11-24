package main

import (
	"net/http"
	"strings"
	"testing"
)

// Owner decision after v1.21.81: the master password is typed into the app's
// own Password manager form; without it the engine never opens a native box.
func TestVaultFormSecretsReplaceTheNativePasswordBox(t *testing.T) {
	a := fixtureUnconfiguredVaultApp(t)
	if a.promptVaultSecret != nil {
		t.Fatal("the fixture opens a native password box")
	}
	post := func(body string) (int, string) {
		r := requestApp(a, http.MethodPost, "/api/credential-vault", body, true)
		return r.Code, r.Body.String()
	}
	// No form: the page is asked for it and nothing changes.
	if code, body := post(`{"action":"setup"}`); code != http.StatusPreconditionRequired || !strings.Contains(body, `"code":"vault-password-required"`) {
		t.Fatalf("setup without the form: %d %s", code, body)
	}
	// The box's own rules: the confirmation must match; nothing typed is echoed.
	if code, body := post(`{"action":"setup","secret":{"password":"form fixture one","confirm":"form fixture two"}}`); code != 400 || !strings.Contains(body, "do not match") || strings.Contains(body, "fixture") {
		t.Fatalf("mismatch: %d %s", code, body)
	}
	if code, body := post(`{"action":"setup","secret":{"password":"form fixture","confirm":"form fixture","extra":"x"}}`); code != 400 {
		t.Fatalf("an unknown form field was accepted: %d %s", code, body)
	}
	if a.credentials.status().State == "unlocked" {
		t.Fatal("a refused form changed the vault")
	}
	if code, body := post(`{"action":"setup","secret":{"password":"` + syntheticMasterPassword + `","confirm":"` + syntheticMasterPassword + `","remember":true}}`); code != 200 || strings.Contains(body, syntheticMasterPassword) {
		t.Fatalf("setup: %d %s", code, body)
	}
	if s := a.credentials.status(); s.State != "unlocked" || !s.Remembered {
		t.Fatalf("setup state: %+v", s)
	}
	if code, body := post(`{"action":"forget"}`); code != 200 {
		t.Fatalf("forget: %d %s", code, body)
	}
	if code, _ := post(`{"action":"unlock","secret":{"password":"not the password"}}`); code == 200 || a.credentials.status().State != "locked" {
		t.Fatal("a wrong master password unlocked the vault")
	}
	if code, body := post(`{"action":"unlock","secret":{"password":"` + syntheticMasterPassword + `"}}`); code != 200 || a.credentials.status().State != "unlocked" {
		t.Fatalf("unlock: %d %s", code, body)
	}
	// Change: the current password is required and checked.
	if code, _ := post(`{"action":"change-password","secret":{"password":"next fixture","confirm":"next fixture"}}`); code != 400 {
		t.Fatal("change without the current password was accepted")
	}
	if code, body := post(`{"action":"change-password","secret":{"currentPassword":"` + syntheticMasterPassword + `","password":"next fixture","confirm":"next fixture"}}`); code != 200 {
		t.Fatalf("change: %d %s", code, body)
	}
	if code, body := post(`{"action":"remove-password","secret":{"password":"next fixture"}}`); code != 200 || a.credentials.status().Mode != "windows" {
		t.Fatalf("remove: %d %s %+v", code, body, a.credentials.status())
	}
}
