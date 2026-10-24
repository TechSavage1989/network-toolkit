package platform

import (
	"context"
	"crypto/x509"
	"errors"
	"fmt"
	"strings"
)

// VerifyOfflineServerCertificate validates a TLS server without fetching roots,
// intermediate certificates, CRLs, or OCSP responses over another network path.
// It is for explicitly source-bound diagnostics. Automatic TLS remains unchanged.
// An unavailable local issuer is a validation failure, never an unverified pass.
func VerifyOfflineServerCertificate(ctx context.Context, certificates []*x509.Certificate, serverName string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if len(certificates) == 0 || len(certificates) > 128 {
		return errors.New("offline certificate validation requires a bounded server certificate chain")
	}
	for _, certificate := range certificates {
		if certificate == nil || len(certificate.Raw) == 0 {
			return errors.New("offline certificate validation received an incomplete certificate")
		}
	}
	serverName = strings.TrimSuffix(serverName, ".")
	if serverName == "" || strings.ContainsRune(serverName, '\x00') {
		return errors.New("offline certificate validation requires the server hostname or IP address")
	}
	if err := certificates[0].VerifyHostname(serverName); err != nil {
		return fmt.Errorf("offline server identity check: %w", err)
	}
	if err := verifyOfflinePlatformCertificate(ctx, certificates, serverName); err != nil {
		return fmt.Errorf("offline certificate validation: %w", err)
	}
	return ctx.Err()
}

// roots must be an ordinary CertPool, not a Windows/macOS SystemCertPool that
// would send Verify back through a platform verifier with URL retrieval enabled.
func verifyOfflineCertificateChain(ctx context.Context, certificates []*x509.Certificate, serverName string, roots *x509.CertPool) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if roots == nil || len(certificates) == 0 {
		return errors.New("no local certificate trust roots or server chain were available")
	}
	intermediates := x509.NewCertPool()
	for _, certificate := range certificates[1:] {
		intermediates.AddCert(certificate)
	}
	_, err := certificates[0].Verify(x509.VerifyOptions{
		DNSName: serverName, Roots: roots, Intermediates: intermediates,
		KeyUsages: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	})
	if ctx.Err() != nil {
		return ctx.Err()
	}
	return err
}
