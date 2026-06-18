#!/usr/bin/env sh
set -eu

VERSION="${1:-dev}"

unset GOOS GOARCH
go test ./...
mkdir -p dist

build_target() {
    os="$1"
    arch="$2"
    ext="$3"
    name="jirawarden-$VERSION-$os-$arch"

    mkdir -p "dist/$name"
    GOOS="$os" GOARCH="$arch" go build \
        -ldflags "-s -w -X main.version=$VERSION" \
        -o "dist/$name/jirawarden$ext" \
        ./cmd/jirawarden

    cp README.md "dist/$name/README.md"
    if [ -f RELEASE.md ]; then cp RELEASE.md "dist/$name/RELEASE.md"; fi
    cp .env.example "dist/$name/.env.example"
    tar -C "dist/$name" -czf "dist/$name.tar.gz" .
}

build_target "windows" "amd64" ".exe"
build_target "linux" "amd64" ""
build_target "darwin" "amd64" ""
build_target "darwin" "arm64" ""

echo "Release files are in dist/"
