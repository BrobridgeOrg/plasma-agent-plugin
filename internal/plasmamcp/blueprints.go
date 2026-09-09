package plasmamcp

import (
	"context"
	"fmt"
	"strings"

	"github.com/BrobridgeOrg/plasma-plugin/internal/plasmaapi"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type blueprintListInput struct {
	Keywords string `json:"keywords,omitempty"`
	Page     int    `json:"page,omitempty"`
	PageSize int    `json:"page_size,omitempty"`
}

func (in blueprintListInput) query() plasmaapi.ViewQuery {
	return plasmaapi.ViewQuery{Keywords: in.Keywords, Page: in.Page, PageSize: in.PageSize}
}

type pgConnectionInput struct {
	DBCID string `json:"dbc_id"`
}
type blueprintInput struct {
	BlueprintID string `json:"blueprint_id"`
}
type jobInput struct {
	JobID string `json:"job_id"`
}
type blueprintJobsInput struct {
	BlueprintID string `json:"blueprint_id"`
	Page        int    `json:"page,omitempty"`
	PageSize    int    `json:"page_size,omitempty"`
}

type createPGBlueprintInput struct {
	Name             string `json:"name"`
	Description      string `json:"description,omitempty"`
	ViewID           string `json:"view_id" jsonschema:"已驗證的一般 view ID"`
	DBCID            string `json:"dbc_id" jsonschema:"使用者從 Ophion 已串接 DB 選定後，核對的 Plasma PG 連線 ID"`
	Database         string `json:"database" jsonschema:"使用者選定的 PG 資料庫名稱，必須與 DBC 相符"`
	Schema           string `json:"schema" jsonschema:"使用者選定的 PG schema，必須與 DBC 相符，不能覆寫 DBC"`
	Table            string `json:"table" jsonschema:"PG 目標資料表的單一識別名稱，不含 schema 前綴"`
	WriteMode        string `json:"write_mode" jsonschema:"必填：append 新增、overwrite 替換資料表、truncate 清空後寫入；沒有 upsert"`
	ForceCreateTable bool   `json:"force_create_table" jsonschema:"目標不存在時是否建立，必須明確指定"`
}

type pgConnectionOutput struct {
	Workspace  string                 `json:"workspace_id"`
	Connection plasmaapi.PGConnection `json:"connection"`
}
type pgConnectionsOutput struct {
	Workspace string `json:"workspace_id"`
	plasmaapi.PGConnectionList
}
type blueprintOutput struct {
	Workspace string              `json:"workspace_id"`
	Blueprint plasmaapi.Blueprint `json:"blueprint"`
	NextStep  string              `json:"next_step"`
}
type blueprintsOutput struct {
	Workspace string `json:"workspace_id"`
	plasmaapi.BlueprintList
}
type jobsOutput struct {
	Workspace string `json:"workspace_id"`
	plasmaapi.JobList
}
type jobOutput struct {
	Workspace string        `json:"workspace_id"`
	Job       plasmaapi.Job `json:"job"`
}
type spawnOutput struct {
	Workspace     string `json:"workspace_id"`
	BlueprintID   string `json:"blueprint_id"`
	PreviousJobID string `json:"previous_job_id,omitempty"`
	Message       string `json:"message"`
	NextStep      string `json:"next_step"`
}

func (s *server) registerBlueprints(srv *mcp.Server) {
	mcp.AddTool(srv, readOnly(&mcp.Tool{Name: "list_pg_connections", Description: "列出目前 workspace 的 PG 連線供核對。先從 Ophion 列出已串接 DB 並反問使用者選擇，不自行代選；結果分頁，不含密碼。"}), s.listPGConnections)
	mcp.AddTool(srv, readOnly(&mcp.Tool{Name: "get_pg_connection", Description: "核對使用者選定的 PG DBC、資料庫、schema 與狀態，不含密碼。"}), s.getPGConnection)
	mcp.AddTool(srv, costly(&mcp.Tool{Name: "create_pg_blueprint", Description: "建立 view → blueprint → PG 定義，選取 view 與使用者指定的既有 PG DBC；不啟動 job。明確指定寫入模式與是否建表，PG 資料庫／schema 必須符合 DBC。"}), s.createPGBlueprint)
	mcp.AddTool(srv, readOnly(&mcp.Tool{Name: "list_blueprints", Description: "分頁列出目前 workspace 的 user query blueprint，供比對既有設定與重用；不保證全部以 PG 為目的地。"}), s.listBlueprints)
	mcp.AddTool(srv, readOnly(&mcp.Tool{Name: "get_blueprint", Description: "讀取 blueprint 的來源 view、目的地、寫入模式與最近 job 狀態，不回傳連線密碼。"}), s.getBlueprint)
	mcp.AddTool(srv, costly(&mcp.Tool{Name: "spawn_blueprint_job", Description: "立即執行一般 view → blueprint → PG，同步前須以中文確認目的地、寫入模式與覆寫影響。回傳送出結果，不代表匯出完成；用 list_blueprint_jobs 找新 job，再用 get_job 追蹤。"}), s.spawnBlueprintJob)
	mcp.AddTool(srv, readOnly(&mcp.Tool{Name: "list_blueprint_jobs", Description: "依 blueprint ID 分頁列出 jobs，建立時間由新到舊。比對執行前的 job ID，不能將舊的 completed 當成這次成功。"}), s.listBlueprintJobs)
	mcp.AddTool(srv, readOnly(&mcp.Tool{Name: "get_job", Description: "讀取指定 job 的狀態與執行訊息。completed 代表後端執行完成；failed、invalid、cancelled、aborted 都不代表成功。"}), s.getJob)
}

func (s *server) listPGConnections(ctx context.Context, _ *mcp.CallToolRequest, in blueprintListInput) (*mcp.CallToolResult, pgConnectionsOutput, error) {
	state, err := s.requireWorkspace()
	if err != nil {
		return nil, pgConnectionsOutput{}, err
	}
	connections, err := s.deps.Plasma.ListPGConnections(ctx, state.WorkspaceID, in.query())
	return nil, pgConnectionsOutput{Workspace: state.WorkspaceID, PGConnectionList: connections}, err
}

func (s *server) checkedPGConnection(ctx context.Context, workspace, id string) (plasmaapi.PGConnection, error) {
	if strings.TrimSpace(id) == "" {
		return plasmaapi.PGConnection{}, fmt.Errorf("dbc_id 必填")
	}
	connection, err := s.deps.Plasma.GetPGConnection(ctx, workspace, id)
	if err != nil {
		return connection, err
	}
	if connection.ID != id || connection.WorkspaceID != workspace || connection.DBType != "postgresql" {
		return connection, fmt.Errorf("所選 DBC 必須是目前 workspace 的 PG 連線")
	}
	return connection, nil
}

func (s *server) getPGConnection(ctx context.Context, _ *mcp.CallToolRequest, in pgConnectionInput) (*mcp.CallToolResult, pgConnectionOutput, error) {
	state, err := s.requireWorkspace()
	if err != nil {
		return nil, pgConnectionOutput{}, err
	}
	connection, err := s.checkedPGConnection(ctx, state.WorkspaceID, in.DBCID)
	return nil, pgConnectionOutput{Workspace: state.WorkspaceID, Connection: connection}, err
}

func (s *server) checkedSourceView(ctx context.Context, workspace, id string) error {
	if strings.TrimSpace(id) == "" {
		return fmt.Errorf("view_id 必填")
	}
	view, err := s.deps.Plasma.GetView(ctx, workspace, id)
	if err != nil {
		return err
	}
	if view.ID != id || view.Type != "view" {
		return fmt.Errorf("PG 匯出來源必須是目前 workspace 的一般 view")
	}
	if strings.TrimSpace(view.Path) == "" {
		return fmt.Errorf("view 尚無可查詢 path，請先確認 view 建立狀態")
	}
	return guardReadOnlySQL(view.ViewSQL)
}

func (s *server) createPGBlueprint(ctx context.Context, _ *mcp.CallToolRequest, in createPGBlueprintInput) (*mcp.CallToolResult, blueprintOutput, error) {
	state, err := s.requireWorkspace()
	if err != nil {
		return nil, blueprintOutput{}, err
	}
	for _, field := range []struct{ name, value string }{
		{"name", in.Name}, {"view_id", in.ViewID}, {"dbc_id", in.DBCID},
		{"database", in.Database}, {"schema", in.Schema}, {"table", in.Table},
	} {
		if strings.TrimSpace(field.value) == "" {
			return nil, blueprintOutput{}, fmt.Errorf("%s 必填", field.name)
		}
	}
	if !oneOf(in.WriteMode, []string{"append", "overwrite", "truncate"}) {
		return nil, blueprintOutput{}, fmt.Errorf("write_mode 必須為 append、overwrite 或 truncate，不提供預設值或 upsert")
	}
	if err := s.checkedSourceView(ctx, state.WorkspaceID, in.ViewID); err != nil {
		return nil, blueprintOutput{}, err
	}
	connection, err := s.checkedPGConnection(ctx, state.WorkspaceID, in.DBCID)
	if err != nil {
		return nil, blueprintOutput{}, err
	}
	if connection.DBName != in.Database || connection.Schema != in.Schema {
		return nil, blueprintOutput{}, fmt.Errorf("所選 PG database／schema 與 DBC 不符；後端會沿用 DBC，請重新請使用者選擇正確連線")
	}
	if connection.Status != "ready" {
		return nil, blueprintOutput{}, fmt.Errorf("PG DBC 尚未 ready：%s", connection.Status)
	}
	blueprint, err := s.deps.Plasma.CreatePGBlueprint(ctx, state.WorkspaceID, plasmaapi.CreatePGBlueprintRequest{
		Name: in.Name, Description: in.Description,
		Configuration: plasmaapi.BlueprintConfiguration{
			SourceViewID: in.ViewID,
			Target:       plasmaapi.BlueprintTarget{Method: "static", DBCID: in.DBCID, Table: in.Table},
			Properties:   plasmaapi.BlueprintProperties{WriteMode: in.WriteMode, ForceCreateTable: in.ForceCreateTable},
		},
	})
	return nil, blueprintOutput{Workspace: state.WorkspaceID, Blueprint: blueprint,
		NextStep: "blueprint 已建立，尚未拉取資料。核對設定並取得同步確認後，呼叫 spawn_blueprint_job。"}, err
}

func (s *server) listBlueprints(ctx context.Context, _ *mcp.CallToolRequest, in blueprintListInput) (*mcp.CallToolResult, blueprintsOutput, error) {
	state, err := s.requireWorkspace()
	if err != nil {
		return nil, blueprintsOutput{}, err
	}
	blueprints, err := s.deps.Plasma.ListBlueprints(ctx, state.WorkspaceID, in.query())
	return nil, blueprintsOutput{Workspace: state.WorkspaceID, BlueprintList: blueprints}, err
}

func (s *server) getBlueprint(ctx context.Context, _ *mcp.CallToolRequest, in blueprintInput) (*mcp.CallToolResult, blueprintOutput, error) {
	state, err := s.requireWorkspace()
	if err != nil {
		return nil, blueprintOutput{}, err
	}
	if strings.TrimSpace(in.BlueprintID) == "" {
		return nil, blueprintOutput{}, fmt.Errorf("blueprint_id 必填")
	}
	blueprint, err := s.deps.Plasma.GetBlueprint(ctx, state.WorkspaceID, in.BlueprintID)
	return nil, blueprintOutput{Workspace: state.WorkspaceID, Blueprint: blueprint}, err
}

func (s *server) spawnBlueprintJob(ctx context.Context, _ *mcp.CallToolRequest, in blueprintInput) (*mcp.CallToolResult, spawnOutput, error) {
	state, err := s.requireWorkspace()
	if err != nil {
		return nil, spawnOutput{}, err
	}
	if strings.TrimSpace(in.BlueprintID) == "" {
		return nil, spawnOutput{}, fmt.Errorf("blueprint_id 必填")
	}
	blueprint, err := s.deps.Plasma.GetBlueprint(ctx, state.WorkspaceID, in.BlueprintID)
	if err != nil {
		return nil, spawnOutput{}, err
	}
	config := blueprint.Configuration
	if blueprint.ID != in.BlueprintID || blueprint.IsDisabled || blueprint.JobType != "query" ||
		config.Target.Method != "static" || config.Target.Type != "postgresql" || config.Target.ViewID != "" ||
		strings.TrimSpace(config.Target.Table) == "" || !oneOf(config.Properties.WriteMode, []string{"append", "overwrite", "truncate"}) {
		return nil, spawnOutput{}, fmt.Errorf("blueprint 必須是啟用中的一般 view → 既有 PG DBC 匯出設定，並具備有效寫入模式")
	}
	if err := s.checkedSourceView(ctx, state.WorkspaceID, config.SourceViewID); err != nil {
		return nil, spawnOutput{}, err
	}
	connection, err := s.checkedPGConnection(ctx, state.WorkspaceID, config.Target.DBCID)
	if err != nil {
		return nil, spawnOutput{}, err
	}
	if connection.Status != "ready" || config.Target.Catalog != connection.SourceCatalogName ||
		config.Target.Namespace != connection.Schema || config.Target.Host != connection.Host ||
		config.Target.Port != connection.Port || config.Target.Username != connection.Username {
		return nil, spawnOutput{}, fmt.Errorf("PG DBC 尚未 ready，或 blueprint 儲存的目的地已與 DBC 不符；請重新核對使用者選擇與 blueprint 設定後再執行")
	}
	message, err := s.deps.Plasma.SpawnBlueprintJob(ctx, state.WorkspaceID, in.BlueprintID)
	if err != nil {
		return nil, spawnOutput{}, err
	}
	return nil, spawnOutput{Workspace: state.WorkspaceID, BlueprintID: in.BlueprintID,
		PreviousJobID: blueprint.LastJobID, Message: message,
		NextStep: "已送出執行請求，尚未確認完成。用 list_blueprint_jobs 找出本次新 job ID，再用 get_job 追蹤至 completed；不要將 previous_job_id 的結果當成本次結果，也不要因 job 尚未出現而重送。"}, nil
}

func (s *server) listBlueprintJobs(ctx context.Context, _ *mcp.CallToolRequest, in blueprintJobsInput) (*mcp.CallToolResult, jobsOutput, error) {
	state, err := s.requireWorkspace()
	if err != nil {
		return nil, jobsOutput{}, err
	}
	if strings.TrimSpace(in.BlueprintID) == "" {
		return nil, jobsOutput{}, fmt.Errorf("blueprint_id 必填")
	}
	jobs, err := s.deps.Plasma.ListBlueprintJobs(ctx, state.WorkspaceID, in.BlueprintID, plasmaapi.ViewQuery{Page: in.Page, PageSize: in.PageSize})
	return nil, jobsOutput{Workspace: state.WorkspaceID, JobList: jobs}, err
}

func (s *server) getJob(ctx context.Context, _ *mcp.CallToolRequest, in jobInput) (*mcp.CallToolResult, jobOutput, error) {
	state, err := s.requireWorkspace()
	if err != nil {
		return nil, jobOutput{}, err
	}
	if strings.TrimSpace(in.JobID) == "" {
		return nil, jobOutput{}, fmt.Errorf("job_id 必填")
	}
	job, err := s.deps.Plasma.GetJob(ctx, state.WorkspaceID, in.JobID)
	return nil, jobOutput{Workspace: state.WorkspaceID, Job: job}, err
}
