package poller

import (
	"context"
	"fmt"
	"time"

	redisClient "github.com/HoBom-s/hobom-event-processor/infra/redis"
)

const TTL72Hours = 72 * time.Hour

// Redis에 DLQ를 저장하도록 한다.
// TTL: 72 시간, Key: dlq:[category]:[event-id], Payload: JsonValue
func saveDLQ(redis *redisClient.RedisDLQStore, ctx context.Context, prefix, eventId string, value []byte) {
	key := fmt.Sprintf("%s:%s", prefix, eventId)
	err := redis.Save(ctx, key, value, TTL72Hours)
	if err != nil {
		fmt.Printf("❌ Failed to save DLQ for %s: %v\n", eventId, err)
	}
}
