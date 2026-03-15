package dlq

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	lawOutboxPb "github.com/HoBom-s/hobom-event-processor/infra/grpc/law/outbox/v1"
	lawPb "github.com/HoBom-s/hobom-event-processor/infra/grpc/law/v1"
	llmPb "github.com/HoBom-s/hobom-event-processor/infra/grpc/llm/v1"
	outboxPb "github.com/HoBom-s/hobom-event-processor/infra/grpc/message/outbox/v1"
	spacePb "github.com/HoBom-s/hobom-event-processor/infra/grpc/space/outbox/v1"
	"github.com/HoBom-s/hobom-event-processor/infra/kafka/publisher"
	"github.com/HoBom-s/hobom-event-processor/infra/redis"
	poller "github.com/HoBom-s/hobom-event-processor/internal/poller"
	"github.com/HoBom-s/hobom-event-processor/pkg/utils"
)

type DLQService struct {
	redisDLQ         redis.DLQStore
	publisher        publisher.KafkaPublisher
	patchClient      outboxPb.PatchOutboxControllerClient
	spacePatchClient spacePb.PatchHoBomSpaceOutboxControllerClient
	llmClient        llmPb.StudyMaterialServiceClient
	saveClient       lawPb.SaveStudyMaterialControllerClient
}

// NewService creates a DLQService with the given dependencies.
// spacePatchClient may be nil if the space gRPC connection is not configured.
// llmClient and saveClient may be nil if the LLM gRPC connection is not configured.
func NewService(
	redisDLQ redis.DLQStore,
	pub publisher.KafkaPublisher,
	patchClient outboxPb.PatchOutboxControllerClient,
	spacePatchClient spacePb.PatchHoBomSpaceOutboxControllerClient,
	llmClient llmPb.StudyMaterialServiceClient,
	saveClient lawPb.SaveStudyMaterialControllerClient,
) *DLQService {
	return &DLQService{
		redisDLQ:         redisDLQ,
		publisher:        pub,
		patchClient:      patchClient,
		spacePatchClient: spacePatchClient,
		llmClient:        llmClient,
		saveClient:       saveClient,
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

// RetryDLQ retries a failed DLQ event. For law events, this re-executes the
// LLM generate → save → mark SENT orchestration. For all other events, it
// republishes to Kafka and marks the outbox as SENT.
func (s *DLQService) RetryDLQ(ctx context.Context, key string) error {
	data, err := s.redisDLQ.Get(ctx, key)
	if err != nil {
		return fmt.Errorf("failed to get DLQ: %w", err)
	}

	// Law events bypass Kafka — re-execute the orchestration flow.
	if strings.HasPrefix(key, poller.HoBomLawDLQPrefix) {
		return s.retryLawEvent(ctx, key, data)
	}

	topic, err := inferTopicFromKey(key)
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

	eventId := extractEventIdFromKey(key)
	if utils.IsEmptyString(eventId) {
		return fmt.Errorf("invalid DLQ key format")
	}

	// Space DLQ 이벤트의 경우 hobom-space-backend의 gRPC를 통해 마킹한다.
	if strings.HasPrefix(key, poller.HoBomSpaceLogDLQPrefix) || strings.HasPrefix(key, poller.HoBomSpaceDLQPrefix) {
		if s.spacePatchClient == nil {
			return fmt.Errorf("space gRPC connection not available")
		}
		if _, err := s.spacePatchClient.PatchOutboxMarkAsSentUseCase(ctx, &spacePb.MarkRequest{
			EventId: eventId,
		}); err != nil {
			slog.Warn("failed to mark space outbox as SENT after DLQ retry", "eventId", eventId, "err", err)
			return err
		}
	} else {
		if _, err := s.patchClient.PatchOutboxMarkAsSentUseCase(ctx, &outboxPb.MarkRequest{
			EventId: eventId,
		}); err != nil {
			slog.Warn("failed to mark as SENT after DLQ retry", "eventId", eventId, "err", err)
			return err
		}
	}

	if err := s.redisDLQ.Delete(ctx, key); err != nil {
		slog.Warn("failed to delete DLQ after retry", "key", key, "err", err)
	}

	return nil
}

func (s *DLQService) retryLawEvent(ctx context.Context, key string, data []byte) error {
	if s.llmClient == nil || s.saveClient == nil {
		return fmt.Errorf("LLM gRPC connection not available for law DLQ retry")
	}

	var payload lawOutboxPb.LawChangedPayload
	if err := json.Unmarshal(data, &payload); err != nil {
		return fmt.Errorf("failed to unmarshal law DLQ payload: %w", err)
	}

	// 1. LLM gRPC: generate study material
	changes := make([]*llmPb.ArticleChange, len(payload.Changes))
	for i, c := range payload.Changes {
		changes[i] = &llmPb.ArticleChange{
			ArticleNo:  c.ArticleNo,
			ChangeType: c.ChangeType,
			Before:     c.Before,
			After:      c.After,
		}
	}

	llmRes, err := s.llmClient.Generate(ctx, &llmPb.StudyMaterialRequest{
		Changes: changes,
	})
	if err != nil {
		return fmt.Errorf("LLM generate failed: %w", err)
	}

	// 2. Save study material to backend
	quizzes := make([]*lawPb.Quiz, len(llmRes.Quizzes))
	for i, q := range llmRes.Quizzes {
		quizzes[i] = &lawPb.Quiz{
			Type:        q.Type,
			Question:    q.Question,
			Answer:      q.Answer,
			Explanation: q.Explanation,
			Choices:     q.Choices,
		}
	}

	if _, err = s.saveClient.SaveStudyMaterial(ctx, &lawPb.SaveStudyMaterialRequest{
		DiffId:    payload.DiffId,
		Summary:   llmRes.Summary,
		KeyPoints: llmRes.KeyPoints,
		Quizzes:   quizzes,
	}); err != nil {
		return fmt.Errorf("save study material failed: %w", err)
	}

	// 3. Mark outbox as SENT
	eventId := extractEventIdFromKey(key)
	if utils.IsEmptyString(eventId) {
		return fmt.Errorf("invalid DLQ key format")
	}
	if _, err = s.patchClient.PatchOutboxMarkAsSentUseCase(ctx, &outboxPb.MarkRequest{
		EventId: eventId,
	}); err != nil {
		return fmt.Errorf("failed to mark law outbox as SENT: %w", err)
	}

	// 4. Delete DLQ entry
	if err := s.redisDLQ.Delete(ctx, key); err != nil {
		slog.Warn("failed to delete law DLQ after retry", "key", key, "err", err)
	}

	return nil
}
