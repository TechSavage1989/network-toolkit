//go:build windows

package platform

import (
	"syscall"
	"testing"
	"unsafe"
)

func TestOfflineWindowsCertificatePolicyConfiguration(t *testing.T) {
	// Required SDK flags are independent of the production composition, so a
	// future refactor cannot silently permit certificate URL/root auto-retrieval.
	if offlineWindowsChainFlags&0x00000004 == 0 || offlineWindowsChainFlags&0x00000100 == 0 || offlineWindowsChainFlags&0x80000000 == 0 {
		t.Fatal("offline certificate retrieval restrictions missing")
	}
	parameters, usage := offlineWindowsChainParameters()
	if parameters.Size != uint32(unsafe.Sizeof(syscall.CertChainPara{})) || parameters.RequestedUsage.Type != syscall.USAGE_MATCH_TYPE_AND || parameters.RequestedUsage.Usage.Length != 1 || parameters.RequestedUsage.Usage.UsageIdentifiers != &usage[0] || usage[0] != &syscall.OID_PKIX_KP_SERVER_AUTH[0] {
		t.Fatal("Windows certificate server-usage policy or ABI layout changed")
	}
}
