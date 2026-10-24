//go:build !windows

package platform

import (
	"context"
	"crypto/x509"
	"errors"
	"runtime"
)

func verifyOfflinePlatformCertificate(ctx context.Context, certificates []*x509.Certificate, serverName string) error {
	if runtime.GOOS == "darwin" || runtime.GOOS == "ios" {
		return errors.New("offline source-bound certificate verification is not implemented on this platform")
	}
	// Linux and the other pure-Go verifier platforms load local certificate
	// files. The Go chain verifier does not retrieve missing issuers or CRLs.
	roots, err := x509.SystemCertPool()
	if err != nil {
		return err
	}
	return verifyOfflineCertificateChain(ctx, certificates, serverName, roots)
}
