package main

import (
	"bufio"
	"bytes"
	"io"
	"testing"
)

// Test-only observer of actual Telnet wire bytes, not /resize HTTP interceptions.
func readNativeResize129NAWS(input io.Reader, emit func(int, int)) error {
	r := bufio.NewReader(input)
	for {
		b, err := r.ReadByte()
		if err != nil {
			return err
		}
		if b != 255 {
			continue
		}
		command, err := r.ReadByte()
		if err != nil {
			return err
		}
		if command >= 251 && command <= 254 {
			if _, err = r.ReadByte(); err != nil {
				return err
			}
			continue
		}
		if command != 250 {
			continue
		}
		option, err := r.ReadByte()
		if err != nil {
			return err
		}
		payload := []byte{}
		overflow := false
		for {
			b, err = r.ReadByte()
			if err != nil {
				return err
			}
			if b == 255 {
				next, e := r.ReadByte()
				if e != nil {
					return e
				}
				if next == 240 {
					break
				}
				if next != 255 {
					overflow = true
					continue
				}
			}
			if len(payload) < 64 {
				payload = append(payload, b)
			} else {
				overflow = true
			}
		}
		if option == 31 && !overflow && len(payload) == 4 {
			cols, rows := int(payload[0])<<8|int(payload[1]), int(payload[2])<<8|int(payload[3])
			if cols > 0 && rows > 0 {
				emit(cols, rows)
			}
		}
	}
}
func TestNativeResize129NAWSWireDecoder(t *testing.T) {
	wire := []byte{255, 251, 31, 255, 250, 31, 0, 80, 0, 24, 255, 240, 255, 250, 31, 0, 255, 255, 0, 32, 255, 240, 255, 250, 31, 0, 0, 0, 24, 255, 240}
	var sizes [][2]int
	if err := readNativeResize129NAWS(bytes.NewReader(wire), func(c, r int) { sizes = append(sizes, [2]int{c, r}) }); err != io.EOF {
		t.Fatal(err)
	}
	if len(sizes) != 2 || sizes[0] != [2]int{80, 24} || sizes[1] != [2]int{255, 32} {
		t.Fatalf("actual NAWS frames decoded incorrectly: %v", sizes)
	}
}
