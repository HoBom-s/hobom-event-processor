package poller

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	outboxPb "github.com/HoBom-s/hobom-event-processor/infra/grpc/message/outbox/v1"
	publisher "github.com/HoBom-s/hobom-event-processor/infra/kafka/publisher"
	redisClient "github.com/HoBom-s/hobom-event-processor/infra/redis"
	"google.golang.org/grpc"
)

type messagePoller struct {
	findClient  outboxPb.FindHoBomMessageOutboxControllerClient
	patchClient outboxPb.PatchOutboxControllerClient
	publisher   publisher.KafkaPublisher
	redisDLQ    redisClient.DLQStore
}

func NewMessagePoller(conn *grpc.ClientConn, publisher publisher.KafkaPublisher, redisDLQ redisClient.DLQStore) Poller {
	return &messagePoller{
		findClient:  outboxPb.NewFindHoBomMessageOutboxControllerClient(conn),
		patchClient: outboxPb.NewPatchOutboxControllerClient(conn),
		publisher:   publisher,
		redisDLQ:    redisDLQ,
	}
}

// gRPC 통신을 통해 for-hobom-backend 서버의 Outbox DB 를 polling 하도록 한다.
// Payload에는 다른 사용자에게 Message를 전송하기 위한 데이터를 가지고 있다.
// Outbox Status 가 `PENDING` 인 것을 가져오도록 한다.
func (p *messagePoller) Poll(ctx context.Context) {
	req := &outboxPb.Request{
		EventType: EventTypeHoBomMessage,
		Status:    OutboxPending,
	}

	res, err := p.findClient.FindOutboxByEventTypeAndStatusUseCase(ctx, req)
	if err != nil {
		slog.Error("failed to fetch message outbox", "err", err)
		return
	}

	for _, item := range res.Items {
		p.handleMessage(ctx, item)
	}
}

func (p *messagePoller) handleMessage(ctx context.Context, item *outboxPb.QueryResult) {
	senderId := item.Payload.SenderId
	cmd := DeliverHoBomMessageCommand{
		Type:      Mail,
		Title:     item.Payload.Title,
		Body:      item.Payload.Body,
		Recipient: item.Payload.Recipient,
		SenderId:  &senderId,
		SentAt:    time.Now(),
	}
	p.publishAndMark(ctx, item.EventId, cmd, HoBomMessage)
}

func (p *messagePoller) publishAndMark(
	ctx context.Context,
	eventId string,
	cmd DeliverHoBomMessageCommand,
	topic string,
) {
	jsonValue, err := json.Marshal(cmd)
	if err != nil {
		slog.Error("failed to marshal message payload", "eventId", eventId, "err", err)
		p.markAsFailed(ctx, eventId, fmt.Sprintf("failed to marshal payload: %v", err))
		return
	}

	if err = publishWithRetry(ctx, p.publisher, publisher.Event{
		Key:       eventId,
		Value:     jsonValue,
		Topic:     topic,
		Timestamp: time.Now(),
	}); err != nil {
		slog.Error("kafka publish failed", "eventId", eventId, "err", err)
		p.markAsFailed(ctx, eventId, fmt.Sprintf("kafka publish failed: %v", err))
		saveDLQ(p.redisDLQ, ctx, HoBomTodayMenuDLQPrefix, eventId, jsonValue)
		return
	}

	p.markAsSent(ctx, eventId)
}

// gRPC 통신을 통해, for-hobom-backend 서버에 Outbox 데이터 업데이트를 위한 통신을 수행하도록 한다.
// Outbox DB 에 `SENT` 상태로 업데이트를 한다.
func (p *messagePoller) markAsSent(ctx context.Context, eventId string) {
	slog.Info("marking message outbox as SENT", "eventId", eventId)
	if _, err := p.patchClient.PatchOutboxMarkAsSentUseCase(ctx, &outboxPb.MarkRequest{
		EventId: eventId,
	}); err != nil {
		slog.Error("failed to mark message outbox as SENT", "eventId", eventId, "err", err)
	}
}

// gRPC 통신을 통해, for-hobom-backend 서버에 Outbox 데이터 업데이트를 위한 통신을 수행하도록 한다.
// Outbox DB 에 `FAILED` 상태로 업데이트를 한다.
func (p *messagePoller) markAsFailed(ctx context.Context, eventId, reason string) {
	if _, err := p.patchClient.PatchOutboxMarkAsFailedUseCase(ctx, &outboxPb.MarkFailedRequest{
		EventId:      eventId,
		ErrorMessage: reason,
	}); err != nil {
		slog.Error("failed to mark message outbox as FAILED", "eventId", eventId, "err", err)
	}
}
