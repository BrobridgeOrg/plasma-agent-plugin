package plasmaapi

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

func TestBlueprintAPIContracts(t *testing.T) {
	cases := []struct {
		name, method, path, response string
		query                        map[string]string
		call                         func(*Client) (any, error)
	}{
		{"connections", "GET", "/apis/v1/w/ws-1/dbcs", `{"connections":[{"id":"dbc-1","db_type":"postgresql","db_name":"reports","schema":"bi","password":"hidden"}],"total":3,"page":2,"page_size":1,"total_pages":3}`,
			map[string]string{"db_type": "postgresql", "keywords": "reports", "page": "2", "page_size": "1"},
			func(client *Client) (any, error) {
				return client.ListPGConnections(context.Background(), "ws-1", ViewQuery{Keywords: "reports", Page: 2, PageSize: 1})
			}},
		{"connection", "GET", "/apis/v1/w/ws-1/dbc/dbc-1", `{"connection":{"id":"dbc-1","db_type":"postgresql","db_name":"reports","schema":"bi","password":"hidden"}}`, nil,
			func(client *Client) (any, error) {
				return client.GetPGConnection(context.Background(), "ws-1", "dbc-1")
			}},
		{"blueprints", "GET", "/apis/v1/w/ws-1/blueprints", `{"blueprints":[{"id":"bp-1","job_type":"query","job_configurations":{"source_view_id":"v-1","target":{"password":"hidden"}}}],"total":1}`, map[string]string{"class": "user", "job_type": "query", "keywords": "daily"},
			func(client *Client) (any, error) {
				return client.ListBlueprints(context.Background(), "ws-1", ViewQuery{Keywords: "daily"})
			}},
		{"blueprint", "GET", "/apis/v1/w/ws-1/blueprint/bp-1", `{"blueprint":{"id":"bp-1","last_job_id":"job-1","current_job_status":"running","job_configurations":{"source_view_id":"v-1","target":{"method":"static","dbc_id":"dbc-1","type":"postgresql","namespace":"bi","table":"daily","password":"hidden"},"properties":{"write_mode":"truncate","force_create_table":true}}}}`, nil,
			func(client *Client) (any, error) { return client.GetBlueprint(context.Background(), "ws-1", "bp-1") }},
		{"spawn", "POST", "/apis/v1/w/ws-1/blueprint/bp-1/spawn", `{"message":"Job spawned successfully"}`, nil,
			func(client *Client) (any, error) {
				return client.SpawnBlueprintJob(context.Background(), "ws-1", "bp-1")
			}},
		{"jobs", "GET", "/apis/v1/w/ws-1/jobs", `{"jobs":[{"id":"job-1","blueprint_id":"bp-1","status":"running"}],"total":1}`, map[string]string{"blueprint_id": "bp-1", "order_by": "created_at", "order": "-1", "page": "2", "page_size": "5"},
			func(client *Client) (any, error) {
				return client.ListBlueprintJobs(context.Background(), "ws-1", "bp-1", ViewQuery{Page: 2, PageSize: 5})
			}},
		{"job", "GET", "/apis/v1/w/ws-1/job/job-1", `{"job":{"id":"job-1","blueprint_id":"bp-1","status":"failed","details":"target table missing","configurations":{"target":{"password":"hidden"}}}}`, nil,
			func(client *Client) (any, error) { return client.GetJob(context.Background(), "ws-1", "job-1") }},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			harness := newHarness(t, tokenConfig())
			harness.handler = func(writer http.ResponseWriter, request *http.Request) {
				if request.Method != test.method || request.URL.Path != test.path || request.Header.Get("Authorization") != "Bearer static-token" {
					t.Errorf("unexpected request: %s %s", request.Method, request.URL)
				}
				for key, value := range test.query {
					if request.URL.Query().Get(key) != value {
						t.Errorf("query %s: %s", key, request.URL.RawQuery)
					}
				}
				writer.Header().Set("Content-Type", "application/json")
				_, _ = writer.Write([]byte(test.response))
			}
			result, err := test.call(harness.client)
			if err != nil {
				t.Fatal(err)
			}
			encoded, err := json.Marshal(result)
			if err != nil {
				t.Fatal(err)
			}
			if strings.Contains(string(encoded), "hidden") || strings.Contains(string(encoded), "password") {
				t.Fatalf("secret leaked: %s", encoded)
			}
			switch value := result.(type) {
			case PGConnectionList:
				if len(value.Connections) != 1 || value.Connections[0].Schema != "bi" || value.TotalPages != 3 {
					t.Fatalf("bad connections: %+v", value)
				}
			case PGConnection:
				if value.ID != "dbc-1" || value.DBName != "reports" {
					t.Fatalf("bad connection: %+v", value)
				}
			case BlueprintList:
				if len(value.Blueprints) != 1 || value.Blueprints[0].ID != "bp-1" {
					t.Fatalf("bad blueprints: %+v", value)
				}
			case Blueprint:
				if value.CurrentJobStatus != "running" || value.Configuration.Target.Namespace != "bi" || value.Configuration.Properties.WriteMode != "truncate" {
					t.Fatalf("bad blueprint: %+v", value)
				}
			case JobList:
				if len(value.Jobs) != 1 || value.Jobs[0].Status != "running" {
					t.Fatalf("bad jobs: %+v", value)
				}
			case Job:
				if value.Status != "failed" || value.Details != "target table missing" {
					t.Fatalf("bad job: %+v", value)
				}
			case string:
				if value != "Job spawned successfully" {
					t.Fatalf("bad spawn message: %q", value)
				}
			}
		})
	}
}

func TestCreatePGBlueprintUsesBackendConfigurationContract(t *testing.T) {
	harness := newHarness(t, tokenConfig())
	harness.handler = func(writer http.ResponseWriter, request *http.Request) {
		writeJSON(writer, http.StatusCreated, map[string]any{"blueprint": map[string]any{"id": "bp-new", "job_type": "query"}})
	}
	result, err := harness.client.CreatePGBlueprint(context.Background(), "ws-1", CreatePGBlueprintRequest{
		Name: "daily", Configuration: BlueprintConfiguration{
			SourceViewID: "v-1", Target: BlueprintTarget{Method: "static", DBCID: "dbc-1", Table: "daily"},
			Properties: BlueprintProperties{WriteMode: "append", ForceCreateTable: false},
		},
	})
	if err != nil || result.ID != "bp-new" {
		t.Fatalf("create: %+v, %v", result, err)
	}
	if len(harness.reqs) != 1 {
		t.Fatalf("creation triggered more requests: %+v", harness.reqs)
	}
	request := harness.reqs[0]
	if request.method != "POST" || request.path != "/apis/v1/w/ws-1/blueprint" {
		t.Fatalf("bad endpoint: %+v", request)
	}
	var body map[string]any
	if err := json.Unmarshal([]byte(request.body), &body); err != nil {
		t.Fatal(err)
	}
	if body["class"] != "user" || body["job_type"] != "query" || body["allow_concurrent_jobs"] != false || body["is_disabled"] != false {
		t.Fatalf("bad blueprint: %s", request.body)
	}
	config := body["job_configurations"].(map[string]any)
	properties := config["properties"].(map[string]any)
	target := config["target"].(map[string]any)
	if config["source_view_id"] != "v-1" || config["sql"] != nil || properties["write_mode"] != "append" || properties["force_create_table"] != false {
		t.Fatalf("bad query configuration: %s", request.body)
	}
	if target["dbc_id"] != "dbc-1" || target["method"] != "static" || target["table"] != "daily" || len(target) != 3 {
		t.Fatalf("target must use stored DBC: %s", request.body)
	}
}

func TestBlueprintAPIErrorsAndMissingScope(t *testing.T) {
	harness := newHarness(t, tokenConfig())
	harness.handler = func(writer http.ResponseWriter, request *http.Request) {
		writeJSON(writer, http.StatusConflict, map[string]string{"error": "concurrent job exists"})
	}
	if _, err := harness.client.SpawnBlueprintJob(context.Background(), "ws-1", "bp-1"); err == nil || !strings.Contains(err.Error(), "concurrent job") {
		t.Fatalf("conflict lost: %v", err)
	}
	if len(harness.reqs) != 1 {
		t.Fatal("spawn conflict must not retry")
	}
	for _, scope := range []struct{ workspace, id string }{{"", "bp-1"}, {"ws-1", " "}} {
		if _, err := harness.client.SpawnBlueprintJob(context.Background(), scope.workspace, scope.id); err == nil {
			t.Fatal("missing scope accepted")
		}
	}
	if len(harness.reqs) != 1 {
		t.Fatal("missing scope reached server")
	}
}
