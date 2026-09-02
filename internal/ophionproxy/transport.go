package ophionproxy

import (
	"net/http"
	"sync"
)

// statusProbe remembers the last HTTP status seen on the upstream endpoint.
//
// The MCP client returns a plain error when a handshake fails, which cannot
// distinguish "this workspace has no knowledge" (404) from "your token is
// wrong" (401) from "nothing is listening". Recording the status as it passes
// costs nothing and keeps the diagnosis truthful, instead of guessing from
// error text.
type statusProbe struct {
	base http.RoundTripper

	mu     sync.Mutex
	status int
}

func (p *statusProbe) RoundTrip(req *http.Request) (*http.Response, error) {
	resp, err := p.base.RoundTrip(req)
	p.mu.Lock()
	if err != nil {
		p.status = 0
	} else {
		p.status = resp.StatusCode
	}
	p.mu.Unlock()
	if err != nil {
		return nil, &transportError{err: err}
	}
	return resp, nil
}

func (p *statusProbe) lastStatus() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.status
}

// transportError marks a failure that never reached Ophion.
type transportError struct{ err error }

func (e *transportError) Error() string { return e.err.Error() }
func (e *transportError) Unwrap() error { return e.err }

// bearerTransport attaches the Ophion service token to every upstream
// request.
type bearerTransport struct {
	base  http.RoundTripper
	token string
}

func (t *bearerTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	// Never mutate the caller's request: the SDK may retry it.
	clone := req.Clone(req.Context())
	clone.Header.Set("Authorization", "Bearer "+t.token)
	return t.base.RoundTrip(clone)
}

func withBearer(client *http.Client, token string) *http.Client {
	base := client.Transport
	if base == nil {
		base = http.DefaultTransport
	}
	clone := *client
	clone.Transport = &bearerTransport{base: base, token: token}
	return &clone
}
