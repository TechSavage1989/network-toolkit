package terminal

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"slices"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"golang.org/x/crypto/ssh"
)

func TestSSHPasswordChallengeRecognition(t *testing.T) {
	for _, test := range []struct {
		name, title, instruction, question string
		echo, want                         bool
	}{
		{name: "plain", question: "Password:", want: true},
		{name: "case and whitespace", question: " \tPaSsWoRd: \r\n", want: true},
		{name: "no colon", question: "Password", want: true},
		{name: "PAM", title: "PAM", question: "Password:", want: true},
		{name: "PAM authentication", title: "PAM authentication", instruction: "Password:", question: "Password:", want: true},
		{name: "SSH authentication", title: "SSH authentication", instruction: "Please enter your password.", question: "Password:", want: true},
		{name: "password authentication", title: "Password authentication", instruction: "Enter your password:", question: "Password:", want: true},
		{name: "echoed password", question: "Password:", echo: true},
		{name: "OTP", question: "One-time code:"},
		{name: "arbitrary hidden field", question: "Response:"},
		{name: "password and OTP", question: "Password and verification code:"},
		{name: "new password", question: "New password:"},
		{name: "current password", question: "Current password:"},
		{name: "repeat password", question: "Retype password:"},
		{name: "expired instruction", instruction: "Your password has expired. Enter a new password.", question: "Password:"},
		{name: "OTP instruction", instruction: "Enter your one-time code", question: "Password:"},
		{name: "MFA title", title: "Multi-factor authentication", question: "Password:"},
		{name: "unknown title", title: "Custom authentication", question: "Password:"},
		{name: "unknown instruction", instruction: "Answer the following question", question: "Password:"},
		{name: "oversized title", title: strings.Repeat(" ", 1024) + "Password", question: "Password:"},
		{name: "control in question", question: "Password:\x00"},
		{name: "additional question text", question: "Password: or security code:"},
	} {
		t.Run(test.name, func(t *testing.T) {
			got := plainSSHPasswordChallenge(test.title, test.instruction, []string{test.question}, []bool{test.echo})
			if got != test.want {
				t.Fatalf("automatic password eligibility = %v, want %v", got, test.want)
			}
		})
	}
	for _, test := range []struct {
		name      string
		questions []string
		echo      []bool
	}{
		{name: "zero questions"},
		{name: "multiple questions", questions: []string{"Password:", "Code:"}, echo: []bool{false, false}},
		{name: "missing echo", questions: []string{"Password:"}},
		{name: "extra echo", questions: []string{"Password:"}, echo: []bool{false, false}},
	} {
		t.Run(test.name, func(t *testing.T) {
			if plainSSHPasswordChallenge("", "", test.questions, test.echo) {
				t.Fatal("non-password-only challenge accepted")
			}
		})
	}
}

func TestSSHPasswordAuthenticationCleanup(t *testing.T) {
	for _, method := range []string{"key", "keyboard"} {
		a := newSSHPasswordAuthentication(method, "fixture-unused-password")
		if a.secret != "" {
			t.Fatal("non-password authentication retained an unused password")
		}
		if _, ok := a.answerChallenge("", "", []string{"Password:"}, []bool{false}); ok {
			t.Fatal("non-password request supplied a login password")
		}
	}
	for _, keyboard := range []bool{false, true} {
		a := newSSHPasswordAuthentication("password", "fixture-secret")
		retainedCallback := a.takePassword
		if keyboard {
			answers, ok := a.answerChallenge("", "", []string{"Password:"}, []bool{false})
			if !ok || !slices.Equal(answers, []string{"fixture-secret"}) {
				t.Fatal("first password challenge was not answered")
			}
		} else if password, err := retainedCallback(); err != nil || password != "fixture-secret" {
			t.Fatal("first ordinary password was not available")
		}
		if a.secret != "" {
			t.Fatal("consumed automatic password remained in handshake state")
		}
		if _, err := retainedCallback(); err == nil {
			t.Fatal("a second authentication method reused the password")
		}
		a.clear()
		if a.secret != "" || a.allowed {
			t.Fatal("authentication cleanup retained a usable password")
		}
		if _, err := retainedCallback(); err == nil {
			t.Fatal("retained callback was usable after cleanup")
		}
	}
	a := newSSHPasswordAuthentication("password", "fixture-unconsumed-password")
	a.clear()
	if a.secret != "" {
		t.Fatal("cleanup retained an unconsumed password")
	}
}

type sshPasswordChallengeRound struct {
	name, instruction string
	questions         []string
	echo              []bool
	want              []string
	manual            bool
}

func sshPasswordFixtureChallenges(rounds []sshPasswordChallengeRound, answered *atomic.Int32) func(ssh.ConnMetadata, ssh.KeyboardInteractiveChallenge) (*ssh.Permissions, error) {
	return func(metadata ssh.ConnMetadata, challenge ssh.KeyboardInteractiveChallenge) (*ssh.Permissions, error) {
		if metadata.User() != "fixture-user" {
			return nil, errors.New("unexpected fixture username")
		}
		for _, round := range rounds {
			answers, err := challenge(round.name, round.instruction, round.questions, round.echo)
			if err != nil {
				return nil, err
			}
			if !slices.Equal(answers, round.want) {
				return nil, errors.New("unexpected fixture authentication answer")
			}
			answered.Add(1)
		}
		return nil, nil
	}
}

func openSSHPasswordFixture(t *testing.T, req ConnectRequest, prompt func(context.Context, Prompt) (PromptReply, error)) (transport, error) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	t.Cleanup(cancel)
	if prompt == nil {
		prompt = func(context.Context, Prompt) (PromptReply, error) {
			return PromptReply{}, errors.New("unexpected interactive authentication prompt")
		}
	}
	tr, err := openSSH(ctx, req, transportCallbacks{Prompt: prompt}, nil)
	if tr != nil {
		t.Cleanup(func() { _ = tr.Close() })
	}
	return tr, err
}

func TestSSHPasswordKeyboardInteractive(t *testing.T) {
	automatic := sshPasswordChallengeRound{questions: []string{"Password:"}, echo: []bool{false}, want: []string{"fixture-secret"}}
	manualPassword := sshPasswordChallengeRound{questions: []string{"Password:"}, echo: []bool{false}, want: []string{"fixture-manual-password"}, manual: true}
	code := sshPasswordChallengeRound{questions: []string{"One-time code:"}, echo: []bool{false}, want: []string{"654321"}, manual: true}
	for _, test := range []struct {
		name            string
		method          string
		withoutPassword bool
		rounds          []sshPasswordChallengeRound
	}{
		{name: "keyboard-only password", rounds: []sshPasswordChallengeRound{automatic}},
		{name: "generic PAM password", rounds: []sshPasswordChallengeRound{{name: "PAM authentication", instruction: "Password:", questions: []string{"PASSWORD"}, echo: []bool{false}, want: []string{"fixture-secret"}}}},
		{name: "follow-up MFA", rounds: []sshPasswordChallengeRound{automatic, code}},
		{name: "repeated password", rounds: []sshPasswordChallengeRound{automatic, manualPassword}},
		{name: "notice before password", rounds: []sshPasswordChallengeRound{{manual: true}, manualPassword}},
		{name: "MFA before password", rounds: []sshPasswordChallengeRound{code, manualPassword}},
		{name: "explicit keyboard method", method: "keyboard", rounds: []sshPasswordChallengeRound{manualPassword}},
		{name: "no available password", withoutPassword: true, rounds: []sshPasswordChallengeRound{manualPassword}},
		{name: "multiple questions", rounds: []sshPasswordChallengeRound{{questions: []string{"Password:", "One-time code:"}, echo: []bool{false, false}, want: []string{"fixture-manual-password", "654321"}, manual: true}}},
		{name: "echoed question", rounds: []sshPasswordChallengeRound{{questions: []string{"Password:"}, echo: []bool{true}, want: []string{"fixture-manual-password"}, manual: true}}},
		{name: "new password", rounds: []sshPasswordChallengeRound{{questions: []string{"New password:"}, echo: []bool{false}, want: []string{"fixture-new-password"}, manual: true}}},
		{name: "expired password instruction", rounds: []sshPasswordChallengeRound{{instruction: "Your password has expired. Enter a new password.", questions: []string{"Password:"}, echo: []bool{false}, want: []string{"fixture-new-password"}, manual: true}}},
		{name: "MFA title", rounds: []sshPasswordChallengeRound{{name: "Multi-factor authentication", questions: []string{"Password:"}, echo: []bool{false}, want: []string{"654321"}, manual: true}}},
		{name: "unknown instruction", rounds: []sshPasswordChallengeRound{{instruction: "Answer the following question", questions: []string{"Password:"}, echo: []bool{false}, want: []string{"fixture-manual-password"}, manual: true}}},
		{name: "oversized title stays manual", rounds: []sshPasswordChallengeRound{{name: strings.Repeat(" ", 1024) + "Password", questions: []string{"Password:"}, echo: []bool{false}, want: []string{"fixture-manual-password"}, manual: true}}},
	} {
		t.Run(test.name, func(t *testing.T) {
			var answered atomic.Int32
			f := newSSHFixture(t, func(config *ssh.ServerConfig) {
				config.PasswordCallback = nil
				config.KeyboardInteractiveCallback = sshPasswordFixtureChallenges(test.rounds, &answered)
			}, true)
			req := f.request()
			req.ExpectedHostFingerprint = ssh.FingerprintSHA256(f.key.PublicKey())
			if test.method != "" {
				req.AuthMethod = test.method
			}
			if test.withoutPassword {
				req.Password = ""
			}
			var manual []sshPasswordChallengeRound
			for _, round := range test.rounds {
				if round.manual {
					manual = append(manual, round)
				}
			}
			prompts := 0
			_, err := openSSHPasswordFixture(t, req, func(_ context.Context, prompt Prompt) (PromptReply, error) {
				if prompts >= len(manual) {
					return PromptReply{}, errors.New("unexpected extra password prompt")
				}
				want := manual[prompts]
				prompts++
				if prompt.Kind != "keyboard-interactive" || len(prompt.Fields) != len(want.questions) {
					return PromptReply{}, errors.New("wrong manual authentication challenge")
				}
				for i, field := range prompt.Fields {
					if field.Label != want.questions[i] || field.Echo != want.echo[i] {
						return PromptReply{}, errors.New("manual challenge fields changed")
					}
				}
				return PromptReply{Action: "submit", Answers: slices.Clone(want.want)}, nil
			})
			if err != nil {
				t.Fatalf("fixture SSH connection failed: %v", err)
			}
			if prompts != len(manual) || int(answered.Load()) != len(test.rounds) {
				t.Fatalf("prompts=%d answered=%d, want prompts=%d answered=%d", prompts, answered.Load(), len(manual), len(test.rounds))
			}
		})
	}
}

func TestSSHPasswordOrdinaryMethodAndFallback(t *testing.T) {
	for _, outcome := range []string{"success", "rejected", "partial success"} {
		t.Run(outcome, func(t *testing.T) {
			var passwords, challenges atomic.Int32
			keyboard := sshPasswordFixtureChallenges([]sshPasswordChallengeRound{{questions: []string{"Password:"}, echo: []bool{false}, want: []string{"fixture-manual-password"}}}, &challenges)
			f := newSSHFixture(t, func(config *ssh.ServerConfig) {
				config.KeyboardInteractiveCallback = keyboard
				config.PasswordCallback = func(_ ssh.ConnMetadata, password []byte) (*ssh.Permissions, error) {
					passwords.Add(1)
					if string(password) != "fixture-secret" {
						return nil, errors.New("unexpected ordinary fixture password")
					}
					switch outcome {
					case "success":
						return nil, nil
					case "partial success":
						return nil, &ssh.PartialSuccessError{Next: ssh.ServerAuthCallbacks{KeyboardInteractiveCallback: keyboard}}
					default:
						return nil, errors.New("fixture password rejected")
					}
				}
			}, true)
			req := f.request()
			req.ExpectedHostFingerprint = ssh.FingerprintSHA256(f.key.PublicKey())
			prompts := 0
			_, err := openSSHPasswordFixture(t, req, func(_ context.Context, prompt Prompt) (PromptReply, error) {
				prompts++
				if prompt.Kind != "keyboard-interactive" || len(prompt.Fields) != 1 || prompt.Fields[0].Label != "Password:" {
					return PromptReply{}, errors.New("wrong fallback challenge")
				}
				return PromptReply{Action: "submit", Answers: []string{"fixture-manual-password"}}, nil
			})
			if err != nil {
				t.Fatalf("fixture SSH connection failed: %v", err)
			}
			wantPrompts := 1
			if outcome == "success" {
				wantPrompts = 0
			}
			if passwords.Load() != 1 || prompts != wantPrompts || int(challenges.Load()) != wantPrompts {
				t.Fatalf("passwords=%d prompts=%d challenges=%d", passwords.Load(), prompts, challenges.Load())
			}
		})
	}
}

func TestSSHPasswordCannotReplayAfterKeyboardPartialSuccess(t *testing.T) {
	var passwords, challenges atomic.Int32
	f := newSSHFixture(t, func(config *ssh.ServerConfig) {
		config.PasswordCallback = nil
		config.KeyboardInteractiveCallback = func(metadata ssh.ConnMetadata, challenge ssh.KeyboardInteractiveChallenge) (*ssh.Permissions, error) {
			_, err := sshPasswordFixtureChallenges([]sshPasswordChallengeRound{{questions: []string{"Password:"}, echo: []bool{false}, want: []string{"fixture-secret"}}}, &challenges)(metadata, challenge)
			if err != nil {
				return nil, err
			}
			return nil, &ssh.PartialSuccessError{Next: ssh.ServerAuthCallbacks{PasswordCallback: func(ssh.ConnMetadata, []byte) (*ssh.Permissions, error) {
				passwords.Add(1)
				return nil, nil
			}}}
		}
	}, true)
	req := f.request()
	req.ExpectedHostFingerprint = ssh.FingerprintSHA256(f.key.PublicKey())
	tr, err := openSSHPasswordFixture(t, req, nil)
	if tr != nil || err == nil || !strings.Contains(err.Error(), "already submitted") {
		t.Fatalf("second automatic password attempt was not refused: %v", err)
	}
	if challenges.Load() != 1 || passwords.Load() != 0 {
		t.Fatalf("automatic password repeated: challenges=%d ordinary=%d", challenges.Load(), passwords.Load())
	}
}

func TestSSHPasswordHostKeyMismatchPrecedesAutomaticAnswer(t *testing.T) {
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
	req.CredentialID = "fixture-saved-password"
	req.ExpectedHostFingerprint = ssh.FingerprintSHA256(other.PublicKey())
	tr, err := openSSHPasswordFixture(t, req, nil)
	if tr != nil || err == nil || !strings.Contains(err.Error(), "host key mismatch") {
		t.Fatalf("saved host fingerprint was not enforced: %v", err)
	}
	if challenges.Load() != 0 {
		t.Fatal("authentication reached a server with a mismatched host key")
	}
}

func TestSSHPasswordChallengeLimitsPrecedeAutomaticAnswer(t *testing.T) {
	var answers atomic.Int32
	f := newSSHFixture(t, func(config *ssh.ServerConfig) {
		config.PasswordCallback = nil
		config.KeyboardInteractiveCallback = func(_ ssh.ConnMetadata, challenge ssh.KeyboardInteractiveChallenge) (*ssh.Permissions, error) {
			_, err := challenge("", "", []string{strings.Repeat(" ", 4096) + "Password:"}, []bool{false})
			if err == nil {
				answers.Add(1)
			}
			return nil, err
		}
	}, true)
	req := f.request()
	req.ExpectedHostFingerprint = ssh.FingerprintSHA256(f.key.PublicKey())
	tr, err := openSSHPasswordFixture(t, req, nil)
	if tr != nil || err == nil || !strings.Contains(err.Error(), "question exceeded its limit") {
		t.Fatalf("automatic answer bypassed the prompt bound: %v", err)
	}
	if answers.Load() != 0 {
		t.Fatal("an oversized password challenge received an answer")
	}
}

func TestSSHPasswordFileConnectionUsesAutomaticAnswer(t *testing.T) {
	var answers atomic.Int32
	f := newSSHFixture(t, func(config *ssh.ServerConfig) {
		config.PasswordCallback = nil
		config.KeyboardInteractiveCallback = sshPasswordFixtureChallenges([]sshPasswordChallengeRound{{questions: []string{"Password:"}, echo: []bool{false}, want: []string{"fixture-secret"}}}, &answers)
	}, true)
	req := f.request()
	req.CredentialID = "fixture-saved-password"
	req.ExpectedHostFingerprint = ssh.FingerprintSHA256(f.key.PublicKey())
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	file, err := OpenFileConnection(ctx, req, req.ExpectedHostFingerprint)
	if err != nil {
		t.Fatalf("shared file authentication failed: %v", err)
	}
	defer file.Close()
	if answers.Load() != 1 {
		t.Fatalf("shared file password answer count = %d, want 1", answers.Load())
	}
	assertNoRetainedAuthentication(t, file.request)
}

func TestSSHPasswordAutomaticAnswerSecrecyAndMFACancellation(t *testing.T) {
	for _, mfa := range []bool{false, true} {
		name := "connected"
		if mfa {
			name = "pending MFA cancelled"
		}
		t.Run(name, func(t *testing.T) {
			var answers atomic.Int32
			rounds := []sshPasswordChallengeRound{{questions: []string{"Password:"}, echo: []bool{false}, want: []string{"fixture-secret"}}}
			if mfa {
				rounds = append(rounds, sshPasswordChallengeRound{questions: []string{"One-time code:"}, echo: []bool{false}, want: []string{"654321"}})
			}
			f := newSSHFixture(t, func(config *ssh.ServerConfig) {
				config.PasswordCallback = nil
				config.KeyboardInteractiveCallback = sshPasswordFixtureChallenges(rounds, &answers)
			}, true)
			m := sshManager(t, t.TempDir())
			req := f.request()
			req.CredentialID = "fixture-saved-password"
			req.ExpectedHostFingerprint = ssh.FingerprintSHA256(f.key.PublicKey())
			info, err := m.Connect(req)
			if err != nil {
				t.Fatal(err)
			}
			current := awaitSession(t, m, info.ID, func(s SessionInfo) bool { return s.State == "connected" || s.State == "error" || s.Prompt != nil })
			if mfa {
				if current.Prompt == nil || current.Prompt.Kind != "keyboard-interactive" || len(current.Prompt.Fields) != 1 || current.Prompt.Fields[0].Label != "One-time code:" {
					t.Fatal("follow-up MFA was not kept interactive")
				}
				if err := m.Disconnect(info.ID); err != nil {
					t.Fatal(err)
				}
				awaitSession(t, m, info.ID, func(s SessionInfo) bool { return s.State == "closed" })
			} else if current.State != "connected" || current.Prompt != nil {
				t.Fatalf("password-only authentication did not connect without a prompt: %s", current.State)
			}
			if answers.Load() != 1 {
				t.Fatalf("automatic password answer count = %d, want 1", answers.Load())
			}
			m.mu.Lock()
			retained := m.sessions[info.ID].request
			m.mu.Unlock()
			assertNoRetainedAuthentication(t, retained)
			batch, err := m.Events(context.Background(), 0, 0)
			if err != nil {
				t.Fatal(err)
			}
			raw, err := json.Marshal(batch)
			if err != nil {
				t.Fatal(err)
			}
			if bytes.Contains(raw, []byte("fixture-secret")) || bytes.Contains(raw, []byte("654321")) {
				t.Fatal("password or MFA answer escaped through session events")
			}
			for _, event := range batch.Events {
				if bytes.Contains(event.Data, []byte("fixture-secret")) || bytes.Contains(event.Data, []byte("654321")) {
					t.Fatal("password or MFA answer escaped through terminal output")
				}
			}
		})
	}
}
