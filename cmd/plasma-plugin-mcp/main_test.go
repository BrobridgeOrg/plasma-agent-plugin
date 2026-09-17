package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestNoSubcommandNamesTheMode(t *testing.T) {
	var out bytes.Buffer
	err := run(nil, strings.NewReader(""), &out)
	if err == nil || !strings.Contains(err.Error(), "hook") {
		t.Fatalf("error = %v, want the hook mode named", err)
	}
}

func TestUnknownSubcommandIsNamed(t *testing.T) {
	var out bytes.Buffer
	err := run([]string{"knowledge"}, strings.NewReader(""), &out)
	if err == nil || !strings.Contains(err.Error(), "knowledge") {
		t.Fatalf("error = %v, want the unknown mode named", err)
	}
}

// An install still configured for the old MCP servers has to be told where
// they went, not that it made a typo. This is the error an operator actually
// hits after upgrading without editing their plugin configuration.
func TestRetiredServerModesPointAtTheGateway(t *testing.T) {
	for _, mode := range []string{"plasma", "ophion"} {
		var out bytes.Buffer
		err := run([]string{mode}, strings.NewReader(""), &out)
		if err == nil {
			t.Fatalf("%s: want an error", mode)
		}
		if !strings.Contains(err.Error(), "mcp_gateway") {
			t.Errorf("%s: error = %v, want mcp_gateway named", mode, err)
		}
	}
}

func TestHookModeWritesTheDecisionToStdout(t *testing.T) {
	var out bytes.Buffer
	// The tool name carries the gateway's server namespacing: the gate
	// matches the bare tool name behind it, so it works against whichever
	// server ends up serving these tools.
	in := strings.NewReader(`{"hook_event_name":"PreToolUse",` +
		`"tool_name":"mcp__plasma__sync_view","tool_input":{}}`)

	if err := run([]string{"hook"}, in, &out); err != nil {
		t.Fatalf("run: %v", err)
	}
	if !strings.Contains(out.String(), `"permissionDecision":"ask"`) {
		t.Errorf("stdout = %s, want the ask decision", out.String())
	}
}

func TestHookModeStaysSilentForOtherTools(t *testing.T) {
	var out bytes.Buffer
	in := strings.NewReader(`{"hook_event_name":"PreToolUse","tool_name":"Bash","tool_input":{}}`)

	if err := run([]string{"hook"}, in, &out); err != nil {
		t.Fatalf("run: %v", err)
	}
	if out.Len() != 0 {
		t.Errorf("stdout = %s, want nothing", out.String())
	}
}

func TestHookModeNeverBlocksOnBadInput(t *testing.T) {
	var out bytes.Buffer
	in := strings.NewReader("not json")

	// A hook that exits non-zero on garbage input would break every tool
	// call, so the failure has to be silent and non-fatal.
	if err := run([]string{"hook"}, in, &out); err != nil {
		t.Fatalf("run: %v", err)
	}
	if out.Len() != 0 {
		t.Errorf("stdout = %s, want nothing", out.String())
	}
}
