package main

import (
	"math"
	"regexp"
	"strings"
)

// Client connection history is metadata, never a credential store. Import and
// HTTP state writes pass through this allow-list as well as the front end.
var transferSSHHostKey = regexp.MustCompile(`^SHA256:[A-Za-z0-9+/=]{20,100}$`)

func sanitizeTransferClientSettings(state map[string]any) {
	if raw, found := state["transferClientLayout"]; found {
		clean := map[string]any{}
		if entries, ok := raw.(map[string]any); ok {
			for _, protocol := range []string{"sftp", "tftp", "ftp"} {
				if row, ok := entries[protocol].(map[string]any); ok {
					tab, _ := row["tab"].(string)
					if tab != "saved" && tab != "recent" {
						tab = "settings"
					}
					side, _ := row["side"].(string)
					if side != "right" {
						side = "left"
					}
					clean[protocol] = map[string]any{
						"collapsed":      row["collapsed"] == true,
						"width":          transferInteger(row["width"], 260, 480, 300),
						"activityHeight": transferInteger(row["activityHeight"], 180, 1200, 0),
						"tab":            tab,
						"side":           side,
					}
				}
			}
		}
		state["transferClientLayout"] = clean
	}
	if raw, found := state["transferServerSidebar"]; found {
		clean := map[string]any{}
		if entries, ok := raw.(map[string]any); ok {
			for _, protocol := range []string{"sftp", "tftp", "ftp"} {
				if row, ok := entries[protocol].(map[string]any); ok {
					clean[protocol] = map[string]any{
						"collapsed":      row["collapsed"] == true,
						"width":          transferInteger(row["width"], 270, 460, 320),
						"activityHeight": transferInteger(row["activityHeight"], 180, 1200, 0),
					}
				}
			}
		}
		state["transferServerSidebar"] = clean
	}
	if raw, present := state["sftpRecent"]; present {
		clean := make([]map[string]any, 0)
		seen := map[string]bool{}
		if entries, ok := raw.([]any); ok {
			for _, rawEntry := range entries {
				if len(clean) >= 50 {
					break
				}
				entry, ok := rawEntry.(map[string]any)
				if !ok {
					continue
				}
				id, okID := transferText(entry["id"], 100)
				host, okHost := safeTransferHostName(entry["host"])
				username, okUser := transferText(entry["username"], 256)
				name, okName := transferText(entry["name"], 100)
				port, okPort := transferPort(entry["port"])
				if !okID || !okHost || !okUser || !okName || !okPort || seen[id] {
					continue
				}
				seen[id] = true
				auth, _ := entry["authMethod"].(string)
				if auth != "key" {
					auth = "password"
				}
				credentialID, _ := transferCredentialID(entry["credentialId"])
				fingerprint, _ := entry["fingerprint"].(string)
				if !transferSSHHostKey.MatchString(fingerprint) {
					fingerprint = ""
				}
				lastUsed := transferTimestamp(entry["lastUsed"])
				clean = append(clean, map[string]any{
					"id": id, "name": name, "type": "ssh", "host": host, "port": port,
					"username": username, "authMethod": auth, "credentialId": credentialID,
					"fingerprint": fingerprint, "sshCompatibility": func() string {
						if entry["sshCompatibility"] == "legacy" {
							return "legacy"
						}
						return "modern"
					}(), "lastUsed": lastUsed,
				})
			}
		}
		state["sftpRecent"] = clean
	}
	for _, key := range []string{"tftpProfiles", "tftpRecent"} {
		raw, present := state[key]
		if !present {
			continue
		}
		limit := 100
		if key == "tftpRecent" {
			limit = 50
		}
		clean := make([]map[string]any, 0)
		seen := map[string]bool{}
		if entries, ok := raw.([]any); ok {
			for _, rawEntry := range entries {
				if len(clean) >= limit {
					break
				}
				entry, ok := rawEntry.(map[string]any)
				if !ok {
					continue
				}
				id, okID := transferText(entry["id"], 100)
				host, okHost := safeTransferHostName(entry["host"])
				name, okName := transferText(entry["name"], 100)
				port, okPort := transferPort(entry["port"])
				if !okID || !okHost || !okName || !okPort || seen[id] {
					continue
				}
				operation, _ := entry["operation"].(string)
				if operation != "upload" && operation != "download" {
					operation = "download"
				}
				remotePath := transferOptionalPath(entry["remotePath"])
				localPath := transferOptionalPath(entry["localPath"])
				blockSize := transferInteger(entry["blockSize"], 8, 65464, 1428)
				timeoutSec := transferInteger(entry["timeoutSec"], 1, 15, 3)
				retries := transferInteger(entry["retries"], 1, 20, 5)
				overwrite := entry["overwrite"] == true
				seen[id] = true
				item := map[string]any{
					"id": id, "name": name, "host": host, "port": port, "operation": operation,
					"remotePath": remotePath, "localPath": localPath, "blockSize": blockSize,
					"timeoutSec": timeoutSec, "retries": retries, "overwrite": overwrite,
				}
				if key == "tftpRecent" {
					item["lastUsed"] = transferTimestamp(entry["lastUsed"])
				}
				clean = append(clean, item)
			}
		}
		state[key] = clean
	}
	if raw, present := state["sftpVerifiedHostKeys"]; present {
		clean := map[string]string{}
		keys, ok := raw.(map[string]any)
		if ok {
			for address, value := range keys {
				if len(clean) >= 150 {
					break
				}
				fingerprint, isString := value.(string)
				if !isString || !transferSSHHostKey.MatchString(fingerprint) || len(address) > 280 || strings.TrimSpace(address) != address || strings.ContainsAny(address, "\x00\r\n\t ") || !strings.Contains(address, ":") {
					continue
				}
				clean[address] = fingerprint
			}
		}
		state["sftpVerifiedHostKeys"] = clean
	}
}

func transferText(value any, max int) (string, bool) {
	text, ok := value.(string)
	if !ok || len(text) == 0 || len(text) > max || strings.TrimSpace(text) != text || strings.ContainsAny(text, "\x00\r\n") {
		return "", false
	}
	return text, true
}
func safeTransferHostName(value any) (string, bool) {
	text, ok := transferText(value, 253)
	if !ok || strings.ContainsAny(text, " \t/\\") {
		return "", false
	}
	return text, true
}
func transferCredentialID(value any) (string, bool) {
	text, ok := value.(string)
	if !ok || text == "" {
		return "", false
	}
	if len(text) > 120 || strings.ContainsAny(text, "/\\\x00\r\n") {
		return "", false
	}
	return text, true
}
func transferPort(value any) (int, bool) {
	number, ok := value.(float64)
	return int(number), ok && number >= 1 && number <= 65535 && number == math.Trunc(number)
}
func transferInteger(value any, min, max, fallback int) int {
	number, ok := value.(float64)
	if !ok || math.IsNaN(number) || number < float64(min) || number > float64(max) || number != math.Trunc(number) {
		return fallback
	}
	return int(number)
}
func transferTimestamp(value any) float64 {
	number, ok := value.(float64)
	if !ok || math.IsInf(number, 0) || math.IsNaN(number) || number < 0 || number > 9999999999999 {
		return 0
	}
	return number
}
func transferOptionalPath(value any) string {
	text, ok := value.(string)
	if !ok || len(text) > 2048 || strings.ContainsAny(text, "\x00\r\n") {
		return ""
	}
	return text
}
