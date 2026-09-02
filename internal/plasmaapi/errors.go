package plasmaapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

var (
	// ErrNotFound is a 404: the view or workspace is not there (or not
	// visible to this user).
	ErrNotFound = errors.New("not found")
	// ErrForbidden is a 403, in practice "not a member of this workspace".
	ErrForbidden = errors.New("forbidden")
	// ErrNotReady is a 409 from the export URL: the view exists but has no
	// successful sync yet.
	ErrNotReady = errors.New("view is not ready")
	// ErrUnauthorized is a 401 that authentication could not repair.
	ErrUnauthorized = errors.New("unauthorized")
	// ErrNoWorkspace is a caller mistake caught before any request: an empty
	// workspace would address a different URL entirely.
	ErrNoWorkspace = errors.New("no workspace selected")
)

// APIError carries what the server actually said. The message is what the
// operator reads in the tool output, so the server's own wording is kept
// rather than replaced with a summary.
type APIError struct {
	Status int
	Method string
	Path   string
	Detail string
	kind   error
}

func (e *APIError) Error() string {
	if e.Detail == "" {
		return fmt.Sprintf("plasma %s %s: HTTP %d", e.Method, e.Path, e.Status)
	}
	return fmt.Sprintf("plasma %s %s: HTTP %d: %s", e.Method, e.Path, e.Status, e.Detail)
}

func (e *APIError) Unwrap() error { return e.kind }

func kindForStatus(status int) error {
	switch status {
	case 401:
		return ErrUnauthorized
	case 403:
		return ErrForbidden
	case 404:
		return ErrNotFound
	case 409:
		return ErrNotReady
	}
	return nil
}

// detailFromBody pulls the human-readable part out of Plasma's several error
// shapes: {"error":…}, {"message":…}, and the validation form that carries
// {"violations":[…]}.
func detailFromBody(body []byte) string {
	var payload struct {
		Error      string   `json:"error"`
		Message    string   `json:"message"`
		Violations []string `json:"violations"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return strings.TrimSpace(string(body))
	}
	parts := make([]string, 0, 3)
	if payload.Error != "" {
		parts = append(parts, payload.Error)
	}
	if payload.Message != "" && payload.Message != payload.Error {
		parts = append(parts, payload.Message)
	}
	if len(payload.Violations) > 0 {
		parts = append(parts, strings.Join(payload.Violations, "; "))
	}
	if len(parts) == 0 {
		return strings.TrimSpace(string(body))
	}
	return strings.Join(parts, ": ")
}
