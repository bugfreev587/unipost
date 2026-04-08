// Package events defines the EventBus interface used by the publish
// path (handler.SocialPostHandler + worker.SchedulerWorker) to fan out
// post.published / post.partial / post.failed / account.disconnected
// events to subscriber webhooks. The actual delivery worker lives in
// internal/worker — this package exists just so handler doesn't have
// to import worker (which would form a cycle: worker → handler is
// fine, handler → worker is not).
//
// EventBus is intentionally tiny — one method, no return value the
// caller has to plumb. Webhook delivery is best-effort: a failed
// enqueue must NEVER block or fail the publish path. Implementations
// log errors and recover from panics.
package events

import "context"

// EventBus is the publisher-side interface. Implementations live in
// internal/worker (real delivery) or in tests (recording stubs).
type EventBus interface {
	// Publish enqueues an event for every webhook subscription in
	// the project that's listening for the named event. Returns
	// nothing — failures are logged, never propagated.
	//
	// data is the JSON-serializable event body. By convention it's
	// the post / account object the event is about, plus any extra
	// fields documented in the event-specific schema.
	Publish(ctx context.Context, projectID, event string, data any)
}

// Standard event names. Keep these in sync with the events documented
// in the public API reference.
const (
	EventPostPublished       = "post.published"
	EventPostPartial         = "post.partial"
	EventPostFailed          = "post.failed"
	EventAccountConnected    = "account.connected"
	EventAccountDisconnected = "account.disconnected"
)

// NoopBus is a safe default for tests and for cmd setups that haven't
// wired the worker yet. Publish is a no-op so handler code can call it
// unconditionally.
type NoopBus struct{}

func (NoopBus) Publish(ctx context.Context, projectID, event string, data any) {}
