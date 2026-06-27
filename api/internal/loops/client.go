package loops

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const defaultBaseURL = "https://app.loops.so/api"

type Config struct {
	APIKey  string
	BaseURL string
	Client  *http.Client
}

type Client struct {
	apiKey  string
	baseURL string
	client  *http.Client
}

func NewClient(cfg Config) *Client {
	baseURL := strings.TrimRight(strings.TrimSpace(cfg.BaseURL), "/")
	if baseURL == "" {
		baseURL = defaultBaseURL
	}
	httpClient := cfg.Client
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 10 * time.Second}
	}
	return &Client{
		apiKey:  strings.TrimSpace(cfg.APIKey),
		baseURL: baseURL,
		client:  httpClient,
	}
}

func (c *Client) Enabled() bool {
	return c != nil && c.apiKey != ""
}

type Contact struct {
	Email      string
	UserID     string
	FirstName  string
	LastName   string
	Source     string
	UserGroup  string
	Properties map[string]any
}

func (c *Client) UpsertContact(ctx context.Context, contact Contact) error {
	payload := map[string]any{}
	for k, v := range contact.Properties {
		if strings.TrimSpace(k) != "" && v != nil {
			payload[k] = v
		}
	}
	set(payload, "email", contact.Email)
	set(payload, "userId", contact.UserID)
	set(payload, "firstName", contact.FirstName)
	set(payload, "lastName", contact.LastName)
	set(payload, "source", contact.Source)
	set(payload, "userGroup", contact.UserGroup)
	return c.doJSON(ctx, http.MethodPut, "/v1/contacts/update", payload, "")
}

type Event struct {
	Email          string
	UserID         string
	Name           string
	IdempotencyKey string
	Properties     map[string]any
}

func (c *Client) SendEvent(ctx context.Context, event Event) error {
	payload := map[string]any{}
	set(payload, "email", event.Email)
	set(payload, "userId", event.UserID)
	set(payload, "eventName", event.Name)
	if len(event.Properties) > 0 {
		payload["eventProperties"] = event.Properties
	}
	return c.doJSON(ctx, http.MethodPost, "/v1/events/send", payload, event.IdempotencyKey)
}

type TransactionalEmail struct {
	TransactionalID string
	Email           string
	UserID          string
	IdempotencyKey  string
	DataVariables   map[string]any
	Audit           EmailAudit
}

func (c *Client) SendTransactional(ctx context.Context, email TransactionalEmail) error {
	payload := map[string]any{}
	set(payload, "transactionalId", email.TransactionalID)
	set(payload, "email", email.Email)
	if len(email.DataVariables) > 0 {
		payload["dataVariables"] = email.DataVariables
	}
	return c.doJSON(ctx, http.MethodPost, "/v1/transactional", payload, email.IdempotencyKey)
}

func (c *Client) doJSON(ctx context.Context, method, path string, payload map[string]any, idempotencyKey string) error {
	if c == nil {
		return errors.New("loops: client is nil")
	}
	if c.apiKey == "" {
		return errors.New("loops: API key is required")
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("loops: marshal: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("loops: new request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Content-Type", "application/json")
	if strings.TrimSpace(idempotencyKey) != "" {
		req.Header.Set("Idempotency-Key", idempotencyKey)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("loops: http: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		_, _ = io.Copy(io.Discard, resp.Body)
		return nil
	}

	raw, _ := io.ReadAll(resp.Body)
	var apiErr struct {
		Message string `json:"message"`
		Error   string `json:"error"`
	}
	if err := json.Unmarshal(raw, &apiErr); err == nil {
		switch {
		case strings.TrimSpace(apiErr.Message) != "":
			return fmt.Errorf("loops: %d: %s", resp.StatusCode, apiErr.Message)
		case strings.TrimSpace(apiErr.Error) != "":
			return fmt.Errorf("loops: %d: %s", resp.StatusCode, apiErr.Error)
		}
	}
	return fmt.Errorf("loops: %d: %s", resp.StatusCode, strings.TrimSpace(string(raw)))
}

func set(payload map[string]any, key, value string) {
	if strings.TrimSpace(value) != "" {
		payload[key] = value
	}
}
