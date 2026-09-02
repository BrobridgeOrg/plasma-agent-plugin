# plasma-plugin 設計（v1）

日期：2026-09-02 ｜ 狀態：待實作

一個 Claude Code plugin，讓 Claude 能**用 Ophion 的知識去操作 Plasma**：把
「我想要這個指標的 API」這句話，走到一條可呼叫的 Data API。

含兩台 MCP server 與三份 skill。兩台 server 都用 Go 寫在本 repo；介面穩定後，
plasma 那台的工具契約再考慮搬進 `plasma-backend` 成為產品能力（已拍板的路徑，
不在 v1 範圍）。

## 1. 範圍

**v1 做**

- plasma MCP server（11 顆工具，REST client 打 Plasma `/apis/v1`）
- ophion MCP server（proxy，轉發 Ophion 某 workspace 的 query-mcp）
- 依「當下 workspace」動態改道，session 內可切換
- 三份 skill：`plasma-data-api`、`ophion-knowledge-lookup`、`plasma-plugin-setup`
- `PreToolUse` hook 強制四顆高成本／對外曝露工具每次取得使用者同意

**v1 不做**

- 多台 Ophion 之間切換（拍板：Ophion 單一台，只切 workspace）
- 改／刪 view、改排程、trigger、iceberg catalog entry、parameterized view
- 把工具搬進 `plasma-backend`
- Plasma 端的 `mcp_manager` tool group（資料面）操作——那是另一件事

## 2. 外部前提（不是本 plugin 能解決的）

- **Ophion 的 query-mcp 是叢集內 internal API**（Plasma 端設定形如
  `http://ophion.internal:5101`），bearer service token。使用者的機器必須連得到
  ——叢集內 URL 或 port-forward。`plasma-plugin-setup` skill 負責檢查並診斷，
  但連不到就是連不到。
- **Plasma 的 workspace_id 就是 Ophion 的 workspace_id**（Ophion 的
  `query_mcp_service` 路由把 workspace 視為 Plasma 的識別碼）。這是兩台 server
  能共用同一個 workspace 狀態的前提。
- Ophion 的 workspace 若未成功發布過 generation，query-mcp 會 404；那是「還沒有
  知識」而不是設定錯誤。

## 3. 架構

```text
Claude Code
├─ MCP「plasma」 (stdio, Go)  ──REST──> Plasma /apis/v1              (JWT)
└─ MCP「ophion」 (stdio, Go)  ──MCP───> Ophion /internal/v1/workspaces/<ws>/query-mcp/<profile>
                                        (Authorization: Bearer <service token>)
        兩台共讀 ~/.plasma-plugin/state.json（當下 workspace / profile）
```

一個 Go module、一顆 binary、兩個 subcommand：

```bash
plasma-plugin-mcp plasma   # stdio MCP server：Plasma 控制面
plasma-plugin-mcp ophion   # stdio MCP server：Ophion 讀取端 proxy
```

### 3.1 動態改道機制（核心）

Claude Code 不會在 session 中重連 MCP server，所以「動態」不能靠改設定檔。

- **ophion 那台是 proxy**：對 Claude Code 是固定的 stdio server，對上游是 MCP client。
- **每次 tool 呼叫都重讀 `state.json`**，用當下 workspace 撥
  `<ophion_url>/internal/v1/workspaces/<ws>/query-mcp/<profile>`。上游連線依
  workspace 快取（`map[workspace]*upstreamSession`），換 workspace 換一條，
  舊的閒置逾時關閉。
- 切換入口是 plasma 那台的 `use_workspace`（它才有 `my_workspaces` 可列清單）。
  寫入 state 後，**下一次** ophion 呼叫立即改道，不需重啟。
- **每筆 ophion 回應強制在最前面加一行 `[workspace=<id> profile=<p>]`**。
  隱藏狀態最大的風險是「以為切了、其實沒切」；標注讓它變成看得見的錯。
- **profile 固定單一**（預設 `all`，可在 config 改，不提供 runtime 切換工具）。
  換 profile 會換上游 tool 清單，而 `notifications/tools/list_changed` 在
  client 端是否即時刷新無法保證——不賭。

### 3.2 為什麼 tool 清單原封轉發

ophion proxy 不重新定義上游工具的 schema：啟動時 `tools/list` 上游一次，把
name / description / inputSchema 原樣登記，呼叫時原樣轉發。包一層就得跟著
Ophion 改版走，而 Ophion 的讀取端契約仍在演進。

## 4. 設定與狀態

**不放 plugin 目錄。** 從 marketplace 安裝時 plugin 住在
`~/.claude/plugins/cache/<marketplace>/<plugin>/<hash>/`，更新會換掉整個目錄，
放在裡面的 `.env` 會消失。

### `~/.plasma-plugin/config.env`（0600，repo 只放 `config.env.example`）

```ini
PLASMA_URL=http://10.10.7.170:5001
# 二選一：直接給 token，或給帳密由 plugin 自行登入
PLASMA_TOKEN=
PLASMA_USERNAME=
PLASMA_PASSWORD=

OPHION_URL=http://127.0.0.1:5101
OPHION_SERVICE_TOKEN=
OPHION_PROFILE=all
```

`PLASMA_TOKEN` 優先。兩者皆空 → 工具回可行動的錯誤，指向 setup skill。

### `~/.plasma-plugin/state.json`（0600，plugin 自己寫）

```json
{
  "workspace_id": "019ea607-6b9d-7b26-b783-02b09a6b722f",
  "workspace_name": "his_database",
  "profile": "all",
  "access_token": "…",
  "refresh_token": "…",
  "expires_at": "2026-09-02T10:00:00Z"
}
```

寫入用 write-temp-then-rename，避免兩台 server 同時讀到半份檔。讀取一律
即時讀檔，不在記憶體長期快取 workspace。

### 認證流程

1. `PLASMA_TOKEN` 有值 → 直接用，不登入、不刷新。
2. 否則用 `PLASMA_USERNAME`/`PLASMA_PASSWORD` 打
   `POST /apis/v1/auth/login`（body `{username_or_email, password}`），
   回應存 `access_token` / `refresh_token` / `expires_at`。
3. 呼叫遇 401 → 用 refresh token 打 `POST /apis/v1/auth/refresh` 重試一次；
   再 401 → 重新登入一次；仍失敗 → 回錯誤，不再重試。

## 5. plasma MCP 工具契約（11 顆）

`R` = 唯讀（`readOnlyHint`）；`C` = 需使用者同意（見 §7）。
所有 workspace-scoped 工具都用 state 的 workspace，回應標注用的是哪一個。

| # | 工具 | 端點 | 類 |
|---|---|---|---|
| 1 | `whoami()` | `POST /apis/v1/auth/verify-token` + 本地狀態 | R |
| 2 | `list_workspaces()` | `GET /apis/v1/my_workspaces` | R |
| 3 | `use_workspace(workspace)` | 本地（接 id 或 name，name 需先解析） | 寫 state |
| 4 | `list_views(keywords?, page?, page_size?)` | `GET /apis/v1/w/:ws/views` | R |
| 5 | `get_view(view_id, with_blueprint_history?)` | `GET /apis/v1/w/:ws/view/:id`（+ `/blueprint_history`） | R |
| 6 | `run_query(sql)` | `POST /apis/v1/w/:ws/query/execution` | C |
| 7 | `create_view(name, type, view_sql, sync_mode, scheduler_settings?, description?)` | `POST /apis/v1/w/:ws/view` | C |
| 8 | `sync_view(view_id)` | `POST /apis/v1/w/:ws/view/:id/sync` | C |
| 9 | `create_access_entry(view_id, name, auth_type, expires_in?, format?, entry_type?, description?)` | `POST /apis/v1/w/:ws/view/:id/access_entry` | C |
| 10 | `list_access_entries(view_id)` | `GET /apis/v1/w/:ws/view/:id/access_entries` | R |
| 11 | `get_export_url(view_id)` | `GET /apis/v1/w/:ws/view/:id/export/url` | R |

**沒有 `list_tables`。** 能不能查、怎麼查是 Ophion 那側帶 `access_mode` 的權威
判斷（`direct` / `definition_required` / `blocked`）；Plasma 這側再列一份表清單
只會多一個會對不上的來源。

### 5.1 `whoami`

回：Plasma URL、登入者（username/role）、當下 workspace（id+name）、profile、
Ophion URL 與 query-mcp 可否連通（一次 `initialize` 探測）。這是排障的第一站。

### 5.2 `use_workspace`

參數接 workspace id 或 name。給 name 時用 `my_workspaces` 解析；找不到或撞名
→ 列出候選並拒絕，不猜。成功後回「已切到 <name> (<id>)」。

### 5.3 `run_query`（需同意）

- Plasma **伺服器端本來就強制**：只允許 SELECT、拒 DML/DDL、自動套上
  **最多 100 列**的 LIMIT。所以本工具**不提供 `limit` 參數**——提供了就是在
  對使用者撒謊說能拿更多。
- Client 端仍先做一層守門（單一敘述、`SELECT`/`WITH` 開頭、無分號串接）：
  目的是快速失敗，以及讓確認框顯示的 SQL 就是真的會跑的那一段。
- Request body：`{"query": "<sql>"}`。Response `data` 是 columnar
  `{"columns": [...], "rows": [[...], ...]}`，工具回傳時渲染成人看得懂的表格
  並附列數與耗時。
- 執行身分是 workspace 名稱（Plasma 用 workspace name 當 catalog 與 user），
  所以 SQL 裡的兩段式 `"<database>"."<table>"` 就夠，不需前置 catalog。

### 5.4 `create_view`（需同意）

- `type`：`view` | `materialized_view`；`sync_mode`：`scheduled` | `manual`。
- 帶 `scheduler_settings` 時，Plasma 一支 API 就建完 **view + blueprint +
  schedule** 並立即跑第一次 sync。所以**沒有獨立的「建 blueprint」工具**——
  那是 Plasma 現有契約，不是本設計簡化掉的。
- 回應把 `view.id`、`status`（`initializing` 代表 mview 正在 provision）、
  `path`（`<catalog>.<schema>.<name>`）攤出來，並提示下一步該用 `get_view`
  看 `last_sync_status`。

### 5.5 `create_access_entry`（需同意）

- `auth_type`：`api_key` | `basic_auth` | `none`
- `expires_in`：人類寫法（`30d`、`12h`），轉成 `expired_at`。不給時的行為
  （Plasma 的 `expired_at` 可為 null，推定＝不過期）**在實作時以真機確認**，
  確認後回應必須明講這條會不會過期——不確認就不要在回應裡斷言。
- `format`：`json`（預設）| `xml` | `rss`；`entry_type`：`signed_api`（預設）
  | `signed_url`。
- 回應含 `secret_key`／金鑰時，明確標示這是**對外可存取的端點**，並提醒金鑰
  只在此處出現一次（依 Plasma 實際行為在實作時確認並校正這句話）。
- `auth_type=none` 一律在回應中標示「無認證的公開端點」。

## 6. ophion MCP（proxy）

- 工具：上游 `all` profile 的全部工具原封轉發（`overview`、`search_knowledge`、
  `find_tables`、`get_table_card`、`list_columns`、`get_value_domain`、
  `plan_value_filter`、`trace_lineage`、`read_source`、`get_knowledge_unit`、
  `search_value_candidates`…），外加一顆本地 `ophion_context()` 回當下
  workspace / profile / 上游連通性（診斷用）。
- 未選 workspace → 回可行動的錯誤：「先用 plasma 的 `use_workspace`」，
  不預設任何 workspace。
- 上游 404 → 譯成「這個 workspace 在 Ophion 尚無已發布的知識」。
- 上游 401 → 譯成「Ophion service token 不對或未設」。
- 連不到 → 譯成「Ophion internal API 不可達（叢集內端點，需 port-forward）」。
- 全部工具皆為唯讀，不需同意 hook。

## 7. 同意機制

`hooks/hooks.json` 註冊 `PreToolUse`，matcher 命中四顆工具
（`run_query`、`create_view`、`sync_view`、`create_access_entry`）時回
`permissionDecision: "ask"`：

```json
{"hookSpecificOutput": {"hookEventName": "PreToolUse",
  "permissionDecision": "ask", "permissionDecisionReason": "…"}}
```

理由（reason）依工具給出成本說明：

- `run_query` / `sync_view`：會在 Trino 上實際執行，吃叢集資源。
- `create_view`：會建立 view/mview 與（帶排程時）週期性 sync job。
- `create_access_entry`：會建立**對外可存取**的端點；`auth_type=none` 時加註無認證。

這個 hook 的價值在於：**即使有人把整台 MCP server 加進 allowlist，這四顆仍然
每次跳確認**，而 Claude Code 的確認框會顯示完整參數，所以使用者按下去之前看得到
要跑的 SQL。工具本身也標 `readOnlyHint` / `destructiveHint` 作為第二層訊號。

matcher 用後綴正則命中（plugin MCP 的工具全名形如
`mcp__plugin_<plugin>_<server>__<tool>`），實作時以實測名稱校正。

## 8. Skills

### 8.1 `plasma-data-api`（情境 skill）

觸發：客戶口吻的「我想要這些資料的 API」「這個指標給我一個 API」「BI 要接資料」。

流程：

1. 釐清要的指標與粒度（不清楚就問，一次問一件）。
2. 用 ophion 工具找來源表與計算方式，讀 `access_mode` 與 provenance。
3. 生 SQL。
4. `run_query` 試跑，把 **SQL 與樣本結果一起**給使用者確認。
5. 建 mview（問同步方式與頻率）。
6. **反問 API 三件事**：認證方式、存活時間、格式。
7. `create_access_entry` + `get_export_url`，交出 URL、金鑰、`curl` 範例，
   以及怎麼接回 BI 工具。

硬規則：

- 知識不足就問人，不用猜的欄位湊 SQL。
- 沒試跑過、使用者沒確認過的 SQL，不建 mview。
- `access_mode=definition_required` 的關聯不得出現在 `FROM`／`JOIN`。
- `auth_type=none` 要對方明確答應，並在交付時重述這是公開端點。

### 8.2 `ophion-knowledge-lookup`

讀取端方法：`overview` → `search_knowledge` → `get_table_card` →
value domain／`plan_value_filter`；怎麼判 `access_mode`；怎麼引 provenance
（file:line、authority=user_qa 的份量）；圖上沒有就說沒有，不編。
也講清楚 Ophion 存的是設計與語意，**不存任何一列資料**。

### 8.3 `plasma-plugin-setup`

首次設定（`~/.plasma-plugin/config.env`）、build、連通性檢查、以及錯誤字典：
Plasma 401／403（非 workspace 成員）、Ophion 404（尚無知識）／401（token）／
連不到（internal API 需 port-forward）。

## 9. Repo 佈局

```text
plasma-plugin/
├── .claude-plugin/
│   ├── plugin.json          # 只放 metadata：name/description/version/author
│   └── marketplace.json     # 讓本地目錄可直接掛成 marketplace 開發
├── .mcp.json                # 兩台 server → ${CLAUDE_PLUGIN_ROOT}/bin/plasma-mcp.sh
├── hooks/hooks.json
├── skills/
│   ├── plasma-data-api/SKILL.md
│   ├── ophion-knowledge-lookup/SKILL.md
│   └── plasma-plugin-setup/SKILL.md
├── cmd/plasma-plugin-mcp/main.go     # subcommand: plasma | ophion
├── internal/
│   ├── pcontext/    # config.env + state.json、JWT 生命週期
│   ├── plasmaapi/   # Plasma REST client（只覆蓋用到的端點，typed DTO）
│   ├── plasmamcp/   # 11 顆工具的定義與 handler、SQL 守門
│   └── ophionproxy/ # stdio ↔ HTTP MCP proxy、per-workspace 上游 session
├── bin/plasma-mcp.sh        # binary 不存在或比原始碼舊 → go build，再 exec
├── Makefile
├── config.env.example
└── README.md
```

`skills/` 與 `hooks/hooks.json` 走標準位置，Claude Code 自動偵測，`plugin.json`
不需要宣告（已在現有 plugin 實例確認）。

`bin/plasma-mcp.sh` 的自我修復是刻意的：使用者忘記 `make build` 時，MCP server
仍能起來，而不是留下一個「server failed」而看不出原因的 session。

`${CLAUDE_PLUGIN_ROOT}` 在 `hooks.json` 已於現有 plugin 實例確認可用；`.mcp.json`
依文件使用，若實測不吃，退路是 setup 時產生一份帶絕對路徑的 wrapper。

## 10. 測試策略

TDD，全部落在不需要真機的層：

- `pcontext` — config/state 讀寫、0600、原子寫入、token 過期與刷新分支、
  workspace name 解析與撞名拒絕。
- `plasmaapi` — `httptest` 假 Plasma：每個端點的成功、4xx、401→refresh→重試、
  refresh 也失敗。
- `plasmamcp` — 假 client 驗 11 顆 handler：參數驗證、SQL 守門（拒 DML/DDL、
  拒多敘述）、`expires_in` 轉換、`auth_type=none` 的標示、回應標注 workspace。
- `ophionproxy` — 起一台 in-process 上游 MCP HTTP server：轉發正確、
  **切 workspace 後下一次呼叫立即改道**、回應前綴標注、404／401／連不到的
  錯誤翻譯。

真 Plasma／真 Ophion 的端到端連通不在單元測試範圍，靠 `plasma-plugin-setup`
的檢查指令；不會把它說成測過。

## 11. 已拍板的決定

- plasma MCP 先在 plugin 自建（REST client），介面穩了再搬進 `plasma-backend`。
- Ophion 單一台，session 內只切 workspace。
- 設定檔＋狀態檔（`~/.plasma-plugin/`），不用純 env、不用每次帶參數。
- v1 含 Data API 出口工具（「客戶要 API」的情境不能斷在 mview）。
- 四顆高成本／對外曝露工具強制使用者同意。
- Runtime 用 Go（與 Plasma／Ophion 同語言，日後搬遷可沿用）。
- 不做 `list_tables`（Ophion 已是權威）。

## 12. 留給 v2

- 改／刪 view、改排程、trigger、iceberg catalog／parameterized view entry。
- 多 Ophion 端點與 profile 的 runtime 切換。
- 把工具契約搬進 `plasma-backend` 成為產品 MCP 能力。
- 進 `BrobridgeOrg/brobridge-plugins` marketplace 發布（v1 先本地掛載）。
