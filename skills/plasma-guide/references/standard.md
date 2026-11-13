# 這個系統能做的事

`overview` 回報資料庫、主要概念與知識的數量，`hint` 是知識服務建議的查法，用來核對下表；要交給哪個技能以下表為準。

| 想做的事 | 需要 | 技能 |
|---|---|---|
| 看有哪些資料庫與資料表，欄位的意義、值域、代碼與來源 | `knowledge:read` | [查 Plasma 知識庫](../../plasma-knowledge-query/SKILL.md) |
| 查業務規則與指標定義、某個概念存在哪裡、資料從哪裡來 | `knowledge:read` | [查 Plasma 知識庫](../../plasma-knowledge-query/SKILL.md) |
| 依知識取得資料：寫出並驗證 SQL，做成可重複查詢、可帶參數的報表 | `knowledge:read`、`query:run`、`views:write` | [建立 pview](../../plasma-create-view/SKILL.md) |
| 做成 BI 報表（Power BI 專案或 HTML 頁面） | 已有 pview | [以 pview 產生 BI 報表](../../plasma-create-bi/SKILL.md) |

先列查知識，再列建立報表與 BI。
