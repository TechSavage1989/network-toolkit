package main

import (
	"context"
	"time"

	"networktoolkit/internal/model"
)

func diagnosticJobContext(parent context.Context, req model.Request) (context.Context, context.CancelFunc) {
	scope, _ := req.Options["scope"].(string)
	if req.Tool == "monitor" || req.Tool == "ping" && scope != "subnet" {
		return context.WithCancel(parent)
	}
	return context.WithTimeout(parent, 2*time.Hour)
}
