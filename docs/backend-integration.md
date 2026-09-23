# pview 工作流程：後端整合與驗收

更新日期：2026-09-22。Plugin v0.7.0 對應 `plasma-backend` 的 MCP gateway pview／mview 工作流程。
本次後端程式修改限於 `pkg/mcp_gateway`；沿用既有 REST API、pview 引擎、mview 同步與排程。
不新增來源限制、資料模型、核准機制或資料 API 發布能力。

## 工作流程與責任

Plugin 維持三份 skills：setup、Plasma 知識庫查核與 plasma-create-pview。
預設：知識查核 → SQL 驗證 → manual mview → 使用者確認 → 首次同步成功 → 定期排程 → pview → 驗證交付。
使用者明確指定 mview 為最終目標時，於同步／排程核對後交付 mview。
只建立定義或拒絕同步時不執行同步，交付已建立定義與尚未完成項目。

運算在 mview 完成，pview 只做最後的參數化 WHERE；mview 保留必要欄位、粒度與歷史範圍。
這是 plugin 預設，不能在 gateway／domain manager 強制 pview 只能引用 mview。
資料 API、access entry、export API 由使用者自行建立，不在 plugin 流程內。

## Gateway 工具

| 工具 | REST（相對於 /apis/v1/w/{workspace_id}） | scope |
|---|---|---|
| list_pviews | GET /pviews | views:read |
| get_pview | GET /pview/{view_id} | views:read |
| create_pview | POST /pview | views:write |
| execute_pview | POST /pview/execute/{view_id} | query:run |
| get_view_schedule | GET /view/{view_id}/schedule | views:read |
| set_view_schedule | PUT /view/{view_id}/schedule | views:write |

沿用 whoami、list_views、get_view、run_query、create_view、sync_view 與 Plasma 知識庫工具。
每項呼叫使用授權綁定的 workspace 與使用者的 Plasma token，不能由工具切換 workspace。
create_pview 後需 get_pview 讀取完整 SQL；參數 schema 保留 default_value 與 has_default_value。
execute_pview 預設每頁 10 列、最大 100 列；保留後端的 total、total_pages 與 truncated。
分頁只限制回傳樣本，不限制來源掃描量，也不能突破後端結果上限。

## 部署設定

部署新版 backend 並重新啟動 gateway，即提供完整 pview／mview 工具組，無須設定 `tool_profile`。
舊設定若仍有此欄位，可移除；它不再控制工具能力。
宿主刷新工具清單／重開 session，權限不足時再重新授權。

Gateway 只提供新流程的建立、查詢、同步與排程工具，不提供發布／匯出／blueprint／job 工具。
支援 views:read、knowledge:read、query:run、views:write，不需要 data-api:publish 或 export scopes。
本次程式提交不修改部署環境或 config.toml 的個人連線設定。

公開 MCP endpoint 為 https://plasma-mcp.bbg-x.top/mcp。
保留受信任 HTTPS、正確 public_url／issuer／discovery 與 callback 設定。
2026-09-22 先前連線測試曾看到 discovery 指向舊內網網址；本次未重新驗證部署，
發布前需確認外部 OAuth 流程，不將本機測試通過視為部署已完成。

## 同步確認與排程

同步確認採 skill 軟限制，不新增 approval URL、token、confirmed 參數或 server gate。
OAuth 授權、工具 metadata 與一般報表需求均不視為使用者已確認同步。

1. create_view 建立 manual mview，不附 scheduler_settings。
2. 等初始化完成，呈現 mview、來源、資料範圍、同步與排程時間／頻率／時區。
3. 使用者明確確認後，執行一次 sync_view，依 get_view 的當次狀態及時間核對。
4. 同步成功後以未來 start=setTime 設定排程，避免 immediately 觸發重複同步。
5. get_view_schedule 核對 settings 與 enabled；get_view 核對 sync_mode。

更新既有停用排程不保證會啟用；工具只反映 REST 回傳，不假裝已啟用。
初始化中可回 409，scheduler 不可用可回 503；逾時先讀取現況，不盲目重試。
同步失敗不回退到直接查來源；既有合適且新鮮的 mview 可重用，不強迫再次同步。

## 驗收

自動化：gateway 的 REST 路由／參數、使用者 token、scope 拒絕、工具清單與範圍外呼叫拒絕、
pview 參數與截斷回應、排程 enabled/null 與錯誤回傳、既有 OAuth／view 工具回歸。
Plugin 驗證三宿主 skills 與 reference 文件一致、連結可解析、舊 skill 不再封裝。

實際環境仍需驗收：

- 預設 pview 與明確指定 mview 的兩條路徑。
- 未確認、拒絕同步、變更同步內容後重新確認；這是宿主中的行為驗收，單元測試不能證明 AI 一定遵守。
- 首次同步、排程觸發、既有物件重用、失敗／逾時後續查，不重複同步。
- pview 引用已同步 mview，兩個不同區間、邊界與空結果正確。
- 運算留在 mview，pview 只有欄位選取與 filter；不發布資料 API。
- 使用者自行建立 API 後，可另驗證 Power BI 的參數連接方式；這不屬於本次自動建立流程。
