// Package plasmamcp exposes Plasma's control plane as MCP tools: pick a
// workspace, look at views, try a SELECT, create a (materialized) view, and
// publish it as a data API.
//
// The plugin asks for confirmation at data synchronization and API publication,
// including scheduled creation because it starts syncing immediately.
package plasmamcp

import (
	"context"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/BrobridgeOrg/plasma-plugin/internal/pcontext"
	"github.com/BrobridgeOrg/plasma-plugin/internal/plasmaapi"
)

// Version is reported to the MCP client.
var Version = "dev"

// Plasma is the slice of the REST client these tools use. It is an interface
// so the tools can be tested without a Plasma deployment.
type Plasma interface {
	BaseURL() string
	VerifyToken(ctx context.Context) (plasmaapi.User, error)
	MyWorkspaces(ctx context.Context) ([]plasmaapi.Workspace, error)
	ListViews(ctx context.Context, workspace string, q plasmaapi.ViewQuery) (plasmaapi.ViewList, error)
	GetView(ctx context.Context, workspace, viewID string) (plasmaapi.View, error)
	BlueprintHistory(ctx context.Context, workspace, viewID string) ([]plasmaapi.BlueprintHistory, error)
	RunQuery(ctx context.Context, workspace, sql string) (plasmaapi.QueryResult, error)
	CreateView(ctx context.Context, workspace string, req plasmaapi.CreateViewRequest) (plasmaapi.View, error)
	SyncView(ctx context.Context, workspace, viewID string) (string, error)
	CreateAccessEntry(ctx context.Context, workspace, viewID string,
		req plasmaapi.CreateAccessEntryRequest) (plasmaapi.AccessEntryCreated, error)
	ListAccessEntries(ctx context.Context, workspace, viewID string) ([]plasmaapi.AccessEntry, error)
	ExportURL(ctx context.Context, workspace, viewID string) (string, error)
}

// Deps is everything the server needs.
type Deps struct {
	Config pcontext.Config
	Home   *pcontext.Home
	Plasma Plasma
}

type server struct {
	deps Deps
}

const instructions = `Plasma control plane for one workspace at a time.

所有對使用者的進度、說明、問題與交付都使用台灣繁體中文；SQL 與識別名稱保留原樣。
原則上一份表單／報表建立一個 mview，整合所有區塊與指標，不因來源表不同而拆分。
只有使用者明確要求才拆成多個 mview。查找、SELECT 驗證與建立 manual mview
不另加逐步確認；只在開始同步拉資料及後續開啟資料 API 時確認。

Start with whoami (it reports the selected workspace and both endpoints) and
use_workspace to select one; every other tool acts on that selection, and
each answer names the workspace it used.

Discovery of tables, columns and their meaning belongs to the ophion server,
not here: it holds what the source system is and how it was designed,
including whether a relation can be named in SQL at all (access_mode).

run_query executes SELECT against the workspace. Plasma allows only reads and
caps the result at 100 rows, so use it to verify the shape of a result before
building anything on it — never as a way to move data.

The path to a data API is: verify the complete form's SQL with run_query,
create one materialized_view with sync_mode=manual, obtain confirmation in
Taiwan Traditional Chinese that sync will start pulling source data into the
mview, then sync_view and poll get_view until last_sync_status=synced.
Scheduled create_view starts the first sync immediately, so confirm before
that call instead, including the recurring schedule. Do not create a second
mview just to add a schedule. Unverified business assumptions must be stated
and accepted in the sync confirmation. Only after a successful sync, obtain
separate Chinese confirmation for API publication, its authentication and
expiry, then create_access_entry and get_export_url. A confirmation should
cover the actual operation once, without an extra duplicate approval round.`

// NewServer builds the MCP server with all eleven tools registered.
func NewServer(deps Deps) *mcp.Server {
	s := &server{deps: deps}
	srv := mcp.NewServer(&mcp.Implementation{
		Name:    "plasma",
		Title:   "Plasma Control Plane",
		Version: Version,
	}, &mcp.ServerOptions{Instructions: instructions})
	s.register(srv)
	return srv
}

// state reads the shared selection. Both servers re-read it on every call, so
// a switch made in one is visible to the other immediately.
func (s *server) state() (pcontext.State, error) {
	return s.deps.Home.LoadState()
}

// requireWorkspace returns the selection, or an error that names the tool
// which fixes the situation.
func (s *server) requireWorkspace() (pcontext.State, error) {
	st, err := s.state()
	if err != nil {
		return st, err
	}
	if st.WorkspaceID == "" {
		return st, fmt.Errorf("no workspace selected: call use_workspace first " +
			"(list_workspaces shows what this account can reach)")
	}
	return st, nil
}
