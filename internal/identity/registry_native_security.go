package identity

import (
	"errors"
	"fmt"
	"strings"
)

// WINHTTP_CALLBACK_STATUS_SECURE_FAILURE values from the Windows SDK.
// https://learn.microsoft.com/en-us/windows/win32/api/winhttp/nc-winhttp-winhttp_status_callback
const (
	registrySecureRevocationUnknown = 0x00000001
	registrySecureInvalidCert       = 0x00000002
	registrySecureRevoked           = 0x00000004
	registrySecureUntrustedCA       = 0x00000008
	registrySecureWrongHost         = 0x00000010
	registrySecureInvalidDate       = 0x00000020
	registrySecureChannel           = 0x80000000
)

type registryNativeSecureError struct {
	operation string
	flags     uint32
	cause     error
}

func (failure *registryNativeSecureError) Error() string {
	reasons := []string{}
	for _, item := range []struct {
		flag uint32
		text string
	}{
		{registrySecureRevocationUnknown, "certificate revocation status could not be checked"},
		{registrySecureInvalidCert, "the server certificate is invalid"},
		{registrySecureRevoked, "the server certificate has been revoked"},
		{registrySecureUntrustedCA, "the certificate authority is not trusted by Windows"},
		{registrySecureWrongHost, "the certificate does not match standards-oui.ieee.org"},
		{registrySecureInvalidDate, "the certificate is expired or not yet valid; check the PC date and time"},
		{registrySecureChannel, "Windows could not establish the TLS security channel"},
	} {
		if failure.flags&item.flag != 0 {
			reasons = append(reasons, item.text)
		}
	}
	const known = registrySecureRevocationUnknown | registrySecureInvalidCert | registrySecureRevoked | registrySecureUntrustedCA | registrySecureWrongHost | registrySecureInvalidDate | registrySecureChannel
	if failure.flags&^uint32(known) != 0 {
		reasons = append(reasons, "Windows reported an additional TLS failure")
	}
	if len(reasons) == 0 {
		reasons = append(reasons, "Windows rejected HTTPS without providing certificate or handshake details")
	}
	return fmt.Sprintf("%s: %s (Windows error 12175; TLS flags 0x%08X)", failure.operation, strings.Join(reasons, "; "), failure.flags)
}

func (failure *registryNativeSecureError) Unwrap() error { return failure.cause }

func registryNativeChannelFailure(err error) bool {
	var failure *registryNativeSecureError
	// A missing reason, any certificate failure or unknown flag must not be
	// mistaken for a protocol negotiation problem.
	return errors.As(err, &failure) && failure.flags == registrySecureChannel
}
