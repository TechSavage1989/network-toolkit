package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"networktoolkit/internal/desktop"
)

// sharedVaultApps returns a main-window app and a standalone Terminal app on
// one data folder, both using the folder's one vault through separate vault
// objects, as two processes do. Both are unlocked with the master password.
func sharedVaultApps(t *testing.T) (root string, main, terminal *application) {
	t.Helper()
	root = t.TempDir()
	main = sharedTestApp(t, root, "", "main")
	terminal = sharedTestApp(t, root, terminalWorkspaceName, "terminal")
	vault := syntheticCredentialVault(t, root)
	main.credentials = vault
	other := reloadFixtureVault(t, vault)
	if err := other.unlock(context.Background(), []byte(syntheticMasterPassword)); err != nil {
		t.Fatal(err)
	}
	bindFixtureVault(t, other)
	terminal.credentials = other
	for _, app := range []*application{main, terminal} {
		app.confirmCredentialDestination = func(context.Context, desktop.CredentialDestinationConfirmation) (bool, error) { return true, nil }
		app.confirmVaultAction = func(context.Context, desktop.VaultActionRequest) (bool, error) { return true, nil }
		if app.sharedSettingsPath() != vault.configPath {
			t.Fatalf("%s binds the vault to %s, not to the data folder's settings %s", app.windowMode, app.sharedSettingsPath(), vault.configPath)
		}
	}
	return root, main, terminal
}

func credentialNames(v *credentialVault) string {
	names := []string{}
	for _, item := range v.list() {
		names = append(names, item.Name)
	}
	return strings.Join(names, ",")
}

func TestSharedVaultAppsSeeEachOthersCredentialsWithoutRestart(t *testing.T) {
	_, main, terminal := sharedVaultApps(t)
	// A credential saved in the Terminal is usable in the main window at
	// once: same vault, same master password, no new prompt.
	saved, err := terminal.credentials.put(credentialRecord{Name: "From Terminal", Username: "operator", Kind: "password", Secret: "fixture-secret-one"})
	if err != nil {
		t.Fatal(err)
	}
	if status := main.credentials.status(); status.State != "unlocked" || status.Warning != "" {
		t.Fatalf("the main window's vault after a save in the Terminal: %+v", status)
	}
	if got := credentialNames(main.credentials); got != "From Terminal" {
		t.Fatalf("the main window lists %q", got)
	}
	if record, ok := main.credentials.get(saved.ID); !ok || record.Secret != "fixture-secret-one" {
		t.Fatal("the main window cannot use the credential saved in the Terminal")
	}
	// Both directions, and neither save overwrites the other.
	if _, err := main.credentials.put(credentialRecord{Name: "From main", Kind: "password", Secret: "fixture-secret-two"}); err != nil {
		t.Fatal(err)
	}
	if _, err := terminal.credentials.put(credentialRecord{Name: "Second from Terminal", Kind: "password", Secret: "fixture-secret-three"}); err != nil {
		t.Fatal(err)
	}
	for _, app := range []*application{main, terminal} {
		if got := credentialNames(app.credentials); got != "From main,From Terminal,Second from Terminal" {
			t.Fatalf("%s lists %q", app.windowMode, got)
		}
	}
	// A page learns that the vault changed from the mark it polls.
	mark := func(a *application) string {
		var polled struct {
			Credentials string `json:"credentials"`
		}
		r := requestApp(a, "GET", "/api/shared-data", "", true)
		if r.Code != 200 || json.Unmarshal(r.Body.Bytes(), &polled) != nil || polled.Credentials == "" {
			t.Fatalf("poll: %d %s", r.Code, r.Body.String())
		}
		return polled.Credentials
	}
	before := mark(main)
	if before != mark(main) {
		t.Fatal("the vault mark changed without a change")
	}
	if err := terminal.credentials.delete(saved.ID); err != nil {
		t.Fatal(err)
	}
	if mark(main) == before {
		t.Fatal("a removal in the Terminal did not change the main window's vault mark")
	}
	if got := credentialNames(main.credentials); got != "From main,Second from Terminal" {
		t.Fatalf("the main window after a removal in the Terminal: %q", got)
	}
	// The stored file never holds a secret in the clear.
	raw, err := os.ReadFile(main.credentials.path)
	if err != nil || bytes.Contains(raw, []byte("fixture-secret")) {
		t.Fatal("the shared vault file holds a plain secret")
	}
}

func TestSharedVaultChangedMasterPasswordLocksOtherAppsUntilUnlocked(t *testing.T) {
	_, main, terminal := sharedVaultApps(t)
	if _, err := main.credentials.put(credentialRecord{Name: "Before", Kind: "password", Secret: "fixture-secret"}); err != nil {
		t.Fatal(err)
	}
	// The master password is changed in the main window. The Terminal's key
	// no longer opens the vault: it is locked there, as after a restart, and
	// only the new password unlocks it. Nothing is guessed or carried over.
	const changed = "changed synthetic master password"
	if err := main.credentials.rotate(context.Background(), []byte(syntheticMasterPassword), []byte(changed), "", false, nil); err != nil {
		t.Fatal(err)
	}
	bindFixtureVault(t, main.credentials)
	status := terminal.credentials.status()
	if status.State != "locked" || !strings.Contains(status.Warning, "Unlock it again") || len(terminal.credentials.list()) != 0 {
		t.Fatalf("the Terminal kept access after the master password changed: %+v", status)
	}
	if _, ok := terminal.credentials.get("anything"); ok {
		t.Fatal("a locked vault returned a credential")
	}
	if err := terminal.credentials.unlock(context.Background(), []byte(syntheticMasterPassword)); err == nil {
		t.Fatal("the old master password still unlocked the vault")
	}
	if err := terminal.credentials.unlock(context.Background(), []byte(changed)); err != nil {
		t.Fatal(err)
	}
	bindFixtureVault(t, terminal.credentials)
	if got := credentialNames(terminal.credentials); got != "Before" {
		t.Fatalf("the Terminal after unlocking with the new password: %q", got)
	}
	// Lock and forget in one app locks the others too.
	if err := main.credentials.lockAndForget(); err != nil {
		t.Fatal(err)
	}
	if terminal.credentials.status().State != "locked" {
		t.Fatal("Lock and forget in the main window left the Terminal unlocked")
	}
	// A deleted vault is not "locked with stale data": it needs setup again.
	if err := main.credentials.destroy(); err != nil {
		t.Fatal(err)
	}
	if state := terminal.credentials.status().State; state != "setup-required" {
		t.Fatalf("the Terminal after the vault was deleted: %s", state)
	}
}

// previousTerminalVault creates the vault an earlier version kept for the
// standalone Terminal, with its own master password, and returns its file.
func previousTerminalVault(t *testing.T, root string, protect, unprotect func([]byte) ([]byte, error), password string, records ...credentialRecord) string {
	t.Helper()
	dir := filepath.Join(root, terminalWorkspaceName)
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	old, err := newCredentialVaultWithProtection(dir, protect, unprotect)
	if err != nil {
		t.Fatal(err)
	}
	if password == "" {
		err = old.setupWithoutPassword(context.Background())
	} else {
		err = old.setup(context.Background(), []byte(password), false, nil)
	}
	if err != nil {
		t.Fatal(err)
	}
	bindFixtureVault(t, old)
	for _, record := range records {
		clean, err := sanitizeCredentialRecord(record, record.ID)
		if err != nil {
			t.Fatal(err)
		}
		old.mu.Lock()
		old.entries[clean.ID] = clean
		err = old.saveLocked()
		old.mu.Unlock()
		if err != nil {
			t.Fatal(err)
		}
	}
	old.close()
	return old.path
}

func TestSharedVaultImportFromPreviousTerminalVault(t *testing.T) {
	root, main, terminal := sharedVaultApps(t)
	binding := credentialBinding{Protocol: "ssh", Host: "192.0.2.22", Port: 2222, Username: "operator", Fingerprint: "SHA256:AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"}
	// The shared vault already has a credential, and one whose id the
	// Terminal's vault uses for a different credential.
	if _, err := main.credentials.put(credentialRecord{ID: "cred-shared", Name: "Same in both", Username: "admin", Kind: "password", Secret: "same-secret"}); err != nil {
		t.Fatal(err)
	}
	if _, err := main.credentials.put(credentialRecord{ID: "cred-taken", Name: "Main window's", Kind: "password", Secret: "main-secret"}); err != nil {
		t.Fatal(err)
	}
	const oldPassword = "previous terminal vault password"
	oldPath := previousTerminalVault(t, root, main.credentials.protect, main.credentials.unprotect, oldPassword,
		credentialRecord{ID: "cred-shared", Name: "Same in both", Username: "admin", Kind: "password", Secret: "same-secret"},
		credentialRecord{ID: "cred-terminal", Name: "Terminal only", Username: "operator", Kind: "password", Secret: "terminal-secret", Bindings: []credentialBinding{binding}},
		credentialRecord{ID: "cred-taken", Name: "Terminal's", Username: "operator", Kind: "password", Secret: "other-secret"})
	oldBefore, err := os.ReadFile(oldPath)
	if err != nil {
		t.Fatal(err)
	}
	// Saved connections: two came from the Terminal workspace with the
	// migration, one is the main window's own and names its own credential.
	page := openSharedPage(t, main)
	for _, row := range []struct{ id, credential string }{{"migrated-a", "cred-terminal"}, {"migrated-b", "cred-taken"}, {"main-own", "cred-taken"}} {
		profile := sharedProfile(row.id, row.id, "192.0.2.22")
		profile["credentialId"] = row.credential
		page.add("terminalProfiles", profile)
	}
	page.save()
	marker := &sharedMarker{Version: 1, Sources: map[string]*sharedMigrationSource{}, Credentials: []sharedMigratedCredential{{Source: "terminal", Key: "terminalProfiles", ID: "migrated-a", CredentialID: "cred-terminal"}, {Source: "terminal", Key: "terminalProfiles", ID: "migrated-b", CredentialID: "cred-taken"}}}
	if err := writeSharedMarker(root, marker); err != nil {
		t.Fatal(err)
	}

	// Until imported, the password manager offers the import and a missing
	// credential says where it may be.
	listed := requestApp(terminal, "GET", "/api/credentials", "", true)
	if listed.Code != 200 || !strings.Contains(listed.Body.String(), `"previousVaults":[{"source":"terminal","label":"Terminal","imported":false}]`) {
		t.Fatalf("the previous vault is not offered: %d %s", listed.Code, listed.Body.String())
	}
	_, err = terminal.resolveCredential(context.Background(), "cred-terminal", binding)
	if err == nil || !strings.Contains(err.Error(), "previous Terminal vault") || !strings.Contains(err.Error(), "Import saved credentials") {
		t.Fatalf("a credential that is only in the previous vault failed without saying so: %v", err)
	}

	sharedBefore, _ := os.ReadFile(main.credentials.path)
	prompts := 0
	terminal.promptVaultSecret = func(_ context.Context, request desktop.VaultSecretRequest) (desktop.VaultSecretResponse, error) {
		prompts++
		if request.Mode != desktop.VaultSecretImport || request.Label != "Terminal" {
			t.Errorf("the previous vault's password was asked for with the wrong dialog: %+v", request)
		}
		return desktop.VaultSecretResponse{Password: []byte("not the password")}, nil
	}
	post := func(body string) (int, map[string]any) {
		r := requestApp(terminal, "POST", "/api/credential-vault", body, true)
		var decoded map[string]any
		_ = json.Unmarshal(r.Body.Bytes(), &decoded)
		return r.Code, decoded
	}
	// A wrong password imports nothing and changes neither vault.
	if code, body := post(`{"action":"import-previous","source":"terminal"}`); code != http.StatusBadRequest || !strings.Contains(body["error"].(string), "did not open the previous Terminal vault") {
		t.Fatalf("a wrong password: %d %v", code, body)
	}
	if now, _ := os.ReadFile(main.credentials.path); !bytes.Equal(now, sharedBefore) {
		t.Fatal("a failed import changed the shared vault")
	}
	if code, _ := post(`{"action":"import-previous","source":"nowhere"}`); code != http.StatusBadRequest {
		t.Fatalf("an unknown source was accepted: %d", code)
	}
	if code, _ := post(`{"action":"import-previous","source":"terminal","password":"must-not-be-accepted"}`); code != http.StatusBadRequest {
		t.Fatalf("a password in the request was accepted: %d", code)
	}
	// A canceled dialog imports nothing either.
	terminal.promptVaultSecret = func(context.Context, desktop.VaultSecretRequest) (desktop.VaultSecretResponse, error) {
		return desktop.VaultSecretResponse{}, errors.New("canceled")
	}
	if code, _ := post(`{"action":"import-previous","source":"terminal"}`); code != http.StatusBadRequest {
		t.Fatalf("a canceled import: %d", code)
	}
	// The right password of the previous vault (not the shared vault's).
	terminal.promptVaultSecret = func(context.Context, desktop.VaultSecretRequest) (desktop.VaultSecretResponse, error) {
		prompts++
		return desktop.VaultSecretResponse{Password: []byte(oldPassword)}, nil
	}
	code, body := post(`{"action":"import-previous","source":"terminal"}`)
	if code != 200 || body["imported"] != float64(2) || body["alreadyPresent"] != float64(1) || body["skipped"] != float64(0) || body["state"] != "unlocked" {
		t.Fatalf("import: %d %v", code, body)
	}
	if strings.Contains(pageJSON(body), "secret") {
		t.Fatalf("the import answer carries a secret: %v", body)
	}
	// Ids are kept where free; both apps can use the imported credentials.
	for _, app := range []*application{main, terminal} {
		record, ok := app.credentials.get("cred-terminal")
		if !ok || record.Secret != "terminal-secret" || len(record.Bindings) != 1 || record.Bindings[0] != binding {
			t.Fatalf("%s: the imported credential: %+v %v", app.windowMode, record.Name, ok)
		}
		if kept, _ := app.credentials.get("cred-taken"); kept.Secret != "main-secret" || kept.Name != "Main window's" {
			t.Fatalf("%s: a credential of the shared vault was overwritten: %+v", app.windowMode, kept.Name)
		}
	}
	if resolved, err := terminal.resolveCredential(context.Background(), "cred-terminal", binding); err != nil || resolved.Secret != "terminal-secret" {
		t.Fatalf("a migrated connection cannot use its imported credential: %v", err)
	}
	// The credential whose id was taken got a new id, and only the connection
	// that came from the Terminal workspace was pointed to it.
	renamed := ""
	for _, item := range main.credentials.list() {
		if item.Name == "Terminal's" {
			renamed = item.ID
		}
	}
	if renamed == "" || renamed == "cred-taken" {
		t.Fatalf("the credential with a taken id was not imported under a new id: %q", renamed)
	}
	if diskRow(t, root, "terminalProfiles", "migrated-b")["credentialId"] != renamed || diskRow(t, root, "terminalProfiles", "main-own")["credentialId"] != "cred-taken" || diskRow(t, root, "terminalProfiles", "migrated-a")["credentialId"] != "cred-terminal" {
		t.Fatalf("credential references after the import: %v", sharedDisk(t, root)["terminalProfiles"])
	}
	// The previous vault is untouched; the import is recorded and no longer
	// offered; importing again copies nothing.
	if now, _ := os.ReadFile(oldPath); !bytes.Equal(now, oldBefore) {
		t.Fatal("the previous Terminal vault was changed")
	}
	if vaults := main.previousVaults(); len(vaults) != 1 || !vaults[0].Imported {
		t.Fatalf("the import was not recorded: %+v", vaults)
	}
	count := len(main.credentials.list())
	if code, body := post(`{"action":"import-previous","source":"terminal"}`); code != 200 || body["imported"] != float64(0) || len(main.credentials.list()) != count {
		t.Fatalf("a second import: %d %v", code, body)
	}
	if prompts != 3 {
		t.Fatalf("the previous vault's password was asked %d times", prompts)
	}
	// A locked shared vault refuses the import before anything is opened.
	if err := terminal.credentials.lockAndForget(); err != nil {
		t.Fatal(err)
	}
	if code, _ := post(`{"action":"import-previous","source":"terminal"}`); code != http.StatusLocked {
		t.Fatalf("an import into a locked vault: %d", code)
	}
}

func TestSharedVaultImportFromPreviousVaultWithoutMasterPassword(t *testing.T) {
	root, main, _ := sharedVaultApps(t)
	previousTerminalVault(t, root, main.credentials.protect, main.credentials.unprotect, "",
		credentialRecord{ID: "cred-auto", Name: "From an automatic vault", Kind: "password", Secret: "auto-secret"})
	main.promptVaultSecret = func(context.Context, desktop.VaultSecretRequest) (desktop.VaultSecretResponse, error) {
		t.Error("a vault without a master password asked for one")
		return desktop.VaultSecretResponse{}, errors.New("unexpected prompt")
	}
	result, err := main.importPreviousVault(context.Background(), "terminal")
	if err != nil || result.Imported != 1 {
		t.Fatalf("import from a vault protected by this account only: %+v %v", result, err)
	}
	if record, ok := main.credentials.get("cred-auto"); !ok || record.Secret != "auto-secret" {
		t.Fatal("the credential was not imported")
	}
	// Another account's protection cannot open it: nothing is imported.
	other := syntheticCredentialVault(t, t.TempDir())
	stranger := sharedTestApp(t, root, standaloneEditorDirectory, standaloneEditorMode)
	stranger.credentials = other
	stranger.shared = newSharedData(root, stranger.dataPath)
	if _, err := stranger.importPreviousVault(context.Background(), "terminal"); err == nil {
		t.Fatal("a previous vault was opened with another account's protection")
	}
}
