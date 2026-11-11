#!/usr/bin/env sh
set -eu
cd "$(dirname "$0")"
mkdir -p build
CGO_ENABLED=0 GOOS=windows GOARCH=amd64 GOTOOLCHAIN=local go build -mod=vendor -buildvcs=false -trimpath -ldflags='-s -w -H windowsgui' -o build/Network_Toolkit.exe ./cmd/network-toolkit
# The separate Text Editor and Terminal programs (docs/SEPARATE_EXECUTABLES.md):
# the same code with a build tag. Each links its own resources from
# internal/winres; the overlay leaves the main program's resource object out
# (Go refuses a program with two resource sections).
printf '{"Replace":{"%s/cmd/network-toolkit/resource_windows_amd64.syso":""}}\n' "$(pwd)" > build/separate-programs.overlay.json
for edition in editor:Network_Toolkit_TextEditor terminal:Network_Toolkit_Terminal; do
  CGO_ENABLED=0 GOOS=windows GOARCH=amd64 GOTOOLCHAIN=local go build -mod=vendor -buildvcs=false -trimpath -tags "ntk_${edition%%:*}" -overlay build/separate-programs.overlay.json -ldflags='-s -w -H windowsgui' -o "build/${edition#*:}.exe" ./cmd/network-toolkit
done
# Linux edition (docs/LINUX_PLAN.md): static builds for the portable .tar.gz files.
for arch in amd64 arm64; do
  mkdir -p "build/linux-$arch"
  CGO_ENABLED=0 GOOS=linux GOARCH="$arch" GOTOOLCHAIN=local go build -mod=vendor -buildvcs=false -trimpath -ldflags='-s -w' -o "build/linux-$arch/network-toolkit" ./cmd/network-toolkit
done
