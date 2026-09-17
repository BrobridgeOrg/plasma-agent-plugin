---
name: plasma-mcp-setup
description: >-
  接上 Plasma 的 MCP server、或工具缺少、連線失敗、權限不足時使用。說明如何用 URL 連線並完成 OAuth 授權（登入後選 workspace），如何確認目前連到哪個 workspace 與有哪些權限，以及同步與開 API 前確認閘的行為。全程台灣繁體中文。
---

# 連上 Plasma MCP 與診斷

## 共通互動原則

- 所有對使用者的回覆都使用**台灣繁體中文**，包含進度、問題、結果、錯誤說明、
  確認文字及交付說明。工具名稱、SQL、欄位名稱、URL 與需忠實引用的原文保留原樣，
  並以台灣繁體中文解釋。
- 在使用者已交付的任務範圍內連續完成診斷步驟，報告進度即可，不要每一步都問
  「是否繼續」。只有缺少會影響正確性的必要資訊時才釐清。
- 資料 API 原則上一份表單建立一個 mview；指定外部資料表則使用 `plasma-export`，
  依 view → blueprint → 目的地執行，不因不同區塊或來源自行拆分。

## 這個 plugin 提供什麼、不提供什麼

**不提供 MCP 工具。** Plasma 的工具（view、blueprint、job、資料 API）與知識工具
（`overview`、`get_table_card`、`get_column_card`…）**都由 plasma-backend 的
MCP gateway 提供**，是一台用 URL 連上的 HTTP MCP server。知識那半是 gateway
向 Ophion 轉發的，所以**使用者只需要加一台 server**，兩半也不可能被指到不同的
workspace。

這個 plugin 剩下兩件事：

1. **工作流 skill** —— `plasma-data-api`、`plasma-export`、`ophion-knowledge-lookup`。
2. **同步與開 API 前的人工確認閘**（PreToolUse hook）。gateway 的授權是連結時
   一次給定 scope，之後不會再問；真正要動資料的那一刻的確認只能從用戶端這側來，
   就是這個閘。

沒有 `config.env`，沒有要填的 endpoint 或帳密，也沒有狀態檔。

## 連線

使用者在自己的 MCP 用戶端加一台 server，只需要 URL：

```
https://<gateway 的 public_url>/mcp
```

例如 Claude Code：

```
claude mcp add --transport http plasma https://mcp.example.com/mcp
```

接著用戶端會自己走完 OAuth 2.1：

1. 第一次呼叫拿到 `401`，用戶端從中讀到授權伺服器位置
2. 用戶端自行註冊（DCR），開瀏覽器
3. **登入頁** —— 輸入 Plasma 帳號密碼
4. **選 workspace** —— 只會列出這個帳號有權限的 workspace
5. **同意頁** —— 顯示用戶端名稱、workspace 與每一項權限的中文說明
6. 同意後導回，用戶端拿到 token

**全程不需要複製貼上任何 token。** 使用者只在瀏覽器輸入一次帳密、點一個 workspace。

### 兩件要先告訴使用者的事

- **登入頁會問 Plasma 密碼，而它不是 Plasma 的網域。** 這是目前的實作方式，
  形狀確實跟釣魚頁一樣。請使用者確認網址正是他們自己部署的 gateway，
  不要在任何其他地方輸入 Plasma 密碼。
- **workspace 在同意當下就固定寫進憑證。** 沒有工具能切換；要換 workspace
  或增加權限，就是重新連結一次。

## 確認連上了什麼

**第一個呼叫一律是 `whoami`。** 它回答四件事：

| 欄位 | 意義 |
|---|---|
| `Workspace` | 這個連線綁定的 workspace，名稱與 ID |
| `User` | 授權這個連線的 Plasma 帳號 |
| `Knowledge` | 知識工具現在可不可用，以及為什麼不可用 |
| 權限清單 | 這個連線實際拿到的 scope，各附中文說明 |

workspace 不是預期的那個，就請使用者重新連結，不要嘗試用工具繞過。

## 權限

| scope | 允許什麼 |
|---|---|
| `views:read` | 讀 view、blueprint、job、access entry。最低門檻 |
| `knowledge:read` | 查資料表與欄位的意義、值域、lineage |
| `query:run` | 執行唯讀查詢（用 Trino 資源，最多 100 列） |
| `views:write` | 建立 view／mview、觸發同步 |
| `export:write` | 建立匯出設定（不執行） |
| `export:run` | 執行匯出，真的寫入目的地資料表 |
| `data-api:publish` | 把 view 發布成對外資料 API |

連結時預設只要前三個。工具回報缺少某個 scope 時：**告訴使用者需要哪個權限、
並說明重新連結一次就會授予；不要重試那個呼叫。**

## 診斷

| 症狀 | 意義 | 處理 |
|---|---|---|
| 完全沒有 Plasma 工具 | 沒連上，或授權沒完成 | 確認 server URL，重跑一次授權流程 |
| 反覆跳出授權 | 授權被撤銷、使用者改過密碼，或 refresh token 過期 | 重新連結 |
| 工具說缺少某個 scope | 連結時沒授予這項權限 | 說明需要哪一項，請使用者重新連結 |
| Plasma 回 `403` | 已通過驗證，但這個帳號在該 workspace 權限不足 | 這不是 MCP 的 scope 問題，要在 Plasma 調整成員權限 |
| 有 Plasma 工具但沒有知識工具 | 見 `whoami` 的 `Knowledge` 一行 | 未設定／沒有 `knowledge:read`／知識服務不可用，三種處理不同 |
| 知識工具回 `404` 或「尚無已發布知識」 | 這個 workspace 還沒發布知識版本 | 不是設定錯誤，需先產生並發布 |
| 用戶端說只支援 SSE | 這個端點只講 Streamable HTTP | 用支援 Streamable HTTP 的用戶端 |

知識服務不可用時，Plasma 自己的工具仍然正常。**但在知識恢復前不要憑欄位名稱
推測語意去寫 SQL** —— 據實說明現在查不到，比給一份看起來合理的猜測好。

## 確認閘

`create_view`（`sync_mode=scheduled`）、`sync_view`、`create_access_entry`、
`spawn_blueprint_job` 這四個呼叫之前，plugin 會跳出中文確認，說明這次會花掉什麼、
會寫到哪裡、`overwrite`／`truncate` 對既有資料的影響。

查找、`run_query` 驗證、建立不啟動同步的 view 或手動 mview **不會**被攔，
那些是準備工作。不要為了避開確認而改用別的工具達成同樣的效果。
