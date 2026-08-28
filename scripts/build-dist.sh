#!/usr/bin/env bash
# build-dist.sh — 交叉编译发布物：四平台 tarball + sha256sums.txt
#
# 用法：VERSION=1.1.0 ./scripts/build-dist.sh
# 产物：dist/llm-api-check_<version>_<os>_<arch>.tar.gz + dist/sha256sums.txt
# dist/ 不入库（见 .gitignore）。
set -euo pipefail

cd "$(dirname "$0")/.."
VERSION="${VERSION:-$(grep -oE 'var version = "[^"]+"' main.go | sed 's/.*"\(.*\)"/\1/')}"
OUT=dist
PLATFORMS=(linux/amd64 linux/arm64 darwin/amd64 darwin/arm64)
BIN=llm-api-check

rm -rf "$OUT"
mkdir -p "$OUT"
echo "版本: $VERSION"

for p in "${PLATFORMS[@]}"; do
  os="${p%/*}"; arch="${p#*/}"
  stage="$OUT/stage-${os}-${arch}"
  rm -rf "$stage"; mkdir -p "$stage"
  CGO_ENABLED=0 GOOS="$os" GOARCH="$arch" \
    go build -trimpath -ldflags="-s -w -X main.version=$VERSION" -o "$stage/$BIN" .
  cp LICENSE "$stage/LICENSE" 2>/dev/null || true
  COPYFILE_DISABLE=1 tar -C "$stage" -czf "$OUT/${BIN}_${VERSION}_${os}_${arch}.tar.gz" "$BIN" LICENSE 2>/dev/null \
    || tar -C "$stage" -czf "$OUT/${BIN}_${VERSION}_${os}_${arch}.tar.gz" "$BIN"
  rm -rf "$stage"
  echo "  ✓ ${BIN}_${VERSION}_${os}_${arch}.tar.gz"
done

( cd "$OUT" && rm -f sha256sums.txt && sha256sum ${BIN}_*.tar.gz > sha256sums.txt )
echo
cat "$OUT/sha256sums.txt"
