package railwaybackup

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
)

const publicGraphQLEndpoint = "https://backboard.railway.com/graphql/v2"

type Identity struct {
	ProjectID     string
	EnvironmentID string
}

type VolumeInstanceIdentity struct {
	ID            string
	ProjectID     string
	EnvironmentID string
	ServiceID     string
}

type Backup struct {
	ID           string
	Name         string
	CreatedAt    string
	ExternalID   string
	ReferencedMB *int64
}

type CreateResult struct {
	WorkflowID string
}

type DatabaseBindingRequest struct {
	ProjectID            string
	EnvironmentID        string
	ApplicationServiceID string
	PostgresServiceID    string
	RuntimeDatabaseURL   string
}

type Client interface {
	Identity(context.Context) (Identity, error)
	VolumeInstanceIdentity(context.Context, string) (VolumeInstanceIdentity, error)
	VerifyDatabaseBinding(context.Context, DatabaseBindingRequest) error
	List(context.Context, string) ([]Backup, error)
	Create(context.Context, string, string) (CreateResult, error)
	Lock(context.Context, string, string) error
}

func (c *GraphQLClient) VerifyDatabaseBinding(ctx context.Context, request DatabaseBindingRequest) error {
	missing := make([]string, 0, 5)
	for name, value := range map[string]string{
		"project ID":             request.ProjectID,
		"environment ID":         request.EnvironmentID,
		"application service ID": request.ApplicationServiceID,
		"Postgres service ID":    request.PostgresServiceID,
		"runtime DATABASE_URL":   request.RuntimeDatabaseURL,
	} {
		if strings.TrimSpace(value) == "" {
			missing = append(missing, name)
		}
	}
	if len(missing) > 0 {
		sort.Strings(missing)
		return fmt.Errorf("verify Railway database binding: missing %s", strings.Join(missing, ", "))
	}

	var data struct {
		Project struct {
			ID       string `json:"id"`
			Services struct {
				Edges []struct {
					Node struct {
						ID   string `json:"id"`
						Name string `json:"name"`
					} `json:"node"`
				} `json:"edges"`
			} `json:"services"`
		} `json:"project"`
		Unrendered map[string]string `json:"unrendered"`
		Rendered   map[string]string `json:"rendered"`
	}
	variables := map[string]any{
		"projectId":            request.ProjectID,
		"environmentId":        request.EnvironmentID,
		"applicationServiceId": request.ApplicationServiceID,
	}
	query := `query($projectId: String!, $environmentId: String!, $applicationServiceId: String!) {
  project(id: $projectId) {
    id
    services { edges { node { id name } } }
  }
  unrendered: variables(
    projectId: $projectId
    environmentId: $environmentId
    serviceId: $applicationServiceId
    unrendered: true
  )
  rendered: variablesForServiceDeployment(
    projectId: $projectId
    environmentId: $environmentId
    serviceId: $applicationServiceId
  )
}`
	if err := execute(ctx, c, query, variables, &data); err != nil {
		return fmt.Errorf("verify Railway database binding: %w", err)
	}
	if data.Project.ID != request.ProjectID {
		return fmt.Errorf("verify Railway database binding: project identity mismatch")
	}

	applicationMatches := 0
	postgresMatches := 0
	postgresNameMatches := 0
	postgresServiceName := ""
	for _, edge := range data.Project.Services.Edges {
		if edge.Node.ID == request.ApplicationServiceID {
			applicationMatches++
		}
		if edge.Node.ID == request.PostgresServiceID {
			postgresMatches++
			postgresServiceName = edge.Node.Name
		}
	}
	if applicationMatches != 1 || postgresMatches != 1 || strings.TrimSpace(postgresServiceName) == "" {
		return fmt.Errorf("verify Railway database binding: application or Postgres service identity is missing or ambiguous")
	}
	for _, edge := range data.Project.Services.Edges {
		if edge.Node.Name == postgresServiceName {
			postgresNameMatches++
		}
	}
	if postgresNameMatches != 1 {
		return fmt.Errorf("verify Railway database binding: Postgres service name is ambiguous")
	}
	wantReference := "${{" + postgresServiceName + ".DATABASE_URL}}"
	if data.Unrendered["DATABASE_URL"] != wantReference {
		return fmt.Errorf("verify Railway database binding: application DATABASE_URL does not exactly reference the verified Postgres service")
	}
	renderedURL := data.Rendered["DATABASE_URL"]
	if strings.TrimSpace(renderedURL) == "" {
		return fmt.Errorf("verify Railway database binding: rendered application DATABASE_URL is missing")
	}
	if renderedURL != request.RuntimeDatabaseURL {
		return fmt.Errorf("verify Railway database binding: rendered DATABASE_URL does not match runtime DATABASE_URL")
	}
	return nil
}

func (c *GraphQLClient) VolumeInstanceIdentity(ctx context.Context, volumeInstanceID string) (VolumeInstanceIdentity, error) {
	var data struct {
		VolumeInstance struct {
			ID            string `json:"id"`
			EnvironmentID string `json:"environmentId"`
			ServiceID     string `json:"serviceId"`
			Volume        struct {
				ProjectID string `json:"projectId"`
			} `json:"volume"`
		} `json:"volumeInstance"`
	}
	variables := map[string]any{"id": volumeInstanceID}
	query := `query($id: String!) {
  volumeInstance(id: $id) {
    id
    environmentId
    serviceId
    volume { projectId }
  }
}`
	if err := execute(ctx, c, query, variables, &data); err != nil {
		return VolumeInstanceIdentity{}, fmt.Errorf("read Railway volume instance identity: %w", err)
	}
	identity := VolumeInstanceIdentity{
		ID:            data.VolumeInstance.ID,
		ProjectID:     data.VolumeInstance.Volume.ProjectID,
		EnvironmentID: data.VolumeInstance.EnvironmentID,
		ServiceID:     data.VolumeInstance.ServiceID,
	}
	missing := make([]string, 0, 4)
	for name, value := range map[string]string{
		"volume instance ID": identity.ID,
		"project ID":         identity.ProjectID,
		"environment ID":     identity.EnvironmentID,
		"service ID":         identity.ServiceID,
	} {
		if strings.TrimSpace(value) == "" {
			missing = append(missing, name)
		}
	}
	if len(missing) > 0 {
		sort.Strings(missing)
		return VolumeInstanceIdentity{}, fmt.Errorf(
			"read Railway volume instance identity: response is missing %s",
			strings.Join(missing, ", "),
		)
	}
	return identity, nil
}

type GraphQLClient struct {
	endpoint   string
	token      string
	httpClient *http.Client
}

func New(token string) *GraphQLClient {
	return &GraphQLClient{
		endpoint:   publicGraphQLEndpoint,
		token:      token,
		httpClient: http.DefaultClient,
	}
}

func newTestClient(endpoint, token string, httpClient *http.Client) *GraphQLClient {
	return &GraphQLClient{endpoint: endpoint, token: token, httpClient: httpClient}
}

func (c *GraphQLClient) Identity(ctx context.Context) (Identity, error) {
	var data struct {
		ProjectToken struct {
			ProjectID     string `json:"projectId"`
			EnvironmentID string `json:"environmentId"`
		} `json:"projectToken"`
	}
	if err := execute(ctx, c, `query { projectToken { projectId environmentId } }`, nil, &data); err != nil {
		return Identity{}, fmt.Errorf("read Railway project token identity: %w", err)
	}
	if data.ProjectToken.ProjectID == "" || data.ProjectToken.EnvironmentID == "" {
		return Identity{}, fmt.Errorf("read Railway project token identity: response is missing project or environment ID")
	}
	return Identity{
		ProjectID:     data.ProjectToken.ProjectID,
		EnvironmentID: data.ProjectToken.EnvironmentID,
	}, nil
}

func (c *GraphQLClient) List(ctx context.Context, volumeInstanceID string) ([]Backup, error) {
	var data struct {
		Backups []struct {
			ID           string `json:"id"`
			Name         string `json:"name"`
			CreatedAt    string `json:"createdAt"`
			ExternalID   string `json:"externalId"`
			ReferencedMB *int64 `json:"referencedMB"`
		} `json:"volumeInstanceBackupList"`
	}
	variables := map[string]any{"volumeInstanceId": volumeInstanceID}
	query := `query($volumeInstanceId: String!) {
  volumeInstanceBackupList(volumeInstanceId: $volumeInstanceId) {
    id name createdAt externalId referencedMB
  }
}`
	if err := execute(ctx, c, query, variables, &data); err != nil {
		return nil, fmt.Errorf("list Railway volume backups: %w", err)
	}
	backups := make([]Backup, 0, len(data.Backups))
	for _, item := range data.Backups {
		backups = append(backups, Backup{
			ID:           item.ID,
			Name:         item.Name,
			CreatedAt:    item.CreatedAt,
			ExternalID:   item.ExternalID,
			ReferencedMB: item.ReferencedMB,
		})
	}
	return backups, nil
}

func (c *GraphQLClient) Create(ctx context.Context, volumeInstanceID, name string) (CreateResult, error) {
	var data struct {
		Result struct {
			WorkflowID string `json:"workflowId"`
		} `json:"volumeInstanceBackupCreate"`
	}
	variables := map[string]any{"volumeInstanceId": volumeInstanceID, "name": name}
	query := `mutation($volumeInstanceId: String!, $name: String) {
  volumeInstanceBackupCreate(volumeInstanceId: $volumeInstanceId, name: $name) { workflowId }
}`
	if err := execute(ctx, c, query, variables, &data); err != nil {
		return CreateResult{}, fmt.Errorf("create Railway volume backup: %w", err)
	}
	if data.Result.WorkflowID == "" {
		return CreateResult{}, fmt.Errorf("create Railway volume backup: response is missing workflow ID")
	}
	return CreateResult{WorkflowID: data.Result.WorkflowID}, nil
}

func (c *GraphQLClient) Lock(ctx context.Context, volumeInstanceID, backupID string) error {
	var data struct {
		Locked bool `json:"volumeInstanceBackupLock"`
	}
	variables := map[string]any{
		"volumeInstanceId":       volumeInstanceID,
		"volumeInstanceBackupId": backupID,
	}
	query := `mutation($volumeInstanceId: String!, $volumeInstanceBackupId: String!) {
  volumeInstanceBackupLock(volumeInstanceId: $volumeInstanceId, volumeInstanceBackupId: $volumeInstanceBackupId)
}`
	if err := execute(ctx, c, query, variables, &data); err != nil {
		return fmt.Errorf("lock Railway volume backup: %w", err)
	}
	if !data.Locked {
		return fmt.Errorf("lock Railway volume backup: Railway returned false")
	}
	return nil
}

type requestEnvelope struct {
	Query     string         `json:"query"`
	Variables map[string]any `json:"variables,omitempty"`
}

type responseEnvelope[T any] struct {
	Data   T `json:"data"`
	Errors []struct {
		Message string `json:"message"`
	} `json:"errors"`
}

func execute[T any](ctx context.Context, client *GraphQLClient, query string, variables map[string]any, data *T) (err error) {
	defer func() {
		if err != nil && client.token != "" {
			err = fmt.Errorf("%s", strings.ReplaceAll(err.Error(), client.token, "[REDACTED]"))
		}
	}()
	payload, err := json.Marshal(requestEnvelope{Query: query, Variables: variables})
	if err != nil {
		return fmt.Errorf("encode request: %w", err)
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, client.endpoint, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Project-Access-Token", client.token)

	response, err := client.httpClient.Do(request)
	if err != nil {
		return fmt.Errorf("send request: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 1<<20))
		return fmt.Errorf("Railway API returned HTTP %d", response.StatusCode)
	}

	var envelope responseEnvelope[T]
	decoder := json.NewDecoder(io.LimitReader(response.Body, 1<<20))
	if err := decoder.Decode(&envelope); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}
	if len(envelope.Errors) > 0 {
		return fmt.Errorf("Railway GraphQL error: %s", envelope.Errors[0].Message)
	}
	*data = envelope.Data
	return nil
}
