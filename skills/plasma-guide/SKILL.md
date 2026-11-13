---
name: plasma-guide
description: >-
  使用者問 Plasma 或這個 plugin 能做什麼、有哪些功能、可以怎麼開始，或剛連上還不知道下一步時使用：核對連線與知識庫現況，依這個 workspace 的權限與內容列出現在能做的事，使用者選定後交給對應的技能。
---

# Plasma 能做什麼

全程使用台灣繁體中文；工具名稱、欄位名稱、符號、SQL 與 URL 保留原樣。
本技能只負責看現況、列出能做的事並帶到對應的技能；每件事的做法寫在各自的技能裡。

## 1. 看現況

1. 依 [連結 Plasma 與診斷](../plasma-mcp-setup/SKILL.md) 確認連線，以 `whoami` 讀 workspace、scopes 與 `Knowledge` 一行。
   `whoami` 提示 schema 已變更、需要重新對應時，先告訴使用者，這件事排在最前面。
   `Knowledge` 說未設定或無法連線時，依 [查 Plasma 知識庫](../plasma-knowledge-query/SKILL.md) 的「工具缺少或失敗」說明原因，知識相關的項目列為暫時不能做。
2. 工具清單有 `overview` 時呼叫它（不帶參數），讀這個 workspace 有哪些資料庫、各有幾張表；沒有 `overview` 時略過這一步。

## 2. 列出現在能做的事

這個系統的知識庫決定它有哪些功能。依工具清單只讀下列其中一頁，照那一頁列出能做的事：

| 工具清單有 | 讀這一頁 |
|---|---|
| `read_knowledge`、`compile_query` | [這個系統能做的事](references/advanced.md) |
| `search_knowledge`、`get_table_card` | [這個系統能做的事](references/standard.md) |

兩組都沒有時，只列 view 與 BI 的項目（[建立 pview](../plasma-create-view/SKILL.md)、[以 pview 產生 BI 報表](../plasma-create-bi/SKILL.md)），並說明知識工具目前不可用的原因。

對使用者只說這個系統有的功能：用使用者的業務用語各說一句，並以 `overview` 讀到的資料庫與表舉例；不舉知識庫沒有的規則或數字當例子。
只列這個連線的 scopes 做得到的；缺權限的項目放在最後，說明需要哪一個權限，要追加時依 [連結 Plasma 與診斷](../plasma-mcp-setup/SKILL.md)。

使用者問到會不會改到來源資料庫時據實說明：`run_query` 只接受唯讀查詢；建立 view、同步與知識的寫入都寫在 Plasma 與知識庫自己的儲存，來源資料庫只被讀取。

## 3. 交給對應的技能

使用者選定要做的事後，依該技能從頭進行，包括它自己的連線與權限核對。
使用者想做的事不在那一頁上時，說明這個系統目前沒有對應的功能，不以其他工具拼湊。
