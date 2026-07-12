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

// Poll fetches PENDING message outbox events from for-hobom-backend and
// publishes each to the "hobom.messages" Kafka topic.
//
// Flow per event:
//
//	gRPC FindOutbox(MESSAGE, PENDING)
//	  └─ for each item:
//	       ├─ build DeliverHoBomMessageCommand
//	       ├─ marshal to JSON
//	       │   └─ fail → markAsFailed
//	       ├─ publishWithRetry to Kafka
//	       │   └─ fail → markAsFailed + saveDLQ
//	       └─ markAsSent
//	            └─ fail → log warning (event already published, no DLQ)
func (p *messagePoller) Poll(ctx context.Context) error {
	req := &outboxPb.Request{
		EventType: EventTypeHoBomMessage.String(),
		Status:    OutboxPending.String(),
	}

	res, err := p.findClient.FindOutboxByEventTypeAndStatusUseCase(ctx, req)
	if err != nil {
		return fmt.Errorf("failed to fetch message outbox: %w", err)
	}

	for _, item := range res.Items {
		p.handleMessage(ctx, item)
	}
	return nil
}

func (p *messagePoller) handleMessage(ctx context.Context, item *outboxPb.QueryResult) {
	senderId := item.Payload.SenderId
	cmd := DeliverHoBomMessageCommand{
		Type:      item.Payload.Type,
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

	if err := p.markAsSent(ctx, eventId); err != nil {
		slog.Warn("published but failed to mark as SENT", "eventId", eventId, "err", err)
	}
}

// markAsSent updates the outbox entry to SENT via gRPC.
// Called only after successful Kafka publish.
func (p *messagePoller) markAsSent(ctx context.Context, eventId string) error {
	slog.Info("marking message outbox as SENT", "eventId", eventId)
	if _, err := p.patchClient.PatchOutboxMarkAsSentUseCase(ctx, &outboxPb.MarkRequest{
		EventId: eventId,
	}); err != nil {
		return fmt.Errorf("failed to mark message outbox as SENT: %w", err)
	}
	return nil
}

// markAsFailed updates the outbox entry to FAILED with the error reason.
// Called when marshal or Kafka publish fails.
func (p *messagePoller) markAsFailed(ctx context.Context, eventId, reason string) {
	if _, err := p.patchClient.PatchOutboxMarkAsFailedUseCase(ctx, &outboxPb.MarkFailedRequest{
		EventId:      eventId,
		ErrorMessage: reason,
	}); err != nil {
		slog.Error("failed to mark message outbox as FAILED", "eventId", eventId, "err", err)
	}
}
