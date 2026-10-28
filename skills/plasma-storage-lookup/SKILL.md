---
name: plasma-storage-lookup
description: >-
  IT 人員要知道 Plasma 進階模式知識庫裡某個業務概念實際存在哪裡、怎麼讀取、資料表之間怎麼關聯，或要看某個概念的 SQL 時使用（工具清單有 `lookup_knowledge` 與 `compile_query`）：查對應、實體層附註與關聯，必要時以 `compile_query` 產生該概念的 SQL。
---

# 查儲存方式與資料關聯

全程使用台灣繁體中文；工具名稱、欄位名稱、符號與 SQL 保留原樣。
開始前先確認連線：依 [連結 Plasma 與診斷](../plasma-mcp-setup/SKILL.md) 完成授權，以 `whoami` 核對 workspace 與 `knowledge:read`。

只回答已發布的知識；知識庫沒有寫的，照實說沒有，並列為待補。

## 1. 找到概念

用 `lookup_knowledge`（`mode` 為 `search`）以使用者的用語找概念或實體，再以 `mode` 為 `definition` 讀它：
定義、對應的欄位與表（橋接）、完整與否（`completeness`）與缺口。對到多個候選時列給使用者選。

## 2. 回答存放與讀取方式

對每個概念說明：

- **存在哪裡**：對應的資料表與欄位（`db.table.column`）；實體對應的表與識別鍵；一個實體有多個來源表時逐一列出。
- **怎麼讀**：實體層附註，包括單位與倍率、時間基準（UTC、當地時間及位移欄位、日曆日期）、讀取格式（文字日期、小數逗號、空值標記、哨兵值、去空白轉大寫）、
  值的意義（代碼 → 意思），以及版本語意（每個鍵取哪一版、得知時間欄位 `known_at`、刪除旗標、有效期間）。
- **怎麼關聯**：用 `lookup_knowledge`（`mode` 為 `dependencies`）看概念讀到哪些概念與關聯，說明連接的欄位與基數（一對一、多對一、一對多），以及基數是否經資料驗證。
- **還缺什麼**：沒有欄位對應的葉子概念、單位未知、基數未知等缺口，附上缺口的問句，交給 [補齊缺口](../plasma-fill-gaps/SKILL.md) 處理。

## 3. 看某個概念的 SQL

使用者要看實際怎麼查時，以 `compile_query` 送出 `select` 請求（`of` 為分類或實體，`attributes` 為要看的屬性），或指標的 `measure` 請求：

- 把 `sql`、`reading` 與 `basis`（依據的規則、對應與假設）給使用者；
- 有 `{{h…}}` 的洞時，說明每個洞的 `question`：那是知識庫還沒說清楚的地方；
- 要執行時依 [出報表](../plasma-report/SKILL.md) 第 3、4 節補洞並以 `run_query` 執行。
