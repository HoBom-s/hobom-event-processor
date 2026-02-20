package poller

import (
	"context"
	"encoding/json"
	"time"

	publisher "github.com/HoBom-s/hobom-event-processor/infra/kafka/publisher"
)

// publishWithRetry publishes an event to Kafka with exponential backoff.
// Retries up to 3 times (200ms → 400ms) before returning the final error.
func publishWithRetry(ctx context.Context, pub publisher.KafkaPublisher, event publisher.Event) error {
	const maxAttempts = 3
	delay := 200 * time.Millisecond
	var err error
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		if err = pub.Publish(ctx, event); err == nil {
			return nil
		}
		if attempt < maxAttempts {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(delay):
			}
			delay *= 2
		}
	}
	return err
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
