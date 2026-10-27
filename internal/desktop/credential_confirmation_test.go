package desktop

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestCredentialConfirmationUsesExactQuotedMetadata(t *testing.T) {
	request := CredentialDestinationConfirmation{Action: "allow", CredentialName: "Switch \u202eadmin", Protocol: "ssh", Host: "router.example", Port: 2222, Username: "operator", Fingerprint: "SHA256:synthetic-fixture-fingerprint"}
	message, err := credentialConfirmationText(request)
	if err != nil || !strings.Contains(message, `"Switch \u202eadmin"`) || !strings.Contains(message, `Host: "router.example"`) || !strings.Contains(message, "Port: 2222") || !strings.Contains(message, `Username: "operator"`) || !strings.Contains(message, `Server identity: "SHA256:synthetic-fixture-fingerprint"`) || strings.ContainsRune(message, '\u202e') {
		t.Fatalf("native metadata confirmation was not safely formed: %v", err)
	}
	if !strings.Contains(message, "Verify this fingerprint using a trusted independent source before approving.") {
		t.Fatal("native approval omitted independent fingerprint verification guidance")
	}
	request.Action = "remove"
	if message, err = credentialConfirmationText(request); err != nil || !strings.HasPrefix(message, "Remove") {
		t.Fatal("removal did not have distinct native consent text")
	}
	request.Protocol, request.Fingerprint = "ftp-explicit", ""
	if message, err = credentialConfirmationText(request); err != nil || !strings.Contains(message, "Normal TLS certificate and hostname verification") {
		t.Fatal("unpinned FTPS verification was not explained")
	}
}

func TestCredentialConfirmationCancellationAndDenialCannotApprove(t *testing.T) {
	request := CredentialDestinationConfirmation{Action: "allow", CredentialName: "Synthetic fixture", Protocol: "ssh", Host: "example.invalid", Port: 22, Fingerprint: "SHA256:synthetic-fixture-fingerprint"}
	for _, fixture := range []struct {
		allow bool
		err   error
	}{{false, nil}, {true, errors.New("native fixture failed")}} {
		allowed, err := confirmCredentialDestination(context.Background(), request, func(string) (bool, error) { return fixture.allow, fixture.err })
		if allowed || (fixture.err != nil && err == nil) {
			t.Fatal("native denial/error approved destination")
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	entered, release, finished := make(chan struct{}), make(chan struct{}), make(chan struct{})
	defer cancel()
	result := make(chan bool, 1)
	go func() {
		allowed, err := confirmCredentialDestination(ctx, request, func(string) (bool, error) { close(entered); <-release; close(finished); return true, nil })
		result <- allowed || err == nil
	}()
	select {
	case <-entered:
	case <-time.After(time.Second):
		t.Fatal("fixture native prompt did not start")
	}
	cancel()
	select {
	case bad := <-result:
		if bad {
			t.Fatal("cancelled request accepted native consent")
		}
	case <-time.After(time.Second):
		t.Fatal("cancelled confirmation did not return")
	}
	if allowed, err := confirmCredentialDestination(context.Background(), request, func(string) (bool, error) { t.Error("queued a second native prompt"); return true, nil }); allowed || err == nil {
		t.Fatal("pending native prompt did not serialize decisions")
	}
	close(release)
	<-finished
	// Await the bounded goroutine's slot release before another test invokes the
	// shared native entrypoint; no user prompt or platform UI is opened here.
	deadline := time.Now().Add(time.Second)
	for len(credentialConfirmationSlot) != 0 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if len(credentialConfirmationSlot) != 0 {
		t.Fatal("cancelled native prompt did not release its slot")
	}
}

func TestCredentialConfirmationRejectsInvalidMetadataAndCancelledContext(t *testing.T) {
	base := CredentialDestinationConfirmation{Action: "allow", CredentialName: "Core", Protocol: "ssh", Host: "router.example", Port: 22, Fingerprint: "SHA256:synthetic-fixture-fingerprint"}
	mutations := []func(*CredentialDestinationConfirmation){
		func(r *CredentialDestinationConfirmation) { r.Action = "execute" },
		func(r *CredentialDestinationConfirmation) { r.Port = 0 },
		func(r *CredentialDestinationConfirmation) { r.Port = 65536 },
		func(r *CredentialDestinationConfirmation) { r.Host = "router\nHost: attacker" },
		func(r *CredentialDestinationConfirmation) { r.CredentialName = strings.Repeat("a", 121) },
		func(r *CredentialDestinationConfirmation) { r.Protocol = "" },
		func(r *CredentialDestinationConfirmation) { r.Fingerprint = strings.Repeat("x", 129) },
		func(r *CredentialDestinationConfirmation) { r.Fingerprint = "fingerprint\nYes" },
		func(r *CredentialDestinationConfirmation) { r.Fingerprint = "" },
	}
	for _, mutate := range mutations {
		request := base
		mutate(&request)
		if _, err := credentialConfirmationText(request); err == nil {
			t.Fatal("invalid native consent metadata accepted")
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if allowed, err := ConfirmCredentialDestination(ctx, base); allowed || err == nil {
		t.Fatal("cancelled confirmation did not fail closed")
	}
	if err := validateCredentialConfirmationContext(nil); err == nil {
		t.Fatal("nil confirmation context accepted")
	}
}
