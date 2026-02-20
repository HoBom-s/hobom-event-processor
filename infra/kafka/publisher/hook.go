package publisher

import "context"

// Hook is an optional extension point called before and after each Publish call.
// Useful for logging, metrics, or tracing without modifying core publish logic.
type Hook interface {
	// BeforePublish is called immediately before the message is written to Kafka.
	BeforePublish(ctx context.Context, event Event)
	// AfterPublish is called after the write attempt; err is nil on success.
	AfterPublish(ctx context.Context, event Event, err error)
}