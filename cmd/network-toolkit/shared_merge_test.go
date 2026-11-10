package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"reflect"
	"sort"
	"strings"
	"testing"
)

func sharedJSON(t *testing.T, raw string) any {
	t.Helper()
	if raw == "" {
		return nil
	}
	var value any
	if err := json.Unmarshal([]byte(raw), &value); err != nil {
		t.Fatalf("fixture %q: %v", raw, err)
	}
	return value
}

func sharedText(t *testing.T, value any) string {
	t.Helper()
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

func TestSharedListMergeAddsRemovesAndEditsOnBothSides(t *testing.T) {
	key := sharedKey{Name: "terminalProfiles", Kind: sharedList}
	a, b, c := `{"id":"a","name":"A"}`, `{"id":"b","name":"B"}`, `{"id":"c","name":"C"}`
	list := func(items ...string) string { return "[" + strings.Join(items, ",") + "]" }
	cases := []struct{ name, base, theirs, ours, want string }{
		{"nothing changed", list(a, b), list(a, b), list(a, b), list(a, b)},
		{"page adds", list(a), list(a), list(a, b), list(a, b)},
		{"other app adds, stale page saves", list(a), list(a, b), list(a), list(a, b)},
		{"both add different items", list(a), list(a, b), list(a, c), list(a, b, c)},
		{"page adds at the front", list(a), list(a, b), list(c, a), list(c, a, b)},
		{"page removes", list(a, b), list(a, b), list(a), list(a)},
		{"other app removes", list(a, b), list(a), list(a, b), list(a)},
		{"both remove different items", list(a, b, c), list(a, c), list(a, b), list(a)},
		{"both remove the same item", list(a, b), list(a), list(a), list(a)},
		{"page edits", list(a, b), list(a, b), list(`{"id":"a","name":"A2"}`, b), list(`{"id":"a","name":"A2"}`, b)},
		{"other app edits", list(a, b), list(`{"id":"a","name":"A3"}`, b), list(a, b), list(`{"id":"a","name":"A3"}`, b)},
		{"both edit the same item: the page's edit is kept", list(a), list(`{"id":"a","name":"theirs"}`), list(`{"id":"a","name":"ours"}`), list(`{"id":"a","name":"ours"}`)},
		{"both edit different items", list(a, b), list(`{"id":"a","name":"A3"}`, b), list(a, `{"id":"b","name":"B2"}`), list(`{"id":"a","name":"A3"}`, `{"id":"b","name":"B2"}`)},
		{"page edits an item removed elsewhere: the edit is kept", list(a, b), list(a), list(a, `{"id":"b","name":"B2"}`), list(a, `{"id":"b","name":"B2"}`)},
		{"page removes an item edited elsewhere: the edit is kept", list(a, b), list(a, `{"id":"b","name":"B3"}`), list(a), list(a, `{"id":"b","name":"B3"}`)},
		{"both add the same id with different content: the page's is kept", list(), list(`{"id":"n","name":"theirs"}`), list(`{"id":"n","name":"ours"}`), list(`{"id":"n","name":"ours"}`)},
		{"both add the same item", list(), list(a), list(a), list(a)},
		{"page reorders", list(a, b, c), list(a, b, c), list(c, a, b), list(c, a, b)},
		{"other app reorders", list(a, b, c), list(b, c, a), list(a, b, c), list(b, c, a)},
		{"page reorders while another app adds", list(a, b), list(a, b, c), list(b, a), list(b, a, c)},
		{"other app reorders while the page adds", list(a, b), list(b, a), list(a, b, c), list(b, a, c)},
		{"page inserts in the middle while another app adds", list(a, b), list(a, b, `{"id":"d","name":"D"}`), list(a, c, b), list(a, c, b, `{"id":"d","name":"D"}`)},
		{"no base (first save): nothing on disk is removed", "", list(a, b), list(c), list(a, b, c)},
		{"no base: the page's version of a common item is kept", "", list(`{"id":"a","name":"disk"}`), list(`{"id":"a","name":"page"}`), list(`{"id":"a","name":"page"}`)},
		{"nothing on disk yet", "", "", list(a), list(a)},
		{"key order inside an item does not count as an edit", list(`{"id":"a","name":"A","port":22}`), list(`{"id":"a","name":"A","port":22,"folder":"X"}`), list(`{"port":22.0,"name":"A","id":"a"}`), list(`{"id":"a","name":"A","port":22,"folder":"X"}`)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := mergeSharedKey(key, sharedJSON(t, tc.base), sharedJSON(t, tc.theirs), sharedJSON(t, tc.ours), true)
			if err != nil {
				t.Fatal(err)
			}
			if !sharedEqual(got, sharedJSON(t, tc.want)) {
				t.Fatalf("merged %s, want %s", sharedText(t, got), tc.want)
			}
		})
	}
}

func TestSharedListMergeNewestFirstLimitAndCustomIdentity(t *testing.T) {
	recent := sharedKey{Name: "terminalRecent", Kind: sharedList, NewestFirst: true, Limit: 3}
	got, err := mergeSharedKey(recent,
		sharedJSON(t, `[{"id":"a","lastUsed":10}]`),
		sharedJSON(t, `[{"id":"b","lastUsed":40},{"id":"a","lastUsed":10},{"id":"d","lastUsed":5}]`),
		sharedJSON(t, `[{"id":"c","lastUsed":30},{"id":"a","lastUsed":20}]`), true)
	if err != nil {
		t.Fatal(err)
	}
	if want := `[{"id":"b","lastUsed":40},{"id":"c","lastUsed":30},{"id":"a","lastUsed":20}]`; sharedText(t, got) != want {
		t.Fatalf("recent list %s, want %s", sharedText(t, got), want)
	}
	names := sharedKey{Name: "terminalPromptNames", Kind: sharedList, Identity: terminalPromptNameIdentity}
	got, err = mergeSharedKey(names,
		sharedJSON(t, `[{"type":"ssh","host":"192.0.2.1","port":22,"username":"a","name":"core"}]`),
		sharedJSON(t, `[{"type":"ssh","host":"192.0.2.1","port":22,"username":"a","name":"core"},{"type":"serial","serialPort":"COM3","name":"console"}]`),
		sharedJSON(t, `[{"type":"ssh","host":"192.0.2.1","port":22,"username":"a","name":"core-1"}]`), true)
	if err != nil {
		t.Fatal(err)
	}
	if want := `[{"host":"192.0.2.1","name":"core-1","port":22,"type":"ssh","username":"a"},{"name":"console","serialPort":"COM3","type":"serial"}]`; sharedText(t, got) != want {
		t.Fatalf("prompt names %s, want %s", sharedText(t, got), want)
	}
}

func TestSharedMergeOtherKindsAndInvalidInput(t *testing.T) {
	list := sharedKey{Name: "terminalProfiles", Kind: sharedList}
	object := sharedKey{Name: "editorPreferences", Kind: sharedMap}
	value := sharedKey{Name: "terminalSavedManualOrder", Kind: sharedValue}
	counter := sharedKey{Name: "terminalNameRevision", Kind: sharedCounter}
	check := func(name string, key sharedKey, base, theirs, ours, want string) {
		t.Helper()
		got, err := mergeSharedKey(key, sharedJSON(t, base), sharedJSON(t, theirs), sharedJSON(t, ours), true)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if !sharedEqual(got, sharedJSON(t, want)) {
			t.Fatalf("%s: merged %s, want %s", name, sharedText(t, got), want)
		}
	}
	check("the page's unchanged value yields to disk", value, `false`, `true`, `false`, `true`)
	check("the page's changed value wins", value, `false`, `false`, `true`, `true`)
	check("both changed: the page wins", value, `1`, `2`, `3`, `3`)
	check("map: members merge one by one", object, `{"wordWrap":false,"fontSize":13}`, `{"wordWrap":true,"fontSize":13}`, `{"wordWrap":false,"fontSize":15}`, `{"wordWrap":true,"fontSize":15}`)
	check("map: page adds, other app adds", object, `{}`, `{"a":1}`, `{"b":2}`, `{"a":1,"b":2}`)
	check("map: page removes, other app removes", object, `{"a":1,"b":2,"c":3}`, `{"a":1,"c":3}`, `{"a":1,"b":2}`, `{"a":1}`)
	check("map: nested values compare by content", object, `{"colors":{"a":"#fff"}}`, `{"colors":{"a":"#fff"},"x":1}`, `{"colors":{"a":"#fff"}}`, `{"colors":{"a":"#fff"},"x":1}`)
	check("counter only grows", counter, `5`, `9`, `7`, `9`)
	check("counter takes the page's higher value", counter, `5`, `6`, `8`, `8`)
	check("a list without usable ids is merged as one value", list, `[{"id":"a"},{"id":"a"}]`, `[{"id":"z"}]`, `[{"id":"a"},{"id":"a"}]`, `[{"id":"z"}]`)

	// A page that leaves a key out, or sends null, removes nothing.
	for _, ours := range []any{nil} {
		for _, present := range []bool{false, true} {
			got, err := mergeSharedKey(list, sharedJSON(t, `[{"id":"a"}]`), sharedJSON(t, `[{"id":"a"},{"id":"b"}]`), ours, present)
			if err != nil || !sharedEqual(got, sharedJSON(t, `[{"id":"a"},{"id":"b"}]`)) {
				t.Fatalf("an absent key changed the list: %s %v", sharedText(t, got), err)
			}
		}
	}
	// Values of the wrong type are refused, never stored.
	for _, tc := range []struct {
		key  sharedKey
		ours string
	}{{list, `"text"`}, {list, `{"id":"a"}`}, {list, `7`}, {object, `[]`}, {object, `"text"`}, {object, `true`}} {
		if _, err := mergeSharedKey(tc.key, nil, nil, sharedJSON(t, tc.ours), true); !errors.Is(err, errSharedInvalid) {
			t.Fatalf("%s accepted %s: %v", tc.key.Name, tc.ours, err)
		}
	}
	// Data only the main window uses is never refused for its shape: it is
	// then saved as one value, the page's if the page changed it.
	mainOnly := sharedKey{Name: "graphPlotPreferences", Kind: sharedMap, MainOnly: true}
	if got, err := mergeSharedKey(mainOnly, sharedJSON(t, `["a"]`), sharedJSON(t, `["a"]`), sharedJSON(t, `["a","b"]`), true); err != nil || !sharedEqual(got, sharedJSON(t, `["a","b"]`)) {
		t.Fatalf("main-only value of an unexpected shape: %s %v", sharedText(t, got), err)
	}
	if got, err := mergeSharedKey(mainOnly, sharedJSON(t, `["a"]`), sharedJSON(t, `["z"]`), sharedJSON(t, `["a"]`), true); err != nil || !sharedEqual(got, sharedJSON(t, `["z"]`)) {
		t.Fatalf("an unchanged main-only value replaced the one on disk: %s %v", sharedText(t, got), err)
	}
	// Go-typed values (as the sanitizers return them) merge like decoded JSON.
	typed := []map[string]any{{"id": "a", "port": uint64(22)}}
	got, err := mergeSharedKey(list, sharedJSON(t, `[{"id":"a","port":22}]`), sharedJSON(t, `[{"id":"a","port":22},{"id":"b"}]`), typed, true)
	if err != nil || !sharedEqual(got, sharedJSON(t, `[{"id":"a","port":22},{"id":"b"}]`)) {
		t.Fatalf("typed list: %s %v", sharedText(t, got), err)
	}
}

func TestSharedSectionIsCanonicalAndCarriesNoSecrets(t *testing.T) {
	a := sharedSection(map[string]any{"terminalProfiles": []any{map[string]any{"id": "p", "name": "Core", "type": "ssh", "host": "192.0.2.1", "port": float64(22)}}, "activePage": "terminal"})
	b := sharedSection(map[string]any{"activePage": "overview", "terminalProfiles": []map[string]any{{"port": 22, "host": "192.0.2.1", "type": "ssh", "name": "Core", "id": "p"}}})
	if _, leaked := a["activePage"]; leaked || sharedRevision(a) != sharedRevision(b) || !reflect.DeepEqual(a, b) {
		t.Fatalf("the shared section must not depend on window state or value types: %v %v", a, b)
	}
	c := sharedSection(map[string]any{"terminalProfiles": []any{map[string]any{"id": "p", "name": "Edge", "type": "ssh", "host": "192.0.2.1", "port": float64(22)}}})
	if sharedRevision(a) == sharedRevision(c) || len(sharedRevision(a)) != 32 {
		t.Fatal("a changed profile must change the revision")
	}
	// A setting is part of the shared section, and so of the revision.
	d := sharedSection(map[string]any{"theme": "light", "terminalFontSize": float64(15), "terminalAppearance": map[string]any{"cursorStyle": "bar"}})
	if d["theme"] != "light" || d["terminalFontSize"] != float64(15) || d["terminalAppearance"] == nil || sharedRevision(d) == sharedRevision(sharedSection(map[string]any{"theme": "dark"})) {
		t.Fatalf("settings are shared: %v", d)
	}
	// Secrets and unknown fields never enter the shared section.
	e := sharedSection(map[string]any{"terminalProfiles": []any{map[string]any{"id": "p", "name": "Core", "password": "secret-marker", "privateKey": "secret-marker"}}, "ftpProfiles": []any{map[string]any{"id": "f", "name": "Files", "host": "files.test", "username": "u", "port": float64(21), "password": "secret-marker"}}, "ftpRecent": []any{map[string]any{"id": "g", "name": "Files", "host": "files.test", "username": "u", "port": float64(21), "password": "secret-marker"}}})
	if strings.Contains(sharedText(t, e), "secret-marker") {
		t.Fatal("a secret reached the shared section")
	}
	// A standalone app's section leaves out what only the main window uses.
	standalone := sharedSectionOf(map[string]any{"theme": "dark", "profiles": []any{map[string]any{"id": "t", "name": "Gateway", "target": "192.0.2.1"}}}, sharedKeysFor(true))
	if _, present := standalone["profiles"]; present || standalone["theme"] != "dark" {
		t.Fatalf("a standalone app's shared section: %v", standalone)
	}
}

// Entries without an id, or with an id that another entry of the list has
// too, must never make the merge fall back to "the page's whole list wins":
// that loses what another app added. Such entries are told apart by their
// content instead.
func TestSharedListMergeNeverLosesAnEntryBecauseOfABadID(t *testing.T) {
	key := sharedKey{Name: "terminalProfiles", Kind: sharedList}
	ids := func(value any) []string {
		out := []string{}
		for _, entry := range value.([]any) {
			row := entry.(map[string]any)
			out = append(out, fmt.Sprint(row["id"], "/", row["name"]))
		}
		sort.Strings(out)
		return out
	}
	list := func(text string) any { return sharedJSON(t, text) }
	// Two entries on disk share an id. One app adds x, then another, which
	// had not heard of x, adds y.
	disk := list(`[{"id":"dup","name":"One"},{"id":"dup","name":"Two"},{"id":"","name":"Nameless"},{"name":"No id at all"}]`)
	afterA, err := mergeSharedList(key, disk, disk, list(`[{"id":"dup","name":"One"},{"id":"dup","name":"Two"},{"id":"","name":"Nameless"},{"name":"No id at all"},{"id":"x","name":"X"}]`))
	if err != nil {
		t.Fatal(err)
	}
	afterB, err := mergeSharedList(key, disk, afterA, list(`[{"id":"dup","name":"One"},{"id":"dup","name":"Two"},{"id":"","name":"Nameless"},{"name":"No id at all"},{"id":"y","name":"Y"}]`))
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"/Nameless", "<nil>/No id at all", "dup/One", "dup/Two", "x/X", "y/Y"}
	if got := ids(afterB); strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("after two apps each added an entry to a list with bad ids: %v", got)
	}
	// The page removes one of the two that share an id; another app renamed
	// the nameless one meanwhile. Both changes hold.
	theirs := list(`[{"id":"dup","name":"One"},{"id":"dup","name":"Two"},{"id":"","name":"Renamed elsewhere"},{"name":"No id at all"}]`)
	merged, err := mergeSharedList(key, disk, theirs, list(`[{"id":"dup","name":"One"},{"id":"","name":"Nameless"},{"name":"No id at all"}]`))
	if err != nil {
		t.Fatal(err)
	}
	if got := ids(merged); strings.Join(got, ",") != "/Renamed elsewhere,<nil>/No id at all,dup/One" {
		t.Fatalf("a removal next to another app's change, with bad ids: %v", got)
	}
	// Exact duplicates are kept as often as they occur.
	twice := list(`[{"id":"a","name":"A"},{"id":"a","name":"A"}]`)
	merged, err = mergeSharedList(key, twice, twice, list(`[{"id":"a","name":"A"},{"id":"a","name":"A"},{"id":"b","name":"B"}]`))
	if err != nil || len(merged.([]any)) != 3 {
		t.Fatalf("exact duplicates: %v %v", merged, err)
	}
}

// Randomized: whatever two sides do to a list, including lists with repeated
// and empty ids, nothing either side added or changed is lost, an unchanged
// side never overrules a change, and nothing that both sides lack appears.
func TestSharedListMergeRandomized(t *testing.T) {
	rng := rand.New(rand.NewSource(7))
	key := sharedKey{Name: "terminalProfiles", Kind: sharedList}
	item := func(id string, v int) map[string]any { return map[string]any{"id": id, "v": float64(v)} }
	content := func(entry any) string { raw, _ := json.Marshal(entry); return string(raw) }
	for round := 0; round < 20000; round++ {
		bad := round%4 == 0 // every fourth round uses repeated and empty ids
		base := []any{}
		for i := 0; i < rng.Intn(6); i++ {
			id := fmt.Sprintf("b%d", i)
			if bad && rng.Intn(3) == 0 {
				id = []string{"", "same"}[rng.Intn(2)]
			}
			base = append(base, item(id, 100+i))
		}
		mutate := func(tag string, offset int) []any {
			out := []any{}
			for _, entry := range base {
				row := entry.(map[string]any)
				switch rng.Intn(5) {
				case 0: // removed
				case 1:
					out = append(out, item(row["id"].(string), offset+rng.Intn(1000)))
				default:
					out = append(out, row)
				}
			}
			for i := 0; i < rng.Intn(3); i++ {
				id := fmt.Sprintf("%s%d", tag, i)
				if bad && rng.Intn(3) == 0 {
					id = []string{"", "same"}[rng.Intn(2)]
				}
				at := rng.Intn(len(out) + 1)
				out = append(out[:at], append([]any{item(id, offset+1000+rng.Intn(1000))}, out[at:]...)...)
			}
			if rng.Intn(4) == 0 {
				rng.Shuffle(len(out), func(i, j int) { out[i], out[j] = out[j], out[i] })
			}
			return out
		}
		ours, theirs := mutate("o", 10000), mutate("t", 20000)
		merged, err := mergeSharedList(key, base, theirs, ours)
		if err != nil {
			t.Fatal(err)
		}
		result := merged.([]any)
		count := func(list []any) map[string]int {
			out := map[string]int{}
			for _, entry := range list {
				out[content(entry)]++
			}
			return out
		}
		r, b, o, th := count(result), count(base), count(ours), count(theirs)
		describe := func() string {
			return fmt.Sprintf("round %d\nbase   %s\nours   %s\ntheirs %s\nresult %s", round, content(base), content(ours), content(theirs), content(result))
		}
		// Everything a side has that the base did not have (added, or changed
		// into) is in the result.
		for text, n := range o {
			if n > b[text] && r[text] < 1 {
				t.Fatalf("something the page added or changed is lost: %s\n%s", text, describe())
			}
		}
		for text, n := range th {
			if n <= b[text] || r[text] >= 1 {
				continue
			}
			// The other app's version may only give way to a change the page
			// made to the same entry (same id, and the page's differs from base).
			var row map[string]any
			_ = json.Unmarshal([]byte(text), &row)
			overruled := false
			for _, entry := range ours {
				mine := entry.(map[string]any)
				if mine["id"] == row["id"] && row["id"] != "" && b[content(mine)] == 0 {
					overruled = true
				}
			}
			if !overruled {
				t.Fatalf("something another app added or changed is lost: %s\n%s", text, describe())
			}
		}
		// Nothing appears that neither side has.
		for text := range r {
			if o[text] == 0 && th[text] == 0 {
				t.Fatalf("an entry appeared that neither side has: %s\n%s", text, describe())
			}
		}
		// With clean ids the result has no repeated id.
		if !bad {
			seen := map[string]bool{}
			for _, entry := range result {
				id := entry.(map[string]any)["id"].(string)
				if seen[id] {
					t.Fatalf("an id is repeated in the result: %s\n%s", id, describe())
				}
				seen[id] = true
			}
		}
	}
}
