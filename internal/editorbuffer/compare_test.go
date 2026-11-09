package editorbuffer

import (
	"context"
	"errors"
	"math/rand"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"testing"
)

func collectDifferences(t *testing.T, left, right ComparisonInput) []Difference {
	t.Helper()
	var got []Difference
	err := Compare(context.Background(), left, right, func(diff Difference) error {
		got = append(got, diff)
		return nil
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	return got
}

func TestStreamingCompareLongLineAndTrailingNewline(t *testing.T) {
	long := strings.Repeat("λ😀", ChunkBytes)
	left := openTest(t, "same\n"+long+"left\n")
	right := openTest(t, "same\n"+long+"right")
	a, err := left.Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	b, err := right.Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()
	want := []Difference{{Left: 1, Right: 1, LeftCount: 2, RightCount: 1}}
	if got := collectDifferences(t, a.ComparisonInput(), b.ComparisonInput()); !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v, want %+v", got, want)
	}
}

func TestStreamingCompareLargeInsertionRetainsSuffix(t *testing.T) {
	const extra = 6000
	suffix := strings.Repeat("retained\n", 4096)
	left := InlineComparisonInput("prefix\n" + suffix)
	right := InlineComparisonInput("prefix\n" + strings.Repeat("inserted\n", extra) + suffix)
	want := []Difference{{Left: 1, Right: 1, RightCount: extra}}
	if got := collectDifferences(t, left, right); !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v, want %+v", got, want)
	}
	want = []Difference{{Left: 1, Right: 1, LeftCount: extra}}
	if got := collectDifferences(t, right, left); !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v, want %+v", got, want)
	}
}

func TestStreamingCompareRepeatedLineReplacement(t *testing.T) {
	left := InlineComparisonInput("repeat\nrepeat\nrepeat\n")
	right := InlineComparisonInput("changed\nrepeat\nrepeat\n")
	want := []Difference{{Left: 0, Right: 0, LeftCount: 1, RightCount: 1}}
	if got := collectDifferences(t, left, right); !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v, want %+v", got, want)
	}
}

func TestStreamingCompareMatchesRepeatedLineByContext(t *testing.T) {
	const prefix = "prefix\n"
	const inserted = "inserted start\nshared heading\ninserted detail\n"
	const retained = "shared heading\nretained detail\nretained ending\n"
	left := InlineComparisonInput(prefix + retained + "left tail\n")
	right := InlineComparisonInput(prefix + inserted + retained + "right tail\n")
	want := []Difference{
		{Left: 1, Right: 1, RightCount: 3},
		{Left: 4, Right: 7, LeftCount: 1, RightCount: 1},
	}
	if got := collectDifferences(t, left, right); !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v, want %+v", got, want)
	}
	want = []Difference{
		{Left: 1, Right: 1, LeftCount: 3},
		{Left: 7, Right: 4, LeftCount: 1, RightCount: 1},
	}
	if got := collectDifferences(t, right, left); !reflect.DeepEqual(got, want) {
		t.Fatalf("reverse comparison got %+v, want %+v", got, want)
	}
}

func TestStreamingCompareMatchesShiftedRepeatedLineByContext(t *testing.T) {
	const retained = "shared heading\nretained detail\nretained ending\n"
	left := InlineComparisonInput("prefix\nleft heading\n" + retained + "left tail\n")
	right := InlineComparisonInput("prefix\nright heading\nshared heading\ninserted detail\n" + retained + "right tail\n")
	want := []Difference{
		{Left: 1, Right: 1, LeftCount: 1, RightCount: 3},
		{Left: 5, Right: 7, LeftCount: 1, RightCount: 1},
	}
	if got := collectDifferences(t, left, right); !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v, want %+v", got, want)
	}
	for i, d := range want {
		want[i] = Difference{Left: d.Right, Right: d.Left, LeftCount: d.RightCount, RightCount: d.LeftCount}
	}
	if got := collectDifferences(t, right, left); !reflect.DeepEqual(got, want) {
		t.Fatalf("reverse comparison got %+v, want %+v", got, want)
	}
}

func TestStreamingCompareRetainsExactRowsBeforeRepeatedContext(t *testing.T) {
	left := InlineComparisonInput("header\nold interface\nshared action\nold group\nshared action\nshared schedule\nshared logging\nfooter\n")
	right := InlineComparisonInput("header\nnew interface\nshared action\nnew group\nshared action\nshared schedule\nshared logging\nfooter\nadded tail\n")
	want := []Difference{
		{Left: 1, Right: 1, LeftCount: 1, RightCount: 1},
		{Left: 3, Right: 3, LeftCount: 1, RightCount: 1},
		{Left: 8, Right: 8, RightCount: 1},
	}
	if got := collectDifferences(t, left, right); !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v, want %+v", got, want)
	}
}

func TestStreamingCompareKeepsExistingPolicyAfterRepeatedInsertions(t *testing.T) {
	policy := func(id string) string {
		return "    edit " + id + "\n" +
			"        set srcintf \"port1\"\n" +
			"        set dstintf \"port2\"\n" +
			"        set action accept\n" +
			"    next\n"
	}
	prefix := strings.Repeat("# unchanged fixture\n", 18000) + "config firewall policy\n"
	retained := policy("10000") + policy("10001") + "end\n"
	added := policy("20000") + policy("20001") + policy("20002")
	left := InlineComparisonInput(prefix + retained + "# before\n")
	right := InlineComparisonInput(prefix + added + retained + "# after\n")
	start := int64(strings.Count(prefix, "\n"))
	linesAdded := int64(strings.Count(added, "\n"))
	changed := start + int64(strings.Count(retained, "\n"))
	want := []Difference{
		{Left: start, Right: start, RightCount: linesAdded},
		{Left: changed, Right: changed + linesAdded, LeftCount: 1, RightCount: 1},
	}
	if got := collectDifferences(t, left, right); !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v, want %+v", got, want)
	}
	want = []Difference{
		{Left: start, Right: start, LeftCount: linesAdded},
		{Left: changed + linesAdded, Right: changed, LeftCount: 1, RightCount: 1},
	}
	if got := collectDifferences(t, right, left); !reflect.DeepEqual(got, want) {
		t.Fatalf("reverse comparison got %+v, want %+v", got, want)
	}
}

func TestStreamingCompareKeepsModifiedPolicyAlignedAfterInsertions(t *testing.T) {
	policy := func(id, name, dst string) string {
		return "    edit " + id + "\n" +
			"        set name " + name + "\n" +
			"        set uuid 12345678-abcd-4567-8910-1234567" + id + "\n" +
			"        set srcintf port1\n" +
			"        set dstintf " + dst + "\n" +
			"        set action accept\n" +
			"    next\n"
	}
	prefix := strings.Repeat("# unchanged fixture\n", 18000) + "config firewall policy\n"
	first := policy("10000", "original", "port2")
	revised := policy("10000", "renamed", "port3")
	second := policy("10001", "second", "port2")
	inserted := policy("20000", "new", "port2") + policy("20001", "new", "port2") + policy("20002", "new", "port2")
	between := policy("20003", "new", "port2")
	left := InlineComparisonInput(prefix + first + second + "end\n")
	right := InlineComparisonInput(prefix + inserted + revised + between + second + "end\n")
	start := int64(strings.Count(prefix, "\n"))
	want := []Difference{
		{Left: start, Right: start, RightCount: 21},
		{Left: start + 1, Right: start + 22, LeftCount: 1, RightCount: 1},
		{Left: start + 4, Right: start + 25, LeftCount: 1, RightCount: 1},
		{Left: start + 7, Right: start + 28, RightCount: 7},
	}
	if got := collectDifferences(t, left, right); !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v, want %+v", got, want)
	}
	for i, d := range want {
		want[i] = Difference{Left: d.Right, Right: d.Left, LeftCount: d.RightCount, RightCount: d.LeftCount}
	}
	if got := collectDifferences(t, right, left); !reflect.DeepEqual(got, want) {
		t.Fatalf("reverse comparison got %+v, want %+v", got, want)
	}
}

func TestStreamingCompareReportsEveryChangedRegion(t *testing.T) {
	rng := rand.New(rand.NewSource(11))
	for range 40 {
		var a, b []string
		for range 2000 {
			line := "same " + strconv.Itoa(rng.Intn(200))
			a = append(a, line)
			switch rng.Intn(20) {
			case 0:
				// Deleted line.
			case 1:
				b = append(b, "replacement "+line)
			case 2:
				b = append(b, "inserted", line)
			default:
				b = append(b, line)
			}
		}
		a, b = append(a, ""), append(b, "")
		differences := collectDifferences(t, InlineComparisonInput(strings.Join(a, "\n")), InlineComparisonInput(strings.Join(b, "\n")))
		var reconstructed []string
		var leftAt, rightAt int64
		for _, diff := range differences {
			if diff.Left < leftAt || diff.Right < rightAt || diff.Left+diff.LeftCount > int64(len(a)) || diff.Right+diff.RightCount > int64(len(b)) {
				t.Fatalf("unordered or out-of-range difference: %+v", diff)
			}
			if !slices.Equal(a[leftAt:diff.Left], b[rightAt:diff.Right]) {
				t.Fatalf("skipped changed lines before %+v", diff)
			}
			reconstructed = append(reconstructed, a[leftAt:diff.Left]...)
			reconstructed = append(reconstructed, b[diff.Right:diff.Right+diff.RightCount]...)
			leftAt, rightAt = diff.Left+diff.LeftCount, diff.Right+diff.RightCount
		}
		reconstructed = append(reconstructed, a[leftAt:]...)
		if !slices.Equal(reconstructed, b) {
			t.Fatal("comparison dropped changed lines")
		}
	}
}

func TestStreamingCompareCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	text := InlineComparisonInput(strings.Repeat("generated\n", 20000))
	err := Compare(ctx, text, text, func(Difference) error {
		t.Fatal("identical documents have a difference")
		return nil
	}, func(CompareProgress) { cancel() })
	if !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
}
func TestCompareWhitespaceOptions(t *testing.T) {
	for _, test := range []struct {
		mode        string
		left, right string
	}{
		{"changed", "  start\tend  ", "start end"},
		{"all", "start\tend", "startend"},
		{"changed", strings.Repeat("a", ChunkBytes-1) + "\t\tend", strings.Repeat("a", ChunkBytes-1) + " end"},
	} {
		for _, reverse := range []bool{false, true} {
			options := CompareOptions{Spaces: test.mode}
			left := newLineSignatures(context.Background(), InlineComparisonInput(test.left), reverse, 1, options)
			right := newLineSignatures(context.Background(), InlineComparisonInput(test.right), reverse, 1, options)
			a, errA := left.next()
			b, errB := right.next()
			if errA != nil || errB != nil || a != b {
				t.Fatalf("spacing mode %q, reverse %v: %v %v", test.mode, reverse, errA, errB)
			}
		}
	}
}
