package editorbuffer

import (
	"context"
	"testing"
)

// Copy Marked Text copies the marks of every search in document order, and
// text that two marks cover once.
func TestFindMarkedTextInDocumentOrder(t *testing.T) {
	text := "a vlan 10 x vlan 20\nend\n"
	vlan := MarkSearch{Search: Search{Query: "vlan", MatchCase: true}}
	digits := MarkSearch{Search: Search{Query: `\d+`, Regex: true}}
	both(t, text, func(name string, s *Snapshot) {
		for _, c := range []struct {
			marks []MarkSearch
			want  string
			count int64
		}{
			{[]MarkSearch{vlan, digits}, "vlan\n10\nvlan\n20", 4},
			{[]MarkSearch{digits, vlan}, "vlan\n10\nvlan\n20", 4},
			// "an 1" overlaps "vlan" and "10": one stretch, copied once.
			{[]MarkSearch{vlan, digits, {Search: Search{Query: "an 1"}}}, "vlan 10\nvlan\n20", 3},
			// A mark of empty text copies nothing.
			{[]MarkSearch{vlan, {Search: Search{Query: `q*`, Regex: true}}}, "vlan\nvlan", 2},
			// A mark made In selection copies only its range.
			{[]MarkSearch{{Search: Search{Query: "vlan"}, Within: &[2]int64{0, 9}}, digits}, "vlan\n10\n20", 3},
		} {
			got, n, err := s.MarkedText(context.Background(), c.marks, "\n", 1<<20)
			if err != nil || got != c.want || n != c.count {
				t.Errorf("%s: %q %d %v, want %q %d", name, got, n, err, c.want, c.count)
			}
		}
	})
}
