# mview 的知識查核與 SQL 驗證

以下用於 mview 運算層；pview 僅讀取已核對的 mview 欄位並設定最後篩選。

1. **理解完整指標與表單。** 整理欄位、指標、粒度（每日、每位病人、每間門市等）、
   期間與篩選。截圖或表單可作為規格：閱讀標籤與區塊，以中文說明理解後繼續，
   只釐清影響結果的必要缺漏。

   一份表單的運算原則上整合在一個 mview。用一份完整 SQL 整合所有區塊，可用 CTE、
   JOIN、條件聚合或語意一致的 UNION ALL。除最後的 pview 篩選層外，不要另建中繼 view／mview，也不要用會
   重複計算的 JOIN 硬湊。確實無法整合時說明限制並釐清，不自行拆分或省略欄位。

2. **只從 Plasma 知識庫查找來源。** SQL 使用的每個來源表與欄位都必須由 Plasma 知識庫查得，
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
     關聯、篩選及假設，不宣稱為 Plasma 知識庫規則。可繼續驗證 SQL，不另加核准；
     建立前說明定義與驗證結果，必要假設取得使用者接受後才建立。有互斥解讀或必要
     資訊不足時先釐清。
   - 缺少必要資料時，列出指標需要的事實、應由哪個資料表／欄位承載、搜尋
     詞彙與知識類型，以及未命中的結果。區分「Plasma 知識庫尚未收錄」與「來源
     系統沒有記錄」。提出補入人工裁定知識、調整為資料可支援的指標或補充來源，
     不交付看似合理的近似結果。

5. **只使用 Trino SQL。** Plasma 透過 Trino 查詢。
   識別名稱採兩段式 `database.table`，workspace 已提供 catalog 範圍；
   原樣使用 Plasma 知識庫的 `sql_name`，不重建來源路徑。一般 snake_case 名稱不需
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
   將步驟 4 的候選假設納入建立前的定義摘要。SQL 驗證完成後才建立 mview。

