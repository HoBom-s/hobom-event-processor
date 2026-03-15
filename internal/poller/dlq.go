package poller

import (
	"context"
	"log/slog"

	redisClient "github.com/HoBom-s/hobom-event-processor/infra/redis"
)

// saveDLQ persists a failed event payload to the DLQ store.
// Key format: dlq:[category]:[event-id], TTL: 72h.
func saveDLQ(store redisClient.DLQStore, ctx context.Context, prefix, eventId string, value []byte) {
	key := prefix + eventId
	if err := store.Save(ctx, key, value, TTL72Hours); err != nil {
		slog.Error("failed to save DLQ", "eventId", eventId, "err", err)
	}
}
