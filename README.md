# plasma-plugin

一個 Claude Code plugin：**用 Ophion 的知識操作 Plasma**。

含兩台 MCP server 與四份 skill，支援資料 API 與指定 PG 資料表兩種交付方式：
資料 API 使用 mview；PG 匯出依 view → blueprint → PG 執行。

- `plasma-plugin-setup`：連線設定與診斷。
- `ophion-knowledge-lookup`：查核來源、欄位與業務規則。
- `plasma-data-api`：建立 mview、同步並發布資料 API。
- `plasma-postgres-export`：建立 view，供 blueprint 選取後匯出至指定 PG 資料表。

Go 工具 `create_view(type=view)` 建立一般 view 定義，不帶 mview 同步設定。
PG 連線先從 Ophion MCP 的已串接 DB 清單整理選項，反問使用者選擇，再核對 Plasma
DBC。`create_pg_blueprint` 綁定 view 與所選 PG DBC，建立後不執行；確認同步後使用
`spawn_blueprint_job`，再透過 jobs 工具追蹤本次結果。支援 `append`、`overwrite`、
`truncate`，沒有 upsert；資料庫與 schema 沿用 DBC。新工具提供單次執行，尚未封裝
blueprint 排程或直接查詢 PG 資料內容。

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

裝完的**第一件事**是跑設定：

```text
/plasma-plugin:plasma-plugin-setup
```

它會先看現況，再讓你選一條路線：

- **逐步問答**：一題一題問 endpoint 與認證，答案直接寫進 `~/.plasma-plugin/config.env`
  （權限 600）。token 與密碼可以選擇不進對話紀錄——在自己的終端機執行
  `bin/plasma-config.sh set-secret PLASMA_PASSWORD`，輸入不回顯。
- **自己編輯設定檔**：只給你設定檔路徑與各欄位說明，其餘你自己填。

設定還沒填完時，每個 session 開頭的 SessionStart hook 會提醒先做設定，填完就
不再出現。設定檔是 MCP server **啟動時**讀的，第一次設定完成請重啟 session。

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

## 設定

設定放 `~/.plasma-plugin/config.env`（**不要**放在 plugin 目錄，marketplace
更新會換掉整個目錄）。要填的是 `PLASMA_URL`、Plasma 認證（`PLASMA_TOKEN`
或 `PLASMA_USERNAME` + `PLASMA_PASSWORD` 擇一）、`OPHION_URL` 與
`OPHION_SERVICE_TOKEN`；`OPHION_PROFILE` 留空即為 `all`。環境變數會蓋過檔案，
所以單一 session 可以不改檔就指向別的部署。

交給 setup skill 問答填寫最省事，也可以自己來：

```bash
mkdir -p ~/.plasma-plugin
cp config.env.example ~/.plasma-plugin/config.env && chmod 600 ~/.plasma-plugin/config.env
```

`bin/plasma-config.sh` 是同一份設定的命令列入口，setup skill 走的也是它：

| 指令 | 用途 |
|---|---|
| `init` | 建立目錄與設定檔（700／600），印出路徑 |
| `set KEY VALUE` | 寫入一個已知欄位，重複指定只留一行 |
| `set-secret KEY` | 從終端機隱藏輸入 token／密碼，不經過命令列與對話 |
| `show` | 檢視目前設定與來源；秘密只顯示長度 |
| `check` | 判斷設定是否完整，缺什麼一起列出 |
| `probe` | 不帶認證測兩個 endpoint 通不通 |

Ophion 的 query-mcp 是叢集內 API，工作站通常要 port-forward：

```bash
kubectl -n <namespace> port-forward svc/ophion 5101:5101
```

設定好、重啟 session 後叫 `whoami`——兩個 endpoint、登入者、當下 workspace
一次看完。

## 工具

**plasma（19 顆）**

| 工具 | 用途 |
|---|---|
| `whoami` | 兩邊 endpoint、登入者、當下 workspace（排障第一站） |
| `list_workspaces` / `use_workspace` | 列出並選擇 workspace（ophion 那台跟著走） |
| `list_views` / `get_view` | 看 view／mview 與同步狀態 |
| `run_query` | 在 workspace 跑一段 SELECT 驗證 SQL，不額外逐次確認 |
| `create_view` | `type=view` 建立一般 view，省略同步／排程設定；mview 的 manual 不額外確認，scheduled 會立即同步，**需先確認** |
| `sync_view` | 觸發一次同步（**需同意**） |
| `create_access_entry` | 把 view 發布成對外 Data API（**需同意**） |
| `list_access_entries` / `get_export_url` | 看既有發布與取回 URL |
| `list_pg_connections` / `get_pg_connection` | 核對使用者選定的既有 PG DBC、database 與 schema，不回傳密碼 |
| `create_pg_blueprint` | 綁定來源 view、所選 PG DBC、目標表與明確寫入模式，不啟動同步 |
| `list_blueprints` / `get_blueprint` | 搜尋與核對 blueprint、目的地及最近 job 狀態 |
| `spawn_blueprint_job` | 執行 view → blueprint → PG（**需同意**），回傳受理訊息 |
| `list_blueprint_jobs` / `get_job` | 找出本次新 job 並追蹤執行結果，不將舊成功當成這次成功 |

沒有 `list_tables`：能不能查、怎麼查是 Ophion 帶 `access_mode` 的權威判斷，
Plasma 這側再列一份表清單只會多一個會對不上的來源。

**ophion**：上游 profile 的知識工具原封轉發，外加 `ophion_context()` 診斷。

## 使用者同意

四份 skill 的所有進度、說明、確認與交付均使用**台灣繁體中文**，保留系統功能英文名稱。
查找知識、查核欄位、SELECT 驗證及建立 manual mview 定義可連續完成，不逐步
要求核准。資料 API 原則上**一份表單／報表建立一個 mview**，PG 匯出則建立一個 view，整合所有指標與區塊，
不因來源表不同而拆分；只有使用者明確要求才拆分。

確認集中在兩個時點：

1. **開始同步拉資料**：`sync_view`，或會立即同步的 scheduled `create_view`。
   中文確認會說明：執行後就會依 SQL 從來源系統拉取資料並寫入／更新 mview，
   使用查詢與同步資源；若有排程，也說明後續自動拉資料的頻率。
   PG 匯出則在 `spawn_blueprint_job` 前確認 view、使用者選定的 PG 連線、目標表與
   寫入模式；`overwrite` 可能刪表重建，`truncate` 會清空既有資料，需一併說明。
2. **同步成功後開啟 API**：`create_access_entry`。中文確認會說明資料範圍、
   驗證方式、有效期限與誰可以透過端點讀取資料。

Claude Code 的 `PreToolUse` hook 會在上述操作回傳 `ask`，即使 MCP server
被加入 allowlist，仍會要求確認。`run_query` 與 manual `create_view` 不由此
hook 強制確認；宿主平台自身的工具權限仍適用。

ChatGPT／Codex 匯入時，不能依賴原 hook 的 `permissionDecision: "ask"`；
應使用宿主支援的工具確認設定。未有適用確認介面時，skill 會在同步與開 API
前用中文取得明確同意，不在前置步驟另加確認。

## 開發

```bash
make check     # fmt + vet + Go tests + build + launcher tests + config tests
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

既有 tag 若未觸發發佈，可在 GitHub 的 **Actions → Release → Run workflow**
選擇 `main`，並在 `tag` 填入版本（例如 `v0.1.1`）。手動執行仍會 checkout 該
tag 的原始碼並驗證版本，不會拿目前 main 的程式替換已標記的版本。
