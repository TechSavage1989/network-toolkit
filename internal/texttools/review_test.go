package texttools

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"
)

// Fixes from the review of the text tools (v1.21.53). Each test failed
// before its fix.

// insertCounter counts the new text a tool writes, keeps the largest single
// insertion, and can run something the first time the tool writes.
type insertCounter struct {
	inserted, largest int64
	first             func()
}

func (c *insertCounter) Keep(int64) error { return nil }
func (c *insertCounter) Drop(int64) error { return nil }
func (c *insertCounter) Insert(p []byte) error {
	if c.first != nil {
		c.first()
		c.first = nil
	}
	c.inserted += int64(len(p))
	c.largest = max(c.largest, int64(len(p)))
	return nil
}

// nested is depth levels of JSON arrays, chains times.
func nested(depth, chains int) string {
	return strings.Repeat(strings.Repeat("[", depth)+strings.Repeat("]", depth), chains)
}

// Pretty-printing writes one indent per level per line: deep nesting is
// refused, growth is bounded, and Cancel stops a tool that writes much more
// than it reads (8 KB of brackets gave 64 MB).
func TestPrettyPrintIsBounded(t *testing.T) {
	for _, c := range []struct{ id, in, want string }{
		{"json-pretty", nested(4000, 1), "deeper than 512 levels"},
		{"xml-pretty", strings.Repeat("<a>", 4000) + strings.Repeat("</a>", 4000), "deeper than 512 levels"},
		{"json-pretty", "[" + strings.Repeat(nested(500, 1)+",", 300) + "1]", "more than 64 times larger"},
	} {
		counter := &insertCounter{}
		_, err := Run(context.Background(), c.id, Options{Indent: 16}, BytesSource(c.in), counter, Env{})
		if err == nil || !strings.Contains(err.Error(), c.want) {
			t.Fatalf("%s of %d bytes: %v (%d bytes written)", c.id, len(c.in), err, counter.inserted)
		}
		if limit := 64*int64(len(c.in)) + 64<<20; counter.inserted > limit {
			t.Fatalf("%s wrote %d bytes, more than %d", c.id, counter.inserted, limit)
		}
	}
}

// Pretty-printing within the depth limit still formats, and Cancel stops a
// tool while it writes, long before it has read 256 KiB.
func TestCancelStopsAToolThatWrites(t *testing.T) {
	if got, _, err := run(t, "json-pretty", Options{Indent: 1}, nested(511, 1)); err != nil || strings.Count(got, "\n") != 1020 {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	in := "[" + strings.TrimSuffix(strings.Repeat(nested(500, 1)+",", 50), ",") + "]"
	counter := &insertCounter{first: cancel}
	_, err := Run(ctx, "json-pretty", Options{Indent: 4}, BytesSource(in), counter, Env{})
	if !errors.Is(err, context.Canceled) || counter.inserted > 1<<20 {
		t.Fatalf("after Cancel: %v, %d bytes written", err, counter.inserted)
	}
}

// A long run of escapes is decoded in blocks, not held whole until the run
// ends (6 MiB of %C3%A9 gave one 2 MiB insertion).
func TestDecodersFlushLongRunsOfEscapes(t *testing.T) {
	for _, c := range []struct{ id, unit, want string }{
		{"url-decode", "%C3%A9", "é"},
		{"url-decode-form", "%41+", "A "},
		{"unescape", `\xC3\xA9`, "é"},
		{"unescape", `é\n`, "é\n"},
	} {
		in := strings.Repeat(c.unit, (6<<20)/len(c.unit))
		counter := &insertCounter{}
		result, err := Run(context.Background(), c.id, Options{}, BytesSource(in), counter, Env{})
		if err != nil || !result.Changed {
			t.Fatal(c.id, err)
		}
		if counter.largest > 160<<10 {
			t.Errorf("%s of %q: one insertion of %d bytes", c.id, c.unit, counter.largest)
		}
		// The blocks together are the whole decoded text.
		if got, _, err := run(t, c.id, Options{}, in); err != nil || got != strings.Repeat(c.want, (6<<20)/len(c.unit)) {
			t.Errorf("%s: %v", c.id, err)
		}
	}
	// A character split between two blocks is still decoded whole, and a
	// carriage return at the end of a block still joins the line break after it.
	in := strings.Repeat("%41", 32<<10) + "%0D" + "\n" + "%C3" + "%A9"
	if got, _, err := run(t, "url-decode", Options{}, in); err != nil || got != strings.Repeat("A", 32<<10)+"\né" {
		t.Fatal(err, len(got))
	}
}

// HTML entities decode takes only complete references: a name that only
// starts with a legacy entity name (&not, &amp, &copy) stays as written.
func TestHTMLDecodeTakesOnlyCompleteReferences(t *testing.T) {
	for in, want := range map[string]string{
		"&notit;":                                "&notit;",
		"&ampfoo;":                               "&ampfoo;",
		"&copy2;":                                "&copy2;",
		"&#x41g;":                                "&#x41g;",
		"&not; &amp; &copy; &semi; &#65; &#x42;": "¬ & © ; A B",
	} {
		got, _, err := run(t, "html-decode", Options{}, in)
		if err != nil || got != want {
			t.Errorf("%q: %q, want %q (%v)", in, got, want, err)
		}
	}
}

// A sort whose scratch folder refuses a new run file part way through a
// merge pass leaves no run file behind (a merged file of that pass leaked).
func TestSortLeavesNoRunFileWhenOneCannotBeCreated(t *testing.T) {
	var lines []string
	for i := 0; i < 3000; i++ {
		lines = append(lines, fmt.Sprintf("host%04d.example.com", (i*7919)%3000))
	}
	input := strings.Join(lines, "\n")
	budget, width, create := runBudget, mergeWidth, createRun
	defer func() { runBudget, mergeWidth, createRun = budget, width, create }()
	runBudget = 2048
	// Count the run files the spills make, with one merge reading them all.
	spills := 0
	createRun = func(dir string) (*os.File, error) { spills++; return create(dir) }
	mergeWidth = 1 << 20
	if _, _, err := run(t, "sort-asc", Options{}, input); err != nil {
		t.Fatal(err)
	}
	// Now merge two at a time, and refuse the second merged file of the
	// first pass.
	mergeWidth = 2
	calls := 0
	createRun = func(dir string) (*os.File, error) {
		if calls++; calls == spills+2 {
			return nil, errors.New("the scratch folder is full")
		}
		return create(dir)
	}
	dir := t.TempDir()
	_, err := Run(context.Background(), "sort-asc", Options{}, BytesSource(input), &Output{In: []byte(input)}, Env{Dir: dir})
	if err == nil || !strings.Contains(err.Error(), "full") {
		t.Fatalf("spills %d, calls %d: %v", spills, calls, err)
	}
	if entries, _ := os.ReadDir(dir); len(entries) != 0 {
		t.Fatalf("%d run files left behind", len(entries))
	}
}

// The result of a tool over page-held text is held in memory and sent to the
// page: it stops at its bound.
func TestPageHeldResultIsBounded(t *testing.T) {
	in := "[" + strings.TrimSuffix(strings.Repeat(nested(500, 1)+",", 50), ",") + "]"
	out := &Output{In: []byte(in)}
	_, err := Run(context.Background(), "json-pretty", Options{Indent: 4}, BytesSource(in), out, Env{MaxOutput: 4 << 20})
	if !errors.Is(err, ErrTooLarge) || len(out.Out) > 5<<20 {
		t.Fatalf("bounded output: %v, %d bytes", err, len(out.Out))
	}
}
