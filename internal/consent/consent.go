// Package consent implements the PreToolUse gate.
//
// Four plasma tools spend Trino time or publish data outside Plasma. Tool
// annotations alone do not stop them from being approved once and then run
// unattended, because a client can allow-list a whole MCP server. This hook
// forces the host to ask every time, and the host's prompt shows the tool's
// arguments — so the operator sees the exact SQL before it runs.
package consent

import (
	"encoding/json"
	"fmt"
	"strings"
)

// gated maps a tool to why it needs a human. The reason is what the operator
// reads in the prompt, so it names the cost, not the mechanism.
var gated = map[string]string{
	"run_query": "This executes SQL on the Trino cluster and consumes query capacity. " +
		"Read the statement above before approving.",
	"sync_view": "This runs the view's SQL on the Trino cluster to rebuild its data, " +
		"which can be an expensive job.",
	"create_view": "This creates a view or materialized view, and with a schedule it " +
		"also creates a recurring sync job that will keep using Trino capacity.",
	"create_access_entry": "This publishes the view outside Plasma: the URL and secret it " +
		"returns can be used to read this data from anywhere the endpoint is reachable.",
}

type hookInput struct {
	HookEventName string `json:"hook_event_name"`
	ToolName      string `json:"tool_name"`
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
	reason, ok := gated[toolSuffix(in.ToolName)]
	if !ok {
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
