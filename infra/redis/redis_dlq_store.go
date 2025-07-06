package redis

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
)

type RedisDLQStore struct {
	client *redis.Client
}

// DLQ 저장소를 위한 Redis Client를 초기화 하도록 한다.
func NewRedisDLQStore(client *redis.Client) *RedisDLQStore {
	return &RedisDLQStore{
		client: client,
	}
}

func (s *RedisDLQStore) Save(ctx context.Context, key string, payload []byte, ttl time.Duration) error {
	return s.client.Set(ctx, key, payload, ttl).Err()
}

func (s *RedisDLQStore) Get(ctx context.Context, key string) ([]byte, error) {
	return s.client.Get(ctx, key).Bytes()
}

func (s *RedisDLQStore) Delete(ctx context.Context, key string) error {
	return s.client.Del(ctx, key).Err()
}

func (s *RedisDLQStore) List(ctx context.Context, pattern string) ([]string, error) {
	return s.client.Keys(ctx, pattern).Result()
}