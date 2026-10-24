#!/usr/bin/env sh
set -eu
cd "$(dirname "$0")"
mkdir -p build
CGO_ENABLED=0 GOOS=windows GOARCH=amd64 GOTOOLCHAIN=local go build -mod=vendor -buildvcs=false -trimpath -ldflags='-s -w -H windowsgui' -o build/Network_Toolkit.exe .
