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
	ListPGConnections(context.Context, string, plasmaapi.ViewQuery) (plasmaapi.PGConnectionList, error)
	GetPGConnection(context.Context, string, string) (plasmaapi.PGConnection, error)
	CreatePGBlueprint(context.Context, string, plasmaapi.CreatePGBlueprintRequest) (plasmaapi.Blueprint, error)
	ListBlueprints(context.Context, string, plasmaapi.ViewQuery) (plasmaapi.BlueprintList, error)
	GetBlueprint(context.Context, string, string) (plasmaapi.Blueprint, error)
	SpawnBlueprintJob(context.Context, string, string) (string, error)
	ListBlueprintJobs(context.Context, string, string, plasmaapi.ViewQuery) (plasmaapi.JobList, error)
	GetJob(context.Context, string, string) (plasmaapi.Job, error)
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
資料 API 原則上一份表單／報表建立一個 mview；指定 PG 資料表則建立一個 view，
依 view → blueprint → PG 執行。整合所有區塊與指標，不因來源表不同而拆分。
只有使用者明確要求才拆分。查找、SELECT 驗證與建立 view 或 manual mview
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

PG 匯出先從 Ophion overview 列出已串接 DB，反問使用者選擇 PG 連線，不自行代選。
用 list_pg_connections／get_pg_connection 核對 DBC、database 與 schema；已明確
選定的連線沿用，不重複問。未選定時可先查核 SQL、建立 view，不建立 blueprint。
create_view(type=view) 省略同步與排程設定。使用 create_pg_blueprint 選取 view
及既有 PG DBC，明確指定 table、write_mode、force_create_table；建立不執行。
同步確認後才 spawn_blueprint_job，再以 list_blueprint_jobs 找本次新 job，用
get_job 追蹤 completed。舊 job 的成功不代表本次結果。不要對一般 view 呼叫
sync_view。PG 寫入模式只支援 append／overwrite／truncate，沒有 upsert；目前
工具不設定 blueprint 排程。來源查核仍由 Ophion 提供。

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

// NewServer builds the MCP server with all tools registered.
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
