package main

import "strings"

// Palette preferences are only bounded color strings. They never contain theme
// objects, profile metadata, paths or credentials, and do not migrate themes.
func sanitizeColorPickerPreferences(state map[string]any) {
	if value, present := state["colorPickerPreferences"]; present {
		state["colorPickerPreferences"] = safeColorPickerPreferences(value)
	}
}

func safeColorPickerPreferences(value any) map[string]any {
	input, _ := value.(map[string]any)
	out := map[string]any{}
	for key, limit := range map[string]int{"recent": 12, "saved": 24} {
		items, _ := input[key].([]any)
		if typed, ok := input[key].([]string); ok {
			if len(typed) > 256 {
				typed = typed[:256]
			}
			items = make([]any, len(typed))
			for i, item := range typed {
				items[i] = item
			}
		}
		colors := make([]string, 0, limit)
		seen := map[string]bool{}
		for i, item := range items {
			if i >= 256 || len(colors) >= limit {
				break
			}
			color, ok := item.(string)
			if !ok || (len(color) != 7 && len(color) != 9) || color[0] != '#' {
				continue
			}
			valid := true
			for _, digit := range color[1:] {
				if !((digit >= '0' && digit <= '9') || (digit >= 'a' && digit <= 'f') || (digit >= 'A' && digit <= 'F')) {
					valid = false
					break
				}
			}
			color = strings.ToLower(color)
			if valid && !seen[color] {
				seen[color] = true
				colors = append(colors, color)
			}
		}
		out[key] = colors
	}
	return out
}
