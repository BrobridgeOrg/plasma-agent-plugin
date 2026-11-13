---
name: plasma-guide
description: >-
  使用者問 Plasma 或這個 plugin 能做什麼、有哪些功能、可以怎麼開始，或剛連上還不知道下一步時使用：核對連線與知識庫現況，依這個 workspace 的模式、權限與內容列出現在能做的事，使用者選定後交給對應的技能。
---

# Plasma 能做什麼

全程使用台灣繁體中文；工具名稱、欄位名稱、符號、SQL 與 URL 保留原樣。
本技能只負責看現況、列出能做的事並帶到對應的技能；每件事的做法寫在各自的技能裡。

## 1. 看現況

1. 依 [連結 Plasma 與診斷](../plasma-mcp-setup/SKILL.md) 確認連線，以 `whoami` 讀 workspace、scopes 與 `Knowledge` 一行。
   `whoami` 提示 schema 已變更、需要重新對應時，先告訴使用者，這件事排在最前面。
2. 依 [查 Plasma 知識庫](../plasma-knowledge-query/SKILL.md) 的「判斷知識庫的模式」判斷一般或進階模式。
   `Knowledge` 說未設定或無法連線時，依該技能的「工具缺少或失敗」說明原因，知識相關的項目列為暫時不能做。
3. 工具清單有 `overview` 時呼叫它（不帶參數）：讀這個 workspace 有哪些資料庫、各有幾張表；
   進階模式讀 `rules`（已發布的規則數）與 `next`，一般模式讀概念與知識的數量與 `hint`。`next` 與 `hint` 是知識服務建議的下一步，用來核對下表；要交給哪個技能以下表為準。
   工具清單沒有 `overview` 時略過這一步，只依 `whoami` 與模式列出能做的事。

## 2. 列出現在能做的事

用使用者的業務用語各說一句，並以 `overview` 讀到的資料庫與表舉例；不舉知識庫沒有的規則或數字當例子。
只列這個連線的 scopes 做得到的；缺權限的項目放在最後，說明需要哪一個權限，要追加時依 [連結 Plasma 與診斷](../plasma-mcp-setup/SKILL.md)。

| 想做的事 | 需要 | 技能 |
|---|---|---|
| 看有哪些資料庫與資料表，欄位的意義、單位、代碼與來源 | `knowledge:read` | [查 Plasma 知識庫](../plasma-knowledge-query/SKILL.md) |
| 查業務規則：某個名詞的定義、某個情況適用哪些規則、規則出自哪份文件 | `knowledge:read` | [查 Plasma 知識庫](../plasma-knowledge-query/SKILL.md) |
| 依規則取得資料：列出符合條件的資料、算指標、出一次性報表 | `knowledge:read`、`query:run` | 進階模式：[出報表](../plasma-report/SKILL.md)；一般模式：[建立 pview](../plasma-create-view/SKILL.md) |
| 做成可重複查詢、可帶參數的報表 | 再加 `views:write` | [建立 pview](../plasma-create-view/SKILL.md) |
| 做成 BI 報表（Power BI 專案或 HTML 頁面） | 已有 pview | [以 pview 產生 BI 報表](../plasma-create-bi/SKILL.md) |
| 把文件交給知識庫分析成規則 | 進階模式、`knowledge:write` | [文件分析](../plasma-document-analysis/SKILL.md) |
| 把程式碼、DDL 或資料庫物件的知識交給知識庫 | 進階模式、`knowledge:write` | [把程式碼讀成知識筆記](../plasma-code-notes/SKILL.md) |
| 回答知識庫提出的問題、補齊缺口 | 進階模式、`knowledge:write` | [補齊缺口](../plasma-fill-gaps/SKILL.md) |
| schema 變更後重新對應既有概念 | 進階模式、`knowledge:write` | [重新對應既有概念](../plasma-remap/SKILL.md) |

依現況調整順序與說法：

- 進階模式而 `rules` 為 0：知識庫目前只有 schema，還沒有規則。先建議上傳文件；查規則與依規則出報表要等文件分析完成、使用者在 Commit 頁發布之後。
- 進階模式而 `rules` 大於 0：先列查規則、依規則取得資料與報表，再列上傳更多文件。
- 一般模式：plugin 目前還不能上傳文件，文件請在 Plasma 介面上傳；列查知識、建立 pview 與 BI。
- 知識庫的內容與發布都由人在頁面上確認；agent 不發布。

使用者問到會不會改到來源資料庫時據實說明：`run_query` 只接受唯讀查詢；建立 view、同步與知識的寫入都寫在 Plasma 與知識庫自己的儲存，來源資料庫只被讀取。

## 3. 交給對應的技能

使用者選定要做的事後，依該技能從頭進行，包括它自己的連線、模式與權限核對。
使用者想做的事不在表上時，說明目前沒有對應的功能，不以其他工具拼湊。
