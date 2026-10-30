# Plasma SQL 寫法

Plasma SQL 採 Trino SQL 語法：以 ANSI SQL 為基礎，加上 Trino 的函式與寫法。
agent 寫的 SQL（一般模式的整份 SQL、進階模式補洞的片段與外層呈現）都照這裡寫。

識別名稱採兩段式 `database.table`，workspace 已提供 catalog 範圍。一般 snake_case 名稱不需
引號；保留字或特殊字元用雙引號包住單一識別名稱，不包住整段含點號路徑。
字串使用單引號。

| 使用方式 | 避免方式 |
|---|---|
| `CAST(x AS type)` | `x::type` |
| `DATE '2026-01-31'`、`TIMESTAMP '2026-01-31 10:00:00'` | 以一般 VARCHAR 字串當日期 |
| `date_diff('day', a, b)` 計算經過單位 | 將 `b - a` 的 INTERVAL 當數字 |
| `date_add('day', 7, x)` 或 INTERVAL 常值 | `DATEADD`、`DATE_SUB`、`x + 7` |
| `CURRENT_DATE`、`CURRENT_TIMESTAMP` | `NOW()`、`GETDATE()`、`SYSDATE` 或加引號的名稱 |
| `\|\|` 或 `concat()` | 用 `+` 串接字串 |
| `IS NULL`、`IS NOT NULL` | `= NULL`、`<> NULL`、`ISNULL()`、`NVL()` |
| `COALESCE` | `IFNULL`、`NVL` |
| 精確中位數用下方的 `row_number()` 寫法；可接受近似值時才用 `approx_percentile(x, 0.5)` | `PERCENTILE_CONT … WITHIN GROUP`、`MEDIAN`、`APPROX_QUANTILE`（不支援） |
| 兩側用 `LOWER()` 不分大小寫比對 | `ILIKE` |
| `COUNT(DISTINCT (a, b))` | `COUNT(DISTINCT a, b)` |
| CTE 中用 `row_number() OVER (...)`，外層篩選 | 用 LIMIT 代替組內排名、TOP、ROWNUM |
| `LIMIT n` | `TOP n`、`FETCH FIRST`、`ROWNUM <= n` |
| 單一陳述式，不附分號 | 尾端分號、多個陳述式、SET／USE／暫存資料表 |

整數除法會截斷小數；比率或平均值將運算元轉為 DOUBLE。小數常值（例如 `86400.0`）是 DECIMAL，
結果的小數位數由常值決定（`1234567 / 86400.0` 得 `14.3`），要完整小數時同樣先 `CAST(… AS DOUBLE)`。DATE 與 TIMESTAMP
比較時明確轉型。SELECT 的非聚合運算式都需列入 GROUP BY。
不確定函式是否支援時，先用 `run_query` 驗證，不把猜測放入正式定義。

`approx_percentile` 是近似值，資料少時常回傳某一列的值而不是中位數（兩筆 100、250.5 得 250.5，正確是 175.25）。
報表的中位數是指標定義，預設算精確值：

```sql
WITH r AS (
  SELECT g, x,
         row_number() OVER (PARTITION BY g ORDER BY x) AS rn,
         count(*) OVER (PARTITION BY g) AS n
  FROM t
  WHERE x IS NOT NULL
)
SELECT g, avg(CAST(x AS DOUBLE)) AS median_x
FROM r
WHERE rn IN ((n + 1) / 2, (n + 2) / 2)
GROUP BY g
```
