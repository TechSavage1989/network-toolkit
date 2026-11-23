package main

import (
	"bytes"
	"encoding/json"
	"strings"

	"networktoolkit/internal/terminal"
)

// Old profiles without this field adopt the new default. Keep any saved off
// choice and do not treat malformed imported values as permission to continue.
func savedSSHNoticePreference(profile map[string]any) bool {
	value, present := profile["sshAutoContinueNotices"]
	if !present {
		return true
	}
	enabled, valid := value.(bool)
	return valid && enabled
}

func (p *terminalWindowPreset) UnmarshalJSON(data []byte) error {
	type plainPreset terminalWindowPreset
	var decoded plainPreset
	wire := struct {
		*plainPreset
		Notice terminal.SSHNoticePreferenceJSON `json:"sshAutoContinueNotices"`
	}{plainPreset: &decoded}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&wire); err != nil {
		return err
	}
	kind := strings.ToLower(strings.TrimSpace(decoded.Type))
	decoded.SSHAutoContinueNotices = (kind == "" || kind == "ssh") && wire.Notice.Enabled()
	*p = terminalWindowPreset(decoded)
	return nil
}

func (r *remoteBrowserRequest) UnmarshalJSON(data []byte) error {
	type plainRequest remoteBrowserRequest
	var decoded plainRequest
	wire := struct {
		*plainRequest
		Notice terminal.SSHNoticePreferenceJSON `json:"sshAutoContinueNotices"`
	}{plainRequest: &decoded}
	if err := json.Unmarshal(data, &wire); err != nil {
		return err
	}
	decoded.sshNoticePreferencePresent = wire.Notice.Present()
	if decoded.sshNoticePreferencePresent {
		decoded.SSHAutoContinueNotices = wire.Notice.Enabled()
	}
	*r = remoteBrowserRequest(decoded)
	return nil
}

// Apply the default only when opening a new SSH transport. Existing-session
// file operations must remain free of fresh connection/authentication fields.
func (r remoteBrowserRequest) sshNoticeContinuation() bool {
	return !r.sshNoticePreferencePresent || r.SSHAutoContinueNotices
}
