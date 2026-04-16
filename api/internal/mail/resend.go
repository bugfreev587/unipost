package mail

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// ResendMailer sends email via the Resend HTTP API
// (https://resend.com/docs/api-reference/emails/send-email). Uses the
// REST endpoint rather than their Go SDK so we don't take on a new
// dependency for what amounts to a single POST.
type ResendMailer struct {
	apiKey string
	from   string // "UniPost <notifications@unipost.dev>"
	client *http.Client
}

func NewResendMailer(apiKey, from string) *ResendMailer {
	return &ResendMailer{
		apiKey: apiKey,
		from:   from,
		client: &http.Client{Timeout: 10 * time.Second},
	}
}

type resendRequest struct {
	From    string   `json:"from"`
	To      []string `json:"to"`
	Subject string   `json:"subject"`
	HTML    string   `json:"html,omitempty"`
	Text    string   `json:"text,omitempty"`
}

type resendError struct {
	StatusCode int    `json:"statusCode"`
	Message    string `json:"message"`
	Name       string `json:"name"`
}

func (r *ResendMailer) Send(ctx context.Context, msg Message) error {
	body, err := json.Marshal(resendRequest{
		From:    r.from,
		To:      []string{msg.To},
		Subject: msg.Subject,
		HTML:    msg.HTML,
		Text:    msg.Text,
	})
	if err != nil {
		return fmt.Errorf("resend: marshal: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", "https://api.resend.com/emails", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("resend: new request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+r.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := r.client.Do(req)
	if err != nil {
		return fmt.Errorf("resend: http: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		io.Copy(io.Discard, resp.Body)
		return nil
	}

	// Resend returns a JSON error body on failure; surface its message
	// so the delivery row's last_error is actually useful for triage.
	raw, _ := io.ReadAll(resp.Body)
	var apiErr resendError
	if jsonErr := json.Unmarshal(raw, &apiErr); jsonErr == nil && apiErr.Message != "" {
		return fmt.Errorf("resend: %d %s: %s", resp.StatusCode, apiErr.Name, apiErr.Message)
	}
	return fmt.Errorf("resend: %d: %s", resp.StatusCode, string(raw))
}
