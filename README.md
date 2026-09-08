# plasma-plugin

一個 Claude Code plugin：**用 Ophion 的知識操作 Plasma**。

含兩台 MCP server 與三份 skill，目標情境是「客戶說我想要這些資料的 API」——
從找來源、驗 SQL、建 materialized view，一路做到交出可呼叫的 Data API。

```text
Claude Code
├─ MCP「plasma」 ──REST──> Plasma /apis/v1        （JWT）
└─ MCP「ophion」 ──MCP───> Ophion query-mcp        （bearer service token）
        兩台共讀 ~/.plasma-plugin/state.json（當下 workspace）
```

## 動態 workspace 是怎麼做到的

Claude Code 不會在 session 中重連 MCP server，所以「動態」不靠改設定檔：

- `ophion` 那台是 **proxy**——對 Claude Code 是固定的 stdio server，對上游是
  MCP client，**每次呼叫都重讀 state 檔**，用當下的 workspace 撥
  `<ophion>/internal/v1/workspaces/<ws>/query-mcp/<profile>`。
- 切換由 `plasma` 的 `use_workspace` 寫檔，下一次 ophion 呼叫立即改道。
- **每筆 ophion 回應開頭都標 `[workspace=… name=… profile=…]`**。隱藏狀態最大的
  風險是「以為切了、其實沒切」，標注讓它變成看得見的錯。

## 安裝

```bash
# 從 GitHub 安裝（私有 repo 需有讀取權限）
/plugin marketplace add BrobridgeOrg/plasma-agent-plugin
/plugin install plasma-plugin@plasma-plugin-local
```

**不需要安裝 Go、Node.js 或 Docker。** 支援 macOS／Linux 的 arm64、amd64；
Windows 請使用 WSL。首次啟動會下載與 plugin 版本一致的 GitHub Release 執行檔，
驗證 SHA-256 後快取到 `~/.plasma-plugin/bin/<version>/<os>-<arch>/`，後續直接執行。
需要 Bash、tar、curl，以及 shasum 或 sha256sum；下載訊息只寫到 stderr。

私有 repo 的自動下載使用已登入的 GitHub CLI（`gh auth login`）。若不想安裝 gh，
可在瀏覽器從 [Releases](https://github.com/BrobridgeOrg/plasma-agent-plugin/releases)
下載對應平台的 `.tar.gz` 與 `checksums.txt` 到同一目錄，再啟動 Claude Code：

```bash
PLASMA_MCP_RELEASE_DIR="$HOME/Downloads/plasma-release" claude
```

第一次安裝完成後不再需要此環境變數。更新 plugin 時需提供新版的下載檔案，或讓
launcher 透過 GitHub 下載。快取與設定都支援以 `PLASMA_PLUGIN_HOME` 改變根目錄。

設定放 `~/.plasma-plugin/config.env`（**不要**放在 plugin 目錄，marketplace
更新會換掉整個目錄）：

```bash
mkdir -p ~/.plasma-plugin
cp config.env.example ~/.plasma-plugin/config.env && chmod 600 ~/.plasma-plugin/config.env
```

Ophion 的 query-mcp 是叢集內 API，工作站通常要 port-forward：

```bash
kubectl -n <namespace> port-forward svc/ophion 5101:5101
```

裝好後在 session 裡叫 `whoami`——兩個 endpoint、登入者、當下 workspace 一次看完。

## 工具

**plasma（11 顆）**

| 工具 | 用途 |
|---|---|
| `whoami` | 兩邊 endpoint、登入者、當下 workspace（排障第一站） |
| `list_workspaces` / `use_workspace` | 列出並選擇 workspace（ophion 那台跟著走） |
| `list_views` / `get_view` | 看 view／mview 與同步狀態 |
| `run_query` | 在 workspace 跑一段 SELECT（**需同意**） |
| `create_view` | 建 view／mview，帶排程時一次建完 view+blueprint+schedule（**需同意**） |
| `sync_view` | 觸發一次同步（**需同意**） |
| `create_access_entry` | 把 view 發布成對外 Data API（**需同意**） |
| `list_access_entries` / `get_export_url` | 看既有發布與取回 URL |

沒有 `list_tables`：能不能查、怎麼查是 Ophion 帶 `access_mode` 的權威判斷，
Plasma 這側再列一份表清單只會多一個會對不上的來源。

**ophion**：上游 profile 的知識工具原封轉發，外加 `ophion_context()` 診斷。

## 使用者同意

`run_query`、`create_view`、`sync_view`、`create_access_entry` 由 plugin 自帶的
`PreToolUse` hook 強制成 `ask`：**即使整台 MCP server 被加進 allowlist，這四顆
仍然每次跳確認**，而確認框會顯示完整參數，所以按下去之前看得到要跑的 SQL。

## 開發

```bash
make check     # fmt + vet + Go tests + build + launcher tests
make test
```

開發者才需要 Go（版本見 `go.mod`）與 Python 3（launcher 測試）。正常啟動不會
自動 build，也不會自動採用 repo 內可能過期的 binary。要測試本機修改：

```bash
make build
PLASMA_MCP_BINARY="$PWD/bin/plasma-plugin-mcp" claude --plugin-dir "$PWD"
```

## 發佈

更新 `VERSION`、`.claude-plugin/plugin.json` 的版本及 `releases/v<version>.md`，
提交後推送對應的 `v<version>` tag。GitHub Actions 會執行檢查、建置四種平台的
執行檔，並在目前 repo 發佈 Release 與 `checksums.txt`。本機可用 `make release`
產生相同格式的資產，輸出在 `dist/v<version>/`。
