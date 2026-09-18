---
name: plasma-mcp-setup
description: >-
  連結 Plasma 遠端 MCP，或診斷工具缺少、連線失敗及權限不足時使用。協助完成宿主的 OAuth 登入、核對 workspace 與 scopes；適用 ChatGPT 與 Claude。全程台灣繁體中文。
---

# 連結 Plasma 與診斷

全程使用台灣繁體中文；工具名稱、SQL、識別名稱與 URL 保留原樣。
在已交付的範圍內連續完成診斷，只有缺少必要資訊時才釐清。

## 連線與工作流程

這個 plugin 提供工作流程。本流程使用的 Plasma view 工具，
以及 Ophion 知識工具，都由同一台遠端 MCP gateway 提供。
安裝 plugin 不等於已連結服務；不要求使用者安裝本機執行檔、執行 shell 或寫設定檔。

1. 工具已可用時，先呼叫 `whoami` 核對連線，不重新要求安裝或登入。
2. 工具尚未出現時，依使用者的宿主說明連結方式：
   - **ChatGPT**：在 Plugins 選擇已配置的 Plasma 連線並完成登入，再將連線加入對話。
     測試自訂部署時，使用帳號／工作區允許的 developer mode 加入管理員提供的
     HTTPS MCP URL（通常以 `/mcp` 結尾）。若沒有這個入口，請工作區管理員配置；
     不提供 Claude CLI 指令作為 ChatGPT 的安裝步驟。
   - **Claude**：在支援遠端 MCP 的連線設定中加入同一個 HTTPS URL，完成 OAuth。
     這是宿主的服務連結設定，不需要執行 plugin 提供的本機程式。
3. 使用者在自己的 gateway 授權頁登入 Plasma、選擇 workspace、查看 scopes 並同意。
   帳密與 token 不透過對話收集。
4. 回到對話後呼叫 `whoami`。若宿主仍顯示舊工具清單，重新整理連線並開新對話。

連線 URL 必須由使用者或管理員提供，不猜測部署位址。
workspace 是授權的一部分，沒有 `use_workspace` 工具；換 workspace 要在宿主重新授權。

## 核對連線

`whoami` 回報 workspace 名稱與 ID、授權帳號、scopes 與知識服務狀態。
核對本次任務需要的 workspace。遇到多個 Plasma 連線，選定一個後，
Plasma 與 Ophion 的工具均使用該連線；不要混用不同連線的物件 ID。
工具名稱可能帶宿主命名空間，以實際可用工具及 schema 為準。

| scope | 允許的操作 |
|---|---|
| `views:read` | 讀取 view／mview；端點的最低權限 |
| `knowledge:read` | 查資料表／欄位意義、值域與 lineage |
| `query:run` | 執行唯讀查詢，最多 100 列 |
| `views:write` | 本流程用於建立 view／mview 定義；後端此 scope 可能還允許其他操作 |

以 `whoami` 的實際 scopes 為準，不假定初始連結已授予前三項或所有權限。
缺少權限時說明具體 scope，使用宿主提供的追加授權流程。重新授權後再次用
`whoami` 核對；若仍缺少，停止該操作並請管理員檢查 gateway 的 scope 請求與
授權流程。不要保證「重新連結一次就會取得」，也不要重複呼叫遭拒工具。

## 診斷

| 症狀 | 處理 |
|---|---|
| 安裝 plugin 後沒有工具 | 檢查遠端 Plasma 連線是否配置、已登入並加入本次對話 |
| 反覆要求授權 | 查核連線授權／refresh 是否失效；重連無效時由管理員檢查 gateway |
| 工具缺少 scope | 依上節追加授權；plugin 不能代替 gateway 授予權限 |
| Plasma `403` | 與缺少 OAuth scope 不同，由 Plasma 管理員檢查 workspace 成員與權限 |
| 有 Plasma 工具但沒有知識工具 | 用 `whoami` 區分未配置 Ophion、缺 `knowledge:read` 或服務不可用 |
| 知識工具 `404`／尚無已發布知識 | 由管理員產生並發布該 workspace 的知識版本 |
| workspace 不符 | 重新授權選擇正確 workspace；不自行替換參數繞過 |
| 連線只使用舊式 SSE | 使用支援 Streamable HTTP 的連線方式 |

知識恢復前，不憑欄位名稱推測語意或編寫來源 SQL。缺少資訊要據實說明。

## 流程範圍

連線確認後依 [建立 view 技能](../plasma-create-view/SKILL.md) 查核來源、驗證 SQL、
建立 view／manual mview 並核對定義，到此結束。本流程不要求匯出或 API 發布權限。
宿主顯示其他後端工具或授予較廣 scopes，不代表 plugin 應接續使用。
plugin 不提供本機 hook；建立操作依使用者交付範圍與宿主權限執行。
