#!/usr/bin/env bash
# Launcher for every mode of the plugin's binary.
#
# It builds on demand so a fresh clone (or a plugin update that replaced the
# cache directory) still starts: an MCP server that fails with "command not
# found" gives the operator nothing to act on.
#
# Build output goes to stderr without exception. stdout is the MCP channel,
# and one stray line of build chatter there corrupts the protocol.
set -euo pipefail

root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
binary="$root/bin/plasma-plugin-mcp"

needs_build=0
if [[ ! -x "$binary" ]]; then
  needs_build=1
elif [[ -d "$root/cmd" ]] && \
     [[ -n "$(find "$root/cmd" "$root/internal" -name '*.go' -newer "$binary" -print -quit 2>/dev/null)" ]]; then
  needs_build=1
fi

if [[ "$needs_build" -eq 1 ]]; then
  if ! command -v go >/dev/null 2>&1; then
    echo "plasma-plugin: Go toolchain not found and $binary is missing or stale." >&2
    echo "plasma-plugin: install Go, or build the binary yourself with 'make build'." >&2
    exit 1
  fi
  ( cd "$root" && go build -o "$binary" ./cmd/plasma-plugin-mcp ) >&2
fi

exec "$binary" "$@"
