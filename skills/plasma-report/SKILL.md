---
name: plasma-report
description: >-
  使用者要從 Plasma 進階模式知識庫出報表、算指標或列出符合條件的資料時使用（工具清單有 `compile_query`）：找到已發布的指標與概念，以 `compile_query` 由程式產生 Plasma SQL，把說明與缺的資訊問使用者並補上，以 `run_query` 執行後交出結果與依據。
---

# 出報表

全程使用台灣繁體中文；工具名稱、欄位名稱、符號與 SQL 保留原樣。
開始前先確認連線：依 [連結 Plasma 與診斷](../plasma-mcp-setup/SKILL.md) 完成授權，以 `whoami` 核對 workspace，
並確認 scopes 含 `knowledge:read` 與 `query:run`。

**本 skill 只用於進階模式。** 核對 scopes 之前，先依 [查 Plasma 知識庫](../plasma-knowledge-query/SKILL.md) 判斷模式：工具清單沒有 `compile_query`、但有 `search_knowledge` 時，這個系統是一般模式（建立後不會改變）。這時告訴使用者：這個系統是一般模式，plugin 目前還沒有一般模式的一次性報表；要做成可重複查詢的報表，可以改依 [建立 pview 技能](../plasma-create-view/SKILL.md) 建立 pview，只想查欄位或規則時依查 Plasma 知識庫，然後結束本 skill：不要求使用者重新授權、不改用其他工具或自己寫 SQL 繞過。兩組工具都沒有時，依連結 Plasma 與診斷的「診斷」處理。

報表是一次性查詢：問題 → SQL → 執行 → 交出結果與依據。要建成可重複查詢、可帶參數的 pview 或 BI 時，改依 [建立 pview 技能](../plasma-create-view/SKILL.md)。

## 1. 取得 SQL

依 [進階模式的 sql-path](../plasma-knowledge-query/references/sql-path/advanced.md) 第 1 至 3 節：
以 `read_knowledge` 找到指標與概念、組成請求，`compile_query` 編譯，把 `reading` 給使用者核對，問使用者每個洞並補上，直到 `complete` 為 true。
一次性報表可以帶 `as_of` 與 `window`；使用者說民國年時換成西元期間並說明換算。

## 2. 執行並交出結果

1. 以 `run_query` 執行 `complete` 的 `sql`。
   使用者要的呈現方式編譯結果沒有提供時（例如年份以民國年顯示、日期格式、排序、四捨五入、欄名），照 sql-path 第 3 節在外面包一層，
   內層原封不動，外層做了什麼要在回報中說明。
2. 交出結果時一併說明：算的是什麼（`reading`）、截點、依據的發布版本與規則、使用者補上的部分，以及 `basis` 中的假設。
3. 結果看起來不合理時，先回頭核對 `reading` 與補上的片段，再請使用者確認。

## 問題無法由知識庫回答時

使用者要的東西沒有對應的指標或概念、或需要知識庫沒有的資料時，說明無法精確回答，並列出需要補充的定義或資料來源；
使用者願意補充時，依 [文件分析](../plasma-document-analysis/SKILL.md) 上傳說明文件，或依 [補齊缺口](../plasma-fill-gaps/SKILL.md) 回答。
