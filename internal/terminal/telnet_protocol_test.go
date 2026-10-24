package terminal

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net"
	"testing"
	"time"
)

func TestTelnetQMethodNegotiationTransitions(t *testing.T) {
	for _, test := range []struct {
		before   telnetQState
		positive bool
		after    telnetQState
		response int
	}{
		{qNo, true, qYes, 1}, {qYes, true, qYes, 0}, {qWantNo, true, qNo, 0},
		{qWantNoOpposite, true, qYes, 0}, {qWantYes, true, qYes, 0}, {qWantYesOpposite, true, qWantNo, -1},
		{qNo, false, qNo, 0}, {qYes, false, qNo, -1}, {qWantNo, false, qNo, 0},
		{qWantNoOpposite, false, qWantYes, 1}, {qWantYes, false, qNo, 0}, {qWantYesOpposite, false, qNo, 0},
	} {
		state := test.before
		response := state.receive(test.positive, true)
		if state != test.after || response != test.response {
			t.Errorf("receive(%d,%v) -> %d/%d, expected %d/%d", test.before, test.positive, state, response, test.after, test.response)
		}
	}
	state := qNo
	if state.receive(true, false) != -1 || state != qNo {
		t.Fatal("an unsupported option was enabled")
	}
	if state.request(true) != 1 || state != qWantYes {
		t.Fatal("initial request was not negotiated")
	}
	if state.request(false) != 0 || state != qWantYesOpposite {
		t.Fatal("opposite request was not queued")
	}
	if state.receive(true, true) != -1 || state != qWantNo {
		t.Fatal("queued disable was not sent after enable acknowledgement")
	}
	if state.request(true) != 0 || state != qWantNoOpposite {
		t.Fatal("enable while disabling was not queued")
	}
	if state.receive(false, true) != 1 || state != qWantYes {
		t.Fatal("queued enable was not sent after disable acknowledgement")
	}
	if state.receive(false, true) != 0 || state != qNo {
		t.Fatal("refusal restarted negotiation")
	}
}

func TestTelnetNegotiationFragmentationAndWindowSize(t *testing.T) {
	input := []byte{'a', '\r', 0, 'b', '\r', '\n', tnIAC, tnWILL, tnEcho, tnIAC, tnDO, tnNAWS,
		tnIAC, tnDO, tnTerminalType, tnIAC, tnSB, tnTerminalType, 1, tnIAC, tnSE,
		tnIAC, tnWILL, 99, tnIAC, tnDO, tnEcho, tnIAC, tnIAC, 'z'}
	wantOutput := []byte{'a', '\r', 'b', '\r', '\n', 255, 'z'}
	wantReply := []byte{tnIAC, tnSB, tnNAWS, 0, 255, 255, 0, 24, tnIAC, tnSE}
	wantReply = append(wantReply, []byte{tnIAC, tnSB, tnTerminalType, 0}...)
	wantReply = append(wantReply, []byte("xterm-256color")...)
	wantReply = append(wantReply, tnIAC, tnSE, tnIAC, tnDONT, 99, tnIAC, tnWONT, tnEcho)
	for fragment := 1; fragment <= len(input); fragment++ {
		p := newTelnetProtocol(255, 24, "xterm-256color")
		if initial := p.initial(); len(initial) != 21 {
			t.Fatal("unexpected initial negotiation")
		}
		var output, reply []byte
		for at := 0; at < len(input); at += fragment {
			end := at + fragment
			if end > len(input) {
				end = len(input)
			}
			o, r, err := p.feed(input[at:end])
			if err != nil {
				t.Fatal(err)
			}
			output = append(output, o...)
			reply = append(reply, r...)
		}
		if !bytes.Equal(output, wantOutput) || !bytes.Equal(reply, wantReply) {
			t.Fatalf("fragment size %d: output=%v reply=%v", fragment, output, reply)
		}
		_, reply, err := p.feed([]byte{tnIAC, tnWILL, tnEcho, tnIAC, tnDO, tnNAWS})
		if err != nil || len(reply) != 0 {
			t.Fatal("duplicate accepted negotiation caused a loop")
		}
		resized, err := p.resize(80, 40)
		if err != nil || !bytes.Equal(resized, []byte{255, 250, 31, 0, 80, 0, 40, 255, 240}) {
			t.Fatalf("NAWS resize: %v %v", resized, err)
		}
		_, _, _ = p.feed([]byte{tnIAC, tnDONT, tnNAWS})
		if resized, err = p.resize(90, 40); err != nil || len(resized) != 0 {
			t.Fatal("NAWS continued after refusal")
		}
	}
}

func TestTelnetBinaryDirectionsAndNVTEncoding(t *testing.T) {
	p := newTelnetProtocol(80, 24, "")
	p.initial()
	o, _, err := p.feed([]byte{tnIAC, tnDO, tnBinary, '\r', 0})
	if err != nil || !bytes.Equal(o, []byte{'\r'}) || p.remote[tnBinary] == qYes || p.local[tnBinary] != qYes {
		t.Fatal("binary negotiation was not directional")
	}
	o, _, err = p.feed([]byte{tnIAC, tnWILL, tnBinary, '\r', 0, tnIAC, tnIAC})
	if err != nil || !bytes.Equal(o, []byte{'\r', 0, 255}) {
		t.Fatalf("binary data changed: %v %v", o, err)
	}
	input := []byte{'a', '\r', 'b', '\n', '\r', '\n', 0, 255}
	wire, ends := telnetEncode(input, false)
	want := []byte{'a', '\r', 0, 'b', '\r', '\n', '\r', '\n', 0, 255, 255}
	if !bytes.Equal(wire, want) || len(ends) != len(input) || ends[len(ends)-1] != len(wire) {
		t.Fatalf("NVT encoding: %v %v", wire, ends)
	}
	wire, _ = telnetEncode(input, true)
	if !bytes.Equal(wire, append(append([]byte(nil), input...), 255)) {
		t.Fatal("binary input was normalized")
	}
}

func TestTelnetRejectsOversizedAndMalformedSubnegotiation(t *testing.T) {
	p := newTelnetProtocol(80, 24, "")
	packet := append([]byte{tnIAC, tnSB, 99}, bytes.Repeat([]byte{'a'}, 4097)...)
	if _, _, err := p.feed(packet); err == nil {
		t.Fatal("unbounded subnegotiation accepted")
	}
	p = newTelnetProtocol(80, 24, "")
	if _, _, err := p.feed([]byte{tnIAC, tnSB, 99, tnIAC, tnWILL}); err == nil {
		t.Fatal("malformed subnegotiation accepted")
	}
	p = newTelnetProtocol(80, 24, "")
	if _, reply, err := p.feed([]byte{tnIAC, tnSB, tnTerminalType, 1, tnIAC, tnSE}); err != nil || len(reply) != 0 {
		t.Fatal("terminal type sent without negotiated permission")
	}
}

func telnetPipeFixture(t *testing.T) (*telnetTransport, net.Conn, chan []byte, context.CancelFunc) {
	t.Helper()
	client, server := net.Pipe()
	ctx, cancel := context.WithCancel(context.Background())
	output := make(chan []byte, 32)
	type result struct {
		transport *telnetTransport
		err       error
	}
	ready := make(chan result, 1)
	go func() {
		tr, err := startTelnet(ctx, client, ConnectRequest{Cols: 80, Rows: 24, Terminal: "xterm"}, transportCallbacks{Output: func(p []byte) { output <- bytes.Clone(p) }})
		ready <- result{tr, err}
	}()
	_ = server.SetDeadline(time.Now().Add(3 * time.Second))
	initial := make([]byte, 21)
	if _, err := io.ReadFull(server, initial); err != nil {
		t.Fatal(err)
	}
	opened := <-ready
	if opened.err != nil {
		t.Fatal(opened.err)
	}
	t.Cleanup(func() { cancel(); _ = server.Close(); _ = opened.transport.Close() })
	return opened.transport, server, output, cancel
}

func collectTransportOutput(t *testing.T, output <-chan []byte, wanted int) []byte {
	t.Helper()
	var data []byte
	deadline := time.After(3 * time.Second)
	for len(data) < wanted {
		select {
		case p := <-output:
			data = append(data, p...)
		case <-deadline:
			t.Fatalf("terminal output timed out: %v", data)
		}
	}
	return data
}

func TestTelnetRemoteEchoSwitchAndTerminalInput(t *testing.T) {
	tr, server, output, _ := telnetPipeFixture(t)
	if _, err := server.Write([]byte{tnIAC, tnWILL, tnEcho, 'R'}); err != nil {
		t.Fatal(err)
	}
	if got := collectTransportOutput(t, output, 1); string(got) != "R" {
		t.Fatal(got)
	}
	written := make(chan error, 1)
	go func() { _, err := tr.Write([]byte("user\r")); written <- err }()
	wire := make([]byte, 6)
	if _, err := io.ReadFull(server, wire); err != nil || !bytes.Equal(wire, []byte("user\r\x00")) {
		t.Fatalf("wire input %v %v", wire, err)
	}
	if err := <-written; err != nil {
		t.Fatal(err)
	}
	select {
	case got := <-output:
		t.Fatalf("input echoed locally while server owns ECHO: %q", got)
	default:
	}
	if _, err := server.Write([]byte{tnIAC, tnWONT, tnEcho, 'L'}); err != nil {
		t.Fatal(err)
	}
	reply := make([]byte, 3)
	if _, err := io.ReadFull(server, reply); err != nil || !bytes.Equal(reply, []byte{tnIAC, tnDONT, tnEcho}) {
		t.Fatalf("ECHO refusal %v %v", reply, err)
	}
	if got := collectTransportOutput(t, output, 1); string(got) != "L" {
		t.Fatal(got)
	}
	go func() { _, err := tr.Write([]byte{'a', 'b', 127, '\r'}); written <- err }()
	wire = make([]byte, 5)
	if _, err := io.ReadFull(server, wire); err != nil {
		t.Fatal(err)
	}
	if err := <-written; err != nil {
		t.Fatal(err)
	}
	if got := collectTransportOutput(t, output, 7); string(got) != "ab\b \b\r\n" {
		t.Fatalf("local echo: %q", got)
	}
}

func TestTelnetCancellationUnblocksReadAndWrite(t *testing.T) {
	tr, _, _, cancel := telnetPipeFixture(t)
	written := make(chan error, 1)
	go func() { _, err := tr.Write(bytes.Repeat([]byte{'x'}, 1024)); written <- err }()
	cancel()
	select {
	case err := <-written:
		if err == nil {
			t.Fatal("blocked write unexpectedly succeeded")
		}
	case <-time.After(time.Second):
		t.Fatal("cancel did not unblock Telnet write")
	}
	select {
	case err, ok := <-tr.Done():
		if !ok || err != nil {
			t.Fatalf("cancel completion: %v %v", ok, err)
		}
	case <-time.After(time.Second):
		t.Fatal("cancel did not finish Telnet reader")
	}
	if _, ok := <-tr.Done(); ok {
		t.Fatal("Done did not close after its single result")
	}
	if err := tr.Close(); err != nil && !errors.Is(err, net.ErrClosed) {
		t.Fatal(err)
	}
}

func TestTelnetLocalEchoPrecedesImmediateResponse(t *testing.T) {
	client, server := net.Pipe()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	defer server.Close()
	holdEcho, releaseEcho := context.WithCancel(ctx)
	defer releaseEcho()
	echoEntered := make(chan struct{})
	responseEntered := make(chan struct{})
	output := make(chan []byte, 2)
	type result struct {
		transport *telnetTransport
		err       error
	}
	ready := make(chan result, 1)
	go func() {
		tr, err := startTelnet(ctx, client, ConnectRequest{Cols: 80, Rows: 24}, transportCallbacks{Output: func(p []byte) {
			if string(p) == "x" {
				close(echoEntered)
				<-holdEcho.Done()
			}
			if string(p) == "R" {
				close(responseEntered)
			}
			output <- bytes.Clone(p)
		}})
		ready <- result{tr, err}
	}()
	_ = server.SetDeadline(time.Now().Add(3 * time.Second))
	if _, err := io.ReadFull(server, make([]byte, 21)); err != nil {
		t.Fatal(err)
	}
	opened := <-ready
	if opened.err != nil {
		t.Fatal(opened.err)
	}
	defer opened.transport.Close()
	written := make(chan error, 1)
	go func() { _, err := opened.transport.Write([]byte("x")); written <- err }()
	if _, err := io.ReadFull(server, make([]byte, 1)); err != nil {
		t.Fatal(err)
	}
	select {
	case <-echoEntered:
	case <-time.After(time.Second):
		t.Fatal("local echo did not start")
	}
	// Deliver the response while the output sink is temporarily busy copying
	// the local echo. Its callback must not run concurrently or overtake it.
	if _, err := server.Write([]byte("R")); err != nil {
		t.Fatal(err)
	}
	overtook := false
	select {
	case <-responseEntered:
		overtook = true
	case <-time.After(50 * time.Millisecond):
	}
	releaseEcho()
	if err := <-written; err != nil {
		t.Fatal(err)
	}
	if got := collectTransportOutput(t, output, 2); overtook || string(got) != "xR" {
		t.Fatalf("remote response overtook local echo: %q", got)
	}
}
