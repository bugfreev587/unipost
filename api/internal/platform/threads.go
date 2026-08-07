package platform

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/xiaoboyu/unipost-api/internal/debugrt"
	"github.com/xiaoboyu/unipost-api/internal/integrationlogs"
)

const minThreadsLongLivedTokenLifetime = 24 * time.Hour

// Threads container readiness polling. Meta's guidance is "once per minute for
// no more than 5 minutes"; we poll faster because the delivery worker is
// already holding a slot and a shorter tail is worth the extra cheap GETs.
// Package vars, not constants, so tests can run the state machine without
// real delays. Not deployment configuration and not a feature flag.
var (
	threadsContainerPollInterval = 5 * time.Second
	threadsContainerPollBudget   = 5 * time.Minute
	// A container that answers 200 but never exposes a recognizable status
	// must not burn the whole budget. Text and image containers publish fine
	// today with a fixed sleep; if they turn out not to report a documented
	// status, stalling them for five minutes and then failing would turn a
	// video-only defect into a platform-wide outage. After this many
	// consecutive unrecognized-but-OK responses we treat the container as
	// ready, which reproduces today's behaviour for those types while
	// keeping strict gating for video, where IN_PROGRESS is actually seen.
	threadsUnknownStatusGraceAttempts = 3
)

// Threads container statuses, per Meta's troubleshooting guide.
const (
	threadsStatusFinished   = "FINISHED"
	threadsStatusInProgress = "IN_PROGRESS"
	threadsStatusError      = "ERROR"
	threadsStatusExpired    = "EXPIRED"
	threadsStatusPublished  = "PUBLISHED"
)

// threadsPermanentMediaErrors are the container error_message values that
// describe a stable defect in the customer's media. Retrying an unchanged
// request cannot fix these, so they become permanent media_error.
var threadsPermanentMediaErrors = map[string]bool{
	"INVALID_ASPEC_RATIO":          true,
	"INVALID_ASPECT_RATIO":         true, // defensive: Meta's own spelling is the typo above
	"INVALID_BIT_RATE":             true,
	"INVALID_DURATION":             true,
	"INVALID_FRAME_RATE":           true,
	"INVALID_AUDIO_CHANNELS":       true,
	"INVALID_AUDIO_CHANNEL_LAYOUT": true,
}

// threadsContainerOutcome is the resolved end state of a readiness wait.
type threadsContainerOutcome struct {
	status       string
	errorMessage string
	// alreadyPublished is set when the resumed container reports PUBLISHED,
	// meaning a prior attempt's publish landed even though we never saw it.
	alreadyPublished bool
}

// threadsContainerDiagnostics carries sanitized, credential-free context for
// terminal container failures. It never holds the access token or the full
// request URL, both of which would leak the token as a query parameter.
type threadsContainerDiagnostics struct {
	containerID    string
	pollCount      int
	elapsed        time.Duration
	lastHTTPStatus int
	status         string
	errorMessage   string
	responseBody   string
	transportError string
	mediaOrigin    MediaOrigin
}

func (d threadsContainerDiagnostics) format() string {
	parts := []string{
		"container_id=" + d.containerID,
		fmt.Sprintf("poll_count=%d", d.pollCount),
		fmt.Sprintf("elapsed_ms=%d", d.elapsed.Milliseconds()),
	}
	if d.lastHTTPStatus != 0 {
		parts = append(parts, fmt.Sprintf("last_http_status=%d", d.lastHTTPStatus))
	}
	if d.status != "" {
		parts = append(parts, "status="+d.status)
	}
	if d.errorMessage != "" {
		parts = append(parts, "error_message="+d.errorMessage)
	}
	if d.mediaOrigin != "" {
		parts = append(parts, "media_origin="+string(d.mediaOrigin))
	}
	if d.transportError != "" {
		parts = append(parts, "last_transport_error="+truncateForLog(d.transportError, 200))
	}
	if d.responseBody != "" {
		parts = append(parts, "response_body="+truncateForLog(d.responseBody, 1000))
	}
	return strings.Join(parts, " ")
}

// waitForContainer polls a Threads container until it is publishable, fails,
// or the budget runs out. Only FINISHED — or the bounded grace above —
// authorizes threads_publish.
func (a *ThreadsAdapter) waitForContainer(ctx context.Context, accessToken, containerID string, origin MediaOrigin) (threadsContainerOutcome, error) {
	return a.waitForContainerUntil(ctx, accessToken, containerID, origin, time.Now().Add(threadsContainerPollBudget))
}

// waitForContainerUntil is waitForContainer with a caller-supplied deadline so
// a set of containers can share one budget instead of each getting a fresh
// one. Carousel children need this: N children with independent budgets would
// allow N times the intended worst-case wait.
func (a *ThreadsAdapter) waitForContainerUntil(ctx context.Context, accessToken, containerID string, origin MediaOrigin, deadline time.Time) (outcome threadsContainerOutcome, err error) {
	start := time.Now()
	diag := threadsContainerDiagnostics{containerID: containerID, mediaOrigin: origin}
	unknownStatusStreak := 0

	// One summary event per wait, never one per poll: a five-minute wait at a
	// five-second interval is up to sixty polls, which would overflow the
	// sixteen-event dispatch buffer and silently drop the terminal event.
	defer func() {
		recordThreadsContainerEvent(ctx, diag, outcome, err, time.Since(start), origin)
	}()

	for {
		diag.pollCount++
		diag.elapsed = time.Since(start)

		status, body, httpStatus, err := a.fetchContainerStatus(ctx, accessToken, containerID)
		diag.lastHTTPStatus = httpStatus
		if body != "" {
			diag.responseBody = body
		}
		switch {
		case err != nil && ctx.Err() != nil:
			return threadsContainerOutcome{}, ctx.Err()
		case err != nil:
			diag.transportError = err.Error()
			// A single failed poll is not a verdict: an auth error or a
			// definitive 4xx is surfaced by fetchContainerStatus as a typed
			// error, so anything reaching here is worth another attempt
			// within the budget.
			var typed threadsPollFatal
			if errors.As(err, &typed) {
				return threadsContainerOutcome{}, typed.err
			}
		default:
			diag.transportError = ""
			diag.status = status.Status
			diag.errorMessage = strings.ToUpper(strings.TrimSpace(status.ErrorMessage))

			switch strings.ToUpper(strings.TrimSpace(status.Status)) {
			case threadsStatusFinished:
				return threadsContainerOutcome{status: threadsStatusFinished}, nil
			case threadsStatusPublished:
				return threadsContainerOutcome{status: threadsStatusPublished, alreadyPublished: true}, nil
			case threadsStatusInProgress:
				unknownStatusStreak = 0
			case threadsStatusError:
				return threadsContainerOutcome{}, a.containerErrorFailure(diag)
			case threadsStatusExpired:
				return threadsContainerOutcome{}, newProviderFailure(
					"Threads could not publish this post in time. It will be retried.",
					map[string]any{"provider": "meta", "reason": threadsStatusExpired},
					FailureContract{ErrorCode: "temporary_platform_error", Stage: "container_processing", IsRetriable: true},
				)
			default:
				// 200 OK but no status we recognize.
				unknownStatusStreak++
				if unknownStatusStreak >= threadsUnknownStatusGraceAttempts {
					slog.Warn("threads container: unknown status assumed ready",
						"container_id", containerID,
						"poll_count", diag.pollCount,
						"status", status.Status)
					return threadsContainerOutcome{status: "UNKNOWN_ASSUMED_READY"}, nil
				}
			}
		}

		if time.Now().Add(threadsContainerPollInterval).After(deadline) {
			diag.elapsed = time.Since(start)
			return threadsContainerOutcome{}, newProviderFailure(
				"Threads is still processing this media. It will be retried. "+diag.format(),
				map[string]any{"provider": "meta"},
				FailureContract{ErrorCode: "temporary_platform_error", Stage: "container_processing", IsRetriable: true},
			)
		}
		select {
		case <-ctx.Done():
			return threadsContainerOutcome{}, ctx.Err()
		case <-time.After(threadsContainerPollInterval):
		}
	}
}

// containerErrorFailure maps a container ERROR to the right customer contract.
// A stable validation defect is the customer's to fix; a download or
// processing failure is Meta's and goes back through the retry budget first.
func (a *ThreadsAdapter) containerErrorFailure(diag threadsContainerDiagnostics) error {
	reason := diag.errorMessage
	if reason == "" {
		reason = "UNKNOWN"
	}
	if threadsPermanentMediaErrors[reason] {
		return newProviderFailure(
			"Threads rejected this media: "+reason+". Replace it with media that meets Threads' requirements. "+diag.format(),
			map[string]any{"provider": "meta", "reason": reason},
			FailureContract{ErrorCode: "media_error", Stage: "container_processing", IsRetriable: false},
		)
	}
	// FAILED_DOWNLOADING_VIDEO, FAILED_PROCESSING_*, UNKNOWN, and anything
	// unrecognized. A download failure can come from fetch availability,
	// signed-URL timing, or provider network behaviour — none of which the
	// customer fixes by replacing valid media. Exhaust retries first.
	return newProviderFailure(
		"Threads could not process this media yet. It will be retried. "+diag.format(),
		map[string]any{"provider": "meta", "reason": reason},
		FailureContract{ErrorCode: "temporary_platform_error", Stage: "container_processing", IsRetriable: true},
	)
}

// recordThreadsContainerEvent publishes the wait's outcome onto the dispatch
// event recorder so the observed container status is queryable through the
// existing integration-log surface instead of only appearing in process logs.
// DispatchEvent has no field capable of holding a URL or a token, so this
// surface cannot leak credentials by construction.
func recordThreadsContainerEvent(
	ctx context.Context,
	diag threadsContainerDiagnostics,
	outcome threadsContainerOutcome,
	waitErr error,
	elapsed time.Duration,
	origin MediaOrigin,
) {
	metadata, ok := DispatchMetadataFromContext(ctx)
	if !ok || metadata.Events == nil {
		return
	}
	event := DispatchEvent{
		Name:          integrationlogs.ActionThreadsContainerReady,
		Status:        "succeeded",
		Duration:      elapsed,
		Reason:        diag.status,
		CustomerInput: origin == MediaOriginExternal,
	}
	if diag.errorMessage != "" {
		event.Reason = diag.status + ":" + diag.errorMessage
	}
	if waitErr != nil {
		event.Name = integrationlogs.ActionThreadsContainerFailed
		event.Status = "failed"
		if carrier, isTyped := waitErr.(interface {
			FailureContractFields() map[string]any
		}); isTyped {
			fields := carrier.FailureContractFields()
			event.ErrorCode, _ = fields["error_code"].(string)
			event.FailureStage, _ = fields["failure_stage"].(string)
			event.Retriable, _ = fields["is_retriable"].(bool)
		}
		if event.Reason == "" {
			event.Reason = "TIMEOUT"
		}
	} else if outcome.status != "" {
		event.Reason = outcome.status
	}
	event.HTTPStatus = diag.lastHTTPStatus
	metadata.Events.Record(event)
}

type threadsContainerStatus struct {
	ID           string `json:"id"`
	Status       string `json:"status"`
	ErrorMessage string `json:"error_message"`
}

// threadsPollFatal marks a poll error that must stop the loop immediately
// rather than be retried within the budget.
type threadsPollFatal struct{ err error }

func (e threadsPollFatal) Error() string { return e.err.Error() }
func (e threadsPollFatal) Unwrap() error { return e.err }

func (a *ThreadsAdapter) fetchContainerStatus(ctx context.Context, accessToken, containerID string) (threadsContainerStatus, string, int, error) {
	u := fmt.Sprintf("https://graph.threads.net/v1.0/%s?fields=id,status,error_message&access_token=%s",
		url.PathEscape(containerID), url.QueryEscape(accessToken))
	req, err := http.NewRequestWithContext(ctx, "GET", u, nil)
	if err != nil {
		return threadsContainerStatus{}, "", 0, err
	}
	resp, err := a.client.Do(req)
	if err != nil {
		return threadsContainerStatus{}, "", 0, err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	body := strings.TrimSpace(string(raw))

	if resp.StatusCode != http.StatusOK {
		// 429 and 5xx are worth another attempt inside the budget. Auth
		// failures and other definitive 4xx are not — surface them so the
		// existing reconnect taxonomy applies instead of burning five
		// minutes on a request that will never succeed.
		if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500 {
			return threadsContainerStatus{}, body, resp.StatusCode, fmt.Errorf("threads container status %d", resp.StatusCode)
		}
		return threadsContainerStatus{}, body, resp.StatusCode, threadsPollFatal{
			err: fmt.Errorf("threads container status failed (%d): %s", resp.StatusCode, truncateForLog(body, 1000)),
		}
	}

	var status threadsContainerStatus
	if err := json.Unmarshal(raw, &status); err != nil {
		// Malformed 200: keep the body for diagnostics and try again.
		return threadsContainerStatus{}, body, resp.StatusCode, fmt.Errorf("threads container status decode: %w", err)
	}
	return status, body, resp.StatusCode, nil
}

type ThreadsAdapter struct {
	client *http.Client
}

func NewThreadsAdapter() *ThreadsAdapter {
	return &ThreadsAdapter{client: debugrt.NewClient(60 * time.Second)}
}

func (a *ThreadsAdapter) Platform() string { return "threads" }

func (a *ThreadsAdapter) DefaultOAuthConfig(baseRedirectURL string) OAuthConfig {
	return OAuthConfig{
		ClientID:     os.Getenv("THREADS_APP_ID"),
		ClientSecret: os.Getenv("THREADS_APP_SECRET"),
		AuthURL:      "https://threads.net/oauth/authorize",
		TokenURL:     "https://graph.threads.net/oauth/access_token",
		RedirectURL:  baseRedirectURL + "/v1/oauth/callback/threads",
		Scopes:       []string{"threads_basic", "threads_content_publish", "threads_manage_replies", "threads_manage_insights", "threads_read_replies"},
	}
}

func (a *ThreadsAdapter) GetAuthURL(config OAuthConfig, state string) string {
	return BuildAuthURL(config.AuthURL, config.ClientID, config.RedirectURL, state, config.Scopes)
}

func (a *ThreadsAdapter) ExchangeCode(ctx context.Context, config OAuthConfig, code string) (*ConnectResult, error) {
	data := url.Values{
		"client_id":     {config.ClientID},
		"client_secret": {config.ClientSecret},
		"grant_type":    {"authorization_code"},
		"redirect_uri":  {config.RedirectURL},
		"code":          {code},
	}

	resp, err := a.client.PostForm(config.TokenURL, data)
	if err != nil {
		return nil, fmt.Errorf("failed to exchange code: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("token exchange failed (%d): %s", resp.StatusCode, string(body))
	}

	var tokenResp struct {
		AccessToken string `json:"access_token"`
		TokenType   string `json:"token_type"`
		UserID      string `json:"user_id"`
	}
	json.NewDecoder(resp.Body).Decode(&tokenResp)

	// Exchange for long-lived token
	longToken, expiresIn, err := a.exchangeForLongLivedToken(ctx, config, tokenResp.AccessToken)
	if err != nil {
		return nil, fmt.Errorf("threads long-lived token exchange failed: %w", err)
	}

	// Get profile
	profile, err := a.getProfile(ctx, longToken, tokenResp.UserID)
	if err != nil {
		profile = &threadsProfile{id: tokenResp.UserID, username: tokenResp.UserID}
	}

	return &ConnectResult{
		AccessToken:       longToken,
		RefreshToken:      longToken,
		TokenExpiresAt:    time.Now().Add(time.Duration(expiresIn) * time.Second),
		ExternalAccountID: profile.id,
		AccountName:       profile.username,
		AvatarURL:         profile.profilePicURL,
		Scopes:            config.Scopes,
		Metadata: map[string]any{
			"threads_user_id": profile.id,
			"username":        profile.username,
			"granted_scopes":  config.Scopes,
		},
	}, nil
}

func (a *ThreadsAdapter) Connect(ctx context.Context, credentials map[string]string) (*ConnectResult, error) {
	return nil, fmt.Errorf("threads requires OAuth flow, use POST /v1/oauth/connect")
}

// Post publishes a text post (with optional media) to Threads.
//
// Readiness is observed, never assumed: the container is polled until Meta
// reports it publishable. A retry resumes the container a prior attempt
// created rather than building a second one, so a publish that succeeded
// server-side but whose response we never saw is detected instead of
// duplicated.
func (a *ThreadsAdapter) Post(ctx context.Context, accessToken string, text string, media []MediaItem, opts map[string]any) (*PostResult, error) {
	userID, err := a.getUserID(ctx, accessToken)
	if err != nil {
		return nil, err
	}
	if len(media) > 20 {
		return nil, fmt.Errorf("threads carousels accept at most 20 items")
	}

	origin := MediaOriginExternal
	if len(media) > 0 {
		origin = media[0].Origin
	}

	creationID, outcome, err := a.resolveContainer(ctx, accessToken, userID, text, media, origin, opts)
	if err != nil {
		return nil, err
	}
	if outcome.alreadyPublished {
		// A prior attempt's threads_publish landed even though we never saw
		// the response. Publishing again would duplicate the post.
		slog.Info("threads post: resumed container already published, reconciling",
			"container_id", creationID)
		return a.buildPostResult(ctx, accessToken, userID, creationID)
	}

	publishedID, err := a.publishContainer(ctx, accessToken, userID, creationID)
	if err != nil {
		return nil, err
	}
	return a.buildPostResult(ctx, accessToken, userID, publishedID)
}

// resolveContainer returns a container that is ready to publish, either by
// resuming the one a prior attempt persisted or by creating a fresh one.
func (a *ThreadsAdapter) resolveContainer(
	ctx context.Context,
	accessToken, userID, text string,
	media []MediaItem,
	origin MediaOrigin,
	opts map[string]any,
) (string, threadsContainerOutcome, error) {
	if resume := resumePublishToken(opts); resume != "" {
		outcome, err := a.waitForContainer(ctx, accessToken, resume, origin)
		switch {
		case err == nil:
			slog.Info("threads post: resumed container", "container_id", resume, "status", outcome.status)
			return resume, outcome, nil
		case errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
			return "", threadsContainerOutcome{}, err
		default:
			// EXPIRED, ERROR, or an unusable response. Publishing an object
			// we can't vouch for is worse than paying to rebuild it, so fall
			// through and create a fresh container. The new token replaces
			// the stale one below.
			slog.Warn("threads post: resumed container unusable, creating a fresh one",
				"container_id", resume, "err", err)
		}
	}

	creationID, err := a.createTopLevelContainer(ctx, accessToken, userID, text, media)
	if err != nil {
		return "", threadsContainerOutcome{}, err
	}
	// Persist before polling: the token has to be durable before the first
	// opportunity to lose the worker, otherwise resume can never help.
	if err := persistPublishToken(opts, creationID); err != nil {
		return "", threadsContainerOutcome{}, fmt.Errorf("threads persist container token: %w", err)
	}
	outcome, err := a.waitForContainer(ctx, accessToken, creationID, origin)
	if err != nil {
		return "", threadsContainerOutcome{}, err
	}
	return creationID, outcome, nil
}

// createTopLevelContainer builds the publishable container for this post.
//
// Threads container shape rules:
//   - 0 items   → media_type=TEXT
//   - 1 image   → media_type=IMAGE, image_url
//   - 1 video   → media_type=VIDEO, video_url
//   - 2+ items  → media_type=CAROUSEL, children=[...] where each child is
//     created up-front with is_carousel_item=true.
func (a *ThreadsAdapter) createTopLevelContainer(ctx context.Context, accessToken, userID, text string, media []MediaItem) (string, error) {
	switch {
	case len(media) == 0:
		return a.createTextContainer(ctx, accessToken, userID, text)
	case len(media) == 1:
		return a.createMediaContainer(ctx, accessToken, userID, text, media[0], false)
	default:
		return a.createCarouselContainer(ctx, accessToken, userID, text, media)
	}
}

func (a *ThreadsAdapter) publishContainer(ctx context.Context, accessToken, userID, creationID string) (string, error) {
	publishURL := fmt.Sprintf("https://graph.threads.net/v1.0/%s/threads_publish?creation_id=%s&access_token=%s",
		userID, url.QueryEscape(creationID), url.QueryEscape(accessToken))
	req, err := http.NewRequestWithContext(ctx, "POST", publishURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	pubResp, err := a.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("threads publish failed: %w", err)
	}
	defer pubResp.Body.Close()

	if pubResp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(pubResp.Body)
		return "", a.publishFailure(pubResp.StatusCode, string(body))
	}

	var published struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(pubResp.Body).Decode(&published); err != nil {
		return "", fmt.Errorf("threads publish failed: decode response: %w", err)
	}
	return published.ID, nil
}

// publishFailure types the threads_publish boundary. Meta code 24 /
// subcode 4279009 ("Media Not Found") is returned when the container is not
// yet resolvable at publish time; despite is_transient=false it succeeds on a
// later attempt, so it is retriable at this boundary specifically.
func (a *ThreadsAdapter) publishFailure(status int, body string) error {
	message := fmt.Sprintf("threads publish failed (%d): %s", status, truncateForLog(body, 1000))
	if strings.Contains(body, `"error_subcode":4279009`) || strings.Contains(body, `"error_subcode": 4279009`) {
		return newProviderFailure(
			message,
			map[string]any{"provider": "meta", "http_status": status, "code": "24", "subcode": "4279009"},
			FailureContract{ErrorCode: "temporary_platform_error", Stage: "dispatch", IsRetriable: true},
		)
	}
	return fmt.Errorf("%s", message)
}

func (a *ThreadsAdapter) buildPostResult(ctx context.Context, accessToken, userID, publishedID string) (*PostResult, error) {
	// Prefer the permalink from the Graph API — Threads URLs use
	// shortcodes that aren't derivable from the numeric post ID.
	// Fresh posts take a few seconds before the permalink is
	// populated, so fetchPermalink retries. If we still can't get
	// it, fall back to the profile URL so the user at least lands
	// on their Threads account rather than a broken /post/{id}.
	permalink, permErr := a.fetchPermalink(ctx, accessToken, publishedID)
	if permErr != nil || permalink == "" {
		if permErr != nil {
			slog.Warn("threads post: permalink fetch failed, falling back to profile URL",
				"post_id", publishedID, "err", permErr)
		}
		profile, _ := a.getProfile(ctx, accessToken, userID)
		username := userID
		if profile != nil && profile.username != "" {
			username = profile.username
		}
		permalink = fmt.Sprintf("https://www.threads.net/@%s", username)
	}

	return &PostResult{
		ExternalID: publishedID,
		URL:        permalink,
	}, nil
}

func (a *ThreadsAdapter) createTextContainer(ctx context.Context, accessToken, userID, text string) (string, error) {
	params := url.Values{
		"media_type":   {"TEXT"},
		"text":         {text},
		"access_token": {accessToken},
	}
	return a.postContainer(ctx, userID, params)
}

func (a *ThreadsAdapter) createMediaContainer(ctx context.Context, accessToken, userID, text string, item MediaItem, isCarouselChild bool) (string, error) {
	params := url.Values{
		"access_token": {accessToken},
	}
	if !isCarouselChild {
		params.Set("text", text)
	} else {
		params.Set("is_carousel_item", "true")
	}

	kind := item.Kind
	if kind == MediaKindUnknown {
		kind = SniffMediaKind(item.URL)
	}

	switch kind {
	case MediaKindVideo:
		params.Set("media_type", "VIDEO")
		params.Set("video_url", item.URL)
	default:
		params.Set("media_type", "IMAGE")
		params.Set("image_url", item.URL)
	}
	return a.postContainer(ctx, userID, params)
}

func (a *ThreadsAdapter) createCarouselContainer(ctx context.Context, accessToken, userID, text string, items []MediaItem) (string, error) {
	childIDs := make([]string, 0, len(items))
	for _, item := range items {
		id, err := a.createMediaContainer(ctx, accessToken, userID, text, item, true)
		if err != nil {
			return "", err
		}
		childIDs = append(childIDs, id)
	}

	// Children must be ready before the parent references them. Threads
	// rejects an in-progress child with code 100 / subcode 4279004,
	// "The children with IDs ... are invalid, nonexistent, or expired" —
	// observed on dev the moment the previous fixed 5-second sleep was
	// removed. The fix is to observe readiness, not to guess at it again:
	// poll each child, sharing one deadline so N children cannot multiply
	// the intended worst-case wait.
	childDeadline := time.Now().Add(threadsContainerPollBudget)
	for _, id := range childIDs {
		if _, err := a.waitForContainerUntil(ctx, accessToken, id, MediaOriginManaged, childDeadline); err != nil {
			return "", fmt.Errorf("threads carousel child %s not ready: %w", id, err)
		}
	}

	params := url.Values{
		"access_token": {accessToken},
		"text":         {text},
		"media_type":   {"CAROUSEL"},
		"children":     {strings.Join(childIDs, ",")},
	}
	return a.postContainer(ctx, userID, params)
}

func (a *ThreadsAdapter) postContainer(ctx context.Context, userID string, params url.Values) (string, error) {
	containerURL := fmt.Sprintf("https://graph.threads.net/v1.0/%s/threads?%s", userID, params.Encode())
	req, err := http.NewRequestWithContext(ctx, "POST", containerURL, nil)
	if err != nil {
		return "", err
	}
	resp, err := a.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to create container: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		text := string(body)
		// Subcode 4279004 ("Invalid Carousel Children") means a referenced
		// child was not usable when the parent was assembled. Children are
		// polled to FINISHED before this call, but one can still expire in
		// the gap, and rebuilding the set on a retry fixes it. Without this
		// it lands as platform_error / contact_support, which tells the
		// customer to open a ticket for something a retry resolves.
		if strings.Contains(text, `"error_subcode":4279004`) || strings.Contains(text, `"error_subcode": 4279004`) {
			return "", newProviderFailure(
				fmt.Sprintf("threads carousel children were not usable (%d): %s", resp.StatusCode, truncateForLog(text, 1000)),
				map[string]any{"provider": "meta", "http_status": resp.StatusCode, "code": "100", "subcode": "4279004"},
				FailureContract{ErrorCode: "temporary_platform_error", Stage: "container_processing", IsRetriable: true},
			)
		}
		return "", fmt.Errorf("container creation failed (%d): %s", resp.StatusCode, text)
	}

	var container struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&container); err != nil {
		return "", err
	}
	return container.ID, nil
}

// FetchMediaDetails returns details about a specific Threads post.
func (a *ThreadsAdapter) FetchMediaDetails(ctx context.Context, accessToken string, mediaID string) (*MediaDetails, error) {
	u := fmt.Sprintf("https://graph.threads.net/v1.0/%s?fields=id,text,media_url,timestamp,media_type,permalink&access_token=%s",
		mediaID, accessToken)
	req, err := http.NewRequestWithContext(ctx, "GET", u, nil)
	if err != nil {
		return nil, err
	}
	resp, err := a.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("threads fetch media details: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("threads fetch media details %d: %s", resp.StatusCode, string(body))
	}
	var raw struct {
		ID        string `json:"id"`
		Text      string `json:"text"`
		MediaURL  string `json:"media_url"`
		Timestamp string `json:"timestamp"`
		MediaType string `json:"media_type"`
		Permalink string `json:"permalink"`
	}
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, err
	}
	return &MediaDetails{
		ID:        raw.ID,
		Caption:   raw.Text,
		MediaURL:  raw.MediaURL,
		Timestamp: raw.Timestamp,
		MediaType: raw.MediaType,
		Permalink: raw.Permalink,
	}, nil
}

// FetchRecentMedia returns the IDs of the account's recent Threads posts
// directly from the API. Covers posts published natively, not just
// those published through UniPost.
func (a *ThreadsAdapter) FetchRecentMedia(ctx context.Context, accessToken string) ([]string, error) {
	userID, err := a.getUserID(ctx, accessToken)
	if err != nil {
		return nil, err
	}
	u := fmt.Sprintf("https://graph.threads.net/v1.0/%s/threads?fields=id&limit=10&access_token=%s",
		userID, accessToken)
	req, err := http.NewRequestWithContext(ctx, "GET", u, nil)
	if err != nil {
		return nil, err
	}
	resp, err := a.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("threads fetch recent media: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("threads fetch recent media %d: %s", resp.StatusCode, string(body))
	}
	var result struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}
	ids := make([]string, 0, len(result.Data))
	for _, m := range result.Data {
		ids = append(ids, m.ID)
	}
	return ids, nil
}

// FetchComments returns replies on a Threads post.
// GET /v1.0/{post-id}/replies?fields=id,text,username,timestamp
func (a *ThreadsAdapter) FetchComments(ctx context.Context, accessToken string, postExternalID string) ([]InboxEntry, error) {
	u := fmt.Sprintf("https://graph.threads.net/v1.0/%s/replies?fields=id,text,username,timestamp&access_token=%s",
		postExternalID, accessToken)
	req, err := http.NewRequestWithContext(ctx, "GET", u, nil)
	if err != nil {
		return nil, err
	}
	resp, err := a.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("threads fetch replies: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		slog.Warn("threads fetch replies failed",
			"status", resp.StatusCode,
			"post_id", postExternalID,
			"body", string(body))
		return nil, fmt.Errorf("threads fetch replies %d: %s", resp.StatusCode, string(body))
	}

	var result struct {
		Data []struct {
			ID        string `json:"id"`
			Text      string `json:"text"`
			Username  string `json:"username"`
			Timestamp string `json:"timestamp"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("threads fetch replies decode: %w", err)
	}

	entries := make([]InboxEntry, 0, len(result.Data))
	for _, r := range result.Data {
		ts, _ := time.Parse(time.RFC3339Nano, r.Timestamp)
		if ts.IsZero() {
			ts, _ = time.Parse("2006-01-02T15:04:05-0700", r.Timestamp)
		}
		if ts.IsZero() {
			ts = time.Now()
		}
		entries = append(entries, InboxEntry{
			ExternalID:       r.ID,
			ParentExternalID: postExternalID,
			AuthorName:       r.Username,
			Body:             r.Text,
			Timestamp:        ts,
			Source:           "threads_reply",
		})
	}
	return entries, nil
}

// ReplyToComment publishes a reply to a Threads post using the
// two-step container flow with reply_to_id.
func (a *ThreadsAdapter) ReplyToComment(ctx context.Context, accessToken string, replyToExternalID string, text string) (*PostResult, error) {
	userID, err := a.getUserID(ctx, accessToken)
	if err != nil {
		return nil, err
	}

	// Step 1: create a TEXT container with reply_to_id
	params := url.Values{
		"media_type":   {"TEXT"},
		"text":         {text},
		"reply_to_id":  {replyToExternalID},
		"access_token": {accessToken},
	}
	containerID, err := a.postContainer(ctx, userID, params)
	if err != nil {
		return nil, fmt.Errorf("threads reply container: %w", err)
	}

	// Step 2: wait for the container to actually be publishable, then publish.
	// A reply is text-only so it is normally ready on the first poll, but the
	// same gate applies rather than a guessed sleep.
	if _, err := a.waitForContainer(ctx, accessToken, containerID, MediaOriginManaged); err != nil {
		return nil, err
	}
	publishedID, err := a.publishContainer(ctx, accessToken, userID, containerID)
	if err != nil {
		return nil, fmt.Errorf("threads reply publish: %w", err)
	}
	return &PostResult{ExternalID: publishedID}, nil
}

func (a *ThreadsAdapter) DeletePost(ctx context.Context, accessToken string, externalID string) error {
	return fmt.Errorf("threads does not support post deletion via API")
}

func (a *ThreadsAdapter) RefreshToken(ctx context.Context, refreshToken string) (string, string, time.Time, error) {
	params := url.Values{
		"grant_type":   {"th_refresh_token"},
		"access_token": {refreshToken},
	}

	resp, err := a.client.Get("https://graph.threads.net/refresh_access_token?" + params.Encode())
	if err != nil {
		return "", "", time.Time{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", "", time.Time{}, fmt.Errorf("refresh failed (%d): %s", resp.StatusCode, string(body))
	}

	var result struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
	}
	json.NewDecoder(resp.Body).Decode(&result)

	return result.AccessToken, result.AccessToken, time.Now().Add(time.Duration(result.ExpiresIn) * time.Second), nil
}

// GetAnalytics fetches post metrics from Threads Insights API.
func (a *ThreadsAdapter) GetAnalytics(ctx context.Context, accessToken string, externalID string) (*PostMetrics, error) {
	insightsURL := fmt.Sprintf(
		"https://graph.threads.net/v1.0/%s/insights?metric=views,likes,replies,reposts,quotes&access_token=%s",
		externalID, accessToken)

	req, err := http.NewRequestWithContext(ctx, "GET", insightsURL, nil)
	if err != nil {
		return nil, err
	}

	resp, err := a.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return &PostMetrics{}, nil
	}

	var result struct {
		Data []struct {
			Name   string `json:"name"`
			Values []struct {
				Value int64 `json:"value"`
			} `json:"values"`
		} `json:"data"`
	}
	json.NewDecoder(resp.Body).Decode(&result)

	m := &PostMetrics{}
	reposts := int64(0)
	quotes := int64(0)
	for _, metric := range result.Data {
		val := int64(0)
		if len(metric.Values) > 0 {
			val = metric.Values[0].Value
		}
		switch metric.Name {
		case "views":
			// Threads "views" represents post impressions (text + media), not
			// just video plays. Map to Impressions and keep legacy Views alias.
			m.Impressions = val
			m.Views = val
		case "likes":
			m.Likes = val
		case "replies":
			m.Comments = val
		case "reposts":
			m.Shares = val
			reposts = val
		case "quotes":
			m.Shares += val
			quotes = val
		}
	}
	m.PlatformSpecific = map[string]any{
		"reposts": reposts,
		"quotes":  quotes,
	}

	// EngagementRate is computed by the analytics handler.
	return m, nil
}

func (a *ThreadsAdapter) exchangeForLongLivedToken(_ context.Context, config OAuthConfig, shortToken string) (string, int, error) {
	params := url.Values{
		"grant_type":    {"th_exchange_token"},
		"client_secret": {config.ClientSecret},
		"access_token":  {shortToken},
	}

	resp, err := a.client.Get("https://graph.threads.net/access_token?" + params.Encode())
	if err != nil {
		return "", 0, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return "", 0, fmt.Errorf("long-lived token swap returned %d: %s", resp.StatusCode, string(body))
	}

	var result struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", 0, fmt.Errorf("long-lived token swap decode: %w", err)
	}
	if result.AccessToken == "" {
		return "", 0, fmt.Errorf("long-lived token swap returned empty access token: %s", string(body))
	}
	if time.Duration(result.ExpiresIn)*time.Second <= minThreadsLongLivedTokenLifetime {
		return "", 0, fmt.Errorf("short-lived Threads token returned by long-lived token swap: expires_in=%d", result.ExpiresIn)
	}

	return result.AccessToken, result.ExpiresIn, nil
}

type threadsProfile struct {
	id            string
	username      string
	profilePicURL string
}

type ThreadsProfile struct {
	ID                string `json:"id"`
	Username          string `json:"username"`
	ProfilePictureURL string `json:"threads_profile_picture_url"`
}

func (a *ThreadsAdapter) FetchProfile(ctx context.Context, accessToken string) (*ThreadsProfile, error) {
	userID, err := a.getUserID(ctx, accessToken)
	if err != nil {
		return nil, err
	}
	profile, err := a.getProfile(ctx, accessToken, userID)
	if err != nil {
		return nil, err
	}
	return &ThreadsProfile{
		ID:                profile.id,
		Username:          profile.username,
		ProfilePictureURL: profile.profilePicURL,
	}, nil
}

type ThreadsAccountInsights struct {
	Views          int64 `json:"views"`
	Likes          int64 `json:"likes"`
	Replies        int64 `json:"replies"`
	Reposts        int64 `json:"reposts"`
	Quotes         int64 `json:"quotes"`
	FollowersCount int64 `json:"followers_count"`
}

func (a *ThreadsAdapter) FetchAccountInsights(ctx context.Context, accessToken, userID string) (*ThreadsAccountInsights, error) {
	if strings.TrimSpace(userID) == "" {
		var err error
		userID, err = a.getUserID(ctx, accessToken)
		if err != nil {
			return nil, err
		}
	}

	params := url.Values{}
	params.Set("metric", "views,likes,replies,reposts,quotes,followers_count")
	params.Set("access_token", accessToken)
	req, err := http.NewRequestWithContext(ctx, "GET",
		fmt.Sprintf("https://graph.threads.net/v1.0/%s/threads_insights?%s", userID, params.Encode()), nil)
	if err != nil {
		return nil, err
	}

	resp, err := a.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("threads account insights: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("threads account insights (%d): %s", resp.StatusCode, string(body))
	}

	var result struct {
		Data []struct {
			Name   string `json:"name"`
			Values []struct {
				Value int64 `json:"value"`
			} `json:"values"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("threads account insights decode: %w", err)
	}

	insights := &ThreadsAccountInsights{}
	for _, metric := range result.Data {
		val := int64(0)
		if len(metric.Values) > 0 {
			val = metric.Values[len(metric.Values)-1].Value
		}
		switch metric.Name {
		case "views":
			insights.Views = val
		case "likes":
			insights.Likes = val
		case "replies":
			insights.Replies = val
		case "reposts":
			insights.Reposts = val
		case "quotes":
			insights.Quotes = val
		case "followers_count":
			insights.FollowersCount = val
		}
	}
	return insights, nil
}

func (a *ThreadsAdapter) GetAccountMetrics(ctx context.Context, accessToken, externalAccountID string) (*AccountMetrics, error) {
	insights, err := a.FetchAccountInsights(ctx, accessToken, externalAccountID)
	if err != nil {
		return nil, err
	}
	return &AccountMetrics{
		FollowerCount: insights.FollowersCount,
		PlatformSpecific: map[string]any{
			"views":   insights.Views,
			"likes":   insights.Likes,
			"replies": insights.Replies,
			"reposts": insights.Reposts,
			"quotes":  insights.Quotes,
		},
	}, nil
}

type ThreadsPost struct {
	ID        string `json:"id"`
	Text      string `json:"text"`
	MediaType string `json:"media_type"`
	MediaURL  string `json:"media_url"`
	Permalink string `json:"permalink"`
	Timestamp string `json:"timestamp"`
}

type ThreadsPostList struct {
	Posts []ThreadsPost `json:"posts"`
}

func (a *ThreadsAdapter) ListPosts(ctx context.Context, accessToken string, limit int) (*ThreadsPostList, error) {
	if limit <= 0 || limit > 50 {
		limit = 20
	}
	userID, err := a.getUserID(ctx, accessToken)
	if err != nil {
		return nil, err
	}
	params := url.Values{}
	params.Set("fields", "id,text,media_type,media_url,permalink,timestamp")
	params.Set("limit", fmt.Sprintf("%d", limit))
	params.Set("access_token", accessToken)

	req, err := http.NewRequestWithContext(ctx, "GET",
		fmt.Sprintf("https://graph.threads.net/v1.0/%s/threads?%s", userID, params.Encode()), nil)
	if err != nil {
		return nil, err
	}
	resp, err := a.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("threads posts list: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("threads posts list (%d): %s", resp.StatusCode, string(body))
	}

	var result struct {
		Data []ThreadsPost `json:"data"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("threads posts list decode: %w", err)
	}
	if result.Data == nil {
		result.Data = []ThreadsPost{}
	}
	return &ThreadsPostList{Posts: result.Data}, nil
}

func (a *ThreadsAdapter) getProfile(ctx context.Context, accessToken string, userID string) (*threadsProfile, error) {
	req, _ := http.NewRequestWithContext(ctx, "GET",
		fmt.Sprintf("https://graph.threads.net/v1.0/%s?fields=id,username,threads_profile_picture_url&access_token=%s", userID, accessToken), nil)
	resp, err := a.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		slog.Warn("threads get profile non-200",
			"status", resp.StatusCode, "user_id", userID, "body", string(body))
		return nil, fmt.Errorf("threads get profile returned %d: %s", resp.StatusCode, string(body))
	}

	var profile struct {
		ID                string `json:"id"`
		Username          string `json:"username"`
		ProfilePictureURL string `json:"threads_profile_picture_url"`
	}
	if err := json.Unmarshal(body, &profile); err != nil {
		return nil, fmt.Errorf("threads get profile decode: %w", err)
	}
	if profile.Username == "" {
		// A 200 with an empty username means scope/permission mismatch
		// — better to surface as an error than to quietly hand back an
		// empty string that breaks URL construction downstream.
		return nil, fmt.Errorf("threads get profile: empty username in response body: %s", string(body))
	}
	return &threadsProfile{
		id:            profile.ID,
		username:      profile.Username,
		profilePicURL: profile.ProfilePictureURL,
	}, nil
}

// fetchPermalink asks the Threads Graph API for a post's canonical
// URL. Fresh posts can take a few seconds before the permalink is
// populated, so we retry up to 3x with a 2s delay.
func (a *ThreadsAdapter) fetchPermalink(ctx context.Context, accessToken string, postID string) (string, error) {
	var lastBody string
	for attempt := 0; attempt < 3; attempt++ {
		if attempt > 0 {
			// Cosmetic retry for a post that is already published, so a
			// worker shutdown should abandon it rather than sleep it out.
			select {
			case <-ctx.Done():
				return "", ctx.Err()
			case <-time.After(2 * time.Second):
			}
		}
		req, _ := http.NewRequestWithContext(ctx, "GET",
			fmt.Sprintf("https://graph.threads.net/v1.0/%s?fields=permalink&access_token=%s", postID, accessToken), nil)
		resp, err := a.client.Do(req)
		if err != nil {
			return "", err
		}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		lastBody = string(body)
		if resp.StatusCode != http.StatusOK {
			continue
		}
		var result struct {
			Permalink string `json:"permalink"`
		}
		if err := json.Unmarshal(body, &result); err != nil {
			continue
		}
		if result.Permalink != "" {
			return result.Permalink, nil
		}
	}
	return "", fmt.Errorf("threads fetch permalink: no permalink after retries: %s", lastBody)
}

func (a *ThreadsAdapter) getUserID(ctx context.Context, accessToken string) (string, error) {
	req, _ := http.NewRequestWithContext(ctx, "GET",
		"https://graph.threads.net/v1.0/me?fields=id&access_token="+accessToken, nil)
	resp, err := a.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	bodyText := strings.TrimSpace(string(body))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("threads get user id failed (%d): %s", resp.StatusCode, bodyText)
	}

	var result struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("threads get user id decode: %w; body: %s", err, bodyText)
	}
	if result.ID == "" {
		return "", fmt.Errorf("threads get user id: empty id in response body: %s", bodyText)
	}
	return result.ID, nil
}
