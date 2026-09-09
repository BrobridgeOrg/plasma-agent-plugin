---
name: plasma-postgres-export
description: >-
  當使用者希望將表單、報表、BI 指標或查詢結果直接寫入 PostgreSQL（PG）指定資料表時使用。查核 Ophion 知識與 Trino SQL，建立 view，在 blueprint 選取此 view 並設定 PG 目的地，再執行及驗證同步。需要資料 API 時改用 plasma-data-api。
---

# 將 BI 資料透過 view → blueprint → PG 匯出

## 目標與互動原則

固定流程為：**建立 view → 設定 blueprint 並選取此 view → 拉到目標 PG 資料表**。
使用一般 `view`，不先建立 `mview`，也不開啟資料 API。

全程使用台灣繁體中文，保留 view、mview、blueprint、PG、workspace 等系統功能名稱，
以及工具名稱、SQL、識別名稱與參數。已交付的知識查找、SQL 驗證、view 建立及
不啟動同步的 blueprint 設定連續完成，不逐步詢問是否繼續。只釐清影響結果或
目的地正確性的缺漏，啟動同步前集中確認一次。

原則上一份表單對應一個 view 及指定目標資料表，不因區塊或來源不同而自行拆分。
先查核既有物件，合適時重用。

## 1. 確定完整需求與目的地

整理所有欄位、指標、粒度、期間與篩選。**PG 連線必須反問使用者選擇，不自行代選。**

先呼叫 Ophion MCP 的 `overview`，從目前 workspace 的 DB 清單整理已與 Plasma
串接的候選 DB。依當下 Ophion 可用的工具補齊資訊，不杜撰 `list_databases` 等
不存在的工具。用 `list_pg_connections` 分頁核對 PG DBC 的顯示名稱、實際資料庫、
schema 與連線狀態；Ophion 的知識識別名稱不一定是 `dbc_id`，不可直接拿來呼叫 API。
以表格列出可辨識的選項，反問：「這次要把資料寫到哪個 PG 連線？」即使只有一個
候選，也請使用者確認選擇，不預設使用來源 DB、第一筆或名稱最相近的連線。

沿用使用者在本次任務已明確選定的連線，不重複問。尚未選定時可繼續來源查核、
SQL 驗證與 view 建立，但不可建立 blueprint 或寫入 PG。選項重名、Ophion 與
Plasma 對應不明時集中釐清，不能把名稱相似當成同一個 DB。Ophion 無法取得清單時
說明原因，提供 Plasma 查得的候選作為替代資訊，仍由使用者選擇。

選定後用 `get_pg_connection(dbc_id=...)` 核對資料庫、schema、主機與狀態，
取得目標資料表名稱。目前使用既有 DBC，不收集或回傳連線密碼；尚未串接的 PG
需先在 Plasma 建立連線，不能自行改用臨時連線。

寫入模式必須明確指定，沒有預設值：`append` 新增資料；`overwrite` 替換整張表，
後端可能刪除後重建；`truncate` 清空既有資料後寫入。後端不支援此流程的 upsert，
不能承諾依主鍵更新。另確認 `force_create_table`，表示目標不存在時是否建立。
欄位名稱、順序與型別需在來源 SQL 中符合目標，不杜撰欄位對應或主鍵參數。
需要排程時另記錄頻率；目前新工具只提供單次執行，不宣稱已設定排程。

## 2. 查核知識並驗證 SQL

依 [Ophion 知識查找技能](../ophion-knowledge-lookup/SKILL.md) 查明來源與證據，
並共用 [資料 API 技能的步驟 1–6](../plasma-data-api/SKILL.md) 完成逐欄、概念、
代碼、規則、存取模式與 SQL 查核。只共用查核步驟，不接續建立 mview 或發布 API。

來源查詢使用 Trino 方言，即使目的地是 PG 也不改寫為 PG 語法。以 `run_query`
驗證完整欄位與粒度；100 列結果是樣本，不是匯出管道，也不要把驗證用的列數限制
留在完整資料定義中。候選推導需明確標示，最後同步確認時取得同意。

## 3. 建立 view

使用 `list_views`、`get_view` 查核既有物件。需要新建時呼叫 `create_view`，
提供名稱、說明及已驗證的 `view_sql`，明確設定 `type=view`，省略 `sync_mode`
與 `scheduler_settings`。

保留回傳的 view ID 與名稱，必要時用 `get_view` 驗證定義。建立成功只代表 view
已存在，不代表資料已寫入 PG。不要呼叫 `sync_view` 或等待 mview 的
`last_sync_status=synced`；PG 資料搬移由 blueprint 執行。

## 4. 設定 blueprint 與 PG 目的地

先用 `list_blueprints` 分頁搜尋既有 user query blueprint，再以 `get_blueprint`
核對來源 `source_view_id`、PG DBC、目標表、寫入模式與是否允許建表。只有完整符合
需求才重用，不能因名稱相同就執行既有 blueprint。此列表也可能有非 PG 目的地。

新建使用 `create_pg_blueprint`，明確提供 `name`、`view_id`、`dbc_id`、
`database`、`schema`、`table`、`write_mode`、`force_create_table`，可加 `description`。
`table` 是單一資料表名稱，不加 schema 前綴或 SQL 引號。

工具以 `source_view_id` 綁定 view，後端在執行時從 view 的 path 組成來源查詢，
不複製來源 SQL。PG 目的地使用 `target.method=static` 與 `target.dbc_id`；
後端從 DBC 取得 catalog、schema 與憑證，會覆寫呼叫端的連線座標。因此要求的
database／schema 必須與 DBC 相符，不符時請使用者選另一個 DBC，不能假裝已切換。

建立 blueprint 不啟動 job，預設禁止同一 blueprint 同時執行多個 jobs。
建立後保留 ID，用 `get_blueprint` 核對實際儲存的設定。
此流程使用一般 blueprint API，不用只適用 mview 的 `view/{id}/export_blueprints`。
`get_view(with_blueprint_history=true)` 是 mview 歷程，不能代替此流程的 blueprint 查詢。

## 5. 確認並執行同步

啟動前一次呈現 workspace、view、blueprint、完整 PG 目的地、欄位範圍、日期
與篩選、SQL 驗證結果、寫入方式、是否允許建表及候選假設。明確說明會從來源拉取資料、
使用資源並寫入指定 PG 資料表；涉及覆寫或刪除時指出對既有資料的影響。
尤其 `overwrite` 可能刪表重建而影響索引／限制；`truncate` 會先清空，後續寫入
失敗可能留下空表。這些影響放在同一次同步確認中，不默默採用破壞性模式。

使用適用的宿主確認介面，否則取得中文明確同意；同一操作不重複確認，不繞過
宿主權限。未獲同意不啟動同步，未涵蓋的新執行或重試需重新確認。
確認前記錄 `get_blueprint` 的 `last_job_id` 及近期 `list_blueprint_jobs` 結果，
確認後呼叫 `spawn_blueprint_job(blueprint_id=...)`。不要用 `sync_view` 或自行
向 PG 寫入代替 blueprint。若工具呼叫逾時，先查 jobs 判斷是否已受理，不直接重送。

## 6. 驗證與交付

`spawn_blueprint_job` 只回傳受理訊息，沒有本次 job ID，也不代表完成。
用 `list_blueprint_jobs` 比對執行前紀錄，找到本次新 job，再用 `get_job` 追蹤。
非同步建立可能稍有延遲，尚未看到新 job 時繼續查詢，不再觸發一次。
若同時有其他人或排程執行而無法辨識本次 job，明列不確定性，不能任取最新一筆。

只有本次 job 的 `status=completed` 才能回報後端匯出完成；`failed`、`invalid`、
`cancelled`、`aborted` 均不是成功，依 `details` 說明原因。`started`、`ready`、
`running` 表示尚未完成。不能將舊 job 的成功或 blueprint 的舊 `last_job_status`
當成本次結果。

透過已授權且可用的 PG 查詢方式，或使用者提供的驗證結果，核對實際目標表、
欄位、筆數與代表性資料。工具沒有直接 PG 查詢功能；未查核時，分別說明
「後端 job 已完成」與「目的地資料內容尚未另行驗證」。來源預覽不能證明 PG 內容。

交付 view／blueprint ID、PG 資料庫／綱要／資料表、寫入方式、更新頻率、執行
狀態及驗證結果。無法查核目的地時明列待驗證項目，不宣稱全部完成。提供 BI
工具連至該 PG 資料表的資訊，不交付資料 API URL。
