package terminal

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"

	"golang.org/x/crypto/ssh"
)

func TestSSHBannerLineEndingFixtures(t *testing.T) {
	// Shared with real-xterm browser acceptance. Expected presentation bytes
	// must match the production helper, not a separately implemented JS fix.
	raw, err := os.ReadFile("../../scripts/fixtures/ssh-banner-lines.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixture struct {
		Cases []struct {
			Name       string
			Chunks     []string
			Normalized string
		}
	}
	if err := json.Unmarshal(raw, &fixture); err != nil {
		t.Fatal(err)
	}
	if len(fixture.Cases) < 8 {
		t.Fatal("missing banner regression cases")
	}
	for _, tc := range fixture.Cases {
		t.Run(tc.Name, func(t *testing.T) {
			check := func(chunks []string) {
				t.Helper()
				var normalizer sshBannerNormalizer
				var output []byte
				for _, chunk := range chunks {
					output = append(output, normalizer.normalize(chunk)...)
				}
				if string(output) != tc.Normalized {
					t.Fatalf("banner normalization: got %q, want %q", output, tc.Normalized)
				}
			}
			check(tc.Chunks)
			message := strings.Join(tc.Chunks, "")
			// Every boundary includes splits within UTF-8 and ANSI sequences.
			for split := 0; split <= len(message); split++ {
				check([]string{message[:split], "", message[split:]})
			}
			byteChunks := make([]string, len(message))
			for i := 0; i < len(message); i++ {
				byteChunks[i] = message[i : i+1]
			}
			check(byteChunks)
		})
	}
}

func TestSSHAuthenticationBannerCallbackBeforeRawShellOutput(t *testing.T) {
	f := newSSHFixture(t, func(config *ssh.ServerConfig) {
		config.BannerCallback = func(ssh.ConnMetadata) string {
			return "BANNER first\nBANNER second\r\n\x1b[32mBANNER end\x1b[0m\n"
		}
	}, true)
	m := sshManager(t, t.TempDir())
	req := f.request()
	req.ExpectedHostFingerprint = ssh.FingerprintSHA256(f.key.PublicKey())
	info, err := m.Connect(req)
	if err != nil {
		t.Fatal(err)
	}
	awaitSession(t, m, info.ID, func(s SessionInfo) bool { return s.State == "connected" })
	var output []byte
	var cursor uint64
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		batch, err := m.Events(context.Background(), cursor, 50*time.Millisecond)
		if err != nil {
			t.Fatal(err)
		}
		cursor = batch.Next
		for _, event := range batch.Events {
			if event.Type == "output" {
				output = append(output, event.Data...)
			}
		}
		if bytes.Contains(output, []byte("fixture ready")) {
			break
		}
	}
	want := []byte("BANNER first\r\nBANNER second\r\n\x1b[32mBANNER end\x1b[0m\r\n")
	if !bytes.HasPrefix(output, want) || !bytes.Contains(output[len(want):], []byte("\x1b[32mfixture ready\x1b[0m\r\n")) {
		t.Fatalf("authentication banner/shell transition: %q", output)
	}
}

func TestSSHBannerStateIsConnectionLocalAndPTYOutputIsUnmodified(t *testing.T) {
	first := sshBannerNormalizer{}
	if got := string(first.normalize("first\r")); got != "first\r" {
		t.Fatalf("lone CR changed: %q", got)
	}
	second := sshBannerNormalizer{}
	if got := string(second.normalize("\nsecond\n")); got != "\r\nsecond\r\n" {
		t.Fatalf("a new connection inherited banner state: %q", got)
	}
	var got []byte
	output := terminalOutput{func(data []byte) { got = append(got, data...) }}
	// Post-banner shell bytes remain byte-identical, even a leading LF, lone
	// CR, ANSI redraw, controls, invalid UTF-8 and all possible byte values.
	want := []byte("\nPTY\r\n\x1b[2K\rPROMPT# \x00\xff")
	for i := 0; i < 256; i++ {
		want = append(want, byte(i))
	}
	for _, chunk := range [][]byte{want[:1], want[1:7], want[7:]} {
		n, err := output.Write(chunk)
		if n != len(chunk) || err != nil {
			t.Fatalf("normal PTY write result: %d, %v", n, err)
		}
	}
	if !bytes.Equal(got, want) {
		t.Fatal("authentication banner compatibility changed normal PTY bytes")
	}
	if got := string(first.normalize("\n")); got != "\n" {
		t.Fatalf("separate PTY output corrupted banner state: %q", got)
	}
}
