package ratelimit

// Error codes returned to API clients on a 429. The HTTP-level code
// stays "RATE_LIMITED" across all three controls so naive clients
// can branch on one code; the normalized_code carries the specific
// reason so SDKs and dashboards can distinguish request bursts from
// queue-depth saturation.
const (
	// CodeRateLimited is the umbrella HTTP error code for any
	// admission-control rejection. Matches §6.1 of the PRD.
	CodeRateLimited = "RATE_LIMITED"

	// NormRequestLimited is the normalized_code for a per-workspace
	// request-rate breach (token bucket empty).
	NormRequestLimited = "rate_limited"

	// NormEnqueueLimited is the normalized_code for an enqueue
	// throughput breach (sliding window full).
	NormEnqueueLimited = "enqueue_rate_limited"

	// NormQueueDepthExceeded is the normalized_code for a workspace
	// queue-depth breach (too many active delivery jobs).
	NormQueueDepthExceeded = "queue_depth_exceeded"
)
