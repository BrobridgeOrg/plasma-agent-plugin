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
      "url": "http://mcp.example.internal/mcp",
      "enabled": true,
      "oauth": {
        "scope": "views:read knowledge:read query:run views:write"
      }
    }
  }
}
```

`scope` 是這次連線要請求的權限，會出現在授權頁讓你逐項確認：
讀 view 定義、查資料知識、執行唯讀查詢、建立 view／mview 定義。
用不到的可以刪掉，之後需要時重新授權即可。

## 3. 授權

```bash
opencode mcp auth plasma
```

瀏覽器會開啟 gateway 的授權頁：輸入 Plasma 帳密、選 workspace、確認權限。
完成後回到終端機即可，token 由 opencode 保管。

確認狀態：

```bash
opencode mcp list
```

開新的 opencode session，先讓它呼叫 `whoami` 核對 workspace 與 scopes。
skills 要下一個 session 才會載入。

## 之後

- 換 workspace：`opencode mcp logout plasma` 再 `opencode mcp auth plasma`，重選一次。
- 權限不足：修改設定裡的 `scope`，重新授權。
- 撤銷：`opencode mcp logout plasma` 清掉本機憑證；
  要讓伺服器端也失效，請管理員在 gateway 撤銷該授權。
