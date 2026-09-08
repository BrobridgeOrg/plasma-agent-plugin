---
name: plasma-plugin-setup
description: >-
  安裝完 plasma-plugin 後的第一件事，以及日後 Plasma 或 Ophion MCP 工具缺少、連線失敗或驗證失敗時使用。提供兩條設定路線：逐步問答直接寫入 config.env，或告知設定檔路徑由使用者自行填寫；另處理狀態檔、預編譯執行檔與錯誤診斷，並說明同步與開 API 的確認時點。全程台灣繁體中文。
---

# Setting up and diagnosing the plugin

## 共通互動原則

- 所有對使用者的回覆都使用**台灣繁體中文**，包含進度、問題、結果、錯誤說明、確認文字及交付說明。工具名稱、SQL、欄位名稱、URL 與需忠實引用的原文保留原樣，並以台灣繁體中文解釋。
- 在使用者已交付的任務範圍內，連續完成知識查找、欄位查核、SQL 驗證及不會啟動同步的 mview 建立；報告進度即可，不要每完成一步就問「是否繼續」。只有缺少會影響正確性的必要資訊時才釐清，釐清不等於每一步都要核准。
- 確認集中在兩個執行時點：**開始同步拉資料**，以及同步成功後**開啟資料 API**。每次以中文清楚說明具體影響；同一動作不要先在對話問一次、又重複要求一次工具確認。若宿主提供符合需求的確認介面，使用該介面；否則以中文取得明確同意後再呼叫工具。
- **原則上一份表單／報表建立一個 mview。** 不因不同區塊、指標、頁籤或來源表就拆成多個 mview；只有使用者明確要求拆分，才改變這個原則。

安裝完 plugin 後，**這份 skill 是第一件該做的事**。設定沒完成之前，Plasma 與
Ophion 的工具只會回認證或連線錯誤，先設定比先試工具快。

## 先看現況，再決定要做什麼

設定檔與快取都在 plugin 目錄**之外**（marketplace 更新會整個換掉 plugin 目錄），
由 `bin/plasma-config.sh` 這支小工具管理。第一步固定是：

```bash
"$CLAUDE_PLUGIN_ROOT/bin/plasma-config.sh" check
```

它印出設定檔路徑、每個欄位的值與來源（`file` 或環境變數 `env`），秘密欄位只
顯示長度不顯示內容，最後給 `status=complete` 或 `status=incomplete` 加上缺什麼。

- `status=complete`：跳到「驗證」。使用者是來排障的，不要重問一次設定。
- `status=incomplete`：問使用者要走哪條路線，再照該路線做。

用宿主的選擇介面問這一題（沒有介面就用中文直接問），兩個選項：

- **逐步問答（建議）**：由你一題一題問，答案直接寫進設定檔。
- **我自己編輯設定檔**：你只給路徑與欄位說明，使用者自己填。

使用者已經明講要哪一種時，就照做，不要再問。

## 路線 A：逐步問答

用 `plasma-config.sh` 寫檔，**不要**用檔案編輯工具直接寫 `config.env`：這支
工具只接受已知欄位、維持 600 權限、重複指定同一欄位也只留一行。

```bash
"$CLAUDE_PLUGIN_ROOT/bin/plasma-config.sh" init             # 建立目錄與設定檔
"$CLAUDE_PLUGIN_ROOT/bin/plasma-config.sh" set KEY VALUE    # 寫入一個欄位
"$CLAUDE_PLUGIN_ROOT/bin/plasma-config.sh" show             # 檢視（秘密只顯示長度）
"$CLAUDE_PLUGIN_ROOT/bin/plasma-config.sh" probe            # 不帶認證測兩個 endpoint 通不通
```

問的順序，Plasma 一組、Ophion 一組，不要一個欄位發一則訊息：

1. **Plasma**
   - `PLASMA_URL`：Plasma 的 API 位址，例如 `http://127.0.0.1:5001`。
   - 認證方式二選一，問使用者用哪一種：
     - **帳號密碼**：`PLASMA_USERNAME` + `PLASMA_PASSWORD`，plugin 會自己登入並維護 JWT，過期會換新。
     - **靜態 token**：`PLASMA_TOKEN`，原樣使用、永不更新，過期就得再換一次。
   - 選定後把另一種的欄位清空（`set PLASMA_TOKEN ""`，或把 username 與 password 設成空字串），避免兩種認證同時存在時 token 悄悄優先。
2. **Ophion**
   - `OPHION_URL`：Ophion query-mcp 位址，例如 `http://127.0.0.1:5101`。這是叢集內 API，工作站通常要先 port-forward：
     `kubectl -n <namespace> port-forward svc/ophion 5101:5101`。
   - `OPHION_SERVICE_TOKEN`：Ophion 的 service token。
   - `OPHION_PROFILE` 預設 `all`，不必問；使用者主動要求才改成 `qa`、`text_to_sql`、`fhir` 或 `audit`。

每收到一組答案就立刻寫入，寫完再問下一組，中途中斷也不會白填。

### 秘密欄位怎麼收

`PLASMA_TOKEN`、`PLASMA_PASSWORD`、`OPHION_SERVICE_TOKEN` 三個是秘密。問之前
先用中文說清楚，讓使用者自己選：

- **直接在對話中提供**：最快，但這段對話紀錄會留下該值。
- **自己在終端機輸入**（不進對話紀錄）：請使用者在自己的終端機執行

  ```bash
  <CLAUDE_PLUGIN_ROOT 的絕對路徑>/bin/plasma-config.sh set-secret PLASMA_PASSWORD
  ```

  輸入時不回顯，直接寫進設定檔。給指令時把 `$CLAUDE_PLUGIN_ROOT` 展開成實際
  絕對路徑，使用者的終端機沒有這個環境變數。輸入完成後你再跑一次 `check`
  確認該欄位已有值。

不論走哪一種，都**不要把秘密值回覆到對話裡**，包含「我幫你設定的是 xxx」這種
覆述；要確認就用 `show`，它只給長度。

### 收尾

填完跑 `check`（應為 `status=complete`）再跑 `probe`。`probe` 不帶認證，只分辨
「位址打錯／port-forward 沒開」和「連得到但要認證」——`reachable (HTTP 401/403/404)`
都算通，`unreachable` 才是設定或網路的問題。然後進「驗證」。

## 路線 B：使用者自己編輯設定檔

先跑 `init` 把範本複製好（權限 600），再把路徑與需要填的東西交給使用者：

```bash
"$CLAUDE_PLUGIN_ROOT/bin/plasma-config.sh" init   # 印出設定檔路徑
```

設定檔預設在 `~/.plasma-plugin/config.env`（`PLASMA_PLUGIN_HOME` 可改根目錄）。
用中文列出要填的欄位，各一句話說明用途：`PLASMA_URL`；`PLASMA_TOKEN`
或 `PLASMA_USERNAME` + `PLASMA_PASSWORD` 擇一；`OPHION_URL`、
`OPHION_SERVICE_TOKEN`；`OPHION_PROFILE` 預設 `all` 可不動。順帶提醒 Ophion
通常要 port-forward，以及環境變數會蓋過檔案內容，所以單一 session 可以不改檔
就指向別的部署。

使用者說填好了，跑 `check` 幫他驗一遍，再進「驗證」。填到一半改變主意想用問
答，隨時可以切到路線 A，已填的值會保留。

## 驗證

設定檔是在 **MCP server 啟動時**讀的。第一次設定完成後，請使用者重啟 Claude
Code session（或以宿主的方式重新連線 MCP server），否則兩台 server 仍帶著舊
的、通常是空的設定在跑。

重啟後叫 `whoami`：它一次報出兩個 endpoint、登入者與當下 workspace，那一次呼叫
就是完整的健康檢查。若還沒選 workspace，用 `list_workspaces` 與 `use_workspace`
選一個。失敗訊息對照下面的表。

## 執行檔安裝

launcher 首次啟動會下載與 plugin 版本一致的預編譯執行檔，驗證 SHA-256 後快取
在 `~/.plasma-plugin/bin/v<version>/<os>-<arch>/`。使用者不需要 Go。支援
macOS／Linux 的 arm64、amd64（Windows 走 WSL）。

私有 repo 的下載需要已登入的 GitHub CLI（`gh auth login`）。或者從 GitHub
Release 下載對應平台的壓縮檔與 `checksums.txt` 到同一個目錄，啟動 Claude Code
時帶 `PLASMA_MCP_RELEASE_DIR=/absolute/path/to/that/directory`；離線安裝也用這個
方式。該變數只在安裝尚未快取的版本時需要。

開發用途才需要 `make build` 並把 `PLASMA_MCP_BINARY` 指向 `bin/plasma-plugin-mcp`
的絕對路徑。正常啟動不會編譯，也不會自己去用那顆開發用執行檔。

## State

`~/.plasma-plugin/state.json`（權限 600）存當下 workspace、profile 與快取的
JWT。兩台 server 每次呼叫都重讀，`use_workspace` 才能不重啟就讓 ophion 那台改
道。它跨 session 保留，所以新 session 通常還在上次的 workspace。刪掉是安全
的：只會清掉選擇並強制重新登入。

## Reading the failures

| What you see | What it means | What to do |
|---|---|---|
| Session 開頭提示「尚未完成設定」 | SessionStart hook 發現必填欄位還缺 | 跑這份 skill，從「先看現況」開始 |
| `Cannot download` / `Release download failed` | Release unavailable or GitHub access missing | Check access to `BrobridgeOrg/plasma-agent-plugin`, authenticate gh, or use `PLASMA_MCP_RELEASE_DIR` |
| `Checksum mismatch` | Archive does not match the release checksums | Download both files again from the same release; do not bypass verification |
| `no Plasma credentials` | Neither auth mode is configured | Set `PLASMA_TOKEN`, or username + password |
| Plasma `401` after retry | Password rejected, or a static token expired | Re-check the credentials; a static token is never refreshed for you |
| Plasma `403` | Authenticated, but not a member of this workspace | `list_workspaces` and pick one you belong to |
| `no workspace selected` | Nothing selected yet | `use_workspace` |
| Ophion `no published knowledge` (404) | The workspace exists but has no published generation | Nothing to fix here; the knowledge has to be generated first |
| Ophion `rejected the service token` | `OPHION_SERVICE_TOKEN` wrong or unset | Fix it in `config.env` |
| Ophion `unreachable` | Nothing listening at `OPHION_URL` | Start the port-forward, or point at the in-cluster URL |
| `OPHION_URL is not set` | Ophion half-configured | Set it; the plugin refuses to guess an endpoint |
| 設定明明填了卻仍報缺值 | server 是在 session 啟動時讀設定的 | 重啟 session 後再叫 `whoami` |
| 同步或開 API 的確認 | `sync_view`、會立即同步的 scheduled `create_view`、`create_access_entry` 需要確認 | 以台灣繁體中文說明：sync 會開始拉取資料並寫入 mview；開 API 會讓符合存取條件的呼叫者讀取資料 |
| 查詢或建立 manual mview 仍逐次跳出確認 | 可能仍在使用舊版 hook／binary，或宿主另設了工具權限 | 檢查安裝版本與宿主設定；目前流程不額外強制這兩步確認，不能以關閉所有同步／API 確認來排障 |

## Changing deployment

把 `PLASMA_URL` / `OPHION_URL` 指向另一套部署（`plasma-config.sh set`，或直接
編輯設定檔），然後重啟 session——MCP server 只在啟動時讀設定。接著重新
`use_workspace`：舊部署的 workspace id 在新部署上解不出來，工具會直接說不
存在，而不是去動別人的 workspace。
