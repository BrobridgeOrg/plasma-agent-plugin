---
name: plasma-create-pview
description: >-
  將報表、表單、Power BI 或資料需求建立為 Plasma pview 時使用；也處理使用者明確指定 mview 為最終目標的需求。預設在 mview 完成運算、經使用者確認同步與排程後，以 pview 提供參數篩選；pview 一律讀取 mview，不可略過 mview 直接查來源。全程台灣繁體中文，不建立或發布資料 API。
---

# 建立 mview 運算層與 pview 篩選層

維持一個建立技能。**預設最終輸出 pview；只有使用者明確指定 mview 為最終目標時，才停在 mview。**
「不需要參數」或 AI 判斷用途不需要參數，不等於使用者指定 mview；必要時釐清最後篩選需求，
不自行改變輸出類型。使用者明確限定「只建立定義」時，遵守範圍，不啟動同步。
本 skill 不建立一般 view；明確要求一般 view 時，說明本流程支援 pview／mview 並釐清，不能靜默替換。

```text
預設：知識查核 → SQL 驗證 → manual mview → 使用者確認同步／排程
      → 同步成功並核對排程 → pview 參數篩選 → 驗證與交付
指定 mview：同一流程到同步成功並核對排程 → 交付 mview
```

**不能略過 mview。** pview 的 FROM 只能是本流程建立或核對重用的 mview，
不能直接查來源表，也不能把運算 SQL 直接放進 pview。下列都不是略過 mview 的理由：

- SQL 很簡單、只有單表篩選，或 `run_query` 已驗證成功。
- 想省去同步等待、使用者尚未確認同步，或同步失敗／逾時。
- pview 本身可以帶參數，看起來「一步完成」比較快。

遇到上述情況時照流程建立 manual mview，並依第 4 節處理同步；
使用者未確認同步時，交付尚未同步的 mview（與指向它的 pview 定義），不改走來源表。

## 共通規則與工具

- 全程台灣繁體中文；SQL、工具名稱、識別名稱與 URL 保留原樣。
- 先用 `whoami` 核對 workspace、權限與知識服務；全程使用同一連線。
  工具或權限不足時依 [連線技能](../plasma-mcp-setup/SKILL.md) 處理，不改用 shell／REST 繞過。
- 本流程使用 Plasma 知識庫工具、`whoami`、`list_views`、`get_view`、`run_query`、
  `create_view`、`sync_view`、`get_view_schedule`、`set_view_schedule`、
  `list_pviews`、`get_pview`、`create_pview`、`execute_pview`。參數以實際工具 schema 為準。
- 不建立 access entry、export API、匯出 blueprint、檔案或外部資料庫輸出。
  **資料 API 由使用者自行建立**，交付時說明即可，不詢問驗證方式／有效期限或要求發布權限。
- 已授權的查核、驗證與建立連續完成；不逐步詢問是否繼續。
  必要假設須明確說明並取得接受；同步與排程另外遵守下方確認規則。

## 1. 確認輸出與運算分工

整理欄位、指標、粒度、歷史涵蓋範圍、時間欄位、時區、最後篩選條件及更新頻率。
沒有頻率或必要參數資訊時，先釐清，不能自行設定高頻同步。

- **mview 完成運算**：JOIN、清理、業務條件、計算欄位、聚合、排名等都在 mview SQL 完成。
  保留 pview 最後篩選所需欄位與粒度；mview 歷史範圍不綁死在本次查詢的起訖日。
- **pview 只做最後 filter**：選取既有欄位，加參數化 WHERE；不加入 JOIN、聚合、
  計算欄位、視窗函式或業務運算。參數型別轉換可放在 WHERE。
- 任意期間去重人數、區間排名等無法由預先運算結果正確篩選取得時，說明限制並釐清
  報表粒度／可支援的區間，不能把每日去重值或平均值相加，也不能偷移運算到 pview。
- 本 skill 的 pview 一律以 mview 為來源。使用者明確要求 pview 直接查來源表時，
  說明本流程不支援並建議改走 mview；不自行改走來源表，也不宣稱 backend 禁止。

## 2. 查核知識、驗證運算 SQL

使用 [Plasma 知識庫技能](../plasma-knowledge-lookup/SKILL.md) 與
[知識查核及 Trino SQL 規則](references/knowledge-sql.md) 完整查核來源後才產生 SQL。
所有來源表、欄位與代碼依證據取得；mview 衍生欄位則以已驗證 SQL 與實際結果核對，
不要求 Plasma 知識庫事先收錄本次新物件。

`run_query` 會實際查來源，最多回傳 100 列；LIMIT 不能保證掃描／聚合量小。
驗證採需求可接受的小時間範圍，避免反覆執行完整重查詢；不能將驗證用的範圍或
樣本 LIMIT 留在正式 mview，除非它本來就是報表需求。
同步確認不代表 SQL 驗證不會查來源，建立前如實說明驗證結果及假設。

## 3. 建立或重用 mview

`list_views` → `get_view` 檢查同名物件及可重用來源。核對 SQL、欄位、粒度、歷史範圍、
workspace 與同步狀態；同名但不相符時不覆寫、不默認可重用。
讀取 `get_view_schedule`，保留既有合適排程；不為重用而重複同步或修改排程。

mview 名稱最多 **30 個字元**。使用者名稱過長時提出替代名稱並取得確認；未指定時
直接產生清楚且符合長度的名稱。建立與查核沿用相同名稱。

新建一律 `create_view(type=materialized_view, sync_mode=manual)`，省略 `scheduler_settings`。
**不能用 scheduled 建立繞過同步確認**；scheduled 會啟動首次同步。
建立後用 `get_view` 核對 ID、SQL、path、類型及狀態，等非同步初始化完成才操作同步／排程。
逾時先查是否已建立，不直接重送。只建立定義的需求到此交付未同步的 mview，
若最終要求 pview，依已核對路徑建立 pview 定義，但明確標記尚未完成資料驗證。

## 4. 同步與排程前取得確認（skill 軟限制）

在任何 `sync_view`、`set_view_schedule` 或其他會啟動同步的操作前，呈現：

- workspace、mview 名稱／ID、來源及資料範圍。
- 是否執行首次／額外同步，以及會查詢來源 DB 的事實。
- 定期排程的頻率、首次排程時間與時區；既有排程是否要修改。

**等待使用者明確確認該次具體內容後才執行。**「幫我做報表」、OAuth 授權、工具 annotations
或先前對不同 SQL／範圍的同意，都不是這次同步核准。不以逾時或沉默視為同意。
有宿主確認工具時使用，否則直接詢問並結束回合等待回覆。

首次同步與排程可一次確認，已確認的定期排程不逐次詢問；變更 SQL、資料範圍或頻率時
重新確認。使用者拒絕或尚未回答時，保留定義並交付「尚未同步／未設定排程」的狀態，
不執行同步、不宣稱資料可用。不把確認做成可由 AI 自填的 `confirmed=true`。
這是 skill 行為規範，gateway 沒有強制核准機制。

## 5. 同步、核對並設定排程

操作前讀取 [API 與工具格式](references/api.md) 的同步／排程部分。

1. 重新讀取 mview 與排程，確認自核准後 SQL／範圍未變、沒有其他同步正在執行。
2. 新建或確實需要刷新時，記錄 `last_sync_at`、`last_sync_status`，呼叫一次 `sync_view`。
   已同步且資料範圍／新鮮度符合需求的既有 mview，可重用，不額外觸發同步。
3. `sync_view` 只表示請求已接受。以 `get_view` 核對當次同步成功，不能把先前的
   `last_sync_status=synced` 當成本次成功；結合 `last_sync_at` 更新判斷。
   每次查詢間隔約 10 秒，最多觀察 5 分鐘；仍執行中時交付 ID 與狀態，讓後續回合續查。
   失敗或請求逾時時先查狀態，不直接重送、不改查來源 DB 代替 mview。
4. 首次同步成功後，設定已確認的定期排程。預設使用未來的 `start=setTime`，
   避免 `immediately` 又觸發一次同步；若已確認時間已過，提出新時間再確認。
   `get_view_schedule` 核對設定與 enabled，並用 `get_view` 核對 sync_mode。
   既有排程 disabled 時不能只 PUT 設定就宣稱已啟用；工具無法啟用時回報限制。
5. 同步成功但排程失敗，明確分別交付，不能回報整體完成。指定 mview 最終輸出時到此結束。

## 6. 建立 pview 與驗證

先讀取 [API 與工具格式](references/api.md) 的 pview 部分。

**呼叫 `create_pview` 前逐項核對，任一項不成立就回到對應步驟，不建立 pview：**

1. 已有 mview ID，且 `get_view` 回傳 `type=materialized_view`（本流程新建或已核對重用）。
2. pview SQL 的 FROM 只有該 mview 的路徑，沒有來源表、JOIN 或子查詢。
3. SELECT 只選 mview 已產出的欄位，WHERE 只有參數化篩選。
4. mview 同步狀態已依第 4、5 節處理；尚未同步時在交付標明。

用 `list_pviews`／`get_pview` 核對同名物件，完整相符可重用；不覆寫不同定義。
pview 使用 `get_view` 回傳並經查詢核對的 mview 路徑，不猜 schema、catalog 或物件名稱。
原樣處理後端回傳的 SQL 路徑，不把 mview 的來源 SQL 複製到 pview。

定義參數名稱、型別、必要值／預設值、時區；時間區間採 `[開始, 結束)`。
若使用者輸入含尾日的日期範圍，說明如何換成下一日的排他結束值。
未提供的必要參數不能默認查全表；確認 start < end。

`create_pview` → `get_pview` 核對 SQL／param_def → `execute_pview` 驗證。
至少驗證兩個不同區間、區間邊界與空結果；視需要檢查缺少／無效參數的回應。
查詢使用 `is_constant=true` 的測試值；日期參數在 SQL 用 DATE／TIMESTAMP 前綴或 CAST。
不把驗證樣本 LIMIT 留在 pview 定義；`truncated=true` 時不把 total 當完整筆數。
建立逾時先查既有物件再決定，不直接重建。

## 7. 交付並結束

交付 workspace、最終 pview／mview 的 ID 與名稱、來源 mview、運算與篩選摘要、
參數格式／範例、已接受假設、SQL 驗證結果、最後成功同步時間及排程。
重用物件標明重用；同步、排程或驗證未完成時分別列出，不宣稱全部成功。

完成 pview（或指定的最終 mview）後，**在對話中直接整理本次 SQL 的輸出欄位表**，
不另建檔案。欄位順序與 pview SELECT 一致，名稱與型別以 `get_pview`／`execute_pview`
回傳或 `get_view` 核對結果為準，不憑記憶填寫：

| 欄位名稱 | 型別 | 說明 | mview 運算方式／來源 |
|---|---|---|---|
| `report_date` | date | 報表日期 | `CAST(o.order_time AS DATE)`，來源 `sales.orders.order_time` |

- 「說明」使用業務語意；「運算方式／來源」寫出 mview 中的運算式或來源 `database.table.column`。
- 在表格下方另列 pview 參數：名稱、型別、是否必填、對應篩選欄位及條件（如 `report_date >= @start_date`）。
- 無法確認的型別或語意標明「未確認」，不猜測。

說明「資料 API 請在 Plasma 自行建立」。不產生 API URL、不建立或發布 export API，
不要求使用者提供 API 驗證方式、有效期限或匯出目的地。
