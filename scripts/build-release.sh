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
# CI can narrow this to one platform; a release always builds every target.
targets="${PLASMA_RELEASE_TARGETS:-darwin/arm64 darwin/amd64 linux/arm64 linux/amd64 windows/amd64 windows/arm64}"
for target in $targets; do
  os="${target%%/*}"
  arch="${target##*/}"
  [[ "$os" != "$target" && "$arch" != "$target" ]] || { echo "Invalid target: $target" >&2; exit 1; }
  # Windows refuses to execute a file without the .exe extension.
  binary=plasma-plugin-mcp
  if [[ "$os" == windows ]]; then binary=plasma-plugin-mcp.exe; fi
  echo "Building ${os}/${arch}..." >&2
  CGO_ENABLED=0 GOOS="$os" GOARCH="$arch" go build -trimpath -ldflags "$ldflags" \
    -o "$staging/$binary" ./cmd/plasma-plugin-mcp
  COPYFILE_DISABLE=1 tar -czf "$dist/plasma-plugin-mcp_${version}_${os}_${arch}.tar.gz" -C "$staging" "$binary"
  rm -f "$staging/$binary"
done
cd "$dist"
if command -v sha256sum >/dev/null 2>&1; then
  sha256sum plasma-plugin-mcp_"${version}"_*.tar.gz > checksums.txt
else
  shasum -a 256 plasma-plugin-mcp_"${version}"_*.tar.gz > checksums.txt
fi
echo "Release assets: $dist" >&2
