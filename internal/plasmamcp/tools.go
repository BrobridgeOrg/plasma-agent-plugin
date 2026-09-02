package plasmamcp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/BrobridgeOrg/plasma-plugin/internal/pcontext"
	"github.com/BrobridgeOrg/plasma-plugin/internal/plasmaapi"
)

// plasmaRowCeiling is what Plasma enforces on /query/execution. It is
// mirrored here only to tell the reader when a result is sitting on the
// ceiling rather than being complete.
const plasmaRowCeiling = 100

var (
	validViewTypes = []string{"view", "materialized_view"}
	validSyncModes = []string{"manual", "scheduled"}
	validAuthTypes = []string{"none", "api_key", "basic_auth"}
)

func readOnly(t *mcp.Tool) *mcp.Tool {
	t.Annotations = &mcp.ToolAnnotations{ReadOnlyHint: true}
	return t
}

// costly marks a tool that spends cluster time or exposes data. ReadOnlyHint
// stays false and DestructiveHint is set so a client that reasons about
// annotations treats it as a real action.
func costly(t *mcp.Tool) *mcp.Tool {
	destructive := true
	t.Annotations = &mcp.ToolAnnotations{DestructiveHint: &destructive}
	return t
}

func textResult(parts ...string) *mcp.CallToolResult {
	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: strings.Join(parts, "\n")}},
	}
}

// ---------- whoami ----------

type emptyInput struct{}

type contextOutput struct {
	PlasmaURL     string `json:"plasma_url"`
	Username      string `json:"username,omitempty"`
	Role          string `json:"role,omitempty"`
	AuthError     string `json:"auth_error,omitempty"`
	WorkspaceID   string `json:"workspace_id,omitempty"`
	WorkspaceName string `json:"workspace_name,omitempty"`
	Profile       string `json:"profile"`
	OphionURL     string `json:"ophion_url"`
}

// ---------- workspaces ----------

type workspacesOutput struct {
	Workspaces []plasmaapi.Workspace `json:"workspaces"`
	Current    string                `json:"current_workspace_id"`
}

type useWorkspaceInput struct {
	Workspace string `json:"workspace" jsonschema:"the workspace id, or its name when unambiguous"`
}

type useWorkspaceOutput struct {
	WorkspaceID   string `json:"workspace_id"`
	WorkspaceName string `json:"workspace_name"`
}

// ---------- views ----------

type listViewsInput struct {
	Keywords string `json:"keywords,omitempty" jsonschema:"filter by name or description"`
	Page     int    `json:"page,omitempty"`
	PageSize int    `json:"page_size,omitempty"`
}

type viewsOutput struct {
	Workspace string           `json:"workspace_id"`
	Views     []plasmaapi.View `json:"views"`
	Total     int64            `json:"total"`
	Page      int              `json:"page,omitempty"`
	PageSize  int              `json:"page_size,omitempty"`
}

type getViewInput struct {
	ViewID               string `json:"view_id"`
	WithBlueprintHistory bool   `json:"with_blueprint_history,omitempty" jsonschema:"also list the blueprints this view has had"`
}

type viewOutput struct {
	Workspace        string                       `json:"workspace_id"`
	View             plasmaapi.View               `json:"view"`
	BlueprintHistory []plasmaapi.BlueprintHistory `json:"blueprint_history,omitempty"`
}

// ---------- query ----------

type runQueryInput struct {
	SQL string `json:"sql" jsonschema:"a single SELECT or WITH statement; Plasma rejects anything else and caps the result at 100 rows"`
}

type queryOutput struct {
	Workspace string   `json:"workspace_id"`
	QueryID   string   `json:"query_id,omitempty"`
	Columns   []string `json:"columns"`
	Rows      [][]any  `json:"rows"`
	RowCount  int      `json:"row_count"`
	ElapsedMs int64    `json:"elapsed_ms,omitempty"`
	Notice    string   `json:"notice,omitempty"`
}

// ---------- create / sync ----------

type createViewInput struct {
	Name              string         `json:"name"`
	Type              string         `json:"type" jsonschema:"view or materialized_view"`
	ViewSQL           string         `json:"view_sql" jsonschema:"the SELECT (or WITH) that defines the view"`
	SyncMode          string         `json:"sync_mode,omitempty" jsonschema:"manual (default) or scheduled"`
	SchedulerSettings map[string]any `json:"scheduler_settings,omitempty" jsonschema:"Plasma scheduler settings; required when sync_mode is scheduled"`
	Description       string         `json:"description,omitempty"`
}

type createViewOutput struct {
	Workspace string         `json:"workspace_id"`
	View      plasmaapi.View `json:"view"`
	NextStep  string         `json:"next_step"`
}

type syncViewInput struct {
	ViewID string `json:"view_id"`
}

type syncViewOutput struct {
	Workspace string `json:"workspace_id"`
	ViewID    string `json:"view_id"`
	Message   string `json:"message"`
}

// ---------- data API ----------

type createAccessEntryInput struct {
	ViewID      string `json:"view_id"`
	Name        string `json:"name"`
	AuthType    string `json:"auth_type" jsonschema:"none, api_key or basic_auth"`
	ExpiresIn   string `json:"expires_in,omitempty" jsonschema:"how long the endpoint stays valid, e.g. 30d or 12h; omitted leaves the decision to Plasma"`
	SecretKey   string `json:"secret_key,omitempty" jsonschema:"required for basic_auth as username:password; api_key generates one when empty"`
	Description string `json:"description,omitempty"`
}

type accessEntryOutput struct {
	Workspace string                `json:"workspace_id"`
	Entry     plasmaapi.AccessEntry `json:"access_entry"`
	AccessURL string                `json:"access_url,omitempty"`
	Warnings  []string              `json:"warnings,omitempty"`
}

type accessEntriesOutput struct {
	Workspace string                  `json:"workspace_id"`
	Entries   []plasmaapi.AccessEntry `json:"entries"`
}

type exportURLInput struct {
	ViewID string `json:"view_id"`
}

type exportURLOutput struct {
	Workspace string `json:"workspace_id"`
	ViewID    string `json:"view_id"`
	ExportURL string `json:"export_url"`
}

func (s *server) register(srv *mcp.Server) {
	mcp.AddTool(srv, readOnly(&mcp.Tool{
		Name: "whoami",
		Description: "Report the Plasma deployment, the authenticated user, the selected " +
			"workspace and the Ophion endpoint. Start here when anything looks wrong.",
	}), s.whoami)

	mcp.AddTool(srv, readOnly(&mcp.Tool{
		Name:        "list_workspaces",
		Description: "List the Plasma workspaces this account is a member of.",
	}), s.listWorkspaces)

	mcp.AddTool(srv, &mcp.Tool{
		Name: "use_workspace",
		Description: "Select the workspace every other tool acts on, including the " +
			"ophion server's knowledge lookups. Accepts an id, or a name when unambiguous.",
	}, s.useWorkspace)

	mcp.AddTool(srv, readOnly(&mcp.Tool{
		Name:        "list_views",
		Description: "List the views and materialized views in the selected workspace.",
	}), s.listViews)

	mcp.AddTool(srv, readOnly(&mcp.Tool{
		Name: "get_view",
		Description: "Read one view: its type, status, path, SQL and last sync outcome. " +
			"This is how you check whether a materialized view has synced.",
	}), s.getView)

	mcp.AddTool(srv, costly(&mcp.Tool{
		Name: "run_query",
		Description: "Execute one SELECT against the workspace and return up to 100 rows. " +
			"This runs on the Trino cluster, so it costs real query time.",
	}), s.runQuery)

	mcp.AddTool(srv, costly(&mcp.Tool{
		Name: "create_view",
		Description: "Create a view or materialized view. With sync_mode=scheduled and " +
			"scheduler_settings, Plasma creates the view, its blueprint and its schedule " +
			"in one call and runs the first sync immediately.",
	}), s.createView)

	mcp.AddTool(srv, costly(&mcp.Tool{
		Name: "sync_view",
		Description: "Trigger one sync of a materialized view. This runs the view's SQL " +
			"on the Trino cluster.",
	}), s.syncView)

	mcp.AddTool(srv, costly(&mcp.Tool{
		Name: "create_access_entry",
		Description: "Publish a view as an externally reachable data API and return its " +
			"URL and secret. This exposes the view's data outside Plasma.",
	}), s.createAccessEntry)

	mcp.AddTool(srv, readOnly(&mcp.Tool{
		Name:        "list_access_entries",
		Description: "List the external access grants on a view.",
	}), s.listAccessEntries)

	mcp.AddTool(srv, readOnly(&mcp.Tool{
		Name: "get_export_url",
		Description: "Get a view's data API URL. A view with no successful sync yet has " +
			"no URL, and says so.",
	}), s.getExportURL)
}

func (s *server) whoami(ctx context.Context, _ *mcp.CallToolRequest, _ emptyInput) (
	*mcp.CallToolResult, contextOutput, error) {

	st, err := s.state()
	if err != nil {
		return nil, contextOutput{}, err
	}
	out := contextOutput{
		PlasmaURL:     s.deps.Plasma.BaseURL(),
		WorkspaceID:   st.WorkspaceID,
		WorkspaceName: st.WorkspaceName,
		Profile:       profileOf(st, s.deps.Config),
		OphionURL:     s.deps.Config.OphionURL,
	}
	// A diagnostic tool that fails when authentication is broken hides the
	// very thing it exists to report, so a bad bearer is data here.
	user, err := s.deps.Plasma.VerifyToken(ctx)
	if err != nil {
		out.AuthError = err.Error()
	} else {
		out.Username, out.Role = user.Username, user.Role
	}

	lines := []string{
		"Plasma:    " + orNotSet(out.PlasmaURL),
		"Ophion:    " + orNotSet(out.OphionURL),
		"Profile:   " + out.Profile,
	}
	if out.AuthError != "" {
		lines = append(lines, "User:      NOT AUTHENTICATED: "+out.AuthError)
	} else {
		lines = append(lines, fmt.Sprintf("User:      %s (%s)", out.Username, out.Role))
	}
	if out.WorkspaceID == "" {
		lines = append(lines, "Workspace: none selected — call use_workspace")
	} else {
		lines = append(lines, fmt.Sprintf("Workspace: %s (%s)", out.WorkspaceName, out.WorkspaceID))
	}
	return textResult(lines...), out, nil
}

func (s *server) listWorkspaces(ctx context.Context, _ *mcp.CallToolRequest, _ emptyInput) (
	*mcp.CallToolResult, workspacesOutput, error) {

	st, err := s.state()
	if err != nil {
		return nil, workspacesOutput{}, err
	}
	workspaces, err := s.deps.Plasma.MyWorkspaces(ctx)
	if err != nil {
		return nil, workspacesOutput{}, err
	}
	lines := make([]string, 0, len(workspaces)+1)
	if len(workspaces) == 0 {
		lines = append(lines, "This account is a member of no workspaces.")
	}
	for _, ws := range workspaces {
		marker := "  "
		if ws.ID == st.WorkspaceID {
			marker = "* "
		}
		lines = append(lines, fmt.Sprintf("%s%s  %s  role=%s", marker, ws.ID, ws.Name, ws.Role))
	}
	return textResult(lines...), workspacesOutput{Workspaces: workspaces, Current: st.WorkspaceID}, nil
}

func (s *server) useWorkspace(ctx context.Context, _ *mcp.CallToolRequest, in useWorkspaceInput) (
	*mcp.CallToolResult, useWorkspaceOutput, error) {

	wanted := strings.TrimSpace(in.Workspace)
	if wanted == "" {
		return nil, useWorkspaceOutput{}, fmt.Errorf("workspace is required")
	}
	workspaces, err := s.deps.Plasma.MyWorkspaces(ctx)
	if err != nil {
		return nil, useWorkspaceOutput{}, err
	}

	var matches []plasmaapi.Workspace
	for _, ws := range workspaces {
		if ws.ID == wanted {
			matches = []plasmaapi.Workspace{ws}
			break
		}
		if ws.Name == wanted {
			matches = append(matches, ws)
		}
	}
	switch {
	case len(matches) == 0:
		return nil, useWorkspaceOutput{}, fmt.Errorf(
			"no workspace %q for this account; available: %s", wanted, describe(workspaces))
	case len(matches) > 1:
		return nil, useWorkspaceOutput{}, fmt.Errorf(
			"%q names %d workspaces (%s): select it by id instead",
			wanted, len(matches), describe(matches))
	}

	// Only the selection changes. The stored JWT belongs to the session, not
	// to the workspace, and dropping it would force a needless re-login.
	st, err := s.state()
	if err != nil {
		return nil, useWorkspaceOutput{}, err
	}
	st.WorkspaceID = matches[0].ID
	st.WorkspaceName = matches[0].Name
	st.Profile = profileOf(st, s.deps.Config)
	if err := s.deps.Home.SaveState(st); err != nil {
		return nil, useWorkspaceOutput{}, err
	}
	return textResult(fmt.Sprintf("Selected %s (%s). The ophion server's next call uses it too.",
			st.WorkspaceName, st.WorkspaceID)),
		useWorkspaceOutput{WorkspaceID: st.WorkspaceID, WorkspaceName: st.WorkspaceName}, nil
}

func (s *server) listViews(ctx context.Context, _ *mcp.CallToolRequest, in listViewsInput) (
	*mcp.CallToolResult, viewsOutput, error) {

	st, err := s.requireWorkspace()
	if err != nil {
		return nil, viewsOutput{}, err
	}
	list, err := s.deps.Plasma.ListViews(ctx, st.WorkspaceID, plasmaapi.ViewQuery{
		Keywords: in.Keywords, Page: in.Page, PageSize: in.PageSize,
	})
	if err != nil {
		return nil, viewsOutput{}, err
	}
	lines := []string{annotate(st), fmt.Sprintf("%d view(s) total", list.Total)}
	for _, v := range list.Views {
		lines = append(lines, fmt.Sprintf("  %s  %s  type=%s status=%s last_sync=%s",
			v.ID, v.Name, v.Type, v.Status, orDash(v.LastSyncStatus)))
	}
	return textResult(lines...), viewsOutput{
		Workspace: st.WorkspaceID, Views: list.Views, Total: list.Total,
		Page: list.Page, PageSize: list.PageSize,
	}, nil
}

func (s *server) getView(ctx context.Context, _ *mcp.CallToolRequest, in getViewInput) (
	*mcp.CallToolResult, viewOutput, error) {

	st, err := s.requireWorkspace()
	if err != nil {
		return nil, viewOutput{}, err
	}
	if strings.TrimSpace(in.ViewID) == "" {
		return nil, viewOutput{}, fmt.Errorf("view_id is required")
	}
	view, err := s.deps.Plasma.GetView(ctx, st.WorkspaceID, in.ViewID)
	if err != nil {
		return nil, viewOutput{}, err
	}
	out := viewOutput{Workspace: st.WorkspaceID, View: view}
	lines := []string{
		annotate(st),
		fmt.Sprintf("%s (%s)", view.Name, view.ID),
		"type:        " + view.Type,
		"status:      " + view.Status,
		"path:        " + orDash(view.Path),
		"sync_mode:   " + orDash(view.SyncMode),
		"last_sync:   " + orDash(view.LastSyncStatus) + " " + view.LastSyncAt,
		"version:     " + fmt.Sprint(view.Version),
		"sql:",
		view.ViewSQL,
	}
	if in.WithBlueprintHistory {
		history, err := s.deps.Plasma.BlueprintHistory(ctx, st.WorkspaceID, in.ViewID)
		if err != nil {
			return nil, viewOutput{}, err
		}
		out.BlueprintHistory = history
		lines = append(lines, "blueprint history:")
		for _, h := range history {
			lines = append(lines, fmt.Sprintf("  %s  %s", h.BlueprintID, h.CreatedAt))
		}
	}
	return textResult(lines...), out, nil
}

func (s *server) runQuery(ctx context.Context, _ *mcp.CallToolRequest, in runQueryInput) (
	*mcp.CallToolResult, queryOutput, error) {

	st, err := s.requireWorkspace()
	if err != nil {
		return nil, queryOutput{}, err
	}
	if err := guardReadOnlySQL(in.SQL); err != nil {
		return nil, queryOutput{}, err
	}
	result, err := s.deps.Plasma.RunQuery(ctx, st.WorkspaceID, strings.TrimSpace(in.SQL))
	if err != nil {
		return nil, queryOutput{}, err
	}
	out := queryOutput{
		Workspace: st.WorkspaceID, QueryID: result.QueryID,
		Columns: result.Columns, Rows: result.Rows,
		RowCount: len(result.Rows), ElapsedMs: result.ElapsedMs,
	}
	if out.RowCount >= plasmaRowCeiling {
		out.Notice = fmt.Sprintf(
			"this result sits on Plasma's %d-row ceiling, so it is a sample, not the whole answer",
			plasmaRowCeiling)
	}
	lines := []string{
		annotate(st),
		renderTable(result.Columns, result.Rows),
		fmt.Sprintf("%d row(s) in %dms", out.RowCount, result.ElapsedMs),
	}
	if out.Notice != "" {
		lines = append(lines, "NOTE: "+out.Notice)
	}
	return textResult(lines...), out, nil
}

func (s *server) createView(ctx context.Context, _ *mcp.CallToolRequest, in createViewInput) (
	*mcp.CallToolResult, createViewOutput, error) {

	st, err := s.requireWorkspace()
	if err != nil {
		return nil, createViewOutput{}, err
	}
	if strings.TrimSpace(in.Name) == "" {
		return nil, createViewOutput{}, fmt.Errorf("name is required")
	}
	if !oneOf(in.Type, validViewTypes) {
		return nil, createViewOutput{}, fmt.Errorf("type must be one of %s, got %q",
			strings.Join(validViewTypes, ", "), in.Type)
	}
	if err := guardReadOnlySQL(in.ViewSQL); err != nil {
		return nil, createViewOutput{}, fmt.Errorf("view_sql: %w", err)
	}
	syncMode := in.SyncMode
	if syncMode == "" {
		syncMode = "manual"
	}
	if !oneOf(syncMode, validSyncModes) {
		return nil, createViewOutput{}, fmt.Errorf("sync_mode must be one of %s, got %q",
			strings.Join(validSyncModes, ", "), in.SyncMode)
	}
	if syncMode == "scheduled" && len(in.SchedulerSettings) == 0 {
		return nil, createViewOutput{}, fmt.Errorf(
			"sync_mode=scheduled needs scheduler_settings, otherwise the view would never sync")
	}

	req := plasmaapi.CreateViewRequest{
		Name: in.Name, Description: in.Description, ViewSQL: strings.TrimSpace(in.ViewSQL),
		Type: in.Type, SyncMode: syncMode,
	}
	if len(in.SchedulerSettings) > 0 {
		encoded, err := json.Marshal(in.SchedulerSettings)
		if err != nil {
			return nil, createViewOutput{}, err
		}
		req.SchedulerSettings = encoded
	}
	view, err := s.deps.Plasma.CreateView(ctx, st.WorkspaceID, req)
	if err != nil {
		return nil, createViewOutput{}, err
	}
	next := "call get_view to watch last_sync_status reach synced"
	if in.Type == "materialized_view" && syncMode == "manual" {
		next = "call sync_view to run the first sync, then get_view to check it"
	}
	out := createViewOutput{Workspace: st.WorkspaceID, View: view, NextStep: next}
	return textResult(
		annotate(st),
		fmt.Sprintf("Created %s (%s) type=%s status=%s", view.Name, view.ID, view.Type, view.Status),
		"path: "+orDash(view.Path),
		"next: "+next,
	), out, nil
}

func (s *server) syncView(ctx context.Context, _ *mcp.CallToolRequest, in syncViewInput) (
	*mcp.CallToolResult, syncViewOutput, error) {

	st, err := s.requireWorkspace()
	if err != nil {
		return nil, syncViewOutput{}, err
	}
	if strings.TrimSpace(in.ViewID) == "" {
		return nil, syncViewOutput{}, fmt.Errorf("view_id is required")
	}
	msg, err := s.deps.Plasma.SyncView(ctx, st.WorkspaceID, in.ViewID)
	if err != nil {
		return nil, syncViewOutput{}, err
	}
	return textResult(annotate(st), msg,
			"The sync runs asynchronously — call get_view to see last_sync_status."),
		syncViewOutput{Workspace: st.WorkspaceID, ViewID: in.ViewID, Message: msg}, nil
}

func (s *server) createAccessEntry(ctx context.Context, _ *mcp.CallToolRequest,
	in createAccessEntryInput) (*mcp.CallToolResult, accessEntryOutput, error) {

	st, err := s.requireWorkspace()
	if err != nil {
		return nil, accessEntryOutput{}, err
	}
	if strings.TrimSpace(in.ViewID) == "" || strings.TrimSpace(in.Name) == "" {
		return nil, accessEntryOutput{}, fmt.Errorf("view_id and name are required")
	}
	if !oneOf(in.AuthType, validAuthTypes) {
		return nil, accessEntryOutput{}, fmt.Errorf("auth_type must be one of %s, got %q",
			strings.Join(validAuthTypes, ", "), in.AuthType)
	}
	if in.AuthType == "basic_auth" {
		if in.SecretKey == "" {
			return nil, accessEntryOutput{}, fmt.Errorf(
				"basic_auth needs secret_key as username:password; " +
					"an auto-generated secret cannot carry a username")
		}
		if !strings.Contains(in.SecretKey, ":") {
			return nil, accessEntryOutput{}, fmt.Errorf(
				"basic_auth secret_key must be username:password")
		}
	}

	req := plasmaapi.CreateAccessEntryRequest{
		Name: in.Name, Description: in.Description,
		AuthType: in.AuthType, SecretKey: in.SecretKey,
	}
	if in.ExpiresIn != "" {
		d, err := parseExpiry(in.ExpiresIn)
		if err != nil {
			return nil, accessEntryOutput{}, err
		}
		if d <= 0 {
			return nil, accessEntryOutput{}, fmt.Errorf("expires_in must be positive, got %q", in.ExpiresIn)
		}
		expiry := time.Now().Add(d).UTC()
		req.ExpiredAt = &expiry
	}

	created, err := s.deps.Plasma.CreateAccessEntry(ctx, st.WorkspaceID, in.ViewID, req)
	if err != nil {
		return nil, accessEntryOutput{}, err
	}

	out := accessEntryOutput{
		Workspace: st.WorkspaceID, Entry: created.Entry, AccessURL: created.AccessURL,
	}
	if in.AuthType == "none" {
		out.Warnings = append(out.Warnings,
			"this endpoint has no authentication: anyone with the URL can read the view's data")
	}
	expiry := "expires: never — this endpoint stays valid until it is deleted"
	if created.Entry.ExpiredAt != nil {
		expiry = "expires: " + *created.Entry.ExpiredAt
	} else if req.ExpiredAt != nil {
		expiry = "expires: " + req.ExpiredAt.Format(time.RFC3339) + " (as requested)"
	}
	out.Warnings = append(out.Warnings, expiry)

	lines := []string{
		annotate(st),
		fmt.Sprintf("Published %s (%s) auth=%s", created.Entry.Name, created.Entry.ID, created.Entry.AuthType),
		"url:    " + orDash(created.AccessURL),
		"secret: " + orDash(created.Entry.SecretKey),
		expiry,
	}
	if in.AuthType == "none" {
		lines = append(lines,
			"WARNING: no authentication — anyone with the URL can read this view's data")
	}
	return textResult(lines...), out, nil
}

func (s *server) listAccessEntries(ctx context.Context, _ *mcp.CallToolRequest, in exportURLInput) (
	*mcp.CallToolResult, accessEntriesOutput, error) {

	st, err := s.requireWorkspace()
	if err != nil {
		return nil, accessEntriesOutput{}, err
	}
	if strings.TrimSpace(in.ViewID) == "" {
		return nil, accessEntriesOutput{}, fmt.Errorf("view_id is required")
	}
	entries, err := s.deps.Plasma.ListAccessEntries(ctx, st.WorkspaceID, in.ViewID)
	if err != nil {
		return nil, accessEntriesOutput{}, err
	}
	lines := []string{annotate(st)}
	if len(entries) == 0 {
		lines = append(lines, "no access entries — this view is not published")
	}
	for _, e := range entries {
		expiry := "never"
		if e.ExpiredAt != nil {
			expiry = *e.ExpiredAt
		}
		lines = append(lines, fmt.Sprintf("  %s  %s  auth=%s expires=%s", e.ID, e.Name, e.AuthType, expiry))
	}
	return textResult(lines...), accessEntriesOutput{Workspace: st.WorkspaceID, Entries: entries}, nil
}

func (s *server) getExportURL(ctx context.Context, _ *mcp.CallToolRequest, in exportURLInput) (
	*mcp.CallToolResult, exportURLOutput, error) {

	st, err := s.requireWorkspace()
	if err != nil {
		return nil, exportURLOutput{}, err
	}
	if strings.TrimSpace(in.ViewID) == "" {
		return nil, exportURLOutput{}, fmt.Errorf("view_id is required")
	}
	url, err := s.deps.Plasma.ExportURL(ctx, st.WorkspaceID, in.ViewID)
	if err != nil {
		if errors.Is(err, plasmaapi.ErrNotReady) {
			return nil, exportURLOutput{}, fmt.Errorf(
				"this view has no successful sync yet, so it has no data API URL: "+
					"run sync_view and wait for last_sync_status=synced (%w)", err)
		}
		return nil, exportURLOutput{}, err
	}
	return textResult(annotate(st), url),
		exportURLOutput{Workspace: st.WorkspaceID, ViewID: in.ViewID, ExportURL: url}, nil
}

func profileOf(st pcontext.State, cfg pcontext.Config) string {
	if st.Profile != "" {
		return st.Profile
	}
	if cfg.OphionProfile != "" {
		return cfg.OphionProfile
	}
	return pcontext.DefaultProfile
}

func describe(workspaces []plasmaapi.Workspace) string {
	parts := make([]string, 0, len(workspaces))
	for _, ws := range workspaces {
		parts = append(parts, fmt.Sprintf("%s (%s)", ws.Name, ws.ID))
	}
	if len(parts) == 0 {
		return "(none)"
	}
	return strings.Join(parts, ", ")
}

func oneOf(v string, allowed []string) bool {
	for _, a := range allowed {
		if v == a {
			return true
		}
	}
	return false
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
