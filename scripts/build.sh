#!/usr/bin/env sh
set -eu

VERSION="${1:-dev}"

unset GOOS GOARCH
go test ./...
mkdir -p bin
go build \
    -ldflags "-s -w -X main.version=$VERSION" \
    -o bin/jirawarden \
    ./cmd/jirawarden

echo "Built bin/jirawarden"
