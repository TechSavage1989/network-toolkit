package deviceinfo

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"networktoolkit/internal/platform"
)

func deviceLoopbackBinding(t *testing.T) *platform.SourceBinding {
	t.Helper()
	interfaces, err := net.Interfaces()
	if err != nil {
		t.Fatal(err)
	}
	for _, nic := range interfaces {
		if nic.Flags&net.FlagLoopback == 0 || nic.Flags&net.FlagUp == 0 {
			continue
		}
		addresses, err := nic.Addrs()
		if err != nil {
			t.Fatal(err)
		}
		for _, address := range addresses {
			ip, _, err := net.ParseCIDR(address.String())
			if err == nil && ip.Equal(net.ParseIP("127.0.0.1")) {
				return &platform.SourceBinding{InterfaceIndex: nic.Index, InterfaceName: nic.Name, Address: "127.0.0.1", Family: "ipv4"}
			}
		}
	}
	t.Fatal("loopback fixture requires an active 127.0.0.1 adapter")
	return nil
}

func TestSelectedDeviceDiscoveryAndDescriptionKeepSource(t *testing.T) {
	binding := deviceLoopbackBinding(t)
	sources := make(chan string, 2)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sources <- r.RemoteAddr
		w.Header().Set("Content-Type", "text/xml")
		fmt.Fprint(w, fixtureDescription)
	}))
	defer server.Close()
	udp, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	if err != nil {
		t.Fatal(err)
	}
	defer udp.Close()
	finished := make(chan error, 1)
	go func() {
		buffer := make([]byte, 4096)
		n, remote, err := udp.ReadFromUDP(buffer)
		if err != nil {
			finished <- err
			return
		}
		if !strings.Contains(string(buffer[:n]), "ST: upnp:rootdevice") {
			finished <- fmt.Errorf("unexpected SSDP query %q", buffer[:n])
			return
		}
		sources <- remote.String()
		_, err = udp.WriteToUDP([]byte("HTTP/1.1 200 OK\r\nST: upnp:rootdevice\r\nLOCATION: "+server.URL+"/desc.xml\r\n\r\n"), remote)
		finished <- err
	}()
	ctx := platform.WithSourceBinding(context.Background(), binding)
	info, err := probe(ctx, "127.0.0.1", 2500*time.Millisecond, udp.LocalAddr().(*net.UDPAddr).Port)
	if err != nil || !info.Responded || info.Model != "Router (R123)" {
		t.Fatalf("bound UPnP: %+v %v", info, err)
	}
	if err := <-finished; err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		host, _, err := net.SplitHostPort(<-sources)
		if err != nil || host != binding.Address {
			t.Fatalf("UPnP request used source %q: %v", host, err)
		}
	}
}

func TestSelectedDeviceDescriptionCannotFallBackAfterBindFailure(t *testing.T) {
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { requests.Add(1); fmt.Fprint(w, fixtureDescription) }))
	defer server.Close()
	location, _ := url.Parse(server.URL + "/desc.xml")
	binding := deviceLoopbackBinding(t)
	binding.InterfaceIndex, binding.InterfaceName = 2147483646, "missing-toolkit-fixture-interface"
	ctx, cancel := context.WithTimeout(platform.WithSourceBinding(context.Background(), binding), time.Second)
	defer cancel()
	_, err := fetchDescription(ctx, netip.MustParseAddr("127.0.0.1"), location)
	if err == nil || !strings.Contains(err.Error(), "bind diagnostic socket") || requests.Load() != 0 {
		t.Fatalf("device description used unbound retry: calls=%d err=%v", requests.Load(), err)
	}
}

func TestSelectedHTTPSDeviceDescriptionStillRejectsUntrustedCertificate(t *testing.T) {
	var requests atomic.Int32
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { requests.Add(1); fmt.Fprint(w, fixtureDescription) }))
	defer server.Close()
	location, _ := url.Parse(server.URL + "/desc.xml")
	ctx, cancel := context.WithTimeout(platform.WithSourceBinding(context.Background(), deviceLoopbackBinding(t)), 2*time.Second)
	defer cancel()
	_, err := fetchDescription(ctx, netip.MustParseAddr("127.0.0.1"), location)
	if err == nil || requests.Load() != 0 {
		t.Fatalf("selected HTTPS description bypassed certificate validation: calls=%d err=%v", requests.Load(), err)
	}
}
