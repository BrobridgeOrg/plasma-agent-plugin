---
name: plasma-fill-gaps
description: >-
  使用者事後回答 Plasma 知識庫提出的問題、後到的文件可以補齊先前的缺口，或想知道還有哪些待補項目時使用：查詢待補項目，轉交使用者的回答，逐項補齊後交使用者在 Commit 頁確認發布。
---

# 補齊缺口

全程使用台灣繁體中文；工具名稱、欄位名稱、符號與網址保留原樣。

開始前先確認連線：依 [連結 Plasma 與診斷](../plasma-mcp-setup/SKILL.md) 完成授權，以 `whoami` 核對 workspace 與 `knowledge:write`，並以 `run_status` 記下 `setup.commit_page`（使用者確認發布的頁面）。

**本 skill 只用於進階模式。** 核對 scopes 之前，先依 [查 Plasma 知識庫](../plasma-knowledge-query/SKILL.md) 判斷模式：工具清單沒有 `compile_query`、但有 `search_knowledge` 時，這個系統是一般模式（建立後不會改變）。這時告訴使用者：這個系統是一般模式，plugin 目前沒有一般模式的補缺口流程，待補的問題請在 Plasma 介面處理。然後結束本 skill：不要求使用者重新授權，也不改用其他工具或自己寫 SQL 繞過。兩組工具都沒有時，依連結 Plasma 與診斷的「診斷」處理。

文件裡缺定義、缺引用對象或有歧義的部分，已知內容與出處先保存為「待補」；補齊並通過檢查後才進入 Commit 區，使用者發布後才生效。

補齊的來源有三種：使用者的回答、文件寫明的內容（已在系統裡的文件與之後上傳的文件），或使用者授權你自行補齊時你讀到的來源（資料表的列、schema、其他系統）。授權補齊時，以 `submit_answer` 把讀到的內容放在 `answer`、讀取位置放在 `source`，它存成回答文件並記下依據，照第 4 節在回答文件引用；片段的 `statement.channel` 用 `user_added`。

使用者沒有授權、回答與來源也都沒寫明的內容，照 `question` 問使用者，或留在待補：不自行編造定義，也不替使用者選擇。

**每個回覆都看 `next_step`。** 照做；做完回到你在流程中的那一步。

## 主流程

1. 呼叫 `pending_items` 列出待補項目（第 1 節）。
2. 決定這次處理哪些項目（第 2 節）：能用現有依據補的自己補，需要使用者的整理成問題一次問。
3. 一項一項補（第 3、5 節）；使用者回答了，照第 4 節轉交後再補。
4. 都走完一次後，再查一次 `pending_items`，請使用者發布（第 6 節）。

## 1. 查詢待補項目

呼叫 `pending_items`；只看某份文件時帶 `document`（檔名、路徑或文件 ID）。每一項有：

- `fragment_id`：用 `resume_item` 開它時用的 id。
- 出處證據、缺什麼（`missing`）、受影響的規則，以及要問使用者的問題（`question`，有的話）。
- `closes_with`：一句話說明由哪個工具、哪個產物補上，依據去哪裡找。**補一項時照它做。**
- `fill`：
  - `evidence`：知識庫或已轉交的回答已有依據，`fill_basis` 指出是什麼，直接補。
  - `retry`：系統處理失敗，不用問人，再做一次。
  - `person`：系統裡找不到依據。先查一次（第 2 節），仍沒有就問使用者。
  - `later`：要等系統之後支援，沒有要問的，也不用 `resume_item`；最後如實告訴使用者，`fill_basis` 是原因。
- `how`：帶 `how` 的項目不用 `resume_item` 開。`how` 說明它由什麼補上，例如使用者在 Commit 頁裁定、`bind_term`，或回報使用者。照 `how` 做。

只是在等子概念的上層概念，會摺疊在它等的那一項底下：那一項的 `dependents` 是等它的項數，補齊那一項，它們就一起完成。要看全部時帶 `all: true`，每項再列出它等的 `waits_on`。

`pending_items` 回覆有 `definition_rulings` 時，那是使用者在 Commit 頁對同一概念兩份定義的裁定，不是待補項目，不用 `resume_item`。`status` 為 `stale` 時，請使用者到 Commit 頁重新裁定。

## 2. 決定處理範圍

- **自己補的**：`fill` 為 `evidence` 與 `retry` 的項目。
- **先查再決定的**：`fill` 為 `person` 的項目，先依 `closes_with` 說的地方查一次。通常是用 `lookup_knowledge` 查 `missing` 的用語或相關的表：別份文件寫明了缺的內容時，以那份文件的出處為依據接手；查不到就照 `question` 問使用者。
- **問使用者**：把要問的整理後一次問，附上出處，讓使用者知道問的是哪一段。
- **每項一次修補**：一個項目開一次 `resume_item`，修補一次，看回覆，再換下一項。
- **同一份文件的多個項目依序做**：除了最後一項，`resume_item` 都帶 `more_repairs: true`，文件停在 `mapping_held`；最後一項不帶，完成時一次 mapping 涵蓋全部修補。最後一項開不起來或已不用補時，對這份文件的 `mapping:` 項目呼叫 `resume_item`，由它完成累積的 mapping。
- **開了才發現不用改**：對重做目標 `close_fragment` 帶 `discard: true`，再 `finish_document`；這次修補放棄，那段摘錄和文件維持原樣（work 結束為 `repair_dropped`），接著做下一項；它若是同一份文件 `more_repairs` 的最後一項，照 next_step 對該文件的 `mapping:` 項目 `resume_item`，完成前面修補的 mapping。
- **結束條件**：範圍內的項目都走完一次、所有文件都到 `ready_to_commit`（或 work 以 `repair_dropped`、`unchanged` 結束）、沒有文件停在 `mapping_held`。

## 3. 碰到各種項目時

每一項的 `closes_with` 已經寫了怎麼補；這裡說的是各種項目的意思，和容易走錯的地方。

- **片段項目**（`fragment:…`）：
  - 缺引用對象（`awaiting_resolution`）：缺的文件請使用者依 [文件分析](../plasma-document-analysis/SKILL.md) 上傳並處理；`missing` 顯示對象已存在時，用 `resume_item` 補。
  - 缺定義或缺資訊（`gap`）：請使用者說明並轉交（第 4 節），或由後到文件以同一符號寫出定義；之後用 `resume_item` 補。
  - 引用對到多份候選（`ambiguity`）：請使用者指定，轉交後用 `resume_item` 連到那一份。
- **文件版本項目**（`document:…`）：有較新的版本還沒處理完。依文件分析 skill 開啟並完成該版本，或用 `resume_item` 開這一項。
- **mapping 項目**（`mapping:…`）：
  - mapping 沒做完：`resume_item` 開它，完成到 `ready_to_commit`。
  - mapping 做完但留下資料對應缺口：照 `question` 請使用者提供資料來源或確認對應，轉交回答後再 `resume_item`，用 `mapping_read_source` 讀回答。
  - 使用者取消了對應、交給另一個人：`resume_item` 開它，照文件分析 skill 第 6 節完成。
- **概念沒有定義**（`gap:…`）：`resume_item` 會開宣告或讀它的那段摘錄，回覆的 `resumed_for` 是原項目；在那裡依原文、回答或授權的來源補上定義。兩者都沒有時，項目帶 `how`：由定義它的文件補，或請使用者回答並轉交，在回答文件中以同一符號寫出定義。
- **讀法待寫**（`read-pending:…`）：`resume_item` 開讀法陳述那段文字，範圍涵蓋的欄寫讀法轉換規則，範圍外的欄以 `submit_annotation`（`kind` `read_format`、`format` `as_stored`）標記照原樣讀。
- **概念要讀的表或欄位不在 schema 裡**（mapping 記成 `no_source`）：問使用者：表或欄位還沒匯入，還是文件用了另一個名稱？是另一個名稱時，重新對應並把使用者說的表或欄位當依據，文件的片段不用重做；還沒匯入時，等匯入後重新對應。
- **計算或統計缺資訊**（`dag:…`）：這些都在節點所在的那段摘錄裡補，不在回答文件裡；回答文件不能修改別份文件的節點。所以使用者回答後，先照第 4 節轉交，再 `resume_item` 這一項，在打開的片段裡以 `submit_node` 補上，引用回答的行。各種缺口要補的東西：
  - 一筆對到幾筆（`grain_mismatch`）：關聯的基數。先用 `lookup_knowledge` 查兩邊的表，看文件有沒有寫（例如「N:M」「一對多」）。有寫，就 `resume_item` 打開關聯所在的片段，以 `submit_node` 重送那個關聯，帶上 `params.cardinality`（從第一個參數的列看到第二個參數的列：`N:1`、`1:N`、`1:1` 或 `N:M`），引用那段原文。沒寫才問使用者。
  - 指標缺欄位（`measure_incomplete`）：依 `detail` 列出的欄位逐項問業務（例如是總和比還是各列比率平均），再重送 measure。
  - 缺單位（`unit_unknown`）：在宣告的 `arg_values` 補 `unit` 與 `unit_quote`；欄位存放的單位，則用 `submit_annotation` 的 `unit` 記在欄位上。
  - 單位對不上（`unit_mismatch`）：加一個 `convert` 節點，係數照文件寫明的（`params.factor`，`basis` 引用係數），或寫明查表；沒有依據就問使用者係數。
  - 換算係數的欄位沒對應（`conversion_unmapped`）：`resume_item` 開 mapping 項目，對應那個欄位。
  - 不完整（`incomplete`）：上層概念因子概念不完整而不完整，已摺疊在根本原因底下（`causes` 是缺口的 `references`，`all: true` 時看 `waits_on`）；補根本原因那一項。
- **規則檢查**（`rule-check:…`）：用 `resume_item` 以該規則的摘錄重開，修正規則；原文同時寫了名稱與代碼時，送 `submit_backbone` kind `code_name`。或在 `finish_document` 以 `rule_check_answers` 說明規則為何正確。
- **事實衝突**（`metadata-contract:…`、`metadata-value:…`、`constant-contract:…`、`constant-value:…`、`versioning:…`）：不同文件說法不一，請使用者到 Commit 頁的「事實衝突」區裁定。讀取了這些事實的規則（`metadata-reader:…`）會在裁定後的下一次發布自然帶入。
- **綁定**（`binding:…`）：文件說明了一個 schema 裡沒有的名稱。使用者確認是哪張表或哪個欄位後，用 `bind_term` 綁定，不用 `resume_item`。
- **跨表正負號**（`sign:…`）：要等系統支援，告訴使用者，沒有要問的。
- **系統狀態**（`system:…`、`policy-validation:…`）：系統處理失敗，把 `missing` 如實回報給使用者，其他項目照常處理。

其他規則：

- 項目帶 `resume_after` 時，它的文件還在這個連線做 mapping：先完成那個 `work_id` 的 mapping，到 `ready_to_commit`，再 `resume_item` 這一項。
- `resume_item` 的 `fragment_id` 用 `pending_items` 或 `read_translation`（摘錄分組的 `fragment_id`）給的值。摘錄代號（`x` 加 8 位十六進位）、`finish_document` 列出的缺口 id、`read_translation` 列出的規則或關聯 id，也會開啟它所屬的片段。
- `resume_item` 的回覆有 `item`：這一項的原因、缺什麼、問題與 `closes_with`。打開後先看它。
- `resume_item` 回覆錯誤時，錯誤說明這一項怎麼補，照它做。

## 4. 轉交使用者的回答

使用者回答後：

1. 呼叫 `submit_answer`：`question` 是你問的問題，`answer` 照錄使用者的原話；回答某個待補項目時，`gap_fragment_id` 帶該項的 `fragment_id`。回答會存成一份回答文件並記下回答的使用者，回覆它的 `upload_id` 與回答所在的行（`answer_lines`）。轉交後，這一項的 `fill` 會變成 `evidence`，`fill_basis` 指向這份回答，不要再問一次。
2. 交給一個子代理以 `open_document` 開啟這個 `upload_id`，之後每次呼叫帶回覆的 `work_id`；像一般文件一樣讀取，並以行號引用回答，完成到結束與 mapping。
   - 回答的是概念沒有定義（`gap:…`）時，在這份回答文件中以缺口的同一符號寫出定義。
   - 使用者確認的是既有概念讀哪個欄位時，以 `record_clue`（`concept` 為該概念的完整名稱、`kind` 為 `column`、`text` 照錄欄位）記下；既有概念用完整名稱讀，不放進 `new_predicates`。
   - 回答的是計算或統計的缺口（`dag:…`）時，回答文件只保存原話；之後照第 3 節 `resume_item` 那一項，在節點所在的片段補上。
3. 依第 3 節的種類收尾，片段、節點與 mapping 項目用 `resume_item` 補齊（第 5 節）。

只轉交使用者實際說的話；使用者沒有回答的問題維持待補。

## 5. 補齊一個項目

`resume_item`（`fragment_id`）把一個項目開成一份工作，回覆它的 `work_id`。補這一項的每次呼叫（文件工具、`run_status`、`read_task`、`mapping_*`）都帶上它。

查已有的知識直接用 `lookup_knowledge`：還沒開工作，或自己的工作已結束時，不帶 `work_id` 也查得到已發布與 Commit 區暫存的知識。

項目多時，每項交給一個子代理，由主代理協調共用的概念與要問使用者的問題。

- **片段項目**：以 `retry_target` 指出的摘錄開啟。
  - 先用 `read_task`（`target` 為 `document`）讀完 protocol，重新讀取相關的行，依 protocol 補齊後呼叫 `finish_document`；之後照文件分析 skill 第 6 節完成 mapping。mapping 只派發這次修補動到的概念。
  - 修一個計算節點：`submit_node` 帶既有概念的名稱（本摘錄的概念用短名，其他摘錄的概念用完整名稱），節點換成新的寫法，片段的規則、宣告與註記照舊。
  - 改規則：`submit_rules` 取代這個片段的全部規則，所以把要留下的規則一起送出。
- **文件版本項目**：像一般文件一樣讀完 protocol，完成整份文件與 mapping。
- **mapping 項目**：直接進行這份文件的 mapping，用 `read_task`（`target` 為 `task`）與 `mapping_*` 工具，每個任務以 `mapping_finish` 結束。`status` 為 `gap` 的項目是再做一次：任務列出還沒對應或有缺口的概念，已對應到欄位的概念保持唯讀；先用 `mapping_read_source` 讀使用者的回答。

修補時碰到這些情況：

- **提交被拒**：修正後再送；原文或回答確實寫不成產物時，以 `report_gap` 記下原文說了什麼與拒絕的原因。這一項會留在待補清單，最後回報原因。被拒之後不要直接 `finish` 或 `close_fragment`：它們會先拒絕一次，把那則拒絕再給你看。
- **`finish_document` 回覆 `finished: false` 與 `lost_products`**：修補還沒生效。列出的是這個片段原有、這次沒帶到的產物。還要保留時，對 `retry_target` 再 `begin_fragment` 一次，連同它們一起匯出；原文不支持、確定要移除時，帶 `confirm: true` 呼叫 `finish_document`，並在最後的回報列出。
- **確認結果**：`finish_document` 回覆的 `changes` 說明這次修補對整份文件做了什麼，mapping 結束後看 `run_status` 的 `document.changes`。`gaps_closed` 是補上的缺口；`item_open` 說這一項是否還在待補，`pending` 說這份文件還有幾項待補。沒有 `changes` 時看 `changes_unavailable`，改以 `read_translation` 確認。
- **工作被系統收回**（回覆說 `work_id` 已結束）：照回覆做，它會說要再 `resume_item` 哪一項。

## 6. 使用者指正翻譯

使用者看了翻譯（`read_translation`）或在 Commit 頁看到某條規則有問題時，照 [文件分析](../plasma-document-analysis/SKILL.md) 第 8 節處理。

指正的是已發布的資料對應（概念對錯欄位）時，以 `withdraw_mapping` 撤回那筆對應，請使用者在頁面上發布撤回，再以 `resume_item` 重開這份文件的 mapping 重新對應。重新上傳同一份文件不會改變對應。

## 7. 完成

補齊的文件到 `ready_to_commit` 後，再查一次 `pending_items`，然後請使用者到 Commit 頁（`setup.commit_page`）逐條勾選、預覽並確認發布；發布只能由使用者在頁面上操作。`setup.commit_page` 是空的時，告訴使用者伺服器沒有設定 Commit 頁網址，請管理員設定，不要自行拼網址。

仍待補的項目與原因如實列給使用者；`later` 的項目與修補中確定移除的產物也一起列出。
