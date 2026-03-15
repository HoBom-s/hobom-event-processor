package poller

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	publisher "github.com/HoBom-s/hobom-event-processor/infra/kafka/publisher"
)

// retryWithBackoff executes fn up to maxAttempts times with exponential backoff.
// It checks ctx before each attempt; returns ctx.Err() immediately if cancelled.
func retryWithBackoff(ctx context.Context, maxAttempts int, initialDelay time.Duration, fn func() error) error {
	delay := initialDelay
	var err error
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if err = fn(); err == nil {
			return nil
		}
		if attempt < maxAttempts {
			slog.Warn("retrying after failure", "attempt", attempt, "maxAttempts", maxAttempts, "err", err)
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(delay):
			}
			delay *= 2
		}
	}
	return fmt.Errorf("failed after %d attempts: %w", maxAttempts, err)
}

// publishWithRetry publishes an event to Kafka with exponential backoff.
// Retries up to 3 times (200ms → 400ms) before returning the final error.
func publishWithRetry(ctx context.Context, pub publisher.KafkaPublisher, event publisher.Event) error {
	return retryWithBackoff(ctx, 3, 200*time.Millisecond, func() error {
		return pub.Publish(ctx, event)
	})
}

func structToMap(v interface{}) (map[string]interface{}, error) {
	data, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	var result map[string]interface{}
	err = json.Unmarshal(data, &result)
	return result, err
}
