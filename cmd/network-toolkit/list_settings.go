package main

import (
	"sort"
	"strings"
	"unicode/utf8"
)

// Match the discovery search input's maxLength and JavaScript string length,
// which count UTF-16 code units. A unit needs at most three UTF-8 bytes; astral
// characters use two units and four bytes. The overall state API still caps
// both the incoming request and serialized settings at 2 MiB.
const networkListQueryUTF16Limit = 32768

func validNetworkListQuery(text string) bool {
	if len(text) > 3*networkListQueryUTF16Limit || !utf8.ValidString(text) {
		return false
	}
	units := 0
	for _, char := range text {
		units++
		if char > 0xffff {
			units++
		}
		if units > networkListQueryUTF16Limit {
			return false
		}
	}
	return true
}

// Persist table controls only. Rows, terminal output and nested arbitrary
// objects must not be retained through the preference schema.
func sanitizeListPreferences(state map[string]any) {
	value, present := state["listPreferences"]
	if !present {
		return
	}
	clean := map[string]any{}
	views, _ := value.(map[string]any)
	keys := make([]string, 0, len(views))
	for key := range views {
		if key != "" && len(key) <= 200 && key != "__proto__" && key != "constructor" && key != "prototype" {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)
	for _, key := range keys {
		if len(clean) >= 160 {
			break
		}
		input, ok := views[key].(map[string]any)
		if !ok {
			continue
		}
		row := map[string]any{}
		for field, limit := range map[string]int{"query": 512, "sort": 128, "status": 64} {
			if text, ok := input[field].(string); ok {
				allowed := len(text) <= limit
				if field == "query" && strings.HasPrefix(key, "network:") {
					allowed = validNetworkListQuery(text)
				}
				if allowed {
					row[field] = text
				}
			}
		}
		if direction, ok := input["direction"].(string); ok && (direction == "asc" || direction == "desc") {
			row["direction"] = direction
		}
		if responded, ok := input["respondedOnly"].(bool); ok {
			row["respondedOnly"] = responded
		}
		clean[key] = row
	}
	state["listPreferences"] = clean
}
