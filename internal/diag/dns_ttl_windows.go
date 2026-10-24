//go:build windows

package diag

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strings"
	"syscall"
	"time"
	"unsafe"
)

// Windows DnsQuery_W returns resource-record TTLs, unlike net.Resolver.
// It follows the Windows DNS client's preferred interfaces and DNS policies.
var (
	dnsAPI         = syscall.NewLazyDLL("dnsapi.dll")
	nativeDNSQuery = dnsAPI.NewProc("DnsQuery_W")
	nativeDNSFree  = dnsAPI.NewProc("DnsRecordListFree")
)

type nativeDNSRecord struct {
	Next       *nativeDNSRecord
	Name       *uint16
	Type       uint16
	DataLength uint16
	Flags      uint32
	TTL        uint32
	Reserved   uint32
	Data       [64]byte
}

func systemDNSWithTTL(ctx context.Context, name, typ string, timeout time.Duration) ([]DNSRecord, error) {
	code, supported := dnsTypes[typ]
	if !supported || !map[uint16]bool{1: true, 28: true, 2: true, 5: true, 12: true, 15: true, 16: true, 6: true, 33: true}[code] {
		return nil, errors.New("use direct DNS query for this record type")
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	result := make(chan struct {
		rows []DNSRecord
		err  error
	}, 1)
	go func() {
		rows, err := windowsTTLQuery(name, code)
		result <- struct {
			rows []DNSRecord
			err  error
		}{rows, err}
	}()
	select {
	case data := <-result:
		return data.rows, data.err
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func windowsTTLQuery(name string, kind uint16) ([]DNSRecord, error) {
	if err := nativeDNSQuery.Find(); err != nil {
		return nil, err
	}
	pointer, err := syscall.UTF16PtrFromString(name)
	if err != nil {
		return nil, err
	}
	var first *nativeDNSRecord
	// DNS_QUERY_STANDARD: Windows DNS cache and configured resolvers.
	status, _, _ := nativeDNSQuery.Call(uintptr(unsafe.Pointer(pointer)), uintptr(kind), 0, 0, uintptr(unsafe.Pointer(&first)), 0)
	if first != nil {
		defer nativeDNSFree.Call(uintptr(unsafe.Pointer(first)), 1)
	}
	if status != 0 {
		// The DNS client reports "record does not exist" separately from transport errors.
		if status == 9501 || status == 9502 || status == 9503 {
			return nil, &net.DNSError{Err: "DNS name or record not found", Name: name, IsNotFound: true}
		}
		return nil, fmt.Errorf("Windows DNS query failed (status %d)", status)
	}
	records := make([]DNSRecord, 0, 4)
	visited := map[*nativeDNSRecord]bool{}
	for r, count := first, 0; r != nil && count < 4096; r, count = r.Next, count+1 {
		if visited[r] {
			return nil, errors.New("Windows DNS record list cycle")
		}
		visited[r] = true
		if r.Type != kind && r.Type != 5 {
			continue
		}
		value, err := nativeTTLValue(r)
		if err != nil {
			continue
		}
		label := dnsNames[r.Type]
		if label == "" {
			label = fmt.Sprintf("TYPE%d", r.Type)
		}
		owner := strings.TrimSpace(dnsWindowsString(r.Name))
		if owner == "" {
			owner = name
		}
		ttl := r.TTL
		records = append(records, DNSRecord{Name: owner, Type: label, Value: value, TTL: &ttl, Section: "answer"})
	}
	return records, nil
}

func nativeTTLValue(r *nativeDNSRecord) (string, error) {
	data := unsafe.Pointer(&r.Data[0])
	wide := func(p uintptr) string {
		if p == 0 {
			return ""
		}
		return dnsWindowsString((*uint16)(unsafe.Pointer(p)))
	}
	ptr := func(offset uintptr) uintptr { return *(*uintptr)(unsafe.Add(data, offset)) }
	u16 := func(offset uintptr) uint16 { return *(*uint16)(unsafe.Add(data, offset)) }
	u32 := func(offset uintptr) uint32 { return *(*uint32)(unsafe.Add(data, offset)) }
	switch r.Type {
	case 1:
		return net.IP(r.Data[:4]).String(), nil
	case 28:
		return net.IP(r.Data[:16]).String(), nil
	case 2, 5, 12:
		return wide(ptr(0)), nil
	case 15:
		return fmt.Sprintf("%d %s", u16(8), wide(ptr(0))), nil
	case 33:
		return fmt.Sprintf("%d %d %d %s", u16(8), u16(10), u16(12), wide(ptr(0))), nil
	case 6:
		return fmt.Sprintf("%s %s %d %d %d %d %d", wide(ptr(0)), wide(ptr(8)), u32(16), u32(20), u32(24), u32(28), u32(32)), nil
	case 16:
		count := u32(0)
		if count > 128 {
			return "", errors.New("invalid TXT string count")
		}
		out := make([]string, 0, count)
		for i := uint32(0); i < count; i++ {
			out = append(out, wide(ptr(8+uintptr(i)*unsafe.Sizeof(uintptr(0)))))
		}
		return strings.Join(out, ""), nil
	}
	return "", errors.New("unsupported native DNS record")
}

func dnsWindowsString(ptr *uint16) string {
	if ptr == nil {
		return ""
	}
	units := make([]uint16, 0, 128)
	for i := uintptr(0); i < 4096; i++ {
		code := *(*uint16)(unsafe.Add(unsafe.Pointer(ptr), i*2))
		if code == 0 {
			break
		}
		units = append(units, code)
	}
	return syscall.UTF16ToString(units)
}
