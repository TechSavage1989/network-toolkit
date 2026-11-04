package main

import (
	"bytes"
	"context"
	"net/http"
	"os"
	"strings"
	"testing"

	"networktoolkit/internal/desktop"
)

// Owner request: the vault may be set up without a master password (random key
// protected by the Windows account, unlocked automatically) and switched both ways.
func TestVaultModesWithoutMasterPasswordRoundTrip(t *testing.T) {
	a := fixtureUnconfiguredVaultApp(t)
	prompts := []desktop.VaultSecretMode{}
	password := "short"
	a.promptVaultSecret = func(_ context.Context, req desktop.VaultSecretRequest) (desktop.VaultSecretResponse, error) {
		prompts = append(prompts, req.Mode)
		return desktop.VaultSecretResponse{Password: []byte(password)}, nil
	}
	confirmed := []desktop.VaultAction{}
	a.confirmVaultAction = func(_ context.Context, req desktop.VaultActionRequest) (bool, error) {
		confirmed = append(confirmed, req.Action)
		return true, nil
	}
	post := func(action string) *httpResult {
		r := requestApp(a, http.MethodPost, "/api/credential-vault", `{"action":"`+action+`"}`, true)
		return &httpResult{r.Code, r.Body.String()}
	}

	// Setup without a master password: native confirmation, no secret prompt.
	if r := post("setup-windows"); r.code != 200 || !strings.Contains(r.body, `"mode":"windows"`) || !strings.Contains(r.body, `"state":"unlocked"`) {
		t.Fatalf("setup without master password: %d %s", r.code, r.body)
	}
	if len(prompts) != 0 || len(confirmed) != 1 || confirmed[0] != desktop.VaultActionNoPassword {
		t.Fatalf("setup must only ask for native confirmation: prompts=%v confirmed=%v", prompts, confirmed)
	}
	if r := requestApp(a, http.MethodPost, "/api/credentials", `{"name":"Lab","username":"operator","kind":"password","secret":"fixture-secret"}`, true); r.Code != 200 {
		t.Fatalf("save in no-password vault: %d %s", r.Code, r.Body.String())
	}
	if raw, err := os.ReadFile(a.credentials.autoPath); err != nil || bytes.Contains(raw, []byte("fixture-secret")) {
		t.Fatal("automatic-unlock file missing or contains plaintext")
	}
	if raw, _ := os.ReadFile(a.credentials.path); bytes.Contains(raw, []byte("fixture-secret")) || bytes.Contains(raw, []byte("operator")) {
		t.Fatal("vault stored plaintext")
	}

	// Lock is refused (it would reopen automatically).
	if r := post("lock"); r.code != 409 {
		t.Fatalf("lock in no-password mode: %d %s", r.code, r.body)
	}

	// A fresh process opens the vault automatically on the first read.
	a.credentials = reloadFixtureVault(t, a.credentials)
	if a.credentials.status().State != "locked" {
		t.Fatal("fresh instance should start locked until the first read")
	}
	r := requestApp(a, http.MethodGet, "/api/credentials", "", true)
	if r.Code != 200 || !strings.Contains(r.Body.String(), `"name":"Lab"`) || !strings.Contains(r.Body.String(), `"username":"operator"`) {
		t.Fatalf("automatic unlock did not expose saved metadata: %s", r.Body.String())
	}

	// Add a short master password (any password is allowed).
	password = "abc"
	if r := post("set-password"); r.code != 200 || !strings.Contains(r.body, `"mode":"password"`) {
		t.Fatalf("add master password: %d %s", r.code, r.body)
	}
	if prompts[len(prompts)-1] != desktop.VaultSecretSetPassword {
		t.Fatal("add master password used the wrong native dialog")
	}
	if _, err := os.Stat(a.credentials.autoPath); !os.IsNotExist(err) {
		t.Fatal("automatic-unlock key was kept after adding a master password")
	}
	a.credentials = reloadFixtureVault(t, a.credentials)
	requestApp(a, http.MethodGet, "/api/credentials", "", true)
	if a.credentials.status().State != "locked" {
		t.Fatal("password vault must not unlock automatically")
	}
	if r := post("unlock"); r.code != 200 {
		t.Fatalf("unlock with the short master password: %d %s", r.code, r.body)
	}
	if _, ok := a.credentials.get(firstCredentialID(t, a)); !ok {
		t.Fatal("entry lost while switching modes")
	}

	// Remove the master password again: asks for the current one.
	if r := post("remove-password"); r.code != 200 || !strings.Contains(r.body, `"mode":"windows"`) || !strings.Contains(r.body, `"state":"unlocked"`) {
		t.Fatalf("remove master password: %d %s", r.code, r.body)
	}
	if prompts[len(prompts)-1] != desktop.VaultSecretRemovePassword {
		t.Fatal("remove master password used the wrong native dialog")
	}
	a.credentials = reloadFixtureVault(t, a.credentials)
	if r := requestApp(a, http.MethodGet, "/api/credentials", "", true); !strings.Contains(r.Body.String(), `"name":"Lab"`) {
		t.Fatal("entries not available after removing the master password")
	}

	// A wrong current password cannot remove the master password.
	password = "abc"
	if r := post("set-password"); r.code != 200 {
		t.Fatal(r.body)
	}
	password = "wrong"
	if r := post("remove-password"); r.code != 400 || !strings.Contains(r.body, `did not authenticate`) {
		t.Fatalf("wrong current password: %d %s", r.code, r.body)
	}
	if a.credentials.status().Mode != "password" {
		t.Fatal("failed removal changed the mode")
	}

	// Deleting the vault removes the automatic-unlock key too.
	password = "abc"
	if r := post("remove-password"); r.code != 200 {
		t.Fatal(r.body)
	}
	if r := post("delete-vault"); r.code != 200 {
		t.Fatal(r.body)
	}
	if _, err := os.Stat(a.credentials.autoPath); !os.IsNotExist(err) {
		t.Fatal("automatic-unlock key survived vault deletion")
	}
}

func TestVaultNoPasswordSetupCancelledChangesNothing(t *testing.T) {
	a := fixtureUnconfiguredVaultApp(t)
	a.confirmVaultAction = func(context.Context, desktop.VaultActionRequest) (bool, error) { return false, nil }
	r := requestApp(a, http.MethodPost, "/api/credential-vault", `{"action":"setup-windows"}`, true)
	if r.Code != 400 || a.credentials.status().State != "setup-required" {
		t.Fatalf("cancelled setup changed state: %d %s", r.Code, r.Body.String())
	}
	for _, path := range []string{a.credentials.path, a.credentials.autoPath} {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatal("cancelled setup wrote", path)
		}
	}
}

type httpResult struct {
	code int
	body string
}

func firstCredentialID(t *testing.T, a *application) string {
	t.Helper()
	list := a.credentials.list()
	if len(list) == 0 {
		t.Fatal("no credentials")
	}
	return list[0].ID
}

// If the vault itself cannot be written, the freshly written automatic-unlock
// key is rolled back so no orphaned key or half-configured mode remains.
func TestVaultNoPasswordSetupRollsBackKeyWhenVaultWriteFails(t *testing.T) {
	a := fixtureUnconfiguredVaultApp(t)
	// Another writer created the vault file after this instance loaded.
	if err := os.WriteFile(a.credentials.path, []byte("concurrent"), 0600); err != nil {
		t.Fatal(err)
	}
	r := requestApp(a, http.MethodPost, "/api/credential-vault", `{"action":"setup-windows"}`, true)
	if r.Code == 200 {
		t.Fatal("setup succeeded over an existing vault file")
	}
	if _, err := os.Stat(a.credentials.autoPath); !os.IsNotExist(err) {
		t.Fatal("automatic-unlock key left behind after a failed vault write")
	}
	if raw, _ := os.ReadFile(a.credentials.path); string(raw) != "concurrent" {
		t.Fatal("existing vault file was changed")
	}
}
