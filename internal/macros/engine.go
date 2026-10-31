package macros

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"networktoolkit/internal/terminal"
)

type Target = terminal.AutomationTarget

type TargetRef struct {
	ID         string `json:"id"`
	Generation uint64 `json:"generation"`
}
type StartRequest struct {
	Macro   Definition        `json:"macro"`
	Targets []TargetRef       `json:"targets"`
	Values  map[string]string `json:"values"`
}
type SendRequest struct {
	Targets []TargetRef `json:"targets"`
	Text    string      `json:"text"`
	Execute bool        `json:"execute"`
}
type SendTargetResult struct {
	ID    string `json:"id"`
	Label string `json:"label"`
	Sent  bool   `json:"sent"`
	Error string `json:"error,omitempty"`
}
type SendResult struct {
	Targets []SendTargetResult `json:"targets"`
}
type RunTarget struct {
	ID          string `json:"id"`
	Generation  uint64 `json:"generation"`
	Label       string `json:"label"`
	State       string `json:"state"`
	Step        int    `json:"step"`
	Total       int    `json:"total"`
	Waiting     string `json:"waiting,omitempty"`
	Error       string `json:"error,omitempty"`
	OutputBytes int    `json:"outputBytes"`
}
type Run struct {
	ID         string      `json:"id"`
	MacroID    string      `json:"macroId"`
	Name       string      `json:"name"`
	State      string      `json:"state"`
	StartedAt  string      `json:"startedAt"`
	FinishedAt string      `json:"finishedAt,omitempty"`
	Targets    []RunTarget `json:"targets"`
}

type outputSource interface {
	Read(context.Context) (terminal.AutomationOutput, error)
	Close()
}
type backend interface {
	targets() []Target
	write(string, uint64, []byte) (uint64, error)
	subscribe(string, uint64) (outputSource, error)
}
type managerBackend struct{ manager *terminal.Manager }

func (b managerBackend) targets() []Target { return b.manager.AutomationTargets() }
func (b managerBackend) write(id string, g uint64, data []byte) (uint64, error) {
	return b.manager.AutomationWriteCursor(id, g, data)
}
func (b managerBackend) subscribe(id string, g uint64) (outputSource, error) {
	return b.manager.AutomationSubscribe(id, g)
}

type capturedChunk struct {
	seq  uint64
	text string
}
type sentPasteCommand struct {
	index int
	command string
	cursor uint64
}
type PasteCaptureStep struct {
	Index int `json:"index"`
	Command string `json:"command"`
	Output string `json:"output"`
}
type PasteCapture struct {
	RunID string `json:"runId"`
	SessionID string `json:"sessionId"`
	State string `json:"state"`
	Steps []PasteCaptureStep `json:"steps"`
}
type task struct {
	target         Target
	view           RunTarget
	source         outputSource
	ctx            context.Context
	cancel         context.CancelFunc
	changed        chan struct{}
	chunks         []capturedChunk
	sentPaste      []sentPasteCommand
	decoder        ansiDecoder
	ended          bool
	done           bool
	stop           bool
	disconnected   bool
	pause          bool
	permits        int
	stepping       bool
	captureClosing bool
	outputRevision uint64 // Includes ANSI-only output for literal-paste stability.
}
type execution struct {
	view         Run
	definition   Definition
	tasks        []*task
	secrets      []string
	paused       bool
	literalPaste bool
}

// Engine runs independently of browser focus and tab ownership. Captured output
// stays in memory until its finished run is cleared or evicted from the last 20 runs.
type Engine struct {
	mu      sync.Mutex
	ctx     context.Context
	cancel  context.CancelFunc
	backend backend
	runs    map[string]*execution
	order   []string
	active  map[string]*task
	closed  bool
	wg      sync.WaitGroup
}

func NewEngine(ctx context.Context, m *terminal.Manager) *Engine {
	return newEngine(ctx, managerBackend{m})
}
func newEngine(parent context.Context, b backend) *Engine {
	ctx, cancel := context.WithCancel(parent)
	return &Engine{ctx: ctx, cancel: cancel, backend: b, runs: map[string]*execution{}, active: map[string]*task{}}
}
func (e *Engine) Targets() []Target { return e.backend.targets() }

func validateRefs(refs []TargetRef) error {
	if len(refs) == 0 || len(refs) > 8 {
		return errors.New("select between one and eight connected terminals")
	}
	seen := map[string]bool{}
	for _, ref := range refs {
		if ref.ID == "" || ref.Generation == 0 || seen[ref.ID] {
			return errors.New("select distinct terminals with their current connection generation")
		}
		seen[ref.ID] = true
	}
	return nil
}
func runtimeValues(def Definition, provided map[string]string) (map[string]string, []string, error) {
	values := map[string]string{}
	known := map[string]bool{}
	secrets := []string{}
	for _, p := range def.Parameters {
		known[p.Name] = true
		value, ok := provided[p.Name]
		if !ok {
			value = p.DefaultValue
		}
		if len(value) > 8192 || !utf8.ValidString(value) || strings.ContainsAny(value, "\x00\r\n") {
			return nil, nil, fmt.Errorf("parameter %s must be single-line text of at most 8192 bytes", p.Name)
		}
		if p.Required && value == "" {
			return nil, nil, fmt.Errorf("enter a value for %s", p.Label)
		}
		values[p.Name] = value
		if p.Secret && value != "" {
			secrets = append(secrets, value)
		}
	}
	for name := range provided {
		if !known[name] {
			return nil, nil, fmt.Errorf("unknown macro parameter %s", name)
		}
	}
	// Replace longer secrets first so overlapping values cannot reveal suffixes.
	sort.Slice(secrets, func(i, j int) bool { return len(secrets[i]) > len(secrets[j]) })
	return values, secrets, nil
}
func (e *Engine) Start(req StartRequest) (Run, error) {
	def, err := Validate(req.Macro)
	if err != nil {
		return Run{}, err
	}
	return e.startPrepared(req, def, false)
}

// startPrepared shares lifecycle, generation fencing and cancellation between
// validated macro definitions and the separately validated literal paste path.
// literal is internal only; it cannot be enabled by a saved/API macro.
func (e *Engine) startPrepared(req StartRequest, def Definition, literal bool) (Run, error) {
	if err := validateRefs(req.Targets); err != nil {
		return Run{}, err
	}
	values, secrets, err := runtimeValues(def, req.Values)
	if err != nil {
		return Run{}, err
	}
	available := map[string]Target{}
	for _, target := range e.backend.targets() {
		available[target.ID] = target
	}
	idBytes := make([]byte, 12)
	if _, err = rand.Read(idBytes); err != nil {
		return Run{}, err
	}
	run := &execution{view: Run{ID: hex.EncodeToString(idBytes), MacroID: def.ID, Name: def.Name, State: "running", StartedAt: now()}, definition: def, secrets: secrets, literalPaste: literal}
	type preparedTask struct {
		task  *task
		steps []Step
	}
	prepared := []preparedTask{}
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.closed || e.ctx.Err() != nil {
		return Run{}, errors.New("macro engine is closed")
	}
	total := 0
	for _, step := range def.Steps {
		if !step.Disabled {
			total++
		}
	}
	for _, ref := range req.Targets {
		target, exists := available[ref.ID]
		if !exists {
			target = Target{ID: ref.ID, Generation: ref.Generation, Label: ref.ID}
		}
		ctx, cancel := context.WithCancel(e.ctx)
		t := &task{target: target, ctx: ctx, cancel: cancel, changed: make(chan struct{}), view: RunTarget{ID: ref.ID, Generation: ref.Generation, Label: target.Label, State: "running", Total: total}}
		run.tasks = append(run.tasks, t)
		fail := func(message string) { t.view.State = "failed"; t.view.Error = message; t.done = true; cancel() }
		if !exists || target.Generation != ref.Generation {
			fail(terminal.ErrTransportChanged.Error())
			continue
		}
		if literal && target.Type != "ssh" {
			fail("prompt-paced paste is available only for SSH terminals")
			continue
		}
		if previous := e.active[ref.ID]; previous != nil {
			if previous.target.Generation == target.Generation {
				fail("a macro is already running in this terminal")
				continue
			}
			previous.disconnected = true
			previous.cancel()
			e.notifyLocked(previous)
		}
		targetValues := map[string]string{}
		for k, v := range values {
			targetValues[k] = v
		}
		targetValues["host"], targetValues["username"], targetValues["session"] = target.Host, target.Username, target.Label
		// Built-ins are reserved case-insensitively by the shared model.
		steps := make([]Step, 0, total)
		for _, step := range def.Steps {
			if step.Disabled {
				continue
			}
			if literal {
				steps = append(steps, step)
				continue
			}
			for _, name := range DetectParams(step.Text) {
				switch strings.ToLower(name) {
				case "host":
					targetValues[name] = target.Host
				case "username":
					targetValues[name] = target.Username
				case "session":
					targetValues[name] = target.Label
				}
			}
			step.Text, err = Expand(step.Text, targetValues)
			if err != nil {
				break
			}
			if step.Kind == "wait" {
				if _, err = regexp.Compile(step.Text); err != nil {
					err = errors.New("expanded wait pattern is not a valid regular expression")
					break
				}
			}
			steps = append(steps, step)
		}
		if err != nil {
			fail(redact(err.Error(), secrets))
			err = nil
			continue
		}
		t.source, err = e.backend.subscribe(ref.ID, ref.Generation)
		if err != nil {
			fail(redact(err.Error(), secrets))
			err = nil
			continue
		}
		e.active[ref.ID] = t
		prepared = append(prepared, preparedTask{t, steps})
	}
	e.runs[run.view.ID] = run
	e.order = append(e.order, run.view.ID)
	e.recomputeLocked(run)
	e.pruneLocked()
	for _, ready := range prepared {
		e.wg.Add(1)
		go e.execute(run, ready.task, ready.steps)
	}
	return e.snapshotLocked(run), nil
}
func now() string { return time.Now().UTC().Format(time.RFC3339Nano) }
func redact(text string, secrets []string) string {
	for _, secret := range secrets {
		if secret != "" {
			text = strings.ReplaceAll(text, secret, "[redacted]")
		}
	}
	return text
}
func (e *Engine) snapshotLocked(run *execution) Run {
	view := run.view
	view.Name = redact(view.Name, run.secrets)
	view.Targets = make([]RunTarget, 0, len(run.tasks))
	for _, t := range run.tasks {
		v := t.view
		v.Label = redact(v.Label, run.secrets)
		v.Waiting = redact(v.Waiting, run.secrets)
		v.Error = redact(v.Error, run.secrets)
		view.Targets = append(view.Targets, v)
	}
	return view
}
func (e *Engine) recomputeLocked(run *execution) {
	active, completed, stopped, failed, paused := 0, 0, 0, 0, 0
	for _, t := range run.tasks {
		if !t.done {
			active++
			if run.paused || t.pause {
				paused++
			}
		} else {
			switch t.view.State {
			case "completed":
				completed++
			case "stopped":
				stopped++
			default:
				failed++
			}
		}
	}
	if active > 0 {
		if paused == active {
			run.view.State = "paused"
		} else {
			run.view.State = "running"
		}
		return
	}
	if run.view.FinishedAt == "" {
		run.view.FinishedAt = now()
	}
	switch {
	case completed == len(run.tasks):
		run.view.State = "completed"
	case completed > 0:
		run.view.State = "partial"
	case stopped > 0 && failed == 0:
		run.view.State = "stopped"
	default:
		run.view.State = "failed"
	}
}
func (e *Engine) pruneLocked() {
	finished := 0
	for _, id := range e.order {
		if e.runs[id].view.FinishedAt != "" {
			finished++
		}
	}
	if finished <= 20 {
		return
	}
	keep := e.order[:0]
	for _, id := range e.order {
		run := e.runs[id]
		if finished > 20 && run.view.FinishedAt != "" {
			delete(e.runs, id)
			finished--
			continue
		}
		keep = append(keep, id)
	}
	e.order = keep
}
func (e *Engine) List() []Run {
	e.mu.Lock()
	defer e.mu.Unlock()
	out := make([]Run, 0, len(e.order))
	for i := len(e.order) - 1; i >= 0; i-- {
		out = append(out, e.snapshotLocked(e.runs[e.order[i]]))
	}
	return out
}
func (e *Engine) Get(id string) (Run, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	run := e.runs[id]
	if run == nil {
		return Run{}, errors.New("macro run not found")
	}
	return e.snapshotLocked(run), nil
}

var ErrRunActive = errors.New("stop this macro before clearing its run output")

// Clear removes a finished run and its retained output. Active tasks keep their
// controls, subscriptions and terminal ownership until execution has stopped.
func (e *Engine) Clear(id string) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	run := e.runs[id]
	if run == nil {
		return errors.New("macro run not found")
	}
	if run.view.FinishedAt == "" {
		return ErrRunActive
	}
	e.clearLocked(map[string]bool{id: true})
	return nil
}

// ClearFinished receives only IDs authorized by the requesting window. Runs
// created after that snapshot and active runs are retained.
func (e *Engine) ClearFinished(ids []string) []string {
	e.mu.Lock()
	defer e.mu.Unlock()
	selected := make(map[string]bool, len(ids))
	for _, id := range ids {
		selected[id] = true
	}
	return e.clearLocked(selected)
}

func (e *Engine) clearLocked(selected map[string]bool) []string {
	cleared := []string{}
	keep := make([]string, 0, len(e.order))
	for _, id := range e.order {
		run := e.runs[id]
		if selected[id] && run.view.FinishedAt != "" {
			delete(e.runs, id)
			cleared = append(cleared, id)
			continue
		}
		keep = append(keep, id)
	}
	e.order = keep
	return cleared
}

func (e *Engine) notifyLocked(t *task) { close(t.changed); t.changed = make(chan struct{}) }
func (e *Engine) Control(id, action string) (Run, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	run := e.runs[id]
	if run == nil {
		return Run{}, errors.New("macro run not found")
	}
	switch action {
	case "pause", "resume", "step", "stop", "interrupt":
	default:
		return Run{}, errors.New("macro action must be pause, resume, step, stop, or interrupt")
	}
	if run.view.FinishedAt != "" {
		return e.snapshotLocked(run), nil
	}
	switch action {
	case "pause":
		run.paused = true
		for _, t := range run.tasks {
			t.stepping = false
			t.permits = 0
		}
	case "resume":
		run.paused = false
		for _, t := range run.tasks {
			t.pause = false
			t.permits = 0
		}
	case "step":
		run.paused = true
		for _, t := range run.tasks {
			if !t.done {
				t.pause = false
				t.permits++
			}
		}
	case "stop", "interrupt":
		for _, t := range run.tasks {
			if t.done {
				continue
			}
			t.stop = true
			t.cancel()
			if action == "interrupt" {
				_, _ = e.backend.write(t.target.ID, t.target.Generation, []byte{3})
			}
		}
	}
	for _, t := range run.tasks {
		if t.done {
			continue
		}
		if (run.paused || t.pause) && t.permits == 0 {
			t.view.State = "paused"
		} else {
			t.view.State = "running"
		}
		e.notifyLocked(t)
	}
	e.recomputeLocked(run)
	return e.snapshotLocked(run), nil
}

// OwnsPager lets browser and HTTP input paths suppress their automatic pager
// replies while the backend owns the target. A macro can then send exactly one
// continuation for each prompt, including before the UI has polled its run.
func (e *Engine) OwnsPager(sessionID string) bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.active[sessionID] != nil
}

// ManualInput cancels automation for this target before the user's input is
// queued. A human typing into a terminal must never race a pending macro step.
func (e *Engine) ManualInput(sessionID string, data []byte) {
	if len(data) == 0 || protocolReply(data) {
		return
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	if t := e.active[sessionID]; t != nil {
		t.stop = true
		t.cancel()
		e.notifyLocked(t)
	}
}
func (e *Engine) Send(req SendRequest) (SendResult, error) {
	if err := validateRefs(req.Targets); err != nil {
		return SendResult{}, err
	}
	if len(req.Text) > MaxStepTextBytes || !utf8.ValidString(req.Text) || strings.ContainsRune(req.Text, 0) {
		return SendResult{}, errors.New("send text must be valid text of at most 64 KiB without NUL")
	}
	if !req.Execute && strings.ContainsAny(req.Text, "\r\n") {
		return SendResult{}, errors.New("Insert accepts one line without Enter; use Send and run for multiple lines")
	}
	available := map[string]Target{}
	for _, target := range e.backend.targets() {
		available[target.ID] = target
	}
	result := SendResult{Targets: make([]SendTargetResult, 0, len(req.Targets))}
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.closed {
		return SendResult{}, errors.New("macro engine is closed")
	}
	for _, ref := range req.Targets {
		target, ok := available[ref.ID]
		out := SendTargetResult{ID: ref.ID, Label: target.Label}
		switch {
		case !ok || target.Generation != ref.Generation:
			out.Error = terminal.ErrTransportChanged.Error()
		case e.active[ref.ID] != nil:
			out.Error = "stop the running macro before sending input"
		default:
			text := normalizeLines(req.Text, lineEnding(target))
			if req.Execute {
				text = strings.TrimRight(text, "\r\n") + lineEnding(target)
			}
			_, err := e.backend.write(ref.ID, ref.Generation, []byte(text))
			if err != nil {
				out.Error = err.Error()
			} else {
				out.Sent = true
			}
		}
		result.Targets = append(result.Targets, out)
	}
	return result, nil
}
func lineEnding(target Target) string {
	switch target.LineEnding {
	case "lf":
		return "\n"
	case "crlf":
		return "\r\n"
	default:
		return "\r"
	}
}
func normalizeLines(text, ending string) string {
	return strings.ReplaceAll(strings.ReplaceAll(strings.ReplaceAll(text, "\r\n", "\n"), "\r", "\n"), "\n", ending)
}
// PasteCapture uses the original transport's sequence cursor at each accepted
// input write. Unlike browser polling, it cannot skip fast steps or label an
// unsent line as executed. It is memory-only and scoped by the HTTP caller.
func (e *Engine) PasteCapture(id, sessionID string) (PasteCapture, error) {
	if sessionID == "" {
		return PasteCapture{}, errors.New("choose a paste run session")
	}
	e.mu.Lock()
	run := e.runs[id]
	if run == nil || !run.literalPaste {
		e.mu.Unlock()
		return PasteCapture{}, errors.New("literal paste run not found")
	}
	var target *task
	for _, t := range run.tasks {
		if t.target.ID == sessionID {
			target = t
			break
		}
	}
	if target == nil {
		e.mu.Unlock()
		return PasteCapture{}, errors.New("paste run session not found")
	}
	commands := append([]sentPasteCommand(nil), target.sentPaste...)
	chunks := append([]capturedChunk(nil), target.chunks...)
	state := target.view.State
	secrets := append([]string(nil), run.secrets...)
	e.mu.Unlock()
	result := PasteCapture{RunID: id, SessionID: sessionID, State: state, Steps: make([]PasteCaptureStep, 0, len(commands))}
	for index, command := range commands {
		var output strings.Builder
		for _, chunk := range chunks {
			if chunk.seq <= command.cursor {
				continue
			}
			if index+1 < len(commands) && chunk.seq > commands[index+1].cursor {
				continue
			}
			output.WriteString(chunk.text)
		}
		result.Steps = append(result.Steps, PasteCaptureStep{Index: command.index, Command: redact(command.command, secrets), Output: maskSecrets(output.String(), secrets)})
	}
	return result, nil
}

func (e *Engine) Output(id, sessionID string) ([]byte, error) {
	// Copy immutable chunk references under the mutex, then assemble large
	// downloads without blocking input, output capture or run controls.
	type capture struct {
		view   RunTarget
		chunks []capturedChunk
	}
	e.mu.Lock()
	run := e.runs[id]
	if run == nil {
		e.mu.Unlock()
		return nil, errors.New("macro run not found")
	}
	captures := make([]capture, 0, len(run.tasks))
	for _, t := range run.tasks {
		captures = append(captures, capture{t.view, append([]capturedChunk(nil), t.chunks...)})
	}
	secrets := append([]string(nil), run.secrets...)
	e.mu.Unlock()
	var out bytes.Buffer
	found := sessionID == ""
	for _, t := range captures {
		if sessionID != "" && sessionID != t.view.ID {
			continue
		}
		found = true
		fmt.Fprintf(&out, "=== %s — %s ===\n", t.view.Label, t.view.State)

		// Redact the contiguous device stream before inserting headings: a
		// secret can straddle both read boundaries and a macro step boundary.
		var stream strings.Builder
		type heading struct {
			offset int
			text   string
		}
		headings := []heading{}
		for _, chunk := range t.chunks {
			if chunk.seq == 0 {
				headings = append(headings, heading{stream.Len(), chunk.text})
			} else {
				stream.WriteString(chunk.text)
			}
		}
		plain := stream.String()
		plain = maskSecrets(plain, secrets)
		position := 0
		for _, h := range headings {
			offset := h.offset
			for offset < len(plain) && !utf8.RuneStart(plain[offset]) {
				offset++
			}
			if offset < position {
				offset = position
			}
			out.WriteString(plain[position:offset])
			out.WriteString(h.text)
			position = offset
		}
		out.WriteString(plain[position:])
		if t.view.Error != "" {
			fmt.Fprintf(&out, "\n[Macro: %s]\n", t.view.Error)
		}
		out.WriteByte('\n')
	}
	if !found {
		return nil, errors.New("macro target not found")
	}
	return []byte(redact(out.String(), secrets)), nil
}

// Captures can be downloaded while a device is still echoing a secret. Hide a
// trailing partial secret too, rather than revealing it until the next chunk.
func maskSecrets(text string, secrets []string) string {
	for _, secret := range secrets {
		text = strings.ReplaceAll(text, secret, strings.Repeat("*", len(secret)))
		for length := min(len(secret)-1, len(text)); length > 0; length-- {
			if strings.HasSuffix(text, secret[:length]) {
				text = text[:len(text)-length] + strings.Repeat("*", length)
				break
			}
		}
	}
	return text
}

func (e *Engine) Close() {
	e.mu.Lock()
	if !e.closed {
		e.closed = true
		e.cancel()
		for _, t := range e.active {
			t.stop = true
			t.cancel()
			e.notifyLocked(t)
		}
	}
	e.mu.Unlock()
	e.wg.Wait()
}

func (e *Engine) execute(run *execution, t *task, steps []Step) {
	defer e.wg.Done()
	readerDone := make(chan struct{})
	go func() { defer close(readerDone); e.readOutput(t) }()
	var failure error
	cursor := uint64(0)
	for index, step := range steps {
		if failure = e.gate(run, t, true); failure != nil {
			break
		}
		e.mu.Lock()
		t.view.Step = index + 1
		t.view.State = "running"
		t.view.Waiting = ""
		t.chunks = append(t.chunks, capturedChunk{text: fmt.Sprintf("\n--- Step %d: %s ---\n", index+1, step.Kind)})
		e.mu.Unlock()
		timeout := step.TimeoutMs
		if timeout == 0 {
			timeout = run.definition.TimeoutMs
		}
		switch step.Kind {
		case "command", "text", "key":
			text := step.Text
			if step.Kind == "command" {
				text += lineEnding(t.target)
			} else if step.Kind == "text" {
				text = normalizeLines(text, lineEnding(t.target))
			} else {
				text, _ = KeyBytes(step.Key)
				if step.Key == "Enter" {
					text = lineEnding(t.target)
				}
			}
			cursor, failure = e.writeTask(run, t, []byte(text))
			if failure == nil && run.literalPaste && step.Kind == "command" {
				e.mu.Lock()
				t.sentPaste = append(t.sentPaste, sentPasteCommand{index: index + 1, command: step.Text, cursor: cursor})
				e.mu.Unlock()
			}
			if failure == nil && step.Kind == "command" && run.definition.WaitForPrompt {
				if run.literalPaste {
					failure = e.waitPastePrompt(run, t, cursor, time.Duration(timeout)*time.Millisecond)
				} else {
					failure = e.waitPattern(run, t, run.definition.Prompt, cursor, time.Duration(timeout)*time.Millisecond)
				}
			}
			// Literal paste folds its minimum pacing into the prompt-stability
			// window, so later output cannot invalidate a blindly delayed send.
			if failure == nil && !run.literalPaste && run.definition.PacingMs > 0 {
				failure = e.delay(run, t, time.Duration(run.definition.PacingMs)*time.Millisecond)
			}
		case "wait":
			failure = e.waitPattern(run, t, step.Text, cursor, time.Duration(timeout)*time.Millisecond)
		case "delay":
			failure = e.delay(run, t, time.Duration(step.DelayMs)*time.Millisecond)
		case "pause":
			e.mu.Lock()
			t.pause = true
			t.stepping = false
			t.view.State = "paused"
			t.view.Waiting = step.Text
			if t.view.Waiting == "" {
				t.view.Waiting = "Paused for review"
			}
			e.recomputeLocked(run)
			e.notifyLocked(t)
			e.mu.Unlock()
			failure = e.gate(run, t, false)
		}
		e.mu.Lock()
		t.stepping = false
		e.mu.Unlock()
		if failure != nil {
			break
		}
	}
	// With prompt waiting disabled, keep collecting until the device has been
	// quiet for a full second. Continuous output reaches an explicit timeout
	// rather than silently looking like a complete capture.
	if failure == nil && !run.definition.WaitForPrompt {
		failure = e.drainOutput(t, time.Duration(run.definition.TimeoutMs)*time.Millisecond)
	}
	e.mu.Lock()
	t.captureClosing = true
	e.mu.Unlock()
	t.source.Close()
	<-readerDone
	t.cancel()
	e.mu.Lock()
	defer e.mu.Unlock()
	t.done = true
	t.view.Waiting = ""
	switch {
	case t.stop || e.closed:
		t.view.State = "stopped"
	case t.disconnected:
		t.view.State = "disconnected"
		t.view.Error = "terminal disconnected or reconnected; this target was stopped"
	case failure != nil:
		t.view.State = "failed"
		t.view.Error = redact(failure.Error(), run.secrets)
	default:
		t.view.State = "completed"
	}
	if e.active[t.target.ID] == t {
		delete(e.active, t.target.ID)
	}
	e.recomputeLocked(run)
	e.pruneLocked()
	e.notifyLocked(t)
}
func (e *Engine) readOutput(t *task) {
	for {
		output, err := t.source.Read(t.ctx)
		e.mu.Lock()
		if len(output.Data) > 0 {
			t.outputRevision++
			plain := t.decoder.decode(output.Data)
			if plain != "" {
				t.chunks = append(t.chunks, capturedChunk{seq: output.Seq, text: plain})
				t.view.OutputBytes += len(plain)
			}
			e.notifyLocked(t)
		}
		if err != nil {
			t.ended = true
			if !errors.Is(err, context.Canceled) && !t.stop && !t.captureClosing {
				t.disconnected = true
				t.cancel()
			}
			e.notifyLocked(t)
			e.mu.Unlock()
			return
		}
		e.mu.Unlock()
	}
}
func (e *Engine) gate(run *execution, t *task, consume bool) error {
	for {
		e.mu.Lock()
		if err := t.ctx.Err(); err != nil {
			e.mu.Unlock()
			return err
		}
		if !run.paused && !t.pause || t.stepping || t.permits > 0 {
			if consume && t.permits > 0 {
				t.permits--
				t.stepping = true
			}
			t.view.State = "running"
			e.mu.Unlock()
			return nil
		}
		t.view.State = "paused"
		changed := t.changed
		e.recomputeLocked(run)
		e.mu.Unlock()
		select {
		case <-t.ctx.Done():
			return t.ctx.Err()
		case <-changed:
		}
	}
}

func (e *Engine) drainOutput(t *task, timeout time.Duration) error {
	quiet := time.Second
	if timeout <= quiet {
		quiet = timeout / 2
	}
	deadline := time.NewTimer(timeout)
	defer deadline.Stop()
	idle := time.NewTimer(quiet)
	defer idle.Stop()
	e.mu.Lock()
	seen := t.view.OutputBytes
	t.view.Waiting = "Capturing output until idle"
	e.mu.Unlock()
	for {
		e.mu.Lock()
		changed := t.changed
		ended := t.ended
		size := t.view.OutputBytes
		e.mu.Unlock()
		if ended {
			return terminal.ErrTransportChanged
		}
		if size != seen {
			seen = size
			if !idle.Stop() {
				select {
				case <-idle.C:
				default:
				}
			}
			idle.Reset(quiet)
		}
		select {
		case <-t.ctx.Done():
			return t.ctx.Err()
		case <-changed:
		case <-idle.C:
			e.mu.Lock()
			latest := t.view.OutputBytes
			e.mu.Unlock()
			if latest != seen {
				seen = latest
				idle.Reset(quiet)
				continue
			}
			return nil
		case <-deadline.C:
			return errors.New("output did not become idle before the macro timeout; use prompt waiting or add an explicit wait step for complete capture")
		}
	}
}

// Serialize the final cancellation check and enqueue with Control/ManualInput.
// A stop acknowledged to the caller cannot be followed by a new macro write.
func (e *Engine) writeTask(run *execution, t *task, data []byte) (uint64, error) {
	for {
		if err := e.gate(run, t, false); err != nil {
			return 0, err
		}
		e.mu.Lock()
		if err := t.ctx.Err(); err != nil {
			e.mu.Unlock()
			return 0, err
		}
		if (run.paused || t.pause) && !t.stepping && t.permits == 0 {
			e.mu.Unlock()
			continue
		}
		cursor, err := e.backend.write(t.target.ID, t.target.Generation, data)
		e.mu.Unlock()
		return cursor, err
	}
}

func (e *Engine) delay(run *execution, t *task, duration time.Duration) error {
	remaining := duration
	for remaining > 0 {
		if err := e.gate(run, t, false); err != nil {
			return err
		}
		e.mu.Lock()
		changed := t.changed
		e.mu.Unlock()
		start := time.Now()
		timer := time.NewTimer(remaining)
		select {
		case <-t.ctx.Done():
			timer.Stop()
			return t.ctx.Err()
		case <-changed:
			timer.Stop()
			remaining -= time.Since(start)
		case <-timer.C:
			return nil
		}
	}
	return nil
}

var pagerPattern = regexp.MustCompile(`(?i)(?:--[ \t]*more[ \t]*--|---\([ \t]*more[ \t]*\)---|press[ \t]+(?:any[ \t]+key|space|<space>)[^\r\n]{0,80}(?:continue|more)|more:[ \t]*<space>)`)

func (e *Engine) waitPattern(run *execution, t *task, pattern string, cursor uint64, timeout time.Duration) error {
	re, err := regexp.Compile(pattern)
	if err != nil {
		return errors.New("wait pattern is not a valid regular expression")
	}
	remaining := timeout
	index := 0
	tail := ""
	lastPagerEnd := 0
	e.mu.Lock()
	t.view.Waiting = "Waiting for output"
	e.mu.Unlock()
	for {
		if err = e.gate(run, t, false); err != nil {
			return err
		}
		e.mu.Lock()
		// Copy chunk references only. Regex matching never blocks incoming
		// capture data, and the rolling matcher stays bounded for huge output.
		fresh := append([]capturedChunk(nil), t.chunks[index:]...)
		index = len(t.chunks)
		changed, ended := t.changed, t.ended
		e.mu.Unlock()
		matched := false
		for _, chunk := range fresh {
			if chunk.seq <= cursor {
				continue
			}
			tail += chunk.text
			if matchOutput(re, pattern, tail) {
				matched = true
			}
			if len(tail) > 128*1024 {
				trim := len(tail) - 64*1024
				tail = tail[trim:]
				lastPagerEnd = max(0, lastPagerEnd-trim)
			}
		}
		if matched {
			e.mu.Lock()
			t.view.Waiting = ""
			e.mu.Unlock()
			return nil
		}

		if ended {
			return terminal.ErrTransportChanged
		}
		if run.definition.AutoPager {
			matches := pagerPattern.FindAllStringIndex(tail, -1)
			for _, match := range matches {
				if match[1] <= lastPagerEnd {
					continue
				}
				if err = e.gate(run, t, false); err != nil {
					return err
				}
				if _, err = e.writeTask(run, t, []byte(" ")); err != nil {
					return err
				}
				lastPagerEnd = match[1]
			}
		}
		if remaining <= 0 {
			return fmt.Errorf("timed out after %s waiting for output", timeout)
		}
		start := time.Now()
		timer := time.NewTimer(remaining)
		select {
		case <-t.ctx.Done():
			timer.Stop()
			return t.ctx.Err()
		case <-changed:
			timer.Stop()
			remaining -= time.Since(start)
		case <-timer.C:
			return fmt.Errorf("timed out after %s waiting for output", timeout)
		}
	}
}

// The default prompt only inspects the final short line. Avoid rescanning a
// large pager response for every read when a device emits one very long line.
func matchOutput(re *regexp.Regexp, pattern, text string) bool {
	if pattern == DefaultPrompt {
		line := text[strings.LastIndexAny(text, "\r\n")+1:]
		if len(line) > 164 {
			return false
		}
		return re.MatchString(line)
	}
	return re.MatchString(text)
}

// ansiDecoder strips streaming CSI/OSC/DCS sequences without losing text when
// an escape sequence straddles read boundaries. It never changes terminal UI.
type ansiDecoder struct{ state uint8 }

func (d *ansiDecoder) decode(data []byte) string {
	var out strings.Builder
	for _, c := range data {
		switch d.state {
		case 0:
			switch {
			case c == 0x1b:
				d.state = 1
			case c == '\n' || c == '\r' || c == '\t' || c >= 0x20 && c != 0x7f:
				out.WriteByte(c)
			}
		case 1:
			switch c {
			case '[':
				d.state = 2
			case ']', 'P', '^', '_':
				d.state = 3
			default:
				if c >= 0x20 && c <= 0x2f {
					d.state = 5
				} else {
					d.state = 0
				}
			}
		case 2:
			if c >= 0x40 && c <= 0x7e {
				d.state = 0
			}
		case 3:
			if c == 7 {
				d.state = 0
			} else if c == 0x1b {
				d.state = 4
			}
		case 4:
			if c == '\\' {
				d.state = 0
			} else if c != 0x1b {
				d.state = 3
			}
		case 5:
			if c >= 0x30 && c <= 0x7e {
				d.state = 0
			}
		}
	}
	return out.String()
}

// Terminal-generated status/device replies are not a human taking control.
// Match entire reply packets, never an escape-prefixed arrow or function key.
var terminalProtocolReply = regexp.MustCompile(`^(?:\x1b\[(?:\??[0-9;]*[cnR]|>[0-9;]*c|[0-9;]*t|[IO])|\x1bP[\s\S]*\x1b\\|\x1b\][^\x07\x1b]*(?:\x07|\x1b\\))+$`)

func protocolReply(data []byte) bool { return terminalProtocolReply.Match(data) }
