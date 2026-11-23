package terminal

import (
	"errors"
	"strings"
)

// sshPasswordAuthentication shares one automatic password use across the two
// SSH methods. It belongs only to the live handshake, never reconnect settings.
type sshPasswordAuthentication struct {
	secret        string
	allowed       bool
	used          bool
	challengeSeen bool
}

func newSSHPasswordAuthentication(method, password string) *sshPasswordAuthentication {
	a := &sshPasswordAuthentication{allowed: method == "password"}
	if a.allowed {
		a.secret = password
	}
	return a
}

func (a *sshPasswordAuthentication) takePassword() (string, error) {
	if !a.allowed || a.used {
		return "", errors.New("SSH login password was already submitted or is unavailable")
	}
	a.used = true
	password := a.secret
	a.secret = ""
	return password, nil
}

func (a *sshPasswordAuthentication) answerChallenge(name, instruction string, questions []string, echo []bool) ([]string, bool) {
	first := !a.challengeSeen
	a.challengeSeen = true
	// Even a zero-question notice closes this opportunity: a later question may
	// be another factor. Unknown or interactive questions retain the normal UI.
	if !first || !a.allowed || a.used || a.secret == "" || !plainSSHPasswordChallenge(name, instruction, questions, echo) {
		return nil, false
	}
	password, err := a.takePassword()
	if err != nil {
		return nil, false
	}
	return []string{password}, true
}

func (a *sshPasswordAuthentication) clear() {
	a.secret = ""
	a.allowed = false
	a.used = true
	a.challengeSeen = true
}

func plainSSHPasswordChallenge(name, instruction string, questions []string, echo []bool) bool {
	if len(name) > 1024 || len(questions) != 1 || len(echo) != 1 || echo[0] {
		return false
	}
	switch strings.ToLower(strings.TrimSpace(questions[0])) {
	case "password", "password:":
	default:
		return false
	}
	// The protocol does not identify a password question. Accept only familiar
	// generic wording; MFA, password changes and unknown text stay interactive.
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "", "password", "authentication", "password authentication", "ssh authentication", "pam", "pam authentication":
	default:
		return false
	}
	switch strings.ToLower(strings.TrimSpace(instruction)) {
	case "", "password", "password:", "password authentication", "password authentication:",
		"enter your password", "enter your password:", "enter your password.",
		"please enter your password", "please enter your password:", "please enter your password.":
		return true
	default:
		return false
	}
}
