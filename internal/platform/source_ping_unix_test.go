//go:build !windows

package platform

import (
	"errors"
	"reflect"
	"testing"
)

func TestSourcePingUnixRequiresDeviceAndAddress(t *testing.T) {
	binding := &SourceBinding{InterfaceIndex: 7, InterfaceName: "fixture0", Address: "192.0.2.20", Family: "ipv4"}
	args, err := sourcePingArgs(binding, "linux")
	if err != nil || !reflect.DeepEqual(args, []string{"-I", "fixture0", "-I", "192.0.2.20", "-B"}) {
		t.Fatalf("source ping arguments: %v %v", args, err)
	}
	if _, err := sourcePingArgs(binding, "darwin"); err == nil {
		t.Fatal("unsupported platform silently accepted binding")
	}
	if args, err := sourcePingArgs(nil, "darwin"); err != nil || len(args) != 0 {
		t.Fatal("Automatic ping behavior changed")
	}
	if err := verifySourcePingVersion([]byte("ping from iputils 20240905"), nil); err != nil {
		t.Fatal(err)
	}
	for _, output := range []string{"BusyBox v1.36.0", "", "other ping"} {
		if err := verifySourcePingVersion([]byte(output), nil); err == nil {
			t.Fatalf("unknown ping implementation accepted: %s", output)
		}
	}
	if err := verifySourcePingVersion([]byte("ping from iputils"), errors.New("fixture error")); err == nil {
		t.Fatal("version command failure was ignored")
	}
}
