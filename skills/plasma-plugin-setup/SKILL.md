---
name: plasma-plugin-setup
description: >-
  當 Plasma 或 Ophion MCP 工具缺少、連線失敗或驗證失敗，或需要初次安裝、更換部署、設定 workspace 時使用。以台灣繁體中文處理 config.env、狀態檔、預編譯執行檔及錯誤診斷，並說明同步與開 API 的確認時點。
---

# Setting up and diagnosing the plugin

## 共通互動原則

- 所有對使用者的回覆都使用**台灣繁體中文**，包含進度、問題、結果、錯誤說明、確認文字及交付說明。工具名稱、SQL、欄位名稱、URL 與需忠實引用的原文保留原樣，並以台灣繁體中文解釋。
- 在使用者已交付的任務範圍內，連續完成知識查找、欄位查核、SQL 驗證及不會啟動同步的 mview 建立；報告進度即可，不要每完成一步就問「是否繼續」。只有缺少會影響正確性的必要資訊時才釐清，釐清不等於每一步都要核准。
- 確認集中在兩個執行時點：**開始同步拉資料**，以及同步成功後**開啟資料 API**。每次以中文清楚說明具體影響；同一動作不要先在對話問一次、又重複要求一次工具確認。若宿主提供符合需求的確認介面，使用該介面；否則以中文取得明確同意後再呼叫工具。
- **原則上一份表單／報表建立一個 mview。** 不因不同區塊、指標、頁籤或來源表就拆成多個 mview；只有使用者明確要求拆分，才改變這個原則。

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
| 同步或開 API 的確認 | `sync_view`、會立即同步的 scheduled `create_view`、`create_access_entry` 需要確認 | 以台灣繁體中文說明：sync 會開始拉取資料並寫入 mview；開 API 會讓符合存取條件的呼叫者讀取資料 |
| 查詢或建立 manual mview 仍逐次跳出確認 | 可能仍在使用舊版 hook／binary，或宿主另設了工具權限 | 檢查安裝版本與宿主設定；目前流程不額外強制這兩步確認，不能以關閉所有同步／API 確認來排障 |

## Changing deployment

Point `PLASMA_URL` / `OPHION_URL` at the other deployment and restart the
session (MCP servers read configuration at startup). Then `use_workspace`
again — a workspace id from the old deployment will not resolve on the new
one, and the tools will say so rather than address a stranger's workspace.
