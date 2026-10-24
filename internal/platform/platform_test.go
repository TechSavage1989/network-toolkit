package platform

import (
	"bytes"
	"context"
	"net"
	"testing"
	"time"
)

func TestMagicPacketAndMACValidation(t *testing.T) {
	packet, mac, err := magicPacket("00-11-22-33-44-55")
	if err != nil {
		t.Fatal(err)
	}
	if len(packet) != 102 || !bytes.Equal(packet[:6], bytes.Repeat([]byte{0xff}, 6)) {
		t.Fatalf("invalid packet framing: %x", packet)
	}
	for i := 0; i < 16; i++ {
		if !bytes.Equal(packet[6+i*6:12+i*6], mac) {
			t.Fatalf("incorrect MAC repetition %d", i)
		}
	}
	plain, _, err := magicPacket("001122334455")
	if err != nil || !bytes.Equal(plain, packet) {
		t.Fatal("plain MAC notation did not produce the same packet")
	}
	for _, bad := range []string{"", "00:11:22", "ff:ff:ff:ff:ff:ff", "01:00:5e:00:00:01", "00:00:00:00:00:00", "00:11:22:33:44:55:66:77"} {
		if _, _, err := magicPacket(bad); err == nil {
			t.Errorf("accepted invalid device MAC %q", bad)
		}
	}
}

func TestWakeLoopbackDatagram(t *testing.T) {
	receiver, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	if err != nil {
		t.Fatal(err)
	}
	defer receiver.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	port := receiver.LocalAddr().(*net.UDPAddr).Port
	result, err := Wake(ctx, "00:11:22:33:44:55", "127.0.0.1", port)
	if err != nil {
		t.Fatal(err)
	}
	if result.Data.(map[string]any)["status"] != "sent" {
		t.Fatal("missing accurate packet-sent status")
	}
	_ = receiver.SetReadDeadline(time.Now().Add(time.Second))
	buffer := make([]byte, 512)
	n, _, err := receiver.ReadFromUDP(buffer)
	if err != nil {
		t.Fatal(err)
	}
	expected, _, _ := magicPacket("00:11:22:33:44:55")
	if !bytes.Equal(buffer[:n], expected) {
		t.Fatalf("received invalid magic packet: %x", buffer[:n])
	}
}

func TestContextAndInputValidation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := Ping(ctx, "127.0.0.1", time.Second, 1); err != context.Canceled {
		t.Fatalf("cancelled Ping returned %v", err)
	}
	if _, err := Wake(ctx, "00:11:22:33:44:55", "127.0.0.1", 9); err != context.Canceled {
		t.Fatalf("cancelled Wake returned %v", err)
	}
	for _, target := range []string{"", "x\n127.0.0.1", "bad host", "http://localhost", "127.0.0.1/24"} {
		if _, err := resolvePingTarget(context.Background(), target); err == nil {
			t.Errorf("accepted invalid ping target %q", target)
		}
	}
	address, err := resolvePingTarget(context.Background(), "fe80::1%5")
	if err != nil || address.Zone != "5" || address.IP.String() != "fe80::1" {
		t.Fatalf("scope was not preserved: %v %v", address, err)
	}
	for _, destination := range []string{"::1", "0.0.0.0", "224.0.0.1", "example.com"} {
		if _, err := Wake(context.Background(), "00:11:22:33:44:55", destination, 9); err == nil {
			t.Errorf("accepted invalid WOL IPv4 destination %q", destination)
		}
	}
}
