//go:build windows && toolkit_terminal_handoff

package main

import "testing"

func TestTerminalHandoffPrivateArgumentsAndSignal(t *testing.T) {
	for _, invalid := range [][]string{
		{},
		{"1", "2", "3", "4"},
		{"1", "2", "3", "4", "5", "--data-dir"},
		{"1", "2", "3", "4", "0"},
		{"1", "2", "3", "4", "4"},
		{"1", "2", "3", "4", "05"},
		{"1", "2", "3", "4", "-1"},
	} {
		if _, err := parseTerminalHandoffHandles(invalid); err == nil {
			t.Fatalf("accepted %v", invalid)
		}
	}
	h, err := parseTerminalHandoffHandles([]string{"1", "2", "3", "4", "5"})
	if err != nil || h != (terminalHandoffHandles{1, 2, 3, 4, 5}) {
		t.Fatalf("handles %+v, %v", h, err)
	}
	if got := terminalHandoffResizePacket(300, 45); got != ([6]byte{8, 0, 44, 1, 45, 0}) {
		t.Fatalf("resize signal = %v", got)
	}
}
