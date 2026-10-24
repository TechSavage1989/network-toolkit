//go:build !windows

package diag

import (
	"context"
	"errors"
	"time"
)

func systemDNSWithTTL(ctx context.Context, name, typ string, timeout time.Duration) ([]DNSRecord, error) {
	return nil, errors.New("Windows native DNS query is unavailable")
}
