package dlq

import (
	"context"
	"fmt"
	"time"

	outboxPb "github.com/HoBom-s/hobom-event-processor/infra/grpc/message/outbox/v1"
	"github.com/HoBom-s/hobom-event-processor/infra/kafka/publisher"
	"github.com/HoBom-s/hobom-event-processor/infra/redis"
	"github.com/HoBom-s/hobom-event-processor/pkg/utils"
)

type DLQService struct {
	RedisDLQ  *redis.RedisDLQStore
	Publisher publisher.KafkaPublisher
	PatchClient outboxPb.PatchOutboxControllerClient
}

// DLQ Service를 초기화 하도록 한다.
func NewService(redis *redis.RedisDLQStore, pub publisher.KafkaPublisher, patchClient outboxPb.PatchOutboxControllerClient) *DLQService {
	return &DLQService{
		RedisDLQ:  redis,
		Publisher: pub,
		PatchClient: patchClient,
	}
}

// DLQ 저장소에서 DLQ Key 목록을 조회한다.
// 기본적으로 모든 DLQ(key: dlq:*)를 조회하며, 선택적으로 `prefix` 필터링도 가능하다.
func (s * DLQService) GetDLQS(ctx context.Context, prefix string) ([]string, error) {
	var dlqPrefix = ""
	if utils.IsEmptyString(prefix) {
		dlqPrefix = "dlq:*"
	} else {
		dlqPrefix = prefix + "*"
	}

	keys, err := s.RedisDLQ.List(ctx, dlqPrefix)
	if err != nil {
		return nil, fmt.Errorf("failed to list DLQ keys: %w", err)
	}
	return keys, nil
}

// DLQ 저장소에서 Key에 해당하는
// DLQ Payload를 가져오도록 한다.
// 만약 해당 Key값이 존재하지 않으면 에러를 발생시킨다.
func (s *DLQService) GetDLQValue(ctx context.Context, key string) ([]byte, error) {
	return s.RedisDLQ.Get(ctx, key)
}

// DLQ를 재발행 하도록 한다.
// DLQ가 없다면 에러가 발생하고, 존재한다면, Event를 재발행 하도록 한다.
// 이벤트를 재발행 한 후, DLQ를 Redis에서 제거하도록 한다.
func (s *DLQService) RetryDLQ(ctx context.Context, key string) error {
	data, err := s.RedisDLQ.Get(ctx, key)
	if err != nil {
		return fmt.Errorf("failed to get DLQ: %w", err)
	}

	// Event를 재발행 하도록 한다.
	err = s.Publisher.Publish(ctx, publisher.Event{
		Key:       key,
		Value:     data,
		Topic:     inferTopicFromKey(key),
		Timestamp: time.Now().UTC(),
	})
	if err != nil {
		return fmt.Errorf("failed to publish: %w", err)
	}

	// Key로부터 EventID를 추출한 후,
	// gRPC 호출을 통해, Outbox에 발행 상태를 `SENT`로 업데이트 시키도록 한다.
	// 만약 EventID가 존재하지 않는다면 다음 로직을 수행하지 않도록 한다.
	eventId := extractEventIdFromKey(key)
	if utils.IsEmptyString(eventId) {
		return fmt.Errorf("invalid DLQ key format, cannot extract event ID from: %s", key)
	}
	if _, err := s.PatchClient.PatchOutboxMarkAsSentUseCase(ctx, &outboxPb.MarkRequest{
		EventId: eventId,
	}); err != nil {
		fmt.Printf("⚠️ Failed to mark as SENT after retry: %v\n", err)
		return err
	}

	// DLQ를 제거하도록 한다.
	if err := s.RedisDLQ.Delete(ctx, key); err != nil {
		fmt.Printf("⚠️ Failed to delete DLQ after retry: %v\n", err)
	}

	return nil
}
