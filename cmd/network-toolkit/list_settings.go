package main

import "sort"

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
			if text, ok := input[field].(string); ok && len(text) <= limit {
				row[field] = text
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
