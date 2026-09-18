# Plasma workflows

供 **opencode、Claude Code 與 Codex CLI** 使用的共用工作流程 plugin。
以 Ophion 知識查核來源與 SQL，建立 view／manual mview 並核對定義後結束。
三種宿主共用同一份 skills，分別提供對應的 manifest／設定檔與 ZIP。

唯一流程：**查核來源 → 驗證 SQL → 建立 view／mview → 核對定義並交付**。

**Plugin 是工作流程；遠端 MCP gateway 是工具服務；MCP client 由宿主提供。**
安裝 plugin 不會自動建立 MCP 連線或授予服務權限。

```text
opencode / Claude Code / Codex CLI
├─ Plasma plugin：共用 skills
└─ 宿主的 MCP client ── OAuth 或存取權杖 ──> plasma-backend /mcp
                                            ├─ Plasma REST API
                                            └─ Ophion 知識工具
```

連線認證依宿主而異，伺服器兩條都支援：

| 宿主 | 認證 | 使用者要做的事 |
|---|---|---|
| **opencode** | OAuth | 設定檔加一段，`opencode mcp auth plasma`，瀏覽器登入完成 |
| **Claude Code**、**Codex CLI** | 存取權杖 | 到 `<gateway>/oauth/pat` 核發、存進環境變數、設定連線 |

差別在用戶端不在伺服器：Claude Code 與 Codex CLI 的 MCP SDK 不接受非 TLS 位址上的
OAuth token endpoint，所以在只有 http 的部署上只能走權杖。gateway 換上用戶端信任的
HTTPS 憑證之後，兩者也可以改用 OAuth。

ChatGPT 網頁版不支援：它由 OpenAI 伺服器連出，連不到內網的 gateway 位址。

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

`~/.config/opencode/opencode.json` 加入連線（URL 換成實際位址）：

```json
{
  "mcp": {
    "plasma": {
      "type": "remote",
      "url": "http://mcp.example.internal/mcp",
      "enabled": true,
      "oauth": { "scope": "views:read knowledge:read query:run views:write" }
    }
  }
}
```

```bash
opencode mcp auth plasma      # 開瀏覽器登入、選 workspace、確認權限
opencode mcp list             # 查看授權狀態
```

token 由 opencode 保管在 `~/.local/share/opencode/mcp-auth.json` 並自動更新，
使用者不需要保存任何字串。完整步驟見安裝包內的 `INSTALL.md`。

## Claude Code 與 Codex CLI:先核發存取權杖

管理員需提供可達的 MCP gateway 位址，並在後端開啟 `pat_enabled`。
使用者到 `<gateway>/oauth/pat` 用自己的 Plasma 帳號登入、選 workspace、
逐項勾選權限，核發一張存取權杖；該頁只顯示權杖一次。

把權杖放進環境變數後設定連線：

```bash
export PLASMA_MCP_TOKEN='<權杖>'

# Claude Code
claude mcp add --transport http plasma http://mcp.internal:5002/mcp \
  --header 'Authorization: Bearer ${PLASMA_MCP_TOKEN}'
```

單引號與 `${...}` 讓 Claude Code 讀取設定時才展開，權杖不會寫進 `~/.claude.json`。

```toml
# Codex CLI — ~/.codex/config.toml
[mcp_servers.plasma]
url = "http://mcp.internal:5002/mcp"
bearer_token_env_var = "PLASMA_MCP_TOKEN"
experimental_use_rmcp_client = true
```

開新對話後先呼叫 `whoami`，確認 workspace、scopes 與知識服務狀態。
權杖到期不會自動更新，重新核發一張即可；外洩時用 `<gateway>/oauth/revoke` 撤銷。

身分驗證、workspace 綁定與權限勾選都由 gateway 的核發頁執行，
權杖本身則是長效憑證，沒有 PKCE 與輪替。取捨與後端設定見
[後端修改清單](docs/backend-integration.md) 的 B7。

### 安裝 Claude Code 的 workflows

保留既有 plugin 名稱與 marketplace，方便原使用者更新：

```text
/plugin marketplace add BrobridgeOrg/plasma-agent-plugin
/plugin install plasma-plugin@plasma-plugin-local
```

設定完成後開新 session，先呼叫 `whoami`。不再安裝或執行任何 plugin binary。
Claude 安裝包為 `plasma-plugin_0.4.0_claude.zip`；三個版本的 skills 完全相同，無 hooks。
其他 Claude 介面的 plugin 安裝能力以該產品為準，這裡的安裝指令專供 Claude Code。

## 升級自舊版

更新 plugin 後重新開啟對話／session，讓宿主移除舊 hook 註冊及舊版 skills。
`plasma-data-api` 與 `plasma-export` 已移除，統一使用 `plasma-create-view`。
不要沿用舊對話載入的匯出／發布指示；新包中只有三份 skills。
若曾手動將舊 hook 複製到宿主設定，請在該宿主刪除該自訂設定；新版不會執行舊 binary。
本次不自動修改使用者家目錄、既有 MCP 連線或其他宿主設定。
workspace 綁在權杖上；換 workspace 要重新核發一張，plugin 不保存選擇。

## 從 GitHub Actions 取得安裝包

在 GitHub 開啟 **Actions → Package plugins → Run workflow**，選擇要封裝的分支，
`tag` 留空即可。完成後在該次執行頁面的 **Artifacts** 下載
`plasma-plugin-<版本>`，解壓縮後可取得 Claude ZIP、Codex ZIP、opencode ZIP 與
SHA-256 `checksums.txt`。Codex 包的檔名沿用 `_chatgpt.zip`，內容是
`.codex-plugin/plugin.json` 與同一份 skills。Artifact 保留 30 天。

手動執行且 tag 留空只產生安裝包，不建立 GitHub Release。
推送 `v*` tag，或手動指定既有 tag，會封裝該 tag 的程式，並同時將安裝包附加到
新建的 GitHub Release；tag 必須與 `VERSION` 及兩份 manifest 一致。
工作流程檔須先推送到 GitHub 預設分支，才會顯示手動執行入口。

## 維護者本機驗證

```bash
make check
make release
```

僅需 Python 3.10+；封裝使用標準函式庫。產物在 `dist/v0.4.0/`，包括三個 ZIP
與 SHA-256 `checksums.txt`。安裝包只收錄對應宿主 manifest 與共用 Markdown skills，
避免將開發工具、本機功能或後端修改清單帶入執行環境。

更新 `VERSION`、兩份 manifest 與對應的 `releases/vX.Y.Z.md`，再依團隊流程提交
及推送 tag。正式安裝包由 GitHub Actions 驗證、封裝並上傳，不再跨平台編譯 binary。
本機產生 ZIP 不會自動推送 tag 或發 GitHub Release。
