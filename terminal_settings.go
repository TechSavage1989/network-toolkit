package main

import (
	"math"
	"strings"
)

// Only connection identifiers and display/serial preferences can reach the
// portable settings file. This independently enforces the same nonsecret
// profile schema as the interface, including for imported settings.
func sanitizeTerminalSettings(state map[string]any) {
	// Earlier releases could launch third-party terminal applications. The
	// built-in transports are now the only terminal destination; remove old
	// application selections and executable paths when loading or saving.
	delete(state, "terminalLaunch")
	if value, present := state["terminalAppearance"]; present {
		state["terminalAppearance"] = safeTerminalAppearance(value)
	}
	if value, present := state["terminalActiveActionsPosition"]; present {
		position, _ := value.(string)
		if position != "bottom" {
			position = "top"
		}
		state["terminalActiveActionsPosition"] = position
	}
	if value, present := state["terminalSidebarTab"]; present {
		tab, _ := value.(string)
		if tab != "settings" && tab != "saved" && tab != "recent" && tab != "macros" && tab != "files" {
			tab = "saved"
		}
		state["terminalSidebarTab"] = tab
	}
	if value, present := state["terminalSidebarWidth"]; present {
		width, ok := value.(float64)
		if !ok || math.IsNaN(width) || math.IsInf(width, 0) || width < 260 || width > 480 || width != math.Trunc(width) {
			width = 300
		}
		state["terminalSidebarWidth"] = width
	}
	if value, present := state["terminalMacroConfirmation"]; present {
		policy, _ := value.(string)
		if policy != "always" && policy != "multiple" && policy != "never" {
			policy = "always"
		}
		state["terminalMacroConfirmation"] = policy
	}
	for _, key := range []string{"terminalSidebarCollapsed", "terminalRecentCollapsed", "terminalSavedCollapsed", "terminalSessionConfirmDelete", "terminalComposerCollapsed"} {
		if value, present := state[key]; present {
			if _, ok := value.(bool); !ok {
				delete(state, key)
			}
		}
	}
	if value, present := state["terminalRecent"]; present {
		state["terminalRecent"] = safeTerminalRecents(value)
	}
	if value, present := state["terminalInteraction"]; present {
		state["terminalInteraction"] = safeTerminalInteraction(value)
	}
	if layout, present := state["terminalLayout"]; present {
		clean := map[string]any{"kind": "single", "columnRatio": 0.5, "rowRatio": 0.5}
		if source, ok := layout.(map[string]any); ok {
			if kind, ok := source["kind"].(string); ok {
				switch kind {
				case "single", "columns", "rows", "three", "grid":
					clean["kind"] = kind
				}
			}
			for _, key := range []string{"columnRatio", "rowRatio"} {
				if ratio, ok := source[key].(float64); ok && !math.IsNaN(ratio) && !math.IsInf(ratio, 0) {
					clean[key] = math.Max(0.2, math.Min(0.8, ratio))
				}
			}
		}
		state["terminalLayout"] = clean
	}
	if value, present := state["terminalFontSize"]; present {
		size, ok := value.(float64)
		if !ok || size < 10 || size > 24 || size != math.Trunc(size) {
			delete(state, "terminalFontSize")
		}
	}
	if value, present := state["terminalHighlight"]; present {
		if _, ok := value.(bool); !ok {
			delete(state, "terminalHighlight")
		}
	}
	if current, present := state["terminalConnection"]; present {
		state["terminalConnection"] = safeTerminalProfile(current, false)
	}
	if profiles, present := state["terminalProfiles"]; present {
		clean := []map[string]any{}
		if list, ok := profiles.([]any); ok {
			for _, profile := range list {
				if len(clean) >= 200 {
					break
				}
				if _, ok := profile.(map[string]any); !ok {
					continue
				}
				clean = append(clean, safeTerminalProfile(profile, true))
			}
		}
		state["terminalProfiles"] = clean
	}
}

func safeTerminalRecents(value any) []map[string]any {
	list, _ := value.([]any)
	if typed, ok := value.([]map[string]any); ok {
		list = make([]any, len(typed))
		for i := range typed {
			list[i] = typed[i]
		}
	}
	clean := []map[string]any{}
	seen := map[string]bool{}
	for _, value := range list {
		if len(clean) == 50 {
			break
		}
		source, ok := value.(map[string]any)
		if !ok {
			continue
		}
		id, idOK := source["id"].(string)
		name, nameOK := source["name"].(string)
		if !idOK || !nameOK || strings.TrimSpace(id) == "" || len(id) > 80 || len(name) > 100 || seen[id] {
			continue
		}
		seen[id] = true
		item := safeTerminalProfile(source, true)
		item["lastUsed"] = float64(0)
		if timestamp, ok := source["lastUsed"].(float64); ok && timestamp >= 0 && timestamp <= 9007199254740991 && timestamp == math.Trunc(timestamp) {
			item["lastUsed"] = timestamp
		}
		clean = append(clean, item)
	}
	return clean
}

// Mouse and keyboard settings can cross a linked-window boundary. Keep that
// object limited to presentation choices and known built-in menu actions.
func safeTerminalInteraction(value any) map[string]any {
	clean := map[string]any{
		"copyOnSelect":         true,
		"copyOnClick":          true,
		"pasteReview":          true,
		"pasteIncludeCommands": false,
		"autoPager":            false,
		"pagerKey":             "auto",
		"rightClick":           "paste",
		"ctrlC":                "send",
		"menuItems":            []string{"copy", "copy-line", "copy-token", "paste", "scroll-top", "scroll-bottom", "paste-start", "copy-paste-output", "select-all", "search", "export", "zoom-in", "zoom-out", "control-c", "move"},
	}
	source, ok := value.(map[string]any)
	if !ok {
		return clean
	}
	for _, key := range []string{"copyOnSelect", "copyOnClick", "pasteReview", "pasteIncludeCommands", "autoPager"} {
		if enabled, ok := source[key].(bool); ok {
			clean[key] = enabled
		}
	}
	if source["rightClick"] == "menu" {
		clean["rightClick"] = "menu"
	}
	if source["ctrlC"] == "copy-selection" {
		clean["ctrlC"] = "copy-selection"
	}
	if key, ok := source["pagerKey"].(string); ok && (key == "auto" || key == "space" || key == "enter") {
		clean["pagerKey"] = key
	}
	var selected map[string]bool
	switch requested := source["menuItems"].(type) {
	case []any:
		selected = map[string]bool{}
		for _, item := range requested {
			if id, ok := item.(string); ok {
				selected[id] = true
			}
		}
	case []string:
		selected = map[string]bool{}
		for _, id := range requested {
			selected[id] = true
		}
	}
	if selected != nil {
		items := []string{}
		for _, id := range []string{"copy", "copy-line", "copy-token", "paste", "scroll-top", "scroll-bottom", "paste-start", "paste-previous", "paste-next", "select-paste-output", "copy-paste-output", "copy-paste-all", "select-all", "search", "clear", "export", "zoom-in", "zoom-out", "reset-font", "fit", "maximize", "control-c", "pager-toggle", "reconnect", "disconnect", "move"} {
			if selected[id] {
				items = append(items, id)
			}
		}
		clean["menuItems"] = items
	}
	return clean
}

func safeTerminalProfile(value any, named bool) map[string]any {
	clean := map[string]any{}
	profile, ok := value.(map[string]any)
	if !ok {
		return clean
	}
	stringFields := []string{"type", "host", "username", "authMethod", "credentialId", "serialPort", "stopBits", "parity", "flowControl", "lineEnding"}
	if named {
		stringFields = append(stringFields, "id", "name")
	}
	for _, key := range stringFields {
		if value, ok := profile[key].(string); ok && len(value) <= 1024 {
			clean[key] = value
		}
	}
	for _, key := range []string{"port", "timeoutMs", "baudRate", "dataBits"} {
		if value, ok := profile[key].(float64); ok {
			clean[key] = value
		}
	}
	if value, ok := profile["localEcho"].(bool); ok {
		clean["localEcho"] = value
	}
	// Compatibility is a bounded connection preference, never a global switch.
	// Unknown/imported values keep the modern policy and other transports ignore it.
	clean["sshCompatibility"] = "modern"
	kind, _ := profile["type"].(string)
	if (kind == "" || kind == "ssh") && profile["sshCompatibility"] == "legacy" {
		clean["sshCompatibility"] = "legacy"
	}
	return clean
}
