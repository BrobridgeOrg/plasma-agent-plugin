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

2. **Find the data with the ophion tools, not by guessing.** `overview` first,
   then `search_knowledge` for the concept, then `get_table_card` on the
   candidates. Use `plan_value_filter` / `get_value_domain` before writing any
   `WHERE` on a coded column — codes are rarely what they look like. Follow
   the `ophion-knowledge-lookup` skill for the method.

3. **Write the SQL.** Two-segment names (`"database"."table"`); the workspace
   is already the catalog. Never put a relation whose `access_mode` is
   `definition_required` or `blocked` into `FROM`/`JOIN`.

4. **Verify with `run_query` and show your work.** Present the SQL and the
   sample rows together and ask whether these are the numbers they meant.
   `run_query` needs their approval each time and is capped at 100 rows, so
   treat the result as a shape check, not a total.

5. **Create the materialized view** with `create_view`
   (`type=materialized_view`). Ask how fresh the data must be:
   - refreshed on a schedule → `sync_mode=scheduled` plus
     `scheduler_settings`;
   - rebuilt only when asked → `sync_mode=manual`, then `sync_view`.
   Then poll `get_view` until `last_sync_status=synced`. There is no API
   before a successful sync.

6. **Interview them about the API — do not choose these for them:**
   - **Authentication.** `api_key` (a key in `X-API-Key`), `basic_auth`
     (you must supply `secret_key` as `username:password`), or `none`.
     `none` means anyone with the URL reads this data; if they want it, say
     that plainly and get an explicit yes.
   - **Lifetime.** `expires_in` such as `30d` or `12h`. No value means the
     endpoint never expires — state which of the two you are creating.

7. **Publish and hand over.** `create_access_entry`, then `get_export_url` if
   you need the URL again. Give them:
   - the URL, the key, and when it expires;
   - a `curl` they can paste;
   - how to plug it into their tool (Power BI: *Get Data → Web → Advanced*,
     with the key as a request header);
   - which view backs it and how it refreshes, so they know why a number
     might be an hour old.

## Rules

- **Ophion's knowledge is not optional.** If the graph does not say where a
  metric comes from, ask the person — do not assemble plausible columns.
  "I could not find how this is derived" is a real answer.
- **No materialized view on unverified SQL.** Steps 4 and 5 are in that order
  every time.
- **`auth_type=none` needs an explicit yes**, and say so again on handover.
- **Never present a 100-row sample as the answer.** It is on Plasma's ceiling.
- One workspace at a time. Every tool answer names the workspace it used —
  if that is not the one you meant, switch with `use_workspace` rather than
  reinterpreting the result.
