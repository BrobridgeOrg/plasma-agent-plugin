---
name: plasma-data-api
description: >-
  當使用者想把表單、報表、BI 畫面或指定指標做成 Plasma 資料 API 時使用。以 Ophion 查核來源與定義，驗證 Trino SQL，原則上一份表單建立一個 mview；全程使用台灣繁體中文，只在開始同步拉資料及後續開啟 API 時確認，最後交付 URL、驗證方式與有效期限。直接寫入 PG 指定資料表時改用 plasma-postgres-export。
---

# 從資料需求建立可呼叫的 API

## 共通互動原則

- 所有對使用者的回覆都使用**台灣繁體中文**，包含進度、問題、結果、錯誤說明、確認文字及交付說明。工具名稱、SQL、欄位名稱、URL 與需忠實引用的原文保留原樣，並以台灣繁體中文解釋。
- 在使用者已交付的任務範圍內，連續完成知識查找、欄位查核、SQL 驗證及不會啟動同步的 mview 建立；報告進度即可，不要每完成一步就問「是否繼續」。只有缺少會影響正確性的必要資訊時才釐清，釐清不等於每一步都要核准。
- 確認集中在兩個執行時點：**開始同步拉資料**，以及同步成功後**開啟資料 API**。每次以中文清楚說明具體影響；同一動作不要先在對話問一次、又重複要求一次工具確認。若宿主提供符合需求的確認介面，使用該介面；否則以中文取得明確同意後再呼叫工具。
- **原則上一份表單／報表建立一個 mview。** 不因不同區塊、指標、頁籤或來源表就拆成多個 mview；只有使用者明確要求拆分，才改變這個原則。

使用者不需要熟悉資料結構。協助找到來源、驗證 SQL 符合整份需求，再交付端點。
若目標是直接寫入 PostgreSQL 指定資料表，改用 `plasma-postgres-export` 的 view → blueprint → PG 流程。

## 執行流程

1. **理解完整指標與表單。** 整理欄位、指標、粒度（每日、每位病人、每間門市等）、
   期間與篩選。截圖或表單可作為規格：閱讀標籤與區塊，以中文說明理解後繼續，
   只釐清影響結果的必要缺漏。

   一份表單原則上建立一個 mview。用一份完整 SQL 整合所有區塊，可用 CTE、
   JOIN、條件聚合或語意一致的 UNION ALL。不要另建中繼 mview，也不要用會
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

   概念上的知識不會出現在資料表／欄位查詢，必須再用 `find_concepts` →
   `get_concept_card` 查核指標概念。採用欄位前先向使用者說明所有已發現陷阱
   與來源證據，不另加逐項核准。每筆 `search_knowledge` 結果的 `subject`
   是實際主體，`about` 是彙整的資料表頁面，可直接據此前往欄位卡片。

4. **無法由單一資料表回答時，依序處理。**
   - 先查既定規則：各資料表以 `list_units(subject=database.table)` 逐類查
     `business_rule`、`validity_rule`、`state_machine`、`event_lifecycle`，
     再讀概念卡片。`governed_by` 包含概念本身及 `SAME_AS`、
     `NORMALIZES_TO` 相關概念的規則。最後才用關鍵字擴大搜尋，不能只查關鍵字。
     命中均用 `get_knowledge_unit` 展開；有規則就忠實實作並引用。
   - 無規則但已查核欄位足夠時，提出明確標示為候選的推導，以中文說明欄位、
     關聯、篩選及假設，不宣稱為 Ophion 規則。可繼續驗證 SQL，不另加核准；
     定義與驗證結果放入最後同步確認，明確接受後才同步。有互斥解讀或必要
     資訊不足時先釐清。
   - 缺少必要資料時，列出指標需要的事實、應由哪個資料表／欄位承載、搜尋
     詞彙與知識類型，以及未命中的結果。區分「Ophion 尚未收錄」與「來源
     系統沒有記錄」。提出補入人工裁定知識、調整為資料可支援的指標或補充來源，
     不交付看似合理的近似結果。

5. **只使用 Trino SQL。** Plasma 透過 Trino 查詢，目的地不改變來源方言。
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
   將步驟 4 的候選假設放入最後同步摘要。SQL 驗證完成後才建立 view／mview。

7. **建立一個 mview，並在開始同步時確認。**

   預設以 `create_view(type=materialized_view, sync_mode=manual)` 建立整份表單的
   單一 mview 定義，不另外要求核准建立動作。建立定義不代表已同步完成。
   SQL 必須先通過步驟 6；已經存在同一份表單的 mview 時先查核並重用適合的
   物件，不為各區塊重複建立。

   在呼叫 `sync_view` 前，集中呈現 workspace、表單／mview 名稱、來源、完整
   表單的欄位涵蓋範圍、日期與篩選、SQL 驗證結果，以及尚待接受的推導假設。
   以台灣繁體中文取得這一次同步的確認，例如：

   > 即將同步「＜表單名稱＞」對應的「＜mview 名稱＞」。執行 sync 後，系統就會
   > 開始依照上述 SQL 從來源系統拉取資料，並寫入／更新這個 mview，會使用查詢
   > 與同步資源。這次同步尚不會開啟資料 API。是否確認開始同步？

   若有推導假設，將它們明列在同一份確認內容，讓使用者一併確認定義與同步。
   未取得明確同意就停在這個時點，不得呼叫同步、不得用其他工具繞過。

   **排程是相同確認時點的例外路徑。** 若使用者已要求排程，使用
   `create_view(sync_mode=scheduled, scheduler_settings=...)` 會立刻啟動首次
   同步，因此必須先完成上述確認，再呼叫 `create_view`；同時以中文說明首次
   拉資料會立即開始，以及後續自動拉資料的頻率。不要先建立 manual mview
   再另建一個 scheduled mview。排程參數若尚未指定，於這次同步確認一併釐清。

   取得確認後執行該次同步，持續用 `get_view` 查狀態，不逐次詢問。只有實際
   回報 `last_sync_status=synced` 才進入開 API 階段；失敗時說明原因，不能宣稱
   已完成。新的同步或重試若未包含在原確認範圍內，需要新的同步確認。

8. **同步成功後，一次確認開啟 API 的內容。** 將 API 設定整理成一份中文
   確認，不要逐欄訪談或先建立才補問：
   - 對應的表單、單一 mview 與要提供的資料範圍。
   - 驗證方式：`api_key`、`basic_auth` 或 `none`；沿用使用者已指定的選擇。
     未指定時可以提出 `api_key` 的建議，於這次確認取得同意後才採用。
     `basic_auth` 需要 `secret_key=username:password`。
   - 有效期限：明確列出 `expires_in`；未填時由 Plasma 決定，不能假設有期限。未指定時於同一
     份確認提出期限建議或詢問必要資訊，不能默默開成永久有效。

   確認文字須以台灣繁體中文說明，例如：

   > 「＜mview 名稱＞」已同步成功。下一步將開啟資料 API，讓可連線到此端點且
   > 通過＜驗證方式＞的呼叫者讀取上述資料；有效期限為＜期限＞。
   > 是否確認開啟 API？

   若 `auth_type=none`，必須改成明確說明「任何可連線到此端點且持有 URL 的人，
   不需驗證即可讀取資料」，並在同一次開 API 確認取得明確同意。
   同意同步不等於同意開 API。

9. **發布並交付。** 完成步驟 8 的確認後呼叫 `create_access_entry`，必要時再用
   `get_export_url`。以中文交付 URL、驗證資訊、有效期限、可用的 `curl` 範例、
   使用者工具的連線方式，以及單一 mview 和更新方式。取回既有 URL 不需重新
   確認發布；金鑰不得寫入儲存庫或共用進度紀錄。

## 必須遵守的界線

- Ophion 是唯一來源依據，逐欄查核與概念查核不可省略；保留來源證據。
- 搜尋未命中與知識清單沒有紀錄不同，明列缺口，不自行補造。
- 候選推導在最後同步確認中接受，不另加獨立關卡，也不當成既定規則。
- 原則上一份表單一個 mview，只在使用者明確要求時拆分。
- 只在同步及開 API 時確認，不在查找、SELECT 驗證、建立手動 mview、
  輪詢或取得既有 URL 時另加確認；宿主權限仍適用。
- `auth_type=none` 必須在開 API 確認時取得明確同意，交付再說明免驗證。
- 一次使用一個 workspace，檢查每筆回應標示；不符時用 `use_workspace` 切換。
- 不把 100 列樣本當完整結果，不以其他 SQL 方言替代 Trino。
