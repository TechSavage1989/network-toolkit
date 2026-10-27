package desktop

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"unicode"
)

// CredentialDestinationConfirmation contains metadata only. Never add a
// password, private key, passphrase, recovery key or vault unlock secret here.
type CredentialDestinationConfirmation struct {
	Action         string
	CredentialName string
	Protocol       string
	Host           string
	Port           int
	Username       string
	Fingerprint    string
}

func credentialConfirmationText(request CredentialDestinationConfirmation) (string, error) {
	if request.Action != "allow" && request.Action != "remove" {
		return "", errors.New("Unsupported credential destination action")
	}
	if request.Port < 1 || request.Port > 65535 {
		return "", errors.New("Invalid credential destination port")
	}
	if request.Protocol != "ssh" && request.Protocol != "ftp-explicit" && request.Protocol != "ftp-implicit" {
		return "", errors.New("Unsupported credential destination protocol")
	}
	if request.Protocol == "ssh" && request.Fingerprint == "" {
		return "", errors.New("SSH credential destination approval requires a verified server fingerprint")
	}
	for _, field := range []struct{ value string; maximum int; required bool }{
		{request.CredentialName, 120, true}, {request.Protocol, 32, true},
		{request.Host, 512, true}, {request.Username, 256, false}, {request.Fingerprint, 128, false},
	} {
		if len(field.value) > field.maximum || (field.required && strings.TrimSpace(field.value) == "") || strings.IndexFunc(field.value, unicode.IsControl) >= 0 {
			return "", errors.New("Invalid credential destination metadata")
		}
	}
	verb := "Allow this saved credential to authenticate to the following destination?"
	if request.Action == "remove" {
		verb = "Remove this approved destination from the saved credential?"
	}
	identity := strconv.QuoteToASCII(request.Fingerprint)
	identityAdvice := "Verify this fingerprint using a trusted independent source before approving."
	if request.Fingerprint == "" {
		identity = "Normal TLS certificate and hostname verification (no custom pin)"
		identityAdvice = "The server must pass normal TLS certificate and hostname verification."
	}
	// ASCII quoting makes invisible/bidirectional Unicode and delimiters visible
	// instead of allowing attacker-controlled metadata to impersonate UI labels.
	return fmt.Sprintf("%s\n\nCredential: %s\nProtocol: %s\nHost: %s\nPort: %d\nUsername: %s\nServer identity: %s\n\n%s\n\nApprove only if you initiated this exact change in Network Toolkit. This does not change an existing remote password or disconnect an existing session.", verb,
		strconv.QuoteToASCII(request.CredentialName), strconv.QuoteToASCII(request.Protocol), strconv.QuoteToASCII(request.Host), request.Port, strconv.QuoteToASCII(request.Username), identity, identityAdvice), nil
}

func validateCredentialConfirmationContext(ctx context.Context) error {
	if ctx == nil {
		return errors.New("Credential confirmation context is unavailable")
	}
	return ctx.Err()
}

// At most one native security decision is visible. After cancellation, keep
// this slot until the native prompt closes, but irrevocably discard its result.
var credentialConfirmationSlot = make(chan struct{}, 1)

func confirmCredentialDestination(ctx context.Context, request CredentialDestinationConfirmation, prompt func(string) (bool, error)) (bool, error) {
	if err := validateCredentialConfirmationContext(ctx); err != nil { return false, err }
	text, err := credentialConfirmationText(request)
	if err != nil { return false, err }
	if prompt == nil { return false, errors.New("Native credential confirmation is unavailable") }
	select {
	case credentialConfirmationSlot <- struct{}{}:
	default:
		return false, errors.New("Another native credential confirmation is already open")
	}
	type decision struct { allowed bool; err error }
	done := make(chan decision, 1)
	go func() {
		result := decision{}
		defer func() { <-credentialConfirmationSlot; done <- result }()
		if err := ctx.Err(); err != nil { result = decision{err: err}; return }
		allowed, err := prompt(text)
		if cancelled := ctx.Err(); cancelled != nil { result = decision{err: cancelled}; return }
		result = decision{allowed: allowed && err == nil, err: err}
	}()
	select {
	case <-ctx.Done(): return false, ctx.Err()
	case result := <-done:
		if err := ctx.Err(); err != nil { return false, err }
		return result.allowed, result.err
	}
}
