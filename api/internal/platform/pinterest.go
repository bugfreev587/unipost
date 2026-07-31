package platform

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/xiaoboyu/unipost-api/internal/debugrt"
	"github.com/xiaoboyu/unipost-api/internal/safefetch"
	"github.com/xiaoboyu/unipost-api/internal/storage"
)

const (
	pinterestOAuthEndpoint  = "https://www.pinterest.com/oauth/"
	pinterestTokenEndpoint  = "https://api.pinterest.com/v5/oauth/token"
	pinterestAPIBase        = "https://api.pinterest.com/v5"
	pinterestSandboxAPIBase = "https://api-sandbox.pinterest.com/v5"
)

var pinterestScopes = []string{
	"boards:read",
	"boards:write",
	"pins:read",
	"pins:write",
	"user_accounts:read",
}

type PinterestMediaStager interface {
	StageExternalMedia(context.Context, string, safefetch.Policy) (storage.ExternalMediaResult, error)
}

type PinterestAdapter struct {
	client      *http.Client
	mediaStager PinterestMediaStager
	boardCache  pinterestBoardCache
	now         func() time.Time
}

type PinterestBoard struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type pinterestPinAnalyticsMetrics struct {
	SummaryMetrics  map[string]float64 `json:"summary_metrics"`
	LifetimeMetrics map[string]int64   `json:"lifetime_metrics"`
}

func NewPinterestAdapter() *PinterestAdapter {
	return &PinterestAdapter{client: debugrt.NewClient(60 * time.Second)}
}

// SetMediaProxy attaches the verified external-media staging boundary.
// Request-supplied URLs must pass the provider-neutral safe fetcher before
// Pinterest receives a UniPost-controlled public URL.
func (a *PinterestAdapter) SetMediaProxy(c *storage.Client) {
	a.mediaStager = c
}

func (a *PinterestAdapter) Platform() string { return "pinterest" }

func PinterestEnvironment() string {
	if pinterestUseSandbox() {
		return "sandbox"
	}
	return "production"
}

func (a *PinterestAdapter) DefaultOAuthConfig(baseRedirectURL string) OAuthConfig {
	clientID := os.Getenv("PINTEREST_APP_ID")
	if clientID == "" {
		clientID = os.Getenv("PINTEREST_CLIENT_ID")
	}
	clientSecret := os.Getenv("PINTEREST_APP_SECRET")
	if clientSecret == "" {
		clientSecret = os.Getenv("PINTEREST_CLIENT_SECRET")
	}
	return OAuthConfig{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		AuthURL:      pinterestAuthURL(),
		TokenURL:     pinterestTokenURL(),
		RedirectURL:  strings.TrimRight(baseRedirectURL, "/") + "/v1/oauth/callback/pinterest",
		Scopes:       pinterestScopes,
	}
}

func (a *PinterestAdapter) GetAuthURL(config OAuthConfig, state string) string {
	q := url.Values{}
	q.Set("client_id", config.ClientID)
	q.Set("redirect_uri", config.RedirectURL)
	q.Set("response_type", "code")
	q.Set("refreshable", "true")
	q.Set("scope", strings.Join(config.Scopes, ","))
	q.Set("state", state)
	return config.AuthURL + "?" + q.Encode()
}

func (a *PinterestAdapter) ExchangeCode(ctx context.Context, config OAuthConfig, code string) (*ConnectResult, error) {
	form := url.Values{}
	form.Set("grant_type", "authorization_code")
	form.Set("code", code)
	form.Set("redirect_uri", config.RedirectURL)

	req, err := http.NewRequestWithContext(ctx, "POST", config.TokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte(config.ClientID+":"+config.ClientSecret)))

	resp, err := a.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("pinterest token exchange: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("pinterest token exchange (%d): %s", resp.StatusCode, string(body))
	}

	var tokenResp struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		ExpiresIn    int    `json:"expires_in"`
		Scope        string `json:"scope"`
	}
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return nil, fmt.Errorf("pinterest token exchange decode: %w", err)
	}
	if tokenResp.AccessToken == "" {
		return nil, fmt.Errorf("pinterest token exchange returned empty access_token")
	}

	profile, err := a.fetchUserAccount(ctx, tokenResp.AccessToken, "account_validation")
	if err != nil {
		return nil, err
	}

	expiresAt := time.Now().Add(time.Duration(tokenResp.ExpiresIn) * time.Second)
	if tokenResp.ExpiresIn == 0 {
		expiresAt = time.Now().Add(30 * 24 * time.Hour)
	}
	scopes := splitPinterestOAuthScopes(tokenResp.Scope)
	if len(scopes) == 0 {
		scopes = append([]string(nil), config.Scopes...)
	}

	return &ConnectResult{
		AccessToken:       tokenResp.AccessToken,
		RefreshToken:      tokenResp.RefreshToken,
		TokenExpiresAt:    expiresAt,
		ExternalAccountID: profile.ID,
		AccountName:       firstNonEmptyString(profile.Username, profile.AccountType, profile.ID),
		AvatarURL:         profile.ProfileImage,
		Scopes:            scopes,
		Metadata: map[string]any{
			"username":     profile.Username,
			"account_type": profile.AccountType,
		},
	}, nil
}

func (a *PinterestAdapter) Connect(ctx context.Context, credentials map[string]string) (*ConnectResult, error) {
	return nil, fmt.Errorf("pinterest requires OAuth flow, use POST /v1/oauth/connect")
}

func (a *PinterestAdapter) Post(ctx context.Context, accessToken string, text string, media []MediaItem, opts map[string]any) (*PostResult, error) {
	if len(media) != 1 {
		return nil, fmt.Errorf("pinterest requires exactly one image or video")
	}

	boardID := strings.TrimSpace(optString(opts, "board_id"))
	if boardID == "" {
		boardID = strings.TrimSpace(optString(opts, "boardId"))
	}
	if boardID == "" {
		return nil, fmt.Errorf("pinterest requires platform_options.board_id")
	}
	if !pinterestBoardIDPattern.MatchString(boardID) {
		return nil, fmt.Errorf("invalid pinterest platform_options.board_id: must be a numeric board ID")
	}
	preflightStartedAt := a.pinterestNow()
	pinterestRecordDispatchEvent(ctx, DispatchEvent{
		Name:             "pinterest_destination_preflight_started",
		Status:           "started",
		Environment:      PinterestEnvironment(),
		BoardFingerprint: pinterestBoardFingerprint(boardID),
		FailureStage:     "destination_preflight",
	})
	if metadata, ok := DispatchMetadataFromContext(ctx); ok &&
		metadata.Environment != "" && metadata.Environment != PinterestEnvironment() {
		err := newProviderFailure(
			"The selected Pinterest board belongs to a different Pinterest environment. Choose another board and publish again.",
			map[string]any{
				"provider": "pinterest",
				"reason":   "board_environment_mismatch",
			},
			FailureContract{
				ErrorCode:   "target_not_found",
				Stage:       "destination_preflight",
				IsRetriable: false,
			},
		)
		pinterestRecordDispatchFailure(ctx, "pinterest_destination_preflight_failed", boardID, a.pinterestNow().Sub(preflightStartedAt), err)
		return nil, err
	}
	if err := a.preflightBoard(ctx, accessToken, boardID); err != nil {
		pinterestRecordDispatchFailure(ctx, "pinterest_destination_preflight_failed", boardID, a.pinterestNow().Sub(preflightStartedAt), err)
		return nil, err
	}
	pinterestRecordDispatchEvent(ctx, DispatchEvent{
		Name:             "pinterest_destination_preflight_succeeded",
		Status:           "succeeded",
		Environment:      PinterestEnvironment(),
		BoardFingerprint: pinterestBoardFingerprint(boardID),
		FailureStage:     "destination_preflight",
		Duration:         a.pinterestNow().Sub(preflightStartedAt),
	})

	title := strings.TrimSpace(optString(opts, "title"))
	link := strings.TrimSpace(optString(opts, "link"))
	item := media[0]
	kind := item.Kind
	if kind == MediaKindUnknown {
		kind = SniffMediaKind(item.URL)
	}
	sourceURL := item.URL
	if item.Origin != MediaOriginManaged {
		mediaStartedAt := a.pinterestNow()
		var staged storage.ExternalMediaResult
		var err error
		if a.mediaStager == nil {
			err = storage.ErrNotConfigured
		} else {
			staged, err = a.mediaStager.StageExternalMedia(ctx, sourceURL, pinterestExternalMediaPolicy(kind))
		}
		if err != nil {
			failure := pinterestMediaFailure(err)
			pinterestRecordDispatchFailure(ctx, "pinterest_media_preflight_failed", boardID, a.pinterestNow().Sub(mediaStartedAt), failure)
			return nil, failure
		}
		if strings.TrimSpace(staged.PublicURL) == "" {
			failure := pinterestMediaFailure(storage.ErrExternalMediaUpload)
			pinterestRecordDispatchFailure(ctx, "pinterest_media_preflight_failed", boardID, a.pinterestNow().Sub(mediaStartedAt), failure)
			return nil, failure
		}
		sourceURL = staged.PublicURL
		if strings.HasPrefix(strings.ToLower(staged.MediaType), "video/") {
			kind = MediaKindVideo
		} else {
			kind = MediaKindImage
		}
		pinterestRecordDispatchEvent(ctx, DispatchEvent{
			Name:             "pinterest_media_staged",
			Status:           "succeeded",
			Environment:      PinterestEnvironment(),
			BoardFingerprint: pinterestBoardFingerprint(boardID),
			FailureStage:     "media_preflight",
			Duration:         a.pinterestNow().Sub(mediaStartedAt),
			MediaType:        staged.MediaType,
			MediaSizeBytes:   staged.SizeBytes,
		})
	}

	reqBody := map[string]any{
		"board_id":    boardID,
		"title":       title,
		"description": text,
	}
	if link != "" {
		reqBody["link"] = link
	}
	switch kind {
	case MediaKindVideo:
		reqBody["media_source"] = map[string]any{
			"source_type": "video_url",
			"url":         sourceURL,
		}
	default:
		reqBody["media_source"] = map[string]any{
			"source_type": "image_url",
			"url":         sourceURL,
		}
	}

	payload, _ := json.Marshal(reqBody)
	req, err := http.NewRequestWithContext(ctx, "POST", pinterestAPIBaseURL()+"/pins", bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err := a.client.Do(req)
	if err != nil {
		failure := pinterestTransportFailure("create pin", err, "dispatch")
		pinterestRecordDispatchFailure(ctx, "pinterest_create_pin_failed", boardID, 0, failure)
		return nil, failure
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		failure := pinterestProviderFailure("create pin", resp.StatusCode, body, "dispatch")
		pinterestRecordDispatchFailure(ctx, "pinterest_create_pin_failed", boardID, 0, failure)
		return nil, failure
	}

	var pin struct {
		ID  string `json:"id"`
		URL string `json:"url"`
	}
	if err := json.Unmarshal(body, &pin); err != nil {
		return nil, fmt.Errorf("pinterest create pin decode: %w", err)
	}
	if pin.URL == "" && pin.ID != "" {
		pin.URL = "https://www.pinterest.com/pin/" + pin.ID + "/"
	}
	return &PostResult{
		ExternalID: pin.ID,
		URL:        pin.URL,
	}, nil
}

type pinterestAPIErrorBody struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func pinterestProviderFailure(operation string, status int, body []byte, stage string) error {
	var providerBody pinterestAPIErrorBody
	_ = json.Unmarshal(body, &providerBody)

	providerCode := ""
	if providerBody.Code != 0 {
		providerCode = strconv.Itoa(providerBody.Code)
	}
	reason := "unknown"
	errorCode := "platform_error"
	message := "Pinterest rejected the request. Contact support if the problem continues."
	isTransient := false
	isRetriable := false

	operation = strings.ToLower(strings.TrimSpace(operation))
	switch {
	case status == http.StatusUnauthorized || providerBody.Code == 2:
		reason = "token_invalid"
		errorCode = "auth_token_invalid"
		message = "Pinterest rejected this connection. Reconnect the account, then try again."
	case status == http.StatusForbidden && (operation == "board collection" || operation == "create board" || operation == "user account"):
		reason = "missing_permission"
		errorCode = "missing_permission"
		message = "Pinterest denied a required account permission. Reconnect the account and update its permissions."
	case providerBody.Code == 29:
		reason = "board_not_accessible"
		errorCode = "target_not_found"
		message = "The selected Pinterest board is unavailable for this connected account."
		if operation == "create pin" {
			stage = "destination_preflight"
		}
	case providerBody.Code == 40:
		reason = "board_not_found"
		errorCode = "target_not_found"
		message = "The selected Pinterest board is unavailable for this connected account."
		if operation == "create pin" {
			stage = "destination_preflight"
		}
	case status == http.StatusTooManyRequests:
		reason = "rate_limited"
		errorCode = "rate_limit"
		message = "Pinterest is rate limiting this request. Try again later."
		isTransient = true
		isRetriable = true
	case status >= http.StatusInternalServerError:
		reason = "provider_temporary_failure"
		errorCode = "temporary_platform_error"
		message = "Pinterest is temporarily unavailable. Try again later."
		isTransient = true
		isRetriable = true
	}

	return newProviderFailure(message, map[string]any{
		"provider":     "pinterest",
		"http_status":  status,
		"code":         providerCode,
		"reason":       reason,
		"is_transient": isTransient,
	}, FailureContract{
		ErrorCode:   errorCode,
		Stage:       strings.TrimSpace(stage),
		IsRetriable: isRetriable,
	})
}

func pinterestTransportFailure(_ string, _ error, stage string) error {
	return newProviderFailure(
		"Pinterest is temporarily unavailable. Try again later.",
		map[string]any{
			"provider":     "pinterest",
			"reason":       "provider_temporary_failure",
			"is_transient": true,
		},
		FailureContract{
			ErrorCode:   "temporary_platform_error",
			Stage:       strings.TrimSpace(stage),
			IsRetriable: true,
		},
	)
}

func pinterestExternalMediaPolicy(kind MediaKind) safefetch.Policy {
	capability := Capabilities["pinterest"].Media
	policy := safefetch.Policy{MaxRedirects: 5}
	formats := capability.Images.AllowedFormats
	policy.MaxBytes = capability.Images.MaxFileSizeBytes
	if kind == MediaKindVideo {
		formats = capability.Videos.AllowedFormats
		policy.MaxBytes = capability.Videos.MaxFileSizeBytes
	}
	policy.AllowedMediaTypes = pinterestMediaTypesForFormats(formats)
	return policy
}

func pinterestMediaTypesForFormats(formats []string) []string {
	result := make([]string, 0, len(formats))
	seen := make(map[string]struct{}, len(formats))
	for _, format := range formats {
		var mediaType string
		switch strings.ToLower(strings.TrimSpace(format)) {
		case "jpg", "jpeg":
			mediaType = "image/jpeg"
		case "png":
			mediaType = "image/png"
		case "webp":
			mediaType = "image/webp"
		case "mp4":
			mediaType = "video/mp4"
		case "mov":
			mediaType = "video/quicktime"
		}
		if mediaType == "" {
			continue
		}
		if _, ok := seen[mediaType]; ok {
			continue
		}
		seen[mediaType] = struct{}{}
		result = append(result, mediaType)
	}
	return result
}

func pinterestMediaFailure(stageErr error) error {
	errorCode := "temporary_platform_error"
	reason := "media_stage_temporary"
	message := "UniPost could not prepare this Pinterest media. Try again later."
	retriable := true
	customerInput := false

	var fetchErr *safefetch.FetchError
	if errors.As(stageErr, &fetchErr) {
		switch fetchErr.Kind {
		case safefetch.ErrorTooLarge:
			errorCode = "media_error"
			reason = "media_too_large"
			retriable = false
			customerInput = true
		case safefetch.ErrorUnsupportedMedia:
			errorCode = "media_error"
			reason = "unsupported_media"
			retriable = false
			customerInput = true
		case safefetch.ErrorSourceTemporary, safefetch.ErrorTimeout:
		default:
			errorCode = "media_error"
			reason = "customer_media_unreachable"
			retriable = false
			customerInput = true
		}
	}
	if !retriable {
		message = "Pinterest could not use this media. Replace it with a publicly available supported image or video."
	}
	fields := map[string]any{
		"provider":       "pinterest",
		"reason":         reason,
		"is_transient":   retriable,
		"customer_input": customerInput,
	}
	if fetchErr != nil && fetchErr.HTTPStatus > 0 {
		fields["http_status"] = fetchErr.HTTPStatus
	}
	return newProviderFailure(message, fields, FailureContract{ErrorCode: errorCode, Stage: "media_preflight", IsRetriable: retriable})
}

func (a *PinterestAdapter) DeletePost(ctx context.Context, accessToken string, externalID string) error {
	req, err := http.NewRequestWithContext(ctx, "DELETE", pinterestAPIBaseURL()+"/pins/"+url.PathEscape(externalID), nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err := a.client.Do(req)
	if err != nil {
		return fmt.Errorf("pinterest delete pin: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("pinterest delete pin (%d): %s", resp.StatusCode, string(body))
	}
	return nil
}

func (a *PinterestAdapter) CreateBoard(ctx context.Context, accessToken string, name string) (*PinterestBoard, error) {
	reqBody := map[string]any{
		"name": strings.TrimSpace(name),
	}
	payload, _ := json.Marshal(reqBody)

	req, err := http.NewRequestWithContext(ctx, "POST", pinterestAPIBaseURL()+"/boards", bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err := a.client.Do(req)
	if err != nil {
		return nil, pinterestTransportFailure("create board", err, "destination_discovery")
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return nil, pinterestProviderFailure("create board", resp.StatusCode, body, "destination_discovery")
	}

	var board PinterestBoard
	if err := json.Unmarshal(body, &board); err != nil {
		return nil, fmt.Errorf("pinterest create board decode: %w", err)
	}
	if board.ID == "" {
		return nil, fmt.Errorf("pinterest create board returned empty id")
	}
	return &board, nil
}

func (a *PinterestAdapter) RefreshToken(ctx context.Context, refreshToken string) (newAccess, newRefresh string, expiresAt time.Time, err error) {
	clientID := os.Getenv("PINTEREST_APP_ID")
	if clientID == "" {
		clientID = os.Getenv("PINTEREST_CLIENT_ID")
	}
	clientSecret := os.Getenv("PINTEREST_APP_SECRET")
	if clientSecret == "" {
		clientSecret = os.Getenv("PINTEREST_CLIENT_SECRET")
	}
	if clientID == "" || clientSecret == "" {
		return "", "", time.Time{}, fmt.Errorf("pinterest client credentials not configured")
	}

	form := url.Values{}
	form.Set("grant_type", "refresh_token")
	form.Set("refresh_token", refreshToken)

	req, err := http.NewRequestWithContext(ctx, "POST", pinterestTokenURL(), strings.NewReader(form.Encode()))
	if err != nil {
		return "", "", time.Time{}, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte(clientID+":"+clientSecret)))

	resp, err := a.client.Do(req)
	if err != nil {
		return "", "", time.Time{}, fmt.Errorf("pinterest refresh token: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return "", "", time.Time{}, fmt.Errorf("pinterest refresh token (%d): %s", resp.StatusCode, string(body))
	}

	var tokenResp struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		ExpiresIn    int    `json:"expires_in"`
	}
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return "", "", time.Time{}, fmt.Errorf("pinterest refresh token decode: %w", err)
	}
	if tokenResp.AccessToken == "" {
		return "", "", time.Time{}, fmt.Errorf("pinterest refresh token returned empty access_token")
	}
	if tokenResp.RefreshToken == "" {
		tokenResp.RefreshToken = refreshToken
	}

	expiresAt = time.Now().Add(time.Duration(tokenResp.ExpiresIn) * time.Second)
	if tokenResp.ExpiresIn == 0 {
		expiresAt = time.Now().Add(30 * 24 * time.Hour)
	}
	return tokenResp.AccessToken, tokenResp.RefreshToken, expiresAt, nil
}

func (a *PinterestAdapter) FetchBoards(ctx context.Context, accessToken string) ([]PinterestBoard, error) {
	account, err := a.fetchUserAccount(ctx, accessToken, "destination_discovery")
	if err != nil {
		return nil, err
	}
	resources, complete, _, err := a.walkOwnedPinterestBoards(ctx, accessToken, account.Username, "")
	if err != nil {
		return nil, err
	}
	if !complete {
		return nil, pinterestBoardProofTemporaryFailure("collection_incomplete")
	}
	boards := make([]PinterestBoard, 0, len(resources))
	ownedIDs := make(map[string]struct{}, len(resources))
	for _, board := range resources {
		boards = append(boards, PinterestBoard{ID: board.ID, Name: board.Name})
		ownedIDs[board.ID] = struct{}{}
	}
	metadata, _ := DispatchMetadataFromContext(ctx)
	a.storePinterestBoardCache(pinterestBoardCacheKey{
		AccountID:   strings.TrimSpace(metadata.SocialAccountID),
		Environment: PinterestEnvironment(),
	}, pinterestBoardCacheEntry{
		TokenFingerprint: pinterestTokenFingerprint(accessToken),
		Username:         strings.TrimSpace(account.Username),
		OwnedIDs:         ownedIDs,
		Complete:         true,
		ExpiresAt:        a.pinterestNow().Add(pinterestBoardCacheTTL),
	})
	return boards, nil
}

func (a *PinterestAdapter) GetAnalytics(ctx context.Context, accessToken string, externalID string) (*PostMetrics, error) {
	if pinterestUseSandbox() {
		return nil, fmt.Errorf("pinterest analytics endpoint is disabled in API sandbox; analytics require production access")
	}

	q := url.Values{}
	q.Set("start_date", time.Now().UTC().AddDate(0, 0, -90).Format("2006-01-02"))
	q.Set("end_date", time.Now().UTC().Format("2006-01-02"))
	q.Set("app_types", "ALL")
	q.Set("metric_types", strings.Join([]string{
		"IMPRESSION",
		"OUTBOUND_CLICK",
		"SAVE",
		"TOTAL_COMMENTS",
		"TOTAL_REACTIONS",
	}, ","))

	req, err := http.NewRequestWithContext(ctx, "GET", pinterestAPIBaseURL()+"/pins/"+url.PathEscape(externalID)+"/analytics?"+q.Encode(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err := a.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("pinterest pin analytics: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("pinterest pin analytics (%d): %s", resp.StatusCode, string(body))
	}

	var raw map[string]pinterestPinAnalyticsMetrics
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("pinterest pin analytics decode: %w", err)
	}
	if len(raw) == 0 {
		return &PostMetrics{}, nil
	}

	var bucket pinterestPinAnalyticsMetrics
	for _, metrics := range raw {
		bucket = metrics
		break
	}

	return &PostMetrics{
		Impressions: int64(bucket.SummaryMetrics["IMPRESSION"]),
		Clicks:      int64(bucket.SummaryMetrics["OUTBOUND_CLICK"]),
		Saves:       int64(bucket.SummaryMetrics["SAVE"]),
		Likes:       bucket.LifetimeMetrics["TOTAL_REACTIONS"],
		Comments:    bucket.LifetimeMetrics["TOTAL_COMMENTS"],
	}, nil
}

type pinterestUserAccount struct {
	ID           string `json:"id"`
	Username     string `json:"username"`
	AccountType  string `json:"account_type"`
	ProfileImage string `json:"profile_image"`
}

func (a *PinterestAdapter) fetchUserAccount(ctx context.Context, accessToken, stage string) (*pinterestUserAccount, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", pinterestAPIBaseURL()+"/user_account", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err := a.client.Do(req)
	if err != nil {
		return nil, pinterestTransportFailure("user account", err, stage)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, pinterestProviderFailure("user account", resp.StatusCode, body, stage)
	}

	var raw pinterestUserAccount
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, pinterestTransportFailure("user account", err, stage)
	}
	if raw.ID == "" {
		return nil, pinterestTransportFailure("user account", fmt.Errorf("empty account id"), stage)
	}
	return &raw, nil
}

func firstNonEmptyString(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

func splitPinterestOAuthScopes(scope string) []string {
	parts := strings.FieldsFunc(scope, func(r rune) bool {
		return r == ',' || r == ' '
	})
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		if s := strings.TrimSpace(part); s != "" {
			out = append(out, s)
		}
	}
	return out
}

func pinterestAuthURL() string {
	if v := strings.TrimSpace(os.Getenv("PINTEREST_AUTH_URL")); v != "" {
		return v
	}
	return pinterestOAuthEndpoint
}

func pinterestTokenURL() string {
	if v := strings.TrimSpace(os.Getenv("PINTEREST_TOKEN_URL")); v != "" {
		return v
	}
	if pinterestUseSandbox() {
		return pinterestSandboxAPIBase + "/oauth/token"
	}
	return pinterestTokenEndpoint
}

func pinterestAPIBaseURL() string {
	if v := strings.TrimSpace(os.Getenv("PINTEREST_API_BASE_URL")); v != "" {
		return strings.TrimRight(v, "/")
	}
	if pinterestUseSandbox() {
		return pinterestSandboxAPIBase
	}
	return pinterestAPIBase
}

func pinterestUseSandbox() bool {
	v := strings.TrimSpace(os.Getenv("PINTEREST_USE_SANDBOX"))
	switch strings.ToLower(v) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

func PinterestUsesSandbox() bool {
	return pinterestUseSandbox()
}
