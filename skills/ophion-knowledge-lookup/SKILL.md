---
name: ophion-knowledge-lookup
description: >-
  撰寫 Plasma SQL 前，需要查明資料表、欄位、代碼、指標定義或來源系統設計時使用。以台灣繁體中文解讀 Ophion 的 access_mode、規則、來源證據與知識缺口，彙整整份表單所需知識，支援一份表單建立一個 mview。
---

# Reading Ophion's knowledge

## 共通互動原則

- 所有對使用者的回覆都使用**台灣繁體中文**，包含進度、問題、結果、錯誤說明、確認文字及交付說明。工具名稱、SQL、欄位名稱、URL 與需忠實引用的原文保留原樣，並以台灣繁體中文解釋。
- 在使用者已交付的任務範圍內，連續完成知識查找、欄位查核、SQL 驗證及不會啟動同步的 mview 建立；報告進度即可，不要每完成一步就問「是否繼續」。只有缺少會影響正確性的必要資訊時才釐清，釐清不等於每一步都要核准。
- 確認集中在兩個執行時點：**開始同步拉資料**，以及同步成功後**開啟資料 API**。每次以中文清楚說明具體影響；同一動作不要先在對話問一次、又重複要求一次工具確認。若宿主提供符合需求的確認介面，使用該介面；否則以中文取得明確同意後再呼叫工具。
- **原則上一份表單／報表建立一個 mview。** 不因不同區塊、指標、頁籤或來源表就拆成多個 mview；只有使用者明確要求拆分，才改變這個原則。

Ophion holds conclusions about **how a data system was designed** — table and
column meaning, business rules, value meanings, derivations, quality traps,
design intent. It holds **no rows**. Any question about a specific record's
value cannot be answered here; say so instead of inferring one.

The tools always answer for the workspace currently selected in the plasma
server, and every answer states it. Check that line.

以使用者交付的整份表單為查找範圍，整理所有欄位／指標的來源、粒度、關聯與
規則，交給 `plasma-data-api` 組成一份 SQL、一個 mview。查到多張來源表或不同
概念，不代表要拆成多個 mview。知識不足時明列缺口；候選推導必須標成假設，
可先驗證並納入最後同步確認，不要每讀一張卡片或發現一條規則就要求核准。

## Order of operations

1. **`overview`** — the databases, the main concepts, the scale. Do this
   before searching, so you know what vocabulary this workspace uses.
2. **`search_knowledge`** — the concept in the user's words. Read the returned
   units, not just their titles.
3. **`find_tables` / `get_table_card` / `list_columns`** — narrow to the
   relations that carry it. The table card is where the traps live.
4. **`get_value_domain` / `search_value_candidates` / `plan_value_filter`** —
   any time a filter touches a coded column. Do not hand-write a code
   predicate from a column name.
5. **`trace_lineage`** — when the number is derived, to see what feeds it.
6. **`read_source`** — when you need the original wording behind a claim.

## Judging what comes back

- **`access_mode` decides whether you may name a relation in SQL.**
  `direct` — use `sql_name`. `definition_required` — the workspace declares it
  but it is not deployed: never put it in `FROM`/`JOIN`; read
  `declaration_source_refs` for the definition. `blocked` — a declared-vs-
  catalog conflict is open; generate no SQL until it is resolved.
- **Lineage is dependencies, not the expression.** It tells you what a
  relation reads, never the full SQL. Do not claim you can reconstruct a
  definition from lineage alone.
- **Provenance travels with every fact** (`file:line`, a confidence score).
  Cite it when the answer will be acted on. `authority=user_qa` marks a
  human-adjudicated answer and outranks ordinary source material.
- **Absence is a finding.** If the graph has nothing on a column, report that
  the source material does not cover it. Do not fill the gap from the column's
  name, and do not present a guess as knowledge.

## When knowledge tools are missing or failing

Call `ophion_context`. It reports the workspace being read and whether Ophion
answers. Three failures mean different things:

- **no workspace selected** → use the plasma server's `use_workspace`;
- **404 / no published knowledge** → this workspace has no generation
  published yet; there is nothing to read, which is not a configuration error;
- **unreachable** → Ophion's query-mcp is a cluster-internal API; a
  workstation usually needs a port-forward. See `plasma-plugin-setup`.
