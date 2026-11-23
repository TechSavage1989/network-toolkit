package terminal

import (
	"context"
	"errors"
	"slices"
	"strings"
	"sync/atomic"
	"testing"
	"time"
	"unicode"
	"unicode/utf8"

	"golang.org/x/crypto/ssh"
)

func openSSHNoticeFixture(t *testing.T, req ConnectRequest, callbacks transportCallbacks) (*sshTransport, error) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	t.Cleanup(cancel)
	if callbacks.Prompt == nil {
		callbacks.Prompt = func(context.Context, Prompt) (PromptReply, error) {
			return PromptReply{}, errors.New("unexpected interactive SSH notice fixture prompt")
		}
	}
	connection, err := openSSHConnection(ctx, req, callbacks, nil, sshTiming{30 * time.Second, 10 * time.Second}, req.ExpectedHostFingerprint)
	if connection != nil {
		t.Cleanup(func() { _ = connection.Close() })
	}
	return connection, err
}

func TestSSHNoticeRequiresExplicitOptIn(t *testing.T) {
	for _, test := range []struct{ label, name, instruction string }{
		{label: "blank"},
		{label: "name only", name: "Fixture sign-in notice"},
		{label: "instruction only", instruction: "This connection is a synthetic test."},
		{label: "name and instruction", name: "Fixture sign-in notice", instruction: "This connection is a synthetic test."},
	} {
		for _, automatic := range []bool{false, true} {
			mode := "manual by default"
			if automatic {
				mode = "automatic when enabled"
			}
			t.Run(test.label+"/"+mode, func(t *testing.T) {
				var answered atomic.Int32
				f := newSSHFixture(t, func(config *ssh.ServerConfig) {
					config.PasswordCallback = nil
					config.KeyboardInteractiveCallback = sshPasswordFixtureChallenges([]sshPasswordChallengeRound{{name: test.name, instruction: test.instruction}}, &answered)
				}, true)
				req := f.request()
				req.ExpectedHostFingerprint = ssh.FingerprintSHA256(f.key.PublicKey())
				req.SSHAutoContinueNotices = automatic
				var output strings.Builder
				prompts := 0
				connection, err := openSSHNoticeFixture(t, req, transportCallbacks{
					Output: func(data []byte) { output.Write(data) },
					Prompt: func(_ context.Context, prompt Prompt) (PromptReply, error) {
						prompts++
						if automatic {
							return PromptReply{}, errors.New("enabled zero-question notice remained interactive")
						}
						if prompt.Kind != "keyboard-interactive" || prompt.Title != "SSH sign-in notice" || len(prompt.Fields) != 0 || prompt.ServerName != test.name || prompt.Message != test.instruction {
							return PromptReply{}, errors.New("manual notice lost its kind, title or server text")
						}
						return PromptReply{Action: "submit"}, nil
					},
				})
				if err != nil || connection == nil {
					t.Fatalf("fixture notice connection failed: %v", err)
				}
				wantPrompts := 1
				if automatic {
					wantPrompts = 0
					for _, text := range []string{test.name, test.instruction} {
						if text != "" && !strings.Contains(output.String(), text) {
							t.Fatal("automatic acknowledgment discarded server notice text")
						}
					}
				}
				if prompts != wantPrompts || answered.Load() != 1 {
					t.Fatalf("prompts=%d answers=%d, want prompts=%d answers=1", prompts, answered.Load(), wantPrompts)
				}
				if strings.Contains(output.String(), req.Password) {
					t.Fatal("automatic acknowledgment emitted the saved password")
				}
			})
		}
	}
}

func TestSSHNoticeDoesNotAnswerAuthenticationFields(t *testing.T) {
	manualPassword := sshPasswordChallengeRound{questions: []string{"Password:"}, echo: []bool{false}, want: []string{"fixture-manual-password"}, manual: true}
	for _, test := range []struct {
		name   string
		rounds []sshPasswordChallengeRound
	}{
		{name: "notice before password", rounds: []sshPasswordChallengeRound{{}, manualPassword}},
		{name: "notice before MFA", rounds: []sshPasswordChallengeRound{{}, {questions: []string{"One-time code:"}, echo: []bool{false}, want: []string{"654321"}, manual: true}}},
		{name: "empty field is still a question", rounds: []sshPasswordChallengeRound{{}, {questions: []string{""}, echo: []bool{false}, want: []string{"fixture-response"}, manual: true}}},
		{name: "multiple fields", rounds: []sshPasswordChallengeRound{{}, {questions: []string{"Password:", "One-time code:"}, echo: []bool{false, false}, want: []string{"fixture-manual-password", "654321"}, manual: true}}},
		{name: "echoed password", rounds: []sshPasswordChallengeRound{{}, {questions: []string{"Password:"}, echo: []bool{true}, want: []string{"fixture-manual-password"}, manual: true}}},
		{name: "password change", rounds: []sshPasswordChallengeRound{{}, {questions: []string{"New password:"}, echo: []bool{false}, want: []string{"fixture-new-password"}, manual: true}}},
		{name: "notice does not renew password allowance", rounds: []sshPasswordChallengeRound{{questions: []string{"Password:"}, echo: []bool{false}, want: []string{"fixture-secret"}}, {}, manualPassword}},
	} {
		t.Run(test.name, func(t *testing.T) {
			var answered atomic.Int32
			f := newSSHFixture(t, func(config *ssh.ServerConfig) {
				config.PasswordCallback = nil
				config.KeyboardInteractiveCallback = sshPasswordFixtureChallenges(test.rounds, &answered)
			}, true)
			req := f.request()
			req.ExpectedHostFingerprint = ssh.FingerprintSHA256(f.key.PublicKey())
			req.SSHAutoContinueNotices = true
			var manual []sshPasswordChallengeRound
			for _, round := range test.rounds {
				if round.manual {
					manual = append(manual, round)
				}
			}
			prompts := 0
			_, err := openSSHNoticeFixture(t, req, transportCallbacks{Prompt: func(_ context.Context, prompt Prompt) (PromptReply, error) {
				if prompts >= len(manual) || len(prompt.Fields) == 0 {
					return PromptReply{}, errors.New("unexpected interactive notice or extra authentication prompt")
				}
				want := manual[prompts]
				prompts++
				if prompt.Kind != "keyboard-interactive" || prompt.Title != "SSH authentication challenge" || len(prompt.Fields) != len(want.questions) {
					return PromptReply{}, errors.New("authentication challenge was changed into a notice")
				}
				for i, field := range prompt.Fields {
					if field.Label != want.questions[i] || field.Echo != want.echo[i] {
						return PromptReply{}, errors.New("manual authentication fields changed")
					}
				}
				return PromptReply{Action: "submit", Answers: slices.Clone(want.want)}, nil
			}})
			if err != nil || prompts != len(manual) || int(answered.Load()) != len(test.rounds) {
				t.Fatalf("manual fields not preserved: prompts=%d answered=%d error=%v", prompts, answered.Load(), err)
			}
		})
	}
}

func TestSSHNoticeHostKeyMismatchPrecedesAcknowledgment(t *testing.T) {
	var challenges atomic.Int32
	f := newSSHFixture(t, func(config *ssh.ServerConfig) {
		config.PasswordCallback = nil
		config.KeyboardInteractiveCallback = func(ssh.ConnMetadata, ssh.KeyboardInteractiveChallenge) (*ssh.Permissions, error) {
			challenges.Add(1)
			return nil, errors.New("authentication must not precede host verification")
		}
	}, true)
	other, _ := generateSSHKey(t)
	req := f.request()
	req.SSHAutoContinueNotices = true
	req.ExpectedHostFingerprint = ssh.FingerprintSHA256(other.PublicKey())
	connection, err := openSSHNoticeFixture(t, req, transportCallbacks{})
	if connection != nil || err == nil || !strings.Contains(err.Error(), "host key mismatch") || challenges.Load() != 0 {
		t.Fatalf("notice setting bypassed host identity: challenges=%d error=%v", challenges.Load(), err)
	}
}

func TestSSHNoticeLimitsPrecedeAcknowledgment(t *testing.T) {
	for _, test := range []struct {
		name        string
		rounds      []sshPasswordChallengeRound
		wantAnswers int32
	}{
		{name: "seventeen notice rounds", rounds: make([]sshPasswordChallengeRound, 17), wantAnswers: 16},
		{name: "oversized instruction", rounds: []sshPasswordChallengeRound{{instruction: strings.Repeat("x", 16*1024+1)}}},
		{name: "too many questions", rounds: []sshPasswordChallengeRound{{questions: make([]string, 17), echo: make([]bool, 17)}}},
		{name: "oversized field", rounds: []sshPasswordChallengeRound{{questions: []string{strings.Repeat("x", 4097)}, echo: []bool{false}}}},
	} {
		t.Run(test.name, func(t *testing.T) {
			var answered atomic.Int32
			f := newSSHFixture(t, func(config *ssh.ServerConfig) {
				config.PasswordCallback = nil
				config.KeyboardInteractiveCallback = sshPasswordFixtureChallenges(test.rounds, &answered)
			}, true)
			req := f.request()
			req.SSHAutoContinueNotices = true
			req.ExpectedHostFingerprint = ssh.FingerprintSHA256(f.key.PublicKey())
			connection, err := openSSHNoticeFixture(t, req, transportCallbacks{})
			if connection != nil || err == nil || !strings.Contains(err.Error(), "exceeded") || answered.Load() != test.wantAnswers {
				t.Fatalf("notice setting bypassed a bound: answered=%d want=%d error=%v", answered.Load(), test.wantAnswers, err)
			}
		})
	}
}

func TestSSHNoticeCancellation(t *testing.T) {
	t.Run("cancel manual notice", func(t *testing.T) {
		var answered atomic.Int32
		f := newSSHFixture(t, func(config *ssh.ServerConfig) {
			config.PasswordCallback = nil
			config.KeyboardInteractiveCallback = sshPasswordFixtureChallenges([]sshPasswordChallengeRound{{}}, &answered)
		}, true)
		req := f.request()
		req.ExpectedHostFingerprint = ssh.FingerprintSHA256(f.key.PublicKey())
		connection, err := openSSHNoticeFixture(t, req, transportCallbacks{Prompt: func(context.Context, Prompt) (PromptReply, error) {
			return PromptReply{Action: "cancel"}, nil
		}})
		if connection != nil || err == nil || answered.Load() != 0 {
			t.Fatalf("cancelled manual notice was acknowledged: answers=%d error=%v", answered.Load(), err)
		}
	})
	t.Run("cancel while retaining automatic notice text", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		var answered atomic.Int32
		f := newSSHFixture(t, func(config *ssh.ServerConfig) {
			config.PasswordCallback = nil
			config.KeyboardInteractiveCallback = sshPasswordFixtureChallenges([]sshPasswordChallengeRound{{instruction: "Fixture notice before cancellation."}}, &answered)
		}, true)
		req := f.request()
		req.SSHAutoContinueNotices = true
		req.ExpectedHostFingerprint = ssh.FingerprintSHA256(f.key.PublicKey())
		outputs := 0
		connection, err := openSSHConnection(ctx, req, transportCallbacks{
			Output: func([]byte) { outputs++; cancel() },
			Prompt: func(context.Context, Prompt) (PromptReply, error) {
				return PromptReply{}, errors.New("automatic notice unexpectedly requested input")
			},
		}, nil, sshTiming{30 * time.Second, 10 * time.Second}, req.ExpectedHostFingerprint)
		if connection != nil {
			_ = connection.Close()
		}
		if connection != nil || err == nil || outputs != 1 || answered.Load() != 0 {
			t.Fatalf("cancelled auto notice was acknowledged: outputs=%d answers=%d error=%v", outputs, answered.Load(), err)
		}
	})
}

func TestSSHNoticeOutputSanitizesServerTextAndSecrets(t *testing.T) {
	const password = "fixture-secret"
	const privateKey = "fixture-private-key-canary"
	const passphrase = "fixture-passphrase-canary"
	var answered atomic.Int32
	f := newSSHFixture(t, func(config *ssh.ServerConfig) {
		config.PasswordCallback = nil
		config.KeyboardInteractiveCallback = sshPasswordFixtureChallenges([]sshPasswordChallengeRound{{
			name:        "Fixture heading \x1b]0;changed\x07\u202e\u2066\x00\x7f\u0085 " + password,
			instruction: "Readable first line\r\nReadable second line\rReadable third line\nFinal\tline\x1b[2J\u009b31m\xff " + password + " " + privateKey + " " + passphrase,
		}}, &answered)
	}, true)
	req := f.request()
	req.SSHAutoContinueNotices = true
	req.Password, req.PrivateKey, req.Passphrase = password, privateKey, passphrase
	req.ExpectedHostFingerprint = ssh.FingerprintSHA256(f.key.PublicKey())
	var output strings.Builder
	_, err := openSSHNoticeFixture(t, req, transportCallbacks{Output: func(data []byte) { output.Write(data) }})
	if err != nil || answered.Load() != 1 {
		t.Fatalf("notice fixture failed: answers=%d error=%v", answered.Load(), err)
	}
	text := output.String()
	for _, readable := range []string{"Fixture heading", "Readable first line", "Readable second line", "Readable third line", "Final"} {
		if !strings.Contains(text, readable) {
			t.Fatal("safe server notice text was discarded")
		}
	}
	for _, secret := range []string{password, privateKey, passphrase} {
		if strings.Contains(text, secret) {
			t.Fatal("notice output exposed a held authentication secret")
		}
	}
	if !utf8.ValidString(text) {
		t.Fatal("notice output contains invalid UTF-8")
	}
	for i, r := range text {
		if r == '\r' {
			if i+1 >= len(text) || text[i+1] != '\n' {
				t.Fatal("server carriage return remained a terminal control")
			}
			continue
		}
		if r == '\n' {
			if i == 0 || text[i-1] != '\r' {
				t.Fatal("notice line ending was not generated as CRLF")
			}
			continue
		}
		if unicode.IsControl(r) || unicode.Is(unicode.Cf, r) {
			t.Fatal("notice output retained a server terminal or formatting control")
		}
	}
}

func TestSSHNoticeServerHeadingIsBoundedWithoutChangingPasswordRecognition(t *testing.T) {
	var answered atomic.Int32
	f := newSSHFixture(t, func(config *ssh.ServerConfig) {
		config.PasswordCallback = nil
		config.KeyboardInteractiveCallback = sshPasswordFixtureChallenges([]sshPasswordChallengeRound{{
			name: "\x1b\u202e" + strings.Repeat("界", 400) + "Password", questions: []string{"Password:"}, echo: []bool{false}, want: []string{"fixture-manual-password"},
		}}, &answered)
	}, true)
	req := f.request()
	req.SSHAutoContinueNotices = true
	req.ExpectedHostFingerprint = ssh.FingerprintSHA256(f.key.PublicKey())
	prompts := 0
	_, err := openSSHNoticeFixture(t, req, transportCallbacks{Prompt: func(_ context.Context, prompt Prompt) (PromptReply, error) {
		prompts++
		if len(prompt.ServerName) > 1024 || !utf8.ValidString(prompt.ServerName) || !strings.Contains(prompt.ServerName, "[truncated]") || prompt.Title != "SSH authentication challenge" {
			return PromptReply{}, errors.New("server heading was not safely bounded with a fixed application title")
		}
		for _, r := range prompt.ServerName {
			if unicode.IsControl(r) || unicode.Is(unicode.Cf, r) {
				return PromptReply{}, errors.New("server heading retained control or formatting characters")
			}
		}
		return PromptReply{Action: "submit", Answers: []string{"fixture-manual-password"}}, nil
	}})
	if err != nil || prompts != 1 || answered.Load() != 1 {
		t.Fatalf("oversized unrecognized server context did not stay manual: prompts=%d answers=%d error=%v", prompts, answered.Load(), err)
	}
}

func TestSSHNoticeFileConnectionNeedsNoOutputCallback(t *testing.T) {
	for _, test := range []struct {
		name        string
		automatic   bool
		question    bool
		wantAnswers int32
		wantSuccess bool
	}{
		{name: "default notice requires interaction"},
		{name: "enabled notice continues without output", automatic: true, wantAnswers: 1, wantSuccess: true},
		{name: "follow-up field still needs interaction", automatic: true, question: true, wantAnswers: 1},
	} {
		t.Run(test.name, func(t *testing.T) {
			rounds := []sshPasswordChallengeRound{{name: "SFTP fixture notice", instruction: "This is a synthetic connection."}}
			if test.question {
				rounds = append(rounds, sshPasswordChallengeRound{questions: []string{"Password:"}, echo: []bool{false}, want: []string{"fixture-manual-password"}})
			}
			var answered atomic.Int32
			f := newSSHFixture(t, func(config *ssh.ServerConfig) {
				config.PasswordCallback = nil
				config.KeyboardInteractiveCallback = sshPasswordFixtureChallenges(rounds, &answered)
			}, true)
			req := f.request()
			req.SSHAutoContinueNotices = test.automatic
			req.ExpectedHostFingerprint = ssh.FingerprintSHA256(f.key.PublicKey())
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			file, err := OpenFileConnection(ctx, req, req.ExpectedHostFingerprint)
			if file != nil {
				defer file.Close()
				assertNoRetainedAuthentication(t, file.request)
				if !file.request.SSHAutoContinueNotices {
					t.Fatal("file connection lost its enabled notice preference")
				}
			}
			if answered.Load() != test.wantAnswers {
				t.Fatalf("answers=%d, want %d", answered.Load(), test.wantAnswers)
			}
			if test.wantSuccess {
				if err != nil || file == nil {
					t.Fatalf("automatic notice required an absent output callback: %v", err)
				}
			} else if file != nil || err == nil || !strings.Contains(err.Error(), "interactive challenge") {
				t.Fatalf("file connection discarded required user interaction: %v", err)
			}
		})
	}
}
