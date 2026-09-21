# 在 opencode 安裝 Plasma workflows

opencode 沒有 plugin marketplace：skills 是目錄裡的 `SKILL.md`，
MCP 連線寫在使用者自己的設定檔。所以安裝是兩件事，各做一次。

連線走 OAuth：opencode 會偵測 gateway 的 401、自動做 Dynamic Client Registration、
開瀏覽器讓你登入，然後把 token 存進 `~/.local/share/opencode/mcp-auth.json` 並自動更新。
不需要手動核發或複製任何權杖。

## 1. 放 skills

把壓縮檔裡的 `skills/` 三個目錄複製到 opencode 會掃描的位置：

```bash
mkdir -p ~/.config/opencode/skills
cp -R skills/* ~/.config/opencode/skills/
```

只想在單一專案使用時，改放到該專案的 `.opencode/skills/`。
opencode 也會讀 `~/.claude/skills/`，已經在 Claude Code 裝過的話那份也算數，
但兩邊同名會重複載入，擇一即可。

## 2. 設定連線

把 `opencode.json` 的 `mcp` 區塊合併進 `~/.config/opencode/opencode.json`，
**把 `url` 換成管理員給的實際 gateway 位址**（結尾是 `/mcp`）：

```json
{
  "$schema": "https://opencode.ai/config.json",
  "mcp": {
    "plasma": {
      "type": "remote",
      "url": "https://mcp.example.internal/mcp",
      "enabled": true
    }
  }
}
```

權限由 gateway 決定，設定檔不必（也無法）指定：每次授權都會取得該部署支援的
完整權限——讀 view 定義、查資料知識、執行唯讀查詢、建立 view／mview 定義。

## 3. 授權

```bash
opencode mcp auth plasma
```

瀏覽器會開啟 gateway 的授權頁：輸入 Plasma 帳密、選 workspace。
選定即完成授權，沒有額外的確認頁。回到終端機即可，token 由 opencode 保管。

確認狀態：

```bash
opencode mcp list
```

開新的 opencode session，先讓它呼叫 `whoami` 核對 workspace 與 scopes。
skills 要下一個 session 才會載入。

## 之後

- 換 workspace：`opencode mcp logout plasma` 再 `opencode mcp auth plasma`，重選一次。
- 權限不足：重新授權一次即可，權限由 gateway 決定。
- 撤銷：`opencode mcp logout plasma` 清掉本機憑證；
  要讓伺服器端也失效，請管理員在 gateway 撤銷該授權。
- 診斷：`opencode mcp debug plasma`

## 常見問題

**一定要用 `mcp auth` 當次印出的網址。** 每次執行都會產生新的一組 state，
用到先前留著的分頁或自行拼湊的網址，會被判定為 CSRF 而失敗
（頁面顯示 `Invalid or expired state parameter`）。

**按下「同意並連結」後頁面不動。** OAuth 最後一步要從 gateway 導回本機的
`127.0.0.1:19876`。當 gateway 位於內網位址而且不是 HTTPS 時，Chrome 142 之後
會擋掉這個跨網段導轉，而且不會顯示任何提示——因為要求該權限的資格僅限 HTTPS 頁面。

這需要管理員處理，擇一：

- 為 gateway 配置用戶端信任的 HTTPS 憑證（自簽而未把 CA 佈到用戶端不算）
- 或由 IT 以 Chrome 政策 `LoopbackNetworkAccessAllowedForUrls` 放行該來源

**舊連線只有部分權限。** 在 gateway 改為一次授予全部之前建立的連線會是這樣，
`opencode mcp logout plasma` 後重新授權一次即可。
