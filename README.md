# Plasma workflows

供 **opencode、Claude Code 與 Codex** 使用的共用工作流程 plugin。
以 Ophion 知識查核來源與 SQL，建立 view／manual mview 並核對定義後結束。
三種宿主共用同一份 skills，分別提供對應的 manifest／設定檔與 ZIP。

唯一流程：**查核來源 → 驗證 SQL → 建立 view／mview → 核對定義並交付**。

**Plugin 是工作流程；遠端 MCP gateway 是工具服務；MCP client 由宿主提供。**
Codex 與 Claude plugin 直接內含 `.mcp.json`，安裝後由宿主載入連線，再完成 OAuth。
公開 MCP endpoint 為 `https://plasma-mcp.bbg-x.top/mcp`，不需另外填 URL 或安裝 client。

```text
opencode / Claude Code / Codex
├─ Plasma plugin：共用 skills
└─ 宿主的 MCP client ──── OAuth ────> plasma-backend /mcp
                                      ├─ Plasma REST API
                                      └─ Ophion 知識工具
```

連線一律走 **OAuth**：使用 plugin 提供的 MCP 連線，由宿主開啟瀏覽器完成授權。
使用者以自己的 Plasma 帳號登入、選 workspace，**選定即完成授權**——沒有額外的權限
確認頁，每次授權都取得該部署支援的完整權限。token 由宿主保管並自動更新，
不需要手動核發或複製任何憑證。

| 宿主 | 設定 | 授權 |
|---|---|---|
| **opencode** | `opencode.json` 的 `mcp` 區塊 | `opencode mcp auth plasma` |
| **Claude Code** | plugin 內含 `.mcp.json` | 對話中 `/mcp` → plugin 的 plasma 連線 → Authenticate |
| **Codex** | plugin 內含 `.mcp.json` | 「幫我登入 Plasma」，或在 plugin 的 MCP 連線選 Authenticate |

Endpoint 已公開，不再有僅內網可達的限制。ChatGPT 網頁版是否接受此 plugin 的 MCP
宣告、OAuth 及匯入方式，仍依宿主支援與 workspace 政策而定，需另行驗收。

### 部署前提

OAuth 的最後一步是從 gateway 導回 `127.0.0.1` 的本機接收埠。**gateway 必須提供
用戶端信任的 HTTPS 憑證**，否則：

- Chrome 142 之後會擋下這個跨網段導轉，且不顯示任何提示（要求該權限的資格僅限
  HTTPS 頁面），使用者選完 workspace 後只會看到頁面不動
- 部分宿主的 MCP SDK 會直接拒絕非 TLS 位址上的 OAuth token endpoint

自簽而未將 CA 佈到用戶端不算受信任。詳見
[後端修改清單](docs/backend-integration.md)。

2026-09-22 測試：公開 `/mcp` 接受 Streamable HTTP 請求並回傳未授權 `401`，但
`WWW-Authenticate` 的 resource metadata URL 及 OAuth discovery 的 issuer／endpoints
仍指向 `https://mcp.192.168.1.120.nip.io`。後端需更新公開 base URL 與 discovery，
才能驗收外網使用者的 OAuth；plugin 不自行替換伺服器回傳的授權網址。

## 功能與平台相容性

| Skill | 用途 |
|---|---|
| `plasma-mcp-setup` | 連結服務、核對 workspace／scopes、診斷 |
| `ophion-knowledge-lookup` | 查核來源、欄位、代碼、業務規則 |
| `plasma-create-view` | 建立 view／manual mview，核對定義後結束 |

三個版本均無本機 hooks、Bash launcher、Go binary、下載快取或狀態檔依賴。
`scripts/` 僅供維護者封裝與測試，不會放進安裝包；使用者不需要 Python 或 Go。
歷史 `releases/` 記錄舊版本，不代表目前仍提供那些功能。

全程台灣繁體中文。查核、驗證與建立工作依使用者交付範圍連續完成，宿主權限仍適用。
一般 view 不傳同步設定；mview 使用 `sync_mode=manual`，建立後不啟動同步。
不建立 blueprint、不匯出檔案或外部資料庫、不發布資料 API，也不安排同步排程。
後端若仍暴露其他工具，本 plugin 不把它們納入流程；伺服器工具權限由後端管理。

## opencode

skills 沒有 marketplace，直接放進 opencode 掃描的目錄；連線走 OAuth：

```bash
mkdir -p ~/.config/opencode/skills
cp -R skills/* ~/.config/opencode/skills/
```

`~/.config/opencode/opencode.json` 合併套件內的連線設定（URL 已預設）：

```json
{
  "mcp": {
    "plasma": {
      "type": "remote",
      "url": "https://plasma-mcp.bbg-x.top/mcp",
      "enabled": true
    }
  }
}
```

```bash
opencode mcp auth plasma      # 開瀏覽器登入、選 workspace，選定即完成
opencode mcp list             # 查看授權狀態
```

token 由 opencode 保管在 `~/.local/share/opencode/mcp-auth.json` 並自動更新，
使用者不需要保存任何字串。完整步驟見安裝包內的 `INSTALL.md`。

## Claude Code

安裝 workflows（保留既有 plugin 名稱與 marketplace，方便原使用者更新）：

```text
/plugin marketplace add BrobridgeOrg/plasma-agent-plugin
/plugin install plasma-plugin@plasma-plugin-local
```

安裝後宿主載入 `.mcp.json`，不需另行 `claude mcp add`。
在對話中輸入 `/mcp`，選 plugin 的 plasma 連線 → Authenticate，瀏覽器完成授權。
token 存進系統憑證庫並自動更新。

Claude 安裝包為 `plasma-plugin_0.6.0_claude.zip`；三個版本的 skills 完全相同，無 hooks。
其他 Claude 介面的 plugin 安裝能力以該產品為準，這裡的安裝指令專供 Claude Code。

## Codex

使用 `plasma-plugin_0.6.0_chatgpt.zip`，依宿主的原生 plugin 流程安裝。
Manifest 已宣告 `mcpServers: "./.mcp.json"`；無額外安裝工具，不需 Python。
安裝後開新對話說「幫我登入 Plasma」，skill 會使用宿主可呼叫的授權入口。
若宿主未提供可呼叫入口，請在 MCP server 清單選該 plugin 的連線並按 Authenticate。
CLI 先確認 `codex mcp list --json` 列出 plugin 連線，再以其實際名稱登入，不猜命名空間。
宿主開啟系統預設瀏覽器後，agent 立即結束當前回合，不等待授權、
不呼叫 `whoami`，也不使用 Browser、computer use 或桌面內建瀏覽器。使用者完成授權並在
新訊息確認後，agent 才於新回合呼叫 `whoami`。

## 升級自舊版

原有手動設定的 `plasma` MCP 可能與 plugin 連線重複，請檢查並選用 plugin 提供的連線。
本分支為 v0.6.0 功能測試，尚未建立新 tag。既有同名 tag／Release 並非此分支產物；
請使用本分支建置的 ZIP，或在 GitHub Actions 選本分支、tag 留空取得測試安裝包。

更新 plugin 後重新開啟對話／session，讓宿主移除舊 hook 註冊及舊版 skills。
`plasma-data-api` 與 `plasma-export` 已移除，統一使用 `plasma-create-view`。
不要沿用舊對話載入的匯出／發布指示；新包中只有三份 skills。
若曾手動將舊 hook 複製到宿主設定，請在該宿主刪除該自訂設定；新版不會執行舊 binary。
本次不自動修改使用者家目錄、既有 MCP 連線或其他宿主設定。
workspace 綁在授權上；換 workspace 要重新授權一次，plugin 不保存選擇。

## 從 GitHub Actions 取得安裝包

在 GitHub 開啟 **Actions → Package plugins → Run workflow**，選擇要封裝的分支，
`tag` 留空即可。完成後在該次執行頁面的 **Artifacts** 下載
`plasma-plugin-<版本>`，解壓縮後可取得 Claude ZIP、Codex ZIP、opencode ZIP 與
SHA-256 `checksums.txt`。Codex 包的檔名沿用 `_chatgpt.zip`，內容是
`.codex-plugin/plugin.json`、`.mcp.json` 與同一份 skills。Artifact 保留 30 天。

手動執行且 tag 留空只產生安裝包，不建立 GitHub Release。
推送 `v*` tag，或手動指定既有 tag，會封裝該 tag 的程式，並同時將安裝包附加到
新建的 GitHub Release；tag 必須與 `VERSION` 及兩份 manifest 一致。
工作流程檔須先推送到 GitHub 預設分支，才會顯示手動執行入口。

## 維護者本機驗證

```bash
make check
make release
```

本分支測試包可使用獨立目錄，避免混入先前同版本的本機產物：

```bash
python3 scripts/package_plugin.py --output dist/v0.6.0-bundled-mcp
```

僅需 Python 3.10+；封裝使用標準函式庫。產物在 `dist/v0.6.0/`，包括三個 ZIP
與 SHA-256 `checksums.txt`。Codex／Claude 安裝包收錄對應 manifest、`.mcp.json` 與共用 Markdown skills，
避免將開發工具、本機功能或後端修改清單帶入執行環境。

更新 `VERSION`、兩份 manifest 與對應的 `releases/vX.Y.Z.md`，再依團隊流程提交
及推送 tag。正式安裝包由 GitHub Actions 驗證、封裝並上傳，不再跨平台編譯 binary。
本機產生 ZIP 不會自動推送 tag 或發 GitHub Release。

Release notes 採一般專案的變更紀錄格式：以使用者可觀察的新增、修正與相容性影響為主，
每項簡短條列。不要寫開發過程、對作者的解釋、對話語氣或未採用方案；只有實際需要操作時
才加入升級或部署提醒。
