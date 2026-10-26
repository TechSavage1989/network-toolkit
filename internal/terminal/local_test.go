package terminal

import (
	"context"
	"testing"
)

func TestLocalShellRequestDiscardsRemoteFields(t *testing.T) {
	for _, kind := range []string{"cmd", "powershell"} {
		req, err := normalizeRequest(ConnectRequest{Type: kind, ProfileID: "local-profile", Host: "remote.invalid", Port: 22, Username: "someone", Password: "hidden", PrivateKey: "hidden", Passphrase: "hidden", AuthMethod: "key", SerialPort: "COM1", SerialDTR: new(bool)})
		if err != nil {
			t.Fatal(err)
		}
		if req.Host != "" || req.Port != 0 || req.Username != "" || req.Password != "" || req.PrivateKey != "" || req.Passphrase != "" || req.AuthMethod != "" || req.SerialPort != "" || req.SerialDTR != nil {
			t.Fatalf("remote fields retained: %#v", req)
		}
		if req.ProfileID != "local-profile" || req.Cols != 80 || req.Rows != 24 || req.LineEnding != "cr" {
			t.Fatalf("local defaults: %#v", req)
		}
		session := newLiveSession(context.Background(), "test", req)
		defer session.cancel()
		if session.info.Host != "" || session.info.Port != 0 || session.info.Label == "" {
			t.Fatalf("local endpoint: %#v", session.info)
		}
	}
}
