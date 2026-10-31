package main

import (
	"regexp"
	"strconv"
	"strings"
	"unicode"
)

var terminalPromptName127 = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.-]{0,99}$`)
var terminalPromptNameDottedNumeric128 = regexp.MustCompile(`^[0-9]+(?:\.[0-9]+)+$`)
var terminalGenericPrompt128 = regexp.MustCompile(`^(?:loader(?:-[a-z])?|bash(?:-[0-9.]+)?|zsh(?:-[0-9.]+)?|sh(?:-[0-9.]+)?|fish(?:-[0-9.]+)?)$`)

// Display metadata only: never part of connection/authentication identity.
func safeTerminalPromptNames(value any) []map[string]any {
	out := []map[string]any{}
	seen := map[string]bool{}
	list, _ := value.([]any)
	if typed, ok := value.([]map[string]any); ok {
		list = make([]any, len(typed))
		for i := range typed {
			list[i] = typed[i]
		}
	}
	for _, raw := range list {
		if len(out) >= 200 {
			break
		}
		item, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		kind, _ := item["type"].(string)
		host, _ := item["host"].(string)
		name, _ := item["name"].(string)
		user, userOK := item["username"].(string)
		port, portOK := item["port"].(float64)
		if kind != "ssh" && kind != "telnet" || host == "" || len(host) > 512 || !userOK || len(user) > 256 || !portOK || port < 1 || port > 65535 || port != float64(int(port)) || !validTerminalPromptName(name) {
			continue
		}
		invalid := false
		for _, r := range host {
			if unicode.IsControl(r) || r <= 32 {
				invalid = true
			}
		}
		for _, r := range user {
			if unicode.IsControl(r) {
				invalid = true
			}
		}
		if invalid {
			continue
		}
		// NUL is rejected in each textual component, keeping keys unambiguous.
		key := kind + "\x00" + host + "\x00" + strconv.Itoa(int(port)) + "\x00" + user
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, map[string]any{"type": kind, "host": host, "port": port, "username": user, "name": name})
	}
	return out
}

func validTerminalPromptName(name string) bool {
	if !terminalPromptName127.MatchString(name) || terminalPromptNameDottedNumeric128.MatchString(name) {
		return false
	}
	switch strings.ToLower(name) {
	case "password", "username", "login", "error", "warning", "failed", "success", "yes", "no", "auditor", "console", "ps", "edit":
		return false
	}
	return !terminalGenericPrompt128.MatchString(strings.ToLower(name))
}
