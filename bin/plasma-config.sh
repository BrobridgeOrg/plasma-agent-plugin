#!/usr/bin/env bash
# Reads and writes ~/.plasma-plugin/config.env for the plasma-plugin-setup skill.
#
# The skill drives the guided route through this script rather than editing the
# file itself, so every write lands on a known key, keeps mode 600, and cannot
# corrupt a hand-edited file. Secrets stay out of the transcript: `set-secret`
# reads them from the terminal and `show` never prints them back.
set -euo pipefail

KEYS=(PLASMA_URL PLASMA_TOKEN PLASMA_USERNAME PLASMA_PASSWORD
      OPHION_URL OPHION_SERVICE_TOKEN OPHION_PROFILE)
SECRETS=(PLASMA_TOKEN PLASMA_PASSWORD OPHION_SERVICE_TOKEN)

root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
# An unlocatable home is not fatal here: session-start has to stay quiet in an
# environment it cannot read, and every other command says so itself.
if [[ -n "${PLASMA_PLUGIN_HOME:-}" ]]; then
  plugin_home="$PLASMA_PLUGIN_HOME"
elif [[ -n "${HOME:-}" ]]; then
  plugin_home="$HOME/.plasma-plugin"
else
  plugin_home=""
fi
config="$plugin_home/config.env"

fail() { echo "plasma-plugin: $*" >&2; exit 1; }

require_home() {
  [[ -n "$plugin_home" ]] || fail "cannot locate the plugin home: set HOME or PLASMA_PLUGIN_HOME"
}

contains() {
  local needle=$1 item
  shift
  for item in "$@"; do [[ "$item" == "$needle" ]] && return 0; done
  return 1
}

require_key() {
  contains "$1" "${KEYS[@]}" || fail "unknown key: $1 (known keys: ${KEYS[*]})"
}

# file_value prints what config.env holds for a key, parsed the way the Go side
# parses it: last assignment wins, surrounding quotes stripped.
file_value() {
  [[ -f "$config" ]] || return 0
  KEY="$1" awk '
    {
      line = $0
      sub(/^[ \t]+/, "", line); sub(/[ \t]+$/, "", line)
      if (line ~ /^#/ || line == "") next
      i = index(line, "="); if (i == 0) next
      k = substr(line, 1, i - 1); sub(/[ \t]+$/, "", k)
      if (k != ENVIRON["KEY"]) next
      v = substr(line, i + 1); sub(/^[ \t]+/, "", v); sub(/[ \t]+$/, "", v)
      if (length(v) >= 2) {
        q = substr(v, 1, 1)
        if ((q == "\"" || q == "'"'"'") && substr(v, length(v), 1) == q)
          v = substr(v, 2, length(v) - 2)
      }
      found = v; seen = 1
    }
    END { if (seen) print found }
  ' "$config"
}

# value is what the servers would actually see: the environment wins over the file.
value() {
  local key=$1 from_env=${!1:-}
  if [[ -n "$from_env" ]]; then printf '%s' "$from_env"; return; fi
  file_value "$key"
}

source_of() {
  local key=$1
  if [[ -n "${!key:-}" ]]; then echo env
  elif [[ -n "$(file_value "$key")" ]]; then echo file
  else echo unset
  fi
}

ensure_file() {
  require_home
  umask 077
  mkdir -p "$plugin_home"
  chmod 700 "$plugin_home" 2>/dev/null || true
  if [[ ! -f "$config" ]]; then
    if [[ -f "$root/config.env.example" ]]; then
      cp "$root/config.env.example" "$config"
    else
      : > "$config"
    fi
  fi
  chmod 600 "$config" 2>/dev/null || true
}

# upsert replaces every assignment of the key with one line, or appends it.
upsert() {
  local key=$1 value=$2 tmp
  ensure_file
  tmp="$(mktemp "$plugin_home/.config.XXXXXXXX")"
  chmod 600 "$tmp"
  KEY="$key" VALUE="$value" awk '
    BEGIN { key = ENVIRON["KEY"]; value = ENVIRON["VALUE"] }
    {
      line = $0
      t = line; sub(/^[ \t]+/, "", t)
      i = index(t, "=")
      if (t !~ /^#/ && i > 0) {
        k = substr(t, 1, i - 1); sub(/[ \t]+$/, "", k)
        if (k == key) { if (!done) { print key "=" value; done = 1 }; next }
      }
      print line
    }
    END { if (!done) print key "=" value }
  ' "$config" > "$tmp"
  mv -f "$tmp" "$config"
}

cmd_init() {
  ensure_file
  echo "$config"
}

cmd_set() {
  local key=${1:-} value=${2-}
  [[ -n "$key" ]] || fail "usage: plasma-config.sh set KEY VALUE"
  require_key "$key"
  [[ "$value" != *$'\n'* ]] || fail "value for $key must be a single line"
  upsert "$key" "$value"
  if contains "$key" "${SECRETS[@]}"; then
    echo "$key written (${#value} characters)"
  else
    echo "$key=$value"
  fi
}

# cmd_set_secret keeps a token or password off the command line and out of the
# conversation: it is typed into the terminal and goes straight into the file.
cmd_set_secret() {
  local key=${1:-} value saved=""
  [[ -n "$key" ]] || fail "usage: plasma-config.sh set-secret KEY"
  require_key "$key"
  contains "$key" "${SECRETS[@]}" || fail "$key is not a secret; use 'set $key VALUE'"
  { exec 3<>/dev/tty; } 2>/dev/null || \
    fail "set-secret needs a terminal. Run it yourself in a shell, or use 'set $key VALUE'."
  # Turn echo off through the terminal itself: bash 3.2, which is what macOS
  # ships, only applies `read -s` to standard input, and this reads /dev/tty.
  saved="$(stty -g <&3 2>/dev/null)" || saved=""
  if [[ -n "$saved" ]]; then
    trap 'stty "$saved" <&3 2>/dev/null || true' EXIT INT TERM
    stty -echo <&3 2>/dev/null || true
  fi
  printf '%s (input is hidden): ' "$key" >&3
  IFS= read -rs value <&3 || fail "no value read"
  if [[ -n "$saved" ]]; then
    stty "$saved" <&3 2>/dev/null || true
    trap - EXIT INT TERM
  fi
  printf '\n' >&3
  exec 3>&-
  [[ -n "$value" ]] || fail "$key left unchanged: nothing was entered"
  upsert "$key" "$value"
  echo "$key written (${#value} characters)"
}

cmd_show() {
  local key v src
  require_home
  if [[ -f "$config" ]]; then
    echo "config=$config (exists, mode $(ls -l "$config" | cut -c1-10))"
  else
    echo "config=$config (missing)"
  fi
  for key in "${KEYS[@]}"; do
    v="$(value "$key")"; src="$(source_of "$key")"
    if [[ -z "$v" ]]; then
      echo "$key=<unset>"
    elif contains "$key" "${SECRETS[@]}"; then
      echo "$key=<set, ${#v} characters> [$src]"
    else
      echo "$key=$v [$src]"
    fi
  done
}

# join_lines concatenates stdin with a separator. `paste -d` cannot do this:
# POSIX makes -d a list of delimiters used cyclically, so a multi-character
# separator alternates, and in the C locale a multibyte one is torn into
# invalid bytes.
join_lines() {
  SEP="$1" awk 'NR > 1 { printf "%s", ENVIRON["SEP"] } { printf "%s", $0 } END { if (NR) print "" }'
}

# missing prints the human-readable name of everything still needed.
missing() {
  local out=()
  [[ -n "$(value PLASMA_URL)" ]] || out+=(PLASMA_URL)
  if [[ -z "$(value PLASMA_TOKEN)" ]] &&
     { [[ -z "$(value PLASMA_USERNAME)" ]] || [[ -z "$(value PLASMA_PASSWORD)" ]]; }; then
    out+=("Plasma credentials")
  fi
  [[ -n "$(value OPHION_URL)" ]] || out+=(OPHION_URL)
  [[ -n "$(value OPHION_SERVICE_TOKEN)" ]] || out+=(OPHION_SERVICE_TOKEN)
  [[ ${#out[@]} -eq 0 ]] || printf '%s\n' "${out[@]}"
}

cmd_check() {
  local gaps
  gaps="$(missing)"
  cmd_show
  if [[ -z "$gaps" ]]; then
    echo "status=complete"
    return 0
  fi
  echo "status=incomplete"
  echo "missing=$(printf '%s\n' "$gaps" | join_lines ', ')"
  return 1
}

# cmd_probe reaches the two endpoints without credentials: it separates "wrong
# URL / port-forward is down" from "reachable but rejected me", which are the
# two failures a fresh setup actually hits.
cmd_probe() {
  require_home
  command -v curl >/dev/null 2>&1 || fail "curl is required for probe"
  local key url code
  for key in PLASMA_URL OPHION_URL; do
    url="$(value "$key")"
    if [[ -z "$url" ]]; then echo "$key=<unset>"; continue; fi
    if code="$(curl --silent --output /dev/null --write-out '%{http_code}' \
                    --max-time 5 --location "$url" 2>/dev/null)" && [[ "$code" != 000 ]]; then
      echo "$key=$url reachable (HTTP $code)"
    else
      echo "$key=$url unreachable"
    fi
  done
}

# cmd_session_start nudges a session that cannot work yet, and is silent
# otherwise. It never fails: a SessionStart hook that errors is noise at the
# top of every session.
cmd_session_start() {
  local gaps list
  [[ -n "$plugin_home" ]] || return 0
  gaps="$(missing 2>/dev/null)" || true
  [[ -n "$gaps" ]] || return 0
  list="$(printf '%s\n' "$gaps" | join_lines '、')"
  printf '{"hookSpecificOutput":{"hookEventName":"SessionStart","additionalContext":"%s"}}\n' \
    "plasma-plugin 尚未完成設定，缺少：${list}。使用者一旦想使用 Plasma 或 Ophion 工具，先執行 plasma-plugin-setup skill 帶他完成設定，不要先呼叫這兩台 MCP server 的工具。"
}

case "${1:-}" in
  path) require_home; echo "$config" ;;
  init) shift; cmd_init "$@" ;;
  set) shift; cmd_set "$@" ;;
  set-secret) shift; cmd_set_secret "$@" ;;
  show) shift; cmd_show "$@" ;;
  check) shift; cmd_check "$@" ;;
  probe) shift; cmd_probe "$@" ;;
  session-start) cmd_session_start || true ;;
  *)
    fail "usage: plasma-config.sh <path|init|set KEY VALUE|set-secret KEY|show|check|probe|session-start>" ;;
esac
