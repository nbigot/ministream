#!/bin/bash
#MINISTREAM_VERSION="v1.0.0"
MINISTREAM_VERSION="${TAG_VERSION-$(git describe --tags --abbrev=0)}"
go build -ldflags="-X 'main.Version=${MINISTREAM_VERSION}'" cmd/ministream/ministream.go
