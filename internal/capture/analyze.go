package capture

import (
	"bytes"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"
)

type Endpoint struct {
	Address  string     `json:"address"`
	Packets  int        `json:"packets"`
	Bytes    uint64     `json:"bytes"`
	Sent     int        `json:"sent"`
	Received int        `json:"received"`
	Start    *time.Time `json:"start,omitempty"`
	End      *time.Time `json:"end,omitempty"`
}
type Conversation struct {
	ID              string     `json:"id"`
	Protocol        string     `json:"protocol"`
	A               string     `json:"a"`
	B               string     `json:"b"`
	Packets         int        `json:"packets"`
	Bytes           uint64     `json:"bytes"`
	AToB            int        `json:"aToB"`
	BToA            int        `json:"bToA"`
	Start           time.Time  `json:"start"`
	End             *time.Time `json:"end,omitempty"`
	Duration        float64    `json:"duration"`
	Resets          int        `json:"resets"`
	Retransmissions int        `json:"retransmissions"`
}
type Bucket struct {
	Seconds         float64    `json:"seconds"`
	Packets         int        `json:"packets"`
	Bytes           uint64     `json:"bytes"`
	Time            *time.Time `json:"time,omitempty"`
	EndTime         *time.Time `json:"endTime,omitempty"`
	IntervalSeconds float64    `json:"intervalSeconds"`
}
type Statistics struct {
	Protocols      map[string]int `json:"protocols"`
	Endpoints      []Endpoint     `json:"endpoints"`
	Conversations  []Conversation `json:"conversations"`
	Timeline       []Bucket       `json:"timeline"`
	Bytes          uint64         `json:"bytes"`
	Expert         int            `json:"expert"`
	Duration       float64        `json:"duration"`
	Started        *time.Time     `json:"started,omitempty"`
	Finished       *time.Time     `json:"finished,omitempty"`
	TimedPackets   int            `json:"timedPackets"`
	UntimedPackets int            `json:"untimedPackets"`
	TimedBytes     uint64         `json:"timedBytes"`
}

func Stats(packets []Packet) Statistics {
	s := Statistics{Protocols: map[string]int{}, Endpoints: []Endpoint{}, Conversations: []Conversation{}, Timeline: []Bucket{}}
	if len(packets) == 0 {
		return s
	}
	var first, last time.Time
	for _, p := range packets {
		if p.Time.IsZero() {
			s.UntimedPackets++
			continue
		}
		s.TimedPackets++
		if !p.Time.IsZero() && (first.IsZero() || p.Time.Before(first)) {
			first = p.Time
		}
		if p.Time.After(last) {
			last = p.Time
		}
	}
	if !first.IsZero() {
		s.Duration = max(0, last.Sub(first).Seconds())
		start, finish := first.UTC(), last.UTC()
		s.Started, s.Finished = &start, &finish
	}
	// Bound the timeline without extending its time range beyond observed data.
	// A single timestamp is still a useful count, but has no measured rate.
	bins := 0
	step := max(1, s.Duration/60)
	if s.TimedPackets > 0 {
		bins = min(60, max(1, int(math.Ceil(s.Duration/step))))
	}
	s.Timeline = make([]Bucket, bins)
	for i := range s.Timeline {
		s.Timeline[i].Seconds = float64(i) * step
		start := first.Add(time.Duration(float64(i) * step * float64(time.Second))).UTC()
		end := first.Add(time.Duration(min(s.Duration, float64(i+1)*step) * float64(time.Second))).UTC()
		if i == bins-1 {
			end = last.UTC()
		}
		s.Timeline[i].Time, s.Timeline[i].EndTime = &start, &end
		s.Timeline[i].IntervalSeconds = max(0, end.Sub(start).Seconds())
	}
	endpoints := map[string]*Endpoint{}
	flows := map[string]*Conversation{}
	seen := map[string]map[uint32][]byte{}
	for _, p := range packets {
		s.Protocols[p.Protocol]++
		s.Bytes += uint64(p.Length)
		if len(p.Expert) > 0 {
			s.Expert++
		}
		if !p.Time.IsZero() && !first.IsZero() {
			bin := min(bins-1, max(0, int(p.Time.Sub(first).Seconds()/step)))
			s.Timeline[bin].Packets++
			s.Timeline[bin].Bytes += uint64(p.Length)
			s.TimedBytes += uint64(p.Length)
		}
		for _, side := range []struct {
			address string
			sent    bool
		}{{p.Source, true}, {p.Destination, false}} {
			if side.address == "" {
				continue
			}
			v := endpoints[side.address]
			if v == nil {
				v = &Endpoint{Address: side.address}
				endpoints[side.address] = v
			}
			v.Packets++
			v.Bytes += uint64(p.Length)
			if !p.Time.IsZero() {
				tm := p.Time.UTC()
				if v.Start == nil || tm.Before(*v.Start) {
					v.Start = &tm
				}
				if v.End == nil || tm.After(*v.End) {
					v.End = &tm
				}
			}
			if side.sent {
				v.Sent++
			} else {
				v.Received++
			}
		}
		if p.Stream == "" {
			continue
		}
		v := flows[p.Stream]
		a, b := fmt.Sprintf("%s:%d", p.Source, p.SourcePort), fmt.Sprintf("%s:%d", p.Destination, p.DestinationPort)
		if v == nil {
			v = &Conversation{ID: p.Stream, Protocol: p.Transport, A: a, B: b, Start: p.Time}
			flows[p.Stream] = v
		}
		v.Packets++
		v.Bytes += uint64(p.Length)
		if v.A == a {
			v.AToB++
		} else {
			v.BToA++
		}
		if !p.Time.IsZero() {
			tm := p.Time.UTC()
			if v.Start.IsZero() || tm.Before(v.Start) {
				v.Start = tm
			}
			if v.End == nil || tm.After(*v.End) {
				v.End = &tm
			}
			v.Duration = max(0, v.End.Sub(v.Start).Seconds())
		}
		if p.TCPFlags&4 != 0 {
			v.Resets++
		}
		if p.Transport == "TCP" && p.PayloadLength > 0 {
			key := p.Stream + " " + a
			segments := seen[key]
			if segments == nil {
				segments = map[uint32][]byte{}
				seen[key] = segments
			}
			payload := p.Record.Data[p.PayloadOffset : p.PayloadOffset+p.PayloadLength]
			if previous, ok := segments[p.Sequence]; ok && bytes.Equal(previous, payload) {
				v.Retransmissions++
			} else if len(segments) < 8192 {
				segments[p.Sequence] = payload
			}
		}
	}
	for _, v := range endpoints {
		s.Endpoints = append(s.Endpoints, *v)
	}
	for _, v := range flows {
		s.Conversations = append(s.Conversations, *v)
	}
	sort.Slice(s.Endpoints, func(i, j int) bool { return s.Endpoints[i].Bytes > s.Endpoints[j].Bytes })
	sort.Slice(s.Conversations, func(i, j int) bool { return s.Conversations[i].Bytes > s.Conversations[j].Bytes })
	if len(s.Endpoints) > 2000 {
		s.Endpoints = s.Endpoints[:2000]
	}
	if len(s.Conversations) > 2000 {
		s.Conversations = s.Conversations[:2000]
	}
	return s
}

type StreamDirection struct {
	From      string `json:"from"`
	To        string `json:"to"`
	Text      string `json:"text"`
	Hex       string `json:"hex"`
	Bytes     int    `json:"bytes"`
	Gaps      int    `json:"gaps"`
	Truncated bool   `json:"truncated"`
}
type Stream struct {
	ID         string            `json:"id"`
	Directions []StreamDirection `json:"directions"`
	Note       string            `json:"note"`
}

func Follow(packets []Packet, id string) Stream {
	s := Stream{ID: id, Directions: []StreamDirection{}, Note: "TCP payload is ordered by sequence number. Duplicate/overlapping bytes are skipped and gaps are marked. Encrypted payload is not decrypted. IP fragments and application protocols are not reassembled."}
	groups := map[string][]Packet{}
	for _, p := range packets {
		if p.Stream == id && p.PayloadLength > 0 {
			key := fmt.Sprintf("%s:%d", p.Source, p.SourcePort)
			groups[key] = append(groups[key], p)
		}
	}
	keys := []string{}
	for k := range groups {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, key := range keys {
		list := groups[key]
		base := list[0].Sequence
		if list[0].Transport == "TCP" {
			sort.SliceStable(list, func(i, j int) bool { return int32(list[i].Sequence-base) < int32(list[j].Sequence-base) })
		}
		d := StreamDirection{From: key, To: fmt.Sprintf("%s:%d", list[0].Destination, list[0].DestinationPort)}
		var out bytes.Buffer
		next := list[0].Sequence
		for _, p := range list {
			payload := p.Record.Data[p.PayloadOffset : p.PayloadOffset+p.PayloadLength]
			if p.Transport == "TCP" {
				delta := int32(p.Sequence - next)
				if delta > 0 {
					d.Gaps++
					fmt.Fprintf(&out, "\n[Missing %d captured bytes]\n", delta)
					next = p.Sequence
				} else if delta < 0 {
					skip := int(-int64(delta))
					if skip >= len(payload) {
						continue
					}
					payload = payload[skip:]
				}
				next += uint32(len(payload))
			}
			if out.Len()+len(payload) > 4<<20 {
				d.Truncated = true
				break
			}
			out.Write(payload)
		}
		data := out.Bytes()
		d.Bytes = len(data)
		d.Text = ASCII(data)
		d.Hex = HexDump(data)
		s.Directions = append(s.Directions, d)
	}
	if strings.HasPrefix(id, "UDP ") {
		s.Note = "UDP datagram payloads are shown in capture order. Datagram boundaries use the packet list. Encrypted data is not decrypted."
	}
	return s
}
