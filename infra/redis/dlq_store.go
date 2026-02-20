package redis

import (
	"context"
	"time"
)

// DLQStore is the port for persisting and querying Dead Letter Queue entries.
// Key format convention: dlq:[category]:[event-id]
type DLQStore interface {
	// Save stores payload under key with the given TTL.
	Save(ctx context.Context, key string, payload []byte, ttl time.Duration) error
	// Get retrieves the raw payload for key. Returns an error if the key does not exist.
	Get(ctx context.Context, key string) ([]byte, error)
	// Delete removes a key from the store.
	Delete(ctx context.Context, key string) error
	// List returns all keys matching the glob pattern.
	List(ctx context.Context, pattern string) ([]string, error)
}