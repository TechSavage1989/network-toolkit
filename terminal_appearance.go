package main

import (
	"math"
	"regexp"
	"strings"
	"unicode"
)

var terminalColorPattern = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)
var terminalSelectionPattern = regexp.MustCompile(`^#[0-9a-fA-F]{6}([0-9a-fA-F]{2})?$`)

func safeTerminalAppearance(value any) map[string]any {
	source, _ := value.(map[string]any)
	clean := map[string]any{"fontFamily": "Consolas", "fontWeight": "normal", "lineHeight": 1.15, "scheme": "app", "highlightUnderlines": true}
	if source["highlightUnderlines"] == false {
		clean["highlightUnderlines"] = false
	}
	highlights := map[string]any{}
	if requested, ok := source["highlightColors"].(map[string]any); ok {
		for _, key := range []string{"address", "mac", "interface", "command", "field", "protocol", "value", "number", "comment", "warning", "error", "success"} {
			if color, ok := requested[key].(string); ok && terminalColorPattern.MatchString(color) {
				highlights[key] = strings.ToLower(color)
			}
		}
	}
	clean["highlightColors"] = highlights
	clean["shells"] = safeTerminalShellAppearance(source["shells"])
	if family, ok := source["fontFamily"].(string); ok {
		family = strings.TrimSpace(family)
		valid := len([]rune(family)) > 0 && len([]rune(family)) <= 80
		for _, r := range family {
			if !unicode.IsLetter(r) && !unicode.IsNumber(r) && !strings.ContainsRune(" _.-", r) {
				valid = false
			}
		}
		if valid {
			clean["fontFamily"] = family
		}
	}
	if source["fontWeight"] == "bold" {
		clean["fontWeight"] = "bold"
	}
	if height, ok := source["lineHeight"].(float64); ok && height >= 1 && height <= 2 {
		clean["lineHeight"] = math.Round(height*100) / 100
	}
	if scheme, ok := source["scheme"].(string); ok {
		switch scheme {
		case "app", "dark", "light", "mobaCurrent", "pastelDusk", "pastelMint", "pastelLavender", "rosePine", "catppuccin", "nord", "tokyoNight", "gruvbox", "dracula", "solarizedDark", "solarizedLight", "campbell", "custom":
			clean["scheme"] = scheme
		}
	}
	colors := map[string]any{}
	if requested, ok := source["colors"].(map[string]any); ok {
		for _, key := range []string{"background", "foreground", "cursor", "selectionBackground", "black", "red", "green", "yellow", "blue", "magenta", "cyan", "white", "brightBlack", "brightRed", "brightGreen", "brightYellow", "brightBlue", "brightMagenta", "brightCyan", "brightWhite"} {
			if color, ok := requested[key].(string); ok && (terminalColorPattern.MatchString(color) || key == "selectionBackground" && terminalSelectionPattern.MatchString(color)) {
				colors[key] = strings.ToLower(color)
			}
		}
	}
	clean["colors"] = colors
	return clean
}

func safeTerminalShellAppearance(value any) map[string]any {
	source, _ := value.(map[string]any)
	clean := map[string]any{}
	for _, kind := range []string{"powershell", "cmd"} {
		settings, _ := source[kind].(map[string]any)
		colors := map[string]any{}
		if requested, ok := settings["colors"].(map[string]any); ok {
			for _, role := range []string{"command", "keyword", "parameter", "variable", "string", "operator", "number", "type", "comment", "outputHeading", "outputField", "outputValue", "outputAddress", "outputSuccess", "outputWarning", "outputError"} {
				if color, ok := requested[role].(string); ok && terminalColorPattern.MatchString(color) {
					colors[role] = strings.ToLower(color)
				}
			}
		}
		clean[kind] = map[string]any{"enabled": settings["enabled"] != false, "colors": colors}
	}
	return clean
}
