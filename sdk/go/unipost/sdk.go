package unipost

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	defaultBaseURL = "https://api.unipost.dev"
	defaultTimeout = 30 * time.Second
)

type Option func(*Client)

func WithAPIKey(apiKey string) Option {
	return func(c *Client) {
		c.apiKey = apiKey
	}
}

func WithBaseURL(baseURL string) Option {
	return func(c *Client) {
		c.baseURL = strings.TrimRight(baseURL, "/")
	}
}

func WithHTTPClient(client *http.Client) Option {
	return func(c *Client) {
		c.http = client
	}
}

type Client struct {
	apiKey    string
	baseURL   string
	http      *http.Client
	Accounts  *AccountsService
	Posts     *PostsService
	Analytics *AnalyticsService
	Connect   *ConnectService
	Users     *UsersService
	Webhooks  *WebhooksService
}

func NewClient(opts ...Option) *Client {
	client := &Client{
		apiKey:  os.Getenv("UNIPOST_API_KEY"),
		baseURL: defaultBaseURL,
		http:    &http.Client{Timeout: defaultTimeout},
	}
	for _, opt := range opts {
		opt(client)
	}
	client.Accounts = &AccountsService{client: client}
	client.Posts = &PostsService{client: client}
	client.Analytics = &AnalyticsService{client: client}
	client.Connect = &ConnectService{client: client}
	client.Users = &UsersService{client: client}
	client.Webhooks = &WebhooksService{client: client}
	return client
}

type APIError struct {
	Status  int
	Code    string
	Message string
}

func (e *APIError) Error() string {
	if e == nil {
		return ""
	}
	if e.Code == "" {
		return fmt.Sprintf("unipost api error (%d): %s", e.Status, e.Message)
	}
	return fmt.Sprintf("unipost api error (%d %s): %s", e.Status, e.Code, e.Message)
}

type apiEnvelope[T any] struct {
	Data T `json:"data"`
	Meta struct {
		Total      *int   `json:"total,omitempty"`
		Limit      *int   `json:"limit,omitempty"`
		HasMore    *bool  `json:"has_more,omitempty"`
		NextCursor string `json:"next_cursor,omitempty"`
	} `json:"meta"`
	NextCursor string `json:"next_cursor,omitempty"`
}

type PageMeta struct {
	Total      *int   `json:"total,omitempty"`
	Limit      *int   `json:"limit,omitempty"`
	HasMore    *bool  `json:"has_more,omitempty"`
	NextCursor string `json:"next_cursor,omitempty"`
}

type apiErrorEnvelope struct {
	Error struct {
		Code           string `json:"code"`
		NormalizedCode string `json:"normalized_code"`
		Message        string `json:"message"`
	} `json:"error"`
}

func (c *Client) do(ctx context.Context, method, path string, query map[string]string, body any, out any, headers map[string]string) error {
	if c.apiKey == "" {
		return fmt.Errorf("unipost api key is required")
	}

	fullURL, err := url.Parse(c.baseURL + path)
	if err != nil {
		return err
	}
	if len(query) > 0 {
		values := fullURL.Query()
		for key, value := range query {
			if value != "" {
				values.Set(key, value)
			}
		}
		fullURL.RawQuery = values.Encode()
	}

	var payload io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			return err
		}
		payload = bytes.NewReader(raw)
	}

	req, err := http.NewRequestWithContext(ctx, method, fullURL.String(), payload)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("User-Agent", "sdk-go/0.2.0-local")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	for key, value := range headers {
		req.Header.Set(key, value)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		var apiErr apiErrorEnvelope
		if len(data) > 0 {
			_ = json.Unmarshal(data, &apiErr)
		}
		return &APIError{
			Status:  resp.StatusCode,
			Code:    firstNonEmpty(apiErr.Error.NormalizedCode, apiErr.Error.Code),
			Message: apiErr.Error.Message,
		}
	}

	if out == nil || len(data) == 0 {
		return nil
	}
	return json.Unmarshal(data, out)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

type SocialAccount struct {
	ID                string `json:"id"`
	Platform          string `json:"platform"`
	AccountName       string `json:"account_name"`
	ExternalUserID    string `json:"external_user_id,omitempty"`
	ExternalUserEmail string `json:"external_user_email,omitempty"`
	Status            string `json:"status"`
	ConnectionType    string `json:"connection_type,omitempty"`
}

type ListAccountsParams struct {
	Platform       string
	ExternalUserID string
	Status         string
	ProfileID      string
}

type AccountsService struct {
	client *Client
}

func (s *AccountsService) List(ctx context.Context, params *ListAccountsParams) ([]SocialAccount, error) {
	page, err := s.ListPage(ctx, params)
	if err != nil {
		return nil, err
	}
	return page.Data, nil
}

type PaginatedAccounts struct {
	Data []SocialAccount
	Meta PageMeta
}

func (s *AccountsService) ListPage(ctx context.Context, params *ListAccountsParams) (*PaginatedAccounts, error) {
	query := map[string]string{}
	if params != nil {
		query["platform"] = params.Platform
		query["external_user_id"] = params.ExternalUserID
		query["status"] = params.Status
		query["profile_id"] = params.ProfileID
	}
	var envelope apiEnvelope[[]SocialAccount]
	if err := s.client.do(ctx, http.MethodGet, "/v1/social-accounts", query, nil, &envelope, nil); err != nil {
		return nil, err
	}
	return &PaginatedAccounts{
		Data: envelope.Data,
		Meta: PageMeta{
			Total:      envelope.Meta.Total,
			Limit:      envelope.Meta.Limit,
			HasMore:    envelope.Meta.HasMore,
			NextCursor: envelope.Meta.NextCursor,
		},
	}, nil
}

type PlatformResult struct {
	ID              string   `json:"id"`
	SocialAccountID string   `json:"social_account_id"`
	Platform        string   `json:"platform"`
	AccountName     string   `json:"account_name,omitempty"`
	Caption         string   `json:"caption,omitempty"`
	Status          string   `json:"status"`
	ExternalID      string   `json:"external_id,omitempty"`
	URL             string   `json:"url,omitempty"`
	ErrorMessage    string   `json:"error_message,omitempty"`
	PublishedAt     string   `json:"published_at,omitempty"`
	Warnings        []string `json:"warnings,omitempty"`
}

type Post struct {
	ID                 string           `json:"id"`
	Caption            *string          `json:"caption"`
	MediaURLs          []string         `json:"media_urls,omitempty"`
	Status             string           `json:"status"`
	ExecutionMode      string           `json:"execution_mode,omitempty"`
	QueuedResultsCount int              `json:"queued_results_count,omitempty"`
	ActiveJobCount     int              `json:"active_job_count,omitempty"`
	RetryingCount      int              `json:"retrying_count,omitempty"`
	DeadCount          int              `json:"dead_count,omitempty"`
	CreatedAt          time.Time        `json:"created_at"`
	ScheduledAt        *time.Time       `json:"scheduled_at,omitempty"`
	PublishedAt        *time.Time       `json:"published_at,omitempty"`
	Results            []PlatformResult `json:"results,omitempty"`
}

type PaginatedPosts struct {
	Data []Post
	Meta struct {
		Total      *int   `json:"total,omitempty"`
		Limit      *int   `json:"limit,omitempty"`
		HasMore    *bool  `json:"has_more,omitempty"`
		NextCursor string `json:"next_cursor,omitempty"`
	}
	NextCursor string
}

type CreatePostPlatform struct {
	AccountID       string         `json:"account_id"`
	Caption         string         `json:"caption,omitempty"`
	MediaURLs       []string       `json:"media_urls,omitempty"`
	MediaIDs        []string       `json:"media_ids,omitempty"`
	ThreadPosition  int            `json:"thread_position,omitempty"`
	FirstComment    string         `json:"first_comment,omitempty"`
	InReplyTo       string         `json:"in_reply_to,omitempty"`
	PlatformOptions map[string]any `json:"platform_options,omitempty"`
}

type CreatePostParams struct {
	Caption        string
	AccountIDs     []string
	MediaURLs      []string
	MediaIDs       []string
	ScheduledAt    string
	Status         string
	IdempotencyKey string
	PlatformPosts  []CreatePostPlatform
}

type ListPostsParams struct {
	Status   string
	Platform string
	From     string
	To       string
	Limit    int
	Cursor   string
}

type DeliveryJob struct {
	ID                 string     `json:"id"`
	PostID             string     `json:"post_id"`
	SocialPostResultID string     `json:"social_post_result_id"`
	SocialAccountID    string     `json:"social_account_id"`
	Platform           string     `json:"platform"`
	Kind               string     `json:"kind"`
	State              string     `json:"state"`
	Attempts           int32      `json:"attempts"`
	MaxAttempts        int32      `json:"max_attempts"`
	FailureStage       *string    `json:"failure_stage,omitempty"`
	ErrorCode          *string    `json:"error_code,omitempty"`
	PlatformErrorCode  *string    `json:"platform_error_code,omitempty"`
	LastError          *string    `json:"last_error,omitempty"`
	NextRunAt          *time.Time `json:"next_run_at,omitempty"`
	LastAttemptAt      *time.Time `json:"last_attempt_at,omitempty"`
	CreatedAt          time.Time  `json:"created_at"`
	UpdatedAt          time.Time  `json:"updated_at"`
}

type PostQueueSnapshot struct {
	Post Post          `json:"post"`
	Jobs []DeliveryJob `json:"jobs"`
}

type PostsService struct {
	client *Client
}

func marshalPostBody(params *CreatePostParams) map[string]any {
	body := map[string]any{}
	if params == nil {
		return body
	}
	if params.Caption != "" {
		body["caption"] = params.Caption
	}
	if len(params.AccountIDs) > 0 {
		body["account_ids"] = params.AccountIDs
	}
	if len(params.MediaURLs) > 0 {
		body["media_urls"] = params.MediaURLs
	}
	if len(params.MediaIDs) > 0 {
		body["media_ids"] = params.MediaIDs
	}
	if params.ScheduledAt != "" {
		body["scheduled_at"] = params.ScheduledAt
	}
	if params.Status != "" {
		body["status"] = params.Status
	}
	if len(params.PlatformPosts) > 0 {
		body["platform_posts"] = params.PlatformPosts
	}
	return body
}

func (s *PostsService) List(ctx context.Context, params *ListPostsParams) (*PaginatedPosts, error) {
	query := map[string]string{}
	if params != nil {
		query["status"] = params.Status
		query["platform"] = params.Platform
		query["from"] = params.From
		query["to"] = params.To
		query["cursor"] = params.Cursor
		if params.Limit > 0 {
			query["limit"] = strconv.Itoa(params.Limit)
		}
	}
	var envelope apiEnvelope[[]Post]
	if err := s.client.do(ctx, http.MethodGet, "/v1/social-posts", query, nil, &envelope, nil); err != nil {
		return nil, err
	}
	resp := &PaginatedPosts{Data: envelope.Data, Meta: envelope.Meta}
	if envelope.Meta.NextCursor != "" {
		resp.NextCursor = envelope.Meta.NextCursor
	} else {
		resp.NextCursor = envelope.NextCursor
	}
	return resp, nil
}

func (s *PostsService) Get(ctx context.Context, postID string) (*Post, error) {
	var envelope apiEnvelope[Post]
	if err := s.client.do(ctx, http.MethodGet, "/v1/social-posts/"+postID, nil, nil, &envelope, nil); err != nil {
		return nil, err
	}
	return &envelope.Data, nil
}

func (s *PostsService) GetQueue(ctx context.Context, postID string) (*PostQueueSnapshot, error) {
	var envelope apiEnvelope[PostQueueSnapshot]
	if err := s.client.do(ctx, http.MethodGet, "/v1/social-posts/"+postID+"/queue", nil, nil, &envelope, nil); err != nil {
		return nil, err
	}
	return &envelope.Data, nil
}

func (s *PostsService) Create(ctx context.Context, params *CreatePostParams) (*Post, error) {
	headers := map[string]string{}
	if params != nil && params.IdempotencyKey != "" {
		headers["Idempotency-Key"] = params.IdempotencyKey
	}
	var envelope apiEnvelope[Post]
	if err := s.client.do(ctx, http.MethodPost, "/v1/social-posts", nil, marshalPostBody(params), &envelope, headers); err != nil {
		return nil, err
	}
	return &envelope.Data, nil
}

func (s *PostsService) Cancel(ctx context.Context, postID string) (*Post, error) {
	var envelope apiEnvelope[Post]
	if err := s.client.do(ctx, http.MethodPost, "/v1/social-posts/"+postID+"/cancel", nil, nil, &envelope, nil); err != nil {
		return nil, err
	}
	return &envelope.Data, nil
}

type PostAnalytics struct {
	PostID      string `json:"post_id"`
	Impressions int    `json:"impressions"`
	Engagements int    `json:"engagements"`
	Likes       int    `json:"likes"`
	Comments    int    `json:"comments"`
	Shares      int    `json:"shares"`
	Clicks      int    `json:"clicks"`
}

type AnalyticsRollupParams struct {
	From        string
	To          string
	Granularity string
	GroupBy     string
}

type AnalyticsBucket struct {
	Key         string `json:"key"`
	Impressions int    `json:"impressions"`
	Engagements int    `json:"engagements"`
	Likes       int    `json:"likes"`
	Comments    int    `json:"comments"`
	Shares      int    `json:"shares"`
	Clicks      int    `json:"clicks"`
}

type AnalyticsRollup struct {
	From        string            `json:"from"`
	To          string            `json:"to"`
	Granularity string            `json:"granularity"`
	Buckets     []AnalyticsBucket `json:"buckets"`
}

type AnalyticsService struct {
	client *Client
}

func (s *AnalyticsService) Rollup(ctx context.Context, params *AnalyticsRollupParams) (*AnalyticsRollup, error) {
	query := map[string]string{}
	if params != nil {
		query["from"] = params.From
		query["to"] = params.To
		query["granularity"] = params.Granularity
		query["group_by"] = params.GroupBy
	}
	var envelope apiEnvelope[AnalyticsRollup]
	if err := s.client.do(ctx, http.MethodGet, "/v1/analytics/rollup", query, nil, &envelope, nil); err != nil {
		return nil, err
	}
	return &envelope.Data, nil
}

type CreateConnectSessionParams struct {
	Platform          string `json:"platform"`
	ProfileID         string `json:"profile_id,omitempty"`
	ExternalUserID    string `json:"external_user_id"`
	ExternalUserEmail string `json:"external_user_email,omitempty"`
	ReturnURL         string `json:"return_url,omitempty"`
}

type ConnectSession struct {
	ID             string    `json:"id"`
	URL            string    `json:"url"`
	Status         string    `json:"status"`
	ExpiresAt      time.Time `json:"expires_at"`
	Platform       string    `json:"platform"`
	ExternalUserID string    `json:"external_user_id"`
}

type ConnectService struct {
	client *Client
}

func (s *ConnectService) CreateSession(ctx context.Context, params *CreateConnectSessionParams) (*ConnectSession, error) {
	var envelope apiEnvelope[ConnectSession]
	if err := s.client.do(ctx, http.MethodPost, "/v1/connect/sessions", nil, params, &envelope, nil); err != nil {
		return nil, err
	}
	return &envelope.Data, nil
}

func (s *ConnectService) GetSession(ctx context.Context, sessionID string) (*ConnectSession, error) {
	var envelope apiEnvelope[ConnectSession]
	if err := s.client.do(ctx, http.MethodGet, "/v1/connect/sessions/"+sessionID, nil, nil, &envelope, nil); err != nil {
		return nil, err
	}
	return &envelope.Data, nil
}

type ManagedUser struct {
	ExternalUserID    string         `json:"external_user_id"`
	ExternalUserEmail string         `json:"external_user_email,omitempty"`
	AccountCount      int            `json:"account_count"`
	PlatformCounts    map[string]int `json:"platform_counts,omitempty"`
	ReconnectCount    int            `json:"reconnect_count"`
}

type UsersService struct {
	client *Client
}

func (s *UsersService) List(ctx context.Context) ([]ManagedUser, error) {
	page, err := s.ListPage(ctx)
	if err != nil {
		return nil, err
	}
	return page.Data, nil
}

type PaginatedManagedUsers struct {
	Data []ManagedUser
	Meta PageMeta
}

func (s *UsersService) ListPage(ctx context.Context) (*PaginatedManagedUsers, error) {
	var envelope apiEnvelope[[]ManagedUser]
	if err := s.client.do(ctx, http.MethodGet, "/v1/users", nil, nil, &envelope, nil); err != nil {
		return nil, err
	}
	return &PaginatedManagedUsers{
		Data: envelope.Data,
		Meta: PageMeta{
			Total:      envelope.Meta.Total,
			Limit:      envelope.Meta.Limit,
			HasMore:    envelope.Meta.HasMore,
			NextCursor: envelope.Meta.NextCursor,
		},
	}, nil
}

func (s *UsersService) Get(ctx context.Context, externalUserID string) (*ManagedUser, error) {
	var envelope apiEnvelope[ManagedUser]
	if err := s.client.do(ctx, http.MethodGet, "/v1/users/"+url.PathEscape(externalUserID), nil, nil, &envelope, nil); err != nil {
		return nil, err
	}
	return &envelope.Data, nil
}

type WebhookSubscription struct {
	ID            string    `json:"id"`
	URL           string    `json:"url"`
	Events        []string  `json:"events"`
	Active        bool      `json:"active"`
	Secret        string    `json:"secret,omitempty"`
	SecretPreview string    `json:"secret_preview"`
	CreatedAt     time.Time `json:"created_at"`
}

type CreateWebhookParams struct {
	URL    string   `json:"url"`
	Events []string `json:"events"`
}

type UpdateWebhookParams struct {
	URL    *string  `json:"url,omitempty"`
	Events []string `json:"events,omitempty"`
	Active *bool    `json:"active,omitempty"`
}

type WebhooksService struct {
	client *Client
}

func (s *WebhooksService) Create(ctx context.Context, params *CreateWebhookParams) (*WebhookSubscription, error) {
	var envelope apiEnvelope[WebhookSubscription]
	if err := s.client.do(ctx, http.MethodPost, "/v1/webhooks", nil, params, &envelope, nil); err != nil {
		return nil, err
	}
	return &envelope.Data, nil
}

func (s *WebhooksService) List(ctx context.Context) ([]WebhookSubscription, error) {
	page, err := s.ListPage(ctx)
	if err != nil {
		return nil, err
	}
	return page.Data, nil
}

type PaginatedWebhookSubscriptions struct {
	Data []WebhookSubscription
	Meta PageMeta
}

func (s *WebhooksService) ListPage(ctx context.Context) (*PaginatedWebhookSubscriptions, error) {
	var envelope apiEnvelope[[]WebhookSubscription]
	if err := s.client.do(ctx, http.MethodGet, "/v1/webhooks", nil, nil, &envelope, nil); err != nil {
		return nil, err
	}
	return &PaginatedWebhookSubscriptions{
		Data: envelope.Data,
		Meta: PageMeta{
			Total:      envelope.Meta.Total,
			Limit:      envelope.Meta.Limit,
			HasMore:    envelope.Meta.HasMore,
			NextCursor: envelope.Meta.NextCursor,
		},
	}, nil
}

func (s *WebhooksService) Get(ctx context.Context, webhookID string) (*WebhookSubscription, error) {
	var envelope apiEnvelope[WebhookSubscription]
	if err := s.client.do(ctx, http.MethodGet, "/v1/webhooks/"+webhookID, nil, nil, &envelope, nil); err != nil {
		return nil, err
	}
	return &envelope.Data, nil
}

func (s *WebhooksService) Update(ctx context.Context, webhookID string, params *UpdateWebhookParams) (*WebhookSubscription, error) {
	var envelope apiEnvelope[WebhookSubscription]
	if err := s.client.do(ctx, http.MethodPatch, "/v1/webhooks/"+webhookID, nil, params, &envelope, nil); err != nil {
		return nil, err
	}
	return &envelope.Data, nil
}

func (s *WebhooksService) Rotate(ctx context.Context, webhookID string) (*WebhookSubscription, error) {
	var envelope apiEnvelope[WebhookSubscription]
	if err := s.client.do(ctx, http.MethodPost, "/v1/webhooks/"+webhookID+"/rotate", nil, nil, &envelope, nil); err != nil {
		return nil, err
	}
	return &envelope.Data, nil
}

func (s *WebhooksService) Delete(ctx context.Context, webhookID string) error {
	return s.client.do(ctx, http.MethodDelete, "/v1/webhooks/"+webhookID, nil, nil, nil, nil)
}

func VerifyWebhookSignature(payload []byte, signature, secret string) bool {
	signature = strings.TrimSpace(signature)
	signature = strings.TrimPrefix(strings.ToLower(signature), "sha256=")
	if signature == "" || secret == "" {
		return false
	}
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(payload)
	expected := hex.EncodeToString(mac.Sum(nil))
	return hmac.Equal([]byte(expected), []byte(signature))
}
