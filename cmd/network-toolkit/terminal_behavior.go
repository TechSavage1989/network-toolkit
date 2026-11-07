package main

import (
	"strings"
	"unicode"
)

// Terminal features and Advanced settings (v1.21.8), matching
// web/assets/terminal-behavior.js normalize(): unknown or invalid values
// fall back to the defaults, which keep the terminal as it was.
const terminalDefaultWordSeparators = " ()[]{}',\"`"

func safeTerminalBehavior(value any) map[string]any {
	source, _ := value.(map[string]any)
	clean := map[string]any{"scrollback": 10000, "scrollbar": true, "bell": "none", "bracketedPaste": true, "wordSeparators": terminalDefaultWordSeparators, "lfImpliesCr": false, "crImpliesLf": false, "answerback": "", "reconnectKeys": true, "closeOnOtherKey": false}
	if lines, ok := source["scrollback"].(float64); ok && lines == float64(int(lines)) && lines >= 500 && lines <= 10000 {
		clean["scrollback"] = int(lines)
	}
	if source["scrollbar"] == false {
		clean["scrollbar"] = false
	}
	switch bell, _ := source["bell"].(string); bell {
	case "none", "visual", "sound":
		clean["bell"] = bell
	}
	if source["bracketedPaste"] == false {
		clean["bracketedPaste"] = false
	}
	clean["lfImpliesCr"] = source["lfImpliesCr"] == true
	clean["reconnectKeys"] = source["reconnectKeys"] != false
	clean["closeOnOtherKey"] = source["closeOnOtherKey"] == true
	clean["crImpliesLf"] = source["crImpliesLf"] == true
	if separators, ok := source["wordSeparators"].(string); ok {
		clean["wordSeparators"] = keepRunes(separators, 64, func(r rune) bool {
			return !unicode.IsControl(r) && !unicode.IsLetter(r) && !unicode.IsNumber(r)
		})
	}
	if answerback, ok := source["answerback"].(string); ok {
		clean["answerback"] = keepRunes(answerback, 64, func(r rune) bool { return r >= 0x20 && r <= 0x7e })
	}
	return clean
}

func keepRunes(value string, limit int, keep func(rune) bool) string {
	var out strings.Builder
	count := 0
	for _, r := range value {
		if count == limit {
			break
		}
		if keep(r) {
			out.WriteRune(r)
			count++
		}
	}
	return out.String()
}
