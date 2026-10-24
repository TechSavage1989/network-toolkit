package terminal

import (
	"bytes"
	"context"
	"crypto/subtle"
	"errors"
	"fmt"
	"io"
	"net"
	"strconv"
	"sync"
	"time"

	"golang.org/x/crypto/ssh"
)

type sshTiming struct{ keepaliveInterval, keepaliveTimeout time.Duration }

type sshTransport struct {
	base            *sshTransport // Non-nil for an interactive shell on a shared transport.
	ownerMu         sync.Mutex
	ownsBase        bool
	ownershipClosed bool
	onOwned         func()

	conn             net.Conn
	client           *ssh.Client
	session          *ssh.Session
	stdin            io.WriteCloser
	ctx              context.Context
	cancel           context.CancelFunc
	stopCancellation func() bool
	done             chan error
	closeOnce        sync.Once
	errMu            sync.Mutex
	exitErr          error
}

func openSSH(ctx context.Context, req ConnectRequest, cb transportCallbacks, known *knownHostStore) (transport, error) {
	return openSSHWithTiming(ctx, req, cb, known, sshTiming{30 * time.Second, 10 * time.Second})
}

func openSSHWithTiming(parent context.Context, req ConnectRequest, cb transportCallbacks, known *knownHostStore, timing sshTiming) (transport, error) {
	base, err := openSSHConnection(parent, req, cb, known, timing, "")
	if err != nil {
		return nil, err
	}
	// Ordinary Terminal owns the whole socket during setup, so retain its
	// network deadline as well as the channel timeout for blocked socket writes.
	if err := base.conn.SetDeadline(time.Now().Add(time.Duration(req.TimeoutMs) * time.Millisecond)); err != nil {
		_ = base.Close()
		return nil, err
	}
	tr, err := base.openShell(parent, req, cb)
	if err != nil {
		_ = base.Close()
		return nil, err
	}
	if err := base.conn.SetDeadline(time.Time{}); err != nil {
		_ = tr.Close()
		_ = base.Close()
		return nil, err
	}
	if err := tr.commitOwnership(); err != nil {
		_ = tr.Close()
		_ = base.Close()
		return nil, err
	}
	return tr, nil
}

// openShell adds only a new subsystem channel. A rejected PTY or shell never
// closes the authenticated connection used by existing SFTP operations.
func (base *sshTransport) openShell(parent context.Context, req ConnectRequest, cb transportCallbacks) (*sshTransport, error) {
	if err := parent.Err(); err != nil {
		return nil, err
	}
	if err := base.ctx.Err(); err != nil {
		return nil, ErrNotConnected
	}
	setupCtx, cancelSetup := context.WithTimeout(parent, time.Duration(req.TimeoutMs)*time.Millisecond)
	defer cancelSetup()
	type sessionResult struct {
		session *ssh.Session
		err     error
	}
	result := make(chan sessionResult)
	go func() {
		sess, err := base.client.NewSession()
		select {
		case result <- sessionResult{sess, err}:
		case <-setupCtx.Done():
			if sess != nil {
				_ = sess.Close()
			}
		case <-base.ctx.Done():
			if sess != nil {
				_ = sess.Close()
			}
		}
	}()
	var session *ssh.Session
	select {
	case opened := <-result:
		if opened.err != nil {
			return nil, fmt.Errorf("open SSH shell channel: %w", opened.err)
		}
		session = opened.session
	case <-setupCtx.Done():
		return nil, setupCtx.Err()
	case <-base.ctx.Done():
		return nil, ErrNotConnected
	}
	opened := false
	closeAttempt := func() { go func() { _ = session.Close() }() }
	stopSetup := context.AfterFunc(setupCtx, closeAttempt)
	defer func() {
		stopSetup()
		if !opened {
			closeAttempt()
		}
	}()
	stdin, err := session.StdinPipe()
	if err != nil {
		return nil, err
	}
	output := terminalOutput{cb.Output}
	session.Stdout, session.Stderr = output, output
	setupResult := make(chan error)
	go func() {
		err := session.RequestPty(req.Terminal, req.Rows, req.Cols, ssh.TerminalModes{ssh.ECHO: 1, ssh.TTY_OP_ISPEED: 38400, ssh.TTY_OP_OSPEED: 38400})
		if err != nil {
			err = fmt.Errorf("request SSH terminal: %w", err)
		} else if setupCtx.Err() == nil {
			if shellErr := session.Shell(); shellErr != nil {
				err = fmt.Errorf("start SSH shell: %w", shellErr)
			}
		}
		select {
		case setupResult <- err:
		case <-setupCtx.Done():
			closeAttempt()
		case <-base.ctx.Done():
			closeAttempt()
		}
	}()
	select {
	case err := <-setupResult:
		if err != nil {
			return nil, err
		}
	case <-setupCtx.Done():
		return nil, setupCtx.Err()
	case <-base.ctx.Done():
		return nil, ErrNotConnected
	}

	// Stop the setup timeout before its cancellation can close the live shell.
	if !stopSetup() || setupCtx.Err() != nil {
		return nil, setupCtx.Err()
	}
	lifetime, cancel := context.WithCancel(parent)
	t := &sshTransport{base: base, conn: base.conn, client: base.client, session: session, stdin: stdin, ctx: lifetime, cancel: cancel, done: make(chan error, 1)}
	t.stopCancellation = context.AfterFunc(lifetime, func() { _ = t.Close() })
	stopBase := context.AfterFunc(base.ctx, cancel)
	opened = true
	go func() {
		err := session.Wait()
		base.errMu.Lock()
		if base.exitErr != nil {
			err = base.exitErr
		} else if lifetime.Err() != nil {
			err = nil
		}
		base.errMu.Unlock()
		_ = t.Close()
		stopBase()
		t.done <- err
		close(t.done)
	}()
	return t, nil
}

func openSSHConnection(parent context.Context, req ConnectRequest, cb transportCallbacks, known *knownHostStore, timing sshTiming, pin string) (*sshTransport, error) {
	if err := parent.Err(); err != nil {
		return nil, err
	}
	if known == nil && pin == "" {
		return nil, errors.New("SSH host-key verification is unavailable")
	}
	if known != nil {
		if err := known.loadError(); err != nil {
			return nil, err
		}
	}
	if cb.Prompt == nil {
		return nil, errors.New("SSH authentication prompts are unavailable")
	}
	var signer ssh.Signer
	var err error
	if req.AuthMethod == "key" {
		signer, err = parseSSHSigner(parent, req, cb.Prompt)
		if err != nil {
			return nil, err
		}
	}
	if err := parent.Err(); err != nil {
		return nil, err
	}
	lifetime, cancel := context.WithCancel(parent)
	endpoint := net.JoinHostPort(req.Host, strconv.Itoa(req.Port))
	timeout := time.Duration(req.TimeoutMs) * time.Millisecond
	connection, err := (&net.Dialer{Timeout: timeout, KeepAlive: 30 * time.Second}).DialContext(lifetime, "tcp", endpoint)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("connect SSH: %w", err)
	}
	stopCancellation := context.AfterFunc(lifetime, func() { _ = connection.Close() })
	opened := false
	defer func() {
		if !opened {
			cancel()
			stopCancellation()
			_ = connection.Close()
		}
	}()
	if err := connection.SetDeadline(time.Now().Add(timeout)); err != nil {
		return nil, err
	}
	// A human trust/MFA prompt has its own deadline. Pausing and resetting the
	// network deadline here prevents a legitimate answer from inheriting an
	// already expired handshake deadline. Lifetime cancellation still closes TCP.
	ask := func(ctx context.Context, prompt Prompt) (PromptReply, error) {
		if err := connection.SetDeadline(time.Time{}); err != nil {
			return PromptReply{}, err
		}
		reply, err := cb.Prompt(ctx, prompt)
		deadlineErr := connection.SetDeadline(time.Now().Add(timeout))
		if err != nil {
			return PromptReply{}, err
		}
		if deadlineErr != nil {
			return PromptReply{}, deadlineErr
		}
		return reply, nil
	}
	var acceptedKey []byte
	var acceptedMu sync.Mutex
	hostKey := func(_ string, _ net.Addr, key ssh.PublicKey) error {
		acceptedMu.Lock()
		defer acceptedMu.Unlock()
		if acceptedKey != nil {
			if bytes.Equal(acceptedKey, key.Marshal()) {
				return nil
			}
			return errors.New("SSH server changed its host key within this connection")
		}
		if pin != "" {
			observed := ssh.FingerprintSHA256(key)
			if subtle.ConstantTimeCompare([]byte(pin), []byte(observed)) != 1 {
				return fmt.Errorf("SSH host key mismatch: expected %s, presented %s", pin, observed)
			}
		} else if err := known.verify(lifetime, endpoint, key, ask); err != nil {
			return err
		}
		acceptedKey = append([]byte(nil), key.Marshal()...)
		return nil
	}
	challengeCount := 0
	keyboard := ssh.KeyboardInteractive(func(_ string, instruction string, questions []string, echo []bool) ([]string, error) {
		challengeCount++
		if challengeCount > 16 || len(questions) > 16 || len(echo) != len(questions) {
			return nil, errors.New("SSH authentication challenge exceeded its limit")
		}
		prompt := Prompt{Kind: "keyboard-interactive", Title: "SSH authentication challenge", Message: instruction, Actions: []string{"cancel", "submit"}}
		for i, question := range questions {
			prompt.Fields = append(prompt.Fields, PromptField{Label: question, Echo: echo[i]})
		}
		if err := validatePrompt(prompt); err != nil {
			return nil, err
		}
		reply, err := ask(lifetime, prompt)
		if err != nil {
			return nil, err
		}
		if reply.Action != "submit" || len(reply.Answers) != len(questions) {
			return nil, errors.New("SSH authentication challenge was not completed")
		}
		return reply.Answers, nil
	})
	auth := []ssh.AuthMethod{}
	switch req.AuthMethod {
	case "key":
		auth = append(auth, ssh.PublicKeys(signer))
	case "password":
		auth = append(auth, ssh.Password(req.Password))
	case "keyboard":
	default:
		return nil, errors.New("unsupported SSH authentication method")
	}
	auth = append(auth, keyboard)
	algorithms := sshAlgorithmPolicy(req.SSHCompatibility)
	config := &ssh.ClientConfig{User: req.Username, Auth: auth, HostKeyCallback: hostKey, HostKeyAlgorithms: algorithms.HostKeys,
		Config:        ssh.Config{KeyExchanges: algorithms.KeyExchanges, Ciphers: algorithms.Ciphers, MACs: algorithms.MACs},
		ClientVersion: "SSH-2.0-NetworkToolkit", BannerCallback: func(message string) error {
			if len(message) > 16*1024 {
				return errors.New("SSH authentication banner exceeded its limit")
			}
			if cb.Output != nil {
				cb.Output([]byte(message))
			}
			return lifetime.Err()
		}}
	clientConnection, channels, requests, err := ssh.NewClientConn(connection, endpoint, config)
	if err != nil {
		return nil, diagnoseSSHHandshake(fmt.Errorf("SSH handshake: %w", err), req.SSHCompatibility)
	}
	client := ssh.NewClient(clientConnection, channels, requests)
	if err := connection.SetDeadline(time.Time{}); err != nil {
		client.Close()
		return nil, err
	}
	t := &sshTransport{conn: connection, client: client, ctx: lifetime, cancel: cancel, stopCancellation: stopCancellation, done: make(chan error, 1)}
	opened = true
	keepaliveDone := make(chan struct{})
	go func() { defer close(keepaliveDone); t.keepalive(timing) }()
	go func() {
		err := client.Wait()
		t.errMu.Lock()
		if t.exitErr != nil {
			err = t.exitErr
		} else if lifetime.Err() != nil {
			err = nil
		}
		t.errMu.Unlock()
		_ = t.Close()
		<-keepaliveDone
		t.done <- err
		close(t.done)
	}()
	return t, nil
}

func parseSSHSigner(ctx context.Context, req ConnectRequest, prompt func(context.Context, Prompt) (PromptReply, error)) (ssh.Signer, error) {
	key := []byte(req.PrivateKey)
	defer clear(key)
	signer, err := ssh.ParsePrivateKey(key)
	var missing *ssh.PassphraseMissingError
	if errors.As(err, &missing) {
		passphrase := req.Passphrase
		for attempt := 0; attempt < 3; attempt++ {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			if passphrase == "" || attempt > 0 {
				message := "Enter the passphrase for the selected private key. It is used only in this live connection."
				if attempt > 0 {
					message = "The passphrase was not accepted. Enter it again."
				}
				reply, promptErr := prompt(ctx, Prompt{Kind: "key-passphrase", Title: "Unlock SSH private key", Message: message,
					Fields: []PromptField{{Label: "Key passphrase", Echo: false}}, Actions: []string{"cancel", "submit"}})
				if promptErr != nil {
					return nil, promptErr
				}
				if reply.Action != "submit" || len(reply.Answers) != 1 {
					return nil, errors.New("private-key passphrase was not supplied")
				}
				passphrase = reply.Answers[0]
			}
			secret := []byte(passphrase)
			signer, err = ssh.ParsePrivateKeyWithPassphrase(key, secret)
			clear(secret)
			passphrase = ""
			if err == nil {
				break
			}
		}
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if err != nil {
		return nil, fmt.Errorf("read SSH private key: %w", err)
	}
	if signer.PublicKey().Type() == ssh.InsecureKeyAlgoDSA {
		return nil, errors.New("DSA private keys are not supported")
	}
	if signer.PublicKey().Type() == ssh.KeyAlgoRSA {
		algorithmSigner, ok := signer.(ssh.AlgorithmSigner)
		if !ok {
			return nil, errors.New("RSA key does not support SHA256/SHA512 signatures")
		}
		algorithms := []string{ssh.KeyAlgoRSASHA512, ssh.KeyAlgoRSASHA256}
		if req.SSHCompatibility == SSHCompatibilityLegacy {
			algorithms = append(algorithms, ssh.KeyAlgoRSA)
		}
		return ssh.NewSignerWithAlgorithms(algorithmSigner, algorithms)
	}
	return signer, nil
}

type terminalOutput struct{ output func([]byte) }

func (w terminalOutput) Write(data []byte) (int, error) {
	if w.output != nil {
		w.output(data)
	}
	return len(data), nil
}
func (t *sshTransport) Write(data []byte) (int, error) { return t.stdin.Write(data) }
func (t *sshTransport) Resize(cols, rows int) error    { return t.session.WindowChange(rows, cols) }
func (t *sshTransport) Done() <-chan error             { return t.done }

// commitOwnership is called only once a manager accepts a successfully opened
// shell. Until then cancellation closes just that attempted shell channel.
func (t *sshTransport) commitOwnership() error {
	if t.base == nil {
		return nil
	}
	t.ownerMu.Lock()
	defer t.ownerMu.Unlock()
	if t.ownershipClosed || t.ctx.Err() != nil {
		return ErrNotConnected
	}
	if !t.ownsBase {
		t.ownsBase = true
		if t.onOwned != nil {
			t.onOwned()
		}
	}
	return nil
}

func (t *sshTransport) Close() error {
	var err error
	t.closeOnce.Do(func() {
		t.ownerMu.Lock()
		t.ownershipClosed = true
		owns := t.ownsBase
		t.ownerMu.Unlock()
		t.cancel()
		t.stopCancellation()
		if t.base == nil {
			err = t.conn.Close()
			_ = t.client.Close()
		} else if owns {
			err = t.base.Close()
		}
		if t.stdin != nil {
			_ = t.stdin.Close()
		}
		if t.session != nil {
			_ = t.session.Close()
		}
	})
	return err
}

func (t *sshTransport) keepalive(timing sshTiming) {
	if timing.keepaliveInterval <= 0 {
		<-t.ctx.Done()
		return
	}
	ticker := time.NewTicker(timing.keepaliveInterval)
	defer ticker.Stop()
	for {
		select {
		case <-t.ctx.Done():
			return
		case <-ticker.C:
			finished := make(chan error, 1)
			go func() { _, _, err := t.client.SendRequest("keepalive@openssh.com", true, nil); finished <- err }()
			timer := time.NewTimer(timing.keepaliveTimeout)
			var err error
			select {
			case err = <-finished:
			case <-t.ctx.Done():
				timer.Stop()
				_ = t.Close()
				<-finished
				return
			case <-timer.C:
				err = errors.New("SSH keepalive timed out")
				t.errMu.Lock()
				t.exitErr = err
				t.errMu.Unlock()
				_ = t.Close()
				<-finished
			}
			timer.Stop()
			if err != nil {
				t.errMu.Lock()
				if t.exitErr == nil {
					t.exitErr = err
				}
				t.errMu.Unlock()
				_ = t.Close()
				return
			}
		}
	}
}
