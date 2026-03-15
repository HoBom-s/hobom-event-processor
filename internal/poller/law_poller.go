package poller

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	lawOutboxPb "github.com/HoBom-s/hobom-event-processor/infra/grpc/law/outbox/v1"
	lawPb "github.com/HoBom-s/hobom-event-processor/infra/grpc/law/v1"
	llmPb "github.com/HoBom-s/hobom-event-processor/infra/grpc/llm/v1"
	patchPb "github.com/HoBom-s/hobom-event-processor/infra/grpc/message/outbox/v1"
	redisClient "github.com/HoBom-s/hobom-event-processor/infra/redis"
	"google.golang.org/grpc"
)

type lawPoller struct {
	findClient  lawOutboxPb.FindHoBomLawOutboxControllerClient
	patchClient patchPb.PatchOutboxControllerClient
	llmClient   llmPb.StudyMaterialServiceClient
	saveClient  lawPb.SaveStudyMaterialControllerClient
	redisDLQ    redisClient.DLQStore
}

// NewLawPoller creates a poller that orchestrates privacy-law study material generation.
// Flow: poll LAW_CHANGED → call LLM gRPC → save result to backend → mark SENT.
// conn is the for-hobom-backend gRPC connection (find outbox + patch + save study material).
// llmConn is the hobom-llm-service-backend gRPC connection (generate study material).
func NewLawPoller(conn *grpc.ClientConn, llmConn *grpc.ClientConn, redisDLQ redisClient.DLQStore) Poller {
	return &lawPoller{
		findClient:  lawOutboxPb.NewFindHoBomLawOutboxControllerClient(conn),
		patchClient: patchPb.NewPatchOutboxControllerClient(conn),
		llmClient:   llmPb.NewStudyMaterialServiceClient(llmConn),
		saveClient:  lawPb.NewSaveStudyMaterialControllerClient(conn),
		redisDLQ:    redisDLQ,
	}
}

func (p *lawPoller) Poll(ctx context.Context) error {
	req := &lawOutboxPb.Request{
		EventType: EventTypeLawChanged.String(),
		Status:    OutboxPending.String(),
	}

	res, err := p.findClient.FindOutboxByEventTypeAndStatusUseCase(ctx, req)
	if err != nil {
		return fmt.Errorf("failed to fetch law outbox: %w", err)
	}

	for _, item := range res.Items {
		p.handleLawChanged(ctx, item)
	}
	return nil
}

func (p *lawPoller) handleLawChanged(ctx context.Context, item *lawOutboxPb.QueryResult) {
	payload := item.Payload
	if payload == nil {
		slog.Error("law outbox payload is nil", "eventId", item.EventId)
		p.markAsFailed(ctx, item.EventId, "payload is nil")
		return
	}

	// 1. LLM gRPC 호출: 변경 조문으로 학습자료 생성 (with retry)
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

	var llmRes *llmPb.StudyMaterialResponse
	err := retryWithBackoff(ctx, 3, 500*time.Millisecond, func() error {
		var genErr error
		llmRes, genErr = p.llmClient.Generate(ctx, &llmPb.StudyMaterialRequest{
			Changes: changes,
		})
		return genErr
	})
	if err != nil {
		slog.Error("LLM generate failed", "eventId", item.EventId, "err", err)
		p.markAsFailed(ctx, item.EventId, fmt.Sprintf("LLM generate failed: %v", err))
		p.saveToDLQ(ctx, item.EventId, payload)
		return
	}

	// 2. for-hobom-backend에 학습자료 저장 gRPC 호출
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

	if _, err = p.saveClient.SaveStudyMaterial(ctx, &lawPb.SaveStudyMaterialRequest{
		DiffId:    payload.DiffId,
		Summary:   llmRes.Summary,
		KeyPoints: llmRes.KeyPoints,
		Quizzes:   quizzes,
	}); err != nil {
		slog.Error("save study material failed", "eventId", item.EventId, "err", err)
		p.markAsFailed(ctx, item.EventId, fmt.Sprintf("save study material failed: %v", err))
		p.saveToDLQ(ctx, item.EventId, payload)
		return
	}

	// 3. outbox SENT 마킹
	if err := p.markAsSent(ctx, item.EventId); err != nil {
		slog.Warn("saved study material but failed to mark law as SENT", "eventId", item.EventId, "err", err)
	}
}

func (p *lawPoller) markAsSent(ctx context.Context, eventId string) error {
	slog.Info("marking law outbox as SENT", "eventId", eventId)
	if _, err := p.patchClient.PatchOutboxMarkAsSentUseCase(ctx, &patchPb.MarkRequest{
		EventId: eventId,
	}); err != nil {
		return fmt.Errorf("failed to mark law outbox as SENT: %w", err)
	}
	return nil
}

func (p *lawPoller) markAsFailed(ctx context.Context, eventId, reason string) {
	if _, err := p.patchClient.PatchOutboxMarkAsFailedUseCase(ctx, &patchPb.MarkFailedRequest{
		EventId:      eventId,
		ErrorMessage: reason,
	}); err != nil {
		slog.Error("failed to mark law outbox as FAILED", "eventId", eventId, "err", err)
	}
}

func (p *lawPoller) saveToDLQ(ctx context.Context, eventId string, payload *lawOutboxPb.LawChangedPayload) {
	jsonValue, err := json.Marshal(payload)
	if err != nil {
		slog.Error("failed to marshal law payload for DLQ", "eventId", eventId, "err", err)
		return
	}
	saveDLQ(p.redisDLQ, ctx, HoBomLawDLQPrefix, eventId, jsonValue)
}
