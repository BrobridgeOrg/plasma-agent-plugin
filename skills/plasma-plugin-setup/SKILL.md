---
name: plasma-plugin-setup
description: Use when the plasma or ophion MCP tools are missing, erroring, or unauthenticated — first-time setup of this plugin, "no workspace selected", 401/403 from Plasma, 404 or unreachable from Ophion, or after moving to another machine or deployment. Covers config.env, the state file, building the binary, and reading each failure.
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

The binary builds itself on first launch when a Go toolchain is present. To
build it deliberately:

```bash
make -C "$CLAUDE_PLUGIN_ROOT" build
```

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
