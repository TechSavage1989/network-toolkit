package vaultcrypto

import (
	"bytes"
	"encoding/json"
	"io"
	"unicode/utf8"
)

// Marshal serializes public parameters and ciphertext only. It does not prove
// authenticity; a parsed envelope must still be successfully unlocked.
func (e *Envelope) Marshal() ([]byte, error) {
	if err := e.validate(); err != nil {
		return nil, err
	}
	wrap := func(s sealedData) wireSealed {
		return wireSealed{Nonce: encoding.EncodeToString(s.nonce), Ciphertext: encoding.EncodeToString(s.ciphertext)}
	}
	vaultID, generation := e.Identity()
	wire := wireEnvelope{
		Version: Version, Cipher: "AES-256-GCM", VaultID: vaultID, Generation: generation,
		KDF:      wireKDF{Name: "Argon2id", Version: 19, MemoryKiB: kdfMemoryKiB, Iterations: kdfIterations, Parallelism: kdfParallelism, KeyBytes: KeyBytes, Salt: encoding.EncodeToString(e.salt)},
		Password: wrap(e.password), Payload: wrap(e.payload),
	}
	if e.recovery != nil {
		r := wrap(*e.recovery)
		wire.Recovery = &r
	}
	data, err := json.Marshal(wire)
	if err != nil {
		return nil, ErrFormat
	}
	if len(data) > MaxEnvelopeBytes {
		return nil, ErrTooLarge
	}
	return data, nil
}

// Parse performs no KDF work. It rejects unsupported parameters, unknown,
// duplicate or differently-cased fields, missing fields, noncanonical base64,
// nulls, trailing documents and oversized inputs before password derivation.
func Parse(data []byte) (*Envelope, error) {
	if len(data) > MaxEnvelopeBytes {
		return nil, ErrTooLarge
	}
	if len(data) == 0 || !utf8.Valid(data) {
		return nil, ErrFormat
	}
	root, err := strictObject(data, []string{"version", "cipher", "vaultID", "generation", "kdf", "password", "payload"}, []string{"recovery"})
	if err != nil {
		return nil, err
	}
	if _, err := strictObject(root["kdf"], []string{"name", "version", "memoryKiB", "iterations", "parallelism", "keyBytes", "salt"}, nil); err != nil {
		return nil, err
	}
	for _, name := range []string{"password", "recovery", "payload"} {
		if raw, exists := root[name]; exists {
			if _, err := strictObject(raw, []string{"nonce", "ciphertext"}, nil); err != nil {
				return nil, err
			}
		}
	}
	var wire wireEnvelope
	if err := json.Unmarshal(data, &wire); err != nil {
		return nil, ErrFormat
	}
	if wire.Version != Version || wire.Cipher != "AES-256-GCM" || wire.KDF.Name != "Argon2id" || wire.KDF.Version != argonVersion || wire.KDF.MemoryKiB != kdfMemoryKiB || wire.KDF.Iterations != kdfIterations || wire.KDF.Parallelism != kdfParallelism || wire.KDF.KeyBytes != KeyBytes {
		return nil, ErrFormat
	}
	e := &Envelope{}
	for _, item := range []struct {
		encoded string
		target  *[]byte
		size    int
	}{
		{wire.VaultID, &e.vaultID, identityBytes},
		{wire.Generation, &e.generation, identityBytes},
		{wire.KDF.Salt, &e.salt, saltBytes},
	} {
		*item.target, err = decodeBytes(item.encoded, item.size, item.size)
		if err != nil {
			return nil, err
		}
	}
	e.password, err = parseSealed(wire.Password, KeyBytes+tagBytes, KeyBytes+tagBytes)
	if err != nil {
		return nil, err
	}
	if wire.Recovery != nil {
		r, err := parseSealed(*wire.Recovery, KeyBytes+tagBytes, KeyBytes+tagBytes)
		if err != nil {
			return nil, err
		}
		e.recovery = &r
	}
	e.payload, err = parseSealed(wire.Payload, tagBytes, MaxPayloadBytes+tagBytes)
	if err != nil {
		return nil, err
	}
	if err := e.validate(); err != nil {
		return nil, err
	}
	return e, nil
}

const argonVersion = 19

func parseSealed(w wireSealed, minBytes, maxBytes int) (sealedData, error) {
	nonce, err := decodeBytes(w.Nonce, nonceBytes, nonceBytes)
	if err != nil {
		return sealedData{}, err
	}
	ciphertext, err := decodeBytes(w.Ciphertext, minBytes, maxBytes)
	if err != nil {
		return sealedData{}, err
	}
	return sealedData{nonce: nonce, ciphertext: ciphertext}, nil
}

func decodeBytes(value string, minBytes, maxBytes int) ([]byte, error) {
	if len(value) < encoding.EncodedLen(minBytes) || len(value) > encoding.EncodedLen(maxBytes) {
		return nil, ErrFormat
	}
	data, err := encoding.DecodeString(value)
	if err != nil || len(data) < minBytes || len(data) > maxBytes || encoding.EncodeToString(data) != value {
		clear(data)
		return nil, ErrFormat
	}
	return data, nil
}

func strictObject(data []byte, required, optional []string) (map[string]json.RawMessage, error) {
	allowed := make(map[string]bool, len(required)+len(optional))
	for _, key := range required {
		allowed[key] = true
	}
	for _, key := range optional {
		allowed[key] = true
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	token, err := decoder.Token()
	if err != nil || token != json.Delim('{') {
		return nil, ErrFormat
	}
	values := make(map[string]json.RawMessage, len(allowed))
	for decoder.More() {
		token, err := decoder.Token()
		if err != nil {
			return nil, ErrFormat
		}
		key, ok := token.(string)
		if !ok || !allowed[key] {
			return nil, ErrFormat
		}
		if _, exists := values[key]; exists {
			return nil, ErrFormat
		}
		var value json.RawMessage
		if err := decoder.Decode(&value); err != nil || bytes.Equal(bytes.TrimSpace(value), []byte("null")) {
			return nil, ErrFormat
		}
		values[key] = value
	}
	if token, err = decoder.Token(); err != nil || token != json.Delim('}') {
		return nil, ErrFormat
	}
	if _, err := decoder.Token(); err != io.EOF {
		return nil, ErrFormat
	}
	for _, key := range required {
		if _, exists := values[key]; !exists {
			return nil, ErrFormat
		}
	}
	return values, nil
}
