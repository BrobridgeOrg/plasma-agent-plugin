package plasmaapi

import (
	"encoding/json"
	"time"
)

// User is who the bearer belongs to.
type User struct {
	ID       string `json:"id"`
	Username string `json:"username"`
	Email    string `json:"email"`
	Role     string `json:"role"`
}

// Workspace is one membership from /my_workspaces.
type Workspace struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Role        string `json:"role"`
	IsPrivate   bool   `json:"is_private"`
}

// View covers the fields a caller reasons about. Plasma returns more; the
// decoder ignores what is not named here so a server-side addition does not
// break this client.
type View struct {
	ID             string `json:"id"`
	Name           string `json:"name"`
	Description    string `json:"description"`
	Type           string `json:"type"`
	Status         string `json:"status"`
	Version        int64  `json:"version"`
	ViewSQL        string `json:"view_sql"`
	Path           string `json:"path"`
	SyncMode       string `json:"sync_mode"`
	BlueprintID    string `json:"blueprint_id"`
	LastSyncStatus string `json:"last_sync_status"`
	LastSyncAt     string `json:"last_sync_at"`
}

// ViewQuery is the pagination/search of ListViews. Zero values are omitted so
// the server applies its own defaults.
type ViewQuery struct {
	Keywords string
	Page     int
	PageSize int
}

// ViewList is one page of views.
type ViewList struct {
	Views      []View `json:"views"`
	Total      int64  `json:"total"`
	Page       int    `json:"page"`
	PageSize   int    `json:"page_size"`
	TotalPages int    `json:"total_pages"`
}

// CreateViewRequest creates a view or materialized view. With
// SchedulerSettings present, Plasma creates the view, its blueprint and its
// schedule in this one call and runs the first sync immediately.
//
// SchedulerSettings stays raw JSON on purpose: it is Plasma's scheduler
// contract (scheduler.CronTypeInfo), and re-declaring it here would add a
// second place to keep in step for no gain.
type CreateViewRequest struct {
	Name              string          `json:"name"`
	Description       string          `json:"description,omitempty"`
	ViewSQL           string          `json:"view_sql"`
	Type              string          `json:"type"`
	SyncMode          string          `json:"sync_mode,omitempty"`
	SchedulerSettings json.RawMessage `json:"scheduler_settings,omitempty"`
}

// AccessEntry is one external access grant on a view.
type AccessEntry struct {
	ID          string  `json:"id"`
	Name        string  `json:"name"`
	Description string  `json:"description"`
	AuthType    string  `json:"auth_type"`
	SecretKey   string  `json:"secret_key"`
	ExpiredAt   *string `json:"expired_at"`
	AccessURL   string  `json:"access_url"`
	CreatedAt   string  `json:"created_at"`
}

// CreateAccessEntryRequest is the body Plasma accepts on a view's
// access_entry endpoint. Format and entry type are not part of it — those
// live on the manifest, not here.
type CreateAccessEntryRequest struct {
	Name        string
	Description string
	AuthType    string
	// SecretKey is optional; Plasma generates one when empty. For basic_auth
	// it must be "username:password".
	SecretKey string
	// ExpiredAt nil leaves the field out of the body entirely, so the server
	// decides. Never guess in the caller's name.
	ExpiredAt *time.Time
}

// AccessEntryCreated is the created grant plus the URL it is reachable at.
type AccessEntryCreated struct {
	Entry     AccessEntry `json:"access_entry"`
	AccessURL string      `json:"access_url"`
	Message   string      `json:"message"`
}

// QueryResult is one synchronous query execution. Plasma caps it server-side
// at 100 rows and rejects anything that is not a SELECT.
type QueryResult struct {
	QueryID   string
	Columns   []string
	Rows      [][]any
	ElapsedMs int64
}

// BlueprintHistory is one view↔blueprint association over time.
type BlueprintHistory struct {
	BlueprintID string `json:"blueprint_id"`
	ViewID      string `json:"view_id"`
	CreatedAt   string `json:"created_at"`
}
