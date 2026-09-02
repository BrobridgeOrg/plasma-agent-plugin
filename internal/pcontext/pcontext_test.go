package pcontext

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func writeConfig(t *testing.T, dir, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, "config.env"), []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestLoadConfigParsesFile(t *testing.T) {
	dir := t.TempDir()
	writeConfig(t, dir, `
# a comment
PLASMA_URL=http://plasma.example:5001

PLASMA_USERNAME=admin
PLASMA_PASSWORD="s3cret"
OPHION_URL = http://ophion.example:5101
OPHION_SERVICE_TOKEN=tok
`)

	cfg, err := New(dir).LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if cfg.PlasmaURL != "http://plasma.example:5001" {
		t.Errorf("PlasmaURL = %q", cfg.PlasmaURL)
	}
	if cfg.PlasmaUsername != "admin" || cfg.PlasmaPassword != "s3cret" {
		t.Errorf("credentials = %q/%q", cfg.PlasmaUsername, cfg.PlasmaPassword)
	}
	if cfg.OphionURL != "http://ophion.example:5101" {
		t.Errorf("OphionURL = %q", cfg.OphionURL)
	}
	if cfg.OphionServiceToken != "tok" {
		t.Errorf("OphionServiceToken = %q", cfg.OphionServiceToken)
	}
}

func TestLoadConfigDefaultsProfileToAll(t *testing.T) {
	dir := t.TempDir()
	writeConfig(t, dir, "PLASMA_URL=http://p\n")

	cfg, err := New(dir).LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if cfg.OphionProfile != "all" {
		t.Errorf("OphionProfile = %q, want all", cfg.OphionProfile)
	}
}

func TestLoadConfigEnvOverridesFile(t *testing.T) {
	dir := t.TempDir()
	writeConfig(t, dir, "PLASMA_URL=http://from-file\n")
	t.Setenv("PLASMA_URL", "http://from-env")

	cfg, err := New(dir).LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if cfg.PlasmaURL != "http://from-env" {
		t.Errorf("PlasmaURL = %q, want the env value", cfg.PlasmaURL)
	}
}

func TestLoadConfigMissingFileIsNotAnError(t *testing.T) {
	t.Setenv("PLASMA_URL", "http://only-env")

	cfg, err := New(t.TempDir()).LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if cfg.PlasmaURL != "http://only-env" {
		t.Errorf("PlasmaURL = %q", cfg.PlasmaURL)
	}
}

func TestPlasmaAuthModeReportsMissingCredentials(t *testing.T) {
	cfg := Config{PlasmaURL: "http://p"}

	if _, err := cfg.PlasmaAuthMode(); err == nil {
		t.Fatal("want an error when neither token nor username/password is set")
	}
}

func TestPlasmaAuthModePrefersStaticToken(t *testing.T) {
	cfg := Config{PlasmaToken: "t", PlasmaUsername: "u", PlasmaPassword: "p"}

	mode, err := cfg.PlasmaAuthMode()
	if err != nil {
		t.Fatalf("PlasmaAuthMode: %v", err)
	}
	if mode != AuthStaticToken {
		t.Errorf("mode = %v, want static token", mode)
	}
}

func TestLoadStateMissingFileYieldsZeroState(t *testing.T) {
	st, err := New(t.TempDir()).LoadState()
	if err != nil {
		t.Fatalf("LoadState: %v", err)
	}
	if st.WorkspaceID != "" {
		t.Errorf("WorkspaceID = %q, want empty", st.WorkspaceID)
	}
}

func TestSaveStateRoundTrips(t *testing.T) {
	dir := t.TempDir()
	h := New(dir)
	want := State{
		WorkspaceID:   "ws-1",
		WorkspaceName: "his_database",
		Profile:       "all",
		AccessToken:   "at",
		RefreshToken:  "rt",
		ExpiresAt:     time.Date(2026, 9, 2, 10, 0, 0, 0, time.UTC),
	}

	if err := h.SaveState(want); err != nil {
		t.Fatalf("SaveState: %v", err)
	}
	got, err := h.LoadState()
	if err != nil {
		t.Fatalf("LoadState: %v", err)
	}
	if got != want {
		t.Errorf("round trip changed the state:\n got %+v\nwant %+v", got, want)
	}
}

func TestSaveStateWritesOwnerOnlyFile(t *testing.T) {
	dir := t.TempDir()
	if err := New(dir).SaveState(State{WorkspaceID: "ws-1"}); err != nil {
		t.Fatalf("SaveState: %v", err)
	}

	info, err := os.Stat(filepath.Join(dir, "state.json"))
	if err != nil {
		t.Fatal(err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Errorf("state.json mode = %o, want 600 (it holds a JWT)", perm)
	}
}

func TestSaveStateLeavesNoTempFileBehind(t *testing.T) {
	dir := t.TempDir()
	if err := New(dir).SaveState(State{WorkspaceID: "ws-1"}); err != nil {
		t.Fatalf("SaveState: %v", err)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Name() != "state.json" {
		var names []string
		for _, e := range entries {
			names = append(names, e.Name())
		}
		t.Errorf("directory holds %v, want only state.json", names)
	}
}

func TestSaveStateCreatesHomeDirectory(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "nested", "home")

	if err := New(dir).SaveState(State{WorkspaceID: "ws-1"}); err != nil {
		t.Fatalf("SaveState: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "state.json")); err != nil {
		t.Errorf("state.json not written: %v", err)
	}
}
