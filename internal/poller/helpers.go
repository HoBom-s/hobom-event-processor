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
//
// Backoff sequence: initialDelay, initialDelay*2, initialDelay*4, ...
// Between attempts, it checks ctx for cancellation — if cancelled, returns
// ctx.Err() immediately without further retries.
//
// On exhaustion, returns a wrapped error: "failed after N attempts: <last error>".
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

// publishWithRetry publishes a single event to Kafka with retry.
// Uses 3 attempts with exponential backoff starting at 200ms (200ms → 400ms).
// This is the standard retry policy for all Kafka publishes in the poller layer.
func publishWithRetry(ctx context.Context, pub publisher.KafkaPublisher, event publisher.Event) error {
	return retryWithBackoff(ctx, 3, 200*time.Millisecond, func() error {
		return pub.Publish(ctx, event)
	})
}

// structToMap converts a protobuf struct to map[string]interface{} via
// JSON round-trip. Used by log pollers to embed the full gRPC payload
// as a nested JSON object in the Kafka message.
func structToMap(v interface{}) (map[string]interface{}, error) {
	data, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	var result map[string]interface{}
	err = json.Unmarshal(data, &result)
	return result, err
}
