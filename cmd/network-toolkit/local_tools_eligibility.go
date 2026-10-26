package main

// Windows reports both configurable network interfaces and implementation
// interfaces through GetAdaptersAddresses. Link-down is deliberately allowed:
// an unplugged NIC, disconnected Wi-Fi card or configurable VPN/virtual NIC
// still has settings that the user may need to edit.
func localIPv4Configurable(id string, index, flags, interfaceType, state, tunnelType uint32) bool {
	if index == 0 || !validAdapterID(id) || flags&0x80 == 0 || flags&0x08 != 0 {
		return false // No IPv4 binding, no stable identity, or receive-only.
	}
	if interfaceType == 24 || state == 6 {
		return false // Loopback or not present.
	}
	if interfaceType == 131 && tunnelType != 1 && tunnelType != 2 {
		// TUNNEL_TYPE_OTHER/DIRECT can expose real IPv4 configuration; the
		// Windows reader additionally requires readable TCP/IP settings and
		// limits these to DNS edits. Exclude transition tunnels and an unknown
		// subtype, rather than classifying a VPN by its display name.
		// https://learn.microsoft.com/windows/win32/api/ifdef/ne-ifdef-tunnel_type
		return false
	}
	return true
}
