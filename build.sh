#!/usr/bin/env sh
set -eu
cd "$(dirname "$0")"
mkdir -p build
CGO_ENABLED=0 GOOS=windows GOARCH=amd64 GOTOOLCHAIN=local go build -mod=vendor -buildvcs=false -trimpath -ldflags='-s -w -H windowsgui' -o build/Network_Toolkit.exe ./cmd/network-toolkit
# Linux edition (docs/LINUX_PLAN.md): static builds for the portable .tar.gz files.
for arch in amd64 arm64; do
  mkdir -p "build/linux-$arch"
  CGO_ENABLED=0 GOOS=linux GOARCH="$arch" GOTOOLCHAIN=local go build -mod=vendor -buildvcs=false -trimpath -ldflags='-s -w' -o "build/linux-$arch/network-toolkit" ./cmd/network-toolkit
done
