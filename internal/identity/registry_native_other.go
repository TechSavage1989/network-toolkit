//go:build !windows

package identity

import "context"

func registryPlatformFallback() func(context.Context, registrySource) ([]byte, error) { return nil }
