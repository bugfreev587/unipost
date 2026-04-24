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
	apiKey              string
	baseURL             string
	http                *http.Client
	Workspace           *WorkspaceService
	Profiles            *ProfilesService
	Accounts            *AccountsService
	Platforms           *PlatformsService
	Plans               *PlansService
	PlatformCredentials *PlatformCredentialsService
	Posts               *PostsService
	DeliveryJobs        *DeliveryJobsService
	Media               *MediaService
	Analytics           *AnalyticsService
	Connect             *ConnectService
	Users               *UsersService
	Webhooks            *WebhooksService
	OAuth               *OAuthService
	Usage               *UsageService
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
	client.Workspace = &WorkspaceService{client: client}
	client.Profiles = &ProfilesService{client: client}
	client.Accounts = &AccountsService{client: client}
	client.Platforms = &PlatformsService{client: client}
	client.Plans = &PlansService{client: client}
	client.PlatformCredentials = &PlatformCredentialsService{client: client}
	client.Posts = &PostsService{client: client}
	client.DeliveryJobs = &DeliveryJobsService{client: client}
	client.Media = &MediaService{client: client}
	client.Analytics = &AnalyticsService{client: client}
	client.Connect = &ConnectService{client: client}
	client.Users = &UsersService{client: client}
	client.Webhooks = &WebhooksService{client: client}
	client.OAuth = &OAuthService{client: client}
	client.Usage = &UsageService{client: client}
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

type JSONMap map[string]any

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
			if strings.TrimSpace(value) != "" {
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

func pageMetaFromEnvelope[T any](envelope apiEnvelope[T]) PageMeta {
	return PageMeta{
		Total:      envelope.Meta.Total,
		Limit:      envelope.Meta.Limit,
		HasMore:    envelope.Meta.HasMore,
		NextCursor: envelope.Meta.NextCursor,
	}
}

func queryFromAnalytics(params *AnalyticsQueryParams) map[string]string {
	query := map[string]string{}
	if params == nil {
		return query
	}
	query["from"] = params.From
	query["to"] = params.To
	query["profile_id"] = params.ProfileID
	query["platform"] = params.Platform
	query["status"] = params.Status
	return query
}

func compactMap(items map[string]any) map[string]any {
	out := map[string]any{}
	for key, value := range items {
		switch v := value.(type) {
		case nil:
			continue
		case string:
			if strings.TrimSpace(v) == "" {
				continue
			}
		}
		out[key] = value
	}
	return out
}

type Workspace struct {
	ID                     string    `json:"id"`
	Name                   string    `json:"name"`
	PerAccountMonthlyLimit *int32    `json:"per_account_monthly_limit,omitempty"`
	UsageModes             []string  `json:"usage_modes,omitempty"`
	CreatedAt              time.Time `json:"created_at"`
	UpdatedAt              time.Time `json:"updated_at"`
}

type WorkspaceService struct {
	client *Client
}

func (s *WorkspaceService) Get(ctx context.Context) (*Workspace, error) {
	var envelope apiEnvelope[Workspace]
	if err := s.client.do(ctx, http.MethodGet, "/v1/workspace", nil, nil, &envelope, nil); err != nil {
		return nil, err
	}
	return &envelope.Data, nil
}

func (s *WorkspaceService) Update(ctx context.Context, perAccountMonthlyLimit *int32) (*Workspace, error) {
	body := map[string]any{}
	if perAccountMonthlyLimit != nil {
		body["per_account_monthly_limit"] = *perAccountMonthlyLimit
	}
	var envelope apiEnvelope[Workspace]
	if err := s.client.do(ctx, http.MethodPatch, "/v1/workspace", nil, body, &envelope, nil); err != nil {
		return nil, err
	}
	return &envelope.Data, nil
}

type Profile struct {
	ID                   string    `json:"id"`
	WorkspaceID          string    `json:"workspace_id"`
	Name                 string    `json:"name"`
	AccountCount         int       `json:"account_count"`
	CreatedAt            time.Time `json:"created_at"`
	UpdatedAt            time.Time `json:"updated_at"`
	BrandingLogoURL      *string   `json:"branding_logo_url,omitempty"`
	BrandingDisplayName  *string   `json:"branding_display_name,omitempty"`
	BrandingPrimaryColor *string   `json:"branding_primary_color,omitempty"`
}

type ProfilesService struct {
	client *Client
}

type PaginatedProfiles struct {
	Data []Profile
	Meta PageMeta
}

type CreateProfileParams struct {
	Name                 string  `json:"name"`
	BrandingLogoURL      *string `json:"branding_logo_url,omitempty"`
	BrandingDisplayName  *string `json:"branding_display_name,omitempty"`
	BrandingPrimaryColor *string `json:"branding_primary_color,omitempty"`
}

func (s *ProfilesService) List(ctx context.Context) (*PaginatedProfiles, error) {
	var envelope apiEnvelope[[]Profile]
	if err := s.client.do(ctx, http.MethodGet, "/v1/profiles", nil, nil, &envelope, nil); err != nil {
		return nil, err
	}
	return &PaginatedProfiles{Data: envelope.Data, Meta: pageMetaFromEnvelope(envelope)}, nil
}

func (s *ProfilesService) Create(ctx context.Context, params *CreateProfileParams) (*Profile, error) {
	body := map[string]any{}
	if params != nil {
		body["name"] = params.Name
		if params.BrandingLogoURL != nil {
			body["branding_logo_url"] = *params.BrandingLogoURL
		}
		if params.BrandingDisplayName != nil {
			body["branding_display_name"] = *params.BrandingDisplayName
		}
		if params.BrandingPrimaryColor != nil {
			body["branding_primary_color"] = *params.BrandingPrimaryColor
		}
	}
	var envelope apiEnvelope[Profile]
	if err := s.client.do(ctx, http.MethodPost, "/v1/profiles", nil, body, &envelope, nil); err != nil {
		return nil, err
	}
	return &envelope.Data, nil
}

func (s *ProfilesService) Get(ctx context.Context, profileID string) (*Profile, error) {
	var envelope apiEnvelope[Profile]
	if err := s.client.do(ctx, http.MethodGet, "/v1/profiles/"+profileID, nil, nil, &envelope, nil); err != nil {
		return nil, err
	}
	return &envelope.Data, nil
}

type UpdateProfileParams struct {
	Name                 *string `json:"name,omitempty"`
	BrandingLogoURL      *string `json:"branding_logo_url,omitempty"`
	BrandingDisplayName  *string `json:"branding_display_name,omitempty"`
	BrandingPrimaryColor *string `json:"branding_primary_color,omitempty"`
}

func (s *ProfilesService) Update(ctx context.Context, profileID string, params *UpdateProfileParams) (*Profile, error) {
	body := map[string]any{}
	if params != nil {
		if params.Name != nil {
			body["name"] = *params.Name
		}
		if params.BrandingLogoURL != nil {
			body["branding_logo_url"] = *params.BrandingLogoURL
		}
		if params.BrandingDisplayName != nil {
			body["branding_display_name"] = *params.BrandingDisplayName
		}
		if params.BrandingPrimaryColor != nil {
			body["branding_primary_color"] = *params.BrandingPrimaryColor
		}
	}
	var envelope apiEnvelope[Profile]
	if err := s.client.do(ctx, http.MethodPatch, "/v1/profiles/"+profileID, nil, body, &envelope, nil); err != nil {
		return nil, err
	}
	return &envelope.Data, nil
}

func (s *ProfilesService) Delete(ctx context.Context, profileID string) error {
	return s.client.do(ctx, http.MethodDelete, "/v1/profiles/"+profileID, nil, nil, nil, nil)
}

type SocialAccount struct {
	ID                string `json:"id"`
	ProfileID         string `json:"profile_id,omitempty"`
	ProfileName       string `json:"profile_name,omitempty"`
	Platform          string `json:"platform"`
	AccountName       string `json:"account_name,omitempty"`
	ExternalUserID    string `json:"external_user_id,omitempty"`
	ExternalUserEmail string `json:"external_user_email,omitempty"`
	Status            string `json:"status"`
	ConnectionType    string `json:"connection_type,omitempty"`
}

type AccountHealth struct {
	SocialAccountID      string     `json:"social_account_id"`
	Platform             string     `json:"platform"`
	Status               string     `json:"status"`
	LastSuccessfulPostAt *time.Time `json:"last_successful_post_at,omitempty"`
	TokenExpiresAt       *time.Time `json:"token_expires_at,omitempty"`
	LastError            *JSONMap   `json:"last_error,omitempty"`
}

type ConnectAccountParams struct {
	ProfileID   string            `json:"profile_id,omitempty"`
	Platform    string            `json:"platform"`
	Credentials map[string]string `json:"credentials"`
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

type PaginatedAccounts struct {
	Data []SocialAccount
	Meta PageMeta
}

func (s *AccountsService) List(ctx context.Context, params *ListAccountsParams) ([]SocialAccount, error) {
	page, err := s.ListPage(ctx, params)
	if err != nil {
		return nil, err
	}
	return page.Data, nil
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
	if err := s.client.do(ctx, http.MethodGet, "/v1/accounts", query, nil, &envelope, nil); err != nil {
		return nil, err
	}
	return &PaginatedAccounts{Data: envelope.Data, Meta: pageMetaFromEnvelope(envelope)}, nil
}

func (s *AccountsService) Get(ctx context.Context, accountID string) (*SocialAccount, error) {
	accounts, err := s.List(ctx, nil)
	if err != nil {
		return nil, err
	}
	for _, account := range accounts {
		if account.ID == accountID {
			copy := account
			return &copy, nil
		}
	}
	return nil, &APIError{Status: http.StatusNotFound, Code: "not_found", Message: "account not found"}
}

func (s *AccountsService) Connect(ctx context.Context, params *ConnectAccountParams) (*SocialAccount, error) {
	var envelope apiEnvelope[SocialAccount]
	if err := s.client.do(ctx, http.MethodPost, "/v1/accounts/connect", nil, params, &envelope, nil); err != nil {
		return nil, err
	}
	return &envelope.Data, nil
}

func (s *AccountsService) Disconnect(ctx context.Context, accountID string) (JSONMap, error) {
	var envelope apiEnvelope[JSONMap]
	if err := s.client.do(ctx, http.MethodDelete, "/v1/accounts/"+accountID, nil, nil, &envelope, nil); err != nil {
		return nil, err
	}
	return envelope.Data, nil
}

func (s *AccountsService) Capabilities(ctx context.Context, accountID string) (JSONMap, error) {
	var envelope apiEnvelope[JSONMap]
	if err := s.client.do(ctx, http.MethodGet, "/v1/accounts/"+accountID+"/capabilities", nil, nil, &envelope, nil); err != nil {
		return nil, err
	}
	return envelope.Data, nil
}

func (s *AccountsService) Health(ctx context.Context, accountID string) (*AccountHealth, error) {
	var envelope apiEnvelope[AccountHealth]
	if err := s.client.do(ctx, http.MethodGet, "/v1/accounts/"+accountID+"/health", nil, nil, &envelope, nil); err != nil {
		return nil, err
	}
	return &envelope.Data, nil
}

func (s *AccountsService) TikTokCreatorInfo(ctx context.Context, accountID string) (JSONMap, error) {
	var envelope apiEnvelope[JSONMap]
	if err := s.client.do(ctx, http.MethodGet, "/v1/accounts/"+accountID+"/tiktok/creator-info", nil, nil, &envelope, nil); err != nil {
		return nil, err
	}
	return envelope.Data, nil
}

func (s *AccountsService) FacebookPageInsights(ctx context.Context, accountID string) (JSONMap, error) {
	var envelope apiEnvelope[JSONMap]
	if err := s.client.do(ctx, http.MethodGet, "/v1/accounts/"+accountID+"/facebook/page-insights", nil, nil, &envelope, nil); err != nil {
		return nil, err
	}
	return envelope.Data, nil
}

type PlatformsService struct {
	client *Client
}

func (s *PlatformsService) Capabilities(ctx context.Context) (JSONMap, error) {
	var envelope apiEnvelope[JSONMap]
	if err := s.client.do(ctx, http.MethodGet, "/v1/platforms/capabilities", nil, nil, &envelope, nil); err != nil {
		return nil, err
	}
	return envelope.Data, nil
}

type Plan struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	PriceCents int32  `json:"price_cents"`
	PostLimit  int32  `json:"post_limit"`
}

type PlansService struct {
	client *Client
}

func (s *PlansService) List(ctx context.Context) ([]Plan, error) {
	var envelope apiEnvelope[[]Plan]
	if err := s.client.do(ctx, http.MethodGet, "/v1/plans", nil, nil, &envelope, nil); err != nil {
		return nil, err
	}
	return envelope.Data, nil
}

type PlatformCredential struct {
	Platform  string    `json:"platform"`
	ClientID  string    `json:"client_id"`
	CreatedAt time.Time `json:"created_at"`
}

type CreatePlatformCredentialParams struct {
	Platform     string `json:"platform"`
	ClientID     string `json:"client_id"`
	ClientSecret string `json:"client_secret"`
}

type PlatformCredentialsService struct {
	client *Client
}

type PaginatedPlatformCredentials struct {
	Data []PlatformCredential
	Meta PageMeta
}

func (s *PlatformCredentialsService) Create(ctx context.Context, workspaceID string, params *CreatePlatformCredentialParams) (*PlatformCredential, error) {
	var envelope apiEnvelope[PlatformCredential]
	if err := s.client.do(ctx, http.MethodPost, "/v1/workspaces/"+workspaceID+"/platform-credentials", nil, params, &envelope, nil); err != nil {
		return nil, err
	}
	return &envelope.Data, nil
}

func (s *PlatformCredentialsService) List(ctx context.Context, workspaceID string) (*PaginatedPlatformCredentials, error) {
	var envelope apiEnvelope[[]PlatformCredential]
	if err := s.client.do(ctx, http.MethodGet, "/v1/workspaces/"+workspaceID+"/platform-credentials", nil, nil, &envelope, nil); err != nil {
		return nil, err
	}
	return &PaginatedPlatformCredentials{Data: envelope.Data, Meta: pageMetaFromEnvelope(envelope)}, nil
}

func (s *PlatformCredentialsService) Delete(ctx context.Context, workspaceID, platform string) error {
	return s.client.do(ctx, http.MethodDelete, "/v1/workspaces/"+workspaceID+"/platform-credentials/"+platform, nil, nil, nil, nil)
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

type UpdatePostParams struct {
	Caption       *string
	AccountIDs    []string
	MediaURLs     []string
	MediaIDs      []string
	ScheduledAt   *string
	Status        *string
	Archived      *bool
	PlatformPosts []CreatePostPlatform
}

type ValidationIssue struct {
	PlatformPostIndex int    `json:"platform_post_index"`
	AccountID         string `json:"account_id,omitempty"`
	Platform          string `json:"platform,omitempty"`
	Field             string `json:"field"`
	Code              string `json:"code"`
	Message           string `json:"message"`
	Severity          string `json:"severity"`
}

type ValidationResult struct {
	Valid    bool              `json:"valid"`
	Errors   []ValidationIssue `json:"errors"`
	Warnings []ValidationIssue `json:"warnings"`
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

type PostAnalyticsItem struct {
	PostID              string  `json:"post_id"`
	SocialAccountID     string  `json:"social_account_id"`
	Platform            string  `json:"platform"`
	ExternalID          string  `json:"external_id"`
	Impressions         int64   `json:"impressions"`
	Reach               int64   `json:"reach"`
	Likes               int64   `json:"likes"`
	Comments            int64   `json:"comments"`
	Shares              int64   `json:"shares"`
	Saves               int64   `json:"saves"`
	Clicks              int64   `json:"clicks"`
	VideoViews          int64   `json:"video_views"`
	Views               int64   `json:"views"`
	EngagementRate      float64 `json:"engagement_rate"`
	ConsecutiveFailures int32   `json:"consecutive_failures"`
	LastFailureReason   string  `json:"last_failure_reason,omitempty"`
}

type PostPreviewLink struct {
	URL       string    `json:"url"`
	Token     string    `json:"token"`
	ExpiresAt time.Time `json:"expires_at"`
}

type BulkError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type BulkPostResult struct {
	Status int        `json:"status"`
	Data   *Post      `json:"data,omitempty"`
	Error  *BulkError `json:"error,omitempty"`
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

func marshalUpdatePostBody(params *UpdatePostParams) map[string]any {
	body := map[string]any{}
	if params == nil {
		return body
	}
	if params.Caption != nil {
		body["caption"] = *params.Caption
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
	if params.ScheduledAt != nil {
		body["scheduled_at"] = *params.ScheduledAt
	}
	if params.Status != nil {
		body["status"] = *params.Status
	}
	if params.Archived != nil {
		body["archived"] = *params.Archived
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
	if err := s.client.do(ctx, http.MethodGet, "/v1/posts", query, nil, &envelope, nil); err != nil {
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
	if err := s.client.do(ctx, http.MethodGet, "/v1/posts/"+postID, nil, nil, &envelope, nil); err != nil {
		return nil, err
	}
	return &envelope.Data, nil
}

func (s *PostsService) GetQueue(ctx context.Context, postID string) (*PostQueueSnapshot, error) {
	var envelope apiEnvelope[PostQueueSnapshot]
	if err := s.client.do(ctx, http.MethodGet, "/v1/posts/"+postID+"/queue", nil, nil, &envelope, nil); err != nil {
		return nil, err
	}
	return &envelope.Data, nil
}

func (s *PostsService) Analytics(ctx context.Context, postID string, refresh bool) ([]PostAnalyticsItem, error) {
	query := map[string]string{}
	if refresh {
		query["refresh"] = "true"
	}
	var envelope apiEnvelope[[]PostAnalyticsItem]
	if err := s.client.do(ctx, http.MethodGet, "/v1/posts/"+postID+"/analytics", query, nil, &envelope, nil); err != nil {
		return nil, err
	}
	if envelope.Data == nil {
		return []PostAnalyticsItem{}, nil
	}
	return envelope.Data, nil
}

func (s *PostsService) Create(ctx context.Context, params *CreatePostParams) (*Post, error) {
	headers := map[string]string{}
	if params != nil && params.IdempotencyKey != "" {
		headers["Idempotency-Key"] = params.IdempotencyKey
	}
	var envelope apiEnvelope[Post]
	if err := s.client.do(ctx, http.MethodPost, "/v1/posts", nil, marshalPostBody(params), &envelope, headers); err != nil {
		return nil, err
	}
	return &envelope.Data, nil
}

func (s *PostsService) Validate(ctx context.Context, params *CreatePostParams) (*ValidationResult, error) {
	var envelope apiEnvelope[ValidationResult]
	if err := s.client.do(ctx, http.MethodPost, "/v1/posts/validate", nil, marshalPostBody(params), &envelope, nil); err != nil {
		return nil, err
	}
	return &envelope.Data, nil
}

func (s *PostsService) Publish(ctx context.Context, postID string) (*Post, error) {
	var envelope apiEnvelope[Post]
	if err := s.client.do(ctx, http.MethodPost, "/v1/posts/"+postID+"/publish", nil, nil, &envelope, nil); err != nil {
		return nil, err
	}
	return &envelope.Data, nil
}

func (s *PostsService) Update(ctx context.Context, postID string, params *UpdatePostParams) (*Post, error) {
	var envelope apiEnvelope[Post]
	if err := s.client.do(ctx, http.MethodPatch, "/v1/posts/"+postID, nil, marshalUpdatePostBody(params), &envelope, nil); err != nil {
		return nil, err
	}
	return &envelope.Data, nil
}

func (s *PostsService) Archive(ctx context.Context, postID string) (*Post, error) {
	var envelope apiEnvelope[Post]
	if err := s.client.do(ctx, http.MethodPost, "/v1/posts/"+postID+"/archive", nil, nil, &envelope, nil); err != nil {
		return nil, err
	}
	return &envelope.Data, nil
}

func (s *PostsService) Restore(ctx context.Context, postID string) (*Post, error) {
	var envelope apiEnvelope[Post]
	if err := s.client.do(ctx, http.MethodPost, "/v1/posts/"+postID+"/restore", nil, nil, &envelope, nil); err != nil {
		return nil, err
	}
	return &envelope.Data, nil
}

func (s *PostsService) Cancel(ctx context.Context, postID string) (*Post, error) {
	var envelope apiEnvelope[Post]
	if err := s.client.do(ctx, http.MethodPost, "/v1/posts/"+postID+"/cancel", nil, nil, &envelope, nil); err != nil {
		return nil, err
	}
	return &envelope.Data, nil
}

func (s *PostsService) Delete(ctx context.Context, postID string) error {
	return s.client.do(ctx, http.MethodDelete, "/v1/posts/"+postID, nil, nil, nil, nil)
}

func (s *PostsService) PreviewLink(ctx context.Context, postID string) (*PostPreviewLink, error) {
	var envelope apiEnvelope[PostPreviewLink]
	if err := s.client.do(ctx, http.MethodPost, "/v1/posts/"+postID+"/preview-link", nil, nil, &envelope, nil); err != nil {
		return nil, err
	}
	return &envelope.Data, nil
}

func (s *PostsService) RetryResult(ctx context.Context, postID, resultID string) (*PlatformResult, error) {
	var envelope apiEnvelope[PlatformResult]
	if err := s.client.do(ctx, http.MethodPost, "/v1/posts/"+postID+"/results/"+resultID+"/retry", nil, nil, &envelope, nil); err != nil {
		return nil, err
	}
	return &envelope.Data, nil
}

func (s *PostsService) BulkCreate(ctx context.Context, posts []*CreatePostParams) ([]BulkPostResult, error) {
	body := make([]map[string]any, 0, len(posts))
	for _, post := range posts {
		body = append(body, marshalPostBody(post))
	}
	payload := map[string]any{"posts": body}
	var envelope apiEnvelope[[]BulkPostResult]
	if err := s.client.do(ctx, http.MethodPost, "/v1/posts/bulk", nil, payload, &envelope, nil); err != nil {
		return nil, err
	}
	return envelope.Data, nil
}

type ListDeliveryJobsParams struct {
	Limit  int
	Offset int
	States string
}

type DeliveryJobsService struct {
	client *Client
}

func (s *DeliveryJobsService) List(ctx context.Context, params *ListDeliveryJobsParams) ([]DeliveryJob, error) {
	query := map[string]string{}
	if params != nil {
		if params.Limit > 0 {
			query["limit"] = strconv.Itoa(params.Limit)
		}
		if params.Offset > 0 {
			query["offset"] = strconv.Itoa(params.Offset)
		}
		query["states"] = params.States
	}
	var envelope apiEnvelope[[]DeliveryJob]
	if err := s.client.do(ctx, http.MethodGet, "/v1/post-delivery-jobs", query, nil, &envelope, nil); err != nil {
		return nil, err
	}
	return envelope.Data, nil
}

func (s *DeliveryJobsService) Summary(ctx context.Context) (JSONMap, error) {
	var envelope apiEnvelope[JSONMap]
	if err := s.client.do(ctx, http.MethodGet, "/v1/post-delivery-jobs/summary", nil, nil, &envelope, nil); err != nil {
		return nil, err
	}
	return envelope.Data, nil
}

func (s *DeliveryJobsService) Retry(ctx context.Context, jobID string) (*DeliveryJob, error) {
	var envelope apiEnvelope[DeliveryJob]
	if err := s.client.do(ctx, http.MethodPost, "/v1/post-delivery-jobs/"+jobID+"/retry", nil, nil, &envelope, nil); err != nil {
		return nil, err
	}
	return &envelope.Data, nil
}

func (s *DeliveryJobsService) Cancel(ctx context.Context, jobID string) (*DeliveryJob, error) {
	var envelope apiEnvelope[DeliveryJob]
	if err := s.client.do(ctx, http.MethodPost, "/v1/post-delivery-jobs/"+jobID+"/cancel", nil, nil, &envelope, nil); err != nil {
		return nil, err
	}
	return &envelope.Data, nil
}

type MediaUploadRequest struct {
	Filename    string
	ContentType string
	SizeBytes   int64
	ContentHash string
}

type MediaUploadResponse struct {
	ID          string    `json:"id,omitempty"`
	MediaID     string    `json:"media_id,omitempty"`
	Status      string    `json:"status"`
	ContentType string    `json:"content_type"`
	SizeBytes   int64     `json:"size_bytes"`
	UploadURL   string    `json:"upload_url,omitempty"`
	DownloadURL string    `json:"download_url,omitempty"`
	ExpiresAt   time.Time `json:"expires_at,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
}

type MediaService struct {
	client *Client
}

func (s *MediaService) Upload(ctx context.Context, params *MediaUploadRequest) (*MediaUploadResponse, error) {
	body := compactMap(map[string]any{
		"filename":     params.Filename,
		"content_type": params.ContentType,
		"size_bytes":   params.SizeBytes,
		"content_hash": params.ContentHash,
	})
	var envelope apiEnvelope[MediaUploadResponse]
	if err := s.client.do(ctx, http.MethodPost, "/v1/media", nil, body, &envelope, nil); err != nil {
		return nil, err
	}
	if envelope.Data.MediaID == "" {
		envelope.Data.MediaID = envelope.Data.ID
	}
	return &envelope.Data, nil
}

func (s *MediaService) Get(ctx context.Context, mediaID string) (*MediaUploadResponse, error) {
	var envelope apiEnvelope[MediaUploadResponse]
	if err := s.client.do(ctx, http.MethodGet, "/v1/media/"+mediaID, nil, nil, &envelope, nil); err != nil {
		return nil, err
	}
	if envelope.Data.MediaID == "" {
		envelope.Data.MediaID = envelope.Data.ID
	}
	return &envelope.Data, nil
}

func (s *MediaService) Delete(ctx context.Context, mediaID string) error {
	return s.client.do(ctx, http.MethodDelete, "/v1/media/"+mediaID, nil, nil, nil, nil)
}

type AnalyticsQueryParams struct {
	From      string
	To        string
	ProfileID string
	Platform  string
	Status    string
}

type AnalyticsRollupParams struct {
	From        string
	To          string
	Granularity string
	GroupBy     string
}

type AnalyticsRollup struct {
	Granularity string    `json:"granularity"`
	GroupBy     []string  `json:"group_by"`
	Series      []JSONMap `json:"series"`
}

type AnalyticsService struct {
	client *Client
}

func (s *AnalyticsService) Summary(ctx context.Context, params *AnalyticsQueryParams) (JSONMap, error) {
	var envelope apiEnvelope[JSONMap]
	if err := s.client.do(ctx, http.MethodGet, "/v1/analytics/summary", queryFromAnalytics(params), nil, &envelope, nil); err != nil {
		return nil, err
	}
	return envelope.Data, nil
}

func (s *AnalyticsService) Trend(ctx context.Context, params *AnalyticsQueryParams) (JSONMap, error) {
	var envelope apiEnvelope[JSONMap]
	if err := s.client.do(ctx, http.MethodGet, "/v1/analytics/trend", queryFromAnalytics(params), nil, &envelope, nil); err != nil {
		return nil, err
	}
	return envelope.Data, nil
}

func (s *AnalyticsService) ByPlatform(ctx context.Context, params *AnalyticsQueryParams) ([]JSONMap, error) {
	var envelope apiEnvelope[[]JSONMap]
	if err := s.client.do(ctx, http.MethodGet, "/v1/analytics/by-platform", queryFromAnalytics(params), nil, &envelope, nil); err != nil {
		return nil, err
	}
	return envelope.Data, nil
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
	ID                       string     `json:"id"`
	URL                      string     `json:"url"`
	Status                   string     `json:"status"`
	ExpiresAt                time.Time  `json:"expires_at"`
	Platform                 string     `json:"platform"`
	ExternalUserID           string     `json:"external_user_id"`
	ExternalUserEmail        string     `json:"external_user_email,omitempty"`
	ReturnURL                string     `json:"return_url,omitempty"`
	CreatedAt                time.Time  `json:"created_at"`
	CompletedAt              *time.Time `json:"completed_at,omitempty"`
	CompletedSocialAccountID string     `json:"completed_social_account_id,omitempty"`
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

type PaginatedManagedUsers struct {
	Data []ManagedUser
	Meta PageMeta
}

func (s *UsersService) List(ctx context.Context) ([]ManagedUser, error) {
	page, err := s.ListPage(ctx)
	if err != nil {
		return nil, err
	}
	return page.Data, nil
}

func (s *UsersService) ListPage(ctx context.Context) (*PaginatedManagedUsers, error) {
	var envelope apiEnvelope[[]ManagedUser]
	if err := s.client.do(ctx, http.MethodGet, "/v1/users", nil, nil, &envelope, nil); err != nil {
		return nil, err
	}
	return &PaginatedManagedUsers{Data: envelope.Data, Meta: pageMetaFromEnvelope(envelope)}, nil
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
	Name          string    `json:"name"`
	URL           string    `json:"url"`
	Events        []string  `json:"events"`
	Active        bool      `json:"active"`
	Secret        string    `json:"secret,omitempty"`
	SecretPreview string    `json:"secret_preview"`
	CreatedAt     time.Time `json:"created_at"`
}

type CreateWebhookParams struct {
	Name   string   `json:"name"`
	URL    string   `json:"url"`
	Events []string `json:"events"`
	Active *bool    `json:"active,omitempty"`
	Secret string   `json:"secret,omitempty"`
}

type UpdateWebhookParams struct {
	Name   *string  `json:"name,omitempty"`
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
	return &PaginatedWebhookSubscriptions{Data: envelope.Data, Meta: pageMetaFromEnvelope(envelope)}, nil
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

type OAuthConnectResponse struct {
	AuthURL string `json:"auth_url"`
}

type OAuthService struct {
	client *Client
}

func (s *OAuthService) Connect(ctx context.Context, platform, redirectURL string) (*OAuthConnectResponse, error) {
	query := map[string]string{}
	if redirectURL != "" {
		query["redirect_url"] = redirectURL
	}
	var envelope apiEnvelope[OAuthConnectResponse]
	if err := s.client.do(ctx, http.MethodGet, "/v1/oauth/connect/"+platform, query, nil, &envelope, nil); err != nil {
		return nil, err
	}
	return &envelope.Data, nil
}

type Usage struct {
	Period     string  `json:"period"`
	PostCount  int     `json:"post_count"`
	PostLimit  int     `json:"post_limit"`
	Plan       string  `json:"plan"`
	Percentage float64 `json:"percentage"`
	Warning    string  `json:"warning,omitempty"`
}

type UsageService struct {
	client *Client
}

func (s *UsageService) Get(ctx context.Context) (*Usage, error) {
	var envelope apiEnvelope[Usage]
	if err := s.client.do(ctx, http.MethodGet, "/v1/usage", nil, nil, &envelope, nil); err != nil {
		return nil, err
	}
	return &envelope.Data, nil
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
