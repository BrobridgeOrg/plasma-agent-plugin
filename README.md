# plasma-plugin

一個 Claude Code plugin：**工作流 skill，加上資料真的要動之前的那一次確認。**

MCP 工具不在這裡。Plasma 的 view／blueprint／job／資料 API 工具，以及知識工具
（`overview`、`get_table_card`、`get_column_card`…），**都由 plasma-backend 的
`mcp_gateway` 提供** —— 一台用 URL 連上的 HTTP MCP server，前面擋著它自己的
OAuth 2.1。知識那半是 gateway 向 Ophion 轉發的。

```text
Claude Code
├─ MCP「plasma」───HTTP + OAuth 2.1──> mcp_gateway ─┬─REST──> Plasma /apis/v1
│   （一台，使用者只加這個 URL）                     └─MCP───> Ophion query-mcp
│
└─ 這個 plugin
   ├─ skills（4 份）
   └─ PreToolUse hook：同步／開 API 前的人工確認
```

## 為什麼還需要這個 plugin

**gateway 的授權是一次性的。** 使用者連結時同意一組 scope，之後 `sync_view`、
`spawn_blueprint_job`、`create_access_entry` 都不會再問。真正要花資源、要寫進別人
資料表、要把資料送到 Plasma 之外的那一刻，確認只能從用戶端這側來 —— 就是這個 hook。

它比對的是去掉命名空間後的工具名，所以不綁定哪一台 server 提供這些工具。

## 內容

| Skill | 用途 |
|---|---|
| `plasma-mcp-setup` | 怎麼連上 Plasma MCP、怎麼確認連到哪個 workspace、診斷 |
| `ophion-knowledge-lookup` | 查核來源、欄位、代碼、業務規則 |
| `plasma-data-api` | 建立 mview、同步並發布資料 API |
| `plasma-export` | 建立 view，供 blueprint 匯出至指定的外部資料表 |

一個 hook：`create_view(sync_mode=scheduled)`、`sync_view`、`create_access_entry`、
`spawn_blueprint_job` 之前跳出中文確認。查找、`run_query` 驗證、建立不啟動同步的
view 或手動 mview 不攔 —— 那些是準備工作。

## 安裝

```bash
/plugin marketplace add BrobridgeOrg/plasma-agent-plugin
/plugin install plasma-plugin@plasma-plugin-local
```

**這個 plugin 沒有任何設定。** 沒有 `config.env`，沒有 endpoint、帳密或狀態檔。

接著加 MCP server —— 只需要一個 URL：

```bash
claude mcp add --transport http plasma https://<gateway>/mcp
```

用戶端會自己走完授權：登入 → 選 workspace → 同意。全程不需要複製貼上 token。
細節與診斷見 `/plasma-plugin:plasma-mcp-setup`。

連上之後**第一個呼叫一律是 `whoami`**：它會說明這個連線綁定的 workspace、
授權的帳號、拿到哪些權限，以及知識工具現在可不可用。

### workspace 是憑證的一部分

使用者在同意頁選的 workspace 寫進連線的憑證，**沒有工具能切換它**。要換 workspace
或增加權限，就是重新連結一次。這也是 Plasma 與知識兩半不可能被指到不同 workspace
的原因 —— 它們讀的是同一張 token。

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

binary 保留 `plasma-plugin-mcp` 這個名字（發佈資產與啟動腳本都用它定址），
但它現在只有一個模式：`hook`。以 `plasma` 或 `ophion` 呼叫會明確報錯並指向
gateway，而不是含糊的「unknown mode」—— 沒更新設定的安裝會撞到這個。

## ChatGPT／Codex

匯入時不能依賴 hook 的 `permissionDecision: "ask"`，應使用宿主支援的工具確認設定。
未有適用確認介面時，skill 會在同步與開 API 前用中文取得明確同意，不在前置步驟
另加確認。

## 發佈

更新 `VERSION`、`.claude-plugin/plugin.json` 的版本及 `releases/v<version>.md`，
提交後推送對應的 `v<version>` tag。GitHub Actions 會執行檢查、建置四種平台的
執行檔，並在目前 repo 發佈 Release 與 `checksums.txt`。本機可用 `make release`
產生相同格式的資產，輸出在 `dist/v<version>/`。

既有 tag 若未觸發發佈，可在 GitHub 的 **Actions → Release → Run workflow**
選擇 `main`，並在 `tag` 填入版本（例如 `v0.1.1`）。手動執行仍會 checkout 該
tag 的原始碼並驗證版本，不會拿目前 main 的程式替換已標記的版本。
