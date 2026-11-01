package diag

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"errors"
	"io"
	"net"
	"sync/atomic"
	"testing"
	"time"

	"golang.org/x/crypto/ssh"
	"networktoolkit/internal/model"
)

// A synthetic SSH server leaves one SFTP directory reply pending. Cancelling
// the SFTP operation must terminate only that channel, not the live shell or
// any subsequent SFTP channel on the same authenticated TCP connection.
func TestSharedSFTPCancellationUnblocksReadAndRetainsShell(t *testing.T) {
	_, key, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	signer, err := ssh.NewSignerFromKey(key)
	if err != nil {
		t.Fatal(err)
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()

	config := &ssh.ServerConfig{NoClientAuth: true}
	config.AddHostKey(signer)
	entered := make(chan struct{}, 1)
	released := make(chan struct{}, 1)
	var subsystems atomic.Int32
	go func() {
		raw, err := listener.Accept()
		if err != nil {
			return
		}
		defer raw.Close()
		server, channels, requests, err := ssh.NewServerConn(raw, config)
		if err != nil {
			return
		}
		defer server.Close()
		go ssh.DiscardRequests(requests)
		for incoming := range channels {
			go func(incoming ssh.NewChannel) {
				if incoming.ChannelType() != "session" {
					_ = incoming.Reject(ssh.UnknownChannelType, "session only")
					return
				}
				channel, requests, err := incoming.Accept()
				if err != nil {
					return
				}
				defer channel.Close()
				for request := range requests {
					if request.Type == "shell" {
						_ = request.Reply(true, nil)
						_, _ = io.Copy(io.Discard, channel)
						return
					}
					if request.Type != "subsystem" {
						_ = request.Reply(false, nil)
						continue
					}
					subsystems.Add(1)
					_ = request.Reply(true, nil)
					wire := &wireSFTP{in: channel, out: channel}
					typ, _, err := wire.recv()
					if err != nil || typ != fxInit {
						return
					}
					if err = wire.send(fxVersion, sfU32(3)); err != nil {
						return
					}
					typ, payload, err := wire.recv()
					if err != nil || typ != fxOpendir {
						// A probe closes after successful version negotiation.
						return
					}
					id, err := sfGetU32(&payload)
					if err != nil {
						return
					}
					if err = wire.send(fxHandle, append(sfU32(id), sfString("dir")...)); err != nil {
						return
					}
					typ, _, err = wire.recv()
					if err != nil || typ != fxReaddir {
						return
					}
					select {
					case entered <- struct{}{}:
					default:
					}
					// Never send a listing response. Wait for the scoped
					// channel closure caused by cancellation.
					_, _ = io.Copy(io.Discard, channel)
					select {
					case released <- struct{}{}:
					default:
					}
					return
				}
			}(incoming)
		}
	}()

	client, err := ssh.Dial("tcp", listener.Addr().String(), &ssh.ClientConfig{
		User: "synthetic", HostKeyCallback: ssh.FixedHostKey(signer.PublicKey()),
		Timeout: 3 * time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()

	shell, err := client.NewSession()
	if err != nil {
		t.Fatal(err)
	}
	defer shell.Close()
	stdin, err := shell.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err = shell.Shell(); err != nil {
		t.Fatal(err)
	}
	req := func(operation string) model.Request {
		return model.Request{Tool: "sftp", Options: map[string]any{
			"operation": operation, "remotePath": "/", "timeoutMs": 3000,
		}}
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	result := make(chan error, 1)
	go func() {
		_, err := RunSFTPOnClient(ctx, client, req("list"), nil)
		result <- err
	}()
	select {
	case <-entered:
	case <-time.After(3 * time.Second):
		t.Fatal("synthetic server did not receive the blocked SFTP listing")
	}
	cancel()
	select {
	case err := <-result:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("cancellation should be reported accurately, got %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("cancellation did not unblock the pending SFTP reply")
	}
	select {
	case <-released:
	case <-time.After(3 * time.Second):
		t.Fatal("cancelled SFTP channel did not close on the server")
	}
	if _, err = stdin.Write([]byte("shell stays connected\\n")); err != nil {
		t.Fatalf("SFTP cancellation disconnected the active terminal: %v", err)
	}
	probeCtx, probeCancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer probeCancel()
	probe, err := RunSFTPOnClient(probeCtx, client, req("probe"), nil)
	if err != nil || probe.Summary != "SFTP subsystem supported" {
		t.Fatalf("new SFTP channel on the same shell was unavailable: %v (%v)", err, probe.Summary)
	}
	// An unresponsive SFTP server must not keep a borrowed terminal's
	// file browser waiting indefinitely. The idle timeout closes only the
	// SFTP channel; it does not expire the interactive shell connection.
	timed := req("list")
	timed.Options["timeoutMs"] = 500
	timedResult := make(chan error, 1)
	go func() {
		_, err := RunSFTPOnClient(context.Background(), client, timed, nil)
		timedResult <- err
	}()
	select {
	case <-entered:
	case <-time.After(3 * time.Second):
		t.Fatal("timed SFTP listing never reached the fake server")
	}
	select {
	case err := <-timedResult:
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("stalled borrowed SFTP response must return deadline exceeded: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("borrowed SFTP timeout did not interrupt a stalled reply")
	}
	select {
	case <-released:
	case <-time.After(3 * time.Second):
		t.Fatal("timed-out SFTP channel remained open on the server")
	}
	if _, err = stdin.Write([]byte("shell still connected after timeout\n")); err != nil {
		t.Fatalf("SFTP response timeout closed the terminal SSH connection: %v", err)
	}
	probe, err = RunSFTPOnClient(probeCtx, client, req("probe"), nil)
	if err != nil || probe.Summary != "SFTP subsystem supported" {
		t.Fatalf("terminal could not open a new SFTP channel after timeout: %v", err)
	}
	count := subsystems.Load()
	alreadyCancelled, cancelAgain := context.WithCancel(context.Background())
	cancelAgain()
	if _, err := RunSFTPOnClient(alreadyCancelled, client, req("probe"), nil); !errors.Is(err, context.Canceled) {
		t.Fatalf("pre-cancelled operation should not open a channel: %v", err)
	}
	if subsystems.Load() != count {
		t.Fatal("pre-cancelled SFTP request opened an additional channel")
	}
}
