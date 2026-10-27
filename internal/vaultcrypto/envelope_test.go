package vaultcrypto

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"testing"
	"time"
)

var testPassword = []byte("correct horse battery staple")
var testPayload = []byte(`{"credentials":[{"name":"private-router","username":"private-user","password":"private-password","privateKey":"private-key-material","host":"private-host","pin":"private-pin"}]}`)

func fixture(t *testing.T, recovery bool) (*Envelope, []byte, string) {
	t.Helper()
	e, key, code, err := Create(testPassword, testPayload, recovery)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { clear(key) })
	return e, key, code
}

func serialized(t *testing.T, e *Envelope) []byte {
	t.Helper()
	data, err := e.Marshal()
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func wireCopy(t *testing.T, e *Envelope) wireEnvelope {
	t.Helper()
	var wire wireEnvelope
	if err := json.Unmarshal(serialized(t, e), &wire); err != nil {
		t.Fatal(err)
	}
	return wire
}

func parseWire(t *testing.T, w wireEnvelope) (*Envelope, error) {
	t.Helper()
	data, err := json.Marshal(w)
	if err != nil {
		t.Fatal(err)
	}
	return Parse(data)
}

func flip(encoded string) string {
	b, err := encoding.DecodeString(encoded)
	if err != nil || len(b) == 0 {
		panic("invalid test fixture")
	}
	b[0] ^= 1
	return encoding.EncodeToString(b)
}

func TestRoundTripEncryptsSecretsAndMetadata(t *testing.T) {
	e, key, recovery := fixture(t, true)
	data := serialized(t, e)
	for _, secret := range []string{"private-router", "private-user", "private-password", "private-key-material", "private-host", "private-pin", string(testPassword), recovery, encoding.EncodeToString(key)} {
		if bytes.Contains(data, []byte(secret)) {
			t.Fatal("plaintext material exposed in envelope")
		}
	}
	parsed, err := Parse(data)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(serialized(t, parsed), data) || !parsed.HasRecovery() {
		t.Fatal("serialization or recovery slot changed")
	}
	id, generation := e.Identity()
	id2, generation2 := parsed.Identity()
	if id == "" || generation == "" || id != id2 || generation != generation2 {
		t.Fatal("identity not preserved")
	}
	payload, unwrapped, err := parsed.UnlockPassword(testPassword)
	if err != nil || !bytes.Equal(payload, testPayload) || !bytes.Equal(unwrapped, key) {
		t.Fatal("password round trip failed", err)
	}
	clear(payload)
	clear(unwrapped)
	payload, unwrapped, err = parsed.UnlockRecovery(recovery)
	if err != nil || !bytes.Equal(payload, testPayload) || !bytes.Equal(unwrapped, key) {
		t.Fatal("recovery round trip failed", err)
	}
	clear(payload)
	clear(unwrapped)
	payload, err = parsed.OpenKey(key)
	if err != nil || !bytes.Equal(payload, testPayload) {
		t.Fatal("remembered key round trip failed", err)
	}
	clear(payload)
}

func TestWrongKeysNeverReturnPlaintext(t *testing.T) {
	e, key, recovery := fixture(t, true)
	for _, password := range [][]byte{[]byte("incorrect horse battery staple"), nil, []byte("short")} {
		payload, dek, err := e.UnlockPassword(password)
		if !errors.Is(err, ErrAuthentication) || payload != nil || dek != nil {
			t.Fatal("wrong password returned material or unexpected error", err)
		}
	}
	for _, code := range []string{"", "NTK2-short", recovery + " ", " " + recovery, "NTK2-" + flip(strings.TrimPrefix(recovery, recoveryPrefix)), strings.ToLower(recovery)} {
		payload, dek, err := e.UnlockRecovery(code)
		if !errors.Is(err, ErrAuthentication) || payload != nil || dek != nil {
			t.Fatal("wrong recovery returned material or unexpected error", err)
		}
	}
	wrong := bytes.Clone(key)
	defer clear(wrong)
	wrong[0] ^= 1
	for _, candidate := range [][]byte{nil, key[:31], append(bytes.Clone(key), 0), wrong} {
		payload, err := e.OpenKey(candidate)
		if !errors.Is(err, ErrAuthentication) || payload != nil {
			t.Fatal("wrong DEK returned material or unexpected error", err)
		}
	}
}

func TestEveryAuthenticatedFieldAndSlotRejectsTampering(t *testing.T) {
	e, key, recovery := fixture(t, true)
	mutations := map[string]func(*wireEnvelope){
		"vault ID":            func(w *wireEnvelope) { w.VaultID = flip(w.VaultID) },
		"generation":          func(w *wireEnvelope) { w.Generation = flip(w.Generation) },
		"salt":                func(w *wireEnvelope) { w.KDF.Salt = flip(w.KDF.Salt) },
		"password nonce":      func(w *wireEnvelope) { w.Password.Nonce = flip(w.Password.Nonce) },
		"password ciphertext": func(w *wireEnvelope) { w.Password.Ciphertext = flip(w.Password.Ciphertext) },
		"recovery nonce":      func(w *wireEnvelope) { w.Recovery.Nonce = flip(w.Recovery.Nonce) },
		"recovery ciphertext": func(w *wireEnvelope) { w.Recovery.Ciphertext = flip(w.Recovery.Ciphertext) },
		"payload nonce":       func(w *wireEnvelope) { w.Payload.Nonce = flip(w.Payload.Nonce) },
		"payload ciphertext":  func(w *wireEnvelope) { w.Payload.Ciphertext = flip(w.Payload.Ciphertext) },
		"removed recovery":    func(w *wireEnvelope) { w.Recovery = nil },
		"swapped slots":       func(w *wireEnvelope) { w.Password, *w.Recovery = *w.Recovery, w.Password },
	}
	for name, mutate := range mutations {
		t.Run(name, func(t *testing.T) {
			w := wireCopy(t, e)
			mutate(&w)
			changed, err := parseWire(t, w)
			if err != nil {
				t.Fatal("well-formed tampering should reach authentication", err)
			}
			if p, err := changed.OpenKey(key); !errors.Is(err, ErrAuthentication) || p != nil {
				t.Fatal("tampering accepted by remembered key", err)
			}
			if p, k, err := changed.UnlockPassword(testPassword); !errors.Is(err, ErrAuthentication) || p != nil || k != nil {
				t.Fatal("tampering accepted by password", err)
			}
			if p, k, err := changed.UnlockRecovery(recovery); !errors.Is(err, ErrAuthentication) || p != nil || k != nil {
				t.Fatal("tampering accepted by recovery", err)
			}
			if _, err := changed.Reseal([]byte("replacement"), key); !errors.Is(err, ErrAuthentication) {
				t.Fatal("reseal repaired an unauthentic envelope", err)
			}
		})
	}
}

func TestNoRecoveryUnlessExplicitlyRequested(t *testing.T) {
	e, key, code := fixture(t, false)
	if code != "" || e.HasRecovery() || bytes.Contains(serialized(t, e), []byte(`"recovery"`)) {
		t.Fatal("unexpected recovery material")
	}
	if p, k, err := e.UnlockRecovery("NTK2-" + encoding.EncodeToString(key)); !errors.Is(err, ErrAuthentication) || p != nil || k != nil {
		t.Fatal("DEK accepted as recovery")
	}
	w := wireCopy(t, e)
	w.Recovery = &wireSealed{Nonce: flip(w.Password.Nonce), Ciphertext: w.Password.Ciphertext}
	changed, err := parseWire(t, w)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := changed.OpenKey(key); !errors.Is(err, ErrAuthentication) {
		t.Fatal("adding a recovery slot was not authenticated", err)
	}
}

func TestRecoveryIsIndependentAndDoesNotRunPasswordKDF(t *testing.T) {
	e, key, recovery := fixture(t, true)
	recoveryKey, err := decodeBytes(strings.TrimPrefix(recovery, recoveryPrefix), KeyBytes, KeyBytes)
	if err != nil {
		t.Fatal(err)
	}
	defer clear(recoveryKey)
	passwordKey := derivePasswordKey(testPassword, e.salt)
	defer clear(passwordKey)
	if bytes.Equal(recoveryKey, key) || bytes.Equal(recoveryKey, passwordKey) {
		t.Fatal("recovery reused DEK or password-derived key")
	}
	kdfMu.Lock()
	done := make(chan error, 1)
	go func() {
		payload, dek, err := e.UnlockRecovery(recovery)
		if err == nil && (!bytes.Equal(payload, testPayload) || !bytes.Equal(dek, key)) {
			err = errors.New("recovery returned unexpected material")
		}
		clear(payload)
		clear(dek)
		done <- err
	}()
	select {
	case err := <-done:
		kdfMu.Unlock()
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		kdfMu.Unlock()
		<-done
		t.Fatal("recovery tried to use the password KDF")
	}
}

func TestResealPreservesIdentityAndWrappingSlotsWithFreshNonces(t *testing.T) {
	e, key, recovery := fixture(t, true)
	original := serialized(t, e)
	seen := make(map[string]bool)
	for _, nonce := range e.nonces() {
		seen[string(nonce)] = true
	}
	current := e
	for i := 0; i < 128; i++ {
		next, err := current.Reseal([]byte("updated encrypted metadata"), key)
		if err != nil {
			t.Fatal(err)
		}
		if seen[string(next.payload.nonce)] {
			t.Fatal("nonce reused")
		}
		seen[string(next.payload.nonce)] = true
		id, generation := e.Identity()
		nextID, nextGeneration := next.Identity()
		if id != nextID || generation != nextGeneration || !bytes.Equal(e.password.ciphertext, next.password.ciphertext) || !bytes.Equal(e.recovery.ciphertext, next.recovery.ciphertext) {
			t.Fatal("reseal changed identity or wrapping slots")
		}
		current = next
	}
	if !bytes.Equal(original, serialized(t, e)) {
		t.Fatal("reseal mutated original")
	}
	payload, dek, err := current.UnlockRecovery(recovery)
	if err != nil || string(payload) != "updated encrypted metadata" || !bytes.Equal(dek, key) {
		t.Fatal("recovery failed after reseal", err)
	}
	clear(payload)
	clear(dek)
	payload, dek, err = current.UnlockPassword(testPassword)
	if err != nil || string(payload) != "updated encrypted metadata" || !bytes.Equal(dek, key) {
		t.Fatal("password failed after reseal", err)
	}
	clear(payload)
	clear(dek)
}

func TestRotationChangesKeysIdentitySaltAndRecovery(t *testing.T) {
	old, oldKey, oldRecovery := fixture(t, true)
	fresh, newKey, newRecovery, err := Create([]byte("different master password"), testPayload, true)
	if err != nil {
		t.Fatal(err)
	}
	defer clear(newKey)
	a, b := old.Identity()
	c, d := fresh.Identity()
	if a == c || b == d || bytes.Equal(old.salt, fresh.salt) || bytes.Equal(oldKey, newKey) || oldRecovery == newRecovery {
		t.Fatal("rotation reused identity, key, salt, or recovery")
	}
	if _, err := fresh.OpenKey(oldKey); !errors.Is(err, ErrAuthentication) {
		t.Fatal("old remembered key accepted")
	}
	if _, _, err := fresh.UnlockPassword(testPassword); !errors.Is(err, ErrAuthentication) {
		t.Fatal("old password accepted")
	}
	if _, _, err := fresh.UnlockRecovery(oldRecovery); !errors.Is(err, ErrAuthentication) {
		t.Fatal("old recovery accepted")
	}
	if _, _, err := old.UnlockRecovery(newRecovery); !errors.Is(err, ErrAuthentication) {
		t.Fatal("new recovery accepted by old envelope")
	}
}

func TestPasswordPolicyCountsUnicodeWithoutNormalization(t *testing.T) {
	for _, password := range [][]byte{nil, []byte("12345678901"), []byte(strings.Repeat("界", 11)), {0xff}, bytes.Repeat([]byte("a"), MaxPasswordBytes+1), []byte(strings.Repeat("界", 342))} {
		if !errors.Is(ValidateNewPassword(password), ErrPassword) {
			t.Fatal("invalid new password accepted")
		}
		if e, k, r, err := Create(password, nil, false); !errors.Is(err, ErrPassword) || e != nil || k != nil || r != "" {
			t.Fatal("invalid password created an envelope")
		}
	}
	for _, password := range [][]byte{[]byte("123456789012"), []byte(strings.Repeat("界", 12)), bytes.Repeat([]byte("a"), MaxPasswordBytes), []byte(" 1234567890 ")} {
		if err := ValidateNewPassword(password); err != nil {
			t.Fatal("valid new password rejected", err)
		}
	}
	password := []byte(" café password ")
	e, key, _, err := Create(password, nil, false)
	if err != nil {
		t.Fatal(err)
	}
	defer clear(key)
	for _, changed := range [][]byte{[]byte("café password"), []byte(" cafe\u0301 password ")} {
		if _, _, err := e.UnlockPassword(changed); !errors.Is(err, ErrAuthentication) {
			t.Fatal("password was trimmed or normalized", err)
		}
	}
	payload, dek, err := e.UnlockPassword(password)
	if err != nil || len(payload) != 0 || !bytes.Equal(dek, key) {
		t.Fatal("empty payload or Unicode password failed", err)
	}
	clear(payload)
	clear(dek)
}

func TestSizeLimitsAndMaximumPayload(t *testing.T) {
	if _, err := Parse(make([]byte, MaxEnvelopeBytes+1)); !errors.Is(err, ErrTooLarge) {
		t.Fatal("oversized envelope accepted", err)
	}
	if _, _, _, err := Create(testPassword, make([]byte, MaxPayloadBytes+1), false); !errors.Is(err, ErrTooLarge) {
		t.Fatal("oversized create accepted", err)
	}
	payload := bytes.Repeat([]byte{0xa5}, MaxPayloadBytes)
	e, key, _, err := Create(testPassword, payload, false)
	if err != nil {
		t.Fatal(err)
	}
	defer clear(key)
	parsed, err := Parse(serialized(t, e))
	if err != nil {
		t.Fatal("maximum payload did not parse", err)
	}
	opened, err := parsed.OpenKey(key)
	if err != nil || !bytes.Equal(opened, payload) {
		t.Fatal("maximum payload round trip failed", err)
	}
	clear(opened)
	if _, err := e.Reseal(make([]byte, MaxPayloadBytes+1), key); !errors.Is(err, ErrTooLarge) {
		t.Fatal("oversized reseal accepted", err)
	}
}

func TestMalformedAndUnsupportedInputsRejectBeforeKDF(t *testing.T) {
	e, _, _ := fixture(t, true)
	valid := serialized(t, e)
	malformed := [][]byte{
		nil, []byte("null"), []byte("[]"), []byte("{}"), []byte("{broken"), append(bytes.Clone(valid), valid...),
		bytes.Replace(valid, []byte(`"version":2`), []byte(`"version":2,"version":2`), 1),
		bytes.Replace(valid, []byte(`"version":2`), []byte(`"Version":2`), 1),
		bytes.Replace(valid, []byte(`"version":2`), []byte(`"version":null`), 1),
		bytes.Replace(valid, []byte(`"version":2`), []byte(`"version":2,"unknown":1`), 1),
		bytes.Replace(valid, []byte(`"version":2,`), nil, 1),
		bytes.Replace(valid, []byte(`"name":"Argon2id"`), []byte(`"name":"Argon2id","name":"Argon2id"`), 1),
		bytes.Replace(valid, []byte(`"name":"Argon2id"`), []byte(`"Name":"Argon2id"`), 1),
		bytes.Replace(valid, []byte(`"name":"Argon2id"`), []byte(`"name":null`), 1),
		append([]byte{0xff}, valid...),
	}
	for _, mutate := range []func(*wireEnvelope){
		func(w *wireEnvelope) { w.Version = 1 },
		func(w *wireEnvelope) { w.Version = 3 },
		func(w *wireEnvelope) { w.Cipher = "AES-128-GCM" },
		func(w *wireEnvelope) { w.KDF.Name = "Argon2i" },
		func(w *wireEnvelope) { w.KDF.Version = 16 },
		func(w *wireEnvelope) { w.KDF.MemoryKiB = 0 },
		func(w *wireEnvelope) { w.KDF.MemoryKiB = ^uint32(0) },
		func(w *wireEnvelope) { w.KDF.Iterations = 0 },
		func(w *wireEnvelope) { w.KDF.Iterations = ^uint32(0) },
		func(w *wireEnvelope) { w.KDF.Parallelism = 0 },
		func(w *wireEnvelope) { w.KDF.Parallelism = 255 },
		func(w *wireEnvelope) { w.KDF.KeyBytes = 16 },
		func(w *wireEnvelope) { w.KDF.KeyBytes = ^uint32(0) },
		func(w *wireEnvelope) { w.KDF.Salt = "" },
		func(w *wireEnvelope) { w.VaultID += "=" },
		func(w *wireEnvelope) { w.Generation += "\n" },
		func(w *wireEnvelope) { w.Password.Nonce = "" },
		func(w *wireEnvelope) { w.Password.Ciphertext = encoding.EncodeToString(make([]byte, 47)) },
		func(w *wireEnvelope) { w.Recovery.Ciphertext = encoding.EncodeToString(make([]byte, 49)) },
		func(w *wireEnvelope) { w.Payload.Nonce = w.Password.Nonce },
		func(w *wireEnvelope) { w.Recovery.Nonce = w.Password.Nonce },
		func(w *wireEnvelope) { w.Payload.Ciphertext = encoding.EncodeToString(make([]byte, tagBytes-1)) },
		func(w *wireEnvelope) {
			w.Payload.Ciphertext = encoding.EncodeToString(make([]byte, MaxPayloadBytes+tagBytes+1))
		},
	} {
		w := wireCopy(t, e)
		mutate(&w)
		data, err := json.Marshal(w)
		if err != nil {
			t.Fatal(err)
		}
		malformed = append(malformed, data)
	}
	// Holding the actual KDF gate proves malformed parse and invalid password
	// bounds cannot attempt derivation, without a replaceable production KDF.
	kdfMu.Lock()
	done := make(chan error, 1)
	go func() {
		for _, data := range malformed {
			if _, err := Parse(data); err == nil {
				done <- errors.New("malformed envelope accepted")
				return
			}
		}
		for _, password := range [][]byte{make([]byte, MaxPasswordBytes+1), {0xff}} {
			if _, _, err := e.UnlockPassword(password); !errors.Is(err, ErrPasswordInput) {
				done <- errors.New("invalid bounded input accepted")
				return
			}
		}
		var missing *Envelope
		if _, _, err := missing.UnlockPassword(testPassword); !errors.Is(err, ErrFormat) {
			done <- errors.New("nil envelope accepted")
			return
		}
		done <- nil
	}()
	select {
	case err := <-done:
		kdfMu.Unlock()
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		kdfMu.Unlock()
		<-done
		t.Fatal("preflight attempted a KDF or exceeded the bounded parsing deadline")
	}
}

func TestStrictJSONAcceptsWhitespaceButNotDuplicateEscapedKeys(t *testing.T) {
	e, _, _ := fixture(t, true)
	data := serialized(t, e)
	var indented bytes.Buffer
	if err := json.Indent(&indented, data, "", "  "); err != nil {
		t.Fatal(err)
	}
	if _, err := Parse(append(indented.Bytes(), '\n')); err != nil {
		t.Fatal("ordinary formatting rejected", err)
	}
	for _, invalid := range [][]byte{
		bytes.Replace(data, []byte(`"version":2`), []byte(`"version":2,"\u0076ersion":2`), 1),
		bytes.Replace(data, []byte(`"nonce":`), []byte(`"nonce":null,"nonce":`), 1),
		bytes.Replace(data, []byte(`"payload":{`), []byte(`"payload":{"unknown":1,`), 1),
		bytes.Replace(data, []byte(`"recovery":{`), []byte(`"recovery":null,"ignored":{`), 1),
	} {
		if _, err := Parse(invalid); err == nil {
			t.Fatal("ambiguous or unknown JSON accepted")
		}
	}
}

type failingReader struct{}

func (failingReader) Read(p []byte) (int, error) { return 0, io.ErrUnexpectedEOF }

func TestRandomFailureAndRepeatedNonceFailClosed(t *testing.T) {
	if e, k, r, err := create(testPassword, nil, true, failingReader{}); err == nil || e != nil || k != nil || r != "" {
		t.Fatal("random source failure did not fail closed")
	}
	key := make([]byte, KeyBytes)
	nonce := make([]byte, nonceBytes)
	if _, err := seal(bytes.NewReader(make([]byte, nonceBytes*8)), key, nil, nil, nonce); err == nil {
		t.Fatal("repeated nonce source did not fail closed")
	}
	if _, err := seal(failingReader{}, key, nil, nil); err == nil {
		t.Fatal("nonce source failure accepted")
	}
}

func FuzzParse(f *testing.F) {
	// A structurally valid but unauthentic envelope exercises the full parser
	// without any key derivation or embedded credential material in fuzz seeds.
	wire := wireEnvelope{
		Version: Version, Cipher: "AES-256-GCM",
		VaultID: encoding.EncodeToString(make([]byte, identityBytes)), Generation: encoding.EncodeToString(bytes.Repeat([]byte{1}, identityBytes)),
		KDF:      wireKDF{Name: "Argon2id", Version: 19, MemoryKiB: kdfMemoryKiB, Iterations: kdfIterations, Parallelism: kdfParallelism, KeyBytes: KeyBytes, Salt: encoding.EncodeToString(make([]byte, saltBytes))},
		Password: wireSealed{Nonce: encoding.EncodeToString(make([]byte, nonceBytes)), Ciphertext: encoding.EncodeToString(make([]byte, KeyBytes+tagBytes))},
		Payload:  wireSealed{Nonce: encoding.EncodeToString(bytes.Repeat([]byte{1}, nonceBytes)), Ciphertext: encoding.EncodeToString(make([]byte, tagBytes))},
	}
	seed, err := json.Marshal(wire)
	if err != nil {
		f.Fatal(err)
	}
	f.Add(seed)
	f.Add([]byte(`{"version":2}`))
	f.Add([]byte("null"))
	f.Add([]byte("{}{}"))
	f.Fuzz(func(t *testing.T, data []byte) {
		e, err := Parse(data)
		if err != nil {
			return
		}
		serialized, err := e.Marshal()
		if err != nil {
			t.Fatal("parsed envelope could not serialize", err)
		}
		if _, err := Parse(serialized); err != nil {
			t.Fatal("serialized envelope could not parse", err)
		}
	})
}
