---
name: plasma-fill-gaps
description: >-
  使用者事後回答 Plasma 知識庫提出的問題、後到的文件可以補齊先前的缺口，或想知道還有哪些待補項目時使用：查詢待補項目，轉交使用者的回答，逐項補齊後交使用者在 Commit 頁確認發布。
---

# 補齊缺口

全程使用台灣繁體中文；工具名稱、欄位名稱、符號與網址保留原樣。
開始前先確認連線：依 [連結 Plasma 與診斷](../plasma-mcp-setup/SKILL.md) 完成授權，以 `whoami` 核對 workspace 與 `knowledge:write`，
並以 `run_status` 記下 `setup.commit_page`（使用者確認發布的頁面）。

文件裡缺定義、缺引用對象或有歧義的部分，已知內容與出處先保存為「待補」；補齊並通過檢查後才進入 Commit 區。
補齊的來源只有兩種：使用者的回答，或之後上傳的文件。agent 不自行編造定義，也不替使用者選擇。

## 1. 查詢待補項目

呼叫 `pending_items`；只看某份文件時帶 `document`（檔名、路徑或文件 ID）。
每一項有 `fragment_id`、出處證據、缺什麼（`missing`）、受影響的規則，以及要問使用者的問題（`question`，有的話）。
把需要使用者回答的問題整理後一次問使用者，附上出處讓使用者知道問的是哪一段。

## 2. 依項目種類補齊

項目的種類看 `fragment_id` 的開頭，再看 `status`：

| 種類 | `fragment_id` | `status`／`reason` | 補齊方式 |
|---|---|---|---|
| 片段 | `fragment:…` | `awaiting_resolution` | 引用對象或其他輸入還不知道。缺的文件請使用者依 [文件分析](../plasma-document-analysis/SKILL.md) 上傳並處理；`missing` 顯示對象已存在時，用 `resume_item` 補齊（第 4 節） |
| 片段 | `fragment:…` | `gap` | 缺定義或缺必要資訊。請使用者說明並轉交（第 3 節），或由後到文件以同一符號寫出定義；之後用 `resume_item` 補齊 |
| 片段 | `fragment:…` | `ambiguity` | 引用對到多份候選文件。請使用者指定是哪一份並轉交（第 3 節），再用 `resume_item` 連結到使用者指定的那一份 |
| 文件版本 | `document:…` | `awaiting_resolution`，`reason` 為 `revision_not_staged` | 這份文件有較新的版本已上傳但還沒處理完：依文件分析 skill 開啟並完成該版本，或用 `resume_item` 開啟這一項 |
| mapping | `mapping:…` | `mapping` | 這份文件的 mapping 沒完成：`resume_item` 重新開啟 mapping，完成到文件為 `ready_to_commit` |
| mapping | `mapping:…` | `gap`，缺口說明為「使用者取消，待另一人做 mapping」 | 使用者取消了這份文件的對應，交給另一個人（通常是 IT）：`resume_item` 重新開啟這份文件的 mapping，照文件分析 skill 第 6 節完成，完成後重新查詢 `pending_items` |
| mapping | `mapping:…` | `gap`，`reason` 為缺口種類（如 `mapping_refuted`、`unmapped`） | mapping 已完成但留下資料對應缺口：照 `question` 請使用者提供資料來源或確認對應，並轉交回答（第 3 節）；之後用 `resume_item` 重新開啟這份文件的 mapping 再做一次（回答文件可用 `mapping_read_source` 讀取），完成後重新查詢 `pending_items` |
| 純缺口 | `gap:…`（以及 `formula-gap:…`、`policy-validation:…`） | `gap` | 沒有片段可重開的缺口，照 `question` 補齊。沒有任何規則定義的概念由定義它的文件補齊：使用者上傳後依文件分析 skill 處理，以同一符號寫出定義；或請使用者回答並轉交（第 3 節），在回答文件中以同一符號寫出定義並引用回答。系統失敗如實回報給使用者。補齊後重新查詢 `pending_items`，確認這一項已消失 |
| 事實衝突 | `metadata-contract:…`、`metadata-value:…`、`constant-contract:…`、`constant-value:…` | `gap` | 不同文件對同一項資料事實或常數說法不一：請使用者到 Commit 頁（`setup.commit_page`）的「事實衝突」區，填上自己的名字裁定採用哪一個；裁定後重新查詢 `pending_items` |
| 待裁定的讀取規則 | `metadata-reader:…` | `gap` | 讀取了有衝突的資料事實的規則，先保留不發布：上一列的衝突裁定後，下一次發布自然帶入 |
| 規則檢查 | `rule-check:…` | `gap`，`reason` 為檢查種類（如 `narrower_than`） | 已暫存的規則，經後到文件帶來的概念比對後，可能讓使用端寫出錯誤 SQL，`question` 說明原因：用 `resume_item` 以該規則的摘錄重開（第 4 節），重新匯出修正規則，或在 `finish_document` 以 `rule_check_answers` 說明規則為何正確；之後重新查詢 `pending_items` 確認這一項已消失 |
| 概念 DAG | `dag:…`，或 `gap:…` 且 `reason` 為 `unit_unknown`、`unit_mismatch`、`conversion_unmapped`、`grain_mismatch`、`measure_incomplete`、`incomplete` | `gap` | 單位、換算、粒度或統計定義缺資訊：`unit_unknown` 請 IT 說明欄位存放的單位；`unit_mismatch` 請說明換算依據（係數或查表）；`conversion_unmapped` 請說明係數或匯率在哪個欄位；`grain_mismatch` 請說明關聯是一對一還是一對多；`measure_incomplete` 依 `detail` 列出的欄位逐項問業務（例如是總和比還是各列比率平均）；`incomplete` 是上層概念因子概念不完整而不完整，順著 `causes`（缺口的 `references`）找到根本原因那一項來補，不重複問上層。回答依第 3 節轉交，補上的單位或係數引用該回答 |
| 系統狀態 | `system:…` | `gap`，`reason` 為 `system_failure` | 待補缺口無法列出：把 `missing` 如實回報給使用者；其他項目照常處理 |

`pending_items` 回覆有 `definition_rulings` 時，是使用者在 Commit 頁對同一概念兩份定義的裁定：採用的一份（`adopted`）與擱置的一份（`set_aside`），各有文件、版本與來源。`status` 為 `staged` 表示下一次發布才生效，`applied` 表示已生效，`stale` 表示裁定後有一份定義改了、裁定不再套用，請使用者到 Commit 頁重新裁定；`retired` 表示已沒有可裁定的兩份定義（`reason` 說明原因，例如某份文件不再定義該概念），不必再請使用者裁定。這不是待補項目，不用 `resume_item`。

`resume_item` 用於片段、文件版本、mapping 與規則檢查項目；純缺口、事實衝突、待裁定的讀取規則與系統狀態由文件、回答或裁定補齊，重新查詢 `pending_items` 確認。
`resume_item` 回覆錯誤時，錯誤說明這一項如何補齊，照該說明處理。

## 3. 轉交使用者的回答

使用者回答後：

1. 呼叫 `submit_answer`：`question` 是你問的問題，`answer` 照錄使用者的原話，回答某個待補項目時 `gap_fragment_id` 帶該項的 `fragment_id`。
   回答會存成一份回答文件並記下回答的使用者，回覆它的 `upload_id` 與回答所在的行（`answer_lines`）。
2. 交給一個子代理以 `open_document` 開啟這個 `upload_id`，之後每次呼叫帶回覆的 `work_id`；像一般文件一樣讀取並以行號引用回答，完成到結束與 mapping。
   回答的是純缺口時，在這份回答文件中以缺口的同一符號寫出定義。
3. 依第 2 節的種類收尾：片段與 mapping 項目用 `resume_item` 補齊（下一節）；純缺口在回答文件完成後重新查詢 `pending_items`。

只轉交使用者實際說的話；使用者沒有回答的問題維持待補。

## 4. 補齊一個項目

`resume_item`（`fragment_id`）把一個項目開成一份工作，回覆它的 `work_id`；補這一項的每次呼叫（文件工具、`run_status`、`read_task`、`mapping_*`）都帶上它。
項目多時每項交給一個子代理，由主代理協調共用的概念與要問使用者的問題。

- 片段項目：以 `retry_target` 指出的摘錄開啟。先用 `read_task`（`target` 為 `document`）讀完 protocol，重新讀取相關的行，依 protocol 補齊後呼叫 `finish_document`；之後照文件分析 skill 第 6 節完成 mapping。
- 文件版本項目：像一般文件一樣讀完 protocol 並完成整份文件與 mapping。
- mapping 項目：直接進行這份文件的 mapping，用 `read_task`（`target` 為 `task`）與 `mapping_*` 工具，每個任務以 `mapping_finish` 結束。`status` 為 `gap` 的項目是再做一次：先用 `mapping_read_source` 讀使用者的回答。

## 5. 使用者指正翻譯

使用者看了翻譯（`read_translation`）指出某一項不對時，照 [文件分析](../plasma-document-analysis/SKILL.md) 第 7 節處理：依該項的 `fragment_id` 以 `resume_item` 帶 `correction`（照錄原話）重開、修正後重新提交，再把改過的翻譯貼給使用者。

## 6. 完成

補齊的文件到 `ready_to_commit` 後，請使用者到 Commit 頁（`setup.commit_page`）逐條勾選、預覽並確認發布；發布只能由使用者在頁面上操作。
`setup.commit_page` 是空的時，告訴使用者伺服器沒有設定 Commit 頁網址，請管理員設定，不要自行拼網址。
仍待補的項目與原因如實列給使用者。
