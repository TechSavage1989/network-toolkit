//go:build windows

package platform

import (
	"bytes"
	"context"
	"crypto/x509"
	"errors"
	"fmt"
	"runtime"
	"syscall"
	"unsafe"
)

// Retain ordinary Windows trust policy and alternate-chain selection, while
// prohibiting issuer/root/CTL network retrieval and any revocation URL fetch.
// No revocation-check flags are added: this preserves Go's existing TLS policy.
// https://learn.microsoft.com/windows/win32/api/wincrypt/nf-wincrypt-certgetcertificatechain
const offlineWindowsChainFlags uint32 = 0x00000004 | 0x00000080 | 0x00000100 | 0x80000000

func offlineWindowsChainParameters() (syscall.CertChainPara, []*byte) {
	usage := []*byte{&syscall.OID_PKIX_KP_SERVER_AUTH[0]}
	parameters := syscall.CertChainPara{Size: uint32(unsafe.Sizeof(syscall.CertChainPara{}))}
	parameters.RequestedUsage.Type = syscall.USAGE_MATCH_TYPE_AND
	parameters.RequestedUsage.Usage.Length = 1
	parameters.RequestedUsage.Usage.UsageIdentifiers = &usage[0]
	return parameters, usage
}

func verifyOfflinePlatformCertificate(ctx context.Context, certificates []*x509.Certificate, serverName string) error {
	store, err := syscall.CertOpenStore(syscall.CERT_STORE_PROV_MEMORY, 0, 0, syscall.CERT_STORE_DEFER_CLOSE_UNTIL_LAST_FREE_FLAG, 0)
	if err != nil {
		return fmt.Errorf("open temporary certificate store: %w", err)
	}
	defer syscall.CertCloseStore(store, 0)
	var leaf *syscall.CertContext
	for i, certificate := range certificates {
		if err := ctx.Err(); err != nil {
			return err
		}
		parsed, err := syscall.CertCreateCertificateContext(syscall.X509_ASN_ENCODING|syscall.PKCS_7_ASN_ENCODING, &certificate.Raw[0], uint32(len(certificate.Raw)))
		if err != nil {
			return fmt.Errorf("read server certificate: %w", err)
		}
		var stored **syscall.CertContext
		if i == 0 {
			stored = &leaf
		}
		err = syscall.CertAddCertificateContextToStore(store, parsed, syscall.CERT_STORE_ADD_ALWAYS, stored)
		syscall.CertFreeCertificateContext(parsed)
		if err != nil {
			return fmt.Errorf("add server certificate to temporary store: %w", err)
		}
		if i == 0 {
			defer syscall.CertFreeCertificateContext(leaf)
		}
	}
	parameters, usage := offlineWindowsChainParameters()
	var chain *syscall.CertChainContext
	err = syscall.CertGetCertificateChain(0, leaf, nil, store, &parameters, offlineWindowsChainFlags, 0, &chain)
	runtime.KeepAlive(usage)
	runtime.KeepAlive(certificates)
	if err != nil {
		return fmt.Errorf("build certificate chain from local stores and cache: %w", err)
	}
	if chain == nil {
		return errors.New("Windows returned no certificate chain")
	}
	defer syscall.CertFreeCertificateChain(chain)
	firstErr := verifyOfflineWindowsChain(ctx, chain, certificates[0], serverName)
	if firstErr == nil {
		return nil
	}
	if chain.LowerQualityChainCount > 64 || (chain.LowerQualityChainCount > 0 && chain.LowerQualityChains == nil) {
		return errors.New("Windows returned an invalid alternate certificate chain list")
	}
	for _, candidate := range unsafe.Slice(chain.LowerQualityChains, int(chain.LowerQualityChainCount)) {
		if err := verifyOfflineWindowsChain(ctx, candidate, certificates[0], serverName); err == nil {
			return nil
		}
		if err := ctx.Err(); err != nil {
			return err
		}
	}
	return firstErr
}

func verifyOfflineWindowsChain(ctx context.Context, chain *syscall.CertChainContext, leaf *x509.Certificate, serverName string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if chain == nil || chain.ChainCount == 0 || chain.ChainCount > 64 || chain.Chains == nil {
		return errors.New("Windows returned an incomplete certificate chain")
	}
	if status := chain.TrustStatus.ErrorStatus; status != 0 {
		return fmt.Errorf("Windows certificate trust check failed (0x%08X); required issuer certificates or trust data may be unavailable locally", status)
	}
	name, err := syscall.UTF16PtrFromString(serverName)
	if err != nil {
		return err
	}
	ssl := syscall.SSLExtraCertChainPolicyPara{Size: uint32(unsafe.Sizeof(syscall.SSLExtraCertChainPolicyPara{})), AuthType: syscall.AUTHTYPE_SERVER, ServerName: name}
	policy := syscall.CertChainPolicyPara{Size: uint32(unsafe.Sizeof(syscall.CertChainPolicyPara{})), ExtraPolicyPara: syscall.Pointer(unsafe.Pointer(&ssl))}
	status := syscall.CertChainPolicyStatus{Size: uint32(unsafe.Sizeof(syscall.CertChainPolicyStatus{}))}
	err = syscall.CertVerifyCertificateChainPolicy(syscall.CERT_CHAIN_POLICY_SSL, chain, &policy, &status)
	runtime.KeepAlive(ssl)
	runtime.KeepAlive(name)
	if err != nil {
		return fmt.Errorf("check Windows SSL server policy: %w", err)
	}
	if status.Error != 0 {
		return fmt.Errorf("Windows SSL server policy rejected the certificate: %w", syscall.Errno(status.Error))
	}
	// Recheck the Windows-trusted chain with Go using an explicit ordinary pool.
	// This retains Go's signature/constraint/distrust checks without triggering
	// its normal Windows verifier or trusting a root solely because a peer sent it.
	simple := unsafe.Slice(chain.Chains, int(chain.ChainCount))[0]
	if simple == nil || simple.NumElements == 0 || simple.NumElements > 128 || simple.Elements == nil {
		return errors.New("Windows returned an incomplete server certificate path")
	}
	path := make([]*x509.Certificate, 0, simple.NumElements)
	for _, element := range unsafe.Slice(simple.Elements, int(simple.NumElements)) {
		if element == nil || element.CertContext == nil || element.CertContext.EncodedCert == nil || element.CertContext.Length == 0 || element.CertContext.Length > 4*1024*1024 {
			return errors.New("Windows returned invalid certificate data")
		}
		raw := bytes.Clone(unsafe.Slice(element.CertContext.EncodedCert, int(element.CertContext.Length)))
		certificate, err := x509.ParseCertificate(raw)
		if err != nil {
			return err
		}
		path = append(path, certificate)
	}
	if !bytes.Equal(path[0].Raw, leaf.Raw) {
		return errors.New("Windows certificate path does not match the server certificate")
	}
	roots := x509.NewCertPool()
	roots.AddCert(path[len(path)-1])
	return verifyOfflineCertificateChain(ctx, path, serverName, roots)
}
