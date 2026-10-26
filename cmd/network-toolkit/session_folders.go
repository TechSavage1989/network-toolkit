package main

import "strings"

// Saved-session folders are labels, never filesystem paths or connection options.
func safeSessionFolder(value any) string {
	name, _ := value.(string)
	name = strings.Map(func(r rune) rune {
		if r < 32 || r == 127 {
			return -1
		}
		return r
	}, name)
	runes := []rune(strings.TrimSpace(name))
	if len(runes) > 100 {
		runes = runes[:100]
	}
	return string(runes)
}
