# Plasma pview 與同步工具格式

本文件描述 MCP 工具對應的既有 Plasma REST 契約，供組合參數與理解結果使用。
**實際操作一律呼叫 MCP 工具**；workspace 與 Plasma 憑證由 gateway 的授權連線處理，
不要求使用者提供 token，不自行以 shell／HTTP 改走 REST。
新版 gateway 直接提供本流程工具，不需模式設定；實際工具 schema 優先。
本流程不建立／發布資料 API；access entry／export API 由使用者自行建立。

## 工具對照

下列 REST 路徑皆相對於 `/apis/v1/w/{workspace_id}`，workspace 不可由工具參數切換。

| MCP 工具 | REST | 主要輸入 |
|---|---|---|
| `list_pviews` | GET `/pviews` | keywords、page、page_size |
| `get_pview` | GET `/pview/{view_id}` | view_id |
| `create_pview` | POST `/pview` | name、description、sql、param_def、tags |
| `execute_pview` | POST `/pview/execute/{view_id}` | view_id、parameters、page、page_size |
| `get_view_schedule` | GET `/view/{view_id}/schedule` | view_id |
| `set_view_schedule` | PUT `/view/{view_id}/schedule` | view_id、scheduler_settings |

`create_view`、`get_view`、`list_views`、`run_query`、`sync_view` 沿用既有工具。
名稱上限：view／mview 30 個字元；不要將此上限當成已確認的 pview 後端限制。

## 建立 pview

以下 `default.sales_daily` 是示意，必須換成 `get_view` 回傳、經查詢驗證的 mview 路徑。
`report_date`、`store_id`、`sales_amount` 都必須是 mview 已產出的欄位。

```json
{
  "name": "sales_by_period",
  "description": "依日期區間篩選已同步的每日銷售資料",
  "sql": "SELECT report_date, store_id, sales_amount FROM default.sales_daily WHERE report_date >= DATE @start_date AND report_date < DATE @end_date",
  "param_def": {
    "parameters": [
      {"variable_name": "start_date", "name": "開始日期", "data_type": "date"},
      {"variable_name": "end_date", "name": "結束日期（不含）", "data_type": "date"}
    ]
  }
}
```

- SQL 用 `@name`，`variable_name` 使用不含 `@` 的名稱。每個 placeholder 都需宣告。
- `@@` 會變成 literal `@`，SQL 字串內的 email 如 `'fred@@example.com'` 也要如此跳脫。
- `data_type` 支援 `string`、`number`、`boolean`、`date`、`datetime`、`timestamp`。
- 每個 default／執行參數值格式為 `{"value":"...","is_constant":true}`。
  `value` 是字串，即使是數字或布林也如此。`is_constant=true` 表示常值；false 表示 SQL expression。
- 必填參數省略 `default_value`。若要固定預設值可提供：
  `"default_value":{"value":"2026-09-01","is_constant":true}`。
  不用任意歷史日期作為正式預設，也不要把測試日期留在定義中。
- 日期／時間常值會先以 SQL 字串呈現，pview SQL 需使用 `DATE @x`、`TIMESTAMP @x`
  或 `CAST(@x AS DATE)` 等明確型別；不要把 placeholder 再包在引號內。
  日期是否合法需透過實際執行驗證，不能假設 gateway 已驗證。
- `has_default_value` 是讀取結果中的欄位；false 表示呼叫者必須提供該值。
  不把回傳欄位直接混入建立請求。

建立回傳 `view.id`、名稱、參數定義等，但不保證包含 SQL；**建立後必須 get_pview 核對**。
不要把 mview ID 當 pview ID；兩者是不同物件與 REST 路徑。

## 執行 pview 驗證

```json
{
  "view_id": "<pview ID>",
  "parameters": {
    "start_date": {"value": "2026-09-01", "is_constant": true},
    "end_date": {"value": "2026-10-01", "is_constant": true}
  },
  "page": 1,
  "page_size": 10
}
```

這是查詢 9 月 1 日起、10 月 1 日之前的資料。start < end 由 skill 先檢查；不承諾後端
會替所有日期參數檢查起訖順序。Timestamp 欄位應依欄位型別與已確認時區調整 SQL。

回傳形狀：

```json
{
  "workspace_id": "<workspace ID>",
  "data": {"columns": ["report_date", "store_id", "sales_amount"], "rows": [["2026-09-01", "S01", 1200]]},
  "total": 1,
  "page": 1,
  "page_size": 10,
  "total_pages": 1,
  "truncated": false,
  "notice": "驗證樣本；page_size 只限制回傳列數，不限制查詢運算量。"
}
```

page 從 1 起；gateway 的 page_size 預設 10、最大 100。
這是回傳樣本限制，不是 SQL 掃描／計算量限制。後端還有自己的結果上限，
`truncated=true` 時 total 只是被截斷的結果筆數；翻頁無法取得上限之外的資料。
`data.rows=[]` 表示空結果，不自行修改業務條件讓它「有資料」。

## 首次同步與排程

建立 mview 定義：

```json
{
  "name": "sales_daily",
  "type": "materialized_view",
  "view_sql": "<已查核與驗證的完整運算 SQL>",
  "sync_mode": "manual"
}
```

`get_view` 核對並等待初始化完成。**先依 SKILL.md 取得具體同步／排程內容的使用者確認**，
才呼叫 `sync_view({"view_id":"<mview ID>"})`。
回傳表示接受非同步請求，不代表完成；以 get_view 的 last_sync_status 與 last_sync_at
判斷當次結果，不沿用上一次成功狀態。同步失敗時不得直接改查來源替代。

當次同步成功後，用 `set_view_schedule` 設定已確認的未來排程，例如每小時：

```json
{
  "view_id": "<mview ID>",
  "scheduler_settings": {
    "start": "setTime",
    "start_time": "2030-01-01T01:00:00+08:00",
    "frequency": "repeat",
    "repeat_interval": 1,
    "repeat_interval_unit": "3600"
  }
}
```

此日期只是格式示例，實際值必須是使用者已確認、仍在未來的時間。
`repeat_interval_unit` 是秒數**字串**：`"60"`、`"3600"`、`"86400"`、`"604800"`。
frequency 還支援 once、calendar，其他欄位依工具 schema；需要定期同步時不能誤用 once。
start=immediately 可能立即同步，不能在手動 sync 後無意再觸發一次。

`get_view_schedule` 回傳 `schedule=null` 表示尚未設定。存在時含 id、enabled、
scheduler_settings、last_triggered_at、trigger_count；設定會建立或更新排程並切換 sync_mode，
但更新已停用排程不等於啟用，需讀回 enabled 核對。不要自行刪除／重建既有停用排程。

## 錯誤與重試

- 缺少 scope：依 setup skill 完成授權；授權不是同步確認。
- 400：先修正參數、型別或 SQL；不重複送相同請求。
- 403／404：檢查 workspace、權限與物件，不換 workspace ID 繞過。
- 排程 409：mview 還在初始化，查狀態後再處理。
- 排程 503：後端 scheduler 不可用，不能回報排程已完成。
- 建立、同步或排程逾時：先讀取物件／同步／排程現況，再決定是否需要重試；不盲目重送。
