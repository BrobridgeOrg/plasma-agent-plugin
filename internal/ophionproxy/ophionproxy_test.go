package ophionproxy

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/BrobridgeOrg/plasma-plugin/internal/pcontext"
)

// upstream is a stand-in Ophion: it serves a real MCP server behind the same
// URL shape and bearer requirement as query_mcp_service.
type upstream struct {
	server *httptest.Server

	mu         sync.Mutex
	workspaces []string // workspaces addressed, in order
	profiles   []string
	tokens     []string

	status int // when non-zero, refuse every request with this status
}

func newUpstream(t *testing.T) *upstream {
	t.Helper()
	u := &upstream{}

	knowledge := mcp.NewServer(&mcp.Implementation{
		Name: "ophion-knowledge", Version: "test",
	}, &mcp.ServerOptions{Instructions: "upstream instructions"})

	type overviewIn struct{}
	type overviewOut struct {
		Databases []string `json:"databases"`
	}
	mcp.AddTool(knowledge, &mcp.Tool{
		Name: "overview", Description: "what this workspace holds",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, _ overviewIn) (
		*mcp.CallToolResult, overviewOut, error) {
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: "2 databases"}},
		}, overviewOut{Databases: []string{"hosp", "icu"}}, nil
	})

	type searchIn struct {
		Query string `json:"query"`
	}
	mcp.AddTool(knowledge, &mcp.Tool{
		Name: "search_knowledge", Description: "search the graph",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in searchIn) (
		*mcp.CallToolResult, overviewOut, error) {
		if in.Query == "explode" {
			return &mcp.CallToolResult{
				IsError: true,
				Content: []mcp.Content{&mcp.TextContent{Text: "upstream refused: bad query"}},
			}, overviewOut{}, nil
		}
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: "matched query=" + in.Query}},
		}, overviewOut{}, nil
	})

	mcpHandler := mcp.NewStreamableHTTPHandler(
		func(*http.Request) *mcp.Server { return knowledge }, nil)

	mux := http.NewServeMux()
	mux.HandleFunc("/internal/v1/workspaces/{ws}/query-mcp/{profile}",
		func(w http.ResponseWriter, r *http.Request) {
			u.mu.Lock()
			u.workspaces = append(u.workspaces, r.PathValue("ws"))
			u.profiles = append(u.profiles, r.PathValue("profile"))
			u.tokens = append(u.tokens, r.Header.Get("Authorization"))
			status := u.status
			u.mu.Unlock()

			if status != 0 {
				http.Error(w, `{"error":"refused"}`, status)
				return
			}
			mcpHandler.ServeHTTP(w, r)
		})

	u.server = httptest.NewServer(mux)
	t.Cleanup(u.server.Close)
	return u
}

func (u *upstream) addressed() []string {
	u.mu.Lock()
	defer u.mu.Unlock()
	return append([]string(nil), u.workspaces...)
}

func (u *upstream) lastToken() string {
	u.mu.Lock()
	defer u.mu.Unlock()
	if len(u.tokens) == 0 {
		return ""
	}
	return u.tokens[len(u.tokens)-1]
}

func (u *upstream) lastProfile() string {
	u.mu.Lock()
	defer u.mu.Unlock()
	if len(u.profiles) == 0 {
		return ""
	}
	return u.profiles[len(u.profiles)-1]
}

type proxy struct {
	t      *testing.T
	client *mcp.ClientSession
	home   *pcontext.Home
}

func newProxy(t *testing.T, ophionURL string, state pcontext.State) *proxy {
	t.Helper()
	home := pcontext.New(t.TempDir())
	if state != (pcontext.State{}) {
		if err := home.SaveState(state); err != nil {
			t.Fatal(err)
		}
	}
	server, closer := NewServer(Deps{
		Config: pcontext.Config{
			OphionURL:          ophionURL,
			OphionServiceToken: "svc-token",
			OphionProfile:      "all",
		},
		Home: home,
	})
	// Registered before the client/server sessions so it runs after them, and
	// before the upstream httptest server is torn down: the streamable
	// transport holds a long-lived stream that would otherwise block Close.
	t.Cleanup(func() { _ = closer.Close() })

	serverTransport, clientTransport := mcp.NewInMemoryTransports()
	ctx := context.Background()
	serverSession, err := server.Connect(ctx, serverTransport, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = serverSession.Close() })

	client, err := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "0"}, nil).
		Connect(ctx, clientTransport, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = client.Close() })

	return &proxy{t: t, client: client, home: home}
}

func (p *proxy) toolNames() map[string]bool {
	p.t.Helper()
	res, err := p.client.ListTools(context.Background(), nil)
	if err != nil {
		p.t.Fatal(err)
	}
	names := map[string]bool{}
	for _, tool := range res.Tools {
		names[tool.Name] = true
	}
	return names
}

func (p *proxy) call(name string, args map[string]any) *mcp.CallToolResult {
	p.t.Helper()
	res, err := p.client.CallTool(context.Background(), &mcp.CallToolParams{
		Name: name, Arguments: args,
	})
	if err != nil {
		p.t.Fatalf("CallTool %s: %v", name, err)
	}
	return res
}

func resultText(t *testing.T, res *mcp.CallToolResult) string {
	t.Helper()
	var b strings.Builder
	for _, c := range res.Content {
		if tc, ok := c.(*mcp.TextContent); ok {
			b.WriteString(tc.Text)
			b.WriteString("\n")
		}
	}
	return b.String()
}

func selected(ws, name string) pcontext.State {
	return pcontext.State{WorkspaceID: ws, WorkspaceName: name, Profile: "all"}
}

func TestUpstreamToolsAreExposedVerbatim(t *testing.T) {
	up := newUpstream(t)
	p := newProxy(t, up.server.URL, selected("ws-1", "his_database"))

	names := p.toolNames()
	for _, want := range []string{"overview", "search_knowledge", "ophion_context"} {
		if !names[want] {
			t.Errorf("tool %q missing; got %v", want, names)
		}
	}
}

func TestForwardedCallReachesTheSelectedWorkspace(t *testing.T) {
	up := newUpstream(t)
	p := newProxy(t, up.server.URL, selected("ws-1", "his_database"))

	res := p.call("overview", map[string]any{})
	if res.IsError {
		t.Fatalf("unexpected error: %s", resultText(t, res))
	}
	if got := up.addressed(); len(got) == 0 || got[0] != "ws-1" {
		t.Fatalf("upstream addressed %v, want ws-1", got)
	}
	if up.lastToken() != "Bearer svc-token" {
		t.Errorf("Authorization = %q", up.lastToken())
	}
	if up.lastProfile() != "all" {
		t.Errorf("profile = %q", up.lastProfile())
	}
}

func TestForwardedResultCarriesTheWorkspaceAnnotation(t *testing.T) {
	up := newUpstream(t)
	p := newProxy(t, up.server.URL, selected("ws-1", "his_database"))

	body := resultText(t, p.call("overview", map[string]any{}))
	if !strings.Contains(body, "workspace=ws-1") {
		t.Errorf("output = %s, want the workspace annotated on every answer", body)
	}
	if !strings.Contains(body, "2 databases") {
		t.Errorf("output = %s, want the upstream content preserved", body)
	}
}

func TestArgumentsAreForwardedVerbatim(t *testing.T) {
	up := newUpstream(t)
	p := newProxy(t, up.server.URL, selected("ws-1", "his_database"))

	body := resultText(t, p.call("search_knowledge", map[string]any{"query": "length of stay"}))
	if !strings.Contains(body, "matched query=length of stay") {
		t.Errorf("output = %s", body)
	}
}

func TestSwitchingWorkspaceRedirectsTheNextCall(t *testing.T) {
	up := newUpstream(t)
	p := newProxy(t, up.server.URL, selected("ws-1", "his_database"))

	p.call("overview", map[string]any{})

	// This is what plasma's use_workspace does: it only writes the file.
	if err := p.home.SaveState(selected("ws-2", "sales")); err != nil {
		t.Fatal(err)
	}

	body := resultText(t, p.call("overview", map[string]any{}))
	if !strings.Contains(body, "workspace=ws-2") {
		t.Errorf("output = %s, want the new workspace", body)
	}
	addressed := up.addressed()
	if len(addressed) < 2 || addressed[len(addressed)-1] != "ws-2" {
		t.Errorf("upstream addressed %v, want the last call on ws-2", addressed)
	}
}

func TestUpstreamToolErrorIsPassedThrough(t *testing.T) {
	up := newUpstream(t)
	p := newProxy(t, up.server.URL, selected("ws-1", "his_database"))

	res := p.call("search_knowledge", map[string]any{"query": "explode"})
	if !res.IsError {
		t.Fatal("want the upstream refusal preserved as an error")
	}
	if !strings.Contains(resultText(t, res), "upstream refused") {
		t.Errorf("output = %s", resultText(t, res))
	}
}

func TestNoWorkspaceSelectedIsActionable(t *testing.T) {
	up := newUpstream(t)
	p := newProxy(t, up.server.URL, pcontext.State{})

	res := p.call("ophion_context", map[string]any{})
	if res.IsError {
		t.Fatalf("ophion_context is the diagnostic tool: %s", resultText(t, res))
	}
	if !strings.Contains(resultText(t, res), "use_workspace") {
		t.Errorf("output = %s, want it to name the tool that fixes this", resultText(t, res))
	}
	if len(up.addressed()) != 0 {
		t.Errorf("upstream was contacted %v times with no workspace selected", up.addressed())
	}
}

func TestToolsAppearOnceAWorkspaceIsSelected(t *testing.T) {
	up := newUpstream(t)
	p := newProxy(t, up.server.URL, pcontext.State{})

	if p.toolNames()["overview"] {
		t.Fatal("upstream tools cannot be known before a workspace is selected")
	}

	if err := p.home.SaveState(selected("ws-1", "his_database")); err != nil {
		t.Fatal(err)
	}

	if !p.toolNames()["overview"] {
		t.Error("after a workspace is selected, the upstream tools must become visible")
	}
}

func TestUnknownWorkspaceExplainsMissingKnowledge(t *testing.T) {
	up := newUpstream(t)
	up.status = http.StatusNotFound
	p := newProxy(t, up.server.URL, selected("ws-1", "his_database"))

	res := p.call("ophion_context", map[string]any{})
	body := strings.ToLower(resultText(t, res))
	if !strings.Contains(body, "no published knowledge") {
		t.Errorf("output = %s, want a 404 read as 'this workspace has no knowledge yet'", body)
	}
}

func TestRejectedServiceTokenIsNamed(t *testing.T) {
	up := newUpstream(t)
	up.status = http.StatusUnauthorized
	p := newProxy(t, up.server.URL, selected("ws-1", "his_database"))

	body := strings.ToLower(resultText(t, p.call("ophion_context", map[string]any{})))
	if !strings.Contains(body, "service token") {
		t.Errorf("output = %s, want a 401 read as a service-token problem", body)
	}
}

func TestUnreachableOphionMentionsTheClusterEndpoint(t *testing.T) {
	p := newProxy(t, "http://127.0.0.1:1", selected("ws-1", "his_database"))

	body := strings.ToLower(resultText(t, p.call("ophion_context", map[string]any{})))
	if !strings.Contains(body, "unreachable") {
		t.Errorf("output = %s, want the connection failure named", body)
	}
	if !strings.Contains(body, "port-forward") {
		t.Errorf("output = %s, want the cluster-internal hint", body)
	}
}

func TestMissingOphionURLIsReportedNotDialed(t *testing.T) {
	p := newProxy(t, "", selected("ws-1", "his_database"))

	body := resultText(t, p.call("ophion_context", map[string]any{}))
	if !strings.Contains(body, "OPHION_URL") {
		t.Errorf("output = %s, want the unset setting named", body)
	}
}
