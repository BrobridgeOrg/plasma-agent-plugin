# Power BI 專案（PBIP）格式

產生 Power BI Desktop 可直接開啟的 PBIP 專案：語意模型用 TMDL，報表用 PBIR。
格式依 Microsoft Learn「Power BI Desktop projects」文件與
[microsoft/json-schemas](https://github.com/microsoft/json-schemas/tree/main/fabric) 公開 schema。
下表版本為 2026-09 核對的最新版；產生前若可連網，先查該 repo 是否有更新版本並改用。

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

`definition/report.json`（`themeCollection` 必填；留空由 Desktop 套用預設主題）：

```json
{
  "$schema": "https://developer.microsoft.com/json-schemas/fabric/item/report/definition/report/3.3.0/schema.json",
  "themeCollection": {}
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

KPI 卡片（`card`，角色 `Values`）：

```json
{
  "$schema": "https://developer.microsoft.com/json-schemas/fabric/item/report/definition/visualContainer/2.12.0/schema.json",
  "name": "kpi_total_admissions",
  "position": { "x": 0, "y": 0, "z": 0, "width": 300, "height": 140 },
  "visual": {
    "visualType": "card",
    "query": {
      "queryState": {
        "Values": {
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

直條圖（`clusteredColumnChart`，類別 `Category`、數值 `Y`）：

```json
{
  "$schema": "https://developer.microsoft.com/json-schemas/fabric/item/report/definition/visualContainer/2.12.0/schema.json",
  "name": "mortality_by_age",
  "position": { "x": 640, "y": 300, "z": 1, "width": 620, "height": 400 },
  "visual": {
    "visualType": "clusteredColumnChart",
    "query": {
      "queryState": {
        "Category": {
          "projections": [{
            "field": { "Column": { "Expression": { "SourceRef": { "Entity": "ByAge" } }, "Property": "age_group" } },
            "queryRef": "ByAge.age_group",
            "nativeQueryRef": "age_group"
          }]
        },
        "Y": {
          "projections": [{
            "field": { "Measure": { "Expression": { "SourceRef": { "Entity": "ByAge" } }, "Property": "Mortality Rate" } },
            "queryRef": "ByAge.Mortality Rate",
            "nativeQueryRef": "Mortality Rate"
          }]
        }
      }
    }
  }
}
```

- 常用 `visualType` 與角色：`card`（`Values`）、`clusteredColumnChart`／`clusteredBarChart`
  （`Category`、`Y`）、`lineChart`（`Category`、`Y`）、`tableEx`（`Values`）。
  其他視覺的角色名稱不確定時，先查官方 schema 或樣本；仍無法確認就改用上述視覺，不猜測。
- `SourceRef.Entity` 與 `Property` 必須與 TMDL 的資料表、欄位、量值名稱完全相同。
- 視覺位置在頁面範圍內（`x + width ≤ 頁寬`、`y + height ≤ 頁高`），依規劃的版面排列。
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
