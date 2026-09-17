// Command plasma-plugin-mcp is the plugin's PreToolUse gate.
//
//	plasma-plugin-mcp hook   # reads the event on stdin, writes the decision
//
// It used to serve two MCP servers as well — Plasma's control plane and a
// proxy onto Ophion's knowledge. Both now live in plasma-backend's
// mcp_gateway, which speaks MCP over HTTP behind its own OAuth 2.1
// authorization and mirrors Ophion's tools itself. A client connects to that
// by URL, so there is one MCP server to add, and this plugin holds no
// credentials, no endpoints and no workspace selection.
//
// What is left is the one thing the gateway cannot do. It grants scopes once,
// when the user links the connection, and never asks again — so a
// confirmation immediately before data actually moves has to come from the
// client side, which is here.
//
// The binary keeps its name: the released assets and the launcher script
// address it by that name.
package main

import (
	"fmt"
	"io"
	"os"

	"github.com/BrobridgeOrg/plasma-plugin/internal/consent"
)

func main() {
	if err := run(os.Args[1:], os.Stdin, os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "plasma-plugin-mcp:", err)
		os.Exit(1)
	}
}

func run(args []string, stdin io.Reader, stdout io.Writer) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: plasma-plugin-mcp hook")
	}
	switch args[0] {
	case "hook":
		return runHook(stdin, stdout)
	case "plasma", "ophion":
		// Named rather than lumped in with a typo. A stale plugin.json or a
		// half-updated install will still ask for these, and "unknown mode"
		// would send the operator looking for a bug instead of for the
		// gateway's URL.
		return fmt.Errorf("the %s MCP server is gone: both are now served by "+
			"plasma-backend's mcp_gateway, one HTTP MCP endpoint reached by URL. "+
			"Remove the mcpServers entries from the plugin", args[0])
	default:
		return fmt.Errorf("unknown mode %q: expected hook", args[0])
	}
}

// runHook writes the gate's decision, and stays quiet about anything else.
//
// Every failure here is silent on purpose: this process runs before each tool
// call, and a hook that errors out would break tool use across the session
// over a malformed event it could not act on anyway.
func runHook(stdin io.Reader, stdout io.Writer) error {
	raw, err := io.ReadAll(stdin)
	if err != nil {
		return nil
	}
	decision, err := consent.Decide(raw)
	if err != nil || len(decision) == 0 {
		return nil
	}
	_, err = stdout.Write(decision)
	return err
}
