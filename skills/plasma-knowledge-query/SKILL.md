---
name: plasma-knowledge-query
description: >-
  查 Plasma 知識庫時使用：資料表、欄位、代碼、指標定義、業務規則與來源設計，某個業務概念存在哪裡、怎麼讀取、資料表之間怎麼關聯，或要看某個概念的 SQL；撰寫 Plasma SQL 前查核來源也用這個。依知識庫的模式（一般或進階）走對應的查法。
---

# 查 Plasma 知識庫

全程使用台灣繁體中文；工具名稱、欄位名稱、符號、SQL 與 URL 保留原樣。
開始前先確認連線：依 [連結 Plasma 與診斷](../plasma-mcp-setup/SKILL.md) 完成授權，以 `whoami` 核對 workspace 與 `knowledge:read`。

Plasma 知識庫保存來源系統的設計知識，**不保存實際資料列**；特定紀錄的數值不能從設計知識猜測。
只回答知識庫寫了的；沒寫的照實說沒有，列為缺口，不從欄位名稱補上猜測。

## 判斷知識庫的模式

一個 workspace 的知識庫是一般模式或進階模式，建立後不會改變；兩種模式保存知識的方式不同，提供的工具也完全不同。
以工具清單判斷，同一個工作只判斷一次：

| 工具清單有 | 模式 | 查法 |
|---|---|---|
| `read_knowledge`、`compile_query` | 進階模式 | [進階模式的查法](references/advanced.md) |
| `search_knowledge`、`get_table_card` | 一般模式 | [一般模式的查法](references/standard.md) |

兩組都沒有時，依下節「工具缺少或失敗」判斷原因，不憑推測繼續。
不混用兩種模式的工具，也不以另一種模式的做法代替。

## 工具缺少或失敗

呼叫 `whoami`，它的 `Knowledge` 一行會說明是哪一種狀況：

- **未設定**：這個 Plasma 部署沒有接上知識服務，沒有知識工具可用。
  據實告訴使用者，不要改用猜測的欄位語意繼續。
- **沒有 `knowledge:read`**：這個連線沒有被授予知識權限。告訴使用者需要這個
  權限，依 `plasma-mcp-setup` 的追加授權流程處理；以 `whoami` 核對授權結果，
  不保證單純重連就能取得，不重試遭拒呼叫。
- **無法連線**：知識服務暫時不可用。Plasma 自己的 view 工具不受
  影響，但在知識恢復前不要憑推測寫 SQL。
- **`404`／`no published knowledge`**：這個 workspace 尚無已發布的知識版本，不是設定錯誤。
  一般模式由管理員產生並發布；進階模式需先上傳文件分析（[文件分析](../plasma-document-analysis/SKILL.md)），再由使用者在 Commit 頁發布。
- workspace 不對：它是連結時選定的，無法從工具變更；要換請重新連結。

## 從知識取得 SQL（sql-path）

其他技能需要「一份依知識寫成、驗證過的 SQL」時，依模式照下列說明取得：

- 一般模式：[查核知識後撰寫並驗證 SQL](references/sql-path/standard.md)
- 進階模式：[由 `compile_query` 編譯並補洞](references/sql-path/advanced.md)

兩種模式都交出同樣的結果，供呼叫的技能接著使用：

1. **SQL**：Plasma SQL 單一陳述式，已驗證可執行（寫法見 [Plasma SQL 寫法](references/sql-path/plasma-sql.md)）。
2. **說明**：這份 SQL 算的是什麼，以使用者的業務用語寫。
3. **依據**：引用的知識（一般模式的 `ku_id`、來源與信心；進階模式的 `basis` 與 `release`），以及使用者補上的部分。
4. **假設**：已向使用者說明並取得接受的假設。
5. **缺口**：知識沒說、使用者也無法回答的部分；有缺口時不交出看似完整的 SQL。

代碼缺少名稱、翻譯或描述時，兩種模式都依 [代碼名稱的候選補充](references/code-names.md)。

使用者只是想看某個概念怎麼查時，也依同一份說明取得 SQL，把說明、依據與缺口一起給使用者。
