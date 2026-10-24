package macros

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"strings"
	"sync"
	"testing"
	"time"

	"networktoolkit/internal/terminal"
)

type fakeSource struct {
	mu      sync.Mutex
	queue   []terminal.AutomationOutput
	ended   bool
	changed chan struct{}
}

func newSource() *fakeSource { return &fakeSource{changed: make(chan struct{})} }
func (s *fakeSource) push(v terminal.AutomationOutput) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.ended {
		return
	}
	v.Data = append([]byte(nil), v.Data...)
	s.queue = append(s.queue, v)
	close(s.changed)
	s.changed = make(chan struct{})
}
func (s *fakeSource) Close() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.ended {
		s.ended = true
		close(s.changed)
	}
}
func (s *fakeSource) Read(ctx context.Context) (terminal.AutomationOutput, error) {
	for {
		s.mu.Lock()
		if len(s.queue) > 0 {
			out := s.queue[0]
			s.queue[0] = terminal.AutomationOutput{}
			s.queue = s.queue[1:]
			s.mu.Unlock()
			return out, nil
		}
		ended, changed := s.ended, s.changed
		s.mu.Unlock()
		if ended {
			return terminal.AutomationOutput{}, io.EOF
		}
		select {
		case <-ctx.Done():
			return terminal.AutomationOutput{}, ctx.Err()
		case <-changed:
		}
	}
}

type written struct {
	id         string
	generation uint64
	text       string
}
type fakeBackend struct {
	mu          sync.Mutex
	live        map[string]Target
	sources     map[string][]*fakeSource
	seq         uint64
	writes      chan written
	onSubscribe func(string)
	onWrite     func(written)
}

func newBackend(ids ...string) *fakeBackend {
	b := &fakeBackend{live: map[string]Target{}, sources: map[string][]*fakeSource{}, writes: make(chan written, 1024)}
	for _, id := range ids {
		b.live[id] = Target{ID: id, Generation: 1, Label: "Device " + id, Host: id, Username: "admin", LineEnding: "cr"}
	}
	return b
}
func (b *fakeBackend) targets() []Target {
	b.mu.Lock()
	defer b.mu.Unlock()
	out := []Target{}
	for _, t := range b.live {
		out = append(out, t)
	}
	return out
}
func (b *fakeBackend) write(id string, g uint64, data []byte) (uint64, error) {
	b.mu.Lock()
	target, ok := b.live[id]
	if !ok || target.Generation != g {
		b.mu.Unlock()
		return 0, terminal.ErrTransportChanged
	}
	cursor := b.seq
	w := written{id, g, string(data)}
	hook := b.onWrite
	b.writes <- w
	b.mu.Unlock()
	if hook != nil {
		hook(w)
	}
	return cursor, nil
}
func (b *fakeBackend) subscribe(id string, g uint64) (outputSource, error) {
	b.mu.Lock()
	target, ok := b.live[id]
	if !ok || target.Generation != g {
		b.mu.Unlock()
		return nil, terminal.ErrTransportChanged
	}
	s := newSource()
	b.sources[id] = append(b.sources[id], s)
	hook := b.onSubscribe
	b.mu.Unlock()
	if hook != nil {
		hook(id)
	}
	return s, nil
}
func (b *fakeBackend) emit(id, text string) {
	b.mu.Lock()
	b.seq++
	v := terminal.AutomationOutput{Seq: b.seq, Data: []byte(text)}
	for _, s := range b.sources[id] {
		s.push(v)
	}
	b.mu.Unlock()
}
func (b *fakeBackend) reconnect(id string) {
	b.mu.Lock()
	for _, s := range b.sources[id] {
		s.Close()
	}
	delete(b.sources, id)
	t := b.live[id]
	t.Generation++
	b.live[id] = t
	b.mu.Unlock()
}
func engineFixture(t *testing.T, ids ...string) (*Engine, *fakeBackend) {
	t.Helper()
	b := newBackend(ids...)
	e := newEngine(context.Background(), b)
	t.Cleanup(e.Close)
	return e, b
}
func definition(commands ...string) Definition {
	return Definition{ID: "fixture", Name: "Test macro", WaitForPrompt: true, Prompt: DefaultPrompt, TimeoutMs: 1500, Steps: FromCommands(strings.Join(commands, "\n"))}
}
func refs(ids ...string) []TargetRef {
	out := []TargetRef{}
	for _, id := range ids {
		out = append(out, TargetRef{ID: id, Generation: 1})
	}
	return out
}
func awaitWrite(t *testing.T, b *fakeBackend, id, text string) {
	t.Helper()
	timer := time.NewTimer(3 * time.Second)
	defer timer.Stop()
	select {
	case w := <-b.writes:
		if w.id != id || w.text != text {
			t.Fatalf("write=%+v, expected %q %q", w, id, text)
		}
	case <-timer.C:
		t.Fatalf("no write for %s %q", id, text)
	}
}
func awaitRun(t *testing.T, e *Engine, id string, check func(Run) bool) Run {
	t.Helper()
	deadline := time.Now().Add(4 * time.Second)
	for {
		run, err := e.Get(id)
		if err != nil {
			t.Fatal(err)
		}
		if check(run) {
			return run
		}
		if time.Now().After(deadline) {
			t.Fatalf("run did not reach expected state: %+v", run)
		}
		time.Sleep(time.Millisecond)
	}
}
func finished(r Run) bool { return r.FinishedAt != "" }
func noWrite(t *testing.T, b *fakeBackend) {
	t.Helper()
	select {
	case w := <-b.writes:
		t.Fatalf("unexpected write %+v", w)
	case <-time.After(30 * time.Millisecond):
	}
}

func TestEngineTargetsProgressIndependentlyAndKeepOutput(t *testing.T) {
	e, b := engineFixture(t, "a", "b")
	run, err := e.Start(StartRequest{Macro: definition("show one", "show two"), Targets: refs("a", "b")})
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	for range 2 {
		select {
		case w := <-b.writes:
			if w.text != "show one\r" {
				t.Fatal(w)
			}
			seen[w.id] = true
		case <-time.After(time.Second):
			t.Fatal("missing initial command")
		}
	}
	if len(seen) != 2 {
		t.Fatal(seen)
	}
	b.emit("a", "first A\r\na#")
	awaitWrite(t, b, "a", "show two\r")
	noWrite(t, b)
	b.emit("b", "first B\r\nb#")
	awaitWrite(t, b, "b", "show two\r")
	b.emit("a", "second A\r\na#")
	b.emit("b", "second B\r\nb#")
	result := awaitRun(t, e, run.ID, finished)
	if result.State != "completed" {
		t.Fatalf("%+v", result)
	}
	for _, target := range result.Targets {
		if target.Step != 2 || target.Total != 2 || target.State != "completed" {
			t.Fatal(target)
		}
	}
	output, err := e.Output(run.ID, "")
	if err != nil {
		t.Fatal(err)
	}
	for _, text := range []string{"first A", "second A", "first B", "second B", "Step 1", "Step 2"} {
		if !bytes.Contains(output, []byte(text)) {
			t.Fatalf("missing %q", text)
		}
	}
}

func TestEngineWaitRequiresFreshOutputAfterAtomicWriteCursor(t *testing.T) {
	e, b := engineFixture(t, "a")
	b.onSubscribe = func(id string) { b.emit(id, "\r\nold#") }
	run, err := e.Start(StartRequest{Macro: definition("show fresh"), Targets: refs("a")})
	if err != nil {
		t.Fatal(err)
	}
	awaitWrite(t, b, "a", "show fresh\r")
	time.Sleep(30 * time.Millisecond)
	current, _ := e.Get(run.ID)
	if current.FinishedAt != "" {
		t.Fatalf("old prompt completed new command: %+v", current)
	}
	b.emit("a", "new data\r\na#")
	result := awaitRun(t, e, run.ID, finished)
	if result.State != "completed" {
		t.Fatal(result)
	}
}

func TestEngineLongCaptureRedactsAcrossChunksAndHandlesPager(t *testing.T) {
	e, b := engineFixture(t, "a")
	def := definition("show {{token}}")
	def.TimeoutMs = 4000
	def.AutoPager = true
	def.Parameters = []Parameter{{Name: "token", Label: "Token", Required: true, Secret: true}}
	run, err := e.Start(StartRequest{Macro: def, Targets: refs("a"), Values: map[string]string{"token": "private-token"}})
	if err != nil {
		t.Fatal(err)
	}
	awaitWrite(t, b, "a", "show private-token\r")
	b.emit("a", "\x1b[3")
	b.emit("a", "1mprivate-")
	b.emit("a", "token\x1b[0m\r\n--More--")
	awaitWrite(t, b, "a", " ")
	block := strings.Repeat("long-output-line-", 1024)
	for range 640 {
		b.emit("a", block)
	}
	b.emit("a", "\r\nFINAL-END\r\na#")
	result := awaitRun(t, e, run.ID, finished)
	if result.State != "completed" {
		t.Fatal(result)
	}
	output, err := e.Output(run.ID, "a")
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(output, []byte("private-token")) || bytes.Contains(output, []byte("\x1b")) {
		t.Fatal("capture leaked secret or ANSI")
	}
	if !bytes.Contains(output, []byte("*************")) || !bytes.Contains(output, []byte("FINAL-END")) {
		t.Fatal("capture lost beginning or ending")
	}
	if got := bytes.Count(output, []byte("long-output-line-")); got != 640*1024 {
		t.Fatalf("capture lost long output: %d", got)
	}
}

func TestEnginePauseStepResumeAndStopKeepTerminalOpen(t *testing.T) {
	e, b := engineFixture(t, "a")
	def := definition("show first", "show second", "show third")
	run, err := e.Start(StartRequest{Macro: def, Targets: refs("a")})
	if err != nil {
		t.Fatal(err)
	}
	awaitWrite(t, b, "a", "show first\r")
	if _, err = e.Control(run.ID, "pause"); err != nil {
		t.Fatal(err)
	}
	b.emit("a", "\r\na#")
	noWrite(t, b)
	if _, err = e.Control(run.ID, "step"); err != nil {
		t.Fatal(err)
	}
	awaitWrite(t, b, "a", "show second\r")
	b.emit("a", "\r\na#")
	noWrite(t, b)
	if _, err = e.Control(run.ID, "resume"); err != nil {
		t.Fatal(err)
	}
	awaitWrite(t, b, "a", "show third\r")
	if _, err = e.Control(run.ID, "interrupt"); err != nil {
		t.Fatal(err)
	}
	awaitWrite(t, b, "a", "\x03")
	result := awaitRun(t, e, run.ID, finished)
	if result.State != "stopped" {
		t.Fatalf("%+v", result)
	}
	if _, err = b.write("a", 1, []byte("still open\r")); err != nil {
		t.Fatal(err)
	}
	awaitWrite(t, b, "a", "still open\r")
}

func TestEnginePauseStepCanBeReviewedThenResumed(t *testing.T) {
	e, b := engineFixture(t, "a")
	def := definition("show done")
	def.Steps = append([]Step{{ID: "review", Kind: "pause", Text: "Review change"}}, def.Steps...)
	run, err := e.Start(StartRequest{Macro: def, Targets: refs("a")})
	if err != nil {
		t.Fatal(err)
	}
	awaitRun(t, e, run.ID, func(r Run) bool { return r.State == "paused" && r.Targets[0].Waiting == "Review change" })
	noWrite(t, b)
	_, _ = e.Control(run.ID, "resume")
	awaitWrite(t, b, "a", "show done\r")
	b.emit("a", "\r\na#")
	if result := awaitRun(t, e, run.ID, finished); result.State != "completed" {
		t.Fatal(result)
	}
}

func TestEngineReconnectCancelsOnlyOriginalTarget(t *testing.T) {
	e, b := engineFixture(t, "a", "b")
	run, err := e.Start(StartRequest{Macro: definition("show one", "show two"), Targets: refs("a", "b")})
	if err != nil {
		t.Fatal(err)
	}
	for range 2 {
		select {
		case <-b.writes:
		case <-time.After(time.Second):
			t.Fatal("missing initial write")
		}
	}
	b.reconnect("a")
	b.emit("b", "\r\nb#")
	awaitWrite(t, b, "b", "show two\r")
	b.emit("b", "\r\nb#")
	result := awaitRun(t, e, run.ID, finished)
	if result.State != "partial" {
		t.Fatalf("%+v", result)
	}
	for _, target := range result.Targets {
		if target.ID == "a" && target.State != "disconnected" {
			t.Fatal(target)
		}
	}
	noWrite(t, b)
	sent, err := e.Send(SendRequest{Targets: refs("a"), Text: "replacement", Execute: true})
	if err != nil {
		t.Fatal(err)
	}
	if sent.Targets[0].Sent {
		t.Fatal("stale generation wrote replacement")
	}
}

func TestEngineManualInputStopsOnlyHumanInput(t *testing.T) {
	e, b := engineFixture(t, "a")
	run, err := e.Start(StartRequest{Macro: definition("show one", "show two"), Targets: refs("a")})
	if err != nil {
		t.Fatal(err)
	}
	awaitWrite(t, b, "a", "show one\r")
	for _, reply := range []string{"\x1b[12;3R", "\x1b[?1;2c", "\x1b[0n", "\x1bP1$r0m\x1b\\", "\x1b]10;rgb:ffff/ffff/ffff\x1b\\", "\x1b[I"} {
		e.ManualInput("a", []byte(reply))
	}
	time.Sleep(20 * time.Millisecond)
	current, _ := e.Get(run.ID)
	if current.FinishedAt != "" {
		t.Fatalf("protocol reply stopped macro: %+v", current)
	}
	e.ManualInput("a", []byte{3})
	if result := awaitRun(t, e, run.ID, finished); result.State != "stopped" {
		t.Fatal(result)
	}
	b.emit("a", "\r\na#")
	noWrite(t, b)
}

func TestEngineBusyPartialStartAndSendLineEndings(t *testing.T) {
	e, b := engineFixture(t, "a", "s")
	b.mu.Lock()
	serial := b.live["s"]
	serial.Type = "serial"
	serial.LineEnding = "crlf"
	b.live["s"] = serial
	b.mu.Unlock()
	first, err := e.Start(StartRequest{Macro: definition("show one"), Targets: refs("a")})
	if err != nil {
		t.Fatal(err)
	}
	awaitWrite(t, b, "a", "show one\r")
	second, err := e.Start(StartRequest{Macro: definition("other"), Targets: refs("a", "s")})
	if err != nil {
		t.Fatal(err)
	}
	awaitWrite(t, b, "s", "other\r\n")
	b.emit("s", "\r\ns#")
	result := awaitRun(t, e, second.ID, finished)
	if result.State != "partial" || result.Targets[0].State != "failed" {
		t.Fatal(result)
	}
	_, _ = e.Control(first.ID, "stop")
	awaitRun(t, e, first.ID, finished)
	sent, err := e.Send(SendRequest{Targets: refs("a", "s"), Text: "one\ntwo", Execute: true})
	if err != nil {
		t.Fatal(err)
	}
	for _, target := range sent.Targets {
		if !target.Sent {
			t.Fatal(target)
		}
	}
	awaitWrite(t, b, "a", "one\rtwo\r")
	awaitWrite(t, b, "s", "one\r\ntwo\r\n")
	if _, err = e.Send(SendRequest{Targets: refs("a"), Text: "one\ntwo"}); err == nil {
		t.Fatal("Insert allowed Enter")
	}
}

func TestEngineTimeoutMissingParametersAndBoundedRunHistory(t *testing.T) {
	e, b := engineFixture(t, "a")
	def := definition("show timeout")
	def.TimeoutMs = 100
	run, err := e.Start(StartRequest{Macro: def, Targets: refs("a")})
	if err != nil {
		t.Fatal(err)
	}
	awaitWrite(t, b, "a", "show timeout\r")
	result := awaitRun(t, e, run.ID, finished)
	if result.State != "failed" || !strings.Contains(result.Targets[0].Error, "timed out") {
		t.Fatal(result)
	}
	def = definition("show {{value}}")
	def.Parameters = []Parameter{{Name: "value", Label: "Value", Required: true}}
	if _, err = e.Start(StartRequest{Macro: def, Targets: refs("a")}); err == nil {
		t.Fatal("missing required parameter accepted")
	}
	b.onWrite = func(w written) { b.emit(w.id, "\r\na#") }
	for i := range 23 {
		def = definition(fmt.Sprintf("show %d", i))
		run, err = e.Start(StartRequest{Macro: def, Targets: refs("a")})
		if err != nil {
			t.Fatal(err)
		}
		awaitRun(t, e, run.ID, finished)
		<-b.writes
	}
	if got := len(e.List()); got != 20 {
		t.Fatalf("history=%d", got)
	}
	if _, err = e.Get("missing"); err == nil {
		t.Fatal("missing run accepted")
	}
}

func TestAnsiDecoderPreservesUnicodeAndSplitSequences(t *testing.T) {
	var d ansiDecoder
	got := d.decode([]byte("\x1b[3")) + d.decode([]byte("1m红色 text\x1b]0;title")) + d.decode([]byte("\x07\x1b[0m\r\n"))
	if got != "红色 text\r\n" {
		t.Fatalf("decode %q", got)
	}
}

func TestEngineNoPromptCaptureWaitsForIdleAndRetainsSlowOutput(t *testing.T) {
	e, b := engineFixture(t, "a")
	def := definition("show slow")
	def.WaitForPrompt = false
	def.TimeoutMs = 3000
	emitted := make(chan struct{})
	b.onWrite = func(w written) {
		go func() {
			defer close(emitted)
			b.emit(w.id, "FIRST\r\n")
			time.Sleep(180 * time.Millisecond)
			b.emit(w.id, "MIDDLE\r\n")
			time.Sleep(180 * time.Millisecond)
			b.emit(w.id, "FINAL\r\n")
		}()
	}
	run, err := e.Start(StartRequest{Macro: def, Targets: refs("a")})
	if err != nil {
		t.Fatal(err)
	}
	awaitWrite(t, b, "a", "show slow\r")
	<-emitted
	result := awaitRun(t, e, run.ID, finished)
	if result.State != "completed" {
		t.Fatal(result)
	}
	output, err := e.Output(run.ID, "a")
	if err != nil {
		t.Fatal(err)
	}
	for _, part := range []string{"FIRST", "MIDDLE", "FINAL"} {
		if !bytes.Contains(output, []byte(part)) {
			t.Fatalf("slow capture omitted %s", part)
		}
	}
}

func TestEngineNoPromptContinuousOutputReportsTimeout(t *testing.T) {
	e, b := engineFixture(t, "a")
	def := definition("show continuous")
	def.WaitForPrompt = false
	def.TimeoutMs = 300
	run, err := e.Start(StartRequest{Macro: def, Targets: refs("a")})
	if err != nil {
		t.Fatal(err)
	}
	awaitWrite(t, b, "a", "show continuous\r")
	stopped := make(chan struct{})
	defer close(stopped)
	go func() {
		ticker := time.NewTicker(20 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-stopped:
				return
			case <-ticker.C:
				b.emit("a", "still streaming\r\n")
			}
		}
	}()
	result := awaitRun(t, e, run.ID, finished)
	if result.State != "failed" || !strings.Contains(result.Targets[0].Error, "did not become idle") {
		t.Fatalf("continuous output appeared complete: %+v", result)
	}
}

func TestEngineRedactsSecretAcrossStepHeading(t *testing.T) {
	e, b := engineFixture(t, "a")
	def := definition("first", "second")
	def.WaitForPrompt = false
	def.TimeoutMs = 3000
	def.PacingMs = 40
	def.Parameters = []Parameter{{Name: "token", Label: "Token", Secret: true, Required: true}}
	b.onWrite = func(w written) {
		if w.text == "first\r" {
			b.emit(w.id, "private-")
		} else {
			b.emit(w.id, "token\r\n")
		}
	}
	run, err := e.Start(StartRequest{Macro: def, Targets: refs("a"), Values: map[string]string{"token": "private-token"}})
	if err != nil {
		t.Fatal(err)
	}
	awaitWrite(t, b, "a", "first\r")
	awaitWrite(t, b, "a", "second\r")
	result := awaitRun(t, e, run.ID, finished)
	if result.State != "completed" {
		t.Fatal(result)
	}
	output, err := e.Output(run.ID, "a")
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(output, []byte("private-")) || bytes.Contains(output, []byte("token")) {
		t.Fatalf("step boundary leaked secret fragments: %q", output)
	}
	if !bytes.Contains(output, []byte("Step 2")) {
		t.Fatal("missing step heading")
	}
}

func TestEngineMaximumStepTextWithEnterAndSerialNormalization(t *testing.T) {
	e, b := engineFixture(t, "a", "s")
	b.mu.Lock()
	serial := b.live["s"]
	serial.Type = "serial"
	serial.LineEnding = "crlf"
	b.live["s"] = serial
	b.mu.Unlock()
	def := definition(strings.Repeat("x", MaxStepTextBytes))
	run, err := e.Start(StartRequest{Macro: def, Targets: refs("a", "s")})
	if err != nil {
		t.Fatal(err)
	}
	for range 2 {
		select {
		case w := <-b.writes:
			want := MaxStepTextBytes + 1
			if w.id == "s" {
				want++
			}
			if len(w.text) != want {
				t.Fatalf("encoded command length=%d want%d", len(w.text), want)
			}
			b.emit(w.id, "\r\nready#")
		case <-time.After(time.Second):
			t.Fatal("missing boundary write")
		}
	}
	if result := awaitRun(t, e, run.ID, finished); result.State != "completed" {
		t.Fatal(result)
	}
}

func TestEngineActiveCaptureDoesNotExposePartialSecret(t *testing.T) {
	e, b := engineFixture(t, "a")
	def := definition("show {{token}}")
	def.Parameters = []Parameter{{Name: "token", Label: "Token", Secret: true, Required: true}}
	run, err := e.Start(StartRequest{Macro: def, Targets: refs("a"), Values: map[string]string{"token": "private-token"}})
	if err != nil {
		t.Fatal(err)
	}
	awaitWrite(t, b, "a", "show private-token\r")
	b.emit("a", "private-")
	awaitRun(t, e, run.ID, func(r Run) bool { return r.Targets[0].OutputBytes == len("private-") })
	output, err := e.Output(run.ID, "a")
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(output, []byte("private-")) {
		t.Fatalf("active download exposed secret prefix: %q", output)
	}
	b.emit("a", "token\r\na#")
	if result := awaitRun(t, e, run.ID, finished); result.State != "completed" {
		t.Fatal(result)
	}
}

func TestEngineOwnsPagerUntilTargetFinishes(t *testing.T) {
	e, b := engineFixture(t, "a")
	if e.OwnsPager("a") {
		t.Fatal("idle target owns pager")
	}
	run, err := e.Start(StartRequest{Macro: definition("show one"), Targets: refs("a")})
	if err != nil {
		t.Fatal(err)
	}
	awaitWrite(t, b, "a", "show one\r")
	if !e.OwnsPager("a") {
		t.Fatal("active target did not own pager")
	}
	_, _ = e.Control(run.ID, "stop")
	awaitRun(t, e, run.ID, finished)
	if e.OwnsPager("a") {
		t.Fatal("finished target retained pager ownership")
	}
}
