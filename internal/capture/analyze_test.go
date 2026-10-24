package capture

import (
	"strings"
	"testing"
)

func TestTCPFollowOrderingOverlapGapsAndStatistics(t *testing.T) {
	var packets []Packet
	for _, v := range []struct {
		seq  uint32
		text string
	}{{103, "DEF"}, {100, "ABC"}, {100, "ABC"}, {105, "FGH"}, {111, "K"}} {
		packets = append(packets, Decode(testTCP(v.seq, 0x18, v.text)))
	}
	packets = append(packets, Decode(testTCP(112, 4, "")))
	id := packets[0].Stream
	s := Follow(packets, id)
	if len(s.Directions) != 1 || s.Directions[0].Text != "ABCDEFGH\n[Missing 3 captured bytes]\nK" || s.Directions[0].Gaps != 1 {
		t.Fatalf("bad stream: %+v", s)
	}
	stats := Stats(packets)
	if len(stats.Conversations) != 1 || stats.Conversations[0].Retransmissions != 1 || stats.Conversations[0].Resets != 1 || len(stats.Endpoints) != 2 {
		t.Fatal(stats)
	}
	if !strings.Contains(s.Note, "not decrypted") {
		t.Fatal("missing encrypted payload context")
	}
}
func TestTCPFollowSequenceWrap(t *testing.T) {
	a := Decode(testTCP(0xfffffffd, 0x18, "ABC"))
	b := Decode(testTCP(0, 0x18, "DEF"))
	s := Follow([]Packet{b, a}, a.Stream)
	if len(s.Directions) != 1 || s.Directions[0].Text != "ABCDEF" {
		t.Fatal(s)
	}
}
