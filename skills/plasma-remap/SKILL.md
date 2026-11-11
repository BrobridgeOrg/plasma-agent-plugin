---
name: plasma-remap
description: >-
  Plasma 進階模式提示資料來源的 schema 已變更（`whoami` 或 `run_status` 的 `setup.remap_needed`／`remap`、Commit 頁的提示），或使用者要把既有概念重新對應到新的 schema 時使用：以 `run_mapping` 重新對應既有概念，不重新摘錄文件；完成前不能上傳或分析新文件。
---

# 重新對應既有概念

全程使用台灣繁體中文；工具名稱、欄位名稱、符號與網址保留原樣。
開始前先確認連線：依 [連結 Plasma 與診斷](../plasma-mcp-setup/SKILL.md) 完成授權，以 `whoami` 核對 workspace 與 `knowledge:write`。

**本 skill 只用於進階模式。** 核對 scopes 之前，先依 [查 Plasma 知識庫](../plasma-knowledge-query/SKILL.md) 判斷模式：工具清單沒有 `compile_query`、但有 `search_knowledge` 時，這個系統是一般模式（建立後不會改變）。這時告訴使用者：這個系統是一般模式，一般模式沒有「重新對應」這一步，不需要也無法在這裡執行，然後結束本 skill：不要求使用者重新授權（一般模式不提供 `knowledge:write`，重新授權也不會有）、不改用其他工具或自己寫 SQL 繞過。兩組工具都沒有時，依連結 Plasma 與診斷的「診斷」處理。

資料來源的 schema 變更後（新增或改了表、欄位），既有概念還對應在舊的 schema 上。
重新對應只重做概念與資料表、欄位之間的對應，概念、規則與文件摘錄都不變；完成前，Plasma 不接受新文件的上傳與分析。

## 1. 確認需要重新對應

以 `work_id` `workspace` 呼叫 `run_status`：

- `setup.remap_needed` 為 true 時，`setup.remap` 說明知識對應於哪一版、目前是哪一版。把這兩版告訴使用者，說明接下來要重新對應既有概念、不重新摘錄，期間不能上傳新文件。
- `setup.remap_needed` 不存在時，不需要重新對應：告訴使用者，不呼叫 `run_mapping`。
- 記下 `setup.commit_page`（使用者確認發布的頁面）。

## 2. 執行重新對應

1. 呼叫 `run_mapping`，開始重新對應的作業。
2. 以 `run_status`（`wait_seconds` 15）等到 `task.ready`，或作業結束（`running` 為 false）。
   重新對應是整個 workspace 的作業，不屬於任何一份文件：`run_status`、`read_task`（`target` 為 `task`）與 `mapping_*` 一律帶 `work_id` `workspace`，即使這個連線（或子代理）還開著文件、文件自己的對應任務也在等待。
   作業等你接任務時，已開著的文件仍可照常 `finish_document`；只是不開始新的上傳與分析。
3. `task.ready` 時，以 `read_task`（`target` 為 `task`）讀完這個任務，一次呼叫讀一頁，讀到 `next` 等於 `total`；
   再以 `mapping_*` 工具完成對應，每個任務以 `mapping_finish` 結束。對應的作法與 [文件分析](../plasma-document-analysis/SKILL.md) 第 6 節相同：
   一個概念葉子只對一個欄位。值在別的表的某一欄、沿知識庫的關聯每一跳都只到一列（多對一或一對一）時，用 `mapping_submit_mapping` 帶 `through`（關聯逐跳列出，`table`、`column` 填最後一跳到達的表與欄）；類別（`kind` 為 `value`）可以跨一對多。值要計算、換算單位或幣別、依單位或有效期間查表，或要對多列加總時，用 `mapping_submit_node` 建出對應子圖，再以 `realizes` 接回概念；節點讀的是這個實體的概念：要用某一欄的值，先宣告一個對應葉子並對到那一欄，再把它當節點輸入。欄位可以用 `table` 加 `column` 指名，也可以用 `stored` 寫它的儲存概念（`exp.derived.stored.<db>.<table>.<column>`），兩者是同一筆對應。
   - 文字欄對到日期或時間的概念：欄位自己有讀法註記時程式自動轉換；沒有時，任務列出的讀法通則若涵蓋這一欄，在 `read` 帶它的 ID；都不適用就照送不帶 `read`，系統會記成 `format_unknown`，等說明讀法的註記到了再對應。
   
   缺依據時保留為提案或記為待補，不自行猜欄位、值或換算係數。
4. 重複第 2、3 步，直到作業結束。

碰到這些情況：

- **作業停下來了**（`running` 為 false，`error` 不為空）：已對應的部分保存著，schema 變更仍在等待，所有上傳都還被擋。把錯誤告訴使用者，然後照 `next_step` 再呼叫一次 `run_mapping`，從第 2 步接著做。同一個錯誤連續出現兩次時，停下來回報使用者。
- **`run_mapping` 說另一個作業在跑**：照回覆，以 `run_status`（`work_id` `workspace`）等它結束或接它的任務；結束後再呼叫 `run_mapping`。

## 3. 確認並交給使用者發布

再呼叫一次 `run_status`：`setup.remap_needed` 不再出現，表示重新對應完成，新文件可以上傳。
仍為 true 時，作業沒有完整結束：照上一節「作業停下來了」再做一次；同一個錯誤連續兩次時，把 `message` 與 `error` 告訴使用者。

最後的回報固定包含以下四項，使用者另有交代（例如這輪先不上傳）時也一樣：

1. 結果：重新對應是否完成，知識現在對應於哪一版 schema，新文件是否可以上傳。
2. 數量：`run_status` 的 `message`，幾個實體已對應、部分對應或未對應，幾個欄位接上。
3. 待補：每個部分對應或未對應的概念與原因（例如唯一鍵未確認、找不到對應欄位；`pending_items` 列出它們），以及使用者補哪些資訊或文件就能補齊。
4. 發布：附上 Commit 頁（`setup.commit_page`），請使用者到該頁逐條勾選、預覽並確認發布；新的對應在發布後才用於報表。
