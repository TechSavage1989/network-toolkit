package texttools

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"unicode/utf8"
)

// run applies a tool to page-held text and checks that its edits account for
// every byte of the input exactly once.
func run(t *testing.T, id string, opts Options, input string) (string, Result, error) {
	t.Helper()
	out := &Output{In: []byte(input)}
	result, err := Run(context.Background(), id, opts, BytesSource(input), out, Env{Dir: t.TempDir()})
	if err != nil {
		return "", result, err
	}
	if !result.Changed {
		return input, result, nil
	}
	if e := out.Done(); e != nil {
		t.Fatalf("%s: %v", id, e)
	}
	if !utf8.Valid(out.Out) || bytes.ContainsAny(out.Out, "\x00\r") {
		t.Fatalf("%s produced text the editor cannot hold: %q", id, out.Out)
	}
	return string(out.Out), result, nil
}

type toolCase struct {
	id    string
	opts  Options
	in    string
	want  string // the result; "" with err set
	err   string // part of the error message
	same  bool   // the tool changes nothing
	count int64  // -1 to skip the check
}

func TestTools(t *testing.T) {
	b64 := func(s string) string { return base64.StdEncoding.EncodeToString([]byte(s)) }
	cases := []toolCase{
		// Lines
		{id: "trim-trailing", in: "a  \nb\t\n c \n", want: "a\nb\n c\n", count: 3},
		{id: "trim-trailing", in: "a\nb", same: true},
		{id: "trim-trailing", in: "x   ", want: "x", count: 1},
		{id: "trim-both", in: "  a  \n\tb\nc", want: "a\nb\nc", count: 2},
		{id: "trim-both", in: "   \n", want: "\n", count: 1},
		{id: "remove-empty", in: "a\n\n  \nb\n", want: "a\nb\n", count: 2},
		{id: "remove-empty", in: "\n\na", want: "a", count: 2},
		{id: "remove-empty", in: "a\n  ", want: "a", count: 1},
		{id: "remove-empty", in: "a\n\n\t", want: "a", count: 2},
		{id: "remove-empty", in: "a\n\n", want: "a\n", count: 1},
		{id: "remove-empty", in: " \n \n", want: "", count: 2},
		{id: "remove-empty", in: "a\nb", same: true},
		{id: "split", opts: Options{Separator: "&"}, in: "a=1&b=2&c=3", want: "a=1\nb=2\nc=3", count: 2},
		{id: "split", opts: Options{Separator: "&"}, in: "a=1&b=2&", want: "a=1\nb=2\n", count: 2},
		{id: "split", opts: Options{Separator: "&", KeepSeparator: true}, in: "a=1&b=2&", want: "a=1&\nb=2&\n", count: 2},
		{id: "split", opts: Options{Separator: "aab"}, in: "aaab aab", want: "a\n \n", count: 2},
		{id: "split", opts: Options{Separator: "&&"}, in: "a&&&b", want: "a\n&b", count: 1},
		{id: "split", opts: Options{Separator: ";"}, in: "no separator", same: true},
		{id: "split", opts: Options{Separator: ""}, in: "x", err: "Enter the text"},
		{id: "split", opts: Options{Separator: "é"}, in: "aébéc", want: "a\nb\nc", count: 2},
		{id: "join", opts: Options{Separator: " "}, in: "a\nb\nc", want: "a b c", count: 2},
		{id: "join", opts: Options{Separator: ", "}, in: "a\nb\n", want: "a, b\n", count: 1},
		{id: "join", opts: Options{Separator: ""}, in: "a\n\nb", want: "ab", count: 2},
		{id: "join", opts: Options{Separator: " "}, in: "one line", same: true},
		{id: "tabs-to-spaces", opts: Options{Indent: 4}, in: "\ta\tb\nab\tc", want: "    a   b\nab  c", count: 3},
		{id: "tabs-to-spaces", opts: Options{Indent: 0}, in: "\tx", want: "    x", count: 1},
		{id: "spaces-to-tabs", opts: Options{Indent: 4}, in: "        a\n     b\n\tc\n  \n", want: "\t\ta\n\t b\n\tc\n  \n", count: 2},
		{id: "sort-asc", in: "item10\nItem2\nitem1\n", want: "item1\nItem2\nitem10\n", count: 3},
		{id: "sort-asc", in: "b\na", want: "a\nb", count: 2},
		{id: "sort-asc", in: "a\nB\nc", same: true},
		{id: "sort-asc", in: "B\nb\na", want: "a\nB\nb", count: 3},
		{id: "sort-desc", in: "a\nc\nb\n", want: "c\nb\na\n", count: 3},
		{id: "sort-desc", in: "v9\nv10", want: "v10\nv9", count: 2},
		{id: "reverse", in: "1\n2\n3\n", want: "3\n2\n1\n", count: 3},
		{id: "reverse", in: "1\n2\n3", want: "3\n2\n1", count: 3},
		{id: "reverse", in: "only", same: true},
		{id: "dedupe", in: "a\nb\na\nc\nb\n", want: "a\nb\nc\n", count: 2},
		{id: "dedupe", in: "a\nb\na", want: "a\nb", count: 1},
		{id: "dedupe", in: "\n\nx\n", want: "\nx\n", count: 1},
		{id: "dedupe", in: "a\nA", same: true},
		// Case
		{id: "upper", in: "abc\nÄß", want: "ABC\nÄß", count: 3},
		{id: "lower", in: "ABC Déf", want: "abc déf", count: 4},
		{id: "upper", in: "ABC", same: true},
		{id: "title", in: "hello wORLD, don't e-mail x86", want: "Hello World, Don't E-Mail X86", count: 10},
		// Encode / decode
		{id: "base64-decode", in: b64("héllo\r\nworld"), want: "héllo\nworld", count: -1},
		{id: "base64-decode", in: "aGVs\nbG8=\n", want: "hello", count: -1},
		{id: "base64-decode", in: "aGVsbG8", want: "hello", count: -1},
		{id: "base64-decode", in: "aGV*bG8=", err: "is not a Base64 character"},
		{id: "base64-decode", in: b64("a\x00b"), err: "NUL"},
		{id: "base64-decode", in: base64.StdEncoding.EncodeToString([]byte{0xff, 0xfe, 0x41}), err: "not UTF-8"},
		{id: "base64-decode", in: "   ", err: "no Base64"},
		{id: "base64-decode", in: "QUJD=", err: "too early"},
		{id: "base64-decode", in: "QQ==QQ", err: "after its padding"},
		{id: "base64-decode", in: "Pz8-", want: "??>", count: -1},
		{id: "base64-encode", in: "héllo\nworld", want: b64("héllo\nworld"), count: -1},
		{id: "url-decode", in: "a%20b%3ac+d%25", want: "a b:c+d%", count: 3},
		{id: "url-decode", in: "caf%C3%A9", want: "café", count: 2},
		{id: "url-decode", in: "line%0D%0Anext%0D\nend", want: "line\nnext\nend", count: 3},
		{id: "url-decode", in: "100%", err: "two hexadecimal digits"},
		{id: "url-decode", in: "%C3x", err: "not UTF-8"},
		{id: "url-decode", in: "%00", err: "NUL"},
		{id: "url-decode-form", in: "a+b%2Bc", want: "a b+c", count: 2},
		{id: "url-encode", in: "a b/é~", want: "a%20b%2F%C3%A9~", count: 3},
		{id: "hex-decode", in: "68 65:6c-6c,6F", want: "hello", count: -1},
		{id: "hex-decode", in: "0x41 0x42", want: "AB", count: -1},
		{id: "hex-decode", in: "414", err: "odd number"},
		{id: "hex-decode", in: "4g", err: "not a hexadecimal digit"},
		{id: "hex-encode", in: "Aé\n", want: "41c3a90a", count: 3},
		{id: "html-decode", in: "a &amp; b &lt;c&gt; &#233;&#x41; &nbsp;&bogus; & x", want: "a & b <c> éA \u00a0&bogus; & x", count: 6},
		{id: "html-decode", in: "x&#13;\ny", want: "x\ny", count: 1},
		{id: "html-encode", in: `<a href="x">&'</a>`, want: "&lt;a href=&quot;x&quot;&gt;&amp;&#39;&lt;/a&gt;", count: 8},
		{id: "unescape", in: `a\nb\t\"c\" \\ \x41 \u00e9 \uD83D\uDE00 \q`, want: "a\nb\t\"c\" \\ A é 😀 \\q", count: 8},
		{id: "unescape", in: `x\r\ny\rz`, want: "x\ny\nz", count: 3},
		{id: "unescape", in: `\xC3\xA9`, want: "é", count: 2},
		{id: "unescape", in: `\0`, err: "NUL"},
		{id: "unescape", in: `end\`, same: true},
		{id: "escape", in: "a\\b\n\t\"c\"\x01", want: `a\\b\n\t\"c\"\x01`, count: 6},
		// Format
		{id: "json-pretty", opts: Options{Indent: 2}, in: `{"a":[1,2,{"b":null}],"c":"x\"y","d":{},"e":[]}`, want: "{\n  \"a\": [\n    1,\n    2,\n    {\n      \"b\": null\n    }\n  ],\n  \"c\": \"x\\\"y\",\n  \"d\": {},\n  \"e\": []\n}", count: -1},
		{id: "json-pretty", opts: Options{Indent: 0}, in: "[1]\n", want: "[\n\t1\n]\n", count: -1},
		{id: "json-pretty", opts: Options{Indent: 2}, in: "{\n  \"a\": 1\n}", same: true},
		{id: "json-minify", in: "{\n  \"a\" : [ 1.5e3, -0, true ],\n  \"b\": \"\\u00e9 \"\n}\n", want: "{\"a\":[1.5e3,-0,true],\"b\":\"\\u00e9 \"}\n", count: -1},
		{id: "json-pretty", in: `{"a":}`, err: "Expected a JSON value"},
		{id: "json-pretty", in: `{"a":1,}`, err: "Expected a name"},
		{id: "json-pretty", in: `[1 2]`, err: "Expected a comma"},
		{id: "json-pretty", in: `{"a":01}`, err: "cannot start with 0"},
		{id: "json-pretty", in: "\"tab\there\"", err: "control character"},
		{id: "json-pretty", in: `[1]x`, err: "more text after"},
		{id: "json-pretty", in: `[1,`, err: "ends before"},
		{id: "json-pretty", in: ``, err: "no JSON"},
		{id: "xml-pretty", opts: Options{Indent: 2}, in: `<?xml version="1.0"?><root a="1>2"><item>text &amp; more</item><empty/><!-- note --><x></x><mixed>one<b>two</b>three</mixed></root>`,
			want: "<?xml version=\"1.0\"?>\n<root a=\"1>2\">\n  <item>text &amp; more</item>\n  <empty/>\n  <!-- note -->\n  <x></x>\n  <mixed>one\n    <b>two</b>\n    three\n  </mixed>\n</root>", count: -1},
		{id: "xml-minify", in: "<a>\n  <b> x </b>\n  <c/>\n</a>\n", want: "<a><b> x </b><c/></a>\n", count: -1},
		{id: "xml-pretty", in: "<a><![CDATA[<x>]]]></a>", want: "<a><![CDATA[<x>]]]></a>", same: true},
		{id: "xml-pretty", in: "<a><b></a>", err: "</a> does not close <b>"},
		{id: "xml-pretty", in: "<a>", err: "<a> is not closed"},
		{id: "xml-pretty", in: "text", err: "outside the XML elements"},
		{id: "xml-pretty", in: "<a><!-- x </a>", err: "comment is not closed"},
		{id: "xml-pretty", in: "", err: "no XML element"},
	}
	for i, c := range cases {
		t.Run(fmt.Sprintf("%02d-%s", i, c.id), func(t *testing.T) {
			got, result, err := run(t, c.id, c.opts, c.in)
			if c.err != "" {
				if err == nil || !strings.Contains(err.Error(), c.err) {
					t.Fatalf("%q: error %v, want one about %q (result %q)", c.in, err, c.err, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("%q: %v", c.in, err)
			}
			if c.same {
				if result.Changed || got != c.in {
					t.Fatalf("%q should not change, got %q", c.in, got)
				}
				return
			}
			if got != c.want || !result.Changed {
				t.Fatalf("%q:\n got %q (changed %t)\nwant %q", c.in, got, result.Changed, c.want)
			}
			if c.count >= 0 && result.Count != c.count {
				t.Errorf("count %d, want %d", result.Count, c.count)
			}
		})
	}
}

// The registry and the page's list of engine tools stay the same: every tool
// the page offers runs here, and every tool here is offered.
func TestToolListMatchesThePage(t *testing.T) {
	source, err := os.ReadFile(filepath.Join("..", "..", "web", "assets", "editor-text-tools.js"))
	if err != nil {
		t.Fatal(err)
	}
	var page []string
	for _, part := range strings.Split(string(source), "engine:'")[1:] {
		page = append(page, part[:strings.IndexByte(part, '\'')])
	}
	ids := IDs()
	slices.Sort(ids)
	slices.Sort(page)
	if !slices.Equal(ids, page) {
		t.Fatalf("engine tools %v\npage tools %v", ids, page)
	}
	for _, id := range ids {
		tool, _ := Lookup(id)
		marked := strings.Contains(string(source), "engine:'"+id+"',lines:true")
		if tool.Lines != marked {
			t.Errorf("%s: whole lines in the engine %t, on the page %t", id, tool.Lines, marked)
		}
	}
}

// settingsLine is a fictional one-line settings file: key=value& settings
// with %XX escapes, as one line of Base64. It is what the editor guide's
// recipe (Base64 decode, Split at text &, URL decode) makes readable.
func settingsLine(settings int) (encoded, plain string) {
	var b strings.Builder
	for i := 0; i < settings; i++ {
		n := fmt.Sprint(i)
		b.WriteString("name_" + n + "=Office%20" + n + "&")
		b.WriteString("address_" + n + "=192.0.2." + fmt.Sprint(i%250) + "&")
		b.WriteString("note_" + n + "=a+b%3Ac&")
		b.WriteString("site_" + n + "=www.example.com%2Fpath&")
	}
	return base64.StdEncoding.EncodeToString([]byte(b.String())), b.String()
}

// A multi-megabyte file on one line: the recipe's three steps, each over the
// whole of it.
func TestLargeSingleLineInputs(t *testing.T) {
	encoded, plain := settingsLine(12000)
	if len(encoded) < 1500000 {
		t.Fatalf("the fixture is only %d bytes", len(encoded))
	}
	got, _, err := run(t, "base64-decode", Options{}, encoded)
	if err != nil || got != plain {
		t.Fatalf("decoding the whole line: %v (%d bytes, want %d)", err, len(got), len(plain))
	}
	lines, result, err := run(t, "split", Options{Separator: "&"}, plain)
	if err != nil || result.Count != 48000 || strings.Count(lines, "\n") != 48000 || strings.Contains(lines, "&") {
		t.Fatalf("splitting: %v (%d places)", err, result.Count)
	}
	readable, _, err := run(t, "url-decode", Options{}, lines)
	if err != nil || !strings.HasPrefix(readable, "name_0=Office 0\naddress_0=192.0.2.0\nnote_0=a+b:c\nsite_0=www.example.com/path\n") || strings.Count(readable, "\n") != 48000 {
		t.Fatalf("URL decoding: %v %q", err, readable[:min(len(readable), 120)])
	}
}

// prefix is the start of s that holds units UTF-16 units.
func prefix(s string, units int64) string {
	for i, r := range s {
		if units <= 0 {
			return s[:i]
		}
		units--
		if r > 0xffff {
			units--
		}
	}
	return s
}

// A decoder that meets a character it does not decode stops and changes
// nothing; it names the character, its position, and whether the text before
// it decodes on its own. Then the same tool over just that part succeeds, and
// otherwise it fails: Decode the part before it is offered only when it works.
func TestDecodersStopAtACharacterAndOfferThePartBefore(t *testing.T) {
	b64 := func(s string) string { return base64.StdEncoding.EncodeToString([]byte(s)) }
	cases := []struct {
		id, in string
		offset int64
		before bool
		found  string
		want   string // the part before, decoded
	}{
		// A one-line file whose Base64 is followed by "&&".
		{"base64-decode", b64("a=1&b=%20&") + "&&", 16, true, "'&'", "a=1&b=%20&"},
		{"base64-decode", b64("hello") + "&&\r\n", 8, true, "'&'", "hello"},
		// Unpadded Base64 before the problem.
		{"base64-decode", strings.TrimRight(b64("hi!?"), "=") + "&&", 6, true, "'&'", "hi!?"},
		// A character in the middle: the rest after it stays as it is.
		{"base64-decode", "aGVsbG8gd29y#bGQ=", 12, true, "'#'", "hello wor"},
		{"base64-decode", "QUJD\nQU#D", 7, true, "'#'", "ABCA"},
		{"base64-decode", "QUJDé", 4, true, "'é'", "ABC"},
		{"base64-decode", "QUJD\u00a0QUJD", 4, true, `'\u00a0'`, "ABC"},
		{"base64-decode", "QUJD😀QUJD", 4, true, "'😀'", "ABC"},
		// Nothing before it, an incomplete group, or bytes that are not text.
		{"base64-decode", "&&QUJD", 0, false, "'&'", ""},
		{"base64-decode", "  &", 2, false, "'&'", ""},
		{"base64-decode", "QUJDR&", 5, false, "'&'", ""},
		{"base64-decode", base64.StdEncoding.EncodeToString([]byte{0x41, 0xc3}) + "&", 4, false, "'&'", ""},
		{"base64-decode", strings.TrimRight(b64("a\x00"), "=") + "&", 3, false, "'&'", ""},
		{"hex-decode", "68 65 6c 6c 6f;", 14, true, "';'", "hello"},
		{"hex-decode", "0x41,0x42 zz", 10, true, "'z'", "AB"},
		{"hex-decode", "41 4z", 4, false, "'z'", ""},
		{"hex-decode", "c3z", 2, false, "'z'", ""},
		{"hex-decode", "zz", 0, false, "'z'", ""},
	}
	for _, c := range cases {
		_, result, err := run(t, c.id, Options{}, c.in)
		var at *PositionError
		if !errors.As(err, &at) || result.Changed {
			t.Fatalf("%s %q: %v (changed %t)", c.id, c.in, err, result.Changed)
		}
		if at.Offset != c.offset || at.Before != c.before || !strings.HasPrefix(at.Message, c.found+" is not a") {
			t.Errorf("%s %q: %+v, want offset %d, before %t, %s", c.id, c.in, at, c.offset, c.before, c.found)
		}
		part := prefix(c.in, at.Offset)
		got, _, err := run(t, c.id, Options{}, part)
		if c.before && (err != nil || got != c.want) {
			t.Errorf("%s of the part before %q: %q, %v", c.id, part, got, err)
		}
		if !c.before && part != "" && err == nil {
			t.Errorf("%s of %q succeeded, but the part before was not offered", c.id, part)
		}
	}
}

// The same over many short inputs: the part before is offered exactly when
// decoding it on its own succeeds.
func TestThePartBeforeIsOfferedExactlyWhenItDecodes(t *testing.T) {
	r := rand.New(rand.NewSource(7))
	for id, alphabet := range map[string][]string{
		"base64-decode": {"Q", "U", "J", "D", "z", "/", "_", "-", "=", " ", "\n", "&", "é", "#", "A", "8"},
		"hex-decode":    {"4", "1", "c", "3", "a", "9", "0", "x", " ", ":", "-", ",", "z", "&", "é", "00"},
	} {
		offered := 0
		for round := 0; round < 4000; round++ {
			var b strings.Builder
			for n := r.Intn(14); n >= 0; n-- {
				b.WriteString(alphabet[r.Intn(len(alphabet))])
			}
			in := b.String()
			_, _, err := run(t, id, Options{}, in)
			var at *PositionError
			if !errors.As(err, &at) || !strings.Contains(at.Message, "is not a") {
				continue
			}
			part := prefix(in, at.Offset)
			_, _, partErr := run(t, id, Options{}, part)
			if at.Before != (part != "" && partErr == nil) {
				t.Fatalf("%s %q: the part before %q was offered %t, decoding it alone: %v", id, in, part, at.Before, partErr)
			}
			if at.Before {
				offered++
			}
		}
		if offered < 50 {
			t.Errorf("%s: only %d inputs offered the part before", id, offered)
		}
	}
}

// Sort, remove duplicates and reverse over runs spilled to files, merged in
// several passes, agree with the same tools in memory.
func TestLineToolsSpillToFiles(t *testing.T) {
	r := rand.New(rand.NewSource(3))
	var lines []string
	for i := 0; i < 3000; i++ {
		lines = append(lines, fmt.Sprintf("host%d %s", r.Intn(400), strings.Repeat("x", r.Intn(5))))
	}
	input := strings.Join(lines, "\n") + "\n"
	want := map[string]string{}
	for _, id := range []string{"sort-asc", "sort-desc", "dedupe", "reverse"} {
		got, _, err := run(t, id, Options{}, input)
		if err != nil {
			t.Fatal(err)
		}
		want[id] = got
	}
	budget, width := runBudget, mergeWidth
	runBudget, mergeWidth = 2048, 3
	defer func() { runBudget, mergeWidth = budget, width }()
	dir := t.TempDir()
	for _, id := range []string{"sort-asc", "sort-desc", "dedupe", "reverse"} {
		out := &Output{In: []byte(input)}
		result, err := Run(context.Background(), id, Options{}, BytesSource(input), out, Env{Dir: dir})
		if err != nil || !result.Changed || out.Done() != nil {
			t.Fatal(id, err)
		}
		if string(out.Out) != want[id] {
			t.Errorf("%s over spilled runs differs from the same tool in memory", id)
		}
		if entries, _ := os.ReadDir(dir); len(entries) != 0 {
			t.Errorf("%s left %d run files behind", id, len(entries))
		}
	}
	sorted := strings.Split(strings.TrimSuffix(want["sort-asc"], "\n"), "\n")
	if !slices.IsSortedFunc(sorted, func(a, b string) int { return naturalCompare([]byte(a), []byte(b)) }) {
		t.Error("the sort is not in order")
	}
	seen := map[string]bool{}
	for _, line := range strings.Split(strings.TrimSuffix(want["dedupe"], "\n"), "\n") {
		if seen[line] {
			t.Fatalf("%q is still there twice", line)
		}
		seen[line] = true
	}
}

// A cancelled tool stops and changes nothing.
func TestToolsStopWhenCancelled(t *testing.T) {
	input := strings.Repeat("some line\n", 200000)
	for _, id := range []string{"upper", "sort-asc", "dedupe", "base64-encode", "remove-empty"} {
		ctx, cancel := context.WithCancel(context.Background())
		calls := 0
		out := &Output{In: []byte(input)}
		_, err := Run(ctx, id, Options{}, BytesSource(input), out, Env{Dir: t.TempDir(), Progress: func(float64) {
			if calls++; calls == 1 {
				cancel()
			}
		}})
		cancel()
		if !errors.Is(err, context.Canceled) {
			t.Errorf("%s after cancel: %v", id, err)
		}
	}
}

// Errors name the position, in UTF-16 units from the start of the input.
func TestErrorsNameThePosition(t *testing.T) {
	_, _, err := run(t, "json-pretty", Options{}, "{\"é😀\": tru}")
	var at *PositionError
	if !errors.As(err, &at) || at.Offset != 8 {
		t.Fatalf("%v %+v", err, at)
	}
	_, _, err = run(t, "base64-decode", Options{}, "QUJD\nQU#D")
	if !errors.As(err, &at) || at.Offset != 7 {
		t.Fatalf("%v %+v", err, at)
	}
}

func TestNaturalCompare(t *testing.T) {
	ordered := []string{"", "1", "01a", "2", "10", "a", "A2", "a10", "b", "é"}
	for i := range ordered {
		for k := range ordered {
			got := naturalCompare([]byte(ordered[i]), []byte(ordered[k]))
			if want := compareInt(i, k); got != want && !(ordered[i] == "1" && ordered[k] == "01a") {
				if (got < 0) != (want < 0) || (got > 0) != (want > 0) {
					t.Errorf("%q vs %q: %d, want %d", ordered[i], ordered[k], got, want)
				}
			}
		}
	}
}
