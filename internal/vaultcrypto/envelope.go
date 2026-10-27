// Package vaultcrypto implements the version 2 credential-vault envelope.
// It has no filesystem, operating-system protection, or UI dependencies.
//
// Credential records, including their metadata, belong inside the opaque payload.
// Only format parameters, random identifiers, nonces and encrypted data are public.
// This package does not prevent rollback to an entire older valid envelope. Callers
// own atomic storage, remembered-key protection, input throttling and lifecycle.
// Returned plaintext and keys belong to the caller and should be cleared promptly.
// Clearing Go byte slices is best effort, not guaranteed removal from RAM, swap,
// crash dumps, runtime copies, or the Argon2 implementation's working memory.
package vaultcrypto

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"io"
	"strings"
	"sync"
	"unicode/utf8"

	"golang.org/x/crypto/argon2"
)

const (
	Version          = 2
	KeyBytes         = 32
	MinPasswordRunes = 12
	MaxPasswordBytes = 1024
	MaxPayloadBytes  = 2 << 20
	MaxEnvelopeBytes = 4 << 20

	saltBytes      = 16
	identityBytes  = 16
	nonceBytes     = 12
	tagBytes       = 16
	kdfMemoryKiB   = 64 * 1024
	kdfIterations  = 3
	kdfParallelism = 1
	recoveryPrefix = "NTK2-"
)

var (
	ErrFormat         = errors.New("invalid or unsupported credential vault envelope")
	ErrAuthentication = errors.New("credential vault authentication failed")
	ErrPassword       = errors.New("master password must contain at least 12 Unicode characters and at most 1024 UTF-8 bytes")
	ErrPasswordInput  = errors.New("master password must be valid UTF-8 and at most 1024 bytes")
	ErrTooLarge       = errors.New("credential vault exceeds the size limit")
)

// Envelope is immutable through the public API. Parse validates its structure,
// but only a successful unlock/OpenKey establishes cryptographic authenticity.
type Envelope struct {
	vaultID    []byte
	generation []byte
	salt       []byte
	password   sealedData
	recovery   *sealedData
	payload    sealedData
}

type sealedData struct {
	nonce      []byte
	ciphertext []byte
}

type wireEnvelope struct {
	Version    int         `json:"version"`
	Cipher     string      `json:"cipher"`
	VaultID    string      `json:"vaultID"`
	Generation string      `json:"generation"`
	KDF        wireKDF     `json:"kdf"`
	Password   wireSealed  `json:"password"`
	Recovery   *wireSealed `json:"recovery,omitempty"`
	Payload    wireSealed  `json:"payload"`
}

type wireKDF struct {
	Name        string `json:"name"`
	Version     int    `json:"version"`
	MemoryKiB   uint32 `json:"memoryKiB"`
	Iterations  uint32 `json:"iterations"`
	Parallelism uint8  `json:"parallelism"`
	KeyBytes    uint32 `json:"keyBytes"`
	Salt        string `json:"salt"`
}

type wireSealed struct {
	Nonce      string `json:"nonce"`
	Ciphertext string `json:"ciphertext"`
}

var encoding = base64.RawURLEncoding.Strict()

// Serialize KDF calls so concurrent local requests cannot multiply the 64 MiB
// working-memory allocation. Callers must still bound/throttle request queues.
var kdfMu sync.Mutex

func derivePasswordKey(password, salt []byte) []byte {
	kdfMu.Lock()
	defer kdfMu.Unlock()
	// Fixed, deliberately non-negotiable v2 profile. OWASP's Argon2id minimum is
	// 19 MiB / 2 passes / 1 lane. RFC 9106's 64 MiB / 3 pass profile uses 4 lanes;
	// this application's 1 lane bounds concurrency, not an exact RFC profile.
	// https://pkg.go.dev/golang.org/x/crypto/argon2#IDKey
	// https://www.rfc-editor.org/rfc/rfc9106.html#section-4
	return argon2.IDKey(password, salt, kdfIterations, kdfMemoryKiB, kdfParallelism, KeyBytes)
}

// ValidateNewPassword checks creation/rotation policy without deriving a key.
// Passwords are never trimmed, normalized, or truncated.
func ValidateNewPassword(password []byte) error {
	if len(password) > MaxPasswordBytes || !utf8.Valid(password) || utf8.RuneCount(password) < MinPasswordRunes {
		return ErrPassword
	}
	return nil
}

// Create encrypts payload under a new random DEK and wraps that DEK with the
// password-derived key. Recovery, if explicitly requested, uses an independent
// random 256-bit key; its text is returned only here, never stored in the envelope.
func Create(password, payload []byte, withRecovery bool) (*Envelope, []byte, string, error) {
	return create(password, payload, withRecovery, rand.Reader)
}

func create(password, payload []byte, withRecovery bool, random io.Reader) (*Envelope, []byte, string, error) {
	if err := ValidateNewPassword(password); err != nil {
		return nil, nil, "", err
	}
	if len(payload) > MaxPayloadBytes {
		return nil, nil, "", ErrTooLarge
	}
	e := &Envelope{}
	var err error
	for _, target := range []*[]byte{&e.vaultID, &e.generation, &e.salt} {
		*target, err = randomBytes(random, identityBytes)
		if err != nil {
			return nil, nil, "", err
		}
	}
	dek, err := randomBytes(random, KeyBytes)
	if err != nil {
		return nil, nil, "", err
	}
	ok := false
	defer func() {
		if !ok {
			clear(dek)
		}
	}()
	var recoveryKey []byte
	if withRecovery {
		recoveryKey, err = randomBytes(random, KeyBytes)
		if err != nil {
			return nil, nil, "", err
		}
		defer clear(recoveryKey)
		e.recovery = &sealedData{}
	}
	passwordKey := derivePasswordKey(password, e.salt)
	defer clear(passwordKey)
	e.password, err = seal(random, passwordKey, dek, e.slotAAD("password"))
	if err != nil {
		return nil, nil, "", err
	}
	if withRecovery {
		r, sealErr := seal(random, recoveryKey, dek, e.slotAAD("recovery"), e.password.nonce)
		if sealErr != nil {
			return nil, nil, "", sealErr
		}
		e.recovery = &r
	}
	e.payload, err = seal(random, dek, payload, e.payloadAAD(), e.nonces()...)
	if err != nil {
		return nil, nil, "", err
	}
	if err := e.validate(); err != nil {
		return nil, nil, "", err
	}
	var recovery string
	if withRecovery {
		recovery = recoveryPrefix + encoding.EncodeToString(recoveryKey)
	}
	ok = true
	return e, dek, recovery, nil
}

// Identity returns public random identifiers. A remembered key must be bound to
// both values. Reseal preserves them; Create (including rotation) replaces them.
func (e *Envelope) Identity() (vaultID, generation string) {
	if e == nil {
		return "", ""
	}
	return encoding.EncodeToString(e.vaultID), encoding.EncodeToString(e.generation)
}

func (e *Envelope) HasRecovery() bool { return e != nil && e.recovery != nil }

// UnlockPassword authenticates the entire envelope, not just the password slot.
// Unlock intentionally does not apply creation's minimum-length requirement.
func (e *Envelope) UnlockPassword(password []byte) ([]byte, []byte, error) {
	if err := e.validate(); err != nil {
		return nil, nil, err
	}
	if len(password) > MaxPasswordBytes || !utf8.Valid(password) {
		return nil, nil, ErrPasswordInput
	}
	kek := derivePasswordKey(password, e.salt)
	defer clear(kek)
	return e.unlockSlot(kek, e.password, "password")
}

// UnlockRecovery is independent of the master password and does not run a KDF.
// The recovery text is case-sensitive and is not trimmed or normalized.
func (e *Envelope) UnlockRecovery(recovery string) ([]byte, []byte, error) {
	if err := e.validate(); err != nil {
		return nil, nil, err
	}
	if e.recovery == nil || len(recovery) != len(recoveryPrefix)+encoding.EncodedLen(KeyBytes) || !strings.HasPrefix(recovery, recoveryPrefix) {
		return nil, nil, ErrAuthentication
	}
	key, err := decodeBytes(recovery[len(recoveryPrefix):], KeyBytes, KeyBytes)
	if err != nil {
		return nil, nil, ErrAuthentication
	}
	defer clear(key)
	return e.unlockSlot(key, *e.recovery, "recovery")
}

func (e *Envelope) unlockSlot(key []byte, slot sealedData, role string) ([]byte, []byte, error) {
	dek, err := open(key, slot, e.slotAAD(role))
	if err != nil {
		return nil, nil, err
	}
	payload, err := e.OpenKey(dek)
	if err != nil {
		clear(dek)
		return nil, nil, err
	}
	return payload, dek, nil
}

// OpenKey authenticates payload and both key slots using an already unwrapped
// DEK, for example one protected by Windows DPAPI. It never stores that key.
func (e *Envelope) OpenKey(dek []byte) ([]byte, error) {
	if err := e.validate(); err != nil {
		return nil, err
	}
	return open(dek, e.payload, e.payloadAAD())
}

// Reseal returns a new envelope with the same identity and wrapping slots, after
// authenticating the old envelope. A fresh random 96-bit nonce is used on every
// write; nonce collisions with this envelope's existing nonces are retried.
// As with ordinary random-nonce GCM, do not use a DEK for 2^32 or more writes.
func (e *Envelope) Reseal(payload, dek []byte) (*Envelope, error) {
	if len(payload) > MaxPayloadBytes {
		return nil, ErrTooLarge
	}
	previous, err := e.OpenKey(dek)
	if err != nil {
		return nil, err
	}
	clear(previous)
	updated := *e
	updated.payload, err = seal(rand.Reader, dek, payload, e.payloadAAD(), e.nonces()...)
	if err != nil {
		return nil, err
	}
	return &updated, nil
}

func (e *Envelope) slotAAD(role string) []byte {
	// All variable components below have fixed length, and role is internal only.
	aad := []byte("network-toolkit/vault/v2\x00" + role + "\x00aes256gcm/argon2id/v19/m65536/t3/p1/k32\x00")
	aad = append(aad, e.vaultID...)
	aad = append(aad, e.generation...)
	aad = append(aad, e.salt...)
	if e.recovery != nil {
		return append(aad, 1)
	}
	return append(aad, 0)
}

func (e *Envelope) payloadAAD() []byte {
	aad := e.slotAAD("payload")
	aad = append(aad, e.password.nonce...)
	aad = append(aad, e.password.ciphertext...)
	if e.recovery != nil {
		aad = append(aad, e.recovery.nonce...)
		aad = append(aad, e.recovery.ciphertext...)
	}
	return aad
}

func (e *Envelope) nonces() [][]byte {
	nonces := [][]byte{e.password.nonce, e.payload.nonce}
	if e.recovery != nil {
		nonces = append(nonces, e.recovery.nonce)
	}
	return nonces
}

func (e *Envelope) validate() error {
	if e == nil || len(e.vaultID) != identityBytes || len(e.generation) != identityBytes || len(e.salt) != saltBytes {
		return ErrFormat
	}
	if !validSealed(e.password, KeyBytes+tagBytes, KeyBytes+tagBytes) || !validSealed(e.payload, tagBytes, MaxPayloadBytes+tagBytes) {
		return ErrFormat
	}
	if e.recovery != nil && !validSealed(*e.recovery, KeyBytes+tagBytes, KeyBytes+tagBytes) {
		return ErrFormat
	}
	nonces := e.nonces()
	for i := range nonces {
		for j := 0; j < i; j++ {
			if bytes.Equal(nonces[i], nonces[j]) {
				return ErrFormat
			}
		}
	}
	return nil
}

func validSealed(s sealedData, minBytes, maxBytes int) bool {
	return len(s.nonce) == nonceBytes && len(s.ciphertext) >= minBytes && len(s.ciphertext) <= maxBytes
}

func randomBytes(random io.Reader, size int) ([]byte, error) {
	buf := make([]byte, size)
	if _, err := io.ReadFull(random, buf); err != nil {
		clear(buf)
		return nil, errors.New("credential vault random source failed")
	}
	return buf, nil
}

func newAEAD(key []byte) (cipher.AEAD, error) {
	if len(key) != KeyBytes {
		return nil, ErrAuthentication
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, ErrAuthentication
	}
	return cipher.NewGCM(block)
}

func seal(random io.Reader, key, plaintext, aad []byte, excluded ...[]byte) (sealedData, error) {
	aead, err := newAEAD(key)
	if err != nil {
		return sealedData{}, err
	}
	// A bounded retry also fails closed for a broken/repeating random source.
	for attempt := 0; attempt < 8; attempt++ {
		nonce, err := randomBytes(random, nonceBytes)
		if err != nil {
			return sealedData{}, err
		}
		duplicate := false
		for _, old := range excluded {
			duplicate = duplicate || bytes.Equal(nonce, old)
		}
		if !duplicate {
			return sealedData{nonce: nonce, ciphertext: aead.Seal(nil, nonce, plaintext, aad)}, nil
		}
	}
	return sealedData{}, errors.New("credential vault random source repeated a nonce")
}

func open(key []byte, sealed sealedData, aad []byte) ([]byte, error) {
	aead, err := newAEAD(key)
	if err != nil {
		return nil, ErrAuthentication
	}
	if len(sealed.nonce) != aead.NonceSize() {
		return nil, ErrFormat
	}
	plaintext, err := aead.Open(nil, sealed.nonce, sealed.ciphertext, aad)
	if err != nil {
		clear(plaintext)
		return nil, ErrAuthentication
	}
	return plaintext, nil
}
