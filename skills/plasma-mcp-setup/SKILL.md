---
name: plasma-mcp-setup
description: >-
  連結 Plasma 遠端 MCP，或診斷工具缺少、連線失敗及權限不足時使用。opencode 走 OAuth，Claude Code 與 Codex CLI 走存取權杖；核對 workspace 與 scopes。全程台灣繁體中文。
---

# 連結 Plasma 與診斷

全程使用台灣繁體中文；工具名稱、SQL、識別名稱與 URL 保留原樣。

設定連線時只回報要使用者做什麼、以及做完的結果。不要解說 OAuth、權杖或
MCP 的運作原理，不要列出你讀過或檢查過哪些檔案，也不要在完成後補一段說明。
使用者要的是「下一步做什麼」與「好了沒有」。

## 連線方式

本流程使用的 Plasma view 工具與 Ophion 知識工具，都由同一台遠端 MCP gateway 提供。
安裝 plugin 不等於已連結服務。連線 URL 必須由使用者或管理員提供，不猜測部署位址。

1. 工具已可用時，先呼叫 `whoami` 核對連線，不重新要求設定或登入。
2. 工具尚未出現時，依宿主選一條：

| 宿主 | 連線方式 |
|---|---|
| **opencode** | OAuth：opencode 自動開瀏覽器完成授權，見〈opencode〉 |
| **Claude Code**、**Codex CLI** | 存取權杖：使用者自行核發後貼進設定，見〈Claude Code 與 Codex CLI〉 |

差別在用戶端，不在伺服器：gateway 兩條路都支援。Claude Code 與 Codex CLI 的
MCP SDK 不接受非 TLS 位址上的 OAuth token endpoint，所以在只有 http 的部署上
只能用權杖；gateway 換上用戶端信任的 HTTPS 憑證後，兩者也可改用 OAuth。

ChatGPT 網頁版不支援：它由 OpenAI 伺服器連出，連不到內網的 gateway 位址。

workspace 是授權的一部分，沒有 `use_workspace` 工具；換 workspace 要重新授權。

## opencode

在 `~/.config/opencode/opencode.json` 的 `mcp` 區塊加入連線，URL 換成實際位址：

```json
{
  "mcp": {
    "plasma": {
      "type": "remote",
      "url": "<gateway>/mcp",
      "enabled": true,
      "oauth": { "scope": "views:read knowledge:read query:run views:write" }
    }
  }
}
```

`scope` 只列本次需要的權限，授權頁會逐項顯示。接著請使用者執行：

```bash
opencode mcp auth plasma
```

瀏覽器會開啟 gateway 授權頁：輸入 Plasma 帳密、選 workspace、確認權限。
帳密不透過對話收集。完成後開新 session，呼叫 `whoami` 核對。

- 狀態查詢：`opencode mcp list`
- 換 workspace 或改權限：`opencode mcp logout plasma` 後修改 `scope`，再 auth 一次
- token 由 opencode 保管並自動更新，不需要使用者保存任何字串

## Claude Code 與 Codex CLI

逐步帶使用者完成，一次一步，等他回報再進行下一步。
其中兩步只能由使用者親手做：核發要輸入 Plasma 密碼，權杖本身不該經過對話。

1. 取得 gateway 位址。沒有確切位址時不要猜，也不要試探常見網址。
2. **（使用者自己做）** 在瀏覽器開啟 `<gateway>/oauth/pat`，登入、選 workspace、
   勾選權限並核發。先問清楚本次任務需要哪些 scope 再請他勾：建立 view 要
   `views:write`，查知識要 `knowledge:read`，執行查詢要 `query:run`。
   頁面只顯示權杖一次；不要求使用者把權杖貼給你。
3. **（使用者自己做）** `export PLASMA_MCP_TOKEN='<權杖>'`，再開新的終端機。
4. 設定連線。Claude Code 可以由你代為執行：

   ```bash
   claude mcp add --transport http plasma <gateway>/mcp \
     --header 'Authorization: Bearer ${PLASMA_MCP_TOKEN}'
   ```

   單引號與 `${...}` 是刻意的：Claude Code 讀取設定時才展開，權杖不會寫進
   `~/.claude.json`。用雙引號會讓 shell 先展開，把權杖明文留在設定檔裡。

   Codex CLI 請使用者自己在 `~/.codex/config.toml` 加入：

   ```toml
   [mcp_servers.plasma]
   url = "<gateway>/mcp"
   bearer_token_env_var = "PLASMA_MCP_TOKEN"
   experimental_use_rmcp_client = true
   ```

5. 請使用者重新開啟對話，再呼叫 `whoami` 核對。工具要等宿主重新載入才會出現。

權杖到期不會自動更新，重新核發一次即可，不要建議改用其他憑證或放寬伺服器設定。
外洩或不再需要時，用 gateway 的 `/oauth/revoke` 撤銷，撤銷後立即失效。

## 核對連線

`whoami` 回報 workspace 名稱與 ID、授權帳號、scopes 與知識服務狀態。
核對本次任務需要的 workspace。遇到多個 Plasma 連線，選定一個後，
Plasma 與 Ophion 的工具均使用該連線；不要混用不同連線的物件 ID。
工具名稱可能帶宿主命名空間，以實際可用工具及 schema 為準。

| scope | 允許的操作 |
|---|---|
| `views:read` | 讀取 view／mview；每次授權都會包含這項 |
| `knowledge:read` | 查資料表／欄位意義、值域與 lineage |
| `query:run` | 執行唯讀查詢，最多 100 列 |
| `views:write` | 本流程用於建立 view／mview 定義；後端此 scope 可能還允許其他操作 |

以 `whoami` 的實際 scopes 為準，不假定授權時給了哪些。缺少權限時說明具體 scope，
請使用者依所用宿主重新授權並選上該項，再次用 `whoami` 核對；
若仍缺少，停止該操作並請管理員檢查 gateway。不要重複呼叫遭拒工具。

opencode 重新授權時，既有權限會一併保留；權杖路徑則不繼承，該勾的要當下勾。

## 診斷

| 症狀 | 處理 |
|---|---|
| 安裝後沒有工具 | 確認連線已設定且該 session 重新載入過；skills 要下一個 session 才生效 |
| 工具缺少 scope | 依上節重新授權；plugin 不能代替 gateway 授予權限 |
| 連線回 `401` | opencode 重跑 `mcp auth`；權杖路徑檢查是否過期、被撤銷或變數沒展開 |
| opencode 不開瀏覽器 | 手動執行 `opencode mcp auth plasma`；仍無反應時用 `opencode mcp list` 看狀態 |
| 授權頁或核發頁打不開 | 確認 gateway 位址可達，由管理員檢查部署 |
| Plasma `403` | 與缺少 scope 不同，由 Plasma 管理員檢查 workspace 成員與權限 |
| 有 Plasma 工具但沒有知識工具 | 用 `whoami` 區分未配置 Ophion、缺 `knowledge:read` 或服務不可用 |
| 知識工具 `404`／尚無已發布知識 | 由管理員產生並發布該 workspace 的知識版本 |
| workspace 不符 | 重新授權並選擇正確 workspace；不自行替換參數繞過 |
| 連線只使用舊式 SSE | 使用支援 Streamable HTTP 的連線方式 |

知識恢復前，不憑欄位名稱推測語意或編寫來源 SQL。缺少資訊要據實說明。

## 流程範圍

連線確認後依 [建立 view 技能](../plasma-create-view/SKILL.md) 查核來源、驗證 SQL、
建立 view／manual mview 並核對定義，到此結束。本流程不要求匯出或 API 發布權限。
宿主顯示其他後端工具或授權帶有較廣 scopes，不代表 plugin 應接續使用。
plugin 不提供本機 hook；建立操作依使用者交付範圍與授權權限執行。
