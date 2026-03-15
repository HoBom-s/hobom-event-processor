package poller

import (
	"context"
	"log/slog"

	redisClient "github.com/HoBom-s/hobom-event-processor/infra/redis"
)

// saveDLQ persists a failed event payload to the Redis DLQ store.
//
// Called when a poller fails to publish an event to Kafka (after all retries)
// or when the LLM call fails for law events. The DLQ entry enables manual
// retry via the DLQ HTTP API (POST /dlq/retry/:key).
//
// Key format: dlq:<category>:<event-id> (prefix already includes "dlq:<category>:").
// TTL: 72 hours — after which the entry auto-expires if not retried.
func saveDLQ(store redisClient.DLQStore, ctx context.Context, prefix, eventId string, value []byte) {
	key := prefix + eventId
	if err := store.Save(ctx, key, value, TTL72Hours); err != nil {
		slog.Error("failed to save DLQ", "eventId", eventId, "err", err)
	}
}
