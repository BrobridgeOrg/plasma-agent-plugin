---
name: ophion-knowledge-lookup
description: >-
  撰寫 Plasma SQL 前，需要查明資料表、欄位、代碼、指標定義或來源系統設計時使用。以台灣繁體中文解讀 Ophion 的 access_mode、規則、來源證據與知識缺口，彙整整份表單所需知識，供建立 view／mview 使用。知識工具由 Plasma MCP server 一併提供。
---

# 查找與判讀 Ophion 知識

## 共通互動原則

- 所有進度、問題、結果、錯誤與交付均使用台灣繁體中文；系統功能名稱如 view、
  view、mview、workspace，以及工具名稱、SQL、欄位名稱與 URL 保留原樣。
- 在已交付範圍內連續完成知識查找、欄位查核及 SQL 驗證，不逐步詢問是否繼續。
  只釐清影響正確性的必要資訊，查到假設時標明，建立定義前確認必要假設。
- 以整份表單為查找單位，不因區塊、指標或來源表不同而自行拆分交付物。
  由 `plasma-create-pview` 建立 mview 運算層與 pview 篩選層；使用者明確指定 mview 時停在 mview。

Ophion 保存來源系統的設計知識：資料表與欄位意義、業務規則、代碼、推導、
品質陷阱及設計意圖，**不保存實際資料列**。特定紀錄的數值不能從設計知識猜測。

這些知識工具與 Plasma 的 view 工具**在同一台 MCP server 上**，
由 Plasma 的 MCP gateway 轉發過來，讀的是同一個 workspace —— 就是連結這個
連線時選定的那一個。沒有切換 workspace 的工具，`whoami` 會說明目前是哪一個。

彙整整份表單所有欄位／指標的來源、粒度、關聯與規則，供後續組成一份完整 SQL。
多個來源或概念不代表要建立多個 view／mview。

## 查找順序

1. `overview`：先掌握資料庫、主要概念與規模，再用這個 workspace 的詞彙搜尋。
2. `search_knowledge`：使用需求中的概念，閱讀回傳內容，不只看標題。
3. `find_tables`／`get_table_card`／`list_columns`：找到承載概念的資料表，
   查核卡片上的陷阱。卡片是索引，相關 `ku_id` 要以 `get_knowledge_unit` 展開。
4. `get_column_card`：逐欄查核型別、意義、空值、來源註解、知識與值域。
   概念層級的工具（`find_concepts`／`get_concept_card`）只在 `all`、`qa`、
   `fhir`、`audit` profile 下提供；預設的 `text-to-sql` 沒有它們，此時概念
   規則改由 `search_knowledge` 與 `get_knowledge_unit` 查核，不可省略。
5. `get_value_domain`／`search_value_candidates`／`plan_value_filter`：
   代碼欄位的篩選必須查明值域，不從欄位名稱或代碼外觀手寫條件。
6. `trace_lineage`：查看衍生數值依賴的來源。
7. `read_source`：需要原始措辭或定義時閱讀來源。

## 判讀結果

- **`access_mode` 決定能否在 SQL 引用來源。** `direct` 使用原樣 `sql_name`；
  `definition_required` 表示有宣告但尚未部署，先讀 `declaration_source_refs`，
  不可放進 FROM／JOIN；`blocked` 表示宣告與目錄有衝突，解決前不產生依賴它的 SQL。
- **資料血緣只代表依賴，不是完整運算式。** 不宣稱能單靠血緣重建定義。
- **每項事實保留證據**，包含來源位置與信心分數。將用於操作的答案需引用來源；
  `authority=user_qa` 是經人工裁定的答案，優先於一般來源。
- **缺漏也是查核結果。** 沒有欄位知識時明說來源未涵蓋，不從名稱補上猜測。
  搜尋未命中與知識清單沒有紀錄不同；某類陷阱未記錄不等於資料沒有風險。
- 查到陷阱時先向使用者說明影響與證據，再落實到 SQL；不逐張卡片要求核准。

完整的逐欄查核、概念規則、無單一來源時的推導順序與 Trino 驗證方式，
依 [知識查核與 SQL 驗證規則](../plasma-create-pview/references/knowledge-sql.md) 執行。
知識查核結果用於建立定義，沒有後續匯出或發布流程。

## 工具缺少或失敗

呼叫 `whoami`，它的 `Knowledge` 一行會說明是哪一種狀況：

- **未設定**：這個 Plasma 部署沒有接上知識服務，沒有知識工具可用。
  據實告訴使用者，不要改用猜測的欄位語意繼續。
- **沒有 `knowledge:read`**：這個連線沒有被授予知識權限。告訴使用者需要這個
  權限，依 `plasma-mcp-setup` 的追加授權流程處理；以 `whoami` 核對授權結果，
  不保證單純重連就能取得，不重試遭拒呼叫。
- **無法連線**：知識服務暫時不可用。Plasma 自己的 view 工具不受
  影響，但在知識恢復前不要憑推測寫 SQL。
- **`404`／`no published knowledge`**：這個 workspace 尚無已發布的知識版本，
  需先產生並發布，不是設定錯誤。
- workspace 不對：它是連結時選定的，無法從工具變更；要換請重新連結。
