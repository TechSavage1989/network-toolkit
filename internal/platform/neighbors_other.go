//go:build !windows && !linux

package platform

import (
	"context"
	"errors"
)

func readNeighbors(ctx context.Context) ([]Neighbor, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return nil, errors.New("structured neighbor inspection is supported on Windows and Linux")
}
