package railwaybackup

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type graphQLRequest struct {
	Query     string         `json:"query"`
	Variables map[string]any `json:"variables"`
}

func newGraphQLServer(t *testing.T, handle func(*http.Request, graphQLRequest) (int, any)) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var request graphQLRequest
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Fatalf("decode GraphQL request: %v", err)
		}
		status, response := handle(r, request)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		if err := json.NewEncoder(w).Encode(response); err != nil {
			t.Fatalf("encode GraphQL response: %v", err)
		}
	}))
}

func TestClientUsesProjectTokenAndReturnsEnvironmentIdentity(t *testing.T) {
	server := newGraphQLServer(t, func(r *http.Request, request graphQLRequest) (int, any) {
		if got := r.Header.Get("Project-Access-Token"); got != "secret-token" {
			t.Fatalf("Project-Access-Token = %q", got)
		}
		if !strings.Contains(request.Query, "projectToken") {
			t.Fatalf("query = %q", request.Query)
		}
		return http.StatusOK, map[string]any{"data": map[string]any{
			"projectToken": map[string]any{
				"projectId":     "project-1",
				"environmentId": "environment-1",
			},
		}}
	})
	defer server.Close()

	client := newTestClient(server.URL, "secret-token", server.Client())
	identity, err := client.Identity(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if identity.ProjectID != "project-1" || identity.EnvironmentID != "environment-1" {
		t.Fatalf("identity = %#v", identity)
	}
}

func TestClientReturnsVolumeInstanceIdentity(t *testing.T) {
	server := newGraphQLServer(t, func(_ *http.Request, request graphQLRequest) (int, any) {
		if !strings.Contains(request.Query, "volumeInstance") ||
			!strings.Contains(request.Query, "environmentId") ||
			!strings.Contains(request.Query, "serviceId") ||
			!strings.Contains(request.Query, "projectId") {
			t.Fatalf("query = %q", request.Query)
		}
		if request.Variables["id"] != "volume-instance-1" {
			t.Fatalf("variables = %#v", request.Variables)
		}
		return http.StatusOK, map[string]any{"data": map[string]any{
			"volumeInstance": map[string]any{
				"id":            "volume-instance-1",
				"environmentId": "environment-1",
				"serviceId":     "postgres-service-1",
				"volume": map[string]any{
					"projectId": "project-1",
				},
			},
		}}
	})
	defer server.Close()

	client := newTestClient(server.URL, "token", server.Client())
	identity, err := client.VolumeInstanceIdentity(context.Background(), "volume-instance-1")
	if err != nil {
		t.Fatal(err)
	}
	want := VolumeInstanceIdentity{
		ID:            "volume-instance-1",
		ProjectID:     "project-1",
		EnvironmentID: "environment-1",
		ServiceID:     "postgres-service-1",
	}
	if identity != want {
		t.Fatalf("volume identity = %#v, want %#v", identity, want)
	}
}

func TestClientRejectsIncompleteVolumeInstanceIdentity(t *testing.T) {
	tests := []struct {
		name     string
		response map[string]any
		missing  string
	}{
		{name: "id", missing: "volume instance ID", response: map[string]any{
			"environmentId": "environment-1", "serviceId": "postgres-service-1", "volume": map[string]any{"projectId": "project-1"},
		}},
		{name: "project", missing: "project ID", response: map[string]any{
			"id": "volume-instance-1", "environmentId": "environment-1", "serviceId": "postgres-service-1", "volume": map[string]any{},
		}},
		{name: "environment", missing: "environment ID", response: map[string]any{
			"id": "volume-instance-1", "serviceId": "postgres-service-1", "volume": map[string]any{"projectId": "project-1"},
		}},
		{name: "service", missing: "service ID", response: map[string]any{
			"id": "volume-instance-1", "environmentId": "environment-1", "volume": map[string]any{"projectId": "project-1"},
		}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			server := newGraphQLServer(t, func(_ *http.Request, _ graphQLRequest) (int, any) {
				return http.StatusOK, map[string]any{"data": map[string]any{"volumeInstance": test.response}}
			})
			defer server.Close()

			client := newTestClient(server.URL, "token", server.Client())
			_, err := client.VolumeInstanceIdentity(context.Background(), "volume-instance-1")
			if err == nil || !strings.Contains(err.Error(), test.missing) {
				t.Fatalf("VolumeInstanceIdentity error = %v, want missing %s", err, test.missing)
			}
		})
	}
}

func TestClientVerifiesDatabaseURLBindingToPostgresService(t *testing.T) {
	const databaseURL = "postgresql://postgres:secret@postgres.railway.internal:5432/railway"
	server := newGraphQLServer(t, func(_ *http.Request, request graphQLRequest) (int, any) {
		for _, want := range []string{"project(id:", "services", "unrendered: variables", "variablesForServiceDeployment"} {
			if !strings.Contains(request.Query, want) {
				t.Fatalf("binding query missing %q: %s", want, request.Query)
			}
		}
		for key, want := range map[string]any{
			"projectId": "project-1", "environmentId": "environment-1", "applicationServiceId": "api-service-1",
		} {
			if request.Variables[key] != want {
				t.Fatalf("binding variable %s = %#v, want %#v", key, request.Variables[key], want)
			}
		}
		return http.StatusOK, map[string]any{"data": map[string]any{
			"project": map[string]any{
				"id": "project-1",
				"services": map[string]any{"edges": []any{
					map[string]any{"node": map[string]any{"id": "api-service-1", "name": "api"}},
					map[string]any{"node": map[string]any{"id": "postgres-service-1", "name": "Postgres"}},
				}},
			},
			"unrendered": map[string]any{"DATABASE_URL": `${{Postgres.DATABASE_URL}}`},
			"rendered":   map[string]any{"DATABASE_URL": databaseURL},
		}}
	})
	defer server.Close()

	client := newTestClient(server.URL, "token", server.Client())
	err := client.VerifyDatabaseBinding(context.Background(), DatabaseBindingRequest{
		ProjectID: "project-1", EnvironmentID: "environment-1",
		ApplicationServiceID: "api-service-1", PostgresServiceID: "postgres-service-1",
		RuntimeDatabaseURL: databaseURL,
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestClientRejectsMiswiredDatabaseURLWithoutLeakingURLs(t *testing.T) {
	const runtimeURL = "postgresql://postgres:runtime-secret@wrong.railway.internal:5432/railway"
	const renderedURL = "postgresql://postgres:rendered-secret@postgres.railway.internal:5432/railway"
	server := newGraphQLServer(t, func(_ *http.Request, _ graphQLRequest) (int, any) {
		return http.StatusOK, map[string]any{"data": map[string]any{
			"project": map[string]any{
				"id": "project-1",
				"services": map[string]any{"edges": []any{
					map[string]any{"node": map[string]any{"id": "api-service-1", "name": "api"}},
					map[string]any{"node": map[string]any{"id": "postgres-service-1", "name": "Postgres"}},
				}},
			},
			"unrendered": map[string]any{"DATABASE_URL": `${{Postgres.DATABASE_URL}}`},
			"rendered":   map[string]any{"DATABASE_URL": renderedURL},
		}}
	})
	defer server.Close()

	client := newTestClient(server.URL, "token", server.Client())
	err := client.VerifyDatabaseBinding(context.Background(), DatabaseBindingRequest{
		ProjectID: "project-1", EnvironmentID: "environment-1",
		ApplicationServiceID: "api-service-1", PostgresServiceID: "postgres-service-1",
		RuntimeDatabaseURL: runtimeURL,
	})
	if err == nil || !strings.Contains(err.Error(), "rendered DATABASE_URL does not match runtime DATABASE_URL") {
		t.Fatalf("VerifyDatabaseBinding error = %v", err)
	}
	if strings.Contains(err.Error(), runtimeURL) || strings.Contains(err.Error(), renderedURL) ||
		strings.Contains(err.Error(), "runtime-secret") || strings.Contains(err.Error(), "rendered-secret") {
		t.Fatalf("binding error leaked a database URL: %v", err)
	}
}

func TestClientRejectsDatabaseURLWithoutExactPostgresServiceReference(t *testing.T) {
	const databaseURL = "postgresql://postgres:secret@postgres.railway.internal:5432/railway"
	tests := []struct {
		name       string
		unrendered string
	}{
		{name: "literal URL", unrendered: databaseURL},
		{name: "wrong service", unrendered: `${{OtherPostgres.DATABASE_URL}}`},
		{name: "sealed value", unrendered: "********"},
		{name: "missing value", unrendered: ""},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			server := newGraphQLServer(t, func(_ *http.Request, _ graphQLRequest) (int, any) {
				return http.StatusOK, map[string]any{"data": map[string]any{
					"project": map[string]any{
						"id": "project-1",
						"services": map[string]any{"edges": []any{
							map[string]any{"node": map[string]any{"id": "api-service-1", "name": "api"}},
							map[string]any{"node": map[string]any{"id": "postgres-service-1", "name": "Postgres"}},
						}},
					},
					"unrendered": map[string]any{"DATABASE_URL": test.unrendered},
					"rendered":   map[string]any{"DATABASE_URL": databaseURL},
				}}
			})
			defer server.Close()

			client := newTestClient(server.URL, "token", server.Client())
			err := client.VerifyDatabaseBinding(context.Background(), DatabaseBindingRequest{
				ProjectID: "project-1", EnvironmentID: "environment-1",
				ApplicationServiceID: "api-service-1", PostgresServiceID: "postgres-service-1",
				RuntimeDatabaseURL: databaseURL,
			})
			if err == nil || !strings.Contains(err.Error(), "does not exactly reference the verified Postgres service") {
				t.Fatalf("VerifyDatabaseBinding error = %v", err)
			}
			if strings.Contains(err.Error(), databaseURL) || strings.Contains(err.Error(), "secret") {
				t.Fatalf("binding error leaked a database URL: %v", err)
			}
		})
	}
}

func TestClientRejectsAmbiguousPostgresServiceName(t *testing.T) {
	const databaseURL = "postgresql://postgres:secret@postgres.railway.internal:5432/railway"
	server := newGraphQLServer(t, func(_ *http.Request, _ graphQLRequest) (int, any) {
		return http.StatusOK, map[string]any{"data": map[string]any{
			"project": map[string]any{
				"id": "project-1",
				"services": map[string]any{"edges": []any{
					map[string]any{"node": map[string]any{"id": "api-service-1", "name": "api"}},
					map[string]any{"node": map[string]any{"id": "postgres-service-1", "name": "Postgres"}},
					map[string]any{"node": map[string]any{"id": "other-postgres-service", "name": "Postgres"}},
				}},
			},
			"unrendered": map[string]any{"DATABASE_URL": `${{Postgres.DATABASE_URL}}`},
			"rendered":   map[string]any{"DATABASE_URL": databaseURL},
		}}
	})
	defer server.Close()

	client := newTestClient(server.URL, "token", server.Client())
	err := client.VerifyDatabaseBinding(context.Background(), DatabaseBindingRequest{
		ProjectID: "project-1", EnvironmentID: "environment-1",
		ApplicationServiceID: "api-service-1", PostgresServiceID: "postgres-service-1",
		RuntimeDatabaseURL: databaseURL,
	})
	if err == nil || !strings.Contains(err.Error(), "Postgres service name is ambiguous") {
		t.Fatalf("VerifyDatabaseBinding error = %v", err)
	}
}

func TestClientCreateReturnsWorkflowNotBackupID(t *testing.T) {
	server := newGraphQLServer(t, func(_ *http.Request, request graphQLRequest) (int, any) {
		if !strings.Contains(request.Query, "volumeInstanceBackupCreate") {
			t.Fatalf("query = %q", request.Query)
		}
		if request.Variables["volumeInstanceId"] != "volume-instance-1" || request.Variables["name"] != "backup-name" {
			t.Fatalf("variables = %#v", request.Variables)
		}
		return http.StatusOK, map[string]any{"data": map[string]any{
			"volumeInstanceBackupCreate": map[string]any{
				"workflowId": "createVolumeInstanceBackup/volume-instance-1",
			},
		}}
	})
	defer server.Close()

	client := newTestClient(server.URL, "token", server.Client())
	result, err := client.Create(context.Background(), "volume-instance-1", "backup-name")
	if err != nil {
		t.Fatal(err)
	}
	if result.WorkflowID != "createVolumeInstanceBackup/volume-instance-1" {
		t.Fatalf("workflow ID = %q", result.WorkflowID)
	}
}

func TestClientListPreservesBackupReadinessFields(t *testing.T) {
	server := newGraphQLServer(t, func(_ *http.Request, request graphQLRequest) (int, any) {
		if !strings.Contains(request.Query, "volumeInstanceBackupList") {
			t.Fatalf("query = %q", request.Query)
		}
		return http.StatusOK, map[string]any{"data": map[string]any{
			"volumeInstanceBackupList": []any{map[string]any{
				"id":           "backup-1",
				"name":         "before-migration",
				"createdAt":    "2026-07-26T16:20:53.874Z",
				"externalId":   "vs_123",
				"referencedMB": 859,
			}},
		}}
	})
	defer server.Close()

	client := newTestClient(server.URL, "token", server.Client())
	backups, err := client.List(context.Background(), "volume-instance-1")
	if err != nil {
		t.Fatal(err)
	}
	if len(backups) != 1 {
		t.Fatalf("backup count = %d", len(backups))
	}
	got := backups[0]
	if got.ID != "backup-1" || got.Name != "before-migration" || got.CreatedAt != "2026-07-26T16:20:53.874Z" || got.ExternalID != "vs_123" {
		t.Fatalf("backup = %#v", got)
	}
	if got.ReferencedMB == nil || *got.ReferencedMB != 859 {
		t.Fatalf("referenced MB = %#v", got.ReferencedMB)
	}
}

func TestClientLockRequiresTrue(t *testing.T) {
	server := newGraphQLServer(t, func(_ *http.Request, request graphQLRequest) (int, any) {
		if request.Variables["volumeInstanceId"] != "volume-instance-1" || request.Variables["volumeInstanceBackupId"] != "backup-1" {
			t.Fatalf("variables = %#v", request.Variables)
		}
		return http.StatusOK, map[string]any{"data": map[string]any{
			"volumeInstanceBackupLock": false,
		}}
	})
	defer server.Close()

	client := newTestClient(server.URL, "token", server.Client())
	err := client.Lock(context.Background(), "volume-instance-1", "backup-1")
	if err == nil || !strings.Contains(err.Error(), "lock") {
		t.Fatalf("Lock error = %v", err)
	}
}

func TestClientRejectsGraphQLErrorsWithoutLeakingToken(t *testing.T) {
	const token = "top-secret-project-token"
	server := newGraphQLServer(t, func(_ *http.Request, _ graphQLRequest) (int, any) {
		return http.StatusOK, map[string]any{
			"errors": []any{map[string]any{"message": "Not Authorized: " + token}},
		}
	})
	defer server.Close()

	client := newTestClient(server.URL, token, server.Client())
	_, err := client.Identity(context.Background())
	if err == nil || !strings.Contains(err.Error(), "Not Authorized") {
		t.Fatalf("Identity error = %v", err)
	}
	if strings.Contains(err.Error(), token) {
		t.Fatalf("error leaked project token: %v", err)
	}
}

func TestClientRejectsNonSuccessHTTPResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
		_, _ = io.WriteString(w, `{"error":"upstream unavailable"}`)
	}))
	defer server.Close()

	client := newTestClient(server.URL, "token", server.Client())
	_, err := client.Identity(context.Background())
	if err == nil || !strings.Contains(err.Error(), "502") {
		t.Fatalf("Identity error = %v", err)
	}
}

func TestNewClientUsesFixedRailwayPublicAPIEndpoint(t *testing.T) {
	client := New("token")
	if client.endpoint != publicGraphQLEndpoint {
		t.Fatalf("endpoint = %q", client.endpoint)
	}
}
