# 這個系統能做的事

`overview` 的 `rules` 是已發布的規則數，`next` 是知識服務建議的下一步，用來核對下表；要交給哪個技能以下表為準。

| 想做的事 | 需要 | 技能 |
|---|---|---|
| 看有哪些資料庫與資料表，欄位的意義、單位、代碼與來源 | `knowledge:read` | [查 Plasma 知識庫](../../plasma-knowledge-query/SKILL.md) |
| 查業務規則：某個名詞的定義、某個情況適用哪些規則、規則出自哪份文件 | `knowledge:read` | [查 Plasma 知識庫](../../plasma-knowledge-query/SKILL.md) |
| 依規則取得資料：列出符合條件的資料、算指標、出一次性報表 | `knowledge:read`、`query:run` | [出報表](../../plasma-report/SKILL.md) |
| 做成可重複查詢、可帶參數的報表 | 再加 `views:write` | [建立 pview](../../plasma-create-view/SKILL.md) |
| 做成 BI 報表（Power BI 專案或 HTML 頁面） | 已有 pview | [以 pview 產生 BI 報表](../../plasma-create-bi/SKILL.md) |
| 把文件交給知識庫分析成規則 | `knowledge:write` | [文件分析](../../plasma-document-analysis/SKILL.md) |
| 把程式碼、DDL 或資料庫物件的知識交給知識庫 | `knowledge:write` | [把程式碼讀成知識筆記](../../plasma-code-notes/SKILL.md) |
| 回答知識庫提出的問題、補齊缺口 | `knowledge:write` | [補齊缺口](../../plasma-fill-gaps/SKILL.md) |
| schema 變更後重新對應既有概念 | `knowledge:write` | [重新對應既有概念](../../plasma-remap/SKILL.md) |

依現況調整順序與說法：

- `rules` 為 0：知識庫目前只有 schema，還沒有規則。先建議上傳文件；查規則與依規則出報表要等文件分析完成、使用者在 Commit 頁發布之後。
- `rules` 大於 0：先列查規則、依規則取得資料與報表，再列上傳更多文件。
- 知識的內容由使用者在 Commit 頁確認後才發布；agent 不發布。
