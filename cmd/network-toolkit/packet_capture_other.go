//go:build !windows

package main

import (
	"context"
	"errors"
	"networktoolkit/internal/capture"
)

func packetCaptureSupport() packetCaptureCapabilities {
	return packetCaptureCapabilities{Note: "Record live traffic from the Windows app. PCAP and PCAPNG import, inspection and export are available here.", Interfaces: []capture.Interface{}}
}
func packetCaptureInterfaces(context.Context) ([]capture.Interface, error) {
	return nil, errors.New("Capture adapters are listed in the Windows app.")
}
func packetCaptureRun(context.Context, capture.Options, <-chan struct{}, func(packetCaptureMessage) error) error {
	return errors.New("Live recording requires the Windows app.")
}
func runPacketCaptureHelper(string, uint32) error {
	return errors.New("Packet capture helpers require Windows.")
}
