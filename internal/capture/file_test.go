package capture

import (
	"bytes"
	"encoding/binary"
	"errors"
	"io"
	"math"
	"testing"
	"time"
)

func readRecords(t *testing.T, raw []byte) []Record {
	t.Helper()
	var out []Record
	if e := Read(bytes.NewReader(raw), func(r Record) error { out = append(out, r); return nil }); e != nil {
		t.Fatal(e)
	}
	return out
}
func TestCaptureFileRoundTrip(t *testing.T) {
	stamp := time.Unix(1780000123, 987654321).UTC()
	original := []Record{{Time: stamp, Data: []byte{1, 2, 3, 4, 5}, OriginalLength: 80, LinkType: 1, Interface: "Ethernet", Direction: "out"}, {Time: stamp.Add(11 * time.Nanosecond), Data: []byte{6, 7}, OriginalLength: 2, LinkType: 101, Interface: "VPN", Direction: "in"}}
	var b bytes.Buffer
	if e := WritePCAPNG(&b, original); e != nil {
		t.Fatal(e)
	}
	got := readRecords(t, b.Bytes())
	if len(got) != 2 {
		t.Fatal(len(got))
	}
	for i, r := range got {
		want := original[i]
		if !r.Time.Equal(want.Time) || r.Interface != want.Interface || r.LinkType != want.LinkType || r.Direction != want.Direction || r.OriginalLength != want.OriginalLength || !bytes.Equal(r.Data, want.Data) {
			t.Fatalf("round trip %d: %#v", i, r)
		}
	}
	b.Reset()
	if WritePCAP(&b, original) == nil {
		t.Fatal("mixed link types accepted by PCAP")
	}
	b.Reset()
	if e := WritePCAP(&b, original[:1]); e != nil {
		t.Fatal(e)
	}
	r := readRecords(t, b.Bytes())[0]
	if !r.Time.Equal(stamp) || !bytes.Equal(r.Data, original[0].Data) {
		t.Fatal("PCAP lost bytes or nanoseconds")
	}
}
func TestClassicPCAPByteOrderAndResolution(t *testing.T) {
	for _, order := range []binary.ByteOrder{binary.LittleEndian, binary.BigEndian} {
		for _, nano := range []bool{false, true} {
			h := make([]byte, 24)
			magic := uint32(0xa1b2c3d4)
			fraction := uint32(123456)
			nanos := 123456000
			if nano {
				magic = 0xa1b23c4d
				fraction = 123456789
				nanos = 123456789
			}
			order.PutUint32(h, magic)
			order.PutUint16(h[4:], 2)
			order.PutUint16(h[6:], 4)
			order.PutUint32(h[16:], 65535)
			order.PutUint32(h[20:], 1)
			p := make([]byte, 16)
			order.PutUint32(p, 123)
			order.PutUint32(p[4:], fraction)
			order.PutUint32(p[8:], 3)
			order.PutUint32(p[12:], 4)
			raw := append(append(h, p...), 1, 2, 3)
			r := readRecords(t, raw)[0]
			if r.Time.Unix() != 123 || r.Time.Nanosecond() != nanos {
				t.Fatal(r.Time)
			}
		}
	}
}
func TestCaptureTimestampIntegerPrecision(t *testing.T) {
	for _, v := range []struct {
		ticks              uint64
		res                byte
		offset, sec, nanos int64
	}{{1780000123987654321, 9, 0, 1780000123, 987654321}, {(uint64(123) << 32) + 1<<31, 0x80 | 32, -2, 121, 500000000}, {123, 0, 4, 127, 0}, {999999999999999999, 18, 0, 0, 999999999}} {
		got, e := timestamp(v.ticks, v.res, v.offset)
		if e != nil || got.Unix() != v.sec || int64(got.Nanosecond()) != v.nanos {
			t.Fatalf("%+v: %s %v", v, got, e)
		}
	}
	for _, v := range []struct {
		ticks uint64
		res   byte
		off   int64
	}{{math.MaxUint64, 0, 0}, {1, 19, 0}, {1, 0xff, 0}, {1, 0, -2}, {1, 0, math.MaxInt64}} {
		if _, e := timestamp(v.ticks, v.res, v.off); e == nil {
			t.Fatal("invalid timestamp accepted", v)
		}
	}
}
func TestCaptureMalformedBlocksAndLengths(t *testing.T) {
	var b bytes.Buffer
	_ = WritePCAPNG(&b, []Record{{Time: time.Unix(100, 0), Data: []byte{1, 2, 3}, OriginalLength: 3, LinkType: 1, Interface: "a"}})
	good := b.Bytes()
	cases := [][]byte{nil, {1, 2, 3}, good[:len(good)-1], append([]byte{}, good...)}
	binary.LittleEndian.PutUint32(cases[3][len(good)-4:], 16)
	for i, raw := range cases {
		if e := Read(bytes.NewReader(raw), func(Record) error { return nil }); e == nil {
			t.Fatal("malformed accepted", i)
		}
	}
	var p bytes.Buffer
	_ = WritePCAP(&p, []Record{{Time: time.Unix(100, 0), Data: []byte{1}, OriginalLength: 1, LinkType: 1}})
	raw := p.Bytes()
	binary.LittleEndian.PutUint32(raw[32:], math.MaxUint32)
	if Read(bytes.NewReader(raw), func(Record) error { return nil }) == nil {
		t.Fatal("oversized PCAP record accepted")
	}
	sentinel := errors.New("stop parser")
	if e := Read(bytes.NewReader(good), func(Record) error { return sentinel }); !errors.Is(e, sentinel) {
		t.Fatal(e)
	}
}

type shortWriter struct{}

func (shortWriter) Write(p []byte) (int, error) { return 0, nil }
func TestCaptureExportShortWriter(t *testing.T) {
	if !errors.Is(WritePCAP(shortWriter{}, nil), io.ErrShortWrite) || !errors.Is(WritePCAPNG(shortWriter{}, nil), io.ErrShortWrite) {
		t.Fatal("short writes were ignored")
	}
}
func FuzzCaptureFileReader(f *testing.F) {
	var b bytes.Buffer
	_ = WritePCAPNG(&b, []Record{{Time: time.Unix(1, 0), Data: []byte{1, 2}, OriginalLength: 2, LinkType: 1}})
	f.Add(b.Bytes())
	f.Add([]byte("invalid capture"))
	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) > 1<<20 {
			t.Skip()
		}
		_ = Read(bytes.NewReader(data), func(r Record) error {
			if len(r.Data) > MaxPacketBytes || len(r.Data) > int(r.OriginalLength) {
				t.Fatal("reader emitted invalid record")
			}
			return nil
		})
	})
}
