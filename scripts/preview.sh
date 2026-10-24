#!/usr/bin/env sh
set -eu
cd "$(dirname "$0")/.."
exec ./build/NetworkToolkit-linux --no-browser --port 4173 --preview-host terminal.local:4173 --data-dir /tmp/network-toolkit-preview
