// Package consent implements the PreToolUse gate.
//
// Confirmation happens when data synchronization starts or an API is opened.
// Queries and manual view definitions are preparation and do not need this
// gate. Scheduled creation also needs confirmation because it starts syncing
// immediately. The host displays the tool arguments alongside the Chinese reason.
package consent

import (
	"encoding/json"
	"fmt"
	"strings"
)

// gated maps a tool to why it needs a human. The reason is what the operator
// reads in the prompt, so it names the cost, not the mechanism.
var gated = map[string]string{
	"sync_view": "執行 sync 後，系統就會開始依照 mview 的 SQL 從來源系統拉取資料，" +
		"並寫入或更新 mview，會使用查詢與同步資源。同步成功後，開啟資料 API 前會另外確認。是否確認開始同步？",
	"create_access_entry": "即將開啟資料 API，讓可連線到此端點且符合驗證設定的呼叫者讀取這個 view 的資料。" +
		"請確認資料範圍、驗證方式及有效期限；若驗證方式為 none，持有 URL 的人不需驗證即可讀取，" +
		"未設定有效期限則不會自動到期。是否確認開啟 API？",
}

type hookInput struct {
	HookEventName string          `json:"hook_event_name"`
	ToolName      string          `json:"tool_name"`
	ToolInput     json.RawMessage `json:"tool_input"`
}

// Decide returns the hook response for one PreToolUse event, or no output at
// all when the tool is none of its business.
//
// Returning nothing is deliberate: a hook that answered for every tool would
// have to decide about tools it knows nothing about.
func Decide(raw []byte) ([]byte, error) {
	var in hookInput
	if err := json.Unmarshal(raw, &in); err != nil {
		return nil, fmt.Errorf("unreadable PreToolUse input: %w", err)
	}
	tool := toolSuffix(in.ToolName)
	reason := gated[tool]
	if tool == "create_view" && len(in.ToolInput) > 0 {
		var args struct {
			SyncMode string `json:"sync_mode"`
		}
		if err := json.Unmarshal(in.ToolInput, &args); err != nil {
			return nil, fmt.Errorf("unreadable create_view input: %w", err)
		}
		if args.SyncMode == "scheduled" {
			reason = "建立這個排程 mview 後，系統就會立即開始依照 SQL 從來源系統拉取資料並寫入 mview，" +
				"之後也會依照指定排程自動拉取資料，使用查詢與同步資源。" +
				"請確認資料範圍與排程頻率；同步成功後，開啟資料 API 前會另外確認。是否確認開始同步並啟用排程？"
		}
	}
	if reason == "" {
		return nil, nil
	}
	event := in.HookEventName
	if event == "" {
		event = "PreToolUse"
	}
	return json.Marshal(map[string]any{
		"hookSpecificOutput": map[string]any{
			"hookEventName":            event,
			"permissionDecision":       "ask",
			"permissionDecisionReason": reason,
		},
	})
}

// toolSuffix is the bare tool name behind the host's namespacing
// (mcp__<server>__<tool>, or mcp__plugin_<plugin>_<server>__<tool>).
//
// Matching the suffix rather than the full name keeps the gate working if the
// host changes how it namespaces plugin tools — and it is exact, so a
// different tool whose name merely starts the same is not caught.
func toolSuffix(name string) string {
	if idx := strings.LastIndex(name, "__"); idx >= 0 {
		return name[idx+2:]
	}
	return name
}
