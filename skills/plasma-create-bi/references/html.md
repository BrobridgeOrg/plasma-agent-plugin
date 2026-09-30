# HTML BI 格式

產生**單一 `.html` 檔**，瀏覽器直接開啟即可使用，不需要建置工具或伺服器。
資料在執行時向使用者的資料 API 取得；檔案本身不內嵌資料列。

## 頁面組成

```text
┌ 標題列：報表名稱、來源 pview、資料更新時間（API 回傳時）
├ 參數列：每個 pview 參數一個輸入（日期用 <input type="date">）＋「查詢」按鈕
├ 狀態列：載入中／筆數／錯誤訊息
├ KPI 卡片列
└ 圖表區（長條、折線、表格）
設定面板：API URL、匯入 JSON 檔
```

## 檔案結構

```html
<!doctype html>
<html lang="zh-Hant">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title><報表名稱></title>
  <style>/* 色彩以 :root 變數定義，並提供 prefers-color-scheme: dark 版本 */</style>
  <!-- 圖表庫：使用一個固定版本的 CDN，例如 ECharts；或以原生 SVG 繪製 -->
</head>
<body>
  ...
  <script>
    const CONFIG = {
      apiUrl: "",                 // 預設空白；使用者明確要求才寫入實際 URL
      params: [                   // 依 pview param_def
        { name: "start_date", label: "開始日期（含）", type: "date", default: "2026-01-01" },
        { name: "end_date",   label: "結束日期（不含）", type: "date", default: "2026-02-01" }
      ],
      rowLimit: 1000              // 下游單次回傳上限
    };
    // parseResponse / loadData / render* 函式
  </script>
</body>
</html>
```

- 圖表庫只用一個，指定固定版本，從 cdnjs 或 jsdelivr 載入；離線環境改用原生 SVG。
- 版面在手機寬度可用：KPI 卡片自動換行，圖表寬度隨容器縮放，不出現水平捲動。
- 數值格式用 `Intl.NumberFormat('zh-TW', ...)`，百分比、千分位、小數位依規劃。

## 資料取得

```js
function buildUrl(base, values) {
  const url = new URL(base);
  for (const [k, v] of Object.entries(values)) url.searchParams.set(k, v);
  return url;
}

function parseResponse(json) {
  // 依 SKILL.md 第 4 節確認的實際回應格式撰寫，例如：
  // { data: { columns: [...], rows: [[...]] } }
  const { columns, rows } = json.data;
  return rows.map(r => Object.fromEntries(columns.map((c, i) => [c, r[i]])));
}
```

- 送出前檢查必填參數與 `開始 < 結束`，錯誤在狀態列顯示，不送出請求。
- 回傳列數達到 `rowLimit` 或回應帶截斷／分頁標記時，顯示「結果可能不完整」並停止繪圖，
  不自行翻頁補齊。
- API URL 存在該瀏覽器的 `localStorage`（以 try/catch 包住，失敗時仍可在本次使用），
  不寫回檔案。
- **CORS**：從本機檔案或其他網域呼叫 API，需要 API 回應允許跨來源。瀏覽器擋下時，
  狀態列說明原因，提示改用「匯入 JSON 檔」或請管理員設定 CORS；不建議關閉瀏覽器安全設定。
- 「匯入 JSON 檔」讀取使用者自行下載的 API 回應，經同一個 `parseResponse` 處理。

## 呈現規則

對應 SKILL.md 第 3 節：

- 依 `section` 等區塊欄位把資料分組，每個視覺只讀自己的區塊。
- 可加總量值跨列時用加總；比率用「分子合計 ÷ 分母合計」。
- 不可加總量值只在該區塊恰好一列時顯示，多列時顯示「—」並以提示說明原因。
- 維度的排序、Top N 依 mview 回傳結果，不在前端重新分組或截斷。
- 空結果顯示「此區間沒有資料」，不顯示 0。

## 本機檢查

- 將 `<script>` 內容取出，以本機可用的 JavaScript 工具檢查語法（例如 `node --check`）。
- 用第 4 節取得的回應範例，透過匯入 JSON 功能確認解析、分組與格式化結果；
  範例資料不留在交付檔案中。
- 無法在瀏覽器實際開啟時，交付時列為未驗證。

## 分享注意事項

- 檔案不含 API URL 時可直接分享，使用者開啟後自行填入。
- 使用者要求寫入 URL 時，提醒取得檔案的人都能用權杖讀取資料，不要上傳到公開位置。
