#!/bin/sh
# Cross-compile AnticaterKnob.exe for 64-bit Windows. Works from macOS, Linux
# or Windows (Git Bash); needs only Go, no extra modules.
set -e
cd "$(dirname "$0")"
GOOS=windows GOARCH=amd64 go build -trimpath -ldflags "-s -w -H=windowsgui" -o AnticaterKnob.exe .
echo "Built $(pwd)/AnticaterKnob.exe"
