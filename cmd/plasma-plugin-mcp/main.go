// Command plasma-plugin-mcp is the plugin's single binary in three modes:
//
//	plasma-plugin-mcp plasma   # MCP server: Plasma control plane over stdio
//	plasma-plugin-mcp ophion   # MCP server: Ophion knowledge for the selected workspace
//	plasma-plugin-mcp hook     # PreToolUse gate, reads the event on stdin
//
// One binary keeps the three in step: they share the config and state files,
// and the gate has to know exactly which tools the server registers.
package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/BrobridgeOrg/plasma-plugin/internal/consent"
	"github.com/BrobridgeOrg/plasma-plugin/internal/ophionproxy"
	"github.com/BrobridgeOrg/plasma-plugin/internal/pcontext"
	"github.com/BrobridgeOrg/plasma-plugin/internal/plasmaapi"
	"github.com/BrobridgeOrg/plasma-plugin/internal/plasmamcp"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if err := run(ctx, os.Args[1:], os.Stdin, os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "plasma-plugin-mcp:", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, args []string, stdin io.Reader, stdout io.Writer) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: plasma-plugin-mcp <plasma|ophion|hook>")
	}
	switch args[0] {
	case "plasma":
		return servePlasma(ctx)
	case "ophion":
		return serveOphion(ctx)
	case "hook":
		return runHook(stdin, stdout)
	default:
		return fmt.Errorf("unknown mode %q: expected plasma, ophion or hook", args[0])
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

func servePlasma(ctx context.Context) error {
	home := pcontext.New(pcontext.DefaultDir())
	cfg, err := home.LoadConfig()
	if err != nil {
		return err
	}
	server := plasmamcp.NewServer(plasmamcp.Deps{
		Config: cfg,
		Home:   home,
		Plasma: plasmaapi.New(cfg, home, nil),
	})
	return server.Run(ctx, &mcp.StdioTransport{})
}

func serveOphion(ctx context.Context) error {
	home := pcontext.New(pcontext.DefaultDir())
	cfg, err := home.LoadConfig()
	if err != nil {
		return err
	}
	server, closer := ophionproxy.NewServer(ophionproxy.Deps{Config: cfg, Home: home})
	defer closer.Close()
	return server.Run(ctx, &mcp.StdioTransport{})
}
