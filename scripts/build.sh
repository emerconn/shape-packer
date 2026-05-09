#!/bin/sh
set -eu

GOOS="${1:?Usage: build.sh <goos> <goarch> <mode> [output]}"
GOARCH="${2:?}"
MODE="${3:?}"
OUTPUT="${4:-shape-packer}"

if [ "$MODE" = "slim" ]; then
  CGO_ENABLED=0 GOAMD64=v3 GOARM64=v8.0,lse,crypto \
    GOOS="$GOOS" GOARCH="$GOARCH" \
    go build -ldflags="-s -w" -trimpath -o "$OUTPUT" .
else
  CGO_ENABLED=0 GOAMD64=v3 GOARM64=v8.0,lse,crypto \
    GOOS="$GOOS" GOARCH="$GOARCH" \
    go build -o "$OUTPUT" .
fi
