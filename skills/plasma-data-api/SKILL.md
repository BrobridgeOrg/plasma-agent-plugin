---
name: plasma-data-api
description: Use when someone wants data out of Plasma as a callable API or endpoint — "I want an API for these numbers", "我想要這些資料的 API", "expose this metric to Power BI / Tableau / our app", "give me a URL for this report", or when they hand you a BI screenshot and ask where the data comes from. Covers the whole path: find the source with Ophion, verify SQL, build the materialized view, interview them for the API's auth and lifetime, publish it, and hand over the URL and key.
---

# From "I want an API" to a callable endpoint

The person asking usually knows the numbers they want and nothing about the
schema. They should not have to. Your job is to find the data, prove the SQL
produces what they described, and hand back an endpoint.

Talk to them in whatever language they used.

## The path

1. **Understand the metric.** What number, at what grain (per day? per
   patient? per store?), over what period, filtered how. Ask one question at a
   time. A screenshot counts as the spec — read the labels, axis titles and
   filters off it and confirm your reading back to them.

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
   2. **No rule, but the pieces are there: compose, then ask.** Derive it from
      the `table.column` you have actually cleared, then state the derivation
      to the person in one sentence — which columns, which join, which
      filter, which assumption — and **ask whether that is what they mean**.
      This question is not a courtesy: without a rule in the graph, your
      derivation is a hypothesis, and only they can confirm it. Wait for the
      answer. If they correct you, redo the derivation and ask again.
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

6. **Verify with `run_query` and show your work.** Present the SQL and the
   sample rows together and ask whether these are the numbers they meant.
   `run_query` needs their approval each time and is capped at 100 rows, so
   treat the result as a shape check, not a total. If step 4.2 applied, this
   is also where the derivation gets its second look — the numbers either
   match what they described or they do not.

7. **Create the materialized view** with `create_view`
   (`type=materialized_view`). Ask how fresh the data must be:
   - refreshed on a schedule → `sync_mode=scheduled` plus
     `scheduler_settings`;
   - rebuilt only when asked → `sync_mode=manual`, then `sync_view`.
   Then poll `get_view` until `last_sync_status=synced`. There is no API
   before a successful sync.

8. **Interview them about the API — do not choose these for them:**
   - **Authentication.** `api_key` (a key in `X-API-Key`), `basic_auth`
     (you must supply `secret_key` as `username:password`), or `none`.
     `none` means anyone with the URL reads this data; if they want it, say
     that plainly and get an explicit yes.
   - **Lifetime.** `expires_in` such as `30d` or `12h`. No value means the
     endpoint never expires — state which of the two you are creating.

9. **Publish and hand over.** `create_access_entry`, then `get_export_url` if
   you need the URL again. Give them:
   - the URL, the key, and when it expires;
   - a `curl` they can paste;
   - how to plug it into their tool (Power BI: *Get Data → Web → Advanced*,
     with the key as a request header);
   - which view backs it and how it refreshes, so they know why a number
     might be an hour old.

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
- **The confirmation in 4.2 is mandatory.** An unconfirmed derivation is never
  a basis for a materialized view.
- **A gap gets named, not filled.** "I could not find how this is derived, and
  here is what I searched" is a real answer; an invented composition is not.
- **No materialized view on unverified SQL.** Step 6 comes before step 7,
  every time.
- **`auth_type=none` needs an explicit yes**, and say so again on handover.
- **Never present a 100-row sample as the answer.** It is on Plasma's ceiling.
- **Trino only.** No `::`, no `NOW()`, no `ILIKE`, no `NVL`, no `TOP` — see the
  table in step 5. Another dialect's syntax is a defect even when it parses.
- One workspace at a time. Every tool answer names the workspace it used —
  if that is not the one you meant, switch with `use_workspace` rather than
  reinterpreting the result.
