---
name: plasma-data-api
description: >-
  當使用者想把表單、報表、BI 畫面或指定指標做成 Plasma 資料 API 時使用。以 Ophion 查核來源與定義，驗證 Trino SQL，原則上一份表單建立一個 mview；全程使用台灣繁體中文，只在開始同步拉資料及後續開啟 API 時確認，最後交付 URL、驗證方式與有效期限。
---

# From "I want an API" to a callable endpoint

## 共通互動原則

- 所有對使用者的回覆都使用**台灣繁體中文**，包含進度、問題、結果、錯誤說明、確認文字及交付說明。工具名稱、SQL、欄位名稱、URL 與需忠實引用的原文保留原樣，並以台灣繁體中文解釋。
- 在使用者已交付的任務範圍內，連續完成知識查找、欄位查核、SQL 驗證及不會啟動同步的 mview 建立；報告進度即可，不要每完成一步就問「是否繼續」。只有缺少會影響正確性的必要資訊時才釐清，釐清不等於每一步都要核准。
- 確認集中在兩個執行時點：**開始同步拉資料**，以及同步成功後**開啟資料 API**。每次以中文清楚說明具體影響；同一動作不要先在對話問一次、又重複要求一次工具確認。若宿主提供符合需求的確認介面，使用該介面；否則以中文取得明確同意後再呼叫工具。
- **原則上一份表單／報表建立一個 mview。** 不因不同區塊、指標、頁籤或來源表就拆成多個 mview；只有使用者明確要求拆分，才改變這個原則。

The person asking usually knows the numbers they want and nothing about the
schema. They should not have to. Your job is to find the data, prove the SQL
produces what they described, and hand back an endpoint.

全程以台灣繁體中文協作，依上述共通互動原則執行。

## The path

1. **Understand the metric.** What number, at what grain (per day? per
   patient? per store?), over what period, filtered how. A screenshot or form
   counts as the spec: read its labels, sections and filters, then explain your
   interpretation in Taiwan Traditional Chinese and continue. Ask only for
   missing information that materially changes the result; do not require
   approval of an otherwise clear reading.

   **一份表單就是一個交付單位，原則上只建立一個 mview。** 先列出整份表單的
   欄位、指標、篩選與資料粒度，再設計一份完整 SQL。多張來源表可用 CTE、
   JOIN、條件聚合或語意一致的 UNION ALL 組合；不要為了方便開發或對應每個
   表單區塊建立多個 mview，也不要額外建立中繼 mview。整合時必須保留正確
   粒度，不能用會重複計算的 JOIN 硬湊。若確實無法正確整合，說明具體限制並
   釐清需求，不得自行拆分或省略表單欄位。

2. **Find the data in Ophion — and nowhere else.** Every table and column
   that ends up in your SQL must be one you found through the ophion tools.
   If Ophion does not have it, you may not use it: not from a name that looks
   right, not from a label on their screenshot, not from how the same system
   looked at another site. Work outward: `overview` → `search_knowledge` (pass
   several synonyms; it ORs them) → `find_tables` → `get_table_card` →
   `list_columns`.

   Two things about the cards, both load-bearing:
   - **A card is an index, not the evidence.** `get_table_card` and
     `get_concept_card` hand you one line plus a `ku_id` per fact. Fetch every
     relevant `ku_id` with `get_knowledge_unit` before you build on it, and
     never quote a card summary as the rule.
   - **A miss is an answer.** `found=false` comes with `suggestions` — pivot on
     those rather than retrying the same string. When two or three different
     vocabularies all miss, treat it as "this workspace does not hold it" and
     go to step 4. Do not fill the hole yourself.

3. **Clear every column before it enters the SQL.** One call per column does
   it: `get_column_card` with `database.table.column` returns the column's
   type, nullability, best meaning, the source database's own comment, every
   knowledge unit anchored on it (by layer, with a per-`unit_type` count), the
   **value domains bound to it**, and its carrier relation's `access_mode` and
   `sql_name`.

   The wider views, when you want them:

   | Want | Call |
   |---|---|
   | Everything known about a table *and its columns*, by unit type | `list_units` with `subject=database.table` (add `exclude_columns=true` for the relation alone) |
   | One type's units for that subject | the same call plus `unit_type=…` — `antipattern_trap`, `data_quality_issue`, `validity_rule` are the ones that change a `WHERE` |
   | Whether the relation may be named in SQL at all | `get_table_card` → `access_mode`; `definition_required` and `blocked` never enter `FROM`/`JOIN` |
   | Whether anything is contested | `list_conflicts` — cards only count conflicts |

   Three things about reading these answers:

   - **A `unit_type` missing from a subject's inventory means the graph holds
     no such knowledge *for that subject*.** That is a real answer — say which
     you got: "no traps recorded for this column" is not "this column is
     safe".
   - **A bound value domain means the column is coded.** Its stored values are
     not the meanings they look like; resolve every literal through
     `search_value_candidates` → `plan_value_filter`, never by hand.
   - **Knowledge can be mounted on a concept instead of a table or column, and
     then the subject axis cannot see it.** In one real workspace a dozen
     `antipattern_trap` units were anchored to concepts. So `find_concepts` →
     `get_concept_card` for the metric's concepts is part of clearing a
     column, not an optional extra.

   Report every trap you find to the person, with its provenance, **before**
   you build on that column. A trap you found and did not mention becomes
   their wrong dashboard.

   You do not need an extra lookup to know which column a keyword hit belongs
   to: every `search_knowledge` hit carries `subject` (its precise anchor) and
   `about` (the relation page it was lifted to), so a hit feeds straight into
   `get_column_card`.

4. **When no single table answers it, climb this ladder in order.** Do not
   skip a rung, and do not jump to inventing SQL.

   1. **Look for a rule that already defines it.** Per table in play:
      `list_units` with `subject=<database.table>` and
      `unit_type=business_rule`, then the same for `validity_rule`,
      `state_machine`, `event_lifecycle`. Then `find_concepts` →
      `get_concept_card`, whose `governed_by` carries rules mounted on the
      concept *and* on its `SAME_AS` / `NORMALIZES_TO` siblings — a metric's
      definition often lives there rather than on any one table, and the
      subject axis cannot reach it. Keyword `search_knowledge` widens the net
      afterwards; it is never the only check. Expand every hit with
      `get_knowledge_unit`. If a rule exists, **that rule is the definition**
      — implement it as written and cite it. Do not improve on it.
   2. **No rule, but the pieces are there: compose a clearly labelled candidate.**
      Derive it only from the `table.column` you have actually cleared. Explain
      the columns, joins, filters and assumptions in Taiwan Traditional
      Chinese, and label the derivation as a proposal rather than an Ophion
      rule. Continue SQL validation without a separate approval round.
      Include the proposed definition and its validation result in the final
      sync confirmation; do not sync it until that definition is explicitly
      accepted there. If required facts are missing or competing meanings
      prevent a sound candidate, ask a focused clarification instead of guessing.
   3. **The pieces are not there: name the gap.** Say plainly:
      - what the metric needs that the workspace does not record;
      - which table or column would have to carry it;
      - what you searched (terms and `unit_types`) and what came back empty —
        so they can tell "Ophion has not learned this" from "the source system
        does not capture it".

      Then offer the real options: get the answer adjudicated into Ophion so
      it becomes knowledge, settle for a metric the data can support, or add
      the missing source data. **Never** close this rung by shipping a
      plausible-looking approximation.

5. **Write the SQL — Trino, and only Trino.** Plasma executes through Trino;
   there is no other dialect and no compatibility layer. SQL that would run in
   PostgreSQL, MySQL, SQL Server, BigQuery, Oracle or Spark and happens to
   resemble Trino is a defect, not a near miss.

   Identifiers: two-segment `database.table` — the workspace is already the
   catalog. Copy the name from Ophion's `sql_name` exactly and never
   reconstruct a source path. Ordinary snake_case names need no quotes;
   double-quote a *single* identifier only when it is a reserved word or holds
   odd characters, never a whole dotted path. Double quotes delimit
   identifiers, single quotes string literals — never one for the other.

   Carry step 3's traps into the SQL itself: a `WHERE` that excludes the
   known-bad rows beats a note in the chat, which nobody reads again once the
   view exists. Filters on coded columns use what `plan_value_filter`
   returned, not codes you typed.

   The Trino rules that actually bite, all of them banned in the right-hand
   column:

   | Use | Never |
   |---|---|
   | `CAST(x AS type)` | `x::type` |
   | `DATE '2026-01-31'`, `TIMESTAMP '2026-01-31 10:00:00'` | a quoted date string, which is VARCHAR |
   | `date_diff('day', a, b)` for elapsed units | `b - a` expecting a number (it yields INTERVAL) |
   | `date_add('day', 7, x)` or an `INTERVAL` literal | `DATEADD`, `DATE_SUB`, `x + 7` |
   | `CURRENT_DATE`, `CURRENT_TIMESTAMP` | `NOW()`, `GETDATE()`, `SYSDATE`, or those names in quotes |
   | `\|\|` or `concat()` | `+` for strings |
   | `IS NULL` / `IS NOT NULL` | `= NULL`, `<> NULL`, `ISNULL()`, `NVL()` |
   | `COALESCE` | `IFNULL`, `NVL` |
   | `approx_percentile(x, 0.5)` | `PERCENTILE_CONT`, `MEDIAN`, `APPROX_QUANTILE` |
   | `LOWER()` on both sides for case-insensitive matching | `ILIKE` |
   | `COUNT(DISTINCT (a, b))` | `COUNT(DISTINCT a, b)` |
   | `row_number() OVER (...)` in a CTE, filtered outside | `LIMIT` inside a per-group ranking, `TOP`, `ROWNUM` |
   | `LIMIT n` | `TOP n`, `FETCH FIRST`, `ROWNUM <= n` |
   | one statement, no semicolon | a trailing `;`, two statements, `SET` / `USE` / temp tables |

   Integer division truncates — cast an operand to `DOUBLE` when the metric is
   a rate or an average. Compare `DATE` with `TIMESTAMP` only with an explicit
   cast. Every non-aggregate expression in `SELECT` must appear in `GROUP BY`.

   If you are unsure whether a function exists in Trino, do not guess it into
   a materialized view: `run_query` it in step 6 first — a view built on a
   non-existent function fails on every sync, not on your screen.

6. **Verify with `run_query` and show your work.** Validate the complete form's
   SQL, report representative rows and data-quality findings in Taiwan
   Traditional Chinese, and continue without asking for per-query approval.
   `run_query` really executes a SELECT against the source through Trino; it
   is not a dry run. It returns at most 100 rows, so use it as a shape check,
   never as proof that the entire dataset has only that many rows. Compare
   the result against the requested form, grain and definitions yourself.
   Include any step 4.2 assumptions in the final sync summary.

7. **建立一個 mview，並在開始同步時確認。**

   預設以 `create_view(type=materialized_view, sync_mode=manual)` 建立整份表單的
   單一 mview 定義，不另外要求核准建立動作。建立定義不代表已同步完成。
   SQL 必須先通過步驟 6；已經存在同一份表單的 mview 時先查核並重用適合的
   物件，不為各區塊重複建立。

   在呼叫 `sync_view` 前，集中呈現 workspace、表單／mview 名稱、來源、完整
   表單的欄位涵蓋範圍、日期與篩選、SQL 驗證結果，以及尚待接受的推導假設。
   以台灣繁體中文取得這一次同步的確認，例如：

   > 即將同步「＜表單名稱＞」對應的「＜mview 名稱＞」。執行 sync 後，系統就會
   > 開始依照上述 SQL 從來源系統拉取資料，並寫入／更新這個 mview，會使用查詢
   > 與同步資源。這次同步尚不會開啟資料 API。是否確認開始同步？

   若有推導假設，將它們明列在同一份確認內容，讓使用者一併確認定義與同步。
   未取得明確同意就停在這個時點，不得呼叫同步、不得用其他工具繞過。

   **排程是相同確認時點的例外路徑。** 若使用者已要求排程，使用
   `create_view(sync_mode=scheduled, scheduler_settings=...)` 會立刻啟動首次
   同步，因此必須先完成上述確認，再呼叫 `create_view`；同時以中文說明首次
   拉資料會立即開始，以及後續自動拉資料的頻率。不要先建立 manual mview
   再另建一個 scheduled mview。排程參數若尚未指定，於這次同步確認一併釐清。

   取得確認後執行該次同步，持續用 `get_view` 查狀態，不逐次詢問。只有實際
   回報 `last_sync_status=synced` 才進入開 API 階段；失敗時說明原因，不能宣稱
   已完成。新的同步或重試若未包含在原確認範圍內，需要新的同步確認。

8. **同步成功後，一次確認開啟 API 的內容。** 將 API 設定整理成一份中文
   確認，不要逐欄訪談或先建立才補問：
   - 對應的表單、單一 mview 與要提供的資料範圍。
   - 驗證方式：`api_key`、`basic_auth` 或 `none`；沿用使用者已指定的選擇。
     未指定時可以提出 `api_key` 的建議，於這次確認取得同意後才採用。
     `basic_auth` 需要 `secret_key=username:password`。
   - 有效期限：明確列出 `expires_in`；不填代表不會自動到期。未指定時於同一
     份確認提出期限建議或詢問必要資訊，不能默默開成永久有效。

   確認文字須以台灣繁體中文說明，例如：

   > 「＜mview 名稱＞」已同步成功。下一步將開啟資料 API，讓可連線到此端點且
   > 通過＜驗證方式＞的呼叫者讀取上述資料；有效期限為＜期限＞。
   > 是否確認開啟 API？

   若 `auth_type=none`，必須改成明確說明「任何可連線到此端點且持有 URL 的人，
   不需驗證即可讀取資料」，並在同一次開 API 確認取得明確同意。
   同意同步不等於同意開 API。

9. **Publish and hand over.** Only after step 8's confirmation, call
   `create_access_entry`, then `get_export_url` if needed. Deliver in Taiwan
   Traditional Chinese: the URL, authentication details, expiry, a usable
   `curl` example, connection instructions for the user's tool, and the
   single backing mview with its refresh behavior. Retrieving an existing URL
   does not require another publication confirmation. Keep API keys out of
   repository files and shared progress logs.

## Rules

- **Ophion is the only admissible source.** A table or column that Ophion did
  not give you does not go into SQL, however obvious it looks.
- **Column clearance (step 3) is not optional.** `get_column_card` is one
  call; skipping it to save a call is how a query that runs returns the wrong
  number.
- **Concept-anchored knowledge needs the concept card.** The table/column axis
  cannot see it, so a clean column card is not a clean bill of health on its
  own.
- **"Search found nothing" and "the KB holds none of these" are different
  answers.** Only the inventory count tells them apart; say which one you got.
- **推導假設要在最後同步確認中取得明確同意。** 不增加步驟 4.2 的獨立確認關卡，
  也不能把未確認假設當成既定規則同步成正式資料。
- **A gap gets named, not filled.** "I could not find how this is derived, and
  here is what I searched" is a real answer; an invented composition is not.
- **No materialized view on unverified SQL.** Step 6 comes before step 7,
  every time.
- **`auth_type=none` 必須在開 API 的那次確認取得明確同意**，交付時再說明免驗證。
- **一份表單／報表原則上對應一個 mview。** 不因指標、區塊或來源表數量拆分，
  不用多個中繼 mview 代替一份完整結果；只有使用者明確要求才拆分。
- **只在同步與開 API 的執行時點要求操作確認。** 不在查找、欄位查核、SELECT
  驗證、建立 manual mview、輪詢狀態或取回既有 URL 時另加確認關卡。
  宿主平台另有權限要求時照其介面處理，不宣稱 skill 可以繞過平台限制。
- **Never present a 100-row sample as the answer.** It is on Plasma's ceiling.
- **Trino only.** No `::`, no `NOW()`, no `ILIKE`, no `NVL`, no `TOP` — see the
  table in step 5. Another dialect's syntax is a defect even when it parses.
- One workspace at a time. Every tool answer names the workspace it used —
  if that is not the one you meant, switch with `use_workspace` rather than
  reinterpreting the result.
