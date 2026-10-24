package diag

import (
	"encoding/binary"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

const maxScanScopeText = 16 * 1024

var scanScopeOperators = regexp.MustCompile(`[ \t]*([/-])[ \t]*`)
var scanPortOperators = regexp.MustCompile(`[ \t]*-[ \t]*`)

type ipv4Interval struct {
	first, last uint64
	canonical   string
	network     string
}

type ipv4ScopePlan struct {
	targets []netip.Addr
	scopes  []string
	network string
}

// scanListSpace deliberately matches the ASCII whitespace accepted by the
// interface's local planner. Scope parsing never resolves hostnames or probes.
func scanListSpace(r rune) bool {
	return r == ' ' || r == '\t' || r == '\r' || r == '\n' || r == '\f' || r == '\v'
}

func decimalDigits(value string) bool {
	if value == "" {
		return false
	}
	for _, r := range value {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func ipv4Number(address netip.Addr) uint64 {
	bytes := address.As4()
	return uint64(binary.BigEndian.Uint32(bytes[:]))
}

func ipv4FromNumber(number uint64) netip.Addr {
	var bytes [4]byte
	binary.BigEndian.PutUint32(bytes[:], uint32(number))
	return netip.AddrFrom4(bytes)
}

func scanIPv4(value string) (netip.Addr, error) {
	address, err := netip.ParseAddr(value)
	if err != nil || !address.Is4() {
		return netip.Addr{}, errors.New("use literal IPv4 addresses; hostnames and IPv6 are not supported in network scan scopes")
	}
	return address, nil
}

func ipv4PrefixInterval(address netip.Addr, bits int) ipv4Interval {
	prefix := netip.PrefixFrom(address, bits).Masked()
	first := ipv4Number(prefix.Addr())
	last := first + (uint64(1) << uint(32-bits)) - 1
	if bits <= 30 {
		first++
		last--
	}
	return ipv4Interval{first: first, last: last, canonical: prefix.String(), network: prefix.String()}
}

func parseIPv4Scope(raw string) (ipv4Interval, error) {
	if strings.Contains(raw, "/") {
		parts := strings.Split(raw, "/")
		if len(parts) != 2 {
			return ipv4Interval{}, errors.New("use one CIDR prefix or subnet mask after the IPv4 address")
		}
		address, err := scanIPv4(parts[0])
		if err != nil {
			return ipv4Interval{}, err
		}
		var bits int
		if strings.Contains(parts[1], ".") {
			mask, err := scanIPv4(parts[1])
			if err != nil {
				return ipv4Interval{}, errors.New("enter a contiguous IPv4 subnet mask, such as 255.255.255.0")
			}
			var size int
			bits, size = net.IPMask(mask.AsSlice()).Size()
			if size != 32 {
				return ipv4Interval{}, errors.New("the IPv4 subnet mask must contain contiguous leading ones")
			}
		} else {
			if !decimalDigits(parts[1]) || len(parts[1]) > 2 {
				return ipv4Interval{}, errors.New("enter an IPv4 CIDR prefix from 0 to 32")
			}
			bits, _ = strconv.Atoi(parts[1])
			if bits > 32 {
				return ipv4Interval{}, errors.New("enter an IPv4 CIDR prefix from 0 to 32")
			}
		}
		return ipv4PrefixInterval(address, bits), nil
	}
	if strings.Contains(raw, "-") {
		parts := strings.Split(raw, "-")
		if len(parts) != 2 {
			return ipv4Interval{}, errors.New("enter one ascending IPv4 range, such as 192.168.1.1-254")
		}
		first, err := scanIPv4(parts[0])
		if err != nil {
			return ipv4Interval{}, err
		}
		var last netip.Addr
		if decimalDigits(parts[1]) && len(parts[1]) <= 3 {
			octet, _ := strconv.Atoi(parts[1])
			if octet > 255 {
				return ipv4Interval{}, errors.New("a short range must end with an octet from 0 to 255")
			}
			bytes := first.As4()
			bytes[3] = byte(octet)
			last = netip.AddrFrom4(bytes)
		} else {
			last, err = scanIPv4(parts[1])
			if err != nil {
				return ipv4Interval{}, err
			}
		}
		if last.Less(first) {
			return ipv4Interval{}, errors.New("IPv4 range endpoints must be in ascending order")
		}
		canonical := first.String() + "-" + last.String()
		return ipv4Interval{first: ipv4Number(first), last: ipv4Number(last), canonical: canonical, network: canonical}, nil
	}
	if strings.Contains(raw, "*") {
		if !strings.HasSuffix(raw, ".*") || strings.Count(raw, "*") != 1 {
			return ipv4Interval{}, errors.New("use a wildcard only for the final IPv4 octet, such as 192.168.1.*")
		}
		address, err := scanIPv4(strings.TrimSuffix(raw, "*") + "0")
		if err != nil {
			return ipv4Interval{}, err
		}
		return ipv4PrefixInterval(address, 24), nil
	}
	address, err := scanIPv4(raw)
	if err != nil {
		return ipv4Interval{}, err
	}
	return ipv4Interval{first: ipv4Number(address), last: ipv4Number(address), canonical: address.String(), network: netip.PrefixFrom(address, 32).String()}, nil
}

// parseIPv4Scopes uses intervals so a huge prefix/range is rejected before
// allocating addresses. Counting the union also prevents duplicate probes for
// overlapping networks while preserving explicitly listed endpoint addresses.
func parseIPv4Scopes(raw string, limit int) (ipv4ScopePlan, error) {
	var plan ipv4ScopePlan
	if len(raw) > maxScanScopeText {
		return plan, fmt.Errorf("scan scope text is limited to %d bytes", maxScanScopeText)
	}
	if limit < 1 {
		return plan, errors.New("the IPv4 address limit must be positive")
	}
	text := scanScopeOperators.ReplaceAllString(strings.TrimSpace(raw), "$1")
	parts := strings.FieldsFunc(text, func(r rune) bool { return scanListSpace(r) || r == ',' || r == ';' })
	if len(parts) == 0 {
		return plan, errors.New("enter at least one IPv4 host, network or range")
	}
	intervals := make([]ipv4Interval, 0, len(parts))
	seen := make(map[string]bool)
	for _, part := range parts {
		interval, err := parseIPv4Scope(part)
		if err != nil {
			return ipv4ScopePlan{}, fmt.Errorf("invalid scope %q: %w", part, err)
		}
		if !seen[interval.canonical] {
			seen[interval.canonical] = true
			plan.scopes = append(plan.scopes, interval.canonical)
			intervals = append(intervals, interval)
		}
	}
	if len(intervals) == 1 {
		plan.network = intervals[0].network
	} else {
		plan.network = strings.Join(plan.scopes, ", ")
	}
	sort.Slice(intervals, func(i, j int) bool {
		if intervals[i].first == intervals[j].first {
			return intervals[i].last < intervals[j].last
		}
		return intervals[i].first < intervals[j].first
	})
	merged := make([]ipv4Interval, 0, len(intervals))
	for _, interval := range intervals {
		if len(merged) == 0 || interval.first > merged[len(merged)-1].last+1 {
			merged = append(merged, interval)
		} else if interval.last > merged[len(merged)-1].last {
			merged[len(merged)-1].last = interval.last
		}
	}
	var count uint64
	for _, interval := range merged {
		count += interval.last - interval.first + 1
		if count > uint64(limit) {
			return ipv4ScopePlan{}, fmt.Errorf("network scans are limited to %d unique IPv4 addresses; reduce the selected networks or ranges", limit)
		}
	}
	plan.targets = make([]netip.Addr, 0, int(count))
	for _, interval := range merged {
		for number := interval.first; number <= interval.last; number++ {
			plan.targets = append(plan.targets, ipv4FromNumber(number))
		}
	}
	return plan, nil
}
