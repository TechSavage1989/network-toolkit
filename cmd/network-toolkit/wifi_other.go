//go:build !windows

package main

import "context"

func readLocalWiFi(context.Context) (localWiFiSnapshot, error) {
	return localWiFiSnapshot{Interfaces: []localWiFiInterface{}, Note: "Wi-Fi profiles and stored keys come to the Linux edition in a later beta."}, nil
}
func revealLocalWiFi(context.Context, localWiFiRevealRequest) localWiFiResult {
	return localWiFiFailure("unsupported", "Wi-Fi key viewing comes to the Linux edition in a later beta.", false)
}
