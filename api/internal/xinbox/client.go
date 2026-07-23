package xinbox

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	defaultXAPIBaseURL                = "https://api.x.com"
	maxStreamRules                    = 1000
	defaultStreamIdle                 = 25 * time.Second
	defaultControlTimeout             = 15 * time.Second
	defaultMaxJSONResponseBytes int64 = 1 << 20
)

var ErrStreamDisconnected = errors.New("X filtered stream disconnected")

type ClientConfig struct {
	BaseURL                  string
	HTTPClient               *http.Client
	StreamHTTPClient         *http.Client
	ControlRequestTimeout    time.Duration
	MaxJSONResponseBytes     int64
	StreamIdleTimeout        time.Duration
	WebhookValidationPolls   int
	WebhookValidationBackoff time.Duration
	Sleep                    func(context.Context, time.Duration) error
}

type Client struct {
	baseURL                  string
	controlHTTP              *http.Client
	streamHTTP               *http.Client
	controlTimeout           time.Duration
	maxJSONResponseBytes     int64
	streamIdle               time.Duration
	webhookValidationPolls   int
	webhookValidationBackoff time.Duration
	sleep                    func(context.Context, time.Duration) error
}

type StreamRule struct {
	ID    string `json:"id,omitempty"`
	Value string `json:"value"`
	Tag   string `json:"tag"`
}

type StreamEvent struct {
	Data struct {
		ID               string            `json:"id"`
		Text             string            `json:"text"`
		AuthorID         string            `json:"author_id"`
		CreatedAt        string            `json:"created_at"`
		ConversationID   string            `json:"conversation_id"`
		ReferencedTweets []ReferencedTweet `json:"referenced_tweets"`
	} `json:"data"`
	Includes      json.RawMessage `json:"includes"`
	MatchingRules []StreamRule    `json:"matching_rules"`
}

type ReferencedTweet struct {
	Type string `json:"type"`
	ID   string `json:"id"`
}

type APIError struct {
	ResourceID string `json:"resource_id"`
	Title      string `json:"title"`
	Type       string `json:"type"`
	Detail     string `json:"detail"`
	Status     int    `json:"status"`
}

type ProviderHTTPError struct {
	Method     string
	Path       string
	StatusCode int
	Code       string
	Title      string
}

func (e *ProviderHTTPError) Error() string {
	if e == nil {
		return "X provider HTTP error"
	}
	message := fmt.Sprintf("X API %s %s returned HTTP %d", e.Method, e.Path, e.StatusCode)
	if e.Code != "" {
		message += fmt.Sprintf(" code=%q", e.Code)
	}
	if e.Title != "" {
		message += fmt.Sprintf(" title=%q", e.Title)
	}
	return message
}

func IsProviderHTTPStatus(err error, status int) bool {
	var providerErr *ProviderHTTPError
	return errors.As(err, &providerErr) && providerErr != nil && providerErr.StatusCode == status
}

func NewClient(config ClientConfig) *Client {
	baseURL := strings.TrimRight(strings.TrimSpace(config.BaseURL), "/")
	if baseURL == "" {
		baseURL = defaultXAPIBaseURL
	}
	httpClient := config.HTTPClient
	if httpClient == nil {
		httpClient = &http.Client{Transport: newXTransport()}
	}
	streamHTTPClient := config.StreamHTTPClient
	if streamHTTPClient == nil {
		if config.HTTPClient != nil {
			streamHTTPClient = config.HTTPClient
		} else {
			streamHTTPClient = &http.Client{Transport: newXTransport()}
		}
	}
	controlTimeout := config.ControlRequestTimeout
	if controlTimeout <= 0 {
		controlTimeout = defaultControlTimeout
	}
	maxJSONResponseBytes := config.MaxJSONResponseBytes
	if maxJSONResponseBytes <= 0 {
		maxJSONResponseBytes = defaultMaxJSONResponseBytes
	}
	streamIdle := config.StreamIdleTimeout
	if streamIdle <= 0 {
		streamIdle = defaultStreamIdle
	}
	webhookValidationPolls := config.WebhookValidationPolls
	if webhookValidationPolls <= 0 {
		webhookValidationPolls = 5
	}
	webhookValidationBackoff := config.WebhookValidationBackoff
	if webhookValidationBackoff <= 0 {
		webhookValidationBackoff = 500 * time.Millisecond
	}
	sleep := config.Sleep
	if sleep == nil {
		sleep = sleepContext
	}
	return &Client{
		baseURL:                  baseURL,
		controlHTTP:              httpClient,
		streamHTTP:               streamHTTPClient,
		controlTimeout:           controlTimeout,
		maxJSONResponseBytes:     maxJSONResponseBytes,
		streamIdle:               streamIdle,
		webhookValidationPolls:   webhookValidationPolls,
		webhookValidationBackoff: webhookValidationBackoff,
		sleep:                    sleep,
	}
}

func FilteredStreamRuleTag(accountID string) string {
	return "unipost:x:account:" + strings.TrimSpace(accountID)
}

func FilteredStreamRuleValue(handle string) string {
	handle = strings.ToLower(strings.TrimPrefix(strings.TrimSpace(handle), "@"))
	return fmt.Sprintf("(@%s OR to:%s) -is:retweet", handle, handle)
}

func (c *Client) ListFilteredStreamRules(ctx context.Context, bearerToken string) ([]StreamRule, error) {
	var response struct {
		Data []StreamRule `json:"data"`
	}
	if err := c.doJSON(ctx, http.MethodGet, "/2/tweets/search/stream/rules", bearerToken, nil, &response); err != nil {
		return nil, err
	}
	return response.Data, nil
}

func (c *Client) EnsureFilteredStreamRule(
	ctx context.Context,
	bearerToken string,
	accountID string,
	handle string,
) (StreamRule, error) {
	rules, err := c.ListFilteredStreamRules(ctx, bearerToken)
	if err != nil {
		return StreamRule{}, err
	}
	tag := FilteredStreamRuleTag(accountID)
	value := FilteredStreamRuleValue(handle)
	replacedStaleRule := false
	for _, rule := range rules {
		if rule.Tag == tag {
			if rule.Value == value {
				return rule, nil
			}
			if err := c.DeleteFilteredStreamRule(ctx, bearerToken, rule.ID); err != nil {
				return StreamRule{}, err
			}
			replacedStaleRule = true
			break
		}
	}
	if len(rules) >= maxStreamRules && !replacedStaleRule {
		return StreamRule{}, fmt.Errorf("X filtered stream rule limit reached (%d)", maxStreamRules)
	}

	request := struct {
		Add []StreamRule `json:"add"`
	}{
		Add: []StreamRule{{
			Value: value,
			Tag:   tag,
		}},
	}
	var response struct {
		Data []StreamRule `json:"data"`
	}
	if err := c.doJSON(ctx, http.MethodPost, "/2/tweets/search/stream/rules", bearerToken, request, &response); err != nil {
		return StreamRule{}, err
	}
	if len(response.Data) != 1 || response.Data[0].ID == "" {
		return StreamRule{}, errors.New("X create filtered stream rule response missing rule id")
	}
	return response.Data[0], nil
}

func (c *Client) DeleteFilteredStreamRule(ctx context.Context, bearerToken, ruleID string) error {
	request := struct {
		Delete struct {
			IDs []string `json:"ids"`
		} `json:"delete"`
	}{}
	request.Delete.IDs = []string{ruleID}
	var response struct {
		Errors []APIError `json:"errors"`
		Meta   struct {
			Summary struct {
				Deleted    int `json:"deleted"`
				NotDeleted int `json:"not_deleted"`
			} `json:"summary"`
		} `json:"meta"`
	}
	_, err := c.do(ctx, http.MethodPost, "/2/tweets/search/stream/rules", bearerToken, request, &response)
	if err != nil {
		if IsProviderHTTPStatus(err, http.StatusNotFound) || IsProviderHTTPStatus(err, http.StatusGone) {
			return nil
		}
		return err
	}
	if len(response.Errors) > 0 {
		if allErrorsAlreadyMissing(response.Errors, ruleID) {
			return nil
		}
		return errors.New("X delete filtered stream rule returned errors without confirmed deletion")
	}
	if response.Meta.Summary.Deleted == 1 && response.Meta.Summary.NotDeleted == 0 {
		return nil
	}
	return errors.New("X delete filtered stream rule response did not confirm deleted=1 and not_deleted=0")
}

func (c *Client) ConsumeFilteredStream(
	ctx context.Context,
	bearerToken string,
	handler func(StreamEvent) error,
) error {
	streamCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	query := url.Values{
		"tweet.fields": {"id,text,author_id,created_at,conversation_id,referenced_tweets"},
		"expansions":   {"author_id,referenced_tweets.id,referenced_tweets.id.author_id"},
		"user.fields":  {"id,name,username,profile_image_url"},
	}
	path := "/2/tweets/search/stream?" + query.Encode()
	request, err := c.newRequest(streamCtx, http.MethodGet, path, bearerToken, nil)
	if err != nil {
		return err
	}
	response, err := c.streamHTTP.Do(request)
	if err != nil {
		return wrapProviderRequestError(http.MethodGet, path, "open stream failed", err)
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 4096))
		return fmt.Errorf("open X filtered stream returned HTTP %d", response.StatusCode)
	}

	type scanResult struct {
		line []byte
		err  error
		done bool
	}
	results := make(chan scanResult, 1)
	go func() {
		scanner := bufio.NewScanner(response.Body)
		scanner.Buffer(make([]byte, 64*1024), 1024*1024)
		for scanner.Scan() {
			select {
			case results <- scanResult{line: append([]byte(nil), scanner.Bytes()...)}:
			case <-streamCtx.Done():
				return
			}
		}
		select {
		case results <- scanResult{err: scanner.Err(), done: true}:
		case <-streamCtx.Done():
		}
	}()

	idle := time.NewTimer(c.streamIdle)
	defer idle.Stop()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-idle.C:
			cancel()
			_ = response.Body.Close()
			return fmt.Errorf("%w: no data or keepalive within %s", ErrStreamDisconnected, c.streamIdle)
		case result := <-results:
			if result.done {
				if result.err != nil {
					return fmt.Errorf("%w: %v", ErrStreamDisconnected, result.err)
				}
				if ctx.Err() != nil {
					return ctx.Err()
				}
				return ErrStreamDisconnected
			}
			if !idle.Stop() {
				select {
				case <-idle.C:
				default:
				}
			}
			idle.Reset(c.streamIdle)
			line := bytes.TrimSpace(result.line)
			if len(line) == 0 {
				continue
			}
			var event StreamEvent
			if err := json.Unmarshal(line, &event); err != nil {
				return fmt.Errorf("decode X filtered stream event: %w", err)
			}
			if err := handler(event); err != nil {
				return err
			}
		}
	}
}

func (c *Client) doJSON(
	ctx context.Context,
	method string,
	path string,
	bearerToken string,
	body any,
	responseBody any,
) error {
	_, err := c.do(ctx, method, path, bearerToken, body, responseBody)
	return err
}

func (c *Client) do(
	ctx context.Context,
	method string,
	path string,
	bearerToken string,
	body any,
	responseBody any,
) (int, error) {
	safePath := providerRequestPath(path)
	requestCtx, cancel := context.WithTimeout(ctx, c.controlTimeout)
	defer cancel()
	request, err := c.newRequest(requestCtx, method, path, bearerToken, body)
	if err != nil {
		return 0, err
	}
	response, err := c.controlHTTP.Do(request)
	if err != nil {
		return 0, wrapProviderRequestError(method, safePath, "request failed", err)
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		providerErr := &ProviderHTTPError{
			Method:     method,
			Path:       safePath,
			StatusCode: response.StatusCode,
		}
		decodeProviderHTTPError(response.Body, c.maxJSONResponseBytes, providerErr)
		return response.StatusCode, providerErr
	}
	if responseBody != nil {
		limited := &io.LimitedReader{R: response.Body, N: c.maxJSONResponseBytes + 1}
		payload, readErr := io.ReadAll(limited)
		if readErr != nil {
			return response.StatusCode, fmt.Errorf("read X API %s %s response: %w", method, safePath, readErr)
		}
		if int64(len(payload)) > c.maxJSONResponseBytes {
			return response.StatusCode, fmt.Errorf(
				"X API %s %s response exceeded %d bytes",
				method,
				safePath,
				c.maxJSONResponseBytes,
			)
		}
		if err := json.Unmarshal(payload, responseBody); err != nil && len(bytes.TrimSpace(payload)) != 0 {
			return response.StatusCode, fmt.Errorf("decode X API %s %s response: %w", method, safePath, err)
		}
	} else {
		_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 4096))
	}
	return response.StatusCode, nil
}

func providerRequestPath(path string) string {
	if parsed, err := url.Parse(path); err == nil && parsed.Path != "" {
		return parsed.EscapedPath()
	}
	path, _, _ = strings.Cut(path, "?")
	path, _, _ = strings.Cut(path, "#")
	return path
}

func wrapProviderRequestError(method, path, operation string, cause error) error {
	for {
		urlErr, ok := cause.(*url.Error)
		if !ok || urlErr == nil || urlErr.Err == nil {
			break
		}
		cause = urlErr.Err
	}
	return fmt.Errorf("X API %s %s %s: %w", method, providerRequestPath(path), operation, cause)
}

func decodeProviderHTTPError(body io.Reader, limit int64, target *ProviderHTTPError) {
	type providerError struct {
		Code   json.RawMessage `json:"code"`
		Type   string          `json:"type"`
		Title  string          `json:"title"`
		Status int             `json:"status"`
	}
	type providerErrorResponse struct {
		Errors [1]providerError `json:"errors"`
		providerError
	}

	var response providerErrorResponse
	decoder := json.NewDecoder(io.LimitReader(body, limit))
	if err := decoder.Decode(&response); err != nil {
		return
	}
	providerErr := response.providerError
	if first := response.Errors[0]; len(first.Code) > 0 || first.Type != "" || first.Title != "" || first.Status != 0 {
		providerErr = response.Errors[0]
	}
	target.Code = providerErrorCode(providerErr.Code)
	if target.Code == "" {
		target.Code = providerErr.Type
	}
	target.Title = providerErr.Title
}

func providerErrorCode(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var code string
	if err := json.Unmarshal(raw, &code); err == nil {
		return code
	}
	var number json.Number
	if err := json.Unmarshal(raw, &number); err == nil {
		return number.String()
	}
	return ""
}

func (c *Client) newRequest(
	ctx context.Context,
	method string,
	path string,
	bearerToken string,
	body any,
) (*http.Request, error) {
	var reader io.Reader
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			return nil, wrapProviderRequestError(method, path, "encode request failed", err)
		}
		reader = bytes.NewReader(encoded)
	}
	request, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, reader)
	if err != nil {
		return nil, wrapProviderRequestError(method, path, "create request failed", err)
	}
	request.Header.Set("Authorization", "Bearer "+bearerToken)
	if body != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	return request, nil
}

func newXTransport() *http.Transport {
	return &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   5 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          100,
		MaxIdleConnsPerHost:   10,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   5 * time.Second,
		ResponseHeaderTimeout: 10 * time.Second,
		ExpectContinueTimeout: time.Second,
	}
}

func allErrorsAlreadyMissing(apiErrors []APIError, resourceID string) bool {
	if len(apiErrors) == 0 {
		return false
	}
	for _, apiErr := range apiErrors {
		errorType := strings.ToLower(apiErr.Type)
		title := strings.ToLower(apiErr.Title)
		if !strings.Contains(errorType, "resource-not-found") &&
			!strings.Contains(title, "not found") &&
			apiErr.Status != http.StatusNotFound &&
			apiErr.Status != http.StatusGone {
			return false
		}
		if apiErr.ResourceID != "" && apiErr.ResourceID != resourceID {
			return false
		}
	}
	return true
}
