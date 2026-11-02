package macros

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"networktoolkit/internal/terminal"
)

// PasteRequest is a one-time literal command list, never a saved macro or
// template. Every line, including blanks, must already have an Enter boundary.
type PasteRequest struct {
	Target TargetRef `json:"target"`
	Text   string    `json:"text"`
	// AutoPager mirrors the session's Auto pager setting. While the engine
	// owns the target the browser's own pager replies are suppressed, so a
	// paged "show" command would otherwise stall at --More-- until timeout.
	AutoPager bool   `json:"autoPager"`
	PagerKey  string `json:"pagerKey"` // auto, space or enter
}

func pasteDefinition(text string) (Definition, error) {
	if len(text) == 0 || len(text) > 64<<10 || !utf8.ValidString(text) {
		return Definition{}, errors.New("paced paste requires valid UTF-8 text of 1 to 65536 bytes")
	}
	for _, char := range text {
		if unicode.IsControl(char) && char != '\r' && char != '\n' {
			return Definition{}, errors.New("paced paste does not accept terminal control characters; review and use raw paste instead")
		}
	}
	text = strings.ReplaceAll(strings.ReplaceAll(text, "\r\n", "\n"), "\r", "\n")
	if !strings.HasSuffix(text, "\n") {
		return Definition{}, errors.New("paced paste requires a final line ending; no Enter is added to unfinished input")
	}
	lines := strings.Split(strings.TrimSuffix(text, "\n"), "\n")
	if len(lines) > MaxSteps {
		return Definition{}, errors.New("paced paste supports at most 1024 lines")
	}
	def := Definition{ID: "literal-paste", Name: "Prompt-paced paste", WaitForPrompt: true, Prompt: DefaultPrompt, TimeoutMs: DefaultTimeoutMs, PacingMs: DefaultPacingMs}
	for index, line := range lines {
		// Unlike FromCommands, blank lines remain literal Enter presses. The
		// dedicated validator above allows them without relaxing macro rules.
		def.Steps = append(def.Steps, Step{ID: fmt.Sprintf("line-%d", index+1), Kind: "command", Text: line})
	}
	return def, nil
}

func (e *Engine) StartPaste(req PasteRequest) (Run, error) {
	def, err := pasteDefinition(req.Text)
	if err != nil {
		return Run{}, err
	}
	def.AutoPager = req.AutoPager
	key := strings.ToLower(strings.TrimSpace(req.PagerKey))
	if key != "space" && key != "enter" {
		key = "auto"
	}
	return e.startPrepared(StartRequest{Targets: []TargetRef{req.Target}}, def, &pasteOptions{pagerKey: key})
}

type pasteOptions struct {
	pagerKey string
}

// Paced paste also recognizes prompts that ask for Enter or Return, which the
// console's own pager handles; ordinary macros keep the narrower pagerPattern.
var pastePagerPattern = regexp.MustCompile(pagerPattern.String() + `|(?i)press[ \t]+(?:the[ \t]+)?(?:enter|return)(?:[ \t]+key)?[^\r\n]{0,40}(?:more|continue|next)`)

// pagerReply chooses the continuation for one recognized pager prompt. "auto"
// presses Enter only when the prompt itself asks for Enter or Return.
func pagerReply(key, prompt string) []byte {
	switch key {
	case "enter":
		return []byte("\r")
	case "space":
		return []byte(" ")
	}
	lower := strings.ToLower(prompt)
	if strings.Contains(lower, "enter") || strings.Contains(lower, "return") {
		return []byte("\r")
	}
	return []byte(" ")
}

const pastePromptStable = 100 * time.Millisecond

// Prompt recognition remains a CLI heuristic, not proof that a device accepted
// a command. For literal paste, do not let an echoed '$' or an intermediate
// prompt-shaped packet authorize the next line: require a fresh line boundary,
// match only the assembled final tail, then recheck after quiet output stability.
// Ordinary macros deliberately retain their existing waitPattern contract.
func (e *Engine) waitPastePrompt(run *execution, t *task, cursor uint64, timeout time.Duration) error {
	re := regexp.MustCompile(DefaultPrompt)
	remaining := timeout
	stability := max(pastePromptStable, time.Duration(run.definition.PacingMs)*time.Millisecond)
	index := 0
	tail := ""
	boundary := false
	lastPagerEnd := 0
	var candidateAt time.Time
	var candidateRevision uint64
	e.mu.Lock()
	t.view.Waiting = "Waiting for a stable fresh prompt"
	e.mu.Unlock()
	for {
		if err := e.gate(run, t, false); err != nil {
			return err
		}
		e.mu.Lock()
		fresh := append([]capturedChunk(nil), t.chunks[index:]...)
		index = len(t.chunks)
		changed, ended, revision := t.changed, t.ended, t.outputRevision
		e.mu.Unlock()
		for _, chunk := range fresh {
			if chunk.seq <= cursor {
				continue
			}
			boundary = boundary || strings.ContainsAny(chunk.text, "\r\n")
			tail += chunk.text
			if len(tail) > 128*1024 {
				trim := len(tail) - 64*1024
				tail = tail[trim:]
				lastPagerEnd = max(0, lastPagerEnd-trim)
			}
		}
		if ended {
			return terminal.ErrTransportChanged
		}
		// A pager prompt is never a command prompt. With Auto pager on, send
		// exactly one continuation per recognized prompt and keep waiting for
		// the real prompt; with it off, the prompt waits for the user (or the
		// 30-second timeout), exactly as before.
		if run.definition.AutoPager && run.paste != nil {
			for _, match := range pastePagerPattern.FindAllStringIndex(tail, -1) {
				if match[1] <= lastPagerEnd {
					continue
				}
				if err := e.gate(run, t, false); err != nil {
					return err
				}
				if _, err := e.writeTask(run, t, pagerReply(run.paste.pagerKey, tail[match[0]:match[1]])); err != nil {
					return err
				}
				lastPagerEnd = match[1]
				candidateAt = time.Time{}
			}
		}
		if !boundary || !matchOutput(re, DefaultPrompt, tail) {
			candidateAt = time.Time{}
		} else if candidateAt.IsZero() || candidateRevision != revision {
			candidateAt, candidateRevision = time.Now(), revision
		} else if time.Since(candidateAt) >= stability {
			// The source reader may have appended bytes while the matcher was
			// outside the mutex. Recheck before accepting the stable snapshot.
			e.mu.Lock()
			stable := t.outputRevision == revision && !t.ended && t.ctx.Err() == nil
			if stable {
				t.view.Waiting = ""
			}
			e.mu.Unlock()
			if stable {
				return nil
			}
			continue
		}
		if remaining <= 0 {
			return fmt.Errorf("timed out after %s waiting for a stable fresh prompt", timeout)
		}
		wait := remaining
		if !candidateAt.IsZero() {
			wait = min(wait, max(time.Nanosecond, stability-time.Since(candidateAt)))
		}
		start := time.Now()
		timer := time.NewTimer(wait)
		select {
		case <-t.ctx.Done():
			timer.Stop()
			return t.ctx.Err()
		case <-changed:
			timer.Stop()
		case <-timer.C:
		}
		remaining -= time.Since(start)
	}
}
