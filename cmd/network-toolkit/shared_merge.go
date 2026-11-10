package main

// Three-way merge for the settings every app of one data folder shares
// (docs/SHARED_APP_DATA.md). Each page keeps its whole state in memory and
// writes all of it back, so a write is never taken as the new truth: it is
// compared with what that page last received (base) and applied on top of
// what is on disk now (theirs). A page that has not heard of another app's
// new profile therefore cannot remove it.
import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strconv"
)

type sharedKeyKind int

const (
	// A list of objects, each with an identity (normally its string "id").
	sharedList sharedKeyKind = iota
	// An object whose members are merged one by one.
	sharedMap
	// Any other value: the page's value is taken only if the page changed it.
	sharedValue
	// A number that only grows (kept by the engine, not by pages).
	sharedCounter
)

// One row of the shared-key table (sharedDataKeys in shared_data.go).
type sharedKey struct {
	Name string
	Kind sharedKeyKind
	// Identity names a list item. Nil means its nonempty string "id".
	Identity func(map[string]any) string
	// Live keys are refreshed in running pages when another app changes them.
	// The other keys are read when a page starts.
	Live bool
	// MainOnly keys are used by the main window only. A standalone app
	// neither receives nor writes them.
	MainOnly bool
	// Newest-first lists are ordered by their "lastUsed" time after a merge.
	NewestFirst bool
	// Limit makes a list a rolling one (recent connections, learned names):
	// only this many entries are kept and the oldest leave on their own.
	Limit int
	// MaxSaved makes a list a saved one (saved sessions, saved targets): the
	// user may add entries up to this number. Such a list is never shortened
	// by loading, saving or a sanitizer; an entry that would take it past the
	// number is refused and the page says so (web/assets/shared-data.js holds
	// the same numbers, checked by scripts/shared-list-limits.test.cjs).
	MaxSaved int
}

var errSharedInvalid = errors.New("Saved connections and shared settings are invalid.")

func sharedItemID(item map[string]any) string {
	id, _ := item["id"].(string)
	return id
}

// sharedCanonical returns the value as produced by encoding/json, so that
// numbers and nested values compare the same wherever they came from.
func sharedCanonical(value any) (any, []byte, error) {
	raw, err := json.Marshal(value)
	if err != nil {
		return nil, nil, err
	}
	var out any
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, nil, err
	}
	// Marshal again: map keys are sorted, so equal values give equal bytes.
	raw, err = json.Marshal(out)
	return out, raw, err
}

func sharedEqual(a, b any) bool {
	_, x, errA := sharedCanonical(a)
	_, y, errB := sharedCanonical(b)
	return errA == nil && errB == nil && bytes.Equal(x, y)
}

type sharedListView struct {
	order []string
	items map[string]any
	raw   map[string][]byte
	// Keys that stand for an entry's content, because the entry has no id of
	// its own: its id is empty, another entry of the list has the same id, or
	// it is not an object.
	byContent map[string]bool
}

// sharedRepeatedIDs returns the identities that more than one entry of one of
// the lists has. Such an identity cannot tell entries apart in any of the
// lists that are merged with it, so all of them use the same answer.
func sharedRepeatedIDs(identity func(map[string]any) string, values ...any) map[string]bool {
	if identity == nil {
		identity = sharedItemID
	}
	repeated := map[string]bool{}
	for _, value := range values {
		canonical, _, err := sharedCanonical(value)
		list, ok := canonical.([]any)
		if err != nil || !ok {
			continue
		}
		uses := map[string]int{}
		for _, entry := range list {
			if item, ok := entry.(map[string]any); ok {
				if id := identity(item); id != "" {
					if uses[id]++; uses[id] > 1 {
						repeated[id] = true
					}
				}
			}
		}
	}
	return repeated
}

// sharedListOf indexes a list by the identity of its entries. An entry
// without a usable identity (none, or one in repeated) is known by what it
// holds instead, so a list with such entries is still merged entry by entry:
// falling back to "the whole list of the page wins" would lose what another
// app added. It reports false only for a value that is not a list.
func sharedListOf(value any, identity func(map[string]any) string, repeated map[string]bool) (sharedListView, bool, error) {
	view := sharedListView{items: map[string]any{}, raw: map[string][]byte{}, byContent: map[string]bool{}}
	if value == nil {
		return view, true, nil
	}
	canonical, _, err := sharedCanonical(value)
	if err != nil {
		return view, false, err
	}
	list, ok := canonical.([]any)
	if !ok {
		return view, false, errSharedInvalid
	}
	if identity == nil {
		identity = sharedItemID
	}
	copies := map[string]int{}
	for _, entry := range list {
		raw, err := json.Marshal(entry)
		if err != nil {
			return view, false, err
		}
		key := ""
		if item, ok := entry.(map[string]any); ok {
			key = identity(item)
		}
		if key == "" || repeated[key] {
			// Equal entries are told apart by their position among their equals.
			digest := sha256.Sum256(raw)
			content := "\x00\x00content:" + hex.EncodeToString(digest[:16])
			key = content + "#" + strconv.Itoa(copies[content])
			copies[content]++
			view.byContent[key] = true
		}
		view.order = append(view.order, key)
		view.items[key], view.raw[key] = entry, raw
	}
	return view, true, nil
}

func (v sharedListView) same(id string, other sharedListView) bool {
	a, okA := v.raw[id]
	b, okB := other.raw[id]
	return okA && okB && bytes.Equal(a, b)
}

// sharedInsertMissing adds the ids of extra (in extra's order) that keep[id]
// allows and that are not in the result yet. An id that came last in extra
// goes to the end; any other goes right after the nearest earlier id of extra
// that is in the result (or to the front).
func sharedInsertMissing(result, extra []string, keep map[string]bool) []string {
	present := map[string]bool{}
	for _, id := range result {
		present[id] = true
	}
	anchor := ""
	for index, id := range extra {
		if present[id] {
			anchor = id
			continue
		}
		if !keep[id] {
			continue
		}
		followed := false
		for _, later := range extra[index+1:] {
			if present[later] {
				followed = true
				break
			}
		}
		at := len(result)
		if followed {
			at = 0
			if anchor != "" {
				for i, existing := range result {
					if existing == anchor {
						at = i + 1
						break
					}
				}
			}
		}
		result = append(result, "")
		copy(result[at+1:], result[at:])
		result[at] = id
		present[id], anchor = true, id
	}
	return result
}

func sharedSubsequence(order []string, keep map[string]bool) []string {
	out := []string{}
	for _, id := range order {
		if keep[id] {
			out = append(out, id)
		}
	}
	return out
}

func sharedSameOrder(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// mergeSharedList merges a list three-way by item identity:
//   - an item the page added, removed or changed is added, removed or changed
//     on top of the list on disk;
//   - an item changed on both sides keeps the page's version;
//   - a change always beats a removal, so an edit is never dropped;
//   - the page's order is used when the page reordered its items, otherwise
//     the order on disk; new items stay next to their neighbours.
func mergeSharedList(key sharedKey, base, theirs, ours any) (any, error) {
	repeated := sharedRepeatedIDs(key.Identity, base, theirs, ours)
	b, okB, err := sharedListOf(base, key.Identity, repeated)
	if err != nil {
		// An unreadable base is treated as "nothing was known before".
		b, okB = sharedListView{items: map[string]any{}, raw: map[string][]byte{}, byContent: map[string]bool{}}, true
	}
	t, okT, err := sharedListOf(theirs, key.Identity, repeated)
	if err != nil {
		// What is on disk cannot be merged item by item; the page's list wins
		// only if the page changed it.
		okT = false
	}
	o, okO, err := sharedListOf(ours, key.Identity, repeated)
	if err != nil {
		return nil, err
	}
	if !okB || !okT || !okO {
		return mergeSharedPlain(base, theirs, ours), nil
	}
	final := map[string]any{}
	for _, id := range o.order {
		_, inBase := b.items[id]
		_, inTheirs := t.items[id]
		switch {
		case inTheirs && inBase && o.same(id, b):
			final[id] = t.items[id] // The page did not touch it.
		case inTheirs:
			final[id] = o.items[id] // Added or changed by the page.
		case inBase && o.same(id, b):
			// Removed by another app and untouched here: stays removed.
		default:
			final[id] = o.items[id] // New here, or changed here after its removal elsewhere.
		}
	}
	for _, id := range t.order {
		if _, inOurs := o.items[id]; inOurs {
			continue
		}
		if _, inBase := b.items[id]; !inBase || !t.same(id, b) {
			final[id] = t.items[id] // Added by another app, or changed there after its removal here.
		}
	}
	keep := map[string]bool{}
	for id := range final {
		keep[id] = true
	}
	common := map[string]bool{}
	for id := range keep {
		if _, inBase := b.items[id]; inBase {
			if _, inOurs := o.items[id]; inOurs {
				common[id] = true
			}
		}
	}
	var order []string
	if sharedSameOrder(sharedSubsequence(o.order, common), sharedSubsequence(b.order, common)) {
		order = sharedInsertMissing(sharedSubsequence(t.order, keep), o.order, keep)
	} else {
		order = sharedInsertMissing(sharedSubsequence(o.order, keep), t.order, keep)
	}
	result := make([]any, 0, len(order))
	for _, id := range order {
		result = append(result, final[id])
	}
	if key.NewestFirst {
		sort.SliceStable(result, func(i, j int) bool { return sharedLastUsed(result[i]) > sharedLastUsed(result[j]) })
	}
	if key.Limit > 0 && len(result) > key.Limit {
		result = result[:key.Limit]
	}
	return result, nil
}

func sharedLastUsed(item any) float64 {
	row, _ := item.(map[string]any)
	used, _ := row["lastUsed"].(float64)
	return used
}

// mergeSharedMap merges an object member by member with the same rules as a
// list item: the page's change wins, and a change beats a removal.
func mergeSharedMap(base, theirs, ours any) (any, error) {
	object := func(value any) (map[string]any, bool, error) {
		if value == nil {
			return map[string]any{}, true, nil
		}
		canonical, _, err := sharedCanonical(value)
		if err != nil {
			return nil, false, err
		}
		m, ok := canonical.(map[string]any)
		return m, ok, nil
	}
	o, okO, err := object(ours)
	if err != nil {
		return nil, err
	}
	if !okO {
		return nil, errSharedInvalid
	}
	b, okB, errB := object(base)
	if errB != nil || !okB {
		b = map[string]any{}
	}
	t, okT, errT := object(theirs)
	if errT != nil || !okT {
		return mergeSharedPlain(base, theirs, ours), nil
	}
	result := map[string]any{}
	for name, value := range o {
		before, inBase := b[name]
		current, inTheirs := t[name]
		switch {
		case inTheirs && inBase && sharedEqual(value, before):
			result[name] = current
		case inTheirs:
			result[name] = value
		case inBase && sharedEqual(value, before):
		default:
			result[name] = value
		}
	}
	for name, current := range t {
		if _, inOurs := o[name]; inOurs {
			continue
		}
		if before, inBase := b[name]; !inBase || !sharedEqual(current, before) {
			result[name] = current
		}
	}
	return result, nil
}

// sharedRefuseOverflow keeps a saved list within its limit without removing
// anything that is saved: entries the page adds (they are not in theirs, the
// list on disk) are taken out again, last added first, until the list fits or
// none is left. It returns the list and the entries that were refused. A list
// that is over its limit already stays as it is.
func sharedRefuseOverflow(key sharedKey, theirs, merged any) (any, []map[string]string) {
	list, ok := merged.([]any)
	if key.MaxSaved <= 0 || !ok || len(list) <= key.MaxSaved {
		return merged, nil
	}
	identity := key.Identity
	if identity == nil {
		identity = sharedItemID
	}
	saved := map[string]bool{}
	if existing, ok := theirs.([]any); ok {
		for _, entry := range existing {
			if item, ok := entry.(map[string]any); ok {
				saved[identity(item)] = true
			}
		}
	}
	kept := append([]any{}, list...)
	var refused []map[string]string
	for index := len(kept) - 1; index >= 0 && len(kept) > key.MaxSaved; index-- {
		item, ok := kept[index].(map[string]any)
		if !ok || saved[identity(item)] {
			continue
		}
		name, _ := item["name"].(string)
		refused = append(refused, map[string]string{"id": identity(item), "name": name})
		kept = append(kept[:index], kept[index+1:]...)
	}
	return kept, refused
}

// mergeSharedPlain takes the page's value only when the page changed it.
func mergeSharedPlain(base, theirs, ours any) any {
	if sharedEqual(ours, base) {
		return theirs
	}
	return ours
}

func sharedNumber(value any) (float64, bool) {
	switch n := value.(type) {
	case float64:
		return n, true
	case int:
		return float64(n), true
	case uint64:
		return float64(n), true
	case json.Number:
		f, err := strconv.ParseFloat(string(n), 64)
		return f, err == nil
	}
	return 0, false
}

// mergeSharedKey merges one key. present reports whether the page sent the
// key at all: a page that leaves a key out never removes it.
func mergeSharedKey(key sharedKey, base, theirs, ours any, present bool) (any, error) {
	if !present || ours == nil {
		return theirs, nil
	}
	switch key.Kind {
	case sharedList:
		if _, ok := ours.([]any); !ok {
			if _, typed := ours.([]map[string]any); !typed {
				if key.MainOnly {
					// The main window's own data in a shape this table does not
					// expect is saved as one value rather than refused.
					return mergeSharedPlain(base, theirs, ours), nil
				}
				return nil, fmt.Errorf("%w %s must be a list.", errSharedInvalid, key.Name)
			}
		}
		return mergeSharedList(key, base, theirs, ours)
	case sharedMap:
		if _, ok := ours.(map[string]any); !ok {
			if key.MainOnly {
				return mergeSharedPlain(base, theirs, ours), nil
			}
			return nil, fmt.Errorf("%w %s must be an object.", errSharedInvalid, key.Name)
		}
		return mergeSharedMap(base, theirs, ours)
	case sharedCounter:
		highest, found := 0.0, false
		for _, value := range []any{theirs, ours} {
			if n, ok := sharedNumber(value); ok && (!found || n > highest) {
				highest, found = n, true
			}
		}
		if !found {
			return theirs, nil
		}
		return highest, nil
	default:
		return mergeSharedPlain(base, theirs, ours), nil
	}
}
