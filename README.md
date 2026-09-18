# Plasma workflows

供 **ChatGPT 網頁版與 Claude** 使用的共用工作流程 plugin。
以 Ophion 知識查核來源與 SQL，建立 view／manual mview 並核對定義後結束。
兩種宿主共用同一份 skills，分別提供原生 manifest 與 ZIP。

唯一流程：**查核來源 → 驗證 SQL → 建立 view／mview → 核對定義並交付**。

**Plugin 是工作流程；遠端 MCP gateway 是工具服務；MCP client 由宿主提供。**
安裝 plugin 不會自動建立 MCP 連線或授予服務權限。

```text
ChatGPT / Claude
├─ Plasma plugin：共用 skills
└─ 宿主的 MCP client ── HTTPS + OAuth ──> plasma-backend /mcp
                                         ├─ Plasma REST API
                                         └─ Ophion 知識工具
```

## 功能與平台相容性

| Skill | 用途 |
|---|---|
| `plasma-mcp-setup` | 連結服務、核對 workspace／scopes、診斷 |
| `ophion-knowledge-lookup` | 查核來源、欄位、代碼、業務規則 |
| `plasma-create-view` | 建立 view／manual mview，核對定義後結束 |

兩個版本均無本機 hooks、Bash launcher、Go binary、下載快取、環境變數或狀態檔依賴。
`scripts/` 僅供維護者封裝與測試，不會放進安裝包；使用者不需要 Python 或 Go。
歷史 `releases/` 記錄舊版本，不代表目前仍提供那些功能。

全程台灣繁體中文。查核、驗證與建立工作依使用者交付範圍連續完成，宿主權限仍適用。
一般 view 不傳同步設定；mview 使用 `sync_mode=manual`，建立後不啟動同步。
不建立 blueprint、不匯出檔案或外部資料庫、不發布資料 API，也不安排同步排程。
後端若仍暴露其他工具，本 plugin 不把它們納入流程；伺服器工具權限由後端管理。

## ChatGPT 網頁版

### 先連結 Plasma 工具

管理員需提供可達的 HTTPS MCP endpoint，例如 `https://mcp.example.com/mcp`。
在帳號及工作區政策允許時，開啟 developer mode，在 Plugins 建立遠端 MCP 連線，
完成 OAuth 登入、選 workspace、同意 scopes，並將連線加入對話。
已由管理員配置 Plasma app 時，直接連結該 app。以目前產品 UI 為準。

開始使用後先呼叫 `whoami`，確認 workspace、scopes 與知識服務狀態。
詳見 [官方連線測試指南](https://developers.openai.com/plugins/deploy/connect-chatgpt)。

### 安裝 workflows

- **工作區 GitHub 匯入**：支援此能力的工作區由管理員在 Admin → Plugins 匯入本
  repository；現有 Claude-compatible marketplace 可供匯入。成員仍須另外連結
  Plasma app，並在對話啟用。匯入不代表已授權後端。
- **ChatGPT 原生封裝**：`make release` 產生 `plasma-plugin_0.3.1_chatgpt.zip`，
  內含 `.codex-plugin/plugin.json` 與三份 skills，供支援該格式的安裝／匯入流程使用。
  這是未綁定 app 的 workflow 包，不是已上架或已完成工具連線的 plugin。
- **綁定既有工作區 app**：取得真實 app ID 後，執行：

  ```bash
  python3 scripts/package_plugin.py --app-id "$PLASMA_CHATGPT_APP_ID"
  ```

  環境變數僅用來將真實 ID 傳給維護者封裝指令，執行 plugin 不需要它。
  會產生額外的 `plasma-plugin_0.3.1_chatgpt-linked.zip`，包含 `.app.json` 與
  manifest 的 app 引用，且不修改 repository 的共用 manifest。
  支援 ID 前綴 `asdk_app_`、`connector_`、`templated_apps_`；不能使用 `plugin_` ID。
  此指令只做封裝，不會註冊 app、驗證其存在、安裝或授予權限。
- **公開 plugin 發布**：使用 OpenAI 的 **With MCP** 流程提交正式 endpoint，並在
  同一 draft 加入 skills；不能以 skills-only 提交代替本專案需要的 MCP 整合。
  `.app.json` 的工作區引用也不能代替公開 MCP 提交。

不要為了綁定網頁版工具而新增 `.mcp.json`、`mcp.json` 或 inline `mcpServers`：
官方工作區匯入會把這類 plugin 標示為 Desktop only，即使 URL 是 HTTPS。
參考 [官方工作區 plugin 管理](https://learn.chatgpt.com/docs/enterprise/plugin-management)
與 [Claude plugin 移植／提交指南](https://developers.openai.com/plugins/guides/submit-claude-plugin)。

目前沒有預填 endpoint 或 app ID。完整連線與寫入流程仍須完成後端項目並在真實
ChatGPT 帳號驗收，見 [後端修改清單](docs/backend-integration.md)。

## Claude Code

保留既有 plugin 名稱與 marketplace，方便原使用者更新：

```text
/plugin marketplace add BrobridgeOrg/plasma-agent-plugin
/plugin install plasma-plugin@plasma-plugin-local
```

透過 Claude Code 加入同一個遠端 MCP server（將 URL 換成真實部署）：

```bash
claude mcp add --transport http plasma https://mcp.example.com/mcp
```

完成授權後開新 session，先呼叫 `whoami`。不再安裝或執行任何 plugin binary。
Claude 安裝包為 `plasma-plugin_0.3.1_claude.zip`；保留相同的三份 skills，無 hooks。
其他 Claude 介面的 plugin 安裝能力以該產品為準，這裡的安裝指令專供 Claude Code。

## 升級自舊版

更新 plugin 後重新開啟對話／session，讓宿主移除舊 hook 註冊及舊版 skills。
`plasma-data-api` 與 `plasma-export` 已移除，統一使用 `plasma-create-view`。
不要沿用舊對話載入的匯出／發布指示；新包中只有三份 skills。
若曾手動將舊 hook 複製到宿主設定，請在該宿主刪除該自訂設定；新版不會執行舊 binary。
本次不自動修改使用者家目錄、既有 MCP 連線或其他宿主設定。
workspace 綁定於後端授權；換 workspace 要重新授權，plugin 不保存選擇。

## 從 GitHub Actions 取得安裝包

在 GitHub 開啟 **Actions → Package plugins → Run workflow**，選擇要封裝的分支，
`tag` 留空即可。完成後在該次執行頁面的 **Artifacts** 下載
`plasma-plugin-<版本>`，解壓縮後可取得 ChatGPT ZIP、Claude ZIP 與
SHA-256 `checksums.txt`。Artifact 保留 30 天。

手動執行且 tag 留空只產生安裝包，不建立 GitHub Release。
推送 `v*` tag，或手動指定既有 tag，會封裝該 tag 的程式，並同時將安裝包附加到
新建的 GitHub Release；tag 必須與 `VERSION` 及兩份 manifest 一致。
工作流程檔須先推送到 GitHub 預設分支，才會顯示手動執行入口。

## 維護者本機驗證

```bash
make check
make release
```

僅需 Python 3.10+；封裝使用標準函式庫。產物在 `dist/v0.3.1/`，包括兩個 ZIP
與 SHA-256 `checksums.txt`。安裝包只收錄對應宿主 manifest 與共用 Markdown skills，
避免將開發工具、本機功能或後端修改清單帶入執行環境。

更新 `VERSION`、兩份 manifest 與對應的 `releases/vX.Y.Z.md`，再依團隊流程提交
及推送 tag。正式安裝包由 GitHub Actions 驗證、封裝並上傳，不再跨平台編譯 binary。
本機產生 ZIP 不會自動推送 tag、發 GitHub Release 或上架 ChatGPT。
