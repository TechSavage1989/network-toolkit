//go:build !windows

package main

func localTransferDrives() []localTransferDrive {
	return []localTransferDrive{{Path: "/", Label: "Filesystem", Kind: "local"}}
}
