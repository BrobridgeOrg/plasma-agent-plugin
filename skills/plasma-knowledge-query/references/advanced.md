# 進階模式：查儲存方式與資料關聯

工具清單有 `read_knowledge`、`compile_query` 時，知識庫是進階模式，依本文查找。
進階模式的知識是概念、規則與它們對應到的資料表、欄位；`read_knowledge` 只讀已發布的知識，Commit 區還沒發布的不會出現。

只回答已發布的知識；知識庫沒有寫的，照實說沒有，並列為待補。

## 0. 先看整體：有哪些資料庫與資料表

使用者問這個 workspace 有哪些資料庫、幾張表、有哪些表，或剛開始還不知道從哪裡查時，用 `overview`：

- 不帶參數：每個資料庫的表數（`tables`）與三個表名（`has_more` 為 true 表示還有其他表）、已發布的規則數（`rules`），以及 `next` 列出的下一步。
- 帶 `database`：那個資料庫全部的表名；要看某張表的欄位再用 `read_schema`。

知識庫的資料庫與表以 `overview` 與 `read_schema` 讀到的已發布 schema 為準；回答時說明這是已發布的版本（`release`）。
工具清單沒有 `overview` 時（知識服務的版本較舊），以 `read_schema`（`mode` 為 `search`）用使用者的用語找表，並告訴使用者目前列不出完整清單。

## 1. 找到概念

用 `read_knowledge`（`mode` 為 `search`）以使用者的用語找概念或實體，再以 `mode` 為 `definition` 讀它：
定義、對應的欄位與表（橋接）、完整與否（`completeness`）與缺口。對到多個候選時列給使用者選。
任何工具的回覆說內容未完（附 `cursor`、`has_more` 為 true）時，先以 `read_tool_reply` 帶那個 `cursor` 讀到 `has_more` 為 false，再呼叫其他工具。

## 2. 回答存放與讀取方式

對每個概念說明：

- **存在哪裡**：對應的資料表與欄位（`db.table.column`）；實體對應的表與識別鍵；一個實體有多個來源表時逐一列出。
- **怎麼讀**：實體層附註，包括單位與倍率、時間基準（UTC、當地時間及位移欄位、日曆日期）、讀取格式（文字日期、小數逗號、空值標記、哨兵值、去空白轉大寫）、
  值的意義（代碼 → 意思），以及版本語意（依查詢截點每個鍵取哪一版：得知時間欄位 `known_at`、刪除旗標、截點當時有效的期間）。依每列日期挑另一張表當時有效的列，是規則裡的區間條件，不屬於版本語意。
- **從表或欄位問起時**（使用者說的是表名、欄位名，不是業務概念）：用 `read_schema`（`mode` 為 `table`，`table` 為 `db.table`）。使用者只說表名時，先以 `overview` 或 `read_schema`（`mode` 為 `search`）找到它的 `db.table`。回覆有這張表自己的附註（`notes`）、計算欄與它的公式（`calculated`），每個欄位的 `notes` 是它有幾則附註；
  再帶 `column` 讀單一欄位：完整的描述、每一則附註（`kind` 是種類，例如 `unit`、`time_basis`、`read_format`、`value_meaning`、`source_system`、`description`；`content` 是內容）與說這件事的文件（`sources`）。沒有那一種附註時，照實說知識庫沒有寫。
  同一份回覆的 `concepts` 是對應到這個欄位的已發布概念（`name` 與它的用語 `term`）；使用者問哪些規則用到這個欄位、或有什麼是從它算出來的，以 `read_knowledge`（`mode` 為 `usage`）帶這些 `name` 查讀到它的規則，連同同一張表的計算欄一起回答。`column` 給計算欄的名稱時，回它的公式與出處，也就是這個值怎麼從同一張表的欄位算出來。
- **怎麼關聯**：用 `read_knowledge`（`mode` 為 `dependencies`）看概念讀到哪些概念與關聯，說明連接的欄位與基數（一對一、多對一、一對多），以及基數是否經資料驗證。
- **還缺什麼**：沒有欄位對應的葉子概念、單位未知、基數未知等缺口，附上缺口的問句，交給 [補齊缺口](../../plasma-fill-gaps/SKILL.md) 處理。

Plasma 上建立的 view 與 mview，進階模式會自動把它的 SQL 轉成知識（view 的欄位與來源欄位的關係），不經 Commit 頁；它們也能以同樣方式查到。

## 3. 看某個概念的 SQL

使用者要看實際怎麼查時，依 [進階模式的 sql-path](sql-path/advanced.md) 編譯：
`select` 請求（`of` 為分類或實體，`attributes` 為要看的屬性），或指標的 `measure` 請求。

- 把 `reading` 與 `basis`（依據的規則、對應與假設）給使用者，`complete` 為 true 時連同 `sql`；
- `complete` 為 false 時，說明每個洞的 `question`：那是知識庫還沒說清楚的地方；
- 要執行時依 [出報表](../../plasma-report/SKILL.md) 補洞並以 `run_query` 執行。
