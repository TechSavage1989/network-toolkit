//go:build !windows

package desktop

import (
	"context"
	"errors"
)

// Run is unsupported on non-Windows development hosts. It deliberately does
// not launch a browser as a substitute for the embedded Windows app window.
func Run(ctx context.Context, cfg Config) error {
	if cfg.Stop != nil {
		cfg.Stop()
	}
	return errors.New("the embedded Network Toolkit window requires Windows and Microsoft Edge WebView2 Runtime")
}
