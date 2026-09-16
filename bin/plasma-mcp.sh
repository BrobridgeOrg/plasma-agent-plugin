#!/usr/bin/env bash
# stdout belongs exclusively to MCP (or the hook's JSON response).
set -euo pipefail

root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
repo="BrobridgeOrg/plasma-agent-plugin"
version="$(cat "$root/VERSION")"

fail() { echo "plasma-plugin: $*" >&2; exit 1; }

# Explicit opt-in for local development; never compile during normal startup.
if [[ -n "${PLASMA_MCP_BINARY:-}" ]]; then
  [[ -x "$PLASMA_MCP_BINARY" ]] || fail "PLASMA_MCP_BINARY is not executable: $PLASMA_MCP_BINARY"
  exec "$PLASMA_MCP_BINARY" "$@"
fi

# Windows runs this launcher under Git Bash, MSYS2 or Cygwin; WSL reports Linux.
exe=""
case "$(uname -s)" in
  Darwin) os=darwin ;;
  Linux) os=linux ;;
  MINGW*|MSYS*|CYGWIN*) os=windows; exe=".exe" ;;
  *) fail "Unsupported OS. Release binaries support macOS, Linux and Windows." ;;
esac
case "$(uname -m)" in
  arm64|aarch64) arch=arm64 ;;
  x86_64|amd64) arch=amd64 ;;
  *) fail "Unsupported CPU. Release binaries support arm64 and amd64." ;;
esac

plugin_home="${PLASMA_PLUGIN_HOME:-${HOME:?HOME must be set}/.plasma-plugin}"
cache="$plugin_home/bin/v$version/$os-$arch"
binary="$cache/plasma-plugin-mcp$exe"
if [[ ! -x "$binary" ]]; then
  umask 077
  mkdir -p "$cache"
  staging="$(mktemp -d "$cache/.download.XXXXXXXX")"
  trap 'rm -rf "$staging"' EXIT
  trap 'exit 130' INT
  trap 'exit 143' TERM
  asset="plasma-plugin-mcp_${version}_${os}_${arch}.tar.gz"

  echo "plasma-plugin: Installing v$version ($os/$arch)..." >&2
  if [[ -n "${PLASMA_MCP_RELEASE_DIR:-}" ]]; then
    cp "$PLASMA_MCP_RELEASE_DIR/$asset" "$PLASMA_MCP_RELEASE_DIR/checksums.txt" "$staging/"
  elif command -v gh >/dev/null 2>&1 && gh auth status >/dev/null 2>&1; then
    gh release download "v$version" --repo "$repo" --pattern "$asset" \
      --pattern checksums.txt --dir "$staging" >&2 || \
      fail "Release download failed. Check GitHub access to $repo, or set PLASMA_MCP_RELEASE_DIR to downloaded release files."
  else
    command -v curl >/dev/null 2>&1 || fail "curl is required to download release binaries."
    base="https://github.com/$repo/releases/download/v$version"
    for file in "$asset" checksums.txt; do
      curl --fail --location --silent --show-error --retry 2 \
        --connect-timeout 15 --max-time 180 --proto '=https' --proto-redir '=https' \
        "$base/$file" --output "$staging/$file" || \
        fail "Cannot download v$version. For a private repository, authenticate GitHub CLI with 'gh auth login', or download the archive and checksums.txt in your browser and set PLASMA_MCP_RELEASE_DIR."
    done
  fi

  expected="$(awk -v name="$asset" '$2 == name {print $1}' "$staging/checksums.txt")"
  [[ "$expected" =~ ^[0-9a-f]{64}$ ]] || fail "Missing or invalid checksum for $asset."
  if command -v sha256sum >/dev/null 2>&1; then
    actual="$(sha256sum "$staging/$asset")"
  elif command -v shasum >/dev/null 2>&1; then
    actual="$(shasum -a 256 "$staging/$asset")"
  else
    fail "sha256sum or shasum is required to verify release binaries."
  fi
  [[ "${actual%% *}" == "$expected" ]] || fail "Checksum mismatch for $asset; refusing to install."

  tar -xzf "$staging/$asset" -C "$staging" "plasma-plugin-mcp$exe"
  [[ -f "$staging/plasma-plugin-mcp$exe" && ! -L "$staging/plasma-plugin-mcp$exe" ]] ||
    fail "Invalid release archive."
  chmod 700 "$staging/plasma-plugin-mcp$exe"
  # Unique staging directories and an atomic rename allow simultaneous MCP starts.
  # Windows locks the running executable, so a rename losing that race is fine.
  mv -f "$staging/plasma-plugin-mcp$exe" "$binary" 2>/dev/null || [[ -x "$binary" ]] ||
    fail "Cannot install $binary."
  rm -rf "$staging"
  trap - EXIT INT TERM
fi

exec "$binary" "$@"
