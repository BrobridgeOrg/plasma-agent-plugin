---
name: plasma-mcp-setup
description: >-
  連結 Plasma 遠端 MCP，或診斷工具缺少、連線失敗及權限不足時使用。以宿主內建的 OAuth 完成授權，核對 workspace 與 scopes；適用 opencode、Claude Code 與 Codex。OAuth 開啟系統瀏覽器後立即結束當前回合，不使用 browser 或 computer use 工具。全程台灣繁體中文。
---

# 連結 Plasma 與診斷

全程使用台灣繁體中文；工具名稱、SQL、識別名稱與 URL 保留原樣。

## OAuth 交接是本回合終點

這項規則優先於本文件其餘步驟。宿主原生 OAuth 入口一旦開啟系統預設瀏覽器，或產生
需要使用者開啟的授權 URL，本回合就到達交接點：

- **立即結束本回合，將操作權交還使用者。**
- 下一個且唯一的 assistant action 必須是回覆使用者並結束本回合，不得先送出進度訊息後繼續工作。
- 本回合不得再呼叫任何工具，包括 Browser、computer use、內建瀏覽器、其他 UI 自動化、
  shell 輪詢／等待、狀態查詢與 `whoami`。
- 不查看、不操作、不代填授權頁，也不把同一個 URL 開到第二個瀏覽器或分頁。
- 若宿主沒有自動開啟瀏覽器，只把當次 URL 提供給使用者自行開啟，然後結束本回合；
  代理不得代開。
- 只有使用者在**後續新訊息**確認已完成授權後，才開始新回合並呼叫 `whoami` 核對。

交接時只需回覆：「授權頁已交由系統預設瀏覽器開啟；請完成登入與 workspace 選擇後告訴我。」

設定連線時只回報要使用者做什麼、以及做完的結果。不要解說 OAuth 或 MCP 的運作原理，
不要列出你讀過或檢查過哪些檔案，也不要在完成後補一段說明。

## 連線方式

本流程使用的 Plasma view 工具與 Ophion 知識工具，都由同一台遠端 MCP gateway 提供。
Codex 與 Claude plugin 已內含遠端 MCP 宣告，endpoint 為
`https://plasma-mcp.bbg-x.top/mcp`。安裝後只需完成 OAuth，不再要求填 URL、
手動新增相同 MCP server 或安裝其他 client。安裝 plugin 不等於已完成授權。

連線一律走 **OAuth**：使用 plugin 提供的連線，由宿主開啟瀏覽器完成授權。
使用者以自己的 Plasma 帳號登入、選 workspace，**選定即完成授權**，沒有額外的權限確認頁。
每次授權都會取得這個部署支援的完整權限，token 由宿主保管並自動更新，
不需要手動複製任何憑證。

1. 工具已可用時，先呼叫 `whoami` 核對連線，不重新要求設定或登入。
2. 工具尚未出現時，依宿主完成下面對應的設定；使用者在後續新訊息確認授權完成後，
   才回到 `whoami`。

workspace 是授權的一部分，沒有 `use_workspace` 工具；換 workspace 要重新授權。

每次授權只透過一個宿主原生入口啟動一次，不得同時混用桌面應用的 Authenticate、
CLI 登入命令或其他宿主入口。重新啟動授權前先結束原流程，避免同時存在兩組 state。

## opencode

在 `~/.config/opencode/opencode.json`（或既有的 `.jsonc`）的 `mcp` 區塊加入連線：

```json
{
  "mcp": {
    "plasma": {
      "type": "remote",
      "url": "https://plasma-mcp.bbg-x.top/mcp",
      "enabled": true
    }
  }
}
```

```bash
opencode mcp auth plasma
```

它會印出授權網址並開啟瀏覽器；此時依〈OAuth 交接是本回合終點〉立即結束本回合。
使用者完成後，回到終端機開新 session 並呼叫 `whoami` 核對。

- 狀態查詢：`opencode mcp list`
- 重新授權：`opencode mcp logout plasma` 後再 auth 一次
- 診斷：`opencode mcp debug plasma`

**授權網址一定要用 `mcp auth` 當次印出的那一個**，不要沿用先前的分頁或自行拼湊：
每次執行都會換一組 state，用到舊的會被判定為 CSRF 而失敗。

## Claude Code

宿主會載入 plugin 根目錄的 `.mcp.json`，不需執行 `claude mcp add`。
在對話中輸入 `/mcp`，選該 plugin 的 plasma 連線 → Authenticate；瀏覽器開啟時立即結束本回合。
使用者登入並選完 workspace 後，再開新 session 呼叫 `whoami`。

## Codex

宿主依 plugin manifest 的 `mcpServers` 載入 `.mcp.json`，不再修改 `config.toml`
新增重複連線。使用者說「幫我登入 Plasma／連結 Plasma」時：

- 已授權且工具可用時，以 `whoami` 核對，不重新開啟 OAuth。
- 需要登入時，優先使用宿主實際提供的 MCP／plugin 授權工具，傳入實際 server ID，
  不猜工具名稱或假設 plugin 連線名稱就是 `plasma`。
- CLI 可先以 `codex mcp list --json` 查詢；確實列出該連線時，才執行
  `codex mcp login <實際 server 名稱>` 一次。
- 無可呼叫入口時，請使用者在 MCP server 清單選該 plugin 的連線並按 Authenticate，
  然後結束回合。不得改用 Computer Use 操作設定頁或另建手動連線。
- 如果宿主沒有載入 plugin MCP，回報安裝／相容性問題，不重複啟動登入。

入口會啟動 OAuth 並開啟系統預設瀏覽器。開啟後依〈OAuth 交接是本回合終點〉立即結束
本回合；不得等待完成或呼叫其他工具。使用者完成後，再於新對話呼叫 `whoami`。

## 核對連線

`whoami` 回報 workspace 名稱與 ID、授權帳號、scopes 與知識服務狀態。
核對本次任務需要的 workspace。遇到多個 Plasma 連線，選定一個後，
Plasma 與 Ophion 的工具均使用該連線；不要混用不同連線的物件 ID。
工具名稱可能帶宿主命名空間，以實際可用工具及 schema 為準。

| scope | 允許的操作 |
|---|---|
| `views:read` | 讀取 view／mview／pview 與同步排程；每次授權都會包含這項 |
| `knowledge:read` | 查資料表／欄位意義、值域與 lineage |
| `query:run` | 執行唯讀 SQL 與 pview 驗證查詢；回傳樣本不代表完整資料 |
| `views:write` | 建立 mview／pview、啟動同步與設定排程；同步仍需 skill 個別確認 |

授權一次會取得上表全部權限，所以正常情況不會缺 scope。仍以 `whoami` 的實際
scopes 為準：舊的連線可能是在這個行為之前建立的，只帶部分權限。
遇到工具回報缺少 scope 時，請使用者重新授權一次即可，不要重複呼叫遭拒工具。

## 診斷

| 症狀 | 處理 |
|---|---|
| 安裝後沒有工具 | 確認連線已設定且該 session 重新載入過；skills 要下一個 session 才生效 |
| OAuth 導向內網或舊網域 | 請管理員修正 gateway 公開 base URL 與 OAuth discovery 中的 issuer／endpoints；plugin 不自行覆寫授權網址 |
| 選完 workspace 後頁面停在原地 | 瀏覽器擋住了導回本機的那一步，見下節 |
| 授權頁顯示「授權流程已結束／已逾時」 | 用的是舊分頁，改用當次 `mcp auth` 印出的網址 |
| 舊連線只有部分權限 | 在改為一次授予全部之前建立的，重新授權一次即可 |
| 回到宿主仍顯示未授權 | 確認瀏覽器已看到成功頁；再用 `opencode mcp debug plasma` 之類的指令查狀態 |
| 缺少 pview／sync／排程工具 | 管理員需部署新版 gateway 並設定 tool_profile=bi（或 full）；definitions 不提供新流程工具，重新登入不會補出未註冊工具 |
| 工具缺少 scope | 依上節重新授權；plugin 不能代替 gateway 授予權限 |
| Plasma `403` | 與缺少 scope 不同，由 Plasma 管理員檢查 workspace 成員與權限 |
| 有 Plasma 工具但沒有知識工具 | 用 `whoami` 區分未配置 Ophion、缺 `knowledge:read` 或服務不可用 |
| 知識工具 `404`／尚無已發布知識 | 由管理員產生並發布該 workspace 的知識版本 |
| workspace 不符 | 重新授權並選擇正確 workspace；不自行替換參數繞過 |
| 連線只使用舊式 SSE | 使用支援 Streamable HTTP 的連線方式 |

### 瀏覽器擋住導回本機

OAuth 最後一步是從 gateway 導回 `127.0.0.1` 的本機接收埠。
當 gateway 位於內網位址而且不是 HTTPS 時，Chrome 142 之後會擋掉這個跨網段導轉，
而且**不會顯示任何提示**——因為要求該權限的資格僅限 HTTPS 頁面。
症狀是按下「授權並連結」後頁面不動，宿主一直停在等待授權。

這不是 plugin 或 gateway 能修的，請管理員擇一處理：

- 為 gateway 配置用戶端信任的 HTTPS 憑證（自簽而未佈署 CA 不算）
- 或由 IT 以 Chrome 政策 `LoopbackNetworkAccessAllowedForUrls` 放行該來源

使用者端可暫時改用未套用此限制的瀏覽器。不要建議關閉瀏覽器安全設定作為常態做法。

知識恢復前，不憑欄位名稱推測語意或編寫來源 SQL。缺少資訊要據實說明。

## 流程範圍

連線確認後依 [建立 pview 技能](../plasma-create-pview/SKILL.md) 執行。
預設輸出 pview，明確指定 mview 時停在 mview；同步及排程前由 skill 等待使用者確認。
本流程不要求匯出或 API 發布權限，API 由使用者自行建立。
所需工具：whoami、list_views、get_view、run_query、create_view、sync_view、
get_view_schedule、set_view_schedule、list_pviews、get_pview、create_pview、execute_pview，
以及 Ophion 知識工具。缺少任一所需能力時回報缺口，不靜默退回舊流程。
宿主顯示其他後端工具或授權帶有較廣 scopes，不代表 plugin 應接續使用。
plugin 不提供本機 hook；同步確認是 skill 軟限制，不是 gateway 強制核准。
