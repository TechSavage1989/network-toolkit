package main

import (
	"regexp"
	"strings"
)

// Terminal syntax highlighting rule sets (owner request, v1.21.6: MobaXterm-
// style syntax colors). Only the active choice and the user's own rule sets
// are saved; built-in rule sets live in web/assets/terminal-syntax-rules.js.
// Patterns are display preferences, never executed by the engine.

var (
	terminalSyntaxIDPattern    = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,39}$`)
	terminalSyntaxBuiltIn      = map[string]bool{"standard": true, "shell": true, "network": true, "perl": true, "sql": true}
	terminalSyntaxColors       = []string{"red", "green", "yellow", "blue", "magenta", "cyan", "underline"}
	terminalSyntaxControlChars = regexp.MustCompile(`[\x00-\x08\x0b-\x1f\x7f]`)
	terminalSyntaxLineBreaks   = regexp.MustCompile(`[\r\n]+`)
)

const (
	terminalSyntaxMaxCustom  = 50
	terminalSyntaxMaxPattern = 8000
	terminalSyntaxMaxName    = 80
	terminalSyntaxMaxLabel   = 40
)

func terminalSyntaxText(value any, limit int) string {
	text, _ := value.(string)
	text = terminalSyntaxLineBreaks.ReplaceAllString(text, " ")
	text = terminalSyntaxControlChars.ReplaceAllString(text, "")
	if runes := []rune(text); len(runes) > limit {
		text = string(runes[:limit])
	}
	return text
}

func safeTerminalSyntax(value any) map[string]any {
	source, _ := value.(map[string]any)
	custom := []any{}
	seen := map[string]bool{}
	if list, ok := source["custom"].([]any); ok {
		for _, raw := range list {
			if len(custom) >= terminalSyntaxMaxCustom {
				break
			}
			item, ok := raw.(map[string]any)
			if !ok {
				continue
			}
			id, _ := item["id"].(string)
			if !terminalSyntaxIDPattern.MatchString(id) || terminalSyntaxBuiltIn[id] || seen[id] {
				continue
			}
			seen[id] = true
			name := strings.TrimSpace(terminalSyntaxText(item["name"], terminalSyntaxMaxName))
			if name == "" {
				name = "Custom syntax"
			}
			requested, _ := item["rules"].(map[string]any)
			rules := map[string]any{}
			for _, color := range terminalSyntaxColors {
				rules[color] = terminalSyntaxText(requested[color], terminalSyntaxMaxPattern)
			}
			// What each color marks (v1.21.15), named by the user.
			requestedLabels, _ := item["labels"].(map[string]any)
			labels := map[string]any{}
			for _, color := range terminalSyntaxColors {
				if label := strings.TrimSpace(terminalSyntaxText(requestedLabels[color], terminalSyntaxMaxLabel)); label != "" {
					labels[color] = label
				}
			}
			custom = append(custom, map[string]any{"id": id, "name": name, "regex": item["regex"] != false, "caseSensitive": item["caseSensitive"] == true, "rules": rules, "labels": labels})
		}
	}
	active, _ := source["active"].(string)
	if active != "smart" && !terminalSyntaxBuiltIn[active] && !seen[active] {
		active = "smart"
	}
	// Per-profile color overrides (v1.21.11) for built-in and kept custom rule sets.
	colors := map[string]any{}
	if requested, ok := source["colors"].(map[string]any); ok {
		for id, raw := range requested {
			slots, ok := raw.(map[string]any)
			if !ok || !(terminalSyntaxBuiltIn[id] || seen[id]) {
				continue
			}
			clean := map[string]any{}
			for _, color := range terminalSyntaxColors {
				if value, ok := slots[color].(string); ok && terminalColorPattern.MatchString(value) && len(value) == 7 {
					clean[color] = strings.ToLower(value)
				}
			}
			if len(clean) > 0 {
				colors[id] = clean
			}
		}
	}
	// Colors switched off per rule set (v1.21.14): {id: ["red", ...]}.
	off := map[string]any{}
	if requested, ok := source["off"].(map[string]any); ok {
		for id, raw := range requested {
			slots, ok := raw.([]any)
			if !ok || !(terminalSyntaxBuiltIn[id] || seen[id]) {
				continue
			}
			clean := []any{}
			for _, color := range append(append([]string{}, terminalSyntaxColors...), "prompt") {
				for _, slot := range slots {
					if slot == color {
						clean = append(clean, color)
						break
					}
				}
			}
			if len(clean) > 0 {
				off[id] = clean
			}
		}
	}
	return map[string]any{"active": active, "custom": custom, "colors": colors, "off": off}
}
