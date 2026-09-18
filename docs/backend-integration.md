# ChatGPT 網頁版：後端修改與驗收清單

評估日期：2026-09-17。目標 repository：`plasma-backend`。
本文件列的是 **backend 要補的項目**；plugin v0.3.1 僅調整封裝與 skills，沒有修改後端。
MCP server 繼續由 Claude 與 ChatGPT 共用，不建立第二套工具服務。

v0.3.1 範圍更新：唯一流程是建立並核對 view／manual mview 定義，沒有後續同步、
blueprint、匯出或 API 發布。先前 B3（blueprint 輸出）與 B6（目的地／job 追蹤）
已退出本 plugin 需求，不再列為接入待辦。後端既有 API 本次未刪除。

## 已具備的能力

`pkg/mcp_gateway` 已有 Streamable HTTP、OAuth discovery、DCR、PKCE S256、
issuer／audience 驗證、refresh rotation／revoke、授權綁定 workspace，及 Ophion 代理。
現有 `go test ./pkg/mcp_gateway/` 已通過，但並非 ChatGPT 真實端到端驗收。
不需為了網頁版另加 SSE 或把 server 搬回 plugin。

## 優先完成

### B1 — 追加 scope 與 ChatGPT 授權 UI

位置：`mcp_server.go` 的 `caller.require`、`mcpHandler`；`oauth_metadata.go`、
`tools_view.go`、`tools_export.go`、`ophion_proxy.go`。

現況：端點 challenge 只提示 `views:read`，resource metadata 提供基本 scopes。
工具缺 scope 時回一般文字錯誤，沒有工具級 OAuth metadata／challenge。
`oauth_authorize.go` 採用 client 請求的 scope，重新連結相同請求不會自動加權限。
因此「重新連結即可取得新權限」不是目前程式能保證的行為。

修改：

- 依工具實際需求提供 `securitySchemes`，確認 Go SDK 的 wire serialization 能
  送出 ChatGPT 需要的欄位；必要時使用其支援的 metadata／擴充方式並驗證實際 JSON。
- 缺權限時，在 tool error result 回傳 `_meta["mcp/www_authenticate"]`，包含
  `error="insufficient_scope"`、`error_description`、resource metadata URL 與
  明確的所需 scopes。不能只更改錯誤文字。
- 追加授權保留既有必要 scopes（尤其 `views:read`）與使用者選定的 workspace；
  不靜默轉換 workspace，也不因新增一項權限讓原有工具失效。
- 確認 consent 頁呈現本次實際請求的權限，token 的 scopes 與同意一致。
- 知識工具在無 `knowledge:read` 時完全不註冊，無法靠呼叫該工具觸發 step-up。
  選定清楚的策略：初始連結明確請求知識權限，或提供不洩漏知識的穩定授權入口；
  不只靠 `whoami` 文字提示。缺權限仍不可執行知識查詢。

驗收：從只有 `views:read` 開始，分別追加 `knowledge:read`、`query:run`、
`views:write`。在 ChatGPT 真實 UI
看到追加授權、拒絕後不執行、同意後 scopes 正確且仍是原 workspace。
測試 wire metadata 和 tool result，不只斷言文字含「重新連結」。

來源：[OpenAI Authentication](https://developers.openai.com/plugins/build/auth)。

### B2 — Ophion 呼叫前重新檢查使用者權限

位置：`ophion_proxy.go: register / forward`、`mcp_server.go: verifyToken`。

現況：gateway 驗證自己的 token 與 grant；知識呼叫檢查 scope 後，直接以 service
token 向 Ophion 查詢，沒有經過 Plasma REST 的使用者／workspace 存取權檢查。
使用者被移出 workspace，或 Plasma 身分失效但 gateway grant 尚未撤銷時，
目前路徑沒有相應阻擋。這是程式路徑發現，尚未在真實部署重現。

修改：在知識工具列舉與呼叫前驗證目前 Plasma 身分及 workspace 存取權，
或建立能覆蓋移除成員、停用帳號、憑證撤銷的可靠 grant 撤銷連動。
只刷新 token 不足以證明仍是 workspace 成員；不要把 service token 當成終端使用者授權。

驗收：授權後移除 workspace 成員／停用帳號，既有 gateway token 不能繼續取得
知識工具或知識內容；明確撤銷 grant 後立即拒絕。測試中斷言 Ophion 未被呼叫。

### B4 — 工具範圍、metadata 與文字一致

位置：`mcp_server.go` 的 server instructions／工具註冊、`tools_view.go`、
`web/consent.html`。檢查後端目前的 instructions，移除對本 plugin 接入仍引導
建立 blueprint、匯出或發布的內容，避免與新 skills 的終點衝突。

plugin 只呼叫 `whoami`、知識查核工具、`list_views`、`get_view`、`run_query`、
`create_view`。若此接入需在 server 層限制範圍，建立對應 profile／allowlist，
不註冊其他操作工具；共用後端供其他 client 使用的能力不必全域刪除。
`views:write` 目前同時授予建立和同步，單靠 plugin 文字無法縮小該權限。
如要求伺服器也只允許建立定義，需驗證 type／sync_mode，拒絕 scheduled 建立及
同步呼叫，而非只隱藏工具。

`create_view` metadata 應按實際副作用設定。consent 說明與此接入提供的能力一致，
不要承諾客戶端永遠不會確認，也不把 annotations 當成人類批准證明。
本次 plugin 沒有同步流程，毋須重建舊 hook 或另做同步批准系統。

驗收：建立一般 view 和 manual mview 後無同步 job、無 blueprint／匯出／發布；
若啟用受限 profile，直接呼叫範圍外工具或傳 scheduled 也必須被後端拒絕。

## 條件式項目與可用性改善

### B5 — 企業網域限制／正式發布的 identity 資訊

目前沒有 OIDC discovery、`openid`／`email` scopes 或 UserInfo endpoint。
若要支援企業 workspace 的網域限制，補齊 discovery、UserInfo，以及可證實的
`email`／`email_verified`。不能把未驗證的 Plasma username 直接標為 verified email。
依當時公開 MCP 提交規則核對這些發布要求；基本連線測試與正式上架分開驗收。
來源：[官方認證文件](https://developers.openai.com/plugins/build/auth)、
[提交指南](https://developers.openai.com/plugins/guides/submit-claude-plugin)。

## 部署／註冊工作（不全是程式修改）

- repository 的 `[mcp_gateway]` 預設關閉；在實際部署配置 enabled、public_url、
  獨立 signing／encryption keys、Ophion endpoint 與 service token。
- gateway 使用獨立 listener（預設 5002）。反向代理需涵蓋 `/mcp`、`/oauth/*`、
  `/.well-known/*`，提供有效 HTTPS，避免僅把 MCP path 轉到 REST 的 5001。
- 驗證 issuer、resource、callback URI 一致，流式回應與逾時設定符合實際查詢。
- 目前實作 **DCR**；README 的「DCR 或 CIMD」超出程式現況。先以 DCR 測試
  ChatGPT；不必為基本相容性強制新增 CIMD，但文件應修正。若選 CIMD 才另實作。
- 在 ChatGPT 建立／註冊連線；工作區綁定用實際 app ID，公開提交則使用正式 MCP URL。
  plugin 包不會建立 app。公開提交的 domain verification、說明與隱私資訊依 portal 完成。
- 開發測試可依官方支援使用 Secure MCP Tunnel；正式公開提交仍按 portal 的 HTTPS 要求。

來源：[官方連線測試流程](https://developers.openai.com/plugins/deploy/connect-chatgpt)。

## 最終端到端驗收

在隔離測試資料上，使用 ChatGPT 網頁版完成：

1. 新連線 → OAuth → 選 workspace → `whoami` → 工具清單。
2. 知識查核 → `run_query`；缺權限時追加授權，不陷入反覆重連。
3. 建立一般 view → `get_view` 核對定義 → 交付並結束。
4. 建立 manual mview → `get_view` 核對定義 → 說明未啟動同步並結束。
5. 確認無同步／排程、blueprint、檔案輸出、外部資料庫匯出或 API 發布副作用。
6. 拒絕授權、過期 token refresh、撤銷 grant、移除 workspace 成員、Ophion 故障。
7. 換 workspace 重新授權，重新整理工具後驗證兩側一致；多個連線不混用 ID。
8. Claude Code 使用同一 gateway 回歸驗證，不需要另一套後端。

現有測試只證明 gateway 自身的一部分合約；仍需記錄真實 ChatGPT 的授權 UI、
實際 scopes、呼叫结果及流程終點，才能宣稱完整可用。
