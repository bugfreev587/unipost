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

func DMSubscriptionTag(accountID string) string {
	return "unipost:x:dm:" + strings.TrimSpace(accountID)
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
			if !webhook.Valid {
				var response struct {
					Data struct {
						Valid bool `json:"valid"`
					} `json:"data"`
				}
				path := "/2/webhooks/" + url.PathEscape(webhook.ID)
				if err := c.doJSON(ctx, http.MethodPut, path, appBearerToken, nil, &response); err != nil {
					return Webhook{}, err
				}
				webhook.Valid = response.Data.Valid
				if !webhook.Valid {
					return Webhook{}, errors.New("X webhook CRC revalidation did not mark webhook valid")
				}
			}
			return webhook, nil
		}
	}

	var raw json.RawMessage
	status, err := c.do(
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
	if status < 200 || status >= 300 {
		return Webhook{}, fmt.Errorf("X API POST /2/webhooks returned HTTP %d", status)
	}
	var direct Webhook
	if err := json.Unmarshal(raw, &direct); err != nil {
		return Webhook{}, fmt.Errorf("decode X create webhook response: %w", err)
	}
	if direct.ID != "" {
		return direct, nil
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
	return wrapped.Data, nil
}

func (c *Client) ListActivitySubscriptions(
	ctx context.Context,
	userAccessToken string,
) ([]ActivitySubscription, error) {
	var response struct {
		Data []ActivitySubscription `json:"data"`
	}
	if err := c.doJSON(ctx, http.MethodGet, "/2/activity/subscriptions", userAccessToken, nil, &response); err != nil {
		return nil, err
	}
	return response.Data, nil
}

func (c *Client) EnsureDMSubscription(
	ctx context.Context,
	userAccessToken string,
	accountID string,
	userID string,
	webhookID string,
) (ActivitySubscription, error) {
	subscriptions, err := c.ListActivitySubscriptions(ctx, userAccessToken)
	if err != nil {
		return ActivitySubscription{}, err
	}
	tag := DMSubscriptionTag(accountID)
	for _, subscription := range subscriptions {
		if subscription.Tag == tag {
			if subscription.EventType == "dm.received" &&
				subscription.Filter.UserID == userID &&
				subscription.WebhookID == webhookID {
				return subscription, nil
			}
			if err := c.DeleteActivitySubscription(ctx, userAccessToken, subscription.ID); err != nil {
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
	if err := c.doJSON(ctx, http.MethodPost, "/2/activity/subscriptions", userAccessToken, request, &raw); err != nil {
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
	return subscription, nil
}

func (c *Client) DeleteActivitySubscription(
	ctx context.Context,
	userAccessToken string,
	subscriptionID string,
) error {
	path := "/2/activity/subscriptions/" + url.PathEscape(subscriptionID)
	status, err := c.do(ctx, http.MethodDelete, path, userAccessToken, nil, nil)
	if err != nil {
		return err
	}
	if isIdempotentDeleteStatus(status) {
		return nil
	}
	if status < 200 || status >= 300 {
		return fmt.Errorf("X delete activity subscription returned HTTP %d", status)
	}
	return nil
}
