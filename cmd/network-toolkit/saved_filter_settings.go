package main

import (
	"regexp"
	"sort"
	"strings"
	"unicode/utf8"
)

var savedFilterScope = regexp.MustCompile(`^(network|result):[a-z][a-z0-9_-]*(:[a-zA-Z0-9_.:-]+)?$`)
var savedFilterID = regexp.MustCompile(`^[a-zA-Z0-9_-]{1,100}$`)
var savedFilterToken = regexp.MustCompile(`^[-\w .]*$`)

func savedFilterUTF16Length(text string) int {
	if !utf8.ValidString(text) {
		return -1
	}
	units := 0
	for _, char := range text {
		units++
		if char > 0xffff {
			units++
		}
	}
	return units
}

func validSavedFilterScope(scope string) bool {
	if len(scope) > 200 || !savedFilterScope.MatchString(scope) {
		return false
	}
	for _, part := range strings.Split(scope, ":") {
		if part == "__proto__" || part == "constructor" || part == "prototype" {
			return false
		}
	}
	return true
}

func cleanSavedFilter(input map[string]any, scope string) (map[string]any, bool) {
	if input == nil {
		return nil, false
	}
	query, ok := input["query"].(string)
	if !ok {
		if _, present := input["query"]; present {
			return nil, false
		}
	}
	units := savedFilterUTF16Length(query)
	limit := 300
	network := strings.HasPrefix(scope, "network:")
	if network {
		limit = networkListQueryUTF16Limit
	}
	if units < 0 || units > limit {
		return nil, false
	}
	query = strings.NewReplacer("\r\n", " ", "\r", " ", "\n", " ", "\u2028", " ", "\u2029", " ").Replace(query)
	out := map[string]any{"query": query}
	fields := []string{"legacyStatus", "status", "type"}
	if network {
		fields = []string{"status"}
	}
	for _, field := range fields {
		value := "all"
		if raw, present := input[field]; present {
			var ok bool
			value, ok = raw.(string)
			if !ok || len(value) > 100 || !savedFilterToken.MatchString(value) {
				return nil, false
			}
			if value == "" {
				value = "all"
			}
		}
		out[field] = value
	}
	if network {
		out["respondedOnly"], _ = input["respondedOnly"].(bool)
	} else {
		value := ""
		if raw, present := input["statistic"]; present {
			var ok bool
			value, ok = raw.(string)
			if !ok {
				return nil, false
			}
		}
		switch value {
		case "", "sent", "received", "loss", "minimum", "average", "maximum", "jitter":
			out["statistic"] = value
		default:
			return nil, false
		}
	}
	return out, true
}

// Named filters have a narrow independent schema. Existing current-filter
// preferences remain unchanged when upgrading from a version without presets.
// The surrounding state API retains its 2 MiB request and saved-file limits.
func sanitizeSavedResultFilters(state map[string]any) {
	raw, present := state["savedResultFilters"]
	if !present {
		return
	}
	scopes := map[string]any{}
	clean := map[string]any{"version": 1, "scopes": scopes}
	state["savedResultFilters"] = clean
	value, ok := raw.(map[string]any)
	if !ok {
		return
	}
	version, ok := value["version"].(float64)
	if integer, integerOK := value["version"].(int); integerOK {
		version, ok = float64(integer), true
	}
	if !ok || version != 1 {
		return
	}
	input, ok := value["scopes"].(map[string]any)
	if !ok {
		return
	}
	keys := make([]string, 0, len(input))
	for key := range input {
		if validSavedFilterScope(key) {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)
	count, units := 0, 0
	for _, scope := range keys {
		rows, ok := input[scope].([]any)
		if !ok {
			continue
		}
		out := []any{}
		names, ids := map[string]bool{}, map[string]bool{}
		for _, entry := range rows {
			if count >= 64 || len(out) >= 20 {
				break
			}
			row, ok := entry.(map[string]any)
			if !ok {
				continue
			}
			name, ok := row["name"].(string)
			if !ok {
				continue
			}
			name = strings.TrimSpace(name)
			nameUnits := savedFilterUTF16Length(name)
			folded := strings.ToLower(name)
			id, ok := row["id"].(string)
			if !ok || !savedFilterID.MatchString(id) || nameUnits <= 0 || nameUnits > 80 || names[folded] || ids[id] {
				continue
			}
			fields, _ := row["filter"].(map[string]any)
			filter, ok := cleanSavedFilter(fields, scope)
			if !ok {
				continue
			}
			queryUnits := savedFilterUTF16Length(filter["query"].(string))
			if units+queryUnits > 262144 {
				continue
			}
			out = append(out, map[string]any{"id": id, "name": name, "filter": filter})
			names[folded] = true
			ids[id] = true
			units += queryUnits
			count++
		}
		if len(out) > 0 {
			scopes[scope] = out
		}
	}
}
