# Power BI 專案（PBIP）格式

產生 Power BI Desktop 可直接開啟的 PBIP 專案：語意模型用 TMDL，報表用 PBIR。
所有檔案不寫任何註解：TMDL 不用 `///`，DAX 與 M 不用 `//`、`/* */`。
格式依 Microsoft Learn「Power BI Desktop projects」文件與
[microsoft/json-schemas](https://github.com/microsoft/json-schemas/tree/main/fabric) 公開 schema。
下表版本為 2026-09 核對的最新版，直接使用即可，不需上網查詢；Desktop 也接受較舊版本
（官方樣本使用 visualContainer 2.4.0、report 3.0.0）。

| 檔案 | `$schema` |
|---|---|
| `*.pbip` | `.../fabric/pbip/pbipProperties/1.0.0/schema.json` |
| `definition.pbir` | `.../fabric/item/report/definitionProperties/2.0.0/schema.json` |
| `definition.pbism` | `.../fabric/item/semanticModel/definitionProperties/1.0.0/schema.json` |
| `.platform` | `.../fabric/gitIntegration/platformProperties/2.1.0/schema.json` |
| `definition/version.json` | `.../fabric/item/report/definition/versionMetadata/1.0.0/schema.json` |
| `definition/report.json` | `.../fabric/item/report/definition/report/3.3.0/schema.json` |
| `pages/pages.json` | `.../fabric/item/report/definition/pagesMetadata/1.1.0/schema.json` |
| `page.json` | `.../fabric/item/report/definition/page/2.1.0/schema.json` |
| `visual.json` | `.../fabric/item/report/definition/visualContainer/2.12.0/schema.json` |

`...` 為 `https://developer.microsoft.com/json-schemas`。

## 資料夾結構

`<Name>` 使用英數、底線或連字號，整體路徑盡量短（Windows 預設 260 字元上限）。

```text
<Name>/
├── <Name>.pbip
├── .gitignore
├── <Name>.SemanticModel/
│   ├── .platform
│   ├── definition.pbism
│   └── definition/
│       ├── database.tmdl
│       ├── model.tmdl
│       ├── expressions.tmdl
│       └── tables/
│           └── <Table>.tmdl
└── <Name>.Report/
    ├── .platform
    ├── definition.pbir
    └── definition/
        ├── version.json
        ├── report.json
        └── pages/
            ├── pages.json
            └── <page>/
                ├── page.json
                └── visuals/
                    └── <visual>/
                        └── visual.json
```

- 全部檔案 UTF-8 無 BOM。TMDL 以 **Tab** 縮排。
- 不產生 `.pbi/cache.abf`；Desktop 開啟後以定義建立空模型，重新整理才載入資料。
- page、visual 的 `name` 與資料夾名稱相同，只含英數、底線、連字號，page 名稱最多 50 字元。
- `lineageTag`、`logicalId` 使用新產生的 GUID，每個物件各一個。

## 專案與項目檔

`<Name>.pbip`：

```json
{
  "$schema": "https://developer.microsoft.com/json-schemas/fabric/pbip/pbipProperties/1.0.0/schema.json",
  "version": "1.0",
  "artifacts": [{ "report": { "path": "<Name>.Report" } }],
  "settings": { "enableAutoRecovery": true }
}
```

`.gitignore`：

```text
**/.pbi/localSettings.json
**/.pbi/cache.abf
```

`<Name>.Report/definition.pbir`（`version` 4.0 以上才支援 PBIR 的 `definition/` 資料夾）：

```json
{
  "$schema": "https://developer.microsoft.com/json-schemas/fabric/item/report/definitionProperties/2.0.0/schema.json",
  "version": "4.0",
  "datasetReference": { "byPath": { "path": "../<Name>.SemanticModel" } }
}
```

`<Name>.SemanticModel/definition.pbism`（`version` 4.0 以上才支援 TMDL）：

```json
{
  "$schema": "https://developer.microsoft.com/json-schemas/fabric/item/semanticModel/definitionProperties/1.0.0/schema.json",
  "version": "4.0",
  "settings": {}
}
```

`.platform`（Report 與 SemanticModel 各一份，`type` 分別為 `Report`、`SemanticModel`）：

```json
{
  "$schema": "https://developer.microsoft.com/json-schemas/fabric/gitIntegration/platformProperties/2.1.0/schema.json",
  "metadata": { "type": "Report", "displayName": "<Name>" },
  "config": { "version": "2.0", "logicalId": "<GUID>" }
}
```

## 語意模型（TMDL）

`database.tmdl`：

```text
database
	compatibilityLevel: 1600
```

`model.tmdl`。`discourageImplicitMeasures` 讓使用者只能用定義好的量值，
避免把不可加總指標拖進視覺後被自動 SUM：

```text
model Model
	culture: zh-TW
	defaultPowerBIDataSourceVersion: powerBI_V3
	discourageImplicitMeasures
	sourceQueryCulture: zh-TW
```

### 參數與共用查詢（`expressions.tmdl`）

pview 的每個參數各一個 Power Query 參數；API 位址拆成基底 URL 與權杖，
讓 `Web.Contents` 的第一個參數是固定字串，Power BI Service 才能排程重新整理。
權杖預設寫占位值，使用者明確要求才寫入實際值。

```text
expression ApiBaseUrl = "https://<host>/apis/access_entry/" meta [IsParameterQuery=true, Type="Text", IsParameterQueryRequired=true]
	lineageTag: <GUID>

expression ApiToken = "<請填入資料 API 權杖>" meta [IsParameterQuery=true, Type="Text", IsParameterQueryRequired=true]
	lineageTag: <GUID>

expression StartDate = #date(2026, 1, 1) meta [IsParameterQuery=true, Type="Date", IsParameterQueryRequired=true]
	lineageTag: <GUID>

expression EndDate = #date(2026, 2, 1) meta [IsParameterQuery=true, Type="Date", IsParameterQueryRequired=true]
	lineageTag: <GUID>

expression PlasmaData =
		let
		    Source = Web.Contents(ApiBaseUrl, [
		        RelativePath = ApiToken,
		        Query = [
		            start_date = Date.ToText(StartDate, "yyyy-MM-dd"),
		            end_date = Date.ToText(EndDate, "yyyy-MM-dd")
		        ]
		    ]),
		    Json = Json.Document(Source),
		    Raw = Table.FromRows(Json[data][rows], Json[data][columns]),
		    Typed = Table.TransformColumnTypes(Raw, {
		        {"period_start", type date},
		        {"section", type text},
		        {"total_admissions", Int64.Type},
		        {"mortality_rate", type number}
		    })
		in
		    Typed
	lineageTag: <GUID>
```

- `Query` 的欄位名稱必須等於 API 的查詢參數名稱（通常即 pview 的 `variable_name`）。
- 參數預設值使用使用者確認的值，不留任意測試日期。
- `Raw` 這行依 SKILL.md 第 4 節確認的**實際回應格式**撰寫，例如：
  - `{"data":{"columns":[...],"rows":[[...]]}}` → `Table.FromRows(Json[data][rows], Json[data][columns])`
  - `[{"col":...}, ...]` → `Table.FromRecords(Json)`
  - `{"rows":[{"col":...}]}` → `Table.FromRecords(Json[rows])`
- `Typed` 列出每個欄位的型別，依 pview 欄位表；日期欄位若回傳含時區字串，
  先 `DateTimeZone.FromText` 再取日期，並註明時區。
- `PlasmaData` 不載入模型，只供下列資料表引用。

### 資料表（`tables/<Table>.tmdl`）

每個區塊（`section`）一個資料表，在 Power Query 以 `section` 篩選，避免在每個視覺設定篩選：

```text
table KPI
	lineageTag: <GUID>

	measure 'Total Admissions' = SUM(KPI[total_admissions])
		formatString: #,0
		lineageTag: <GUID>

	measure 'Mortality Rate' = DIVIDE(SUM(KPI[death_cnt]), SUM(KPI[adm_cnt]))
		formatString: 0.00%
		lineageTag: <GUID>

	measure 'Unique Patients' = IF(COUNTROWS(KPI) = 1, SELECTEDVALUE(KPI[distinct_patients]))
		formatString: #,0
		lineageTag: <GUID>

	column period_start
		dataType: dateTime
		formatString: yyyy-mm-dd
		lineageTag: <GUID>
		summarizeBy: none
		sourceColumn: period_start

	column total_admissions
		dataType: int64
		formatString: #,0
		lineageTag: <GUID>
		summarizeBy: none
		sourceColumn: total_admissions

	partition KPI = m
		mode: import
		source =
				let
				    Source = PlasmaData,
				    Rows = Table.SelectRows(Source, each [section] = "kpi")
				in
				    Rows
```

量值規則（對應 SKILL.md 第 3 節）：

| 欄位分類 | DAX 寫法 |
|---|---|
| 可加總（次數、總和） | `SUM(T[col])` |
| 比率 | `DIVIDE(SUM(T[分子]), SUM(T[分母]))`，mview 需提供分子與分母 |
| 不可加總（去重人數、平均、預先算好的比率） | `IF(COUNTROWS(T) = 1, SELECTEDVALUE(T[col]))`，多列時空白 |

- 所有欄位 `summarizeBy: none`，只透過量值呈現數值。
- `dataType` 使用 `string`、`int64`、`double`、`decimal`、`dateTime`、`boolean`，與 M 的型別一致。
- 資料表、欄位、量值名稱含空白或特殊字元時以單引號包住。

## 報表（PBIR）

`definition/version.json`：

```json
{
  "$schema": "https://developer.microsoft.com/json-schemas/fabric/item/report/definition/versionMetadata/1.0.0/schema.json",
  "version": "2.0.0"
}
```

`definition/report.json`（`themeCollection` 必填；`baseTheme` 照抄官方樣本中 Desktop 寫出的值）：

```json
{
  "$schema": "https://developer.microsoft.com/json-schemas/fabric/item/report/definition/report/3.3.0/schema.json",
  "themeCollection": {
    "baseTheme": {
      "name": "CY19SU12",
      "reportVersionAtImport": { "visual": "1.8.44", "report": "2.0.44", "page": "1.3.44" },
      "type": "SharedResources"
    }
  }
}
```

`pages/pages.json`：

```json
{
  "$schema": "https://developer.microsoft.com/json-schemas/fabric/item/report/definition/pagesMetadata/1.1.0/schema.json",
  "pageOrder": ["overview"],
  "activePageName": "overview"
}
```

`pages/overview/page.json`：

```json
{
  "$schema": "https://developer.microsoft.com/json-schemas/fabric/item/report/definition/page/2.1.0/schema.json",
  "name": "overview",
  "displayName": "總覽",
  "displayOption": "FitToPage",
  "width": 1280,
  "height": 720
}
```

### 視覺（`visuals/<visual>/visual.json`）

**視覺類型與角色名稱以下表為準，不需上網搜尋。** 下表整理自 Microsoft 官方
[microsoft/BCApps](https://github.com/microsoft/BCApps/tree/main/src/Apps/W1/PowerBIReports) 中
Power BI Desktop 實際存出的 PBIR 報表（2026-10 統計 1,023 個 visual.json），
只列出樣本中出現過的類型與角色。`queryState` 的 key 就是角色名稱，大小寫須完全相同。

| 用途 | `visualType` | 角色（`queryState` key） |
|---|---|---|
| KPI 卡片 | `cardVisual` | `Data`（量值） |
| 表格 | `tableEx` | `Values`（欄位與量值依序排列） |
| 矩陣 | `pivotTable` | `Rows`、`Columns`、`Values` |
| 群組直條圖 | `clusteredColumnChart` | `Category`、`Y`、`Tooltips` |
| 群組橫條圖 | `clusteredBarChart` | `Category`、`Y`、`Tooltips` |
| 堆疊直條圖 | `columnChart` | `Category`、`Y`、`Series`、`Tooltips` |
| 堆疊橫條圖 | `barChart` | `Category`、`Y`、`Series`、`Tooltips` |
| 100% 堆疊橫條圖 | `hundredPercentStackedBarChart` | `Category`、`Y` |
| 折線圖 | `lineChart` | `Category`、`Y`、`Series`、`Tooltips` |
| 區域圖 | `areaChart` | `Category`、`Y`、`Tooltips` |
| 直條＋折線組合圖 | `lineClusteredColumnComboChart` | `Category`、`Y`（直條）、`Y2`（折線）、`Tooltips` |
| 緞帶圖 | `ribbonChart` | `Category`、`Y`、`Series` |
| 圓餅圖 | `pieChart` | `Category`、`Y`、`Tooltips` |
| 環圈圖 | `donutChart` | `Category`、`Y`、`Tooltips` |
| 樹狀圖 | `treemap` | `Group`、`Details`、`Values`、`Tooltips` |
| 漏斗圖 | `funnel` | `Category`、`Y`、`Tooltips` |
| 量測計 | `gauge` | `Y`、`MaxValue`、`TargetValue`、`Tooltips` |
| 散佈圖 | `scatterChart` | `Category`（明細）、`Series`、`X`、`Y`、`Size` |
| 交叉分析篩選器 | `slicer` | `Values`（欄位） |
| 文字方塊 | `textbox` | 無 `query` |

- 類別、圖例、明細角色（`Category`、`Series`、`Group`、`Details`、`Rows`、`Columns`）放**欄位**
  （`Column`）；數值角色（`Data`、`Y`、`Y2`、`X`、`Size`、`MaxValue`、`TargetValue`）放**量值**（`Measure`）。
- KPI 卡片使用 `cardVisual`。舊版 `card`（角色 `Values`）仍可開啟，但新報表不使用。
- 需要的視覺不在表內（例如地圖、瀑布圖、自訂視覺）時，**不猜測角色名稱、不上網搜尋**；
  改用表內最接近的類型（地圖改用橫條圖或表格），並在交付時告知使用者可在 Desktop 自行替換。
- 每個角色都可放多個 projection，依陣列順序呈現。

**欄位參照寫法**（`field`）：

```json
{ "Column":  { "Expression": { "SourceRef": { "Entity": "<資料表>" } }, "Property": "<欄位>" } }
{ "Measure": { "Expression": { "SourceRef": { "Entity": "<資料表>" } }, "Property": "<量值>" } }
```

每個 projection 另加 `"queryRef": "<資料表>.<名稱>"` 與 `"nativeQueryRef": "<名稱>"`；
類別欄位可加 `"active": true`。`SourceRef.Entity` 與 `Property` 必須與 TMDL 名稱完全相同。

KPI 卡片範例：

```json
{
  "$schema": "https://developer.microsoft.com/json-schemas/fabric/item/report/definition/visualContainer/2.12.0/schema.json",
  "name": "kpi_total_admissions",
  "position": { "x": 0, "y": 0, "z": 0, "width": 300, "height": 140 },
  "visual": {
    "visualType": "cardVisual",
    "query": {
      "queryState": {
        "Data": {
          "projections": [{
            "field": { "Measure": { "Expression": { "SourceRef": { "Entity": "KPI" } }, "Property": "Total Admissions" } },
            "queryRef": "KPI.Total Admissions",
            "nativeQueryRef": "Total Admissions"
          }]
        }
      }
    }
  }
}
```

圓餅圖範例（類別放欄位、數值放量值，依數值遞減排序）：

```json
{
  "$schema": "https://developer.microsoft.com/json-schemas/fabric/item/report/definition/visualContainer/2.12.0/schema.json",
  "name": "admissions_by_type",
  "position": { "x": 0, "y": 300, "z": 1, "width": 620, "height": 400 },
  "visual": {
    "visualType": "pieChart",
    "query": {
      "queryState": {
        "Category": {
          "projections": [{
            "field": { "Column": { "Expression": { "SourceRef": { "Entity": "ByType" } }, "Property": "admission_type" } },
            "queryRef": "ByType.admission_type",
            "nativeQueryRef": "admission_type",
            "active": true
          }]
        },
        "Y": {
          "projections": [{
            "field": { "Measure": { "Expression": { "SourceRef": { "Entity": "ByType" } }, "Property": "Admissions" } },
            "queryRef": "ByType.Admissions",
            "nativeQueryRef": "Admissions"
          }]
        }
      },
      "sortDefinition": {
        "sort": [{
          "field": { "Measure": { "Expression": { "SourceRef": { "Entity": "ByType" } }, "Property": "Admissions" } },
          "direction": "Descending"
        }],
        "isDefaultSort": true
      }
    },
    "visualContainerObjects": {
      "title": [{ "properties": {
        "show": { "expr": { "Literal": { "Value": "true" } } },
        "text": { "expr": { "Literal": { "Value": "'各入院類型住院人次'" } } }
      } }]
    }
  }
}
```

直條圖、橫條圖、折線圖、環圈圖、漏斗圖的結構與圓餅圖相同，只換 `visualType`
（組合圖再加 `Y2`、堆疊圖可加 `Series`）。

**格式設定**只用以下已在樣本確認的寫法，其餘外觀交給 Desktop 預設主題：

| 設定 | 位置 | 寫法 |
|---|---|---|
| 視覺標題 | `visual.visualContainerObjects.title` | `show`：`"true"`／`"false"`；`text`：`"'標題文字'"` |
| 排序 | `visual.query.sortDefinition` | `direction`：`Ascending`／`Descending`，加 `"isDefaultSort": true` |

- `Literal.Value` 是字串：布林寫 `"true"`，文字外層再包單引號 `"'文字'"`。
- 數值格式（百分比、千分位）寫在 TMDL 量值的 `formatString`，不在 visual.json 設定。
- 視覺位置在頁面範圍內（`x + width ≤ 頁寬`、`y + height ≤ 頁高`），`z` 依疊放順序遞增。
- 不在 visual.json 寫入任何資料值或篩選值。

## 本機檢查

- 以 JSON parser 讀取每個 `.json`、`.pbip`、`.pbir`、`.pbism`、`.platform`。
- 可連網且有 JSON Schema 驗證工具時，依 `$schema` 驗證；沒有時列為未驗證。
- 逐一比對 visual.json 的 `Entity`／`Property` 與 TMDL 名稱。
- 確認 TMDL 沒有空白縮排混用、每個物件都有 `lineageTag`。

## 使用者需完成的步驟（交付時列出）

1. 在 Windows 的 Power BI Desktop 開啟 `<Name>.pbip`。
2. 「轉換資料 → 編輯參數」填入 `ApiToken` 與起訖日期（結束日不含）。
3. 首次連線的認證選「匿名」（權杖已在 URL 路徑中），套用並重新整理。
4. 開啟失敗時，依 Desktop 顯示的檔案與位置回報錯誤，由 agent 修正對應檔案。
5. 權杖會存在 `expressions.tmdl`，提交 Git 或分享專案前先移除。
