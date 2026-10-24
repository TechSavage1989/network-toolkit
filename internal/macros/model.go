// Package macros describes terminal input sequences. Values are sent only to
// already connected remote terminals; no step executes a local program.
package macros

import (
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"
)

const (
	MaxDefinitionBytes = 4 << 20
	MaxStepTextBytes   = 64 << 10
	MaxSteps           = 1024
	MaxParameters      = 128
	DefaultTimeoutMs   = 30000
	DefaultPacingMs    = 100
	MaxTimeoutMs       = 3600000
	MaxPacingMs        = 60000
	DefaultPrompt      = `(?:^|[\r\n])[^\r\n]{0,160}[#>$%][ \t]*$`
)

type Definition struct {
	ID            string      `json:"id"`
	Name          string      `json:"name"`
	Description   string      `json:"description,omitempty"`
	Folder        string      `json:"folder,omitempty"`
	Tags          []string    `json:"tags"`
	Favorite      bool        `json:"favorite"`
	Shortcut      string      `json:"shortcut,omitempty"`
	Parameters    []Parameter `json:"parameters"`
	Steps         []Step      `json:"steps"`
	WaitForPrompt bool        `json:"waitForPrompt"`
	Prompt        string      `json:"prompt"`
	TimeoutMs     int         `json:"timeoutMs"`
	PacingMs      int         `json:"pacingMs"`
	AutoPager     bool        `json:"autoPager"`
}

type Parameter struct {
	Name         string `json:"name"`
	Label        string `json:"label"`
	DefaultValue string `json:"defaultValue,omitempty"`
	Secret       bool   `json:"secret"`
	Required     bool   `json:"required"`
}

type Step struct {
	ID        string `json:"id"`
	Kind      string `json:"kind"`
	Text      string `json:"text,omitempty"`
	Key       string `json:"key,omitempty"`
	TimeoutMs int    `json:"timeoutMs"`
	DelayMs   int    `json:"delayMs"`
	Disabled  bool   `json:"disabled"`
}

// JSON defaults preserve explicitly selected false/zero values.
func (definition *Definition) UnmarshalJSON(data []byte) error {
	type plain Definition
	value := plain{WaitForPrompt: true, Prompt: DefaultPrompt, TimeoutMs: DefaultTimeoutMs, PacingMs: DefaultPacingMs}
	if err := json.Unmarshal(data, &value); err != nil {
		return err
	}
	*definition = Definition(value)
	return nil
}

func (parameter *Parameter) UnmarshalJSON(data []byte) error {
	type plain Parameter
	value := plain{Required: true}
	if err := json.Unmarshal(data, &value); err != nil {
		return err
	}
	*parameter = Parameter(value)
	return nil
}

var parameterName = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_]{0,63}$`)
var identifier = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:-]{0,79}$`)
var placeholder = regexp.MustCompile(`\{\{([A-Za-z][A-Za-z0-9_]{0,63})\}\}`)
var shortcutKey = regexp.MustCompile(`^(?:[A-Z0-9]|F(?:[1-9]|1[0-9]|2[0-4])|Space|Enter|Tab|Escape|Up|Down|Left|Right)$`)

func metadata(value, label string, limit int, multiline bool) (string, error) {
	value = strings.TrimSpace(value)
	if !utf8.ValidString(value) || utf8.RuneCountInString(value) > limit || strings.IndexFunc(value, func(r rune) bool {
		return unicode.IsControl(r) && !(multiline && (r == '\n' || r == '\r' || r == '\t'))
	}) >= 0 {
		return "", fmt.Errorf("%s must contain at most %d characters without control characters", label, limit)
	}
	return value, nil
}

func NormalizeShortcut(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", nil
	}
	parts := strings.Split(value, "+")
	modifiers := map[string]bool{}
	for _, part := range parts[:len(parts)-1] {
		switch strings.ToLower(strings.TrimSpace(part)) {
		case "ctrl", "control":
			modifiers["Ctrl"] = true
		case "alt":
			modifiers["Alt"] = true
		case "shift":
			modifiers["Shift"] = true
		case "meta", "cmd", "command":
			modifiers["Meta"] = true
		default:
			return "", errors.New("choose a shortcut such as Ctrl+Shift+1 or Alt+F6")
		}
	}
	key := strings.TrimSpace(parts[len(parts)-1])
	if len(key) == 4 && strings.EqualFold(key[:3], "Key") {
		key = key[3:]
	}
	if len(key) == 1 || strings.HasPrefix(strings.ToLower(key), "f") {
		key = strings.ToUpper(key)
	} else {
		for _, candidate := range []string{"Space", "Enter", "Tab", "Escape", "Up", "Down", "Left", "Right"} {
			if strings.EqualFold(key, candidate) {
				key = candidate
			}
		}
	}
	if !shortcutKey.MatchString(key) || len(modifiers) == 0 && !strings.HasPrefix(key, "F") {
		return "", errors.New("choose a shortcut with a modifier or a function key")
	}
	result := []string{}
	for _, name := range []string{"Ctrl", "Alt", "Shift", "Meta"} {
		if modifiers[name] {
			result = append(result, name)
		}
	}
	return strings.Join(append(result, key), "+"), nil
}

// Validate returns a detached canonical value. Unknown JSON fields cannot be
// retained in this typed schema. No runtime parameter values belong here.
func Validate(definition Definition) (Definition, error) {
	clean := definition
	var err error
	clean.ID = strings.TrimSpace(clean.ID)
	if !identifier.MatchString(clean.ID) {
		return Definition{}, errors.New("choose a macro ID of 1 to 80 letters, digits or ._:-")
	}
	if clean.Name, err = metadata(clean.Name, "Macro name", 100, false); err != nil || clean.Name == "" {
		return Definition{}, errors.New("enter a macro name of 1 to 100 characters")
	}
	if clean.Description, err = metadata(clean.Description, "Description", 2000, true); err != nil {
		return Definition{}, err
	}
	if clean.Folder, err = metadata(clean.Folder, "Folder", 120, false); err != nil {
		return Definition{}, err
	}
	if clean.Shortcut, err = NormalizeShortcut(clean.Shortcut); err != nil {
		return Definition{}, err
	}
	if clean.TimeoutMs == 0 {
		clean.TimeoutMs = DefaultTimeoutMs
	}
	if clean.TimeoutMs < 100 || clean.TimeoutMs > MaxTimeoutMs || clean.PacingMs < 0 || clean.PacingMs > MaxPacingMs {
		return Definition{}, errors.New("macro timeout must be 100 ms to 1 hour and pacing must be 0 to 60 seconds")
	}
	clean.Prompt = strings.TrimSpace(clean.Prompt)
	if clean.Prompt == "" {
		clean.Prompt = DefaultPrompt
	}
	if len(clean.Prompt) > 4096 {
		return Definition{}, errors.New("prompt pattern exceeds 4096 bytes")
	}
	if _, err := regexp.Compile(clean.Prompt); err != nil {
		return Definition{}, errors.New("the prompt pattern is not a valid RE2 regular expression")
	}
	clean.Tags = []string{}
	seenTags := map[string]bool{}
	if len(definition.Tags) > 32 {
		return Definition{}, errors.New("a macro can have at most 32 tags")
	}
	for _, tag := range definition.Tags {
		value, err := metadata(tag, "Tag", 64, false)
		if err != nil {
			return Definition{}, err
		}
		if value != "" && !seenTags[value] {
			clean.Tags = append(clean.Tags, value)
			seenTags[value] = true
		}
	}
	if len(definition.Parameters) > MaxParameters {
		return Definition{}, errors.New("a macro can have at most 128 parameters")
	}
	clean.Parameters = []Parameter{}
	seenParameters := map[string]bool{}
	for _, parameter := range definition.Parameters {
		parameter.Name = strings.TrimSpace(parameter.Name)
		if !parameterName.MatchString(parameter.Name) || seenParameters[parameter.Name] || isBuiltin(parameter.Name) {
			return Definition{}, errors.New("use unique parameter names beginning with a letter; host, username and session are built in")
		}
		if parameter.Label, err = metadata(parameter.Label, "Parameter label", 100, false); err != nil {
			return Definition{}, err
		}
		if parameter.Label == "" {
			parameter.Label = parameter.Name
		}
		if parameter.Secret {
			parameter.DefaultValue = ""
		}
		if len(parameter.DefaultValue) > 8192 || strings.ContainsAny(parameter.DefaultValue, "\x00\r\n") || !utf8.ValidString(parameter.DefaultValue) {
			return Definition{}, errors.New("parameter defaults must be single-line text of at most 8192 bytes")
		}
		seenParameters[parameter.Name] = true
		clean.Parameters = append(clean.Parameters, parameter)
	}
	if len(definition.Steps) == 0 || len(definition.Steps) > MaxSteps {
		return Definition{}, errors.New("a macro must have 1 to 1024 steps")
	}
	clean.Steps = []Step{}
	seenSteps := map[string]bool{}
	active := 0
	for index, step := range definition.Steps {
		step.ID = strings.TrimSpace(step.ID)
		if step.ID == "" {
			step.ID = fmt.Sprintf("step-%d", index+1)
		}
		if !identifier.MatchString(step.ID) || seenSteps[step.ID] {
			return Definition{}, errors.New("each macro step needs a unique ID of at most 80 characters")
		}
		seenSteps[step.ID] = true
		step.Kind = strings.TrimSpace(step.Kind)
		if step.Kind == "" {
			step.Kind = "command"
		}
		if !utf8.ValidString(step.Text) || len(step.Text) > MaxStepTextBytes || strings.IndexByte(step.Text, 0) >= 0 {
			return Definition{}, fmt.Errorf("step %d text must be valid text of at most 64 KiB without NUL", index+1)
		}
		if step.TimeoutMs < 0 || step.TimeoutMs > MaxTimeoutMs || step.TimeoutMs > 0 && step.TimeoutMs < 100 || step.DelayMs < 0 || step.DelayMs > MaxTimeoutMs {
			return Definition{}, fmt.Errorf("step %d has an invalid timeout or delay", index+1)
		}
		switch step.Kind {
		case "command", "text":
			step.Key, step.DelayMs = "", 0
			if !step.Disabled && step.Text == "" {
				return Definition{}, fmt.Errorf("enter text for step %d", index+1)
			}
			if step.Kind == "command" && strings.ContainsAny(step.Text, "\r\n") {
				return Definition{}, fmt.Errorf("step %d command must be one line; use separate commands or a Send text step", index+1)
			}
		case "key":
			step.Text, step.DelayMs, step.TimeoutMs = "", 0, 0
			if _, ok := KeyBytes(step.Key); !ok {
				return Definition{}, fmt.Errorf("choose a supported key for step %d", index+1)
			}
		case "wait":
			step.Key, step.DelayMs = "", 0
			if !step.Disabled {
				if step.Text == "" {
					return Definition{}, fmt.Errorf("enter an output pattern for step %d", index+1)
				}
				if _, err := regexp.Compile(step.Text); err != nil {
					return Definition{}, fmt.Errorf("step %d output pattern is not a valid RE2 regular expression", index+1)
				}
			}
		case "delay":
			step.Text, step.Key, step.TimeoutMs = "", "", 0
		case "pause":
			step.Key, step.DelayMs, step.TimeoutMs = "", 0, 0
		default:
			return Definition{}, fmt.Errorf("step %d has an unknown action", index+1)
		}
		if step.Kind == "command" || step.Kind == "text" || step.Kind == "wait" || step.Kind == "pause" {
			if err := validateTemplate(step.Text); err != nil {
				return Definition{}, fmt.Errorf("step %d: %w", index+1, err)
			}
			for _, name := range DetectParams(step.Text) {
				if !isBuiltin(name) && !seenParameters[name] {
					return Definition{}, fmt.Errorf("define the parameter %s used in step %d", name, index+1)
				}
			}
		}
		if !step.Disabled {
			active++
		}
		clean.Steps = append(clean.Steps, step)
	}
	if active == 0 {
		return Definition{}, errors.New("enable at least one macro step")
	}
	encoded, err := json.Marshal(clean)
	if err != nil || len(encoded) > MaxDefinitionBytes {
		return Definition{}, errors.New("macro definition exceeds the 4 MiB limit")
	}
	return clean, nil
}

func isBuiltin(name string) bool {
	return strings.EqualFold(name, "host") || strings.EqualFold(name, "username") || strings.EqualFold(name, "session")
}

func validateTemplate(text string) error {
	rest := placeholder.ReplaceAllString(text, "")
	if strings.Contains(rest, "{{") || strings.Contains(rest, "}}") {
		return errors.New("use parameter placeholders in the form {{name}}")
	}
	return nil
}

func DetectParams(text string) []string {
	result := []string{}
	seen := map[string]bool{}
	for _, match := range placeholder.FindAllStringSubmatch(text, -1) {
		if !seen[match[1]] {
			result = append(result, match[1])
			seen[match[1]] = true
		}
	}
	return result
}

// Expand substitutes literal parameter text once, without executing code or
// interpreting placeholders inside a supplied value. Values cannot add lines.
func Expand(text string, values map[string]string) (string, error) {
	if len(text) > MaxStepTextBytes || !utf8.ValidString(text) {
		return "", errors.New("macro step text exceeds 64 KiB or is invalid UTF-8")
	}
	if err := validateTemplate(text); err != nil {
		return "", err
	}
	var failure error
	result := placeholder.ReplaceAllStringFunc(text, func(token string) string {
		name := token[2 : len(token)-2]
		value, ok := values[name]
		if !ok && isBuiltin(name) {
			value, ok = values[strings.ToLower(name)]
		}
		if !ok {
			failure = fmt.Errorf("enter a value for %s", name)
			return ""
		}
		if strings.ContainsAny(value, "\x00\r\n") || !utf8.ValidString(value) {
			failure = fmt.Errorf("parameter %s must be a single line of valid text", name)
			return ""
		}
		return value
	})
	if failure != nil {
		return "", failure
	}
	if len(result) > MaxStepTextBytes {
		return "", errors.New("expanded macro step exceeds 64 KiB")
	}
	return result, nil
}

func FromCommands(text string) []Step {
	steps := []Step{}
	text = strings.ReplaceAll(strings.ReplaceAll(text, "\r\n", "\n"), "\r", "\n")
	for _, line := range strings.Split(text, "\n") {
		if strings.TrimSpace(line) != "" {
			steps = append(steps, Step{ID: fmt.Sprintf("step-%d", len(steps)+1), Kind: "command", Text: line})
		}
	}
	return steps
}

func KeyBytes(name string) (string, bool) {
	keys := map[string]string{"Enter": "\r", "Tab": "\t", "Escape": "\x1b", "Ctrl+C": "\x03", "Ctrl+Z": "\x1a", "Ctrl+D": "\x04", "Up": "\x1b[A", "Down": "\x1b[B", "Left": "\x1b[D", "Right": "\x1b[C", "Backspace": "\x7f"}
	value, ok := keys[name]
	return value, ok
}
