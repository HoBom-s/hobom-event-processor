package dlq

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	angelPb "github.com/HoBom-s/hobom-event-processor/infra/grpc/angel/outbox/v1"
	outboxPb "github.com/HoBom-s/hobom-event-processor/infra/grpc/message/outbox/v1"
	spacePb "github.com/HoBom-s/hobom-event-processor/infra/grpc/space/outbox/v1"
	"github.com/HoBom-s/hobom-event-processor/infra/kafka/publisher"
	"github.com/HoBom-s/hobom-event-processor/infra/redis"
	"github.com/HoBom-s/hobom-event-processor/pkg/utils"
)

// DLQService handles DLQ operations: list, get, and retry.
//
// Retry delegates to the appropriate strategy based on the DLQ category:
//   - Kafka events (menu, log, space, space-log): republish → mark SENT → delete
//
// gRPC clients may be nil when the corresponding backend connection is not
// configured (e.g. spacePatchClient is nil when HOBOM_SPACE_GRPC_ADDR is unset).
type DLQService struct {
	redisDLQ         redis.DLQStore
	publisher        publisher.KafkaPublisher
	patchClient      outboxPb.PatchOutboxControllerClient
	spacePatchClient spacePb.PatchHoBomSpaceOutboxControllerClient
	angelPatchClient angelPb.PatchHoBomAngelOutboxControllerClient
}

func NewService(
	redisDLQ redis.DLQStore,
	pub publisher.KafkaPublisher,
	patchClient outboxPb.PatchOutboxControllerClient,
	spacePatchClient spacePb.PatchHoBomSpaceOutboxControllerClient,
	angelPatchClient angelPb.PatchHoBomAngelOutboxControllerClient,
) *DLQService {
	return &DLQService{
		redisDLQ:         redisDLQ,
		publisher:        pub,
		patchClient:      patchClient,
		spacePatchClient: spacePatchClient,
		angelPatchClient: angelPatchClient,
	}
}

// GetDLQS returns all DLQ keys matching the given prefix.
// An empty prefix matches all "dlq:*" keys.
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

// GetDLQValue returns the raw payload bytes for the given DLQ key.
func (s *DLQService) GetDLQValue(ctx context.Context, key string) ([]byte, error) {
	return s.redisDLQ.Get(ctx, key)
}

// RetryDLQ retries a failed DLQ event by re-executing the original flow.
//
// Retry flow:
//
//	Redis GET(key) → Kafka Publish(topic) → gRPC MarkAsSent → Redis DELETE(key)
//
// The outbox marking target depends on the category:
//   - space/space-log events → hobom-space-backend (spacePatchClient)
//   - all others             → for-hobom-backend (patchClient)
func (s *DLQService) RetryDLQ(ctx context.Context, key string) error {
	k, err := ParseDLQKey(key)
	if err != nil {
		return fmt.Errorf("invalid DLQ key: %w", err)
	}
	if !k.Valid() {
		return fmt.Errorf("invalid DLQ key format")
	}

	data, err := s.redisDLQ.Get(ctx, key)
	if err != nil {
		return fmt.Errorf("failed to get DLQ: %w", err)
	}

	topic, err := k.Topic()
	if err != nil {
		return fmt.Errorf("invalid DLQ key: %w", err)
	}
	if err = s.publisher.Publish(ctx, publisher.Event{
		Key:       key,
		Value:     data,
		Topic:     topic,
		Timestamp: time.Now().UTC(),
	}); err != nil {
		return fmt.Errorf("failed to publish: %w", err)
	}

	// Mark as SENT via the correct backend's gRPC service.
	if k.IsSpaceKey() {
		if s.spacePatchClient == nil {
			return fmt.Errorf("space gRPC connection not available")
		}
		if _, err := s.spacePatchClient.PatchOutboxMarkAsSentUseCase(ctx, &spacePb.MarkRequest{
			EventId: k.EventID,
		}); err != nil {
			slog.Warn("failed to mark space outbox as SENT after DLQ retry", "eventId", k.EventID, "err", err)
			return err
		}
	} else if k.IsAngelKey() {
		if s.angelPatchClient == nil {
			return fmt.Errorf("angel gRPC connection not available")
		}
		if _, err := s.angelPatchClient.PatchOutboxMarkAsSentUseCase(ctx, &angelPb.MarkRequest{
			EventId: k.EventID,
		}); err != nil {
			slog.Warn("failed to mark angel outbox as SENT after DLQ retry", "eventId", k.EventID, "err", err)
			return err
		}
	} else {
		if _, err := s.patchClient.PatchOutboxMarkAsSentUseCase(ctx, &outboxPb.MarkRequest{
			EventId: k.EventID,
		}); err != nil {
			slog.Warn("failed to mark as SENT after DLQ retry", "eventId", k.EventID, "err", err)
			return err
		}
	}

	if err := s.redisDLQ.Delete(ctx, key); err != nil {
		slog.Warn("failed to delete DLQ after retry", "key", key, "err", err)
	}

	return nil
}
