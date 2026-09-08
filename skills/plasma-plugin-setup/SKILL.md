---
name: plasma-plugin-setup
description: Use when the plasma or ophion MCP tools are missing, erroring, or unauthenticated — first-time setup of this plugin, "no workspace selected", 401/403 from Plasma, 404 or unreachable from Ophion, or after moving to another machine or deployment. Covers config.env, the state file, installing the release binary, and reading each failure.
---

# Setting up and diagnosing the plugin

## One-time setup

Configuration lives **outside** the plugin directory, because a plugin
installed from a marketplace is replaced on update:

```bash
mkdir -p ~/.plasma-plugin
cp "$CLAUDE_PLUGIN_ROOT/config.env.example" ~/.plasma-plugin/config.env
chmod 600 ~/.plasma-plugin/config.env
```

Fill in:

- `PLASMA_URL`, plus either `PLASMA_TOKEN` (used verbatim, never refreshed) or
  `PLASMA_USERNAME` + `PLASMA_PASSWORD` (the plugin logs in and maintains the
  JWT itself).
- `OPHION_URL` and `OPHION_SERVICE_TOKEN`. Ophion's query-mcp is a
  cluster-internal API, so from a workstation this is usually a port-forward:

```bash
kubectl -n <namespace> port-forward svc/ophion 5101:5101
```

Environment variables override the file, so a single session can be pointed
elsewhere without editing it.

The launcher downloads the exact plugin version's prebuilt binary on first
launch, verifies SHA-256, and caches it under
`~/.plasma-plugin/bin/v<version>/<os>-<arch>/`. Users do not need Go.
Supported platforms are macOS/Linux, arm64/amd64 (Windows through WSL).

Private repository downloads require an authenticated GitHub CLI (`gh auth
login`). Alternatively, download the matching archive and `checksums.txt`
from the repository's GitHub Release into one directory, and launch Claude
Code with `PLASMA_MCP_RELEASE_DIR=/absolute/path/to/that/directory` in its
environment. This also supports offline installation. The variable is only
needed while installing a version that is not cached yet.

For local development only, run `make build` and set `PLASMA_MCP_BINARY` to
the absolute path of `bin/plasma-plugin-mcp`. Normal startup never builds or
implicitly uses that development binary.

Then start a session and call `whoami`. It reports both endpoints, the
authenticated user and the selected workspace — that one call is the whole
health check.

## State

`~/.plasma-plugin/state.json` (mode 600) holds the selected workspace, the
profile and the cached JWT. Both servers re-read it on every call, which is
how `use_workspace` in the plasma server re-points the ophion server without a
restart. It persists across sessions, so a new session usually starts on the
workspace you left. Deleting the file is safe: it only clears the selection
and forces a fresh login.

## Reading the failures

| What you see | What it means | What to do |
|---|---|---|
| `Cannot download` / `Release download failed` | Release unavailable or GitHub access missing | Check access to `BrobridgeOrg/plasma-agent-plugin`, authenticate gh, or use `PLASMA_MCP_RELEASE_DIR` |
| `Checksum mismatch` | Archive does not match the release checksums | Download both files again from the same release; do not bypass verification |
| `no Plasma credentials` | Neither auth mode is configured | Set `PLASMA_TOKEN`, or username + password |
| Plasma `401` after retry | Password rejected, or a static token expired | Re-check the credentials; a static token is never refreshed for you |
| Plasma `403` | Authenticated, but not a member of this workspace | `list_workspaces` and pick one you belong to |
| `no workspace selected` | Nothing selected yet | `use_workspace` |
| Ophion `no published knowledge` (404) | The workspace exists but has no published generation | Nothing to fix here; the knowledge has to be generated first |
| Ophion `rejected the service token` | `OPHION_SERVICE_TOKEN` wrong or unset | Fix it in `config.env` |
| Ophion `unreachable` | Nothing listening at `OPHION_URL` | Start the port-forward, or point at the in-cluster URL |
| `OPHION_URL is not set` | Ophion half-configured | Set it; the plugin refuses to guess an endpoint |
| A tool prompt you cannot skip | `run_query`, `create_view`, `sync_view` and `create_access_entry` are gated by a `PreToolUse` hook | Intended. Read the arguments in the prompt and approve or decline |

## Changing deployment

Point `PLASMA_URL` / `OPHION_URL` at the other deployment and restart the
session (MCP servers read configuration at startup). Then `use_workspace`
again — a workspace id from the old deployment will not resolve on the new
one, and the tools will say so rather than address a stranger's workspace.
