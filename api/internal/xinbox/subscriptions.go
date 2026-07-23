package xinbox

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

type Webhook struct {
	ID    string `json:"id"`
	URL   string `json:"url"`
	Valid bool   `json:"valid"`
}

type ActivityFilter struct {
	UserID string `json:"user_id"`
}

type ActivitySubscription struct {
	ID        string         `json:"subscription_id,omitempty"`
	EventType string         `json:"event_type"`
	Filter    ActivityFilter `json:"filter"`
	Tag       string         `json:"tag"`
	WebhookID string         `json:"webhook_id"`
}

const maxAppWebhookURLLength = 200

func DMSubscriptionTag(accountID string) string {
	return "unipost:x:dm:" + strings.TrimSpace(accountID)
}

func AppWebhookURL(baseURL, webhookRouteKey string) (string, error) {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	webhookRouteKey = strings.TrimSpace(webhookRouteKey)
	if baseURL == "" {
		return "", errors.New("X_INBOX_WEBHOOK_URL is not configured")
	}
	if webhookRouteKey == "" {
		return "", errors.New("X webhook route key is not configured")
	}
	parsed, err := url.Parse(baseURL)
	if err != nil || parsed.Scheme != "https" || parsed.Hostname() == "" || parsed.Port() != "" || parsed.User != nil ||
		parsed.RawQuery != "" || parsed.ForceQuery || parsed.Fragment != "" {
		return "", errors.New("X_INBOX_WEBHOOK_URL must be an absolute HTTPS URL")
	}
	parsed.Path = strings.TrimRight(parsed.Path, "/") + "/" + url.PathEscape(webhookRouteKey)
	webhookURL := parsed.String()
	if len(webhookURL) > maxAppWebhookURLLength {
		return "", errors.New("X webhook URL exceeds provider length limit")
	}
	return webhookURL, nil
}

func (c *Client) ListWebhooks(ctx context.Context, appBearerToken string) ([]Webhook, error) {
	var response struct {
		Data []Webhook `json:"data"`
	}
	if err := c.doJSON(ctx, http.MethodGet, "/2/webhooks", appBearerToken, nil, &response); err != nil {
		return nil, err
	}
	return response.Data, nil
}

func (c *Client) EnsureWebhook(ctx context.Context, appBearerToken, configuredURL string) (Webhook, error) {
	webhooks, err := c.ListWebhooks(ctx, appBearerToken)
	if err != nil {
		return Webhook{}, err
	}
	for _, webhook := range webhooks {
		if webhook.URL == configuredURL {
			if err := validateProviderResourceID(webhook.ID); err != nil {
				return Webhook{}, err
			}
			if !webhook.Valid {
				var response struct {
					Data struct {
						Attempted bool `json:"attempted"`
					} `json:"data"`
				}
				path := "/2/webhooks/" + url.PathEscape(webhook.ID)
				if err := c.doJSON(ctx, http.MethodPut, path, appBearerToken, nil, &response); err != nil {
					return Webhook{}, err
				}
				if !response.Data.Attempted {
					return Webhook{}, errors.New("X webhook CRC revalidation was not attempted")
				}
				for poll := 0; poll < c.webhookValidationPolls; poll++ {
					if err := c.sleep(ctx, c.webhookValidationBackoff); err != nil {
						return Webhook{}, err
					}
					current, err := c.ListWebhooks(ctx, appBearerToken)
					if err != nil {
						return Webhook{}, err
					}
					for _, candidate := range current {
						if candidate.ID == webhook.ID && candidate.URL == configuredURL && candidate.Valid {
							return candidate, nil
						}
					}
				}
				return Webhook{}, errors.New("X webhook CRC revalidation did not become valid before polling limit")
			}
			return webhook, nil
		}
	}

	var raw json.RawMessage
	_, err = c.do(
		ctx,
		http.MethodPost,
		"/2/webhooks",
		appBearerToken,
		map[string]string{"url": configuredURL},
		&raw,
	)
	if err != nil {
		return Webhook{}, err
	}
	var direct Webhook
	if err := json.Unmarshal(raw, &direct); err != nil {
		return Webhook{}, fmt.Errorf("decode X create webhook response: %w", err)
	}
	if direct.ID != "" {
		return c.confirmCreatedWebhook(ctx, appBearerToken, configuredURL, direct)
	}

	var wrapped struct {
		Data Webhook `json:"data"`
	}
	if err := json.Unmarshal(raw, &wrapped); err != nil {
		return Webhook{}, fmt.Errorf("decode wrapped X create webhook response: %w", err)
	}
	if wrapped.Data.ID == "" {
		return Webhook{}, errors.New("X create webhook response missing webhook id")
	}
	return c.confirmCreatedWebhook(ctx, appBearerToken, configuredURL, wrapped.Data)
}

func (c *Client) confirmCreatedWebhook(
	ctx context.Context,
	appBearerToken string,
	configuredURL string,
	created Webhook,
) (Webhook, error) {
	if err := validateProviderResourceID(created.ID); err != nil {
		return Webhook{}, err
	}
	if created.URL != "" && created.URL != configuredURL {
		return Webhook{}, errors.New("X create webhook response did not match configured URL")
	}
	if created.URL == configuredURL && created.Valid {
		return created, nil
	}
	for poll := 0; poll < c.webhookValidationPolls; poll++ {
		if err := c.sleep(ctx, c.webhookValidationBackoff); err != nil {
			return Webhook{}, err
		}
		current, err := c.ListWebhooks(ctx, appBearerToken)
		if err != nil {
			return Webhook{}, err
		}
		for _, candidate := range current {
			if candidate.ID == created.ID && candidate.URL == configuredURL && candidate.Valid {
				return candidate, nil
			}
		}
	}
	return Webhook{}, errors.New("X created webhook did not become valid before polling limit")
}

func (c *Client) DeleteWebhook(ctx context.Context, appBearerToken, webhookID string) error {
	if err := validateProviderResourceID(webhookID); err != nil {
		return err
	}
	path := "/2/webhooks/" + url.PathEscape(webhookID)
	return c.deleteConfirmedProviderResource(ctx, appBearerToken, path, "webhook")
}

func (c *Client) ListActivitySubscriptions(
	ctx context.Context,
	appBearerToken string,
) ([]ActivitySubscription, error) {
	const (
		selfServeSubscriptionLimit = 1500
		maxSubscriptionPageSize    = 1000
	)
	subscriptions := make([]ActivitySubscription, 0)
	nextToken := ""
	seenTokens := make(map[string]struct{})
	for page := 0; page < selfServeSubscriptionLimit && len(subscriptions) < selfServeSubscriptionLimit; page++ {
		pageSize := selfServeSubscriptionLimit - len(subscriptions)
		if pageSize > maxSubscriptionPageSize {
			pageSize = maxSubscriptionPageSize
		}
		query := url.Values{"max_results": {fmt.Sprintf("%d", pageSize)}}
		if nextToken != "" {
			query.Set("pagination_token", nextToken)
		}
		var response struct {
			Data []ActivitySubscription `json:"data"`
			Meta struct {
				NextToken string `json:"next_token"`
			} `json:"meta"`
		}
		path := "/2/activity/subscriptions?" + query.Encode()
		if err := c.doJSON(ctx, http.MethodGet, path, appBearerToken, nil, &response); err != nil {
			return nil, err
		}
		remaining := selfServeSubscriptionLimit - len(subscriptions)
		if len(response.Data) > remaining {
			response.Data = response.Data[:remaining]
		}
		subscriptions = append(subscriptions, response.Data...)
		nextToken = response.Meta.NextToken
		if nextToken == "" {
			break
		}
		if _, duplicate := seenTokens[nextToken]; duplicate {
			return nil, errors.New("X activity subscription pagination repeated next_token")
		}
		seenTokens[nextToken] = struct{}{}
	}
	if nextToken != "" && len(subscriptions) < selfServeSubscriptionLimit {
		return nil, errors.New("X activity subscription pagination exceeded self-serve bound")
	}
	return subscriptions, nil
}

func (c *Client) EnsureDMSubscription(
	ctx context.Context,
	appBearerToken string,
	accountID string,
	userID string,
	webhookID string,
) (ActivitySubscription, error) {
	if err := validateProviderResourceID(webhookID); err != nil {
		return ActivitySubscription{}, err
	}
	subscriptions, err := c.ListActivitySubscriptions(ctx, appBearerToken)
	if err != nil {
		return ActivitySubscription{}, err
	}
	tag := DMSubscriptionTag(accountID)
	for _, subscription := range subscriptions {
		if subscription.Tag == tag {
			if err := validateProviderResourceID(subscription.ID); err != nil {
				return ActivitySubscription{}, err
			}
			if subscription.EventType == "dm.received" &&
				subscription.Filter.UserID == userID &&
				subscription.WebhookID == webhookID {
				return subscription, nil
			}
			if err := c.DeleteActivitySubscription(ctx, appBearerToken, subscription.ID); err != nil {
				return ActivitySubscription{}, err
			}
			break
		}
	}

	request := ActivitySubscription{
		EventType: "dm.received",
		Filter:    ActivityFilter{UserID: userID},
		Tag:       tag,
		WebhookID: webhookID,
	}
	var raw json.RawMessage
	if err := c.doJSON(ctx, http.MethodPost, "/2/activity/subscriptions", appBearerToken, request, &raw); err != nil {
		return ActivitySubscription{}, err
	}
	var response struct {
		Data json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(raw, &response); err != nil {
		return ActivitySubscription{}, fmt.Errorf("decode X create activity subscription response: %w", err)
	}
	var subscription ActivitySubscription
	if err := json.Unmarshal(response.Data, &subscription); err != nil {
		var subscriptions []ActivitySubscription
		if arrayErr := json.Unmarshal(response.Data, &subscriptions); arrayErr != nil {
			return ActivitySubscription{}, fmt.Errorf("decode X activity subscription: %w", err)
		}
		if len(subscriptions) > 0 {
			subscription = subscriptions[0]
		}
	}
	if subscription.ID == "" {
		var nested struct {
			Subscription ActivitySubscription `json:"subscription"`
		}
		if err := json.Unmarshal(response.Data, &nested); err != nil {
			return ActivitySubscription{}, fmt.Errorf("decode nested X activity subscription: %w", err)
		}
		subscription = nested.Subscription
	}
	if subscription.ID == "" {
		return ActivitySubscription{}, errors.New("X create activity subscription response missing subscription id")
	}
	if err := validateProviderResourceID(subscription.ID); err != nil {
		return ActivitySubscription{}, err
	}
	return subscription, nil
}

func (c *Client) DeleteActivitySubscription(
	ctx context.Context,
	appBearerToken string,
	subscriptionID string,
) error {
	if err := validateProviderResourceID(subscriptionID); err != nil {
		return err
	}
	path := "/2/activity/subscriptions/" + url.PathEscape(subscriptionID)
	return c.deleteConfirmedProviderResource(ctx, appBearerToken, path, "activity subscription")
}

func (c *Client) deleteConfirmedProviderResource(
	ctx context.Context,
	appBearerToken string,
	path string,
	resourceName string,
) error {
	var response struct {
		Data struct {
			Deleted bool `json:"deleted"`
		} `json:"data"`
		Errors []APIError `json:"errors"`
	}
	status, err := c.do(ctx, http.MethodDelete, path, appBearerToken, nil, &response)
	if err != nil {
		if IsProviderHTTPStatus(err, http.StatusNotFound) || IsProviderHTTPStatus(err, http.StatusGone) {
			return nil
		}
		return err
	}
	if status != http.StatusOK {
		return fmt.Errorf("X delete %s returned an unconfirmed status", resourceName)
	}
	if len(response.Errors) > 0 {
		return fmt.Errorf("X delete %s returned provider errors", resourceName)
	}
	if !response.Data.Deleted {
		return fmt.Errorf("X delete %s response did not confirm deletion", resourceName)
	}
	return nil
}

func validateProviderResourceID(resourceID string) error {
	if len(resourceID) == 0 || len(resourceID) > 19 {
		return errors.New("X provider resource ID is invalid")
	}
	for _, character := range resourceID {
		if character < '0' || character > '9' {
			return errors.New("X provider resource ID is invalid")
		}
	}
	return nil
}
