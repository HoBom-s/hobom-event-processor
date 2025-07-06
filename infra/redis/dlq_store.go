package redis

import (
	"context"
	"time"
)

type DLQStore interface {
	Save(ctx context.Context, key string, payload []byte, ttl time.Duration) error

	Get(ctx context.Context, key string) ([]byte, error)

	Delete(ctx context.Context, key string) error
	
	List(ctx context.Context, pattern string) ([]string, error)
}