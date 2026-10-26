//go:build windows

package main

func lockUpdateRetentionDirectory(root string) (func(), error) {
	return lockUpdateNamingDirectory(root)
}
