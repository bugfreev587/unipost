// Package mail abstracts transactional email delivery. The only
// concrete implementation today is ResendMailer; tests can substitute
// StubMailer (captures calls) or NoopMailer (drops silently).
//
// Kept tiny on purpose: one method, a single Message struct, no
// template engine. Templating lives in internal/worker/notification.go
// where the delivery worker composes the body from the event payload.
package mail

import "context"

// Message is a rendered email ready for the wire.
type Message struct {
	To      string // single recipient; we don't currently batch
	Subject string
	HTML    string // UTF-8 HTML body
	Text    string // plaintext fallback; optional but improves deliverability
}

// Mailer sends a rendered Message. Implementations must be safe for
// concurrent use and must return a non-nil error if the provider
// rejected the message or the network call failed.
type Mailer interface {
	Send(ctx context.Context, msg Message) error
}

// NoopMailer silently drops every message. Useful in local dev when
// RESEND_API_KEY is unset — the dispatcher still inserts delivery
// rows, the worker still marks them sent, but no email leaves the
// machine. Prevents a local developer from accidentally spamming
// real users in their account.
type NoopMailer struct{}

func (NoopMailer) Send(ctx context.Context, msg Message) error { return nil }
