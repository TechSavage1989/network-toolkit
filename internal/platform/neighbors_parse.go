package platform

import (
	"encoding/binary"
	"errors"
	"fmt"
	"net/netip"
	"strconv"
)

// Windows SDK MIB_IPNET_ROW2 uses an eight-byte aligned NET_LUID. The row is
// 88 bytes and follows four bytes of padding after MIB_IPNET_TABLE2.NumEntries.
// Native compile-time assertions separately verify these offsets on Windows.
// https://learn.microsoft.com/windows/win32/api/netioapi/ns-netioapi-mib_ipnet_row2
const windowsNeighborTableOffset = 8
const windowsNeighborRowSize = 88

func parseWindowsNeighborTable(raw []byte, names map[int]string) ([]Neighbor, error) {
	if len(raw) < windowsNeighborTableOffset {
		return nil, errors.New("truncated Windows neighbor table")
	}
	count := binary.LittleEndian.Uint32(raw[:4])
	if count > maxNeighborEntries || uint64(count)*windowsNeighborRowSize > uint64(len(raw)-windowsNeighborTableOffset) {
		return nil, errors.New("invalid Windows neighbor table length")
	}
	rows := make([]Neighbor, 0, count)
	for i := uint32(0); i < count; i++ {
		row := raw[windowsNeighborTableOffset+int(i)*windowsNeighborRowSize:][:windowsNeighborRowSize]
		index := int(binary.LittleEndian.Uint32(row[28:32]))
		var address netip.Addr
		family := ""
		switch binary.LittleEndian.Uint16(row[:2]) {
		case 2: // AF_INET, SOCKADDR_IN.sin_addr
			address = netip.AddrFrom4([4]byte(row[4:8]))
			family = "IPv4"
		case 23: // AF_INET6, SOCKADDR_IN6.sin6_addr and sin6_scope_id
			address = netip.AddrFrom16([16]byte(row[8:24]))
			if address.IsLinkLocalUnicast() {
				scope := binary.LittleEndian.Uint32(row[24:28])
				if scope == 0 {
					scope = uint32(index)
				}
				if scope != 0 {
					address = address.WithZone(strconv.FormatUint(uint64(scope), 10))
				}
			}
			family = "IPv6"
		default:
			continue
		}
		macLength := binary.LittleEndian.Uint32(row[72:76])
		if macLength > 32 {
			return rows, errors.New("invalid Windows neighbor physical address length")
		}
		state := "unknown"
		switch binary.LittleEndian.Uint32(row[76:80]) {
		case 0:
			state = "unreachable"
		case 1:
			state = "incomplete"
		case 2:
			state = "probe"
		case 3:
			state = "delay"
		case 4:
			state = "stale"
		case 5:
			state = "reachable"
		case 6:
			state = "permanent"
		}
		rows = append(rows, Neighbor{Address: address.String(), MAC: neighborMAC(row[40 : 40+int(macLength)]),
			Interface: neighborInterfaceName(index, names), InterfaceIndex: index,
			Family: family, State: state, IsRouter: row[80]&1 != 0})
	}
	return rows, nil
}

// Linux neighbor messages use the public ndmsg and NDA_* attribute ABI from
// linux/neighbour.h. Their integer fields use the host's native byte order.
func parseLinuxNeighborMessage(raw []byte, names map[int]string) (Neighbor, bool, error) {
	if len(raw) < 12 {
		return Neighbor{}, false, errors.New("truncated Linux neighbor message")
	}
	var addressLength int
	family := ""
	switch raw[0] {
	case 2: // AF_INET
		addressLength, family = 4, "IPv4"
	case 10: // AF_INET6
		addressLength, family = 16, "IPv6"
	default:
		// AF_BRIDGE entries are bridge forwarding records, not IP neighbors.
		return Neighbor{}, false, nil
	}
	index := int(int32(binary.NativeEndian.Uint32(raw[4:8])))
	if index <= 0 {
		return Neighbor{}, false, errors.New("invalid Linux neighbor interface index")
	}
	var addressBytes, macBytes []byte
	for attrs := raw[12:]; len(attrs) > 0; {
		if len(attrs) < 4 {
			return Neighbor{}, false, errors.New("truncated Linux neighbor attribute")
		}
		length := int(binary.NativeEndian.Uint16(attrs[:2]))
		if length < 4 || length > len(attrs) {
			return Neighbor{}, false, errors.New("invalid Linux neighbor attribute length")
		}
		switch binary.NativeEndian.Uint16(attrs[2:4]) & 0x3fff {
		case 1: // NDA_DST
			addressBytes = attrs[4:length]
		case 2: // NDA_LLADDR
			macBytes = attrs[4:length]
		}
		aligned := (length + 3) &^ 3
		if aligned > len(attrs) {
			if length == len(attrs) {
				break
			}
			return Neighbor{}, false, errors.New("invalid Linux neighbor attribute padding")
		}
		attrs = attrs[aligned:]
	}
	if len(addressBytes) != addressLength || len(macBytes) > 32 {
		return Neighbor{}, false, errors.New("invalid Linux neighbor address length")
	}
	address, _ := netip.AddrFromSlice(addressBytes)
	if address.IsLinkLocalUnicast() && address.Is6() {
		address = address.WithZone(strconv.Itoa(index))
	}
	state := "unknown"
	switch value := binary.NativeEndian.Uint16(raw[8:10]); value {
	case 0x00:
		state = "none"
	case 0x01:
		state = "incomplete"
	case 0x02:
		state = "reachable"
	case 0x04:
		state = "stale"
	case 0x08:
		state = "delay"
	case 0x10:
		state = "probe"
	case 0x20:
		state = "unreachable"
	case 0x40:
		state = "no ARP"
	case 0x80:
		state = "permanent"
	default:
		state = fmt.Sprintf("flags 0x%x", value)
	}
	return Neighbor{Address: address.String(), MAC: neighborMAC(macBytes), Interface: neighborInterfaceName(index, names),
		InterfaceIndex: index, Family: family, State: state, IsRouter: raw[10]&0x80 != 0}, true, nil
}
