package dlq

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	outboxPb "github.com/HoBom-s/hobom-event-processor/infra/grpc/message/outbox/v1"
	"github.com/HoBom-s/hobom-event-processor/infra/kafka/publisher"
	"github.com/HoBom-s/hobom-event-processor/infra/redis"
	"github.com/HoBom-s/hobom-event-processor/pkg/utils"
)

type DLQService struct {
	redisDLQ    redis.DLQStore
	publisher   publisher.KafkaPublisher
	patchClient outboxPb.PatchOutboxControllerClient
}

// NewService creates a DLQService with the given dependencies.
func NewService(redisDLQ redis.DLQStore, pub publisher.KafkaPublisher, patchClient outboxPb.PatchOutboxControllerClient) *DLQService {
	return &DLQService{
		redisDLQ:    redisDLQ,
		publisher:   pub,
		patchClient: patchClient,
	}
}

// GetDLQS returns all DLQ keys. If prefix is non-empty, only keys with that
// prefix are returned. An empty prefix matches all dlq:* keys.
func (s *DLQService) GetDLQS(ctx context.Context, prefix string) ([]string, error) {
	dlqPrefix := "dlq:*"
	if !utils.IsEmptyString(prefix) {
		dlqPrefix = prefix + "*"
	}

	keys, err := s.redisDLQ.List(ctx, dlqPrefix)
	if err != nil {
		return nil, fmt.Errorf("failed to list DLQ keys: %w", err)
	}
	return keys, nil
}

// GetDLQValue returns the raw payload for the given DLQ key.
// Returns an error if the key does not exist.
func (s *DLQService) GetDLQValue(ctx context.Context, key string) ([]byte, error) {
	return s.redisDLQ.Get(ctx, key)
}

// RetryDLQ republishes the stored event to Kafka, marks the outbox as SENT via
// gRPC, and removes the key from the DLQ store. Returns an error if any of
// the first two steps fail; DLQ deletion failure is logged but not returned.
func (s *DLQService) RetryDLQ(ctx context.Context, key string) error {
	data, err := s.redisDLQ.Get(ctx, key)
	if err != nil {
		return fmt.Errorf("failed to get DLQ: %w", err)
	}

	// Event를 재발행 하도록 한다.
	if err = s.publisher.Publish(ctx, publisher.Event{
		Key:       key,
		Value:     data,
		Topic:     inferTopicFromKey(key),
		Timestamp: time.Now().UTC(),
	}); err != nil {
		return fmt.Errorf("failed to publish: %w", err)
	}

	// Key로부터 EventID를 추출한 후,
	// gRPC 호출을 통해, Outbox에 발행 상태를 `SENT`로 업데이트 시키도록 한다.
	// 만약 EventID가 존재하지 않는다면 다음 로직을 수행하지 않도록 한다.
	eventId := extractEventIdFromKey(key)
	if utils.IsEmptyString(eventId) {
		return fmt.Errorf("invalid DLQ key format, cannot extract event ID from: %s", key)
	}
	if _, err := s.patchClient.PatchOutboxMarkAsSentUseCase(ctx, &outboxPb.MarkRequest{
		EventId: eventId,
	}); err != nil {
		slog.Warn("failed to mark as SENT after DLQ retry", "eventId", eventId, "err", err)
		return err
	}

	// DLQ를 제거하도록 한다.
	if err := s.redisDLQ.Delete(ctx, key); err != nil {
		slog.Warn("failed to delete DLQ after retry", "key", key, "err", err)
	}

	return nil
}
