package capture

import (
	"bufio"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math"
	"math/bits"
	"time"
)

// Read accepts classic micro/nanosecond PCAP and multi-section, multi-interface
// PCAPNG. Malformed lengths are rejected before allocating packet data.
func Read(reader io.Reader, emit func(Record) error) error {
	r := bufio.NewReader(io.LimitReader(reader, MaxFileBytes+1))
	magic, e := r.Peek(4)
	if e != nil {
		return errors.New("The capture file is empty or incomplete.")
	}
	if binary.LittleEndian.Uint32(magic) == 0x0a0d0d0a {
		return readNG(r, emit)
	}
	return readPCAP(r, emit)
}
func readPCAP(r io.Reader, emit func(Record) error) error {
	var h [24]byte
	if _, e := io.ReadFull(r, h[:]); e != nil {
		return e
	}
	var order binary.ByteOrder
	scale := int64(1000)
	switch binary.LittleEndian.Uint32(h[:4]) {
	case 0xa1b2c3d4:
		order = binary.LittleEndian
	case 0xd4c3b2a1:
		order = binary.BigEndian
	case 0xa1b23c4d:
		order = binary.LittleEndian
		scale = 1
	case 0x4d3cb2a1:
		order = binary.BigEndian
		scale = 1
	default:
		return errors.New("Choose a PCAP or PCAPNG file.")
	}
	if order.Uint16(h[4:]) != 2 || order.Uint16(h[6:]) != 4 {
		return errors.New("Unsupported PCAP version.")
	}
	snap := order.Uint32(h[16:])
	link := order.Uint32(h[20:]) & 0xffff
	if snap == 0 || snap > MaxPacketBytes {
		return errors.New("Invalid PCAP snapshot length.")
	}
	total := 24
	for n := 0; ; n++ {
		var ph [16]byte
		_, e := io.ReadFull(r, ph[:])
		if e == io.EOF {
			return nil
		}
		if e != nil {
			return errors.New("A PCAP packet header is incomplete.")
		}
		if n >= MaxPackets {
			return errors.New("The capture exceeds 200000 packets.")
		}
		incl, orig := order.Uint32(ph[8:]), order.Uint32(ph[12:])
		fraction := order.Uint32(ph[4:])
		if incl > snap || incl > MaxPacketBytes || incl > orig || int64(fraction)*scale >= 1e9 {
			return errors.New("A PCAP packet length or timestamp is invalid.")
		}
		total += 16 + int(incl)
		if total > MaxFileBytes {
			return errors.New("The capture exceeds 128 MiB.")
		}
		data := make([]byte, incl)
		if _, e = io.ReadFull(r, data); e != nil {
			return errors.New("A PCAP packet is incomplete.")
		}
		if e = emit(Record{Time: time.Unix(int64(order.Uint32(ph[:])), int64(fraction)*scale).UTC(), Data: data, OriginalLength: orig, LinkType: uint16(link), Interface: "Interface 1"}); e != nil {
			return e
		}
	}
}

type ngInterface struct {
	link       uint16
	snap       uint32
	name       string
	resolution byte
	offset     int64
}

func options(data []byte, order binary.ByteOrder, fn func(uint16, []byte) error) error {
	for len(data) >= 4 {
		code, n := order.Uint16(data), int(order.Uint16(data[2:]))
		data = data[4:]
		if code == 0 {
			if n != 0 {
				return errors.New("Invalid PCAPNG end option.")
			}
			return nil
		}
		padded := (n + 3) &^ 3
		if padded > len(data) {
			return errors.New("A PCAPNG option exceeds its block.")
		}
		if e := fn(code, data[:n]); e != nil {
			return e
		}
		data = data[padded:]
	}
	if len(data) != 0 {
		return errors.New("An incomplete PCAPNG option was found.")
	}
	return nil
}
func timestamp(ticks uint64, res byte, offset int64) (time.Time, error) {
	base := uint64(1)
	if res&0x80 != 0 {
		if res&0x7f > 63 {
			return time.Time{}, errors.New("Invalid binary timestamp resolution.")
		}
		base <<= res & 0x7f
	} else {
		if res > 18 {
			return time.Time{}, errors.New("Invalid decimal timestamp resolution.")
		}
		for i := byte(0); i < res; i++ {
			base *= 10
		}
	}
	whole, fraction := ticks/base, ticks%base
	if whole > math.MaxInt64 || offset > 0 && int64(whole) > math.MaxInt64-offset {
		return time.Time{}, errors.New("Capture timestamp is out of range.")
	}
	seconds := int64(whole) + offset
	if seconds < 0 || seconds > 7258118400 {
		return time.Time{}, errors.New("Capture timestamp is out of range.")
	}
	hi, lo := bits.Mul64(fraction, 1e9)
	nanos, _ := bits.Div64(hi, lo, base)
	return time.Unix(seconds, int64(nanos)).UTC(), nil
}
func readNG(r io.Reader, emit func(Record) error) error {
	var order binary.ByteOrder
	interfaces := []ngInterface{}
	total, count, sections := 0, 0, 0
	for {
		var h [12]byte
		_, e := io.ReadFull(r, h[:])
		if e == io.EOF {
			return nil
		}
		if e != nil {
			return errors.New("A PCAPNG block header is incomplete.")
		}
		section := binary.LittleEndian.Uint32(h[:4]) == 0x0a0d0d0a
		if section {
			switch binary.LittleEndian.Uint32(h[8:]) {
			case 0x1a2b3c4d:
				order = binary.LittleEndian
			case 0x4d3c2b1a:
				order = binary.BigEndian
			default:
				return errors.New("Invalid PCAPNG byte order.")
			}
			interfaces = nil
			sections++
		}
		if order == nil {
			return errors.New("PCAPNG must begin with a section header.")
		}
		length := order.Uint32(h[4:])
		if length < 16 || length%4 != 0 || length > MaxFileBytes || total+int(length) > MaxFileBytes {
			return errors.New("Invalid or oversized PCAPNG block.")
		}
		total += int(length)
		body := make([]byte, length-12)
		if _, e = io.ReadFull(r, body); e != nil {
			return errors.New("A PCAPNG block is incomplete.")
		}
		if order.Uint32(body[len(body)-4:]) != length {
			return errors.New("PCAPNG block lengths do not match.")
		}
		data := append(h[8:12:12], body[:len(body)-4]...)
		switch order.Uint32(h[:4]) {
		case 0x0a0d0d0a:
			if length < 28 || order.Uint16(data[4:]) != 1 {
				return errors.New("Unsupported PCAPNG section version.")
			}
		case 1:
			if len(data) < 8 || len(interfaces) >= 1024 {
				return errors.New("Invalid PCAPNG interface block.")
			}
			v := ngInterface{link: order.Uint16(data), snap: order.Uint32(data[4:]), name: fmt.Sprintf("Section %d interface %d", sections, len(interfaces)+1), resolution: 6}
			if v.snap > MaxPacketBytes {
				return errors.New("PCAPNG interface snapshot length is too large.")
			}
			e = options(data[8:], order, func(code uint16, b []byte) error {
				switch code {
				case 2:
					if len(b) > 512 {
						return errors.New("Interface name is too long.")
					}
					v.name = string(b)
				case 9:
					if len(b) != 1 {
						return errors.New("Invalid timestamp resolution option.")
					}
					v.resolution = b[0]
				case 14:
					if len(b) != 8 {
						return errors.New("Invalid timestamp offset option.")
					}
					v.offset = int64(order.Uint64(b))
				}
				return nil
			})
			if e != nil {
				return e
			}
			interfaces = append(interfaces, v)
		case 6, 2:
			if len(data) < 20 {
				return errors.New("Invalid PCAPNG packet block.")
			}
			id := order.Uint32(data)
			if order.Uint32(h[:4]) == 2 {
				id = uint32(order.Uint16(data))
			}
			if id >= uint32(len(interfaces)) {
				return errors.New("A packet refers to an unknown PCAPNG interface.")
			}
			v := interfaces[id]
			incl, orig := order.Uint32(data[12:]), order.Uint32(data[16:])
			if incl > MaxPacketBytes || incl > orig || v.snap != 0 && incl > v.snap || uint64(20)+uint64((incl+3)&^3) > uint64(len(data)) {
				return errors.New("A PCAPNG packet length is invalid.")
			}
			stamp, e := timestamp(uint64(order.Uint32(data[4:]))<<32|uint64(order.Uint32(data[8:])), v.resolution, v.offset)
			if e != nil {
				return e
			}
			record := Record{Time: stamp, Data: append([]byte{}, data[20:20+incl]...), OriginalLength: orig, LinkType: v.link, Interface: v.name}
			e = options(data[20+((incl+3)&^3):], order, func(code uint16, b []byte) error {
				if code == 2 && len(b) == 4 {
					switch order.Uint32(b) & 3 {
					case 1:
						record.Direction = "in"
					case 2:
						record.Direction = "out"
					}
				}
				return nil
			})
			if e != nil {
				return e
			}
			count++
			if count > MaxPackets {
				return errors.New("The capture exceeds 200000 packets.")
			}
			if e = emit(record); e != nil {
				return e
			}
		case 3:
			if len(interfaces) == 0 || len(data) < 4 {
				return errors.New("Invalid PCAPNG simple packet.")
			}
			v := interfaces[0]
			orig := order.Uint32(data)
			incl := orig
			if v.snap != 0 && incl > v.snap {
				incl = v.snap
			}
			if incl > MaxPacketBytes || int((incl+3)&^3) != len(data)-4 {
				return errors.New("Invalid PCAPNG simple packet length.")
			}
			count++
			if count > MaxPackets {
				return errors.New("The capture exceeds 200000 packets.")
			}
			if e = emit(Record{Data: append([]byte{}, data[4:4+incl]...), OriginalLength: orig, LinkType: v.link, Interface: v.name}); e != nil {
				return e
			}
		}
	}
}
func writeAll(w io.Writer, b []byte) error {
	for len(b) > 0 {
		n, e := w.Write(b)
		if e != nil {
			return e
		}
		if n <= 0 || n > len(b) {
			return io.ErrShortWrite
		}
		b = b[n:]
	}
	return nil
}
func WritePCAP(w io.Writer, records []Record) error {
	link := uint16(1)
	if len(records) > 0 {
		link = records[0].LinkType
	}
	for _, r := range records {
		if r.LinkType != link {
			return errors.New("This capture contains multiple link types. Export PCAPNG instead.")
		}
	}
	h := make([]byte, 24)
	binary.LittleEndian.PutUint32(h, 0xa1b23c4d)
	binary.LittleEndian.PutUint16(h[4:], 2)
	binary.LittleEndian.PutUint16(h[6:], 4)
	binary.LittleEndian.PutUint32(h[16:], MaxPacketBytes)
	binary.LittleEndian.PutUint32(h[20:], uint32(link))
	if e := writeAll(w, h); e != nil {
		return e
	}
	for _, r := range records {
		sec := r.Time.Unix()
		if r.Time.IsZero() {
			sec = 0
		}
		if sec < 0 || sec > math.MaxUint32 {
			return errors.New("Timestamp cannot be represented in PCAP.")
		}
		p := make([]byte, 16)
		binary.LittleEndian.PutUint32(p, uint32(sec))
		if !r.Time.IsZero() {
			binary.LittleEndian.PutUint32(p[4:], uint32(r.Time.Nanosecond()))
		}
		binary.LittleEndian.PutUint32(p[8:], uint32(len(r.Data)))
		binary.LittleEndian.PutUint32(p[12:], r.OriginalLength)
		if e := writeAll(w, p); e != nil {
			return e
		}
		if e := writeAll(w, r.Data); e != nil {
			return e
		}
	}
	return nil
}
func ngBlock(w io.Writer, kind uint32, body []byte) error {
	length := 12 + ((len(body) + 3) &^ 3)
	b := make([]byte, length)
	binary.LittleEndian.PutUint32(b, kind)
	binary.LittleEndian.PutUint32(b[4:], uint32(length))
	copy(b[8:], body)
	binary.LittleEndian.PutUint32(b[length-4:], uint32(length))
	return writeAll(w, b)
}
func ngOption(code uint16, b []byte) []byte {
	out := make([]byte, 4+((len(b)+3)&^3))
	binary.LittleEndian.PutUint16(out, code)
	binary.LittleEndian.PutUint16(out[2:], uint16(len(b)))
	copy(out[4:], b)
	return out
}
func WritePCAPNG(w io.Writer, records []Record) error {
	h := make([]byte, 16)
	binary.LittleEndian.PutUint32(h, 0x1a2b3c4d)
	binary.LittleEndian.PutUint16(h[4:], 1)
	binary.LittleEndian.PutUint64(h[8:], math.MaxUint64)
	if e := ngBlock(w, 0x0a0d0d0a, h); e != nil {
		return e
	}
	ids := map[string]uint32{}
	for _, r := range records {
		key := fmt.Sprintf("%d:%s", r.LinkType, r.Interface)
		if _, ok := ids[key]; ok {
			continue
		}
		ids[key] = uint32(len(ids))
		body := make([]byte, 8)
		binary.LittleEndian.PutUint16(body, r.LinkType)
		binary.LittleEndian.PutUint32(body[4:], MaxPacketBytes)
		name := r.Interface
		if len(name) > 512 {
			name = name[:512]
		}
		body = append(body, ngOption(2, []byte(name))...)
		body = append(body, ngOption(9, []byte{9})...)
		body = append(body, 0, 0, 0, 0)
		if e := ngBlock(w, 1, body); e != nil {
			return e
		}
	}
	for _, r := range records {
		body := make([]byte, 20+((len(r.Data)+3)&^3))
		binary.LittleEndian.PutUint32(body, ids[fmt.Sprintf("%d:%s", r.LinkType, r.Interface)])
		var ticks uint64
		if !r.Time.IsZero() {
			if r.Time.Unix() < 0 || r.Time.Unix() > 7258118400 {
				return errors.New("Timestamp is outside the supported capture range.")
			}
			ticks = uint64(r.Time.Unix())*1e9 + uint64(r.Time.Nanosecond())
		}
		binary.LittleEndian.PutUint32(body[4:], uint32(ticks>>32))
		binary.LittleEndian.PutUint32(body[8:], uint32(ticks))
		binary.LittleEndian.PutUint32(body[12:], uint32(len(r.Data)))
		binary.LittleEndian.PutUint32(body[16:], r.OriginalLength)
		copy(body[20:], r.Data)
		if r.Direction != "" {
			flags := []byte{1, 0, 0, 0}
			if r.Direction == "out" {
				flags[0] = 2
			}
			body = append(body, ngOption(2, flags)...)
		}
		if r.DropReason != 0 {
			body = append(body, ngOption(1, []byte(fmt.Sprintf("Windows drop reason 0x%X", r.DropReason)))...)
		}
		body = append(body, 0, 0, 0, 0)
		if e := ngBlock(w, 6, body); e != nil {
			return e
		}
	}
	return nil
}
