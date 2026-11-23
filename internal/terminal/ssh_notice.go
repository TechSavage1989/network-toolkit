package terminal

import (
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"
)

const sshServerNameLimit = 1024

// The SSH name is an untrusted server heading, separate from the app's title.
// Truncate only this display copy, never the raw context used to recognize a
// password challenge. RFC 4256 permits visibly marked name truncation.
func sshAuthenticationServerName(name string, req ConnectRequest) string {
	name = redactSSHAuthenticationText(name, req)
	name = sshAuthenticationDisplayText(name, false)
	const suffix = " [truncated]"
	if len(name) > sshServerNameLimit {
		end := sshServerNameLimit - len(suffix)
		for end > 0 && !utf8.RuneStart(name[end]) {
			end--
		}
		name = name[:end] + suffix
	}
	return strings.TrimSpace(name)
}

// Notice text bypasses the prompt UI only after its original size is validated.
// Frame every server-controlled line and generate all terminal line endings
// ourselves: server escape sequences, carriage returns and bidi controls must
// never acquire terminal behavior just because a notice was auto-continued.
func sshAuthenticationNoticeOutput(name, instruction string, req ConnectRequest) []byte {
	heading := sshAuthenticationServerName(name, req)
	message := sshAuthenticationDisplayText(redactSSHAuthenticationText(instruction, req), true)
	if heading == "" && strings.TrimSpace(message) == "" {
		return nil
	}
	var notice strings.Builder
	notice.WriteString("\r\n[SSH sign-in notice - automatically continued]\r\n")
	if heading != "" {
		notice.WriteString("[Server heading] ")
		notice.WriteString(heading)
		notice.WriteString("\r\n")
	}
	if message != "" {
		for _, line := range strings.Split(message, "\n") {
			notice.WriteString("[Server notice] ")
			notice.WriteString(line)
			notice.WriteString("\r\n")
		}
	}
	notice.WriteString("[End SSH sign-in notice]\r\n")
	return []byte(notice.String())
}

// Redact credentials already held by this synchronous handshake. Do not retain
// manual answers or another credential copy for subsequent notice processing.
func redactSSHAuthenticationText(text string, req ConnectRequest) string {
	secrets := []string{req.Password, req.Passphrase, req.PrivateKey}
	slices.SortFunc(secrets, func(a, b string) int { return len(b) - len(a) })
	var replacements []string
	for _, secret := range secrets {
		if secret != "" {
			replacements = append(replacements, secret, "[redacted]")
		}
	}
	if len(replacements) == 0 {
		return text
	}
	// A single pass prevents another short secret from repeatedly expanding the
	// replacement marker and keeps output growth bounded by the original input.
	return strings.NewReplacer(replacements...).Replace(text)
}

func sshAuthenticationDisplayText(text string, multiline bool) string {
	text = strings.ReplaceAll(text, "\r\n", "\n")
	var safe strings.Builder
	safe.Grow(len(text))
	for _, r := range text {
		switch {
		case r == '\r' || r == '\n' || r == '\u2028' || r == '\u2029':
			if multiline {
				safe.WriteByte('\n')
			} else {
				safe.WriteByte(' ')
			}
		case r == '\t':
			safe.WriteByte(' ')
		case unicode.IsControl(r) || unicode.Is(unicode.Cf, r):
			safe.WriteRune(unicode.ReplacementChar)
		default:
			safe.WriteRune(r)
		}
	}
	return safe.String()
}
