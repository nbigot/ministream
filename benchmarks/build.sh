#!/bin/sh
# Builds, for Linux, the load generator (bin/loader) and the ministream server of this
# repository (bin/ministream-<name>, name "current" by default).
# usage: ./build.sh [name]
set -e
cd "$(dirname "$0")"
mkdir -p bin
export CGO_ENABLED=0 GOOS=linux GOARCH=amd64
go build -o bin/loader .
cd ..
go build -o "benchmarks/bin/ministream-${1:-current}" ./cmd/ministream
