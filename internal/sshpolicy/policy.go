// Package sshpolicy defines one explicit SSH algorithm policy shared by the
// interactive terminal and standalone SFTP client. Never silently downgrade.
package sshpolicy

import (
	"golang.org/x/crypto/ssh"
	"strings"
)

func Algorithms(compatibility string) ssh.Algorithms {
	algorithms := ssh.SupportedAlgorithms()
	hostKeys := make([]string, 0, len(algorithms.HostKeys))
	for _, name := range algorithms.HostKeys {
		if !strings.Contains(name, "-cert-") {
			hostKeys = append(hostKeys, name)
		}
	}
	algorithms.HostKeys = hostKeys
	if compatibility == "legacy" {
		algorithms.KeyExchanges = append(algorithms.KeyExchanges, ssh.InsecureKeyExchangeDH14SHA1, ssh.InsecureKeyExchangeDHGEXSHA1, ssh.InsecureKeyExchangeDH1SHA1)
		algorithms.HostKeys = append(algorithms.HostKeys, ssh.KeyAlgoRSA)
		algorithms.Ciphers = append(algorithms.Ciphers, ssh.InsecureCipherAES128CBC)
	}
	return algorithms
}
