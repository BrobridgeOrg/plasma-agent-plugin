package plasmamcp

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/BrobridgeOrg/plasma-plugin/internal/pcontext"
	"github.com/BrobridgeOrg/plasma-plugin/internal/plasmaapi"
)

// fakePlasma records what the tools asked of Plasma and answers with
// canned data. Nothing here talks to a network.
type fakePlasma struct {
	workspaces []plasmaapi.Workspace
	user       plasmaapi.User
	view       plasmaapi.View
	views      plasmaapi.ViewList
	history    []plasmaapi.BlueprintHistory
	query      plasmaapi.QueryResult
	created    plasmaapi.AccessEntryCreated
	entries    []plasmaapi.AccessEntry
	exportURL  string
	syncMsg    string
	err        error

	queriedSQL       string
	queriedWorkspace string
	createdView      *plasmaapi.CreateViewRequest
	createdEntry     *plasmaapi.CreateAccessEntryRequest
	syncedView       string
	viewQuery        plasmaapi.ViewQuery
	calls            int
}

func (f *fakePlasma) BaseURL() string { return "http://plasma.test" }

func (f *fakePlasma) VerifyToken(context.Context) (plasmaapi.User, error) {
	f.calls++
	return f.user, f.err
}

func (f *fakePlasma) MyWorkspaces(context.Context) ([]plasmaapi.Workspace, error) {
	f.calls++
	return f.workspaces, f.err
}

func (f *fakePlasma) ListViews(_ context.Context, ws string, q plasmaapi.ViewQuery) (plasmaapi.ViewList, error) {
	f.calls++
	f.queriedWorkspace, f.viewQuery = ws, q
	return f.views, f.err
}

func (f *fakePlasma) GetView(_ context.Context, ws, id string) (plasmaapi.View, error) {
	f.calls++
	f.queriedWorkspace = ws
	return f.view, f.err
}

func (f *fakePlasma) BlueprintHistory(context.Context, string, string) ([]plasmaapi.BlueprintHistory, error) {
	f.calls++
	return f.history, f.err
}

func (f *fakePlasma) RunQuery(_ context.Context, ws, sql string) (plasmaapi.QueryResult, error) {
	f.calls++
	f.queriedWorkspace, f.queriedSQL = ws, sql
	return f.query, f.err
}

func (f *fakePlasma) CreateView(_ context.Context, ws string, req plasmaapi.CreateViewRequest) (plasmaapi.View, error) {
	f.calls++
	f.queriedWorkspace, f.createdView = ws, &req
	return f.view, f.err
}

func (f *fakePlasma) SyncView(_ context.Context, ws, id string) (string, error) {
	f.calls++
	f.queriedWorkspace, f.syncedView = ws, id
	return f.syncMsg, f.err
}

func (f *fakePlasma) CreateAccessEntry(_ context.Context, ws, viewID string,
	req plasmaapi.CreateAccessEntryRequest) (plasmaapi.AccessEntryCreated, error) {
	f.calls++
	f.queriedWorkspace, f.createdEntry = ws, &req
	return f.created, f.err
}

func (f *fakePlasma) ListAccessEntries(context.Context, string, string) ([]plasmaapi.AccessEntry, error) {
	f.calls++
	return f.entries, f.err
}

func (f *fakePlasma) ExportURL(context.Context, string, string) (string, error) {
	f.calls++
	return f.exportURL, f.err
}

type session struct {
	t      *testing.T
	client *mcp.ClientSession
	home   *pcontext.Home
	plasma *fakePlasma
}

func newSession(t *testing.T, plasma *fakePlasma, state pcontext.State) *session {
	t.Helper()
	home := pcontext.New(t.TempDir())
	if state != (pcontext.State{}) {
		if err := home.SaveState(state); err != nil {
			t.Fatal(err)
		}
	}
	server := NewServer(Deps{
		Config: pcontext.Config{OphionURL: "http://ophion.test", OphionProfile: "all"},
		Home:   home,
		Plasma: plasma,
	})

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

	return &session{t: t, client: client, home: home, plasma: plasma}
}

func (s *session) call(name string, args map[string]any) *mcp.CallToolResult {
	s.t.Helper()
	res, err := s.client.CallTool(context.Background(), &mcp.CallToolParams{
		Name: name, Arguments: args,
	})
	if err != nil {
		s.t.Fatalf("CallTool %s: %v", name, err)
	}
	return res
}

func text(t *testing.T, res *mcp.CallToolResult) string {
	t.Helper()
	var b strings.Builder
	for _, c := range res.Content {
		if tc, ok := c.(*mcp.TextContent); ok {
			b.WriteString(tc.Text)
		}
	}
	return b.String()
}

func structured(t *testing.T, res *mcp.CallToolResult, out any) {
	t.Helper()
	raw, err := json.Marshal(res.StructuredContent)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, out); err != nil {
		t.Fatalf("structured content %s: %v", raw, err)
	}
}

func selectedState() pcontext.State {
	return pcontext.State{WorkspaceID: "ws-1", WorkspaceName: "his_database", Profile: "all"}
}

func TestToolListCoversTheDocumentedSurface(t *testing.T) {
	s := newSession(t, &fakePlasma{}, pcontext.State{})

	res, err := s.client.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]bool{}
	for _, tool := range res.Tools {
		got[tool.Name] = true
	}
	want := []string{
		"whoami", "list_workspaces", "use_workspace", "list_views", "get_view",
		"run_query", "create_view", "sync_view", "create_access_entry",
		"list_access_entries", "get_export_url",
	}
	for _, name := range want {
		if !got[name] {
			t.Errorf("tool %q is missing", name)
		}
	}
	if len(res.Tools) != len(want) {
		t.Errorf("%d tools registered, want exactly %d", len(res.Tools), len(want))
	}
}

func TestToolAnnotationsDistinguishQueriesFromMutations(t *testing.T) {
	s := newSession(t, &fakePlasma{}, pcontext.State{})

	res, err := s.client.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	readOnly := map[string]bool{}
	for _, tool := range res.Tools {
		readOnly[tool.Name] = tool.Annotations != nil && tool.Annotations.ReadOnlyHint
	}
	for _, name := range []string{"create_view", "sync_view", "create_access_entry"} {
		if readOnly[name] {
			t.Errorf("%s is marked read-only; it changes state or exposes data", name)
		}
	}
	for _, name := range []string{"whoami", "list_workspaces", "list_views", "get_view",
		"list_access_entries", "get_export_url", "run_query"} {
		if !readOnly[name] {
			t.Errorf("%s should be marked read-only", name)
		}
	}
}

func TestUseWorkspaceByIDPersistsSelection(t *testing.T) {
	plasma := &fakePlasma{workspaces: []plasmaapi.Workspace{
		{ID: "ws-1", Name: "his_database"}, {ID: "ws-2", Name: "sales"},
	}}
	s := newSession(t, plasma, pcontext.State{})

	res := s.call("use_workspace", map[string]any{"workspace": "ws-2"})
	if res.IsError {
		t.Fatalf("unexpected error: %s", text(t, res))
	}

	st, err := s.home.LoadState()
	if err != nil {
		t.Fatal(err)
	}
	if st.WorkspaceID != "ws-2" || st.WorkspaceName != "sales" {
		t.Errorf("state = %+v, want ws-2/sales", st)
	}
}

func TestUseWorkspaceResolvesAName(t *testing.T) {
	plasma := &fakePlasma{workspaces: []plasmaapi.Workspace{
		{ID: "ws-1", Name: "his_database"}, {ID: "ws-2", Name: "sales"},
	}}
	s := newSession(t, plasma, pcontext.State{})

	if res := s.call("use_workspace", map[string]any{"workspace": "his_database"}); res.IsError {
		t.Fatalf("unexpected error: %s", text(t, res))
	}

	st, _ := s.home.LoadState()
	if st.WorkspaceID != "ws-1" {
		t.Errorf("workspace = %q, want the id behind the name", st.WorkspaceID)
	}
}

func TestUseWorkspaceRefusesAnUnknownName(t *testing.T) {
	plasma := &fakePlasma{workspaces: []plasmaapi.Workspace{{ID: "ws-1", Name: "his_database"}}}
	s := newSession(t, plasma, pcontext.State{})

	res := s.call("use_workspace", map[string]any{"workspace": "nope"})
	if !res.IsError {
		t.Fatal("want an error for an unknown workspace")
	}
	if !strings.Contains(text(t, res), "his_database") {
		t.Errorf("error = %s, want the available workspaces listed", text(t, res))
	}
	st, _ := s.home.LoadState()
	if st.WorkspaceID != "" {
		t.Errorf("state changed to %q despite the refusal", st.WorkspaceID)
	}
}

func TestUseWorkspaceRefusesAnAmbiguousName(t *testing.T) {
	plasma := &fakePlasma{workspaces: []plasmaapi.Workspace{
		{ID: "ws-1", Name: "shared"}, {ID: "ws-2", Name: "shared"},
	}}
	s := newSession(t, plasma, pcontext.State{})

	res := s.call("use_workspace", map[string]any{"workspace": "shared"})
	if !res.IsError {
		t.Fatal("want a refusal: two workspaces answer to this name")
	}
	if !strings.Contains(text(t, res), "ws-1") || !strings.Contains(text(t, res), "ws-2") {
		t.Errorf("error = %s, want both candidate ids", text(t, res))
	}
}

func TestUseWorkspaceKeepsTheStoredToken(t *testing.T) {
	plasma := &fakePlasma{workspaces: []plasmaapi.Workspace{{ID: "ws-2", Name: "sales"}}}
	s := newSession(t, plasma, pcontext.State{
		WorkspaceID: "ws-1", AccessToken: "at", RefreshToken: "rt",
		ExpiresAt: time.Now().Add(time.Hour).UTC(),
	})

	if res := s.call("use_workspace", map[string]any{"workspace": "ws-2"}); res.IsError {
		t.Fatalf("unexpected error: %s", text(t, res))
	}

	st, _ := s.home.LoadState()
	if st.AccessToken != "at" || st.RefreshToken != "rt" {
		t.Errorf("switching the workspace dropped the session tokens: %+v", st)
	}
}

func TestWorkspaceScopedToolWithoutSelectionIsActionable(t *testing.T) {
	plasma := &fakePlasma{}
	s := newSession(t, plasma, pcontext.State{})

	res := s.call("list_views", map[string]any{})
	if !res.IsError {
		t.Fatal("want an error when no workspace is selected")
	}
	if !strings.Contains(text(t, res), "use_workspace") {
		t.Errorf("error = %s, want it to name the tool that fixes this", text(t, res))
	}
	if plasma.calls != 0 {
		t.Errorf("%d Plasma calls made, want none", plasma.calls)
	}
}

func TestRunQueryRejectsNonSelectBeforeCallingPlasma(t *testing.T) {
	plasma := &fakePlasma{}
	s := newSession(t, plasma, selectedState())

	res := s.call("run_query", map[string]any{"sql": "DELETE FROM patients"})
	if !res.IsError {
		t.Fatal("want a refusal for a DML statement")
	}
	if plasma.calls != 0 {
		t.Errorf("%d Plasma calls made, want none: the refusal must precede the request", plasma.calls)
	}
}

func TestRunQueryRejectsMultipleStatements(t *testing.T) {
	plasma := &fakePlasma{}
	s := newSession(t, selectedFake(plasma), selectedState())

	res := s.call("run_query", map[string]any{"sql": "SELECT 1; DROP TABLE t"})
	if !res.IsError {
		t.Fatal("want a refusal for two statements in one call")
	}
	if plasma.calls != 0 {
		t.Errorf("%d Plasma calls made, want none", plasma.calls)
	}
}

func TestRunQueryAcceptsATrailingSemicolon(t *testing.T) {
	plasma := &fakePlasma{query: plasmaapi.QueryResult{Columns: []string{"n"}, Rows: [][]any{{1}}}}
	s := newSession(t, plasma, selectedState())

	if res := s.call("run_query", map[string]any{"sql": "SELECT 1;"}); res.IsError {
		t.Fatalf("a trailing semicolon is one statement: %s", text(t, res))
	}
}

func TestRunQueryAcceptsACommonTableExpression(t *testing.T) {
	plasma := &fakePlasma{query: plasmaapi.QueryResult{Columns: []string{"n"}, Rows: [][]any{{1}}}}
	s := newSession(t, plasma, selectedState())

	res := s.call("run_query", map[string]any{"sql": "WITH x AS (SELECT 1 AS n) SELECT n FROM x"})
	if res.IsError {
		t.Fatalf("WITH is a read: %s", text(t, res))
	}
	if !strings.HasPrefix(plasma.queriedSQL, "WITH") {
		t.Errorf("sql sent = %q", plasma.queriedSQL)
	}
}

func TestRunQueryReportsRowsAndTheWorkspaceItUsed(t *testing.T) {
	plasma := &fakePlasma{query: plasmaapi.QueryResult{
		QueryID: "q-1", Columns: []string{"id", "name"},
		Rows: [][]any{{1, "Alice"}, {2, nil}}, ElapsedMs: 42,
	}}
	s := newSession(t, plasma, selectedState())

	res := s.call("run_query", map[string]any{"sql": "SELECT id, name FROM t"})
	if res.IsError {
		t.Fatalf("unexpected error: %s", text(t, res))
	}
	body := text(t, res)
	if !strings.Contains(body, "workspace=ws-1") {
		t.Errorf("output = %s, want the workspace annotated", body)
	}
	if !strings.Contains(body, "Alice") {
		t.Errorf("output = %s, want the rows rendered", body)
	}
	var out struct {
		RowCount int `json:"row_count"`
	}
	structured(t, res, &out)
	if out.RowCount != 2 {
		t.Errorf("row_count = %d, want 2", out.RowCount)
	}
	if plasma.queriedWorkspace != "ws-1" {
		t.Errorf("queried workspace = %q", plasma.queriedWorkspace)
	}
}

func TestRunQueryMentionsPlasmasHundredRowCeiling(t *testing.T) {
	rows := make([][]any, 100)
	for i := range rows {
		rows[i] = []any{i}
	}
	plasma := &fakePlasma{query: plasmaapi.QueryResult{Columns: []string{"n"}, Rows: rows}}
	s := newSession(t, plasma, selectedState())

	body := text(t, s.call("run_query", map[string]any{"sql": "SELECT n FROM t"}))
	if !strings.Contains(body, "100") {
		t.Errorf("output = %s, want the ceiling mentioned when the result sits on it", body)
	}
}

func TestCreateViewRejectsAnUnknownType(t *testing.T) {
	plasma := &fakePlasma{}
	s := newSession(t, plasma, selectedState())

	res := s.call("create_view", map[string]any{
		"name": "x", "type": "table", "view_sql": "SELECT 1", "sync_mode": "manual",
	})
	if !res.IsError {
		t.Fatal("want a refusal for an unsupported view type")
	}
	if plasma.calls != 0 {
		t.Errorf("%d Plasma calls made, want none", plasma.calls)
	}
}

func TestCreateViewRejectsScheduledModeWithoutSettings(t *testing.T) {
	plasma := &fakePlasma{}
	s := newSession(t, plasma, selectedState())

	res := s.call("create_view", map[string]any{
		"name": "x", "type": "materialized_view", "view_sql": "SELECT 1",
		"sync_mode": "scheduled",
	})
	if !res.IsError {
		t.Fatal("want a refusal: scheduled sync with no schedule would never run")
	}
	if plasma.calls != 0 {
		t.Errorf("%d Plasma calls made, want none", plasma.calls)
	}
}

func TestCreateViewPassesSchedulerSettingsThrough(t *testing.T) {
	plasma := &fakePlasma{view: plasmaapi.View{ID: "v-1", Status: "initializing"}}
	s := newSession(t, plasma, selectedState())

	res := s.call("create_view", map[string]any{
		"name": "daily", "type": "materialized_view", "view_sql": "SELECT 1",
		"sync_mode": "scheduled",
		"scheduler_settings": map[string]any{
			"start": "immediately", "frequency": "repeat",
			"repeat_interval": 1, "repeat_interval_unit": "3600",
		},
	})
	if res.IsError {
		t.Fatalf("unexpected error: %s", text(t, res))
	}
	if plasma.createdView == nil {
		t.Fatal("Plasma was never asked to create the view")
	}
	if !strings.Contains(string(plasma.createdView.SchedulerSettings), `"frequency":"repeat"`) {
		t.Errorf("scheduler_settings sent = %s", plasma.createdView.SchedulerSettings)
	}
}

func TestCreateViewRejectsNonSelectSQL(t *testing.T) {
	plasma := &fakePlasma{}
	s := newSession(t, plasma, selectedState())

	res := s.call("create_view", map[string]any{
		"name": "x", "type": "materialized_view", "view_sql": "DROP TABLE t",
		"sync_mode": "manual",
	})
	if !res.IsError {
		t.Fatal("want a refusal: a view body must be a read")
	}
	if plasma.calls != 0 {
		t.Errorf("%d Plasma calls made, want none", plasma.calls)
	}
}

func TestSyncViewReportsTheServerMessage(t *testing.T) {
	plasma := &fakePlasma{syncMsg: "sync started"}
	s := newSession(t, plasma, selectedState())

	res := s.call("sync_view", map[string]any{"view_id": "v-1"})
	if res.IsError {
		t.Fatalf("unexpected error: %s", text(t, res))
	}
	if plasma.syncedView != "v-1" {
		t.Errorf("synced view = %q", plasma.syncedView)
	}
	if !strings.Contains(text(t, res), "sync started") {
		t.Errorf("output = %s", text(t, res))
	}
}

func TestGetViewIncludesBlueprintHistoryOnRequest(t *testing.T) {
	plasma := &fakePlasma{
		view:    plasmaapi.View{ID: "v-1", Name: "daily", LastSyncStatus: "synced"},
		history: []plasmaapi.BlueprintHistory{{BlueprintID: "bp-1"}},
	}
	s := newSession(t, plasma, selectedState())

	body := text(t, s.call("get_view", map[string]any{
		"view_id": "v-1", "with_blueprint_history": true,
	}))
	if !strings.Contains(body, "bp-1") {
		t.Errorf("output = %s, want the blueprint history", body)
	}
}

func TestGetViewSkipsBlueprintHistoryByDefault(t *testing.T) {
	plasma := &fakePlasma{
		view:    plasmaapi.View{ID: "v-1"},
		history: []plasmaapi.BlueprintHistory{{BlueprintID: "bp-1"}},
	}
	s := newSession(t, plasma, selectedState())

	body := text(t, s.call("get_view", map[string]any{"view_id": "v-1"}))
	if strings.Contains(body, "bp-1") {
		t.Errorf("output = %s, want no history unless it was asked for", body)
	}
}

func TestCreateAccessEntryTurnsExpiresInIntoATimestamp(t *testing.T) {
	plasma := &fakePlasma{created: plasmaapi.AccessEntryCreated{
		Entry:     plasmaapi.AccessEntry{ID: "e-1", AuthType: "api_key", SecretKey: "sk"},
		AccessURL: "http://plasma/public/abc",
	}}
	s := newSession(t, plasma, selectedState())

	res := s.call("create_access_entry", map[string]any{
		"view_id": "v-1", "name": "powerbi", "auth_type": "api_key", "expires_in": "30d",
	})
	if res.IsError {
		t.Fatalf("unexpected error: %s", text(t, res))
	}
	if plasma.createdEntry == nil || plasma.createdEntry.ExpiredAt == nil {
		t.Fatal("expiry was not sent to Plasma")
	}
	want := time.Now().Add(30 * 24 * time.Hour)
	if diff := plasma.createdEntry.ExpiredAt.Sub(want); diff > time.Minute || diff < -time.Minute {
		t.Errorf("expiry = %v, want about %v", plasma.createdEntry.ExpiredAt, want)
	}
}

func TestCreateAccessEntryRejectsAnUnparseableExpiry(t *testing.T) {
	plasma := &fakePlasma{}
	s := newSession(t, plasma, selectedState())

	res := s.call("create_access_entry", map[string]any{
		"view_id": "v-1", "name": "x", "auth_type": "api_key", "expires_in": "soon",
	})
	if !res.IsError {
		t.Fatal("want a refusal for an unparseable duration")
	}
	if plasma.calls != 0 {
		t.Errorf("%d Plasma calls made, want none", plasma.calls)
	}
}

func TestCreateAccessEntryWithoutExpiryLeavesItToPlasma(t *testing.T) {
	plasma := &fakePlasma{created: plasmaapi.AccessEntryCreated{
		Entry: plasmaapi.AccessEntry{ID: "e-1", AuthType: "api_key"},
	}}
	s := newSession(t, plasma, selectedState())

	res := s.call("create_access_entry", map[string]any{
		"view_id": "v-1", "name": "x", "auth_type": "api_key",
	})
	if res.IsError {
		t.Fatalf("unexpected error: %s", text(t, res))
	}
	if plasma.createdEntry.ExpiredAt != nil {
		t.Errorf("expiry = %v, want nil so Plasma decides", plasma.createdEntry.ExpiredAt)
	}
	if !strings.Contains(strings.ToLower(text(t, res)), "expir") {
		t.Errorf("output = %s, want the expiry situation stated either way", text(t, res))
	}
}

func TestCreateAccessEntryFlagsAnUnauthenticatedEndpoint(t *testing.T) {
	plasma := &fakePlasma{created: plasmaapi.AccessEntryCreated{
		Entry:     plasmaapi.AccessEntry{ID: "e-1", AuthType: "none"},
		AccessURL: "http://plasma/public/abc",
	}}
	s := newSession(t, plasma, selectedState())

	body := text(t, s.call("create_access_entry", map[string]any{
		"view_id": "v-1", "name": "open", "auth_type": "none",
	}))
	if !strings.Contains(strings.ToLower(body), "no authentication") {
		t.Errorf("output = %s, want the public exposure stated plainly", body)
	}
}

func TestCreateAccessEntryRejectsAnUnknownAuthType(t *testing.T) {
	plasma := &fakePlasma{}
	s := newSession(t, plasma, selectedState())

	res := s.call("create_access_entry", map[string]any{
		"view_id": "v-1", "name": "x", "auth_type": "oauth",
	})
	if !res.IsError {
		t.Fatal("want a refusal for an auth type Plasma does not implement")
	}
	if plasma.calls != 0 {
		t.Errorf("%d Plasma calls made, want none", plasma.calls)
	}
}

func TestBasicAuthRequiresAUsernamePasswordSecret(t *testing.T) {
	plasma := &fakePlasma{}
	s := newSession(t, plasma, selectedState())

	res := s.call("create_access_entry", map[string]any{
		"view_id": "v-1", "name": "x", "auth_type": "basic_auth", "secret_key": "justapassword",
	})
	if !res.IsError {
		t.Fatal("want a refusal: basic_auth secrets are username:password")
	}
	if plasma.calls != 0 {
		t.Errorf("%d Plasma calls made, want none", plasma.calls)
	}
}

func TestGetExportURLExplainsAnUnsyncedView(t *testing.T) {
	plasma := &fakePlasma{err: plasmaapi.ErrNotReady}
	s := newSession(t, plasma, selectedState())

	res := s.call("get_export_url", map[string]any{"view_id": "v-1"})
	if !res.IsError {
		t.Fatal("want an error")
	}
	if !strings.Contains(strings.ToLower(text(t, res)), "sync") {
		t.Errorf("error = %s, want it to say the view has no successful sync yet", text(t, res))
	}
}

func TestWhoamiReportsBothEndpointsAndTheSelection(t *testing.T) {
	plasma := &fakePlasma{user: plasmaapi.User{Username: "admin", Role: "admin"}}
	s := newSession(t, plasma, selectedState())

	body := text(t, s.call("whoami", map[string]any{}))
	for _, want := range []string{"admin", "his_database", "http://plasma.test", "http://ophion.test"} {
		if !strings.Contains(body, want) {
			t.Errorf("output = %s, want %q in it", body, want)
		}
	}
}

func TestWhoamiWorksBeforeAnyWorkspaceIsChosen(t *testing.T) {
	plasma := &fakePlasma{user: plasmaapi.User{Username: "admin"}}
	s := newSession(t, plasma, pcontext.State{})

	res := s.call("whoami", map[string]any{})
	if res.IsError {
		t.Fatalf("whoami is the diagnostic tool; it must answer without a workspace: %s", text(t, res))
	}
}

func TestPlasmaFailuresSurfaceAsToolErrors(t *testing.T) {
	plasma := &fakePlasma{err: errors.New("boom")}
	s := newSession(t, plasma, selectedState())

	res := s.call("list_views", map[string]any{})
	if !res.IsError {
		t.Fatal("want the failure reported")
	}
	if !strings.Contains(text(t, res), "boom") {
		t.Errorf("error = %s, want the underlying message", text(t, res))
	}
}

func TestListViewsForwardsPagination(t *testing.T) {
	plasma := &fakePlasma{views: plasmaapi.ViewList{Total: 0}}
	s := newSession(t, plasma, selectedState())

	if res := s.call("list_views", map[string]any{
		"keywords": "daily", "page": 2, "page_size": 5,
	}); res.IsError {
		t.Fatalf("unexpected error: %s", text(t, res))
	}
	if plasma.viewQuery != (plasmaapi.ViewQuery{Keywords: "daily", Page: 2, PageSize: 5}) {
		t.Errorf("query = %+v", plasma.viewQuery)
	}
}

func TestListWorkspacesMarksTheCurrentOne(t *testing.T) {
	plasma := &fakePlasma{workspaces: []plasmaapi.Workspace{
		{ID: "ws-1", Name: "his_database"}, {ID: "ws-2", Name: "sales"},
	}}
	s := newSession(t, plasma, selectedState())

	var out struct {
		Current string `json:"current_workspace_id"`
	}
	structured(t, s.call("list_workspaces", map[string]any{}), &out)
	if out.Current != "ws-1" {
		t.Errorf("current_workspace_id = %q, want ws-1", out.Current)
	}
}

// selectedFake keeps the helper honest when a test only needs the recorder.
func selectedFake(f *fakePlasma) *fakePlasma { return f }

func TestBasicAuthRequiresAnExplicitSecret(t *testing.T) {
	plasma := &fakePlasma{}
	s := newSession(t, plasma, selectedState())

	res := s.call("create_access_entry", map[string]any{
		"view_id": "v-1", "name": "x", "auth_type": "basic_auth",
	})
	if !res.IsError {
		t.Fatal("want a refusal: an auto-generated secret cannot be username:password")
	}
	if plasma.calls != 0 {
		t.Errorf("%d Plasma calls made, want none", plasma.calls)
	}
}
