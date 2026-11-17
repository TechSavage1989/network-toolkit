package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf16"

	"networktoolkit/internal/editorbuffer"
)

type findQuickEntry struct {
	Group   string    `json:"group"`
	Label   string    `json:"label"`
	Title   string    `json:"title"`
	Find    string    `json:"find"`
	Replace *string   `json:"replace"`
	Mode    string    `json:"mode"`
	Example [2]string `json:"example"`
}

// The Find window's Quick searches (QUICK in web/assets/editor-find.js) run
// in the engine: every pattern is one the engine accepts, and each entry does
// what its example says (a replacement turns the first text into the second;
// a search finds the second text first).
func TestFindQuickSearches(t *testing.T) {
	source, err := os.ReadFile(filepath.Join("..", "..", "web", "assets", "editor-find.js"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(source)
	start, end := strings.Index(text, "// QUICK-BEGIN"), strings.Index(text, "// QUICK-END")
	if start < 0 || end < start {
		t.Fatal("QUICK markers are missing")
	}
	var entries []findQuickEntry
	if err := json.Unmarshal([]byte(text[start+len("// QUICK-BEGIN"):end]), &entries); err != nil {
		t.Fatal(err)
	}
	if len(entries) < 10 {
		t.Fatalf("only %d quick searches", len(entries))
	}
	// The escapes the entries use in Extended mode, decoded as the page's
	// decodeExtended does.
	extended := strings.NewReplacer(`\n`, "\n", `\t`, "\t", `\\`, `\`)
	labels := map[string]bool{}
	for _, entry := range entries {
		t.Run(entry.Label, func(t *testing.T) {
			if labels[entry.Label] || entry.Title == "" || entry.Find == "" || (entry.Group != "Find" && entry.Group != "Replace") || (entry.Group == "Replace") != (entry.Replace != nil) {
				t.Fatalf("entry: %+v", entry)
			}
			labels[entry.Label] = true
			query, replacement := entry.Find, ""
			if entry.Replace != nil {
				replacement = *entry.Replace
			}
			switch entry.Mode {
			case "normal":
			case "extended":
				query, replacement = extended.Replace(query), extended.Replace(replacement)
			case "regex":
				if problem := editorbuffer.CheckPattern(query); problem != nil {
					t.Fatalf("the engine refuses %q: %s", query, problem.Message)
				}
			default:
				t.Fatalf("mode %q", entry.Mode)
			}
			search := editorbuffer.Search{Query: query, Regex: entry.Mode == "regex", MatchCase: true}
			ctx := context.Background()
			if entry.Replace != nil {
				got, count, err := editorbuffer.ReplaceInText(ctx, entry.Example[0], search, replacement, 0, -1, 1<<20)
				if err != nil || count == 0 || got != entry.Example[1] {
					t.Fatalf("%q → %q (%d, %v), want %q", entry.Example[0], got, count, err, entry.Example[1])
				}
				return
			}
			s, err := editorbuffer.TextSnapshot(entry.Example[0])
			if err != nil {
				t.Fatal(err)
			}
			defer s.Close()
			match, err := s.FindNext(ctx, search, 0, 0, -1, false, false)
			if err != nil || !match.Found {
				t.Fatalf("no match in %q: %v", entry.Example[0], err)
			}
			units := utf16.Encode([]rune(entry.Example[0]))
			if got := string(utf16.Decode(units[match.From:match.To])); got != entry.Example[1] {
				t.Fatalf("found %q in %q, want %q", got, entry.Example[0], entry.Example[1])
			}
		})
	}
}
