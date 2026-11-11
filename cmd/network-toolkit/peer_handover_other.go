//go:build !windows

package main

// peerHandoverRemoteDrive: elsewhere a network mount looks like any folder;
// the receiving editor opens the path through its normal local open.
func peerHandoverRemoteDrive(string) bool { return false }
