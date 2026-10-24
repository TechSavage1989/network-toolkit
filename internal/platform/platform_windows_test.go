//go:build windows

package platform

import (
	"context"
	"strings"
	"testing"
	"time"
	"unsafe"
)

func TestWindowsNativeStructureLayouts(t *testing.T) {
	if unsafe.Sizeof(uintptr(0)) != 8 {
		t.Skip("Windows x64 layout assertions")
	}
	if got := unsafe.Offsetof(icmpEchoReply{}.Options); got != 24 {
		t.Fatalf("ICMP options offset %d, want24", got)
	}
	if got := unsafe.Sizeof(icmpEchoReply{}); got != 40 {
		t.Fatalf("ICMP echo reply size %d, want40", got)
	}
	if got := unsafe.Sizeof(sockaddr6{}); got != 28 {
		t.Fatalf("IPv6 socket address size %d, want28", got)
	}
	if got := unsafe.Offsetof(adapterAddress{}.TransmitLinkSpeed); got != 184 {
		t.Fatalf("adapter transmit speed offset %d, want184", got)
	}
	if got := unsafe.Offsetof(adapterAddress{}.Gateway); got != 208 {
		t.Fatalf("adapter gateway offset %d, want208", got)
	}
}

func TestWindowsSnapshotParsing(t *testing.T) {
	data := "Active Connections\n  Proto  Local Address  Foreign Address  State  PID\n TCP  0.0.0.0:443  0.0.0.0:0  LISTENING  188\n UDP  [::]:53  *:*  540\n malformed row"
	rows := parseNetstat(data)
	if len(rows) != 2 || rows[0]["pid"] != 188 || rows[1]["local"] != "[::]:53" {
		t.Fatalf("invalid connection rows: %v", rows)
	}
	if text := decodeCommandOutput([]byte{0xff, 0xfe, 'N', 0, 'e', 0, 't', 0}); text != "Net" {
		t.Fatalf("UTF-16 decoding: %q", text)
	}
}

func TestWindowsNativeLoopback(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 6*time.Second)
	defer cancel()
	for _, target := range []string{"127.0.0.1", "::1"} {
		reply, err := Ping(ctx, target, time.Second, 64)
		if err != nil || !reply.Success || !strings.HasPrefix(reply.Address, target) {
			t.Errorf("native %s: %+v %v", target, reply, err)
		}
	}
	data, err := System(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(data["interfaces"].([]map[string]any)) == 0 {
		t.Error("native adapters list empty")
	}
}
