package handler

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

var googleOAuthRevokeEndpoint = "https://oauth2.googleapis.com/revoke"

func revokeYouTubeOAuthToken(ctx context.Context, client *http.Client, token string) error {
	token = strings.TrimSpace(token)
	if token == "" {
		return nil
	}
	if client == nil {
		client = http.DefaultClient
	}

	form := url.Values{"token": {token}}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, googleOAuthRevokeEndpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	return fmt.Errorf("youtube oauth revoke failed (%d): %s", resp.StatusCode, strings.TrimSpace(string(body)))
}
