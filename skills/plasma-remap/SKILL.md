---
name: plasma-remap
description: >-
  Plasma 進階模式提示資料來源的 schema 已變更（`whoami` 或 `run_status` 的 `setup.remap_needed`／`remap`、查詢工具回覆的 `schema_changed`、Commit 頁的提示），或使用者要把既有概念重新對應到新的 schema 時使用：以 `run_mapping` 重新對應既有概念，不重新摘錄文件；完成前不能上傳或分析新文件。
---

# 重新對應既有概念

全程使用台灣繁體中文；工具名稱、欄位名稱、符號與網址保留原樣。
開始前先確認連線：依 [連結 Plasma 與診斷](../plasma-mcp-setup/SKILL.md) 完成授權，以 `whoami` 核對 workspace 與 `knowledge:write`。

**本 skill 只用於進階模式。** 核對 scopes 之前，先依 [查 Plasma 知識庫](../plasma-knowledge-query/SKILL.md) 判斷模式：工具清單沒有 `compile_query`、但有 `search_knowledge` 時，這個系統是一般模式（建立後不會改變）。這時告訴使用者：這個系統是一般模式，一般模式沒有「重新對應」這一步，不需要也無法在這裡執行，然後結束本 skill：不要求使用者重新授權（一般模式不提供 `knowledge:write`，重新授權也不會有）、不改用其他工具或自己寫 SQL 繞過。兩組工具都沒有時，依連結 Plasma 與診斷的「診斷」處理。

資料來源的 schema 變更後（新增或改了表、欄位），既有概念還對應在舊的 schema 上。
重新對應只重做概念與資料表、欄位之間的對應紀錄，概念、規則與文件的產物都不變；完成前，Plasma 不接受新文件的上傳與分析。

本 skill 只寫流程，以及和使用者之間的事。對應怎麼做、每種對應用哪個工具，讀 MCP：`read_task`（`target` 為 `task`）的任務協定，與每個工具自己的說明。

## 1. 確認需要重新對應

呼叫 `run_status`：

- `setup.remap_needed` 為 true 時，`setup.remap` 說明知識對應於哪一版、目前是哪一版。把這兩版告訴使用者，說明接下來要重新對應既有概念、不重新摘錄，期間不能上傳新文件。
- `setup.remap_needed` 不存在時，不需要重新對應：告訴使用者，不呼叫 `run_mapping`。
- 記下 `setup.commit_page`（使用者確認發布的頁面）。

## 2. 執行重新對應

重新對應是整個 workspace 的作業，一次處理所有文件的概念。

1. 呼叫 `run_mapping`，開始重新對應的作業。
2. 以 `run_status`（`wait_seconds` 15）等到 `task.ready`，或作業結束（`running` 為 false）。
3. `task.ready` 時，以 `read_task`（`target` 為 `task`）讀完這個任務與它的協定，一次呼叫讀一頁，讀到 `next` 等於 `total`；
   照協定以 `map_*` 與 `mapping_*` 工具完成對應，每個任務以 `mapping_finish` 結束。
   - 缺依據時，照協定記為無法對應並附理由，留給使用者或之後的文件補；欄位、值與換算係數以文件與 schema 寫明的為準。
   - 協定說要抽樣探測某一欄時，以 Plasma 的 `profile_column` 查（需要 `query:run`），作業結束後照協定交出結論：它以「資料推測」進 Commit 區；和文件說法衝突的結論列成 `probe-conflict:` 待補項目，由使用者決定（[補齊缺口](../plasma-fill-gaps/SKILL.md)）。
4. 重複第 2、3 步，直到作業結束。

碰到這些情況：

- **作業停下來了**（`running` 為 false，`error` 不為空）：已對應的部分保存著，schema 變更仍在等待，所有上傳都還被擋。把錯誤告訴使用者，然後照 `next_step` 再呼叫一次 `run_mapping`，從第 2 步接著做。同一個錯誤連續出現兩次時，停下來回報使用者。
- **`run_mapping` 說另一個作業在跑**：照回覆，以 `run_status` 等它結束或接它的任務；結束後再呼叫 `run_mapping`。

## 3. 確認並交給使用者發布

再呼叫一次 `run_status`：`setup.remap_needed` 不再出現，表示重新對應完成，新文件可以上傳。
仍為 true 時，作業沒有完整結束：照上一節「作業停下來了」再做一次；同一個錯誤連續兩次時，把 `message` 與 `error` 告訴使用者。

最後的回報固定包含以下四項，使用者另有交代（例如這輪先不上傳）時也一樣：

1. 結果：重新對應是否完成，知識現在對應於哪一版 schema，新文件是否可以上傳。
2. 數量：`run_status` 的 `message`，幾個實體已對應、部分對應或未對應，幾個欄位接上。
3. 待補：每個部分對應或未對應的概念與原因（例如唯一鍵未確認、找不到對應欄位；`pending_items` 列出它們），以及使用者補哪些資訊或文件就能補齊。
4. 發布：附上 Commit 頁（`setup.commit_page`），請使用者到該頁逐條勾選、預覽並確認發布；新的對應在發布後才用於報表。
