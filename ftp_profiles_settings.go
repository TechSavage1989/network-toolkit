package main

import (
	"math"
	"strings"
)

// FTP/FTPS profiles are preferences, not a password store. Allow-list fields
// when persisting so a caller cannot smuggle secrets into an ordinary settings
// file. A credentialId is a reference to the existing Windows-protected vault.
func sanitizeFTPProfiles(state map[string]any) {
	raw, present := state["ftpProfiles"]
	if !present {
		return
	}
	list, ok := raw.([]any)
	if !ok {
		state["ftpProfiles"] = []any{}
		return
	}
	clean := make([]map[string]any, 0, len(list))
	seen := map[string]bool{}
	for _, row := range list {
		if len(clean) >= 100 {
			break
		}
		item, ok := row.(map[string]any)
		if !ok {
			continue
		}
		id, ok1 := item["id"].(string)
		name, ok2 := item["name"].(string)
		host, ok3 := item["host"].(string)
		username, ok4 := item["username"].(string)
		port, ok5 := item["port"].(float64)
		if !ok1 || !ok2 || !ok3 || !ok4 || !ok5 || strings.TrimSpace(id) == "" || seen[id] || len(id) > 100 || len(name) == 0 || len(name) > 100 || len(host) == 0 || len(host) > 253 || len(username) > 256 || port < 1 || port > 65535 || port != math.Trunc(port) {
			continue
		}
		if strings.ContainsAny(id, "/\\\x00\n") || strings.ContainsAny(host, "\x00\n") {
			continue
		}
		seen[id] = true
		security, _ := item["security"].(string)
		if security != "explicit" && security != "implicit" && security != "plain" {
			security = "explicit"
		}
		fingerprint, _ := item["fingerprint"].(string)
		if len(fingerprint) > 256 {
			fingerprint = ""
		}
		credentialID, _ := item["credentialId"].(string)
		if len(credentialID) > 120 || strings.ContainsAny(credentialID, "/\\\x00\n") {
			credentialID = ""
		}
		clean = append(clean, map[string]any{"id": id, "name": name, "host": host, "username": username, "port": port, "security": security, "fingerprint": fingerprint, "credentialId": credentialID})
	}
	state["ftpProfiles"] = clean
}
