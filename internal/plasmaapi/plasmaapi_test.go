package plasmaapi

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/BrobridgeOrg/plasma-plugin/internal/pcontext"
)

// harness wires a Client against a fake Plasma.
type harness struct {
	t       *testing.T
	server  *httptest.Server
	home    *pcontext.Home
	client  *Client
	reqs    []recorded
	handler func(w http.ResponseWriter, r *http.Request)
}

type recorded struct {
	method string
	path   string
	query  string
	auth   string
	body   string
}

func newHarness(t *testing.T, cfg pcontext.Config) *harness {
	t.Helper()
	h := &harness{t: t, home: pcontext.New(t.TempDir())}
	h.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		h.reqs = append(h.reqs, recorded{
			method: r.Method, path: r.URL.Path, query: r.URL.RawQuery,
			auth: r.Header.Get("Authorization"), body: string(body),
		})
		h.handler(w, r)
	}))
	t.Cleanup(h.server.Close)
	cfg.PlasmaURL = h.server.URL
	h.client = New(cfg, h.home, h.server.Client())
	return h
}

func (h *harness) calls(path string) int {
	n := 0
	for _, r := range h.reqs {
		if r.path == path {
			n++
		}
	}
	return n
}

func tokenConfig() pcontext.Config { return pcontext.Config{PlasmaToken: "static-token"} }

func passwordConfig() pcontext.Config {
	return pcontext.Config{PlasmaUsername: "admin", PlasmaPassword: "pw"}
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func TestMyWorkspacesSendsStaticTokenAndParsesList(t *testing.T) {
	h := newHarness(t, tokenConfig())
	h.handler = func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, 200, map[string]any{"workspaces": []map[string]any{
			{"id": "ws-1", "name": "his_database", "role": "owner"},
			{"id": "ws-2", "name": "sales", "role": "member"},
		}})
	}

	got, err := h.client.MyWorkspaces(context.Background())
	if err != nil {
		t.Fatalf("MyWorkspaces: %v", err)
	}
	if len(got) != 2 || got[0].ID != "ws-1" || got[0].Name != "his_database" {
		t.Fatalf("workspaces = %+v", got)
	}
	if h.reqs[0].path != "/apis/v1/my_workspaces" {
		t.Errorf("path = %q", h.reqs[0].path)
	}
	if h.reqs[0].auth != "Bearer static-token" {
		t.Errorf("Authorization = %q", h.reqs[0].auth)
	}
}

func TestPasswordModeLogsInThenPersistsTokens(t *testing.T) {
	h := newHarness(t, passwordConfig())
	h.handler = func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/apis/v1/auth/login":
			writeJSON(w, 200, map[string]any{
				"access_token": "at-1", "refresh_token": "rt-1",
				"expires_at": time.Now().Add(time.Hour).UTC().Format(time.RFC3339),
			})
		default:
			writeJSON(w, 200, map[string]any{"workspaces": []map[string]any{}})
		}
	}

	if _, err := h.client.MyWorkspaces(context.Background()); err != nil {
		t.Fatalf("MyWorkspaces: %v", err)
	}

	if got := h.reqs[0].path; got != "/apis/v1/auth/login" {
		t.Fatalf("first call = %q, want the login", got)
	}
	if !strings.Contains(h.reqs[0].body, `"username_or_email":"admin"`) {
		t.Errorf("login body = %s", h.reqs[0].body)
	}
	if h.reqs[1].auth != "Bearer at-1" {
		t.Errorf("Authorization = %q, want the fresh access token", h.reqs[1].auth)
	}
	st, err := h.home.LoadState()
	if err != nil {
		t.Fatal(err)
	}
	if st.AccessToken != "at-1" || st.RefreshToken != "rt-1" {
		t.Errorf("state tokens = %q/%q, want them persisted", st.AccessToken, st.RefreshToken)
	}
}

func TestPasswordModeReusesUnexpiredStoredToken(t *testing.T) {
	h := newHarness(t, passwordConfig())
	if err := h.home.SaveState(pcontext.State{
		AccessToken: "stored", RefreshToken: "rt", ExpiresAt: time.Now().Add(time.Hour),
	}); err != nil {
		t.Fatal(err)
	}
	h.handler = func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, 200, map[string]any{"workspaces": []map[string]any{}})
	}

	if _, err := h.client.MyWorkspaces(context.Background()); err != nil {
		t.Fatalf("MyWorkspaces: %v", err)
	}

	if n := h.calls("/apis/v1/auth/login"); n != 0 {
		t.Errorf("login called %d times, want 0 while the stored token is valid", n)
	}
	if h.reqs[0].auth != "Bearer stored" {
		t.Errorf("Authorization = %q", h.reqs[0].auth)
	}
}

func TestUnauthorizedTriggersRefreshThenRetries(t *testing.T) {
	h := newHarness(t, passwordConfig())
	if err := h.home.SaveState(pcontext.State{
		AccessToken: "stale", RefreshToken: "rt-1", ExpiresAt: time.Now().Add(time.Hour),
	}); err != nil {
		t.Fatal(err)
	}
	h.handler = func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/apis/v1/auth/refresh":
			writeJSON(w, 200, map[string]any{
				"access_token": "at-2", "refresh_token": "rt-2",
				"expires_at": time.Now().Add(time.Hour).UTC().Format(time.RFC3339),
			})
		case r.Header.Get("Authorization") == "Bearer stale":
			writeJSON(w, 401, map[string]any{"error": "token expired"})
		default:
			writeJSON(w, 200, map[string]any{"workspaces": []map[string]any{}})
		}
	}

	if _, err := h.client.MyWorkspaces(context.Background()); err != nil {
		t.Fatalf("MyWorkspaces: %v", err)
	}

	if n := h.calls("/apis/v1/my_workspaces"); n != 2 {
		t.Errorf("workspaces called %d times, want 2 (the 401 and the retry)", n)
	}
	st, _ := h.home.LoadState()
	if st.AccessToken != "at-2" {
		t.Errorf("stored access token = %q, want the refreshed one", st.AccessToken)
	}
}

func TestFailedRefreshFallsBackToLogin(t *testing.T) {
	h := newHarness(t, passwordConfig())
	if err := h.home.SaveState(pcontext.State{
		AccessToken: "stale", RefreshToken: "dead", ExpiresAt: time.Now().Add(time.Hour),
	}); err != nil {
		t.Fatal(err)
	}
	h.handler = func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/apis/v1/auth/refresh":
			writeJSON(w, 401, map[string]any{"error": "refresh token rejected"})
		case r.URL.Path == "/apis/v1/auth/login":
			writeJSON(w, 200, map[string]any{
				"access_token": "at-3", "refresh_token": "rt-3",
				"expires_at": time.Now().Add(time.Hour).UTC().Format(time.RFC3339),
			})
		case r.Header.Get("Authorization") == "Bearer stale":
			writeJSON(w, 401, map[string]any{"error": "token expired"})
		default:
			writeJSON(w, 200, map[string]any{"workspaces": []map[string]any{}})
		}
	}

	if _, err := h.client.MyWorkspaces(context.Background()); err != nil {
		t.Fatalf("MyWorkspaces: %v", err)
	}

	if n := h.calls("/apis/v1/auth/login"); n != 1 {
		t.Errorf("login called %d times, want 1 after the refresh was rejected", n)
	}
	st, _ := h.home.LoadState()
	if st.AccessToken != "at-3" {
		t.Errorf("stored access token = %q", st.AccessToken)
	}
}

func TestStaticTokenIsNeverRefreshed(t *testing.T) {
	h := newHarness(t, tokenConfig())
	h.handler = func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, 401, map[string]any{"error": "nope"})
	}

	_, err := h.client.MyWorkspaces(context.Background())
	if err == nil {
		t.Fatal("want an error when the static token is rejected")
	}
	if n := h.calls("/apis/v1/auth/login") + h.calls("/apis/v1/auth/refresh"); n != 0 {
		t.Errorf("%d auth calls, want 0: a static token has nothing to refresh", n)
	}
	if !strings.Contains(err.Error(), "401") {
		t.Errorf("error = %v, want the status in it", err)
	}
}

func TestMissingCredentialsFailsBeforeAnyRequest(t *testing.T) {
	h := newHarness(t, pcontext.Config{})
	h.handler = func(w http.ResponseWriter, r *http.Request) { writeJSON(w, 200, map[string]any{}) }

	_, err := h.client.MyWorkspaces(context.Background())
	if !errors.Is(err, pcontext.ErrNoPlasmaCredentials) {
		t.Fatalf("err = %v, want ErrNoPlasmaCredentials", err)
	}
	if len(h.reqs) != 0 {
		t.Errorf("%d requests made, want none", len(h.reqs))
	}
}

func TestRunQueryPostsQueryAndParsesColumnarResult(t *testing.T) {
	h := newHarness(t, tokenConfig())
	h.handler = func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, 200, map[string]any{
			"query_id": "q-1",
			"data": map[string]any{
				"columns": []string{"id", "name"},
				"rows":    [][]any{{1, "Alice"}, {2, nil}},
			},
			"elapsed_ms": 245,
		})
	}

	got, err := h.client.RunQuery(context.Background(), "ws-1", "SELECT id, name FROM t")
	if err != nil {
		t.Fatalf("RunQuery: %v", err)
	}
	if h.reqs[0].path != "/apis/v1/w/ws-1/query/execution" {
		t.Errorf("path = %q", h.reqs[0].path)
	}
	if !strings.Contains(h.reqs[0].body, `"query":"SELECT id, name FROM t"`) {
		t.Errorf("body = %s", h.reqs[0].body)
	}
	if got.QueryID != "q-1" || got.ElapsedMs != 245 {
		t.Errorf("result meta = %+v", got)
	}
	if len(got.Columns) != 2 || len(got.Rows) != 2 {
		t.Errorf("columns/rows = %v / %v", got.Columns, got.Rows)
	}
}

func TestRunQuerySurfacesValidationViolations(t *testing.T) {
	h := newHarness(t, tokenConfig())
	h.handler = func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, 400, map[string]any{
			"error":      "SQL validation failed",
			"violations": []string{"only SELECT statements are allowed"},
		})
	}

	_, err := h.client.RunQuery(context.Background(), "ws-1", "DROP TABLE t")
	if err == nil {
		t.Fatal("want an error")
	}
	if !strings.Contains(err.Error(), "only SELECT statements are allowed") {
		t.Errorf("error = %v, want the server's violation in it", err)
	}
}

func TestCreateViewPostsRequestAndParsesView(t *testing.T) {
	h := newHarness(t, tokenConfig())
	h.handler = func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, 201, map[string]any{
			"message": "created",
			"view": map[string]any{
				"id": "v-1", "name": "daily", "type": "materialized_view",
				"status": "initializing", "path": "ws.public.daily", "sync_mode": "scheduled",
			},
		})
	}

	got, err := h.client.CreateView(context.Background(), "ws-1", CreateViewRequest{
		Name: "daily", Type: "materialized_view", ViewSQL: "SELECT 1", SyncMode: "scheduled",
		SchedulerSettings: json.RawMessage(`{"start":"immediately"}`),
	})
	if err != nil {
		t.Fatalf("CreateView: %v", err)
	}
	if h.reqs[0].method != http.MethodPost || h.reqs[0].path != "/apis/v1/w/ws-1/view" {
		t.Errorf("request = %s %s", h.reqs[0].method, h.reqs[0].path)
	}
	if !strings.Contains(h.reqs[0].body, `"scheduler_settings":{"start":"immediately"}`) {
		t.Errorf("body = %s", h.reqs[0].body)
	}
	if got.ID != "v-1" || got.Status != "initializing" || got.Path != "ws.public.daily" {
		t.Errorf("view = %+v", got)
	}
}

func TestCreateViewOmitsSchedulerSettingsWhenAbsent(t *testing.T) {
	h := newHarness(t, tokenConfig())
	h.handler = func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, 201, map[string]any{"view": map[string]any{"id": "v-1"}})
	}

	if _, err := h.client.CreateView(context.Background(), "ws-1", CreateViewRequest{
		Name: "manual", Type: "materialized_view", ViewSQL: "SELECT 1", SyncMode: "manual",
	}); err != nil {
		t.Fatalf("CreateView: %v", err)
	}

	if strings.Contains(h.reqs[0].body, "scheduler_settings") {
		t.Errorf("body = %s, want no scheduler_settings key when none was asked for", h.reqs[0].body)
	}
}

func TestGetViewNotFoundIsTyped(t *testing.T) {
	h := newHarness(t, tokenConfig())
	h.handler = func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, 404, map[string]any{"error": "View not found"})
	}

	_, err := h.client.GetView(context.Background(), "ws-1", "nope")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}

func TestGetViewParsesSyncFields(t *testing.T) {
	h := newHarness(t, tokenConfig())
	h.handler = func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, 200, map[string]any{"view": map[string]any{
			"id": "v-1", "name": "daily", "status": "active",
			"last_sync_status": "synced", "version": 3, "view_sql": "SELECT 1",
		}})
	}

	got, err := h.client.GetView(context.Background(), "ws-1", "v-1")
	if err != nil {
		t.Fatalf("GetView: %v", err)
	}
	if got.LastSyncStatus != "synced" || got.Version != 3 {
		t.Errorf("view = %+v", got)
	}
}

func TestForbiddenIsTyped(t *testing.T) {
	h := newHarness(t, tokenConfig())
	h.handler = func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, 403, map[string]any{"error": "Access denied: user is not a member of the workspace"})
	}

	_, err := h.client.GetView(context.Background(), "ws-1", "v-1")
	if !errors.Is(err, ErrForbidden) {
		t.Fatalf("err = %v, want ErrForbidden", err)
	}
}

func TestSyncViewPostsToSyncEndpoint(t *testing.T) {
	h := newHarness(t, tokenConfig())
	h.handler = func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, 200, map[string]any{"message": "sync started"})
	}

	msg, err := h.client.SyncView(context.Background(), "ws-1", "v-1")
	if err != nil {
		t.Fatalf("SyncView: %v", err)
	}
	if h.reqs[0].method != http.MethodPost || h.reqs[0].path != "/apis/v1/w/ws-1/view/v-1/sync" {
		t.Errorf("request = %s %s", h.reqs[0].method, h.reqs[0].path)
	}
	if msg != "sync started" {
		t.Errorf("message = %q", msg)
	}
}

func TestCreateAccessEntrySendsAuthTypeAndExpiry(t *testing.T) {
	h := newHarness(t, tokenConfig())
	h.handler = func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, 201, map[string]any{
			"message": "created",
			"access_entry": map[string]any{
				"id": "e-1", "name": "powerbi", "auth_type": "api_key",
				"secret_key": "sk-abc", "expired_at": "2026-10-02T00:00:00Z",
			},
			"access_url": "http://plasma/apis/v1/public/view/abc",
		})
	}
	expiry := time.Date(2026, 10, 2, 0, 0, 0, 0, time.UTC)

	got, err := h.client.CreateAccessEntry(context.Background(), "ws-1", "v-1", CreateAccessEntryRequest{
		Name: "powerbi", AuthType: "api_key", ExpiredAt: &expiry,
	})
	if err != nil {
		t.Fatalf("CreateAccessEntry: %v", err)
	}
	if h.reqs[0].path != "/apis/v1/w/ws-1/view/v-1/access_entry" {
		t.Errorf("path = %q", h.reqs[0].path)
	}
	if !strings.Contains(h.reqs[0].body, `"auth_type":"api_key"`) ||
		!strings.Contains(h.reqs[0].body, `"expired_at":"2026-10-02T00:00:00Z"`) {
		t.Errorf("body = %s", h.reqs[0].body)
	}
	if got.AccessURL != "http://plasma/apis/v1/public/view/abc" || got.Entry.SecretKey != "sk-abc" {
		t.Errorf("created = %+v", got)
	}
}

func TestCreateAccessEntryOmitsExpiryWhenNil(t *testing.T) {
	h := newHarness(t, tokenConfig())
	h.handler = func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, 201, map[string]any{"access_entry": map[string]any{"id": "e-1"}})
	}

	if _, err := h.client.CreateAccessEntry(context.Background(), "ws-1", "v-1",
		CreateAccessEntryRequest{Name: "forever", AuthType: "none"}); err != nil {
		t.Fatalf("CreateAccessEntry: %v", err)
	}

	if strings.Contains(h.reqs[0].body, `"expired_at"`) {
		t.Errorf("body = %s, want expired_at omitted so the server applies its own default", h.reqs[0].body)
	}
}

func TestListAccessEntriesParsesEntries(t *testing.T) {
	h := newHarness(t, tokenConfig())
	h.handler = func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, 200, map[string]any{"entries": []map[string]any{
			{"id": "e-1", "name": "powerbi", "auth_type": "api_key"},
		}, "total": 1})
	}

	got, err := h.client.ListAccessEntries(context.Background(), "ws-1", "v-1")
	if err != nil {
		t.Fatalf("ListAccessEntries: %v", err)
	}
	if h.reqs[0].path != "/apis/v1/w/ws-1/view/v-1/access_entries" {
		t.Errorf("path = %q", h.reqs[0].path)
	}
	if len(got) != 1 || got[0].Name != "powerbi" {
		t.Errorf("entries = %+v", got)
	}
}

func TestExportURLParsesURL(t *testing.T) {
	h := newHarness(t, tokenConfig())
	h.handler = func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, 200, map[string]any{"export_url": "http://plasma/export/abc"})
	}

	got, err := h.client.ExportURL(context.Background(), "ws-1", "v-1")
	if err != nil {
		t.Fatalf("ExportURL: %v", err)
	}
	if h.reqs[0].path != "/apis/v1/w/ws-1/view/v-1/export/url" {
		t.Errorf("path = %q", h.reqs[0].path)
	}
	if got != "http://plasma/export/abc" {
		t.Errorf("url = %q", got)
	}
}

func TestExportURLOnUnsyncedViewIsTyped(t *testing.T) {
	h := newHarness(t, tokenConfig())
	h.handler = func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, 409, map[string]any{"message": "view is not ready"})
	}

	_, err := h.client.ExportURL(context.Background(), "ws-1", "v-1")
	if !errors.Is(err, ErrNotReady) {
		t.Fatalf("err = %v, want ErrNotReady", err)
	}
}

func TestListViewsPassesPaginationAndKeywords(t *testing.T) {
	h := newHarness(t, tokenConfig())
	h.handler = func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, 200, map[string]any{
			"views": []map[string]any{{"id": "v-1", "name": "daily"}},
			"total": 1, "page": 2, "page_size": 5,
		})
	}

	got, err := h.client.ListViews(context.Background(), "ws-1", ViewQuery{
		Keywords: "daily", Page: 2, PageSize: 5,
	})
	if err != nil {
		t.Fatalf("ListViews: %v", err)
	}
	if h.reqs[0].path != "/apis/v1/w/ws-1/views" {
		t.Errorf("path = %q", h.reqs[0].path)
	}
	q := h.reqs[0].query
	for _, want := range []string{"keywords=daily", "page=2", "page_size=5"} {
		if !strings.Contains(q, want) {
			t.Errorf("query = %q, want %s in it", q, want)
		}
	}
	if got.Total != 1 || len(got.Views) != 1 {
		t.Errorf("list = %+v", got)
	}
}

func TestBlueprintHistoryParsesEntries(t *testing.T) {
	h := newHarness(t, tokenConfig())
	h.handler = func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, 200, map[string]any{
			"history": []map[string]any{{"blueprint_id": "bp-1", "view_id": "v-1"}},
			"total":   1,
		})
	}

	got, err := h.client.BlueprintHistory(context.Background(), "ws-1", "v-1")
	if err != nil {
		t.Fatalf("BlueprintHistory: %v", err)
	}
	if h.reqs[0].path != "/apis/v1/w/ws-1/view/v-1/blueprint_history" {
		t.Errorf("path = %q", h.reqs[0].path)
	}
	if len(got) != 1 || got[0].BlueprintID != "bp-1" {
		t.Errorf("history = %+v", got)
	}
}

func TestVerifyTokenReturnsUser(t *testing.T) {
	h := newHarness(t, tokenConfig())
	h.handler = func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, 200, map[string]any{"id": "u-1", "username": "admin", "role": "admin"})
	}

	got, err := h.client.VerifyToken(context.Background())
	if err != nil {
		t.Fatalf("VerifyToken: %v", err)
	}
	if h.reqs[0].path != "/apis/v1/auth/verify-token" {
		t.Errorf("path = %q", h.reqs[0].path)
	}
	if got.Username != "admin" || got.Role != "admin" {
		t.Errorf("user = %+v", got)
	}
}

func TestEmptyWorkspaceIsRejectedBeforeRequest(t *testing.T) {
	h := newHarness(t, tokenConfig())
	h.handler = func(w http.ResponseWriter, r *http.Request) { writeJSON(w, 200, map[string]any{}) }

	if _, err := h.client.GetView(context.Background(), "", "v-1"); err == nil {
		t.Fatal("want an error for an empty workspace")
	}
	if len(h.reqs) != 0 {
		t.Errorf("%d requests made, want none: an empty workspace would address the wrong URL", len(h.reqs))
	}
}
