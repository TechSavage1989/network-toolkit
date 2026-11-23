package terminal

import (
	"bytes"
	"encoding/json"
	"errors"
	"strings"
)

// SSHNoticePreferenceJSON is a decode-only field used by connection schemas.
// Missing fields adopt the default; each supplied occurrence must be a boolean.
// Validating every occurrence also rejects a malformed earlier duplicate.
type SSHNoticePreferenceJSON struct {
	enabled bool
	present bool
}

func (p *SSHNoticePreferenceJSON) UnmarshalJSON(raw []byte) error {
	switch string(bytes.TrimSpace(raw)) {
	case "true":
		p.enabled = true
	case "false":
		p.enabled = false
	default:
		return errors.New("sshAutoContinueNotices must be a boolean")
	}
	p.present = true
	return nil
}

func (p SSHNoticePreferenceJSON) Enabled() bool { return !p.present || p.enabled }
func (p SSHNoticePreferenceJSON) Present() bool { return p.present }

// UnmarshalJSON distinguishes the new-connection default from an explicit
// false. Keep the terminal API's unknown-field check inside this decoder:
// an outer Decoder cannot enforce it once a type implements UnmarshalJSON.
func (r *ConnectRequest) UnmarshalJSON(data []byte) error {
	type plainRequest ConnectRequest
	var decoded plainRequest
	wire := struct {
		*plainRequest
		Notice SSHNoticePreferenceJSON `json:"sshAutoContinueNotices"`
	}{plainRequest: &decoded}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&wire); err != nil {
		return err
	}
	decoded.SSHAutoContinueNotices = strings.EqualFold(strings.TrimSpace(decoded.Type), "ssh") && wire.Notice.Enabled()
	*r = ConnectRequest(decoded)
	return nil
}
