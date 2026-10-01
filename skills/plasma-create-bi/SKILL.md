---
name: plasma-create-bi
description: >-
  以 Plasma pview（Parameter View）為資料來源產生 BI 報表時使用，來源可以是本 session 剛建立的 pview，或使用者指定的 pview 名稱。輸出 Power BI 專案（PBIP）或單一 HTML BI 頁面。不含資料 API 發布。
---

# 以 pview 產生 BI 報表

資料來源一律是 Plasma pview，BI 只負責呈現。運算與彙總已由
[建立 pview 技能](../plasma-create-pview/SKILL.md) 在 mview 完成；本 skill 不修改 pview／mview，
發現來源不適合時回到該技能處理。

```text
決定來源 pview → 核對粒度與欄位語意 → 使用者選擇路徑（PBIP／HTML）
→ 規劃視覺與參數 → 確認資料 API 回應 → 產生檔案 → 驗證與交付
```

## 共通規則

- 全程台灣繁體中文；SQL、DAX、M、工具名稱、識別名稱與 URL 保留原樣。
- 先用 `whoami` 核對 workspace；工具或權限不足時依 [連線技能](../plasma-mcp-setup/SKILL.md) 處理。
- 使用 `whoami`、`list_pviews`、`get_pview`、`list_views`、`get_view`、`execute_pview`。
  `execute_pview` 只用於核對欄位、型別與筆數，**不作為 BI 的資料來源或內嵌資料**。
- 不建立 access entry、export API 或排程；**BI 讀取使用者在 Plasma 自行建立的資料 API**。
- 資料 API URL 若含存取權杖，視同憑證：不寫進對話摘要以外的地方、不發布到外部網站、
  不提交到 Git。產生的檔案預設放占位值，使用者明確要求才寫入實際 URL，並提醒風險。
- 已確認的規劃連續完成；不逐檔詢問是否繼續。

## 1. 決定來源 pview

- **接續本 session 的建立流程**：直接使用 `plasma-create-pview` 剛交付的 pview（以其 ID），
  並沿用當時整理的輸出欄位表與參數說明，不再要求使用者指定。
- **沒有前序建立流程**：請使用者提供 pview 名稱。以 `list_pviews(keywords=<名稱>)` 查找，
  只接受名稱完全相符的一筆；沒有相符時列出相近名稱請使用者確認，多筆相符時請使用者選擇。
  不從清單自行挑選、不以 ID 或描述猜測。

以 `get_pview` 讀取 SQL 與 `param_def`，並核對：

1. **來源是 mview**：SQL 的 FROM 路徑對得上 `list_views`／`get_view` 中 `type=materialized_view`
   的 `path`。不是時說明風險並建議回到 `plasma-create-pview`；使用者明確接受才繼續。
2. **粒度適合報表**：以使用者預期的最大參數範圍執行一次 `execute_pview`（`page_size` 取小值），
   `total` 須低於下游單次回傳上限（未提供時以 1,000 列為假設），且 `truncated=false`。
   結果是逐筆明細或超過上限時停止，說明原因並回到 `plasma-create-pview` 調整 mview 粒度，
   不在 BI 端分頁撈取或加總明細。
3. **欄位語意**：本 session 有欄位表時沿用；沒有時以 `get_view` 讀 mview SQL 判讀。
   將每個欄位分類為「期別」「區塊（如 `section`）」「維度」「可加總量值（分子、分母、次數、總和）」
   「不可加總量值（去重人數、比率、平均、中位數、排名）」。無法判斷的欄位請使用者確認，不猜測。

## 2. 選擇輸出路徑

使用者尚未指定時，**請使用者二選一**（宿主有提問工具時使用）：

| 路徑 | 產出 | 適合 |
|---|---|---|
| **1. Power BI（PBIP）** | Power BI Desktop 專案資料夾：TMDL 語意模型、PBIR 報表、Power Query 參數 | 已使用 Power BI、要發布到 Power BI Service |
| **2. HTML BI** | 單一 `.html` 檔：參數表單、KPI 卡片與圖表，瀏覽器直接開啟 | 不需安裝軟體、快速分享或嵌入 |

不替使用者決定；使用者要求兩種都做時依序完成。

## 3. 規劃報表

依使用者提供的規格（截圖、既有報表、文字描述）規劃；沒有規格時依欄位分類提出版面。
**一次**向使用者確認以下內容後再產生檔案：

- 視覺清單：每個視覺的類型（KPI 卡片、長條圖、折線圖、圓餅圖、表格等；PBIP 路徑只用
  [PBIP 格式](references/pbip.md) 視覺表內的類型）、使用的區塊與欄位、
  排序、數值格式（百分比、千分位、小數位、單位）。
- 參數：pview 的每個 `variable_name` 對應報表上的哪個輸入，型別與預設值；
  時間區間沿用 pview 的 `[開始, 結束)` 語意，並在報表上註明結束日不含。
- 視覺與欄位對照依下列規則，不另外發明運算：
  - 視覺直接呈現 pview 回傳的值；BI 端只允許篩選區塊、排序與格式化。
  - 需要跨列合計時，只能加總可加總量值，比率以「分子合計 ÷ 分母合計」計算。
  - 不可加總量值只在篩選後恰好一列時顯示；多列時顯示空白並註明，
    **不做 SUM、AVERAGE 或加權**。
  - 維度類別的 Top N、「其他」等呈現規則已在 mview 決定，BI 不再截斷或重新分組。

## 4. 確認資料 API

1. 請使用者提供自行建立的資料 API URL；尚未建立時說明需先在 Plasma 建立，本 skill 不代建。
2. 核對 URL 的查詢參數名稱與 pview 的 `variable_name` 一一對應。
3. **確認實際回應格式**：徵得使用者同意後，以代表性參數呼叫一次 API（例如宿主的 shell `curl`），
   或請使用者貼上一份回應範例。記錄最上層結構、欄位名稱、型別、是否有分頁或截斷欄位。
   不憑 `execute_pview` 的回應形狀或慣例猜測 API 格式。
4. 回傳列數低於上限且一次取得完整結果才繼續；需要分頁才能取完時回到第 1 節第 2 點。

## 5. 產生檔案

- **路徑 1**：依 [PBIP 格式](references/pbip.md) 產生專案資料夾。檔案結構、`$schema` 版本、
  `visualType` 與角色名稱都以該文件為準，不上網搜尋；文件未列的視覺依其中的替代規則處理。
- **路徑 2**：依 [HTML BI 格式](references/html.md) 產生單一 HTML 檔。

**產生的檔案不寫任何註解**：HTML 不用 `<!-- -->`，CSS、JavaScript、DAX、M 不用 `//`、`/* */`，
TMDL 不用 `///` 描述。

輸出位置使用使用者指定的目錄；未指定時放在目前工作目錄，以報表名稱命名。
目標已存在時先查看內容，不直接覆寫；需覆寫時先取得使用者同意。

## 6. 驗證與交付

驗證能在本機完成的部分，無法驗證的項目如實列出，不宣稱已在 Power BI Desktop 或瀏覽器中測試：

- **PBIP**：所有 JSON 可解析且 `$schema` 正確；TMDL 以 Tab 縮排、UTF-8 無 BOM；
  視覺引用的資料表、欄位、量值名稱都存在於 TMDL；資料夾與物件名稱只含英數、底線、連字號。
  Power BI Desktop 僅支援 Windows，開啟與重新整理需由使用者完成。
- **HTML**：以本機可用工具檢查 JavaScript 語法；用第 4 節取得的回應範例驗證解析與繪圖邏輯
  （透過頁面的匯入 JSON 功能，不把資料寫死在檔案）。

交付時列出：

- 來源 pview 名稱與 ID、其讀取的 mview、workspace。
- 產生的檔案樹與開啟方式。
- 視覺對照表：

  | 視覺 | 類型 | 區塊／欄位 | 計算方式 | 格式 |
  |---|---|---|---|---|
  | Total Admissions | KPI 卡片 | `kpi`／`total_admissions` | 單列直接顯示 | `#,0` |

- 參數與 API 查詢參數的對應、結束日不含的說明。
- 不可加總指標的限制、API 權杖處理方式，以及使用者尚需完成的步驟
  （例如 Power BI 填參數、設定匿名認證並重新整理；HTML 填入 API URL）。
