---
name: plasma-create-view
description: >-
  將表單、報表、BI 指標或資料需求建立為 Plasma view／mview 時使用。查核 Ophion 知識、驗證 Trino SQL、建立並核對定義後結束。全程台灣繁體中文。
---

# 查核資料需求並建立 view／mview

唯一流程：**查核來源 → 驗證 SQL → 建立 view／mview → 核對定義並交付**。

## 共通互動原則

- 全程使用台灣繁體中文；工具名稱、SQL、識別名稱與 URL 保留原樣。
- 已交付的知識查核、SQL 驗證與建立工作連續完成，不逐步詢問是否繼續。
  只釐清影響正確性的資訊；推導假設須明確標示並取得接受，不能當成來源事實。
- 原則上一份表單／報表建立一個 view／mview，整合所有區塊，不因来源不同自行拆分。
- 先用所選連線的 `whoami` 核對 workspace、權限與知識可用性，全程使用同一連線。
  工具缺少或缺權限時依 [連線技能](../plasma-mcp-setup/SKILL.md) 處理。
- 參數以工具實際 schema 為準。只使用知識查核、`whoami`、`list_views`、`get_view`、
  `run_query`、`create_view` 來完成本流程。
- 本流程在定義建立完成後結束。不建立或執行 blueprint、不輸出檔案、不匯出外部
  資料庫、不發布資料 API，也不接續呼叫 `sync_view` 或安排同步。
  若需求包含這些後續操作，先說明目前 plugin 僅支援建立 view／mview，
  不把後續操作默認成建立流程的一部分。

## 執行流程

1. **理解完整指標與表單。** 整理欄位、指標、粒度（每日、每位病人、每間門市等）、
   期間與篩選。截圖或表單可作為規格：閱讀標籤與區塊，以中文說明理解後繼續，
   只釐清影響結果的必要缺漏。

   一份表單原則上建立一個 view／mview。用一份完整 SQL 整合所有區塊，可用 CTE、
   JOIN、條件聚合或語意一致的 UNION ALL。不要另建中繼 view／mview，也不要用會
   重複計算的 JOIN 硬湊。確實無法整合時說明限制並釐清，不自行拆分或省略欄位。

2. **只從 Ophion 查找來源。** SQL 使用的每個來源表與欄位都必須由 Ophion 查得，
   不可從名稱、截圖或其他場域的經驗猜測。依序使用 `overview` →
   `search_knowledge`（多個同義詞以 OR 搜尋）→ `find_tables` →
   `get_table_card` → `list_columns`。

   卡片是索引，不是完整證據。`get_table_card` 與 `get_concept_card` 回傳的
   相關 `ku_id` 都要用 `get_knowledge_unit` 展開後才可引用或採用。
   `found=false` 時依 `suggestions` 改換詞彙；兩三組不同詞彙都未命中，
   就明列知識缺口並進入步驟 4，不重複相同搜尋或自行補上猜測。

3. **逐欄查核後才寫入 SQL。** 對每個 `database.table.column` 呼叫
   `get_column_card`，檢查型別、可否為空值、意義、來源註解、分層知識與
   每個 `unit_type` 的數量、綁定值域，以及來源的 `access_mode`、`sql_name`。

   | 查核需求 | 工具與判讀 |
   |---|---|
   | 資料表與欄位的各類知識 | `list_units(subject=database.table)`；只查資料表加 `exclude_columns=true` |
   | 特定類型知識 | 加 `unit_type`，特別查 `antipattern_trap`、`data_quality_issue`、`validity_rule` |
   | 是否可在 SQL 引用 | `get_table_card` 的 `access_mode`；`direct` 使用原樣 `sql_name`；`definition_required` 先讀 `declaration_source_refs`，不得放進 FROM／JOIN；`blocked` 解決衝突前不產生依賴它的 SQL |
   | 實際衝突內容 | `list_conflicts`；卡片只列數量 |

   某類知識未記錄，只代表該主體沒有這類紀錄；「未記錄陷阱」不等於「沒有風險」。
   綁定值域表示欄位使用代碼，所有篩選值都要經 `search_value_candidates` →
   `plan_value_filter` 解析，不手填猜測的代碼。

   概念上的知識需另行查核。工具清單有 `find_concepts`／`get_concept_card` 時使用；
   預設 `text-to-sql` profile 未提供時，改用 `search_knowledge` 與
   `get_knowledge_unit` 查核概念規則，不杜撰工具或略過查核。
   採用欄位前先向使用者說明所有已發現陷阱
   與來源證據，不另加逐項核准。每筆 `search_knowledge` 結果的 `subject`
   是實際主體，`about` 是彙整的資料表頁面，可直接據此前往欄位卡片。

4. **無法由單一資料表回答時，依序處理。**
   - 先查既定規則：各資料表以 `list_units(subject=database.table)` 逐類查
     `business_rule`、`validity_rule`、`state_machine`、`event_lifecycle`，
     再查核概念規則（依步驟 3 的可用工具）。`governed_by` 包含概念本身及 `SAME_AS`、
     `NORMALIZES_TO` 相關概念的規則。最後才用關鍵字擴大搜尋，不能只查關鍵字。
     命中均用 `get_knowledge_unit` 展開；有規則就忠實實作並引用。
   - 無規則但已查核欄位足夠時，提出明確標示為候選的推導，以中文說明欄位、
     關聯、篩選及假設，不宣稱為 Ophion 規則。可繼續驗證 SQL，不另加核准；
     建立前說明定義與驗證結果，必要假設取得使用者接受後才建立。有互斥解讀或必要
     資訊不足時先釐清。
   - 缺少必要資料時，列出指標需要的事實、應由哪個資料表／欄位承載、搜尋
     詞彙與知識類型，以及未命中的結果。區分「Ophion 尚未收錄」與「來源
     系統沒有記錄」。提出補入人工裁定知識、調整為資料可支援的指標或補充來源，
     不交付看似合理的近似結果。

5. **只使用 Trino SQL。** Plasma 透過 Trino 查詢。
   識別名稱採兩段式 `database.table`，workspace 已提供 catalog 範圍；
   原樣使用 Ophion 的 `sql_name`，不重建來源路徑。一般 snake_case 名稱不需
   引號；保留字或特殊字元用雙引號包住單一識別名稱，不包住整段含點號路徑。
   字串使用單引號。

   將步驟 3 的資料品質排除條件落實於 WHERE，不能只在對話提醒。
   代碼篩選使用 `plan_value_filter` 的結果。

   | 使用方式 | 避免方式 |
   |---|---|
   | `CAST(x AS type)` | `x::type` |
   | `DATE '2026-01-31'`、`TIMESTAMP '2026-01-31 10:00:00'` | 以一般 VARCHAR 字串當日期 |
   | `date_diff('day', a, b)` 計算經過單位 | 將 `b - a` 的 INTERVAL 當數字 |
   | `date_add('day', 7, x)` 或 INTERVAL 常值 | `DATEADD`、`DATE_SUB`、`x + 7` |
   | `CURRENT_DATE`、`CURRENT_TIMESTAMP` | `NOW()`、`GETDATE()`、`SYSDATE` 或加引號的名稱 |
   | `\|\|` 或 `concat()` | 用 `+` 串接字串 |
   | `IS NULL`、`IS NOT NULL` | `= NULL`、`<> NULL`、`ISNULL()`、`NVL()` |
   | `COALESCE` | `IFNULL`、`NVL` |
   | `approx_percentile(x, 0.5)` | `PERCENTILE_CONT`、`MEDIAN`、`APPROX_QUANTILE` |
   | 兩側用 `LOWER()` 不分大小寫比對 | `ILIKE` |
   | `COUNT(DISTINCT (a, b))` | `COUNT(DISTINCT a, b)` |
   | CTE 中用 `row_number() OVER (...)`，外層篩選 | 用 LIMIT 代替組內排名、TOP、ROWNUM |
   | `LIMIT n` | `TOP n`、`FETCH FIRST`、`ROWNUM <= n` |
   | 單一陳述式，不附分號 | 尾端分號、多個陳述式、SET／USE／暫存資料表 |

   整數除法會截斷小數；比率或平均值將運算元轉為 DOUBLE。DATE 與 TIMESTAMP
   比較時明確轉型。SELECT 的非聚合運算式都需列入 GROUP BY。
   不確定函式是否支援時，先用 `run_query` 驗證，不把猜測放入正式定義。

6. **執行 SQL 驗證並呈現結果。** 用 `run_query` 驗證整份表單的欄位、粒度與
   定義，中文回報代表性資料及品質問題，不逐次要求查詢核准。這會實際透過
   Trino 查詢來源，不是模擬；最多回傳 100 列，只能作為樣本，不代表完整筆數。
   將步驟 4 的候選假設納入建立前的定義摘要。SQL 驗證完成後才建立 view／mview。

7. **建立 view／mview 定義。**

   確認物件名稱、說明與類型。沿用使用者已指定的 view 或 mview；未指定且需求
   無法判斷時，只釐清這項選擇，不因後續匯出用途自行代選。
   使用 `list_views`、`get_view` 查核同名既有物件；定義完整相符時可重用並明確說明，
   不自行覆寫或重複建立。

   - **view**：呼叫 `create_view(type=view)`，提供已驗證的 `view_sql`，
     省略 `sync_mode` 與 `scheduler_settings`。
   - **mview**：呼叫 `create_view(type=materialized_view, sync_mode=manual)`，
     提供已驗證的 `view_sql`，不設定 `scheduler_settings`。

   `sync_mode=scheduled` 可能立即啟動同步，不屬於本流程；即使要求排程，
   也不藉建立動作啟動它，應先說明此流程只建立定義。
   不把 SQL 驗證的 100 列樣本上限留在完整定義中，除非需求本身需要該限制。
   建立逾時時先查核物件是否存在，不直接重送建立要求。

8. **核對定義、交付並結束。**

   用 `get_view` 核對回傳 ID、名稱、類型、SQL 與可用的同步模式資訊。
   交付 workspace、view／mview ID、名稱、類型、定義摘要、SQL 驗證結果及已接受的
   假設。新建 manual mview 應說明「定義已建立，本流程未啟動資料同步」。
   重用既有物件時忠實呈現其現況，不宣稱是本次新建或本次已同步。
   讀取核對失敗時回報已知建立結果與尚未核對項目，不誤報完整成功。

   到此結束，不要求使用者提供匯出目的地、檔名、API 驗證方式或有效期限，
   不提出下一步執行 blueprint、同步或發布 API。
