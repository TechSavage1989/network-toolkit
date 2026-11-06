//go:build !windows

package main

import (
	"context"
	"errors"
	"networktoolkit/internal/capture"
)

func packetCaptureSupport() packetCaptureCapabilities {
	return packetCaptureCapabilities{Note: "Recording live traffic comes to the Linux edition in a later beta. PCAP and PCAPNG import, inspection and export work now.", Interfaces: []capture.Interface{}}
}
func packetCaptureInterfaces(context.Context) ([]capture.Interface, error) {
	return nil, errors.New("Capture adapters are listed once live recording arrives in the Linux edition (a later beta).")
}
func packetCaptureRun(context.Context, capture.Options, <-chan struct{}, func(packetCaptureMessage) error) error {
	return errors.New("Live recording comes to the Linux edition in a later beta.")
}
func runPacketCaptureHelper(string, uint32) error {
	return errors.New("Live recording comes to the Linux edition in a later beta.")
}
