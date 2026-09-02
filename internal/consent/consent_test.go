package consent

import (
	"encoding/json"
	"strings"
	"testing"
)

func decide(t *testing.T, toolName string) map[string]any {
	t.Helper()
	in := []byte(`{"hook_event_name":"PreToolUse","tool_name":"` + toolName +
		`","tool_input":{"sql":"SELECT 1"}}`)
	out, err := Decide(in)
	if err != nil {
		t.Fatalf("Decide: %v", err)
	}
	// No output is how the hook says "not my business"; the tests read that
	// as a decision-free response rather than a parse failure.
	if len(out) == 0 {
		return map[string]any{}
	}
	var parsed map[string]any
	if err := json.Unmarshal(out, &parsed); err != nil {
		t.Fatalf("output is not JSON: %s", out)
	}
	return parsed
}

func hookOutput(t *testing.T, parsed map[string]any) map[string]any {
	t.Helper()
	specific, ok := parsed["hookSpecificOutput"].(map[string]any)
	if !ok {
		t.Fatalf("no hookSpecificOutput in %v", parsed)
	}
	return specific
}

func TestCostlyToolsAlwaysAsk(t *testing.T) {
	for _, tool := range []string{
		"mcp__plugin_plasma-plugin_plasma__run_query",
		"mcp__plugin_plasma-plugin_plasma__create_view",
		"mcp__plugin_plasma-plugin_plasma__sync_view",
		"mcp__plugin_plasma-plugin_plasma__create_access_entry",
	} {
		specific := hookOutput(t, decide(t, tool))
		if specific["permissionDecision"] != "ask" {
			t.Errorf("%s: permissionDecision = %v, want ask", tool, specific["permissionDecision"])
		}
		reason, _ := specific["permissionDecisionReason"].(string)
		if strings.TrimSpace(reason) == "" {
			t.Errorf("%s: no reason given; the prompt would not say what it costs", tool)
		}
	}
}

func TestReasonNamesTheCostOfTheSpecificTool(t *testing.T) {
	cases := map[string]string{
		"mcp__plugin_plasma-plugin_plasma__run_query":           "Trino",
		"mcp__plugin_plasma-plugin_plasma__sync_view":           "Trino",
		"mcp__plugin_plasma-plugin_plasma__create_access_entry": "outside",
	}
	for tool, want := range cases {
		specific := hookOutput(t, decide(t, tool))
		reason, _ := specific["permissionDecisionReason"].(string)
		if !strings.Contains(reason, want) {
			t.Errorf("%s: reason = %q, want %q in it", tool, reason, want)
		}
	}
}

func TestOtherToolsAreLeftAlone(t *testing.T) {
	for _, tool := range []string{
		"mcp__plugin_plasma-plugin_plasma__list_views",
		"mcp__plugin_plasma-plugin_ophion__overview",
		"Bash",
	} {
		parsed := decide(t, tool)
		if _, found := parsed["hookSpecificOutput"]; found {
			t.Errorf("%s: the hook returned a decision; read-only work must not be gated", tool)
		}
	}
}

func TestAnyServerNamingIsMatched(t *testing.T) {
	// The plugin's MCP tools are namespaced by the host, and that prefix has
	// changed shape before. Matching on the tool suffix keeps the gate
	// working when it changes again.
	for _, tool := range []string{
		"mcp__plasma__run_query",
		"mcp__plugin_plasma-plugin_plasma__run_query",
		"mcp__plugin_someothername_plasma__run_query",
	} {
		specific := hookOutput(t, decide(t, tool))
		if specific["permissionDecision"] != "ask" {
			t.Errorf("%s: not gated", tool)
		}
	}
}

func TestUnrelatedSuffixMatchIsNotGated(t *testing.T) {
	parsed := decide(t, "mcp__other__run_query_builder")
	if _, found := parsed["hookSpecificOutput"]; found {
		t.Error("run_query_builder is a different tool; the gate must not catch it")
	}
}

func TestMalformedInputIsNotADecision(t *testing.T) {
	out, err := Decide([]byte("not json"))
	if err == nil {
		t.Fatal("want an error for unreadable hook input")
	}
	if len(out) != 0 {
		t.Errorf("output = %s, want nothing: a broken hook must not fabricate a decision", out)
	}
}

func TestEventNameIsEchoedBack(t *testing.T) {
	specific := hookOutput(t, decide(t, "mcp__plugin_plasma-plugin_plasma__run_query"))
	if specific["hookEventName"] != "PreToolUse" {
		t.Errorf("hookEventName = %v, want PreToolUse", specific["hookEventName"])
	}
}
