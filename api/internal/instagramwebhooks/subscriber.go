package instagramwebhooks

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const defaultGraphBase = "https://graph.instagram.com/v21.0"

type Subscriber struct {
	client    *http.Client
	graphBase string
}

func NewSubscriber(client *http.Client, graphBase string) *Subscriber {
	if client == nil {
		client = &http.Client{Timeout: 15 * time.Second}
	}
	if strings.TrimSpace(graphBase) == "" {
		graphBase = defaultGraphBase
	}
	return &Subscriber{
		client:    client,
		graphBase: strings.TrimRight(graphBase, "/"),
	}
}

func (s *Subscriber) Subscribe(ctx context.Context, accountID, accessToken string) error {
	form := url.Values{
		"access_token":      {accessToken},
		"subscribed_fields": {"messages,messaging_postbacks,comments"},
	}
	endpoint := fmt.Sprintf("%s/%s/subscribed_apps", s.graphBase, url.PathEscape(accountID))
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return fmt.Errorf("instagram webhook subscription request could not be created")
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("instagram webhook subscription request failed")
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("instagram webhook subscription failed (%d)", resp.StatusCode)
	}

	var result struct {
		Success bool `json:"success"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return fmt.Errorf("instagram webhook subscription response invalid")
	}
	if !result.Success {
		return fmt.Errorf("instagram webhook subscription rejected")
	}
	return nil
}
