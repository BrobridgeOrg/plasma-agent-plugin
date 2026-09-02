// Package pcontext owns the two files both MCP servers share: the operator's
// configuration and the mutable "which workspace am I on" state.
//
// Neither file lives inside the plugin directory. A plugin installed from a
// marketplace is a cache directory that gets replaced wholesale on update, so
// anything kept there is lost the first time the plugin is upgraded.
package pcontext

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	configFile = "config.env"
	stateFile  = "state.json"

	// DefaultProfile is the Ophion tool profile used when none is configured.
	// It is deliberately not switchable at runtime: changing the profile
	// changes the upstream tool list, and a client is not guaranteed to
	// refresh on notifications/tools/list_changed.
	DefaultProfile = "all"
)

// AuthMode is how the Plasma client obtains its bearer.
type AuthMode int

const (
	// AuthStaticToken uses PLASMA_TOKEN verbatim: no login, no refresh.
	AuthStaticToken AuthMode = iota + 1
	// AuthPassword logs in with username/password and maintains the JWT.
	AuthPassword
)

// ErrNoPlasmaCredentials is returned when neither auth mode is configured.
var ErrNoPlasmaCredentials = errors.New(
	"no Plasma credentials: set PLASMA_TOKEN, or PLASMA_USERNAME and PLASMA_PASSWORD")

// Config is the operator-supplied configuration. Environment variables win
// over the file so a single call can be pointed at another deployment without
// editing anything.
type Config struct {
	PlasmaURL      string
	PlasmaToken    string
	PlasmaUsername string
	PlasmaPassword string

	OphionURL          string
	OphionServiceToken string
	OphionProfile      string
}

// PlasmaAuthMode reports how to authenticate, or why it cannot.
func (c Config) PlasmaAuthMode() (AuthMode, error) {
	if c.PlasmaToken != "" {
		return AuthStaticToken, nil
	}
	if c.PlasmaUsername != "" && c.PlasmaPassword != "" {
		return AuthPassword, nil
	}
	return 0, ErrNoPlasmaCredentials
}

// State is what changes while the session runs.
type State struct {
	WorkspaceID   string    `json:"workspace_id"`
	WorkspaceName string    `json:"workspace_name"`
	Profile       string    `json:"profile"`
	AccessToken   string    `json:"access_token"`
	RefreshToken  string    `json:"refresh_token"`
	ExpiresAt     time.Time `json:"expires_at"`
}

// Home is a plugin home directory (~/.plasma-plugin by default).
type Home struct {
	dir string
}

// New addresses the given directory. The directory need not exist yet.
func New(dir string) *Home { return &Home{dir: dir} }

// DefaultDir is $PLASMA_PLUGIN_HOME, else ~/.plasma-plugin.
func DefaultDir() string {
	if dir := os.Getenv("PLASMA_PLUGIN_HOME"); dir != "" {
		return dir
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ".plasma-plugin"
	}
	return filepath.Join(home, ".plasma-plugin")
}

// Dir is the directory this Home addresses.
func (h *Home) Dir() string { return h.dir }

// StatePath is where LoadState/SaveState read and write.
func (h *Home) StatePath() string { return filepath.Join(h.dir, stateFile) }

// ConfigPath is the config file this Home reads.
func (h *Home) ConfigPath() string { return filepath.Join(h.dir, configFile) }

// LoadConfig reads config.env and applies environment overrides. A missing
// file is not an error: an all-environment setup is legitimate.
func (h *Home) LoadConfig() (Config, error) {
	values, err := readEnvFile(h.ConfigPath())
	if err != nil {
		return Config{}, err
	}
	get := func(key string) string {
		if v := os.Getenv(key); v != "" {
			return v
		}
		return values[key]
	}
	cfg := Config{
		PlasmaURL:          get("PLASMA_URL"),
		PlasmaToken:        get("PLASMA_TOKEN"),
		PlasmaUsername:     get("PLASMA_USERNAME"),
		PlasmaPassword:     get("PLASMA_PASSWORD"),
		OphionURL:          get("OPHION_URL"),
		OphionServiceToken: get("OPHION_SERVICE_TOKEN"),
		OphionProfile:      get("OPHION_PROFILE"),
	}
	if cfg.OphionProfile == "" {
		cfg.OphionProfile = DefaultProfile
	}
	return cfg, nil
}

// LoadState reads state.json. A missing file yields the zero State: nothing
// has been selected yet, which is a state, not a failure.
func (h *Home) LoadState() (State, error) {
	data, err := os.ReadFile(h.StatePath())
	if errors.Is(err, os.ErrNotExist) {
		return State{}, nil
	}
	if err != nil {
		return State{}, err
	}
	var st State
	if err := json.Unmarshal(data, &st); err != nil {
		return State{}, fmt.Errorf("%s is not readable JSON: %w", h.StatePath(), err)
	}
	return st, nil
}

// SaveState writes state.json atomically. Both servers read this file on
// every call, so a half-written file would be read as a torn state.
func (h *Home) SaveState(st State) error {
	if err := os.MkdirAll(h.dir, 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(h.dir, ".state-*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)

	if err := tmp.Chmod(0o600); err != nil {
		tmp.Close()
		return err
	}
	if _, err := tmp.Write(append(data, '\n')); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpName, h.StatePath())
}

// readEnvFile parses KEY=VALUE lines, skipping blanks and # comments.
func readEnvFile(path string) (map[string]string, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return map[string]string{}, nil
	}
	if err != nil {
		return nil, err
	}
	values := map[string]string{}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		values[strings.TrimSpace(key)] = unquote(strings.TrimSpace(value))
	}
	return values, nil
}

func unquote(v string) string {
	if len(v) >= 2 && (v[0] == '"' || v[0] == '\'') && v[len(v)-1] == v[0] {
		return v[1 : len(v)-1]
	}
	return v
}
