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
