// Package ophionproxy exposes one Ophion workspace's read-only knowledge
// tools over stdio, and re-points them whenever the shared workspace
// selection changes.
//
// Claude Code does not reconnect an MCP server mid-session, so "dynamic"
// cannot mean editing configuration. Instead this server is a proxy: it is a
// fixed stdio server to the client and an MCP client to Ophion, and it reads
// the selected workspace from the shared state file on every call. The
// upstream tool list is forwarded verbatim — wrapping Ophion's schemas here
// would create a second copy to keep in step with a contract that is still
// moving.
package ophionproxy

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/BrobridgeOrg/plasma-plugin/internal/pcontext"
)

// Version is reported to the MCP client.
const Version = "0.1.0"

// Deps is everything the proxy needs.
type Deps struct {
	Config pcontext.Config
	Home   *pcontext.Home
	// HTTPClient is optional; the default carries a 60s timeout, which bounds
	// a knowledge query without cutting a slow graph read short.
	HTTPClient *http.Client
}

const instructions = `Read-only knowledge about how one data system was designed,
for the workspace currently selected in the plasma server.

Switch workspaces with the plasma server's use_workspace: the next call here
follows it. Every answer states the workspace it came from — check that line
before trusting a result.

ophion_context reports the selected workspace and whether Ophion is reachable.`

type proxyServer struct {
	deps   Deps
	server *mcp.Server
	http   *http.Client
	probe  *statusProbe

	mu         sync.Mutex
	sessions   map[string]*mcp.ClientSession // keyed by workspace
	registered bool                          // upstream tools mirrored already
}

// NewServer builds the stdio-side MCP server and a closer for the upstream
// sessions it opens.
//
// The closer matters even though a stdio server normally dies with its
// process: the streamable transport holds a long-lived stream per workspace,
// and something has to be able to end them.
//
// The upstream tool list cannot be known before a workspace is selected, so
// registration is lazy: the first request after a selection mirrors the
// upstream tools (which also emits tools/list_changed). ophion_context is
// always present, so the server is never empty and can always explain itself.
func NewServer(deps Deps) (*mcp.Server, io.Closer) {
	httpClient := deps.HTTPClient
	probe := &statusProbe{}
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 60 * time.Second}
	}
	// Wrap whatever transport we ended up with so a failed connect can be
	// explained by the status the server actually returned.
	base := httpClient.Transport
	if base == nil {
		base = http.DefaultTransport
	}
	clone := *httpClient
	probe.base = base
	clone.Transport = probe
	httpClient = &clone

	p := &proxyServer{
		deps:     deps,
		http:     httpClient,
		probe:    probe,
		sessions: map[string]*mcp.ClientSession{},
	}
	p.server = mcp.NewServer(&mcp.Implementation{
		Name:    "ophion",
		Title:   "Ophion Knowledge (current Plasma workspace)",
		Version: Version,
	}, &mcp.ServerOptions{Instructions: instructions})

	mcp.AddTool(p.server, &mcp.Tool{
		Name: "ophion_context",
		Description: "Report which workspace these knowledge tools are reading, and " +
			"whether Ophion is reachable. Use it when a knowledge tool is missing or failing.",
		Annotations: &mcp.ToolAnnotations{ReadOnlyHint: true},
	}, p.contextTool)

	// Mirroring on every inbound request keeps tools/list correct without a
	// client having to know it should ask again.
	p.server.AddReceivingMiddleware(func(next mcp.MethodHandler) mcp.MethodHandler {
		return func(ctx context.Context, method string, req mcp.Request) (mcp.Result, error) {
			p.mirrorUpstreamTools(ctx)
			return next(ctx, method, req)
		}
	})
	return p.server, p
}

// Close ends every upstream session this proxy opened.
func (p *proxyServer) Close() error {
	p.mu.Lock()
	sessions := make([]*mcp.ClientSession, 0, len(p.sessions))
	for workspace, session := range p.sessions {
		sessions = append(sessions, session)
		delete(p.sessions, workspace)
	}
	p.mu.Unlock()

	var err error
	for _, session := range sessions {
		if closeErr := session.Close(); closeErr != nil && err == nil {
			err = closeErr
		}
	}
	return err
}

type contextInput struct{}

type contextOutput struct {
	WorkspaceID   string `json:"workspace_id,omitempty"`
	WorkspaceName string `json:"workspace_name,omitempty"`
	Profile       string `json:"profile"`
	OphionURL     string `json:"ophion_url"`
	Reachable     bool   `json:"reachable"`
	Diagnosis     string `json:"diagnosis,omitempty"`
	Tools         int    `json:"upstream_tools,omitempty"`
}

func (p *proxyServer) contextTool(ctx context.Context, _ *mcp.CallToolRequest, _ contextInput) (
	*mcp.CallToolResult, contextOutput, error) {

	st, err := p.deps.Home.LoadState()
	if err != nil {
		return nil, contextOutput{}, err
	}
	out := contextOutput{
		WorkspaceID:   st.WorkspaceID,
		WorkspaceName: st.WorkspaceName,
		Profile:       p.profile(st),
		OphionURL:     p.deps.Config.OphionURL,
	}
	lines := []string{"Ophion:    " + orNotSet(out.OphionURL), "Profile:   " + out.Profile}

	if st.WorkspaceID == "" {
		out.Diagnosis = "no workspace selected: call the plasma server's use_workspace first"
		lines = append(lines, "Workspace: none selected", "", out.Diagnosis)
		return textResult(lines...), out, nil
	}
	lines = append(lines, fmt.Sprintf("Workspace: %s (%s)", orDash(st.WorkspaceName), st.WorkspaceID))

	session, err := p.session(ctx, st.WorkspaceID)
	if err != nil {
		out.Diagnosis = err.Error()
		lines = append(lines, "", "NOT REACHABLE: "+out.Diagnosis)
		return textResult(lines...), out, nil
	}
	tools, err := session.ListTools(ctx, nil)
	if err != nil {
		out.Diagnosis = err.Error()
		lines = append(lines, "", "NOT REACHABLE: "+out.Diagnosis)
		return textResult(lines...), out, nil
	}
	out.Reachable = true
	out.Tools = len(tools.Tools)
	lines = append(lines, fmt.Sprintf("Reachable: yes, %d knowledge tool(s)", out.Tools))
	return textResult(lines...), out, nil
}

// mirrorUpstreamTools registers the upstream tool list on this server once a
// workspace is selected and Ophion answers. Failures are left to
// ophion_context to report: a knowledge outage must not take down the
// diagnostic that explains it.
func (p *proxyServer) mirrorUpstreamTools(ctx context.Context) {
	p.mu.Lock()
	if p.registered {
		p.mu.Unlock()
		return
	}
	p.mu.Unlock()

	st, err := p.deps.Home.LoadState()
	if err != nil || st.WorkspaceID == "" {
		return
	}
	session, err := p.session(ctx, st.WorkspaceID)
	if err != nil {
		return
	}
	tools, err := session.ListTools(ctx, nil)
	if err != nil {
		return
	}

	p.mu.Lock()
	defer p.mu.Unlock()
	if p.registered {
		return
	}
	for _, tool := range tools.Tools {
		if tool.Name == "ophion_context" {
			continue // ours wins; the upstream has no such tool today
		}
		p.server.AddTool(cloneTool(tool), p.forward(tool.Name))
	}
	p.registered = true
}

// cloneTool copies the upstream declaration so the client sees Ophion's own
// schema and description, not a paraphrase.
func cloneTool(t *mcp.Tool) *mcp.Tool {
	copied := *t
	if copied.Annotations == nil {
		// Every Ophion read tool is read-only; saying so lets a client treat
		// them as the cheap calls they are.
		copied.Annotations = &mcp.ToolAnnotations{ReadOnlyHint: true}
	}
	return &copied
}

// forward returns a handler that sends the call upstream unchanged and
// annotates the answer with the workspace that produced it.
func (p *proxyServer) forward(name string) mcp.ToolHandler {
	return func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		st, err := p.deps.Home.LoadState()
		if err != nil {
			return nil, err
		}
		if st.WorkspaceID == "" {
			return nil, fmt.Errorf("no workspace selected: call the plasma server's use_workspace first")
		}
		session, err := p.session(ctx, st.WorkspaceID)
		if err != nil {
			return nil, err
		}

		var args any
		if req.Params != nil && len(req.Params.Arguments) > 0 {
			args = json.RawMessage(req.Params.Arguments)
		}
		res, err := session.CallTool(ctx, &mcp.CallToolParams{Name: name, Arguments: args})
		if err != nil {
			// A dead upstream session (Ophion restarted, token rotated) must
			// not poison every later call, so drop it and let the next call
			// dial again.
			p.dropSession(st.WorkspaceID)
			return nil, p.explain(err)
		}
		return annotateResult(p.annotation(st), res), nil
	}
}

// annotateResult prepends the workspace line, leaving the upstream content
// and error flag untouched.
func annotateResult(prefix string, res *mcp.CallToolResult) *mcp.CallToolResult {
	if res == nil {
		return textResult(prefix)
	}
	out := *res
	out.Content = append([]mcp.Content{&mcp.TextContent{Text: prefix}}, res.Content...)
	return &out
}

// session returns a connected upstream session for the workspace, dialing on
// first use. Sessions are cached per workspace so switching back and forth
// does not re-handshake every time.
func (p *proxyServer) session(ctx context.Context, workspace string) (*mcp.ClientSession, error) {
	if p.deps.Config.OphionURL == "" {
		return nil, fmt.Errorf("OPHION_URL is not set: put it in %s", p.deps.Home.ConfigPath())
	}

	p.mu.Lock()
	if session, ok := p.sessions[workspace]; ok {
		p.mu.Unlock()
		return session, nil
	}
	p.mu.Unlock()

	endpoint := fmt.Sprintf("%s/internal/v1/workspaces/%s/query-mcp/%s",
		strings.TrimRight(p.deps.Config.OphionURL, "/"), workspace, p.profileFor(workspace))

	httpClient := p.http
	if token := p.deps.Config.OphionServiceToken; token != "" {
		httpClient = withBearer(httpClient, token)
	}
	client := mcp.NewClient(&mcp.Implementation{Name: "plasma-plugin-ophion", Version: Version}, nil)
	session, err := client.Connect(ctx, &mcp.StreamableClientTransport{
		Endpoint: endpoint, HTTPClient: httpClient, MaxRetries: -1,
	}, nil)
	if err != nil {
		return nil, p.explain(err)
	}

	p.mu.Lock()
	defer p.mu.Unlock()
	if existing, ok := p.sessions[workspace]; ok {
		// Another call won the race; keep one session per workspace.
		go session.Close()
		return existing, nil
	}
	p.sessions[workspace] = session
	return session, nil
}

func (p *proxyServer) dropSession(workspace string) {
	p.mu.Lock()
	session, ok := p.sessions[workspace]
	delete(p.sessions, workspace)
	p.mu.Unlock()
	if ok {
		go session.Close()
	}
}

// explain turns a transport failure into the operational cause. The three
// cases mean genuinely different things, and an operator staring at "connect
// failed" cannot tell them apart.
func (p *proxyServer) explain(err error) error {
	switch p.probe.lastStatus() {
	case http.StatusNotFound:
		return fmt.Errorf("this workspace has no published knowledge in Ophion yet "+
			"(HTTP 404 from the query-mcp endpoint): %w", err)
	case http.StatusUnauthorized, http.StatusForbidden:
		return fmt.Errorf("Ophion rejected the service token "+
			"(check OPHION_SERVICE_TOKEN): %w", err)
	case http.StatusTooManyRequests:
		return fmt.Errorf("Ophion is at its workspace limit for query-mcp sessions: %w", err)
	}
	var netErr *transportError
	if errors.As(err, &netErr) || p.probe.lastStatus() == 0 {
		return fmt.Errorf("Ophion is unreachable at %s: it is a cluster-internal API, "+
			"so a workstation usually needs a port-forward: %w", p.deps.Config.OphionURL, err)
	}
	return err
}

func (p *proxyServer) profile(st pcontext.State) string {
	if st.Profile != "" {
		return st.Profile
	}
	if p.deps.Config.OphionProfile != "" {
		return p.deps.Config.OphionProfile
	}
	return pcontext.DefaultProfile
}

func (p *proxyServer) profileFor(string) string {
	st, err := p.deps.Home.LoadState()
	if err != nil {
		return pcontext.DefaultProfile
	}
	return p.profile(st)
}

func (p *proxyServer) annotation(st pcontext.State) string {
	name := st.WorkspaceName
	if name == "" {
		name = "(unnamed)"
	}
	return fmt.Sprintf("[workspace=%s name=%s profile=%s]", st.WorkspaceID, name, p.profile(st))
}

func textResult(lines ...string) *mcp.CallToolResult {
	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: strings.Join(lines, "\n")}},
	}
}

func orDash(v string) string {
	if v == "" {
		return "-"
	}
	return v
}

func orNotSet(v string) string {
	if v == "" {
		return "(not set)"
	}
	return v
}
