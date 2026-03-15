// Package redis provides the DLQ (Dead Letter Queue) storage abstraction
// and its Redis implementation.
//
// DLQ entries are created by pollers when event processing fails (Kafka
// publish error, LLM call error, etc.). They are inspected and retried
// via the DLQ HTTP API.
//
// Key format: "dlq:<category>:<event-id>"
// TTL: 72 hours (auto-expire if not manually retried).
package redis

import (
	"context"
	"time"
)

// DLQStore is the port for persisting and querying Dead Letter Queue entries.
type DLQStore interface {
	// Save stores payload under key with the given TTL.
	Save(ctx context.Context, key string, payload []byte, ttl time.Duration) error
	// Get retrieves the raw payload for key. Returns an error if the key does not exist.
	Get(ctx context.Context, key string) ([]byte, error)
	// Delete removes a key from the store.
	Delete(ctx context.Context, key string) error
	// List returns all keys matching the glob pattern (e.g. "dlq:menu:*").
	List(ctx context.Context, pattern string) ([]string, error)
}
