package deviceinfo

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

const fixtureDescription = `<?xml version="1.0"?>
<root xmlns="urn:schemas-upnp-org:device-1-0"><specVersion><major>1</major><minor>0</minor></specVersion>
<device><deviceType>urn:schemas-upnp-org:device:InternetGatewayDevice:1</deviceType>
<friendlyName>Office router</friendlyName><manufacturer>Example Devices</manufacturer>
<modelName>Router</modelName><modelNumber>R123</modelNumber>
<deviceList><device><deviceType>urn:schemas-upnp-org:device:WANDevice:1</deviceType>
<manufacturer>Wrong embedded manufacturer</manufacturer><modelName>Wrong embedded model</modelName></device></deviceList>
</device></root>`

func TestDescriptionUsesOnlyRootDeviceReportedFields(t *testing.T) {
	info, err := parseDescription([]byte(fixtureDescription))
	if err != nil || info.Manufacturer != "Example Devices" || info.Model != "Router (R123)" || info.ModelName != "Router" || info.ModelNumber != "R123" || info.FriendlyName != "Office router" || info.ModelSource != "UPnP device description" {
		t.Fatalf("unexpected description: %+v %v", info, err)
	}
	noModel := `<root><device><deviceType>urn:example-org:device:Device:1</deviceType><friendlyName>My named device</friendlyName><manufacturer>Example</manufacturer></device></root>`
	info, err = parseDescription([]byte(noModel))
	if err != nil || info.Model != "" || info.ModelName != "" || info.Manufacturer != "Example" {
		t.Fatalf("friendly name was promoted to a model: %+v %v", info, err)
	}
	for _, bad := range []string{
		`<html><body>Example Model R1</body></html>`,
		`<!DOCTYPE root SYSTEM "http://192.0.2.1/other"><root/>`,
		`<root><device><modelName>Guessed</modelName></device></root>`,
		`<root><device><deviceType>urn:x:device:y:1</deviceType><modelName>A</modelName><modelName>B</modelName></device></root>`,
		`<root><device><deviceType>urn:x:device:y:1</deviceType><modelName><x>nested</x></modelName></device></root>`,
		fixtureDescription + `<root/>`,
		`<root>` + strings.Repeat(`<x>`, 33) + strings.Repeat(`</x>`, 33) + `</root>`,
	} {
		if _, err := parseDescription([]byte(bad)); err == nil {
			t.Errorf("accepted malformed or ambiguous description %q", bad)
		}
	}
}

func TestDescriptionLocationStaysAtTarget(t *testing.T) {
	target := netip.MustParseAddr("192.0.2.5")
	for _, good := range []string{"http://192.0.2.5/desc.xml", "http://192.0.2.5:8080/desc.xml?q=1", "https://192.0.2.5:443/desc.xml"} {
		if _, err := descriptionLocation(good, target); err != nil {
			t.Errorf("rejected target description %q: %v", good, err)
		}
	}
	for _, bad := range []string{"http://192.0.2.6/desc.xml", "http://example.com/desc.xml", "file:///etc/passwd", "//192.0.2.5/desc.xml", "http://u:p@192.0.2.5/desc.xml", "http://192.0.2.5:0/desc.xml", "http://192.0.2.5:65536/desc.xml", "http://192.0.2.5:/desc.xml", "http://192.0.2.5/desc.xml#fragment", "http://192.0.2.5/\r\nInjected: yes"} {
		if _, err := descriptionLocation(bad, target); err == nil {
			t.Errorf("accepted unrelated or invalid URL %q", bad)
		}
	}
	if _, err := descriptionLocation("http://[fe80::1%257]:8080/desc.xml", netip.MustParseAddr("fe80::1%7")); err != nil {
		t.Fatalf("matching IPv6 scope rejected: %v", err)
	}
	if _, err := descriptionLocation("http://[fe80::1%258]:8080/desc.xml", netip.MustParseAddr("fe80::1%7")); err == nil {
		t.Fatal("accepted different IPv6 interface scope")
	}
}

func TestSSDPRequiresMatchingRootResponse(t *testing.T) {
	response := "HTTP/1.1 200 OK\r\nST: upnp:rootdevice\r\nLOCATION: http://192.0.2.1/desc.xml\r\nSERVER: This is not a model\r\n\r\n"
	location, err := parseSSDPResponse([]byte(response))
	if err != nil || location != "http://192.0.2.1/desc.xml" {
		t.Fatalf("valid response rejected: %q %v", location, err)
	}
	for _, bad := range []string{strings.Replace(response, "200 OK", "500 Error", 1), strings.Replace(response, "upnp:rootdevice", "ssdp:all", 1), strings.Replace(response, "LOCATION:", "IGNORED:", 1), strings.Replace(response, "SERVER:", "LOCATION:", 1), strings.ReplaceAll(response, "\r\n", "\n")} {
		if _, err := parseSSDPResponse([]byte(bad)); err == nil {
			t.Errorf("accepted nonmatching discovery response %q", bad)
		}
	}
}

func startDeviceFixture(t *testing.T, location string) (int, <-chan string) {
	t.Helper()
	conn, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	requests := make(chan string, 1)
	go func() {
		buffer := make([]byte, 4096)
		n, address, err := conn.ReadFromUDP(buffer)
		if err != nil {
			return
		}
		requests <- string(buffer[:n])
		if location != "" {
			response := "HTTP/1.1 200 OK\r\nST: upnp:rootdevice\r\nLOCATION: " + location + "\r\nUSN: uuid:fixture::upnp:rootdevice\r\n\r\n"
			_, _ = conn.WriteToUDP([]byte(response), address)
		}
	}()
	return conn.LocalAddr().(*net.UDPAddr).Port, requests
}

func TestProbeLoopbackDescription(t *testing.T) {
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		if r.Method != http.MethodGet || r.URL.Path != "/desc.xml" || r.Header.Get("Authorization") != "" {
			t.Errorf("unexpected description request: %s %s", r.Method, r.URL.Path)
		}
		w.Header().Set("Content-Type", "text/xml")
		_, _ = fmt.Fprint(w, fixtureDescription)
	}))
	defer server.Close()
	port, captured := startDeviceFixture(t, server.URL+"/desc.xml")
	info, err := probe(context.Background(), "127.0.0.1", 2500*time.Millisecond, port)
	if err != nil || !info.Responded || info.Model != "Router (R123)" || info.DescriptionURL != server.URL+"/desc.xml" || requests.Load() != 1 {
		t.Fatalf("loopback device: %+v, requests=%d, %v", info, requests.Load(), err)
	}
	request := <-captured
	if !strings.HasPrefix(request, "M-SEARCH * HTTP/1.1\r\n") || !strings.Contains(request, fmt.Sprintf("HOST: 127.0.0.1:%d\r\n", port)) || !strings.Contains(request, "ST: upnp:rootdevice\r\n") || strings.Contains(request, "239.255.255.250") || strings.Contains(request, "MX:") {
		t.Fatalf("incorrect unicast request: %q", request)
	}
}

func TestProbeDoesNotFollowRedirectsOrOtherTargets(t *testing.T) {
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		http.Redirect(w, r, "/other", http.StatusFound)
	}))
	defer server.Close()
	port, _ := startDeviceFixture(t, server.URL+"/desc.xml")
	info, err := probe(context.Background(), "127.0.0.1", 2500*time.Millisecond, port)
	if err == nil || !info.Responded || info.Model != "" || requests.Load() != 1 {
		t.Fatalf("redirect handling: %+v requests=%d err=%v", info, requests.Load(), err)
	}
	port, _ = startDeviceFixture(t, strings.Replace(server.URL, "127.0.0.1", "127.0.0.2", 1)+"/desc.xml")
	info, err = probe(context.Background(), "127.0.0.1", 2500*time.Millisecond, port)
	if err == nil || !info.Responded || requests.Load() != 1 {
		t.Fatalf("different target URL was contacted: %+v requests=%d err=%v", info, requests.Load(), err)
	}
}

func TestProbeDescriptionBodyLimit(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/xml")
		_, _ = fmt.Fprint(w, strings.Repeat("x", maxDescriptionBytes+1))
	}))
	defer server.Close()
	port, _ := startDeviceFixture(t, server.URL+"/desc.xml")
	info, err := probe(context.Background(), "127.0.0.1", 2500*time.Millisecond, port)
	if err == nil || !info.Responded || !strings.Contains(err.Error(), "512 KiB") {
		t.Fatalf("oversized response: %+v %v", info, err)
	}
}

func TestProbeCancellationInterruptsReplyWait(t *testing.T) {
	port, received := startDeviceFixture(t, "")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() {
		_, err := probe(ctx, "127.0.0.1", 2500*time.Millisecond, port)
		done <- err
	}()
	select {
	case <-received:
	case <-time.After(time.Second):
		t.Fatal("fixture did not receive discovery query")
	}
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("cancellation returned %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("cancellation did not interrupt UDP reply wait")
	}
}

func TestProbeNoResponseAndValidation(t *testing.T) {
	port, _ := startDeviceFixture(t, "")
	started := time.Now()
	info, err := probe(context.Background(), "127.0.0.1", 2500*time.Millisecond, port)
	if !errors.Is(err, ErrNoResponse) || info.Responded || time.Since(started) < time.Second || time.Since(started) > 2400*time.Millisecond {
		t.Fatalf("reply window or response evidence incorrect: %+v %v elapsed=%v", info, err, time.Since(started))
	}
	for _, bad := range []string{"example.com", "192.0.2.0/24", "0.0.0.0", "224.0.0.1", "255.255.255.255", "::", "ff02::c", "fe80::1"} {
		if _, err := Probe(context.Background(), bad, 2500*time.Millisecond); err == nil {
			t.Errorf("accepted invalid target %q", bad)
		}
	}
	for _, duration := range []time.Duration{0, time.Second, 6 * time.Second} {
		if _, err := Probe(context.Background(), "127.0.0.1", duration); err == nil {
			t.Errorf("accepted invalid total timeout %v", duration)
		}
	}
}
