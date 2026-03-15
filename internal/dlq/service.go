package dlq

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	lawOutboxPb "github.com/HoBom-s/hobom-event-processor/infra/grpc/law/outbox/v1"
	lawPb "github.com/HoBom-s/hobom-event-processor/infra/grpc/law/v1"
	llmPb "github.com/HoBom-s/hobom-event-processor/infra/grpc/llm/v1"
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
//   - Law events: LLM generate → save → mark SENT → delete
//
// gRPC clients may be nil when the corresponding backend connection is not
// configured (e.g. spacePatchClient is nil when HOBOM_SPACE_GRPC_ADDR is unset).
type DLQService struct {
	redisDLQ         redis.DLQStore
	publisher        publisher.KafkaPublisher
	patchClient      outboxPb.PatchOutboxControllerClient
	spacePatchClient spacePb.PatchHoBomSpaceOutboxControllerClient
	llmClient        llmPb.StudyMaterialServiceClient
	saveClient       lawPb.SaveStudyMaterialControllerClient
}

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
// Retry flow (non-law events):
//
//	Redis GET(key) → Kafka Publish(topic) → gRPC MarkAsSent → Redis DELETE(key)
//
// Retry flow (law events):
//
//	Redis GET(key) → LLM Generate → gRPC SaveStudyMaterial → gRPC MarkAsSent → Redis DELETE(key)
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

	// Law events bypass Kafka — re-execute the full orchestration.
	if k.IsLawKey() {
		return s.retryLawEvent(ctx, k, data)
	}

	// All other events: republish to Kafka.
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

// retryLawEvent re-executes the full law study material pipeline:
//
//  1. Unmarshal the DLQ payload back into LawChangedPayload.
//  2. Call LLM Generate() to produce study material.
//  3. Save the study material to for-hobom-backend via gRPC.
//  4. Mark the outbox as SENT.
//  5. Delete the DLQ entry.
//
// Note: Unlike the poller's law flow, DLQ retry does NOT use retryWithBackoff
// for the LLM call — the operator is already manually retrying, so one attempt
// with a clear error is more useful than silent retries.
func (s *DLQService) retryLawEvent(ctx context.Context, k DLQKey, data []byte) error {
	if s.llmClient == nil || s.saveClient == nil {
		return fmt.Errorf("LLM gRPC connection not available for law DLQ retry")
	}

	var payload lawOutboxPb.LawChangedPayload
	if err := json.Unmarshal(data, &payload); err != nil {
		return fmt.Errorf("failed to unmarshal law DLQ payload: %w", err)
	}

	// Step 1: Build ArticleChange list, skipping nil entries.
	var changes []*llmPb.ArticleChange
	for _, c := range payload.Changes {
		if c == nil {
			continue
		}
		changes = append(changes, &llmPb.ArticleChange{
			ArticleNo:  c.ArticleNo,
			ChangeType: c.ChangeType,
			Before:     c.Before,
			After:      c.After,
		})
	}

	// Step 2: Call LLM to generate study material.
	llmRes, err := s.llmClient.Generate(ctx, &llmPb.StudyMaterialRequest{
		Changes: changes,
	})
	if err != nil {
		return fmt.Errorf("LLM generate failed: %w", err)
	}

	// Step 3: Save to backend.
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

	// Step 4: Mark outbox as SENT.
	if _, err = s.patchClient.PatchOutboxMarkAsSentUseCase(ctx, &outboxPb.MarkRequest{
		EventId: k.EventID,
	}); err != nil {
		return fmt.Errorf("failed to mark law outbox as SENT: %w", err)
	}

	// Step 5: Delete DLQ entry.
	if err := s.redisDLQ.Delete(ctx, k.String()); err != nil {
		slog.Warn("failed to delete law DLQ after retry", "key", k.String(), "err", err)
	}

	return nil
}
