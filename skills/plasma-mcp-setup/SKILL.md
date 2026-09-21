---
name: plasma-mcp-setup
description: >-
  連結 Plasma 遠端 MCP，或診斷工具缺少、連線失敗及權限不足時使用。以宿主內建的 OAuth 完成授權，核對 workspace 與 scopes；適用 opencode、Claude Code 與 Codex CLI。全程台灣繁體中文。
---

# 連結 Plasma 與診斷

全程使用台灣繁體中文；工具名稱、SQL、識別名稱與 URL 保留原樣。

設定連線時只回報要使用者做什麼、以及做完的結果。不要解說 OAuth 或 MCP 的運作原理，
不要列出你讀過或檢查過哪些檔案，也不要在完成後補一段說明。

## 連線方式

本流程使用的 Plasma view 工具與 Ophion 知識工具，都由同一台遠端 MCP gateway 提供。
安裝 plugin 不等於已連結服務。連線 URL 必須由使用者或管理員提供，不猜測部署位址。

連線一律走 **OAuth**：在宿主設定 MCP server，由宿主開啟瀏覽器完成授權。
使用者以自己的 Plasma 帳號登入、選 workspace，**選定即完成授權**，沒有額外的權限確認頁。
每次授權都會取得這個部署支援的完整權限，token 由宿主保管並自動更新，
不需要手動複製任何憑證。

1. 工具已可用時，先呼叫 `whoami` 核對連線，不重新要求設定或登入。
2. 工具尚未出現時，依宿主完成下面對應的設定，再回到 `whoami`。

workspace 是授權的一部分，沒有 `use_workspace` 工具；換 workspace 要重新授權。

## opencode

在 `~/.config/opencode/opencode.json`（或既有的 `.jsonc`）的 `mcp` 區塊加入連線：

```json
{
  "mcp": {
    "plasma": {
      "type": "remote",
      "url": "<gateway>/mcp",
      "enabled": true
    }
  }
}
```

```bash
opencode mcp auth plasma
```

它會印出授權網址並開啟瀏覽器。登入後選 workspace 即完成，回到終端機開新 session，
呼叫 `whoami` 核對。

- 狀態查詢：`opencode mcp list`
- 重新授權：`opencode mcp logout plasma` 後再 auth 一次
- 診斷：`opencode mcp debug plasma`

**授權網址一定要用 `mcp auth` 當次印出的那一個**，不要沿用先前的分頁或自行拼湊：
每次執行都會換一組 state，用到舊的會被判定為 CSRF 而失敗。

## Claude Code

```bash
claude mcp add --transport http plasma <gateway>/mcp
```

加入後在對話中輸入 `/mcp`，選 plasma → Authenticate，瀏覽器登入並選 workspace 即完成。
接著開新 session，呼叫 `whoami`。

## Codex CLI

在 `~/.codex/config.toml` 加入：

```toml
[mcp_servers.plasma]
url = "<gateway>/mcp"
experimental_use_rmcp_client = true
```

首次使用時依宿主提示完成瀏覽器授權，再開新對話呼叫 `whoami`。

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

授權一次會取得上表全部權限，所以正常情況不會缺 scope。仍以 `whoami` 的實際
scopes 為準：舊的連線可能是在這個行為之前建立的，只帶部分權限。
遇到工具回報缺少 scope 時，請使用者重新授權一次即可，不要重複呼叫遭拒工具。

## 診斷

| 症狀 | 處理 |
|---|---|
| 安裝後沒有工具 | 確認連線已設定且該 session 重新載入過；skills 要下一個 session 才生效 |
| 選完 workspace 後頁面停在原地 | 瀏覽器擋住了導回本機的那一步，見下節 |
| 授權頁顯示「授權流程已結束／已逾時」 | 用的是舊分頁，改用當次 `mcp auth` 印出的網址 |
| 舊連線只有部分權限 | 在改為一次授予全部之前建立的，重新授權一次即可 |
| 回到宿主仍顯示未授權 | 確認瀏覽器已看到成功頁；再用 `opencode mcp debug plasma` 之類的指令查狀態 |
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

連線確認後依 [建立 view 技能](../plasma-create-view/SKILL.md) 查核來源、驗證 SQL、
建立 view／manual mview 並核對定義，到此結束。本流程不要求匯出或 API 發布權限。
宿主顯示其他後端工具或授權帶有較廣 scopes，不代表 plugin 應接續使用。
plugin 不提供本機 hook；建立操作依使用者交付範圍與授權權限執行。
