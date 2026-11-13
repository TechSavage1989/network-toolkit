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
		{id: "trim", in: "  a  \n\tb\nc", want: "a\nb\nc", count: 2},
		{id: "trim", in: "   \n", want: "\n", count: 1},
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

// A fictional export with the shape of a real one.
func sonicwallExport(settings int) (string, string) {
	var plain, readable strings.Builder
	add := func(key, value, shown string) {
		plain.WriteString(key + "=" + value + "&")
		readable.WriteString(key + "=" + shown + "\n")
	}
	add("checksumVersion", "1", "1")
	add("buildNum", "7.1.3-0000-R0000", "7.1.3-0000-R0000")
	add("shortProdName", "TZ%20370", "TZ 370")
	for i := 0; i < settings; i++ {
		n := fmt.Sprint(i)
		add("addrObjId_"+n, "LAN%20Subnets%20"+n, "LAN Subnets "+n)
		add("addrObjIp1_"+n, "192.0.2."+fmt.Sprint(i%250), "192.0.2."+fmt.Sprint(i%250))
		add("schedObjType_"+n, "a+b%3ac", "a+b:c")
		add("uuid_InternalInstanceId_"+n, "line%0Abreak%09tab%01", `line\nbreak`+"\ttab"+`\x01`)
	}
	add("checksum", "0000abcd", "0000abcd")
	return base64.StdEncoding.EncodeToString([]byte(plain.String())), readable.String()
}

func TestSonicWallReadable(t *testing.T) {
	export, want := sonicwallExport(3)
	got, result, err := run(t, "sonicwall-readable", Options{}, export)
	if err != nil || got != want {
		t.Fatalf("%v\n got %q\nwant %q", err, got, want)
	}
	if result.Count != 3+4*3+1 {
		t.Errorf("count %d", result.Count)
	}
	if !LooksLikeSonicWall([]byte(export)) {
		t.Error("the export was not recognised")
	}
	// Bytes that are not UTF-8 and control characters are shown as escapes.
	odd := base64.StdEncoding.EncodeToString([]byte("buildNum=%FF%C3%A9%0D%E2x&checksum=1&"))
	if got, _, err = run(t, "sonicwall-readable", Options{}, odd); err != nil || got != "buildNum=\\xffé\\r\\xe2x\nchecksum=1\n" {
		t.Fatalf("%v %q", err, got)
	}
	for _, other := range []string{"plain text", base64.StdEncoding.EncodeToString([]byte("hello world, not settings")), base64.StdEncoding.EncodeToString([]byte("a=1&b=2&c=3&d=4&e=5&f=6&"))} {
		if LooksLikeSonicWall([]byte(other)) {
			t.Errorf("%q was taken for an export", other)
		}
	}
	if _, _, err = run(t, "sonicwall-readable", Options{}, base64.StdEncoding.EncodeToString([]byte("<html> not settings"))); err == nil || !strings.Contains(err.Error(), "does not look like") {
		t.Fatal(err)
	}
	// Only a prefix is needed to recognise one.
	if !LooksLikeSonicWall([]byte(export[:120])) {
		t.Error("the start of the export was not recognised")
	}
}

// A multi-megabyte export on one line, as the owner's 1.75 MB file was.
func TestLargeSingleLineInputs(t *testing.T) {
	export, want := sonicwallExport(11000)
	if len(export) < 1500000 {
		t.Fatalf("the fixture is only %d bytes", len(export))
	}
	got, result, err := run(t, "sonicwall-readable", Options{}, export)
	if err != nil || got != want {
		t.Fatalf("%v (%d bytes, want %d)", err, len(got), len(want))
	}
	if result.Count != 44004 {
		t.Errorf("%d settings", result.Count)
	}
	plain, _ := base64.StdEncoding.DecodeString(export)
	if got, _, err = run(t, "base64-decode", Options{}, export); err != nil || got != string(plain) {
		t.Fatalf("decoding the whole line: %v (%d bytes, want %d)", err, len(got), len(plain))
	}
	if got, _, err = run(t, "split", Options{Separator: "&"}, string(plain)); err != nil || strings.Count(got, "\n") != 44004 || strings.Contains(got, "&") {
		t.Fatalf("splitting: %v", err)
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
