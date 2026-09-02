---
name: ophion-knowledge-lookup
description: Use when you need to know what a table, column, code or metric in a Plasma workspace actually means, where a number comes from, or how the source system was designed — before writing SQL against it. Also use when reading Ophion's answers: how to judge access_mode, cite provenance, and tell "the graph does not know this" from "this does not exist".
---

# Reading Ophion's knowledge

Ophion holds conclusions about **how a data system was designed** — table and
column meaning, business rules, value meanings, derivations, quality traps,
design intent. It holds **no rows**. Any question about a specific record's
value cannot be answered here; say so instead of inferring one.

The tools always answer for the workspace currently selected in the plasma
server, and every answer states it. Check that line.

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
