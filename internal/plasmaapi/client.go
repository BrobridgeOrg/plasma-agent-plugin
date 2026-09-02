// Package plasmaapi is a narrow client for the Plasma REST endpoints this
// plugin uses. It covers only those endpoints on purpose: a fuller client
// would be a second, drifting copy of Plasma's contract.
package plasmaapi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/BrobridgeOrg/plasma-plugin/internal/pcontext"
)

// tokenExpirySlack renews slightly early so a call does not race its own
// token expiry.
const tokenExpirySlack = 30 * time.Second

// Client talks to one Plasma deployment as one user.
//
// It is safe for concurrent use: the JWT is guarded by a mutex, and the
// state file it persists to is written atomically by pcontext.
type Client struct {
	cfg  pcontext.Config
	home *pcontext.Home
	http *http.Client

	mu sync.Mutex
}

// New builds a client. A nil httpClient gets a 60s default — long enough for
// a Trino-backed query execution, short enough to fail rather than hang a
// tool call forever.
func New(cfg pcontext.Config, home *pcontext.Home, httpClient *http.Client) *Client {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 60 * time.Second}
	}
	return &Client{cfg: cfg, home: home, http: httpClient}
}

// BaseURL is the Plasma deployment this client addresses.
func (c *Client) BaseURL() string { return strings.TrimRight(c.cfg.PlasmaURL, "/") }

// VerifyToken reports who the current bearer belongs to.
func (c *Client) VerifyToken(ctx context.Context) (User, error) {
	var out User
	err := c.do(ctx, http.MethodPost, "/apis/v1/auth/verify-token", nil, nil, &out)
	return out, err
}

// MyWorkspaces lists the workspaces this user is a member of.
func (c *Client) MyWorkspaces(ctx context.Context) ([]Workspace, error) {
	var out struct {
		Workspaces []Workspace `json:"workspaces"`
	}
	if err := c.do(ctx, http.MethodGet, "/apis/v1/my_workspaces", nil, nil, &out); err != nil {
		return nil, err
	}
	return out.Workspaces, nil
}

// ListViews returns one page of the workspace's views.
func (c *Client) ListViews(ctx context.Context, workspace string, q ViewQuery) (ViewList, error) {
	var out ViewList
	if err := requireWorkspace(workspace); err != nil {
		return out, err
	}
	query := url.Values{}
	if q.Keywords != "" {
		query.Set("keywords", q.Keywords)
	}
	if q.Page > 0 {
		query.Set("page", strconv.Itoa(q.Page))
	}
	if q.PageSize > 0 {
		query.Set("page_size", strconv.Itoa(q.PageSize))
	}
	err := c.do(ctx, http.MethodGet, "/apis/v1/w/"+workspace+"/views", query, nil, &out)
	return out, err
}

// GetView reads one view, including its sync state.
func (c *Client) GetView(ctx context.Context, workspace, viewID string) (View, error) {
	var out struct {
		View View `json:"view"`
	}
	if err := requireWorkspace(workspace); err != nil {
		return View{}, err
	}
	err := c.do(ctx, http.MethodGet,
		"/apis/v1/w/"+workspace+"/view/"+viewID, nil, nil, &out)
	return out.View, err
}

// BlueprintHistory lists the blueprints a materialized view has had.
func (c *Client) BlueprintHistory(ctx context.Context, workspace, viewID string) ([]BlueprintHistory, error) {
	if err := requireWorkspace(workspace); err != nil {
		return nil, err
	}
	var out struct {
		History []BlueprintHistory `json:"history"`
	}
	if err := c.do(ctx, http.MethodGet,
		"/apis/v1/w/"+workspace+"/view/"+viewID+"/blueprint_history", nil, nil, &out); err != nil {
		return nil, err
	}
	return out.History, nil
}

// RunQuery executes SQL synchronously. Plasma enforces SELECT-only and a
// 100-row ceiling server-side; this method does not pretend to offer more.
func (c *Client) RunQuery(ctx context.Context, workspace, sql string) (QueryResult, error) {
	if err := requireWorkspace(workspace); err != nil {
		return QueryResult{}, err
	}
	var out struct {
		QueryID string `json:"query_id"`
		Data    *struct {
			Columns []string `json:"columns"`
			Rows    [][]any  `json:"rows"`
		} `json:"data"`
		ElapsedMs int64 `json:"elapsed_ms"`
	}
	body := map[string]string{"query": sql}
	if err := c.do(ctx, http.MethodPost,
		"/apis/v1/w/"+workspace+"/query/execution", nil, body, &out); err != nil {
		return QueryResult{}, err
	}
	result := QueryResult{QueryID: out.QueryID, ElapsedMs: out.ElapsedMs}
	if out.Data != nil {
		result.Columns = out.Data.Columns
		result.Rows = out.Data.Rows
	}
	return result, nil
}

// CreateView creates a view or materialized view.
func (c *Client) CreateView(ctx context.Context, workspace string, req CreateViewRequest) (View, error) {
	if err := requireWorkspace(workspace); err != nil {
		return View{}, err
	}
	var out struct {
		View View `json:"view"`
	}
	err := c.do(ctx, http.MethodPost, "/apis/v1/w/"+workspace+"/view", nil, req, &out)
	return out.View, err
}

// SyncView triggers one sync and returns the server's message.
func (c *Client) SyncView(ctx context.Context, workspace, viewID string) (string, error) {
	if err := requireWorkspace(workspace); err != nil {
		return "", err
	}
	var out struct {
		Message string `json:"message"`
	}
	err := c.do(ctx, http.MethodPost,
		"/apis/v1/w/"+workspace+"/view/"+viewID+"/sync", nil, nil, &out)
	return out.Message, err
}

// CreateAccessEntry grants external access to a view's data.
func (c *Client) CreateAccessEntry(ctx context.Context, workspace, viewID string,
	req CreateAccessEntryRequest) (AccessEntryCreated, error) {

	if err := requireWorkspace(workspace); err != nil {
		return AccessEntryCreated{}, err
	}
	body := map[string]any{"name": req.Name}
	if req.Description != "" {
		body["description"] = req.Description
	}
	if req.AuthType != "" {
		body["auth_type"] = req.AuthType
	}
	if req.SecretKey != "" {
		body["secret_key"] = req.SecretKey
	}
	if req.ExpiredAt != nil {
		body["expired_at"] = req.ExpiredAt.UTC().Format(time.RFC3339)
	}
	var out AccessEntryCreated
	err := c.do(ctx, http.MethodPost,
		"/apis/v1/w/"+workspace+"/view/"+viewID+"/access_entry", nil, body, &out)
	return out, err
}

// ListAccessEntries lists a view's external access grants.
func (c *Client) ListAccessEntries(ctx context.Context, workspace, viewID string) ([]AccessEntry, error) {
	if err := requireWorkspace(workspace); err != nil {
		return nil, err
	}
	var out struct {
		Entries []AccessEntry `json:"entries"`
	}
	if err := c.do(ctx, http.MethodGet,
		"/apis/v1/w/"+workspace+"/view/"+viewID+"/access_entries", nil, nil, &out); err != nil {
		return nil, err
	}
	return out.Entries, nil
}

// ExportURL is the view's data API URL. A view with no successful sync
// answers ErrNotReady.
func (c *Client) ExportURL(ctx context.Context, workspace, viewID string) (string, error) {
	if err := requireWorkspace(workspace); err != nil {
		return "", err
	}
	var out struct {
		ExportURL string `json:"export_url"`
	}
	err := c.do(ctx, http.MethodGet,
		"/apis/v1/w/"+workspace+"/view/"+viewID+"/export/url", nil, nil, &out)
	return out.ExportURL, err
}

func requireWorkspace(workspace string) error {
	if strings.TrimSpace(workspace) == "" {
		return ErrNoWorkspace
	}
	return nil
}

// do performs one request, and on a 401 repairs authentication once before
// retrying. "Once" is the whole policy: a second 401 after a fresh login is
// a real refusal, and retrying it would only turn one failure into a loop.
func (c *Client) do(ctx context.Context, method, path string, query url.Values, body, out any) error {
	token, err := c.token(ctx)
	if err != nil {
		return err
	}
	apiErr, err := c.attempt(ctx, method, path, query, body, token, out)
	if err != nil {
		return err
	}
	if apiErr == nil {
		return nil
	}
	if apiErr.Status != http.StatusUnauthorized {
		return apiErr
	}

	mode, err := c.cfg.PlasmaAuthMode()
	if err != nil {
		return err
	}
	if mode == pcontext.AuthStaticToken {
		// Nothing to repair: the operator gave us this token verbatim.
		return apiErr
	}
	token, err = c.reauthenticate(ctx)
	if err != nil {
		return err
	}
	apiErr, err = c.attempt(ctx, method, path, query, body, token, out)
	if err != nil {
		return err
	}
	if apiErr != nil {
		return apiErr
	}
	return nil
}

// attempt returns (*APIError, nil) for an HTTP-level refusal and (nil, err)
// for a transport or decoding failure, so callers can tell "the server said
// no" from "we never got an answer".
func (c *Client) attempt(ctx context.Context, method, path string, query url.Values,
	body any, token string, out any) (*APIError, error) {

	if c.BaseURL() == "" {
		return nil, fmt.Errorf("PLASMA_URL is not set")
	}
	endpoint := c.BaseURL() + path
	if len(query) > 0 {
		endpoint += "?" + query.Encode()
	}

	var reader io.Reader
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		reader = bytes.NewReader(encoded)
	}
	req, err := http.NewRequestWithContext(ctx, method, endpoint, reader)
	if err != nil {
		return nil, err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("plasma %s %s: %w", method, path, err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return &APIError{
			Status: resp.StatusCode, Method: method, Path: path,
			Detail: detailFromBody(raw), kind: kindForStatus(resp.StatusCode),
		}, nil
	}
	if out == nil || len(bytes.TrimSpace(raw)) == 0 {
		return nil, nil
	}
	if err := json.Unmarshal(raw, out); err != nil {
		return nil, fmt.Errorf("plasma %s %s: unreadable response: %w", method, path, err)
	}
	return nil, nil
}

// token returns a usable bearer, logging in first if that is the configured
// mode and nothing valid is stored.
func (c *Client) token(ctx context.Context) (string, error) {
	mode, err := c.cfg.PlasmaAuthMode()
	if err != nil {
		return "", err
	}
	if mode == pcontext.AuthStaticToken {
		return c.cfg.PlasmaToken, nil
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	st, err := c.home.LoadState()
	if err != nil {
		return "", err
	}
	if st.AccessToken != "" && time.Until(st.ExpiresAt) > tokenExpirySlack {
		return st.AccessToken, nil
	}
	return c.login(ctx, st)
}

// reauthenticate refreshes, and falls back to a full login when the refresh
// token is no longer accepted.
func (c *Client) reauthenticate(ctx context.Context) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	st, err := c.home.LoadState()
	if err != nil {
		return "", err
	}
	if st.RefreshToken != "" {
		if token, err := c.refresh(ctx, st); err == nil {
			return token, nil
		}
	}
	return c.login(ctx, st)
}

type authResponse struct {
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token"`
	ExpiresAt    time.Time `json:"expires_at"`
}

// login exchanges the configured credentials for a JWT and stores it.
func (c *Client) login(ctx context.Context, st pcontext.State) (string, error) {
	var out authResponse
	apiErr, err := c.attempt(ctx, http.MethodPost, "/apis/v1/auth/login", nil,
		map[string]string{
			"username_or_email": c.cfg.PlasmaUsername,
			"password":          c.cfg.PlasmaPassword,
		}, "", &out)
	if err != nil {
		return "", err
	}
	if apiErr != nil {
		return "", fmt.Errorf("plasma login failed: %w", apiErr)
	}
	return c.storeTokens(st, out)
}

// refresh trades the refresh token for a new access token.
func (c *Client) refresh(ctx context.Context, st pcontext.State) (string, error) {
	var out authResponse
	apiErr, err := c.attempt(ctx, http.MethodPost, "/apis/v1/auth/refresh", nil,
		map[string]string{"refresh_token": st.RefreshToken}, "", &out)
	if err != nil {
		return "", err
	}
	if apiErr != nil {
		return "", apiErr
	}
	return c.storeTokens(st, out)
}

// storeTokens persists the JWT next to the selected workspace, so the other
// server in this plugin sees the same session.
func (c *Client) storeTokens(st pcontext.State, out authResponse) (string, error) {
	if out.AccessToken == "" {
		return "", fmt.Errorf("plasma returned no access token")
	}
	st.AccessToken = out.AccessToken
	if out.RefreshToken != "" {
		st.RefreshToken = out.RefreshToken
	}
	st.ExpiresAt = out.ExpiresAt
	if err := c.home.SaveState(st); err != nil {
		return "", err
	}
	return out.AccessToken, nil
}
