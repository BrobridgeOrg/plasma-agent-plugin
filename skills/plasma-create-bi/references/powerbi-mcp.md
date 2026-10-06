# 以 Power BI MCP 完成 PBIP

本文件用於 Power BI 路徑。先讀宿主當次提供的工具 schema 與 `Help`，以實際能力
決定建模方式；不要把本次安裝位置或連線名稱寫成下次工作的固定設定。
這是跨宿主流程，一般工作使用 MCP 與檔案工具完成。

## 能力與連線

- 本機 Power BI Authoring MCP 的工具可建立／修改語意模型、資料表、量值、M 參數與
  partition，並提供 TMDL 匯入／匯出。若沒有 report/page/visual 操作，PBIR 由
  [PBIP 格式](pbip.md) 補齊；不要杜撰視覺工具或只交付語意模型。
- 同一次建模選定一個 server；不要混用 local 與 hosted Power BI Authoring server。
- 新建本機專案可先寫最小 TMDL，再以 `connection_operations.ConnectFolder` 連到
  `<Name>.SemanticModel/definition`，或用 `database_operations.Create` 建立離線模型。
  `ConnectFolder` 的目錄須有 `database.tmdl`，或含有該檔案的 `definition` 子目錄。
- `ConnectFolder`／`Connect` 的 connectionName 由工具產生，不在連線請求指定；
  保存回傳名稱，後續每次模型操作都明確帶入，避免操作到其他 agent 的最後連線。
  重連同一資料夾可能產生帶 ` 2` 等後綴的新名稱；驗收須用新連線，舊離線連線可能
  仍持有匯出前的模型快照。
- 不同 agent／工具 session 可能看不到彼此的 MCP 連線。交接已保存的 TMDL 路徑，
  驗收 agent 自行 `ConnectFolder`；收到 `Connection not found` 時先查本 session
  的連線，不重建整個資料庫，也不假設對方尚未完成建模。
- `database_operations.Create` 可能回傳 `content: []`、`isError: false`。此時以
  `ListConnections` 核對剛建立的模型及連線，不把空內容當失敗而重複建立。
- 修改既有 Desktop 模型前，先 `ListLocalInstances` 並核對其是否為本次報表；不能只
  因有一個執行個體就改動它。離線模型不等於正在執行的 Analysis Services 模型。
- 若 MCP 提供其他可執行模型連線，依實際工具核對目標與權限後使用；只有離線來源時，
  以定義查核完成 PBIP 交付，請使用者開啟後自行重新整理。

## 建模、保存與報表

1. API 契約確認後，建立 M 連線參數與查詢。使用可用的 named expression、table、
   column、partition、measure 操作；批次工具依 schema 傳 definitions／references。
   schema 不完整時先讀 `Help`，不要用猜測的 property 名稱反覆重試。
   `named_expression_operations.CreateParameter` 明確傳 `kind: "M"`；曾實測 Help
   宣稱可自動設定，但省略後回覆 `Kind is required for creating named expressions`。
   日期參數傳完整 `#date(...) meta [IsParameterQuery=true, Type="Date", IsParameterQueryRequired=true]`
   並讀回確認，不能依賴工具預設（可能是 `Type="Text"`）。
2. 支援長表 `metric_code`／`metric_value`：每個 KPI 先篩選自己的代碼，再檢查
   恰好一列才回傳值；不能對整張 KPI 表 SUM，也不能用單純 SELECTEDVALUE 隱藏
   多筆同值資料。比例、平均與去重指標不重新聚合。保留來源 null。
3. 使用本機 API 憑證時避免把完整 expression 回顯。可控制回傳長度的匯出操作使用
   `maxReturnCharacters=0`；驗證只讀物件名稱、型別、數量及必要的量值運算式。
   不在 log、skill、可分享模型副本中保留真實 URL／權杖。
4. 完成操作後使用 `database_operations.ExportToTmdlFolder` 保存到目標 definition
   目錄，再重新載入核對；不要把 MCP 記憶體內的變更當成已寫入磁碟。
   `ConnectFolder` 已實測會警告變更不自動寫盤，必須匯出後再驗收。補齊 `.pbip`、`.pbism`、`.pbir`、
   PBIR 頁面／視覺，報表引用使用相對路徑。
5. 產物內不加入測試結果作為靜態正式資料源；Power Query 保持讀取 API。
   日期標題由載入期別推導，不把預設月份永久寫死在標題。
   匯出 TMDL 只保存模型定義，不等於 Desktop 已保存資料快取；不要因此宣稱離線
   重開即有資料。已有實際刷新但無 Desktop 保存能力時，交付完整定義並說明重開需
   重新整理，現有工作階段可由使用者保存快取。

## 驗證與錯誤處理

- **API**：至少核對兩個月份的期別、列數和代表性指標；測空結果，M 查詢仍保有完整
  欄位結構。有錯誤或截斷時明確失敗，不把回應解析成一張看似有效的空表。
- **檔案**：JSON 解析及可取得的 schema 驗證；PBIR 欄位／量值引用、排序、視覺邊界、
  參數與 API 契約一致。將使用者每項需求對到實際視覺，不以檔案數推斷完成。
- **模型**：MCP 重新連線保存後的 TMDL，列出 tables／columns／measures，核對名稱、
  型別、運算式與格式。讀回含憑證的來源時先遮罩。
- **執行**：先核對連線是否有執行引擎、工具是否支援及認證是否就緒。離線連線只做
  定義查核；刷新、DAX Validate 與 Execute 使用具備執行引擎的連線。
  有本次可執行模型時，直接使用 MCP refresh 與 DAX Execute 比對來源 KPI、
  圖表合計及無資料時的空值。明確區分 DAX Validate 與實際 Execute 的證據。
  變更月份後以 DAX 讀回每張表的期別及代表數值；工具回空 success 不足以證明刷新。
  曾遇到 table refresh 回成功但仍保留舊月，改用 `partition_operations.RefreshWithXMLA`
  的 `refreshDefinitions: [{tableName, partitionName, refreshType: "Full"}]` 才更新。
  依當次 schema 選對刷新層級，逐表核對後恢復使用者指定的預設月，再刷新並匯出。
  空月可保留具完整欄位的空表，或明確回報無資料錯誤；採後者時須說明刷新失敗可能
  保留舊資料，畫面期別由已載入資料推導，不能把新參數值當成刷新成功的標題。
- **視覺與互動設定**：核對 PBIR 的欄位引用、完整數字格式、版面邊界、長類別軸寬、
  單選月份及跨圖表互動；MCP 有相關讀取／檢查能力時直接使用。完成這些設定查核
  即可交付。檔案、DAX 與畫面檢查各有不同範圍，紀錄以實際完成的操作為準。
- 錯誤先判定為連線、工具參數、TMDL、API 契約、刷新或 PBIR 哪一層，再修正該層。
  遇到本機權限／登入介面阻擋時保留檔案並說明；不改動無關模型、不發布到 Fabric，
  不將私有 API 或資料上傳到外部服務來繞過。

完成可用的 API、檔案與 MCP 模型查核後交付專案；刷新環境未就緒時，請使用者
開啟後按「重新整理」。檢查紀錄留在專案說明，不在一般交付列出「尚未驗證」清單。
實際錯誤仍須修正或簡短告知；已刷新的執行模型不代表匯出的 PBIP 含有資料快取。
