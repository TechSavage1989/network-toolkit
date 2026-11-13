package texttools

import (
	"context"
	"errors"
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
