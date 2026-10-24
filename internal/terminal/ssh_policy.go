package terminal

import (
	"errors"
	"slices"

	"golang.org/x/crypto/ssh"
	"networktoolkit/internal/sshpolicy"
)

// sshAlgorithmPolicy starts with the existing modern policy for every
// connection. Legacy is an explicit per-connection opt-in, never a retry or a
// process-wide default. Keep this list finite rather than adding everything in
// ssh.InsecureAlgorithms. Certificates and host-key trust are unchanged.
func sshAlgorithmPolicy(compatibility string) ssh.Algorithms {
	return sshpolicy.Algorithms(compatibility)
}

// This typed diagnosis preserves the original error and offered algorithm
// lists. It means the explicit compatibility profile has an alternative for
// this failed category, not that a retry will necessarily authenticate.
type sshLegacyRequiredError struct{ cause error }

func (e *sshLegacyRequiredError) Error() string { return e.cause.Error() }
func (e *sshLegacyRequiredError) Unwrap() error { return e.cause }

func diagnoseSSHHandshake(err error, compatibility string) error {
	if err == nil || compatibility == SSHCompatibilityLegacy {
		return err
	}
	var negotiation *ssh.AlgorithmNegotiationError
	if !errors.As(err, &negotiation) {
		return err
	}
	var additions []string
	switch negotiation.What {
	case "key exchange":
		additions = []string{ssh.InsecureKeyExchangeDH14SHA1, ssh.InsecureKeyExchangeDHGEXSHA1, ssh.InsecureKeyExchangeDH1SHA1}
	case "host key":
		additions = []string{ssh.KeyAlgoRSA}
	case "client to server cipher", "server to client cipher":
		additions = []string{ssh.InsecureCipherAES128CBC}
	default:
		return err
	}
	for _, algorithm := range negotiation.RequestedAlgorithms {
		if slices.Contains(additions, algorithm) {
			return &sshLegacyRequiredError{cause: err}
		}
	}
	return err
}
