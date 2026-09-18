---
name: plasma-mcp-setup
description: >-
  連結 Plasma 遠端 MCP，或診斷工具缺少、連線失敗及權限不足時使用。帶使用者到 gateway 的權杖核發頁取得存取權杖、設定用戶端、核對 workspace 與 scopes；適用 Claude Code 與 Codex CLI。全程台灣繁體中文。
---

# 連結 Plasma 與診斷

全程使用台灣繁體中文；工具名稱、SQL、識別名稱與 URL 保留原樣。
在已交付的範圍內連續完成診斷，只有缺少必要資訊時才釐清。

## 連線方式

這個 plugin 提供工作流程。本流程使用的 Plasma view 工具，
以及 Ophion 知識工具，都由同一台遠端 MCP gateway 提供。
安裝 plugin 不等於已連結服務；不要求使用者安裝本機執行檔或執行 plugin 提供的程式。

連線方式是：使用者到 gateway 的 `/oauth/pat` 頁面核發一張存取權杖，
放進環境變數，再於用戶端設定一個帶 `Authorization` 標頭的 MCP 連線。
核發頁本身會要求 Plasma 帳密登入、選擇 workspace、逐項勾選權限。

適用在本機執行的用戶端：**Claude Code** 與 **Codex CLI**。
ChatGPT 網頁版目前不支援：它由 OpenAI 伺服器連出，連不到內網的 gateway 位址。

連線 URL 必須由使用者或管理員提供，不猜測部署位址。
workspace 是權杖的一部分，沒有 `use_workspace` 工具；換 workspace 要重新核發一張。

1. 工具已可用時，先呼叫 `whoami` 核對連線，不重新要求設定或登入。
2. 工具尚未出現時，依下節帶使用者完成連線。

## 建立連線

在對話中逐步帶使用者完成，一次一步，等他回報再進行下一步。
其中兩步只能由使用者親手做：核發要輸入 Plasma 密碼，權杖本身不該經過對話。

1. 請使用者或管理員提供 gateway 位址。沒有確切位址時不要猜，也不要試探常見網址。
2. **（使用者自己做）** 在瀏覽器開啟 `<gateway>/oauth/pat`，用自己的 Plasma 帳號
   登入、選 workspace、勾選這張權杖需要的權限並核發。先問清楚本次任務需要哪些
   scope 再請他勾：建立 view 要 `views:write`，查知識要 `knowledge:read`，
   執行查詢要 `query:run`。頁面只顯示權杖一次。
   帳密與權杖都不透過對話收集，也不要求使用者貼給你或寫進你讀得到的檔案。
3. **（使用者自己做）** 把權杖放進環境變數，例如在 shell 設定檔加入
   `export PLASMA_MCP_TOKEN='<權杖>'`，再開新的終端機讓它生效。
4. 設定連線。Claude Code 可以由你代為執行；請使用者確認位址無誤後再跑：

   ```bash
   claude mcp add --transport http plasma <gateway>/mcp \
     --header 'Authorization: Bearer ${PLASMA_MCP_TOKEN}'
   ```

   單引號與 `${...}` 是刻意的：Claude Code 讀取設定時才展開，權杖不會被寫進
   `~/.claude.json`。用雙引號會讓 shell 先展開，把權杖明文留在設定檔裡。

   Codex CLI 沒有對應指令，請使用者自己在 `~/.codex/config.toml` 加入：

   ```toml
   [mcp_servers.plasma]
   url = "<gateway>/mcp"
   bearer_token_env_var = "PLASMA_MCP_TOKEN"
   experimental_use_rmcp_client = true
   ```

5. 請使用者重新開啟對話，再呼叫 `whoami` 核對 workspace 與 scopes。
   工具要等宿主重新載入才會出現，在同一個對話裡重試沒有用。

權杖到期不會自動更新，`whoami` 回 401 或工具消失時，請使用者重新核發一次，
不要建議改用其他憑證或放寬伺服器設定。權杖外洩或不再需要時，
用 gateway 的 `/oauth/revoke` 撤銷；撤銷後該權杖立即失效。

## 核對連線

`whoami` 回報 workspace 名稱與 ID、授權帳號、scopes 與知識服務狀態。
核對本次任務需要的 workspace。遇到多個 Plasma 連線，選定一個後，
Plasma 與 Ophion 的工具均使用該連線；不要混用不同連線的物件 ID。
工具名稱可能帶宿主命名空間，以實際可用工具及 schema 為準。

| scope | 允許的操作 |
|---|---|
| `views:read` | 讀取 view／mview；權杖一定包含這項 |
| `knowledge:read` | 查資料表／欄位意義、值域與 lineage |
| `query:run` | 執行唯讀查詢，最多 100 列 |
| `views:write` | 本流程用於建立 view／mview 定義；後端此 scope 可能還允許其他操作 |

以 `whoami` 的實際 scopes 為準，不假定核發時勾了哪些。
缺少權限時說明具體 scope，請使用者重新核發一張並勾選該項，再依上節重設連線。
重新核發後再次用 `whoami` 核對；若仍缺少，停止該操作並請管理員檢查 gateway。
新核發的權杖不會繼承舊權杖的權限，該勾的要當下勾。不要重複呼叫遭拒工具。

## 診斷

| 症狀 | 處理 |
|---|---|
| 安裝 plugin 後沒有工具 | 檢查用戶端是否已設定連線、環境變數是否在該終端機生效 |
| 工具缺少 scope | 依上節重新核發並勾選；plugin 不能代替 gateway 授予權限 |
| 權杖過期或被撤銷 | 重新到 `/oauth/pat` 核發，不改用其他憑證或放寬伺服器設定 |
| 連線回 `401` | 權杖過期、被撤銷，或環境變數沒展開；先確認用戶端實際送出的標頭 |
| 核發頁打不開 | 確認 gateway 位址與 `pat_enabled` 是否已開啟，由管理員檢查部署 |
| Plasma `403` | 與缺少 scope 不同，由 Plasma 管理員檢查 workspace 成員與權限 |
| 有 Plasma 工具但沒有知識工具 | 用 `whoami` 區分未配置 Ophion、缺 `knowledge:read` 或服務不可用 |
| 知識工具 `404`／尚無已發布知識 | 由管理員產生並發布該 workspace 的知識版本 |
| workspace 不符 | 重新核發並選擇正確 workspace；不自行替換參數繞過 |
| 連線只使用舊式 SSE | 使用支援 Streamable HTTP 的連線方式 |

知識恢復前，不憑欄位名稱推測語意或編寫來源 SQL。缺少資訊要據實說明。

## 流程範圍

連線確認後依 [建立 view 技能](../plasma-create-view/SKILL.md) 查核來源、驗證 SQL、
建立 view／manual mview 並核對定義，到此結束。本流程不要求匯出或 API 發布權限。
宿主顯示其他後端工具或權杖帶有較廣 scopes，不代表 plugin 應接續使用。
plugin 不提供本機 hook；建立操作依使用者交付範圍與權杖權限執行。
