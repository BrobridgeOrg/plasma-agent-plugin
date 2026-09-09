package plasmaapi

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

type PGConnection struct {
	ID                string `json:"id"`
	DisplayName       string `json:"display_name"`
	WorkspaceID       string `json:"workspace_id"`
	DBType            string `json:"db_type"`
	DBName            string `json:"db_name"`
	Schema            string `json:"schema"`
	Host              string `json:"host"`
	Port              int    `json:"port"`
	Username          string `json:"username"`
	Status            string `json:"status"`
	SourceCatalogName string `json:"source_catalog_name"`
}

type PGConnectionList struct {
	Connections []PGConnection `json:"connections"`
	Total       int64          `json:"total"`
	Page        int            `json:"page"`
	PageSize    int            `json:"page_size"`
	TotalPages  int            `json:"total_pages"`
}

type BlueprintTarget struct {
	Method    string `json:"method"`
	DBCID     string `json:"dbc_id,omitempty"`
	Catalog   string `json:"catalog,omitempty"`
	Type      string `json:"type,omitempty"`
	Namespace string `json:"namespace,omitempty"`
	Table     string `json:"table,omitempty"`
	Host      string `json:"host,omitempty"`
	Port      int    `json:"port,omitempty"`
	Username  string `json:"username,omitempty"`
	ViewID    string `json:"view_id,omitempty"`
}

type BlueprintProperties struct {
	WriteMode        string `json:"write_mode"`
	ForceCreateTable bool   `json:"force_create_table"`
}

type BlueprintConfiguration struct {
	SourceViewID string              `json:"source_view_id"`
	Target       BlueprintTarget     `json:"target"`
	Properties   BlueprintProperties `json:"properties"`
}

type CreatePGBlueprintRequest struct {
	Name          string
	Description   string
	Configuration BlueprintConfiguration
}

type Blueprint struct {
	ID                  string                 `json:"id"`
	Name                string                 `json:"name"`
	Description         string                 `json:"description"`
	JobType             string                 `json:"job_type"`
	IsDisabled          bool                   `json:"is_disabled"`
	AllowConcurrentJobs bool                   `json:"allow_concurrent_jobs"`
	Configuration       BlueprintConfiguration `json:"job_configurations"`
	LastJobID           string                 `json:"last_job_id"`
	LastJobStatus       string                 `json:"last_job_status"`
	CurrentJobStatus    string                 `json:"current_job_status"`
}

type BlueprintList struct {
	Blueprints []Blueprint `json:"blueprints"`
	Total      int64       `json:"total"`
	Page       int         `json:"page"`
	PageSize   int         `json:"page_size"`
	TotalPages int         `json:"total_pages"`
}

type Job struct {
	ID          string `json:"id"`
	BlueprintID string `json:"blueprint_id"`
	Name        string `json:"name"`
	Status      string `json:"status"`
	Details     string `json:"details"`
	CreatedAt   string `json:"created_at"`
	UpdatedAt   string `json:"updated_at"`
}

type JobList struct {
	Jobs       []Job `json:"jobs"`
	Total      int64 `json:"total"`
	Page       int   `json:"page"`
	PageSize   int   `json:"page_size"`
	TotalPages int   `json:"total_pages"`
}

func scopedPath(workspace, resource, id string) (string, error) {
	if err := requireWorkspace(workspace); err != nil {
		return "", err
	}
	if strings.TrimSpace(id) == "" {
		return "", fmt.Errorf("%s ID is required", resource)
	}
	return "/apis/v1/w/" + url.PathEscape(workspace) + "/" + resource + "/" + url.PathEscape(id), nil
}

func pageQuery(query ViewQuery) url.Values {
	values := url.Values{}
	if query.Keywords != "" {
		values.Set("keywords", query.Keywords)
	}
	if query.Page > 0 {
		values.Set("page", strconv.Itoa(query.Page))
	}
	if query.PageSize > 0 {
		values.Set("page_size", strconv.Itoa(query.PageSize))
	}
	return values
}

func (c *Client) ListPGConnections(ctx context.Context, workspace string, query ViewQuery) (PGConnectionList, error) {
	var out PGConnectionList
	if err := requireWorkspace(workspace); err != nil {
		return out, err
	}
	values := pageQuery(query)
	values.Set("db_type", "postgresql")
	err := c.do(ctx, http.MethodGet, "/apis/v1/w/"+url.PathEscape(workspace)+"/dbcs", values, nil, &out)
	return out, err
}

func (c *Client) GetPGConnection(ctx context.Context, workspace, id string) (PGConnection, error) {
	var out struct {
		Connection PGConnection `json:"connection"`
	}
	path, err := scopedPath(workspace, "dbc", id)
	if err != nil {
		return out.Connection, err
	}
	err = c.do(ctx, http.MethodGet, path, nil, nil, &out)
	return out.Connection, err
}

func (c *Client) CreatePGBlueprint(ctx context.Context, workspace string, request CreatePGBlueprintRequest) (Blueprint, error) {
	var out struct {
		Blueprint Blueprint `json:"blueprint"`
	}
	if err := requireWorkspace(workspace); err != nil {
		return out.Blueprint, err
	}
	body := map[string]any{
		"name": request.Name, "description": request.Description, "class": "user",
		"job_type": "query", "is_disabled": false, "allow_concurrent_jobs": false,
		"job_configurations": map[string]any{
			"source_view_id": request.Configuration.SourceViewID,
			"target":         request.Configuration.Target, "properties": request.Configuration.Properties,
		},
	}
	err := c.do(ctx, http.MethodPost, "/apis/v1/w/"+url.PathEscape(workspace)+"/blueprint", nil, body, &out)
	return out.Blueprint, err
}

func (c *Client) ListBlueprints(ctx context.Context, workspace string, query ViewQuery) (BlueprintList, error) {
	var out BlueprintList
	if err := requireWorkspace(workspace); err != nil {
		return out, err
	}
	values := pageQuery(query)
	values.Set("class", "user")
	values.Set("job_type", "query")
	err := c.do(ctx, http.MethodGet, "/apis/v1/w/"+url.PathEscape(workspace)+"/blueprints", values, nil, &out)
	return out, err
}

func (c *Client) GetBlueprint(ctx context.Context, workspace, id string) (Blueprint, error) {
	var out struct {
		Blueprint Blueprint `json:"blueprint"`
	}
	path, err := scopedPath(workspace, "blueprint", id)
	if err != nil {
		return out.Blueprint, err
	}
	err = c.do(ctx, http.MethodGet, path, nil, nil, &out)
	return out.Blueprint, err
}

func (c *Client) SpawnBlueprintJob(ctx context.Context, workspace, id string) (string, error) {
	var out struct {
		Message string `json:"message"`
	}
	path, err := scopedPath(workspace, "blueprint", id)
	if err != nil {
		return "", err
	}
	err = c.do(ctx, http.MethodPost, path+"/spawn", nil, nil, &out)
	return out.Message, err
}

func (c *Client) ListBlueprintJobs(ctx context.Context, workspace, blueprintID string, query ViewQuery) (JobList, error) {
	var out JobList
	if _, err := scopedPath(workspace, "blueprint", blueprintID); err != nil {
		return out, err
	}
	values := pageQuery(query)
	values.Set("blueprint_id", blueprintID)
	values.Set("order_by", "created_at")
	values.Set("order", "-1")
	err := c.do(ctx, http.MethodGet, "/apis/v1/w/"+url.PathEscape(workspace)+"/jobs", values, nil, &out)
	return out, err
}

func (c *Client) GetJob(ctx context.Context, workspace, id string) (Job, error) {
	var out struct {
		Job Job `json:"job"`
	}
	path, err := scopedPath(workspace, "job", id)
	if err != nil {
		return out.Job, err
	}
	err = c.do(ctx, http.MethodGet, path, nil, nil, &out)
	return out.Job, err
}
