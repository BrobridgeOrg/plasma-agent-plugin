package plasmamcp

import (
	"context"
	"errors"
	"testing"

	"github.com/BrobridgeOrg/plasma-plugin/internal/pcontext"
	"github.com/BrobridgeOrg/plasma-plugin/internal/plasmaapi"
)

func (f *fakePlasma) ListPGConnections(_ context.Context, workspace string, _ plasmaapi.ViewQuery) (plasmaapi.PGConnectionList, error) {
	f.calls++
	f.queriedWorkspace = workspace
	return plasmaapi.PGConnectionList{Connections: []plasmaapi.PGConnection{f.connection}}, f.err
}
func (f *fakePlasma) GetPGConnection(_ context.Context, workspace, _ string) (plasmaapi.PGConnection, error) {
	f.calls++
	f.queriedWorkspace = workspace
	return f.connection, f.err
}
func (f *fakePlasma) CreatePGBlueprint(_ context.Context, workspace string, request plasmaapi.CreatePGBlueprintRequest) (plasmaapi.Blueprint, error) {
	f.calls++
	f.queriedWorkspace = workspace
	f.createdBlueprint = &request
	return f.blueprint, f.err
}
func (f *fakePlasma) ListBlueprints(_ context.Context, workspace string, _ plasmaapi.ViewQuery) (plasmaapi.BlueprintList, error) {
	f.calls++
	f.queriedWorkspace = workspace
	return plasmaapi.BlueprintList{Blueprints: []plasmaapi.Blueprint{f.blueprint}}, f.err
}
func (f *fakePlasma) GetBlueprint(_ context.Context, workspace, _ string) (plasmaapi.Blueprint, error) {
	f.calls++
	f.queriedWorkspace = workspace
	return f.blueprint, f.err
}
func (f *fakePlasma) SpawnBlueprintJob(_ context.Context, workspace, id string) (string, error) {
	f.calls++
	f.queriedWorkspace = workspace
	f.spawnedBlueprint = id
	return "Job spawned successfully", f.err
}
func (f *fakePlasma) ListBlueprintJobs(_ context.Context, workspace, _ string, _ plasmaapi.ViewQuery) (plasmaapi.JobList, error) {
	f.calls++
	f.queriedWorkspace = workspace
	return plasmaapi.JobList{Jobs: []plasmaapi.Job{f.job}}, f.err
}
func (f *fakePlasma) GetJob(_ context.Context, workspace, _ string) (plasmaapi.Job, error) {
	f.calls++
	f.queriedWorkspace = workspace
	return f.job, f.err
}

func pgFake() *fakePlasma {
	return &fakePlasma{
		view:       plasmaapi.View{ID: "v-1", Type: "view", Path: "ws.views.report", ViewSQL: "SELECT 1"},
		connection: plasmaapi.PGConnection{ID: "dbc-1", WorkspaceID: "ws-1", DBType: "postgresql", DBName: "reports", Schema: "bi", Status: "ready"},
		blueprint: plasmaapi.Blueprint{ID: "bp-1", JobType: "query", LastJobID: "old-job", Configuration: plasmaapi.BlueprintConfiguration{
			SourceViewID: "v-1", Target: plasmaapi.BlueprintTarget{Method: "static", DBCID: "dbc-1", Type: "postgresql", Namespace: "bi", Table: "daily"},
			Properties: plasmaapi.BlueprintProperties{WriteMode: "append"},
		}},
		job: plasmaapi.Job{ID: "job-1", BlueprintID: "bp-1", Status: "running"},
	}
}

func pgCreateArgs() map[string]any {
	return map[string]any{"name": "daily", "view_id": "v-1", "dbc_id": "dbc-1", "database": "reports", "schema": "bi", "table": "daily", "write_mode": "append", "force_create_table": false}
}

func TestCreatePGBlueprintBindsViewAndSelectedDBCWithoutSpawning(t *testing.T) {
	for _, mode := range []string{"append", "overwrite", "truncate"} {
		t.Run(mode, func(t *testing.T) {
			plasma := pgFake()
			session := newSession(t, plasma, selectedState())
			args := pgCreateArgs()
			args["write_mode"] = mode
			result := session.call("create_pg_blueprint", args)
			if result.IsError {
				t.Fatal(text(t, result))
			}
			request := plasma.createdBlueprint
			if request == nil {
				t.Fatal("no blueprint created")
			}
			config := request.Configuration
			if config.SourceViewID != "v-1" || config.Target.DBCID != "dbc-1" || config.Target.Method != "static" || config.Target.Table != "daily" || config.Properties.WriteMode != mode || config.Properties.ForceCreateTable {
				t.Fatalf("incorrect configuration: %+v", config)
			}
			if config.Target.Host != "" || config.Target.Namespace != "" || plasma.spawnedBlueprint != "" || plasma.queriedWorkspace != "ws-1" {
				t.Fatal("creation must use DBC coordinates and must not spawn")
			}
		})
	}
}

func TestCreatePGBlueprintRefusesInvalidSelection(t *testing.T) {
	cases := map[string]func(*fakePlasma, map[string]any){
		"missing write mode":    func(_ *fakePlasma, args map[string]any) { delete(args, "write_mode") },
		"missing create choice": func(_ *fakePlasma, args map[string]any) { delete(args, "force_create_table") },
		"unsupported upsert":    func(_ *fakePlasma, args map[string]any) { args["write_mode"] = "upsert" },
		"database mismatch":     func(_ *fakePlasma, args map[string]any) { args["database"] = "other" },
		"schema mismatch":       func(_ *fakePlasma, args map[string]any) { args["schema"] = "public" },
		"empty table":           func(_ *fakePlasma, args map[string]any) { args["table"] = " " },
		"foreign connection":    func(fake *fakePlasma, _ map[string]any) { fake.connection.WorkspaceID = "ws-2" },
		"not PG":                func(fake *fakePlasma, _ map[string]any) { fake.connection.DBType = "mysql" },
		"not ready":             func(fake *fakePlasma, _ map[string]any) { fake.connection.Status = "preparing" },
		"mview":                 func(fake *fakePlasma, _ map[string]any) { fake.view.Type = "materialized_view" },
		"no path":               func(fake *fakePlasma, _ map[string]any) { fake.view.Path = "" },
		"invalid SQL":           func(fake *fakePlasma, _ map[string]any) { fake.view.ViewSQL = "DROP TABLE t" },
		"upstream error":        func(fake *fakePlasma, _ map[string]any) { fake.err = errors.New("unavailable") },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			plasma := pgFake()
			args := pgCreateArgs()
			mutate(plasma, args)
			result := newSession(t, plasma, selectedState()).call("create_pg_blueprint", args)
			if !result.IsError || plasma.createdBlueprint != nil || plasma.spawnedBlueprint != "" {
				t.Fatalf("must refuse: %s", text(t, result))
			}
		})
	}
}

func TestSpawnPGBlueprintPreservesPriorJobForPolling(t *testing.T) {
	plasma := pgFake()
	result := newSession(t, plasma, selectedState()).call("spawn_blueprint_job", map[string]any{"blueprint_id": "bp-1"})
	if result.IsError {
		t.Fatal(text(t, result))
	}
	var out spawnOutput
	structured(t, result, &out)
	if out.PreviousJobID != "old-job" || out.Workspace != "ws-1" || plasma.spawnedBlueprint != "bp-1" {
		t.Fatalf("unexpected spawn: %+v", out)
	}
}

func TestSpawnRefusesNonPGOrDisabledBlueprint(t *testing.T) {
	for _, mutate := range []func(*fakePlasma){
		func(fake *fakePlasma) { fake.blueprint.IsDisabled = true },
		func(fake *fakePlasma) { fake.blueprint.Configuration.Target.Type = "mysql" },
		func(fake *fakePlasma) { fake.blueprint.Configuration.Target.Method = "custom" },
		func(fake *fakePlasma) { fake.blueprint.Configuration.Target.ViewID = "mview" },
		func(fake *fakePlasma) { fake.blueprint.Configuration.Properties.WriteMode = "" },
		func(fake *fakePlasma) { fake.view.Type = "materialized_view" },
		func(fake *fakePlasma) { fake.connection.Schema = "changed" },
		func(fake *fakePlasma) { fake.connection.Host = "different-host" },
		func(fake *fakePlasma) { fake.connection.Status = "failed" },
	} {
		plasma := pgFake()
		mutate(plasma)
		result := newSession(t, plasma, selectedState()).call("spawn_blueprint_job", map[string]any{"blueprint_id": "bp-1"})
		if !result.IsError || plasma.spawnedBlueprint != "" {
			t.Fatal("invalid blueprint spawned")
		}
	}
}

func TestBlueprintToolsRequireWorkspaceAndPreserveReadResults(t *testing.T) {
	cases := map[string]map[string]any{
		"list_pg_connections": {}, "get_pg_connection": {"dbc_id": "dbc-1"},
		"create_pg_blueprint": pgCreateArgs(), "list_blueprints": {},
		"get_blueprint": {"blueprint_id": "bp-1"}, "spawn_blueprint_job": {"blueprint_id": "bp-1"},
		"list_blueprint_jobs": {"blueprint_id": "bp-1"}, "get_job": {"job_id": "job-1"},
	}
	for name, args := range cases {
		t.Run(name, func(t *testing.T) {
			plasma := pgFake()
			result := newSession(t, plasma, pcontext.State{}).call(name, args)
			if !result.IsError || plasma.calls != 0 {
				t.Fatal("workspace required before API calls")
			}
			result = newSession(t, plasma, selectedState()).call(name, args)
			if result.IsError {
				t.Fatal(text(t, result))
			}
			var out map[string]any
			structured(t, result, &out)
			if out["workspace_id"] != "ws-1" {
				t.Fatalf("workspace missing: %+v", out)
			}
		})
	}
}
