package main

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

func TestNoSubcommandListsTheModes(t *testing.T) {
	var out bytes.Buffer
	err := run(context.Background(), nil, strings.NewReader(""), &out)
	if err == nil {
		t.Fatal("want an error when no mode is given")
	}
	for _, want := range []string{"plasma", "ophion", "hook"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error = %v, want %q listed", err, want)
		}
	}
}

func TestUnknownSubcommandIsNamed(t *testing.T) {
	var out bytes.Buffer
	err := run(context.Background(), []string{"knowledge"}, strings.NewReader(""), &out)
	if err == nil || !strings.Contains(err.Error(), "knowledge") {
		t.Fatalf("error = %v, want the unknown mode named", err)
	}
}

func TestHookModeWritesTheDecisionToStdout(t *testing.T) {
	var out bytes.Buffer
	in := strings.NewReader(`{"hook_event_name":"PreToolUse",` +
		`"tool_name":"mcp__plugin_plasma-plugin_plasma__run_query","tool_input":{}}`)

	if err := run(context.Background(), []string{"hook"}, in, &out); err != nil {
		t.Fatalf("run: %v", err)
	}
	if !strings.Contains(out.String(), `"permissionDecision":"ask"`) {
		t.Errorf("stdout = %s, want the ask decision", out.String())
	}
}

func TestHookModeStaysSilentForOtherTools(t *testing.T) {
	var out bytes.Buffer
	in := strings.NewReader(`{"hook_event_name":"PreToolUse","tool_name":"Bash","tool_input":{}}`)

	if err := run(context.Background(), []string{"hook"}, in, &out); err != nil {
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
	if err := run(context.Background(), []string{"hook"}, in, &out); err != nil {
		t.Fatalf("run: %v", err)
	}
	if out.Len() != 0 {
		t.Errorf("stdout = %s, want nothing", out.String())
	}
}
