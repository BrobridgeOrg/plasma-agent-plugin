---
name: plasma-report
description: >-
  使用者要從 Plasma 進階模式知識庫出報表、算指標或列出符合條件的資料時使用（工具清單有 `compile_query`）：找到已發布的指標與概念，以 `compile_query` 由程式產生 Trino SQL，把說明與缺的資訊問使用者並補上，以 `run_query` 執行後交出結果與依據。
---

# 出報表

全程使用台灣繁體中文；工具名稱、欄位名稱、符號與 SQL 保留原樣。
開始前先確認連線：依 [連結 Plasma 與診斷](../plasma-mcp-setup/SKILL.md) 完成授權，以 `whoami` 核對 workspace，
並確認 scopes 含 `knowledge:read` 與 `query:run`。

報表的 SQL 全部由程式依已發布的知識產生：agent 把使用者的問題轉成「報表請求」，程式回 SQL；
知識沒說清楚的地方，SQL 會留下標記 `{{h…}}` 的洞，由使用者回答後 agent 補上。只用已發布的知識；Commit 區還沒發布的規則不會出現在 SQL 裡。

## 1. 找到指標與概念

1. 用 `lookup_knowledge`（`mode` 為 `search`）以使用者的用語找已發布的指標（measure）或分類（class）、屬性與實體；
   再用 `mode` 為 `definition` 讀它的定義，確認意思就是使用者要的。
2. 使用者的說法對到不只一個概念、或對不到時，把候選的中文名稱與定義列給使用者選，不自行決定。
3. 把問題拆成請求的各部分：
   - 要算的指標（`measure`），或要列出的分類或實體與附帶的屬性（`select`）；
   - 分組（`dimensions`）：依實體、依屬性（時間可加 `bucket`：day、week、month、quarter、year），或依一組分類（`classes`，其他歸入 `other`）；
   - 篩選（`filters`）：只計某分類，或某屬性比較某值；
   - 截點（`as_of`）與期間（`window`）；
   - 輸出欄名（`columns`：概念 → 欄名），使用者指定欄名時照用。

## 2. 編譯並請使用者核對

呼叫 `compile_query`。回覆中：

- `reading`：這份 SQL 算的是什麼，原樣給使用者看，請使用者確認就是他要的。
- `holes`：知識沒說的地方。每個洞有 `question`（要問使用者的話）、`reason`、`expects`（要補的是條件、值或幾欄的列）與 `gaps`（對應的知識庫缺口）。
- `basis`：依據的規則、對應與假設（例如「以封閉世界讀強否定」）；假設也告訴使用者。
- `release`：依據的發布版本。

回覆是請求錯誤時（概念不存在），照回覆列出的相近概念回到第 1 節。

## 3. 補洞

1. 把每個洞的 `question` 問使用者，一次問完；附上 `reading` 讓使用者知道問的是哪一部分。
2. 使用者回答後：
   - 以 `submit_answer` 保存使用者的原話（問題與回答），之後可以依 [補齊缺口](../plasma-fill-gaps/SKILL.md) 補進知識庫，下次就不再有這個洞；
   - 依回答把每個洞寫成一段 SQL：一個運算式，或用括號包起來的一個子查詢，只用 `expects` 列出的別名與欄位，或 schema 裡的表與欄位；
   - 以 `compile_query` 送出同一個請求並帶 `fills`（洞的 id → SQL 片段）。其餘部分由程式產生，照原樣保留。
3. 回覆的 `complete` 為 true 才執行；`validation.valid` 為 false 時依錯誤修改片段再送一次。
4. 使用者說不知道時，如實告訴使用者這份報表缺什麼資訊、目前無法算出，列出缺的項目；不自行假設數值或規則。

## 4. 執行並交出結果

1. 以 `run_query` 執行 `complete` 的 `sql`。
2. 交出結果時一併說明：算的是什麼（`reading`）、截點、依據的發布版本與規則、使用者補上的部分，以及 `basis` 中的假設。
3. 結果看起來不合理時，先回頭核對 `reading` 與補上的片段，再請使用者確認。

## 問題無法由知識庫回答時

使用者要的東西沒有對應的指標或概念、或需要知識庫沒有的資料時，說明無法精確回答，並列出需要補充的定義或資料來源；
使用者願意補充時，依 [文件分析](../plasma-document-analysis/SKILL.md) 上傳說明文件，或依補齊缺口 skill 回答。
