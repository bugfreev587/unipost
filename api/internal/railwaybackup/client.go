package railwaybackup

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

const publicGraphQLEndpoint = "https://backboard.railway.com/graphql/v2"

type Identity struct {
	ProjectID     string
	EnvironmentID string
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

type Client interface {
	Identity(context.Context) (Identity, error)
	List(context.Context, string) ([]Backup, error)
	Create(context.Context, string, string) (CreateResult, error)
	Lock(context.Context, string, string) error
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
