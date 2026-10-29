---
name: plasma-remap
description: >-
  Plasma 進階模式提示資料來源的 schema 已變更（`whoami` 或 `run_status` 的 `setup.remap_needed`／`remap`、Commit 頁的提示），或使用者要把既有概念重新對應到新的 schema 時使用：以 `run_mapping` 重新對應既有概念，不重新摘錄文件；完成前不能上傳或分析新文件。
---

# 重新對應既有概念

全程使用台灣繁體中文；工具名稱、欄位名稱、符號與網址保留原樣。
開始前先確認連線：依 [連結 Plasma 與診斷](../plasma-mcp-setup/SKILL.md) 完成授權，以 `whoami` 核對 workspace 與 `knowledge:write`。

資料來源的 schema 變更後（新增或改了表、欄位），既有概念還對應在舊的 schema 上。
重新對應只重做概念與資料表、欄位之間的對應，概念、規則與文件摘錄都不變；完成前，Plasma 不接受新文件的上傳與分析。

## 1. 確認需要重新對應

不帶 `work_id` 呼叫 `run_status`：

- `setup.remap_needed` 為 true 時，`setup.remap` 說明知識對應於哪一版、目前是哪一版。把這兩版告訴使用者，說明接下來要重新對應既有概念、不重新摘錄，期間不能上傳新文件。
- `setup.remap_needed` 不存在時，不需要重新對應：告訴使用者，不呼叫 `run_mapping`。
- 記下 `setup.commit_page`（使用者確認發布的頁面）。

## 2. 執行重新對應

1. 呼叫 `run_mapping`，開始重新對應的作業。
2. 以 `run_status`（`wait_seconds` 15）等到 `task.ready`，或作業結束（`running` 為 false）。
3. `task.ready` 時，以 `read_task`（`target` 為 `task`）讀完這個任務，一次呼叫讀一頁，讀到 `next` 等於 `total`；
   再以 `mapping_*` 工具完成對應，每個任務以 `mapping_finish` 結束。對應的作法與 [文件分析](../plasma-document-analysis/SKILL.md) 第 6 節相同：
   一個概念葉子只對一個欄位；值在別的表、要計算、單位逐列不同或要換幣別時，用 `mapping_submit_node` 建出對應子圖，再以 `realizes` 接回概念；
   缺依據時保留為提案或記為待補，不自行猜欄位、值或換算係數。
4. 重複第 2、3 步，直到作業結束。`error` 不為空時，把錯誤原樣告訴使用者並停止。

## 3. 確認並交給使用者發布

1. 再呼叫一次 `run_status`：`setup.remap_needed` 不再出現，表示重新對應完成，新文件可以上傳。
   仍為 true 時，作業沒有完整結束：把 `message` 與 `error` 告訴使用者，不要重複呼叫 `run_mapping`。
2. 告訴使用者 `run_status` 的 `message`（幾個實體已對應、部分對應或未對應），以及 `pending_items` 中仍待補的資料對應項目。
3. 請使用者到 Commit 頁（`setup.commit_page`）逐條勾選、預覽並確認發布；新的對應在發布後才用於報表。
