package platform

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"errors"
	"math/big"
	"testing"
	"time"
)

func offlineCertificateFixture(t *testing.T, changeLeaf func(*x509.Certificate)) ([]*x509.Certificate, *x509.CertPool) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	rootTemplate := &x509.Certificate{SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "Offline fixture root"}, NotBefore: now.Add(-time.Hour), NotAfter: now.Add(time.Hour), IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign | x509.KeyUsageCRLSign}
	rootDER, err := x509.CreateCertificate(rand.Reader, rootTemplate, rootTemplate, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	root, err := x509.ParseCertificate(rootDER)
	if err != nil {
		t.Fatal(err)
	}
	leafTemplate := &x509.Certificate{SerialNumber: big.NewInt(2), DNSNames: []string{"fixture.example"}, NotBefore: now.Add(-time.Hour), NotAfter: now.Add(time.Hour), KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}}
	if changeLeaf != nil {
		changeLeaf(leafTemplate)
	}
	leafDER, err := x509.CreateCertificate(rand.Reader, leafTemplate, root, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	leaf, err := x509.ParseCertificate(leafDER)
	if err != nil {
		t.Fatal(err)
	}
	roots := x509.NewCertPool()
	roots.AddCert(root)
	return []*x509.Certificate{leaf, root}, roots
}

func TestOfflineCertificateValidationRetainsServerChecks(t *testing.T) {
	ctx := context.Background()
	certificates, roots := offlineCertificateFixture(t, nil)
	if err := verifyOfflineCertificateChain(ctx, certificates, "fixture.example", roots); err != nil {
		t.Fatalf("valid local chain: %v", err)
	}
	if err := verifyOfflineCertificateChain(ctx, certificates, "different.example", roots); err == nil {
		t.Fatal("wrong server hostname was accepted")
	}
	if err := verifyOfflineCertificateChain(ctx, certificates, "fixture.example", x509.NewCertPool()); err == nil {
		t.Fatal("peer-supplied untrusted root was accepted")
	}
	for _, change := range []func(*x509.Certificate){
		func(leaf *x509.Certificate) { leaf.NotAfter = time.Now().Add(-time.Minute) },
		func(leaf *x509.Certificate) { leaf.NotBefore = time.Now().Add(time.Minute) },
		func(leaf *x509.Certificate) { leaf.ExtKeyUsage = []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth} },
	} {
		certificates, roots := offlineCertificateFixture(t, change)
		if err := verifyOfflineCertificateChain(ctx, certificates, "fixture.example", roots); err == nil {
			t.Fatal("invalid validity period or server usage was accepted")
		}
	}
	certificates[0].Signature[0] ^= 1
	if err := verifyOfflineCertificateChain(ctx, certificates, "fixture.example", roots); err == nil {
		t.Fatal("invalid certificate signature was accepted")
	}
}

func TestOfflineCertificateValidationInputAndCancellation(t *testing.T) {
	certificates, roots := offlineCertificateFixture(t, nil)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := VerifyOfflineServerCertificate(ctx, certificates, "fixture.example"); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled public validation: %v", err)
	}
	if err := verifyOfflineCertificateChain(ctx, certificates, "fixture.example", roots); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled chain validation: %v", err)
	}
	for _, chain := range [][]*x509.Certificate{nil, {nil}, {&x509.Certificate{}}} {
		if err := VerifyOfflineServerCertificate(context.Background(), chain, "fixture.example"); err == nil {
			t.Fatal("missing certificate input accepted")
		}
	}
	for _, name := range []string{"", "fixture.example\x00other", "different.example"} {
		if err := VerifyOfflineServerCertificate(context.Background(), certificates, name); err == nil {
			t.Fatalf("invalid server name %q accepted", name)
		}
	}
}
