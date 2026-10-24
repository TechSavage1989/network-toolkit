//go:build !windows

package main

import "context"

func readLocalWiFi(context.Context) (localWiFiSnapshot, error) {
	return localWiFiSnapshot{Interfaces: []localWiFiInterface{}, Note: "Wi-Fi profiles and stored keys are available in the Windows app."}, nil
}
func revealLocalWiFi(context.Context, localWiFiRevealRequest) localWiFiResult {
	return localWiFiFailure("unsupported", "Wi-Fi key viewing is available in the Windows app.", false)
}
