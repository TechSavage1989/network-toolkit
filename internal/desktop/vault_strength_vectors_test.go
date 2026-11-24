package desktop

import "testing"

// The Password manager form's gauge (vaultStrength in
// web/assets/credential-manager.js) ports this estimate; the same vectors are
// in scripts/connection-credentials.test.cjs. Keep both lists equal.
func TestVaultStrengthVectors(t *testing.T) {
	for _, v := range []struct{ password, label string }{
		{"", "Enter a password"}, {"password", "Very weak"}, {"Password1!", "Very weak"}, {"abcdefgh", "Very weak"},
		{"correct horse battery staple", "Very strong"}, {"Tr0ub4dor&3", "Strong"}, {"kX9#mQ2v", "Fair"},
		{"zq8!Lm3@Rt7$wp", "Very strong"}, {"пароль-секрет-2026", "Very strong"}, {"aaaaaaaaaaaaaaaaaaaa", "Very weak"},
	} {
		if got := estimateVaultPasswordStrength([]byte(v.password)).Label; got != v.label {
			t.Errorf("%q: %s", v.password, got)
		}
	}
}

func TestVaultSecretFromEntryKeepsTheBoxRules(t *testing.T) {
	entry := func() VaultSecretEntry {
		return VaultSecretEntry{Password: []byte("fixture one"), Confirm: []byte("fixture one"), CurrentPassword: []byte("old fixture"), RecoveryKey: []byte("KEY-FIXTURE"), GenerateRecovery: true, Remember: true}
	}
	got, err := VaultSecretFromEntry(VaultSecretRequest{Mode: VaultSecretRotate}, entry())
	if err != nil || string(got.Password) != "fixture one" || string(got.CurrentPassword) != "old fixture" || got.RecoveryKey != nil || !got.Remember || !got.GenerateRecovery {
		t.Fatalf("rotate: %+v %v", got, err)
	}
	// Remove and import have no remember box and create no recovery key.
	for _, mode := range []VaultSecretMode{VaultSecretRemovePassword, VaultSecretImport, VaultSecretUnlock} {
		got, err := VaultSecretFromEntry(VaultSecretRequest{Mode: mode}, entry())
		if err != nil || got.CurrentPassword != nil || got.RecoveryKey != nil || got.GenerateRecovery {
			t.Fatalf("%s: %+v %v", mode, got, err)
		}
		if got.Remember != (mode == VaultSecretUnlock) {
			t.Fatalf("%s remember: %v", mode, got.Remember)
		}
	}
	bad := entry()
	bad.Confirm = []byte("fixture two")
	if _, err := VaultSecretFromEntry(VaultSecretRequest{Mode: VaultSecretSetup}, bad); err == nil {
		t.Fatal("a mismatched confirmation was accepted")
	}
	bad = entry()
	bad.RecoveryKey = nil
	if _, err := VaultSecretFromEntry(VaultSecretRequest{Mode: VaultSecretRecover}, bad); err == nil {
		t.Fatal("recover without a key was accepted")
	}
	bad = entry()
	bad.Password = []byte("nul\x00inside")
	if _, err := VaultSecretFromEntry(VaultSecretRequest{Mode: VaultSecretUnlock}, bad); err == nil {
		t.Fatal("a NUL byte was accepted")
	}
}
