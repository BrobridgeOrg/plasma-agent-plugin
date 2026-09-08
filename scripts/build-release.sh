#!/usr/bin/env bash
set -euo pipefail
root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$root"
version="$(cat VERSION)"
[[ "$version" =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]] || { echo "Invalid VERSION" >&2; exit 1; }
if [[ -n "${GITHUB_REF_NAME:-}" && "$GITHUB_REF_NAME" != "v$version" ]]; then
  echo "Release tag must match VERSION (v$version)." >&2
  exit 1
fi
dist="$root/dist/v$version"
mkdir -p "$dist"
staging="$(mktemp -d "$dist/.build.XXXXXXXX")"
trap 'rm -rf "$staging"' EXIT
module="github.com/BrobridgeOrg/plasma-plugin"
ldflags="-s -w -X $module/internal/plasmamcp.Version=$version -X $module/internal/ophionproxy.Version=$version"
for os in darwin linux; do
  for arch in arm64 amd64; do
    echo "Building ${os}/${arch}..." >&2
    CGO_ENABLED=0 GOOS="$os" GOARCH="$arch" go build -trimpath -ldflags "$ldflags" \
      -o "$staging/plasma-plugin-mcp" ./cmd/plasma-plugin-mcp
    COPYFILE_DISABLE=1 tar -czf "$dist/plasma-plugin-mcp_${version}_${os}_${arch}.tar.gz" -C "$staging" plasma-plugin-mcp
  done
done
cd "$dist"
if command -v sha256sum >/dev/null 2>&1; then
  sha256sum plasma-plugin-mcp_"${version}"_*.tar.gz > checksums.txt
else
  shasum -a 256 plasma-plugin-mcp_"${version}"_*.tar.gz > checksums.txt
fi
echo "Release assets: $dist" >&2
