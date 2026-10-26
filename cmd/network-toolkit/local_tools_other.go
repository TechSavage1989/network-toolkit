//go:build !windows

package main

import (
	"context"
	"os"
)

func localPathIsReparse(os.FileInfo) bool { return false }

func localToolsAvailable() bool { return false }
func readLocalHosts(context.Context) (localHostsSnapshot, error) {
	return localHostsSnapshot{Warnings: []string{}, Note: errLocalToolsUnavailable.Error()}, nil
}
func readLocalIP(context.Context) (localIPSnapshot, error) {
	return localIPSnapshot{Adapters: []localIPAdapter{}, Note: errLocalToolsUnavailable.Error()}, nil
}
func applyLocalTools(context.Context, localToolsRequest) localApplyResult {
	return localApplyResult{Error: errLocalToolsUnavailable.Error(), ErrorCode: "unsupported"}
}
func runLocalToolsHelper(string, uint32) error { return errLocalToolsUnavailable }
